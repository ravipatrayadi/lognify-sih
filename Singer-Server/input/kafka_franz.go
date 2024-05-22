package input

import (
	"context"
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	krb5client "github.com/jcmturner/gokrb5/v8/client"
	krb5config "github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/keytab"
	"github.com/thanos-io/thanos/pkg/errors"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"github.com/twmb/franz-go/plugin/kzap"
	"go.uber.org/zap"
)

const (
	Krb5KeytabAuth = 2
	CommitRetries  = 6
	RetryBackoff   = 5 * time.Second
	processTimeOut = 10
)

type KafkaFranz struct {
	cfg       *config.Config
	grpConfig *config.GroupConfig
	cl        *kgo.Client
	ctx       context.Context
	cancel    context.CancelFunc
	wgRun     sync.WaitGroup
	fetch     chan *kgo.Fetches
	cleanupFn func()
}

func NewKafkaFranz() *KafkaFranz {
	return &KafkaFranz{}
}

func (k *KafkaFranz) Init(cfg *config.Config, gCfg *config.GroupConfig, f chan *kgo.Fetches, cleanupFn func()) (err error) {
	k.cfg = cfg
	k.grpConfig = gCfg
	k.ctx, k.cancel = context.WithCancel(context.Background())
	k.fetch = f
	k.cleanupFn = cleanupFn
	kfkCfg := &cfg.Kafka
	var opts []kgo.Opt
	if opts, err = GetFranzConfig(kfkCfg); err != nil {
		return
	}
	opts = append(opts,
		kgo.ConsumeTopics(k.grpConfig.Topics...),
		kgo.ConsumerGroup(k.grpConfig.Name),
		kgo.DisableAutoCommit(),
	)

	maxPartBytes := int32(1 << (util.GetShift(100*k.grpConfig.BufferSize) - 1))

	opts = append(opts,
		kgo.FetchMaxBytes(maxPartBytes),
		kgo.FetchMaxPartitionBytes(maxPartBytes),
		kgo.OnPartitionsRevoked(k.onPartitionRevoked),
		kgo.RebalanceTimeout(time.Minute*2),
		kgo.SessionTimeout(time.Minute*2),
		kgo.RequestTimeoutOverhead(time.Minute*1),
	)
	if !k.grpConfig.Earliest {
		opts = append(opts, kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()))
	}

	if k.cl, err = kgo.NewClient(opts...); err != nil {
		err = errors.Wrapf(err, "")
		return
	}
	return nil
}

func GetFranzConfig(kfkCfg *config.KafkaConfig) (opts []kgo.Opt, err error) {
	opts = []kgo.Opt{
		kgo.SeedBrokers(strings.Split(kfkCfg.Brokers, ",")...),
		// kgo.BrokerMaxReadBytes(), // 100 MB
		kgo.MaxConcurrentFetches(2),
		kgo.WithLogger(kzap.New(util.Logger)),
	}
	if kfkCfg.TLS.Enable {
		var tlsCfg *tls.Config
		if tlsCfg, err = util.NewTLSConfig(kfkCfg.TLS.CaCertFiles, kfkCfg.TLS.ClientCertFile, kfkCfg.TLS.ClientKeyFile, kfkCfg.TLS.EndpIdentAlgo == ""); err != nil {
			return
		}
		opts = append(opts, kgo.DialTLSConfig(tlsCfg))
	}
	if kfkCfg.Sasl.Enable {
		var mch sasl.Mechanism
		switch kfkCfg.Sasl.Mechanism {
		case "PLAIN":
			auth := plain.Auth{
				User: kfkCfg.Sasl.Username,
				Pass: kfkCfg.Sasl.Password,
			}
			mch = auth.AsMechanism()
		case "SCRAM-SHA-256", "SCRAM-SHA-512":
			auth := scram.Auth{
				User: kfkCfg.Sasl.Username,
				Pass: kfkCfg.Sasl.Password,
			}
			switch kfkCfg.Sasl.Mechanism {
			case "SCRAM-SHA-256":
				mch = auth.AsSha256Mechanism()
			case "SCRAM-SHA-512":
				mch = auth.AsSha512Mechanism()
			default:
			}
		case "GSSAPI":
			gssapiCfg := kfkCfg.Sasl.GSSAPI
			auth := kerberos.Auth{Service: gssapiCfg.ServiceName}
			var krbCfg *krb5config.Config
			var kt *keytab.Keytab
			if krbCfg, err = krb5config.Load(gssapiCfg.KerberosConfigPath); err != nil {
				err = errors.Wrapf(err, "")
				return
			}
			if gssapiCfg.AuthType == Krb5KeytabAuth {
				if kt, err = keytab.Load(gssapiCfg.KeyTabPath); err != nil {
					err = errors.Wrapf(err, "")
					return
				}
				auth.Client = krb5client.NewWithKeytab(gssapiCfg.Username, gssapiCfg.Realm, kt, krbCfg, krb5client.DisablePAFXFAST(gssapiCfg.DisablePAFXFAST))
			} else {
				auth.Client = krb5client.NewWithPassword(gssapiCfg.Username,
					gssapiCfg.Realm, gssapiCfg.Password, krbCfg, krb5client.DisablePAFXFAST(gssapiCfg.DisablePAFXFAST))
			}
			mch = auth.AsMechanismWithClose()
		}
		if mch != nil {
			opts = append(opts, kgo.SASL(mch))
		}
	}
	return
}

func (k *KafkaFranz) Run() {
	k.wgRun.Add(1)
	defer k.wgRun.Done()
LOOP:
	for {
		fetches := k.cl.PollRecords(k.ctx, k.grpConfig.BufferSize)
		err := fetches.Err()
		if fetches == nil || fetches.IsClientClosed() || errors.Is(err, context.Canceled) {
			break
		}
		if err != nil {
			err = errors.Wrapf(err, "")
			util.Logger.Info("kgo.Client.PollFetchs() got an error", zap.Error(err))
		}

		util.Logger.Debug("Records fetched", zap.String("records", strconv.Itoa(fetches.NumRecords())), zap.String("consumer group", k.grpConfig.Name))

		t := time.NewTimer(processTimeOut * time.Minute)
		select {
		case k.fetch <- &fetches:
			t.Stop()
		case <-k.ctx.Done():
			t.Stop()
			break LOOP
		case <-t.C:
			util.Logger.Fatal(fmt.Sprintf("Sinker abort because group %s was not processing in last %d minutes", k.grpConfig.Name, processTimeOut))
		}
	}
	k.cl.Close()
	util.Logger.Info("KafkaFranz.Run quit due to context has been canceled", zap.String("consumer group", k.grpConfig.Name))
}

func (k *KafkaFranz) CommitMessages(msg *model.InputMessage) error {
	var err error
	for i := 0; i < CommitRetries; i++ {
		err = k.cl.CommitRecords(context.Background(), &kgo.Record{Topic: msg.Topic, Partition: int32(msg.Partition), Offset: msg.Offset, LeaderEpoch: -1})
		if err == nil {
			break
		}
		err = errors.Wrapf(err, "")
		if i < CommitRetries-1 && !errors.Is(err, context.Canceled) {
			util.Logger.Error("cl.CommitRecords failed, will retry later", zap.String("consumer group", k.grpConfig.Name), zap.Int("try", i), zap.Error(err))
			time.Sleep(RetryBackoff)
		}
	}
	return err
}

func (k *KafkaFranz) Stop() {
	k.cancel()

	quit := make(chan struct{})
	go func() {
		select {
		case <-k.fetch:
		case <-quit:
		}
	}()

	k.wgRun.Wait()
	select {
	case quit <- struct{}{}:
	default:
	}
}

func (k *KafkaFranz) Description() string {
	return fmt.Sprint("kafka consumer group ", k.grpConfig.Name)
}

func (k *KafkaFranz) onPartitionRevoked(_ context.Context, _ *kgo.Client, _ map[string][]int32) {
	begin := time.Now()
	k.cleanupFn()
	util.Logger.Info("consumer group cleanup",
		zap.String("consumer group", k.grpConfig.Name),
		zap.Duration("cost", time.Since(begin)))
}
