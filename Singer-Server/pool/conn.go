package pool

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/thanos-io/thanos/pkg/errors"
	"go.uber.org/zap"
)

var (
	lock        sync.Mutex
	clusterConn []*ShardConn
)

type ShardConn struct {
	lock        sync.Mutex
	conn        *Conn
	dbVer       int
	opts        clickhouse.Options
	replicas    []string
	nextRep     int
	writingPool *util.WorkerPool
	protocol    clickhouse.Protocol
	chCfg       *config.ClickHouseConfig
}

func (sc *ShardConn) SubmitTask(fn func()) (err error) {
	return sc.writingPool.Submit(fn)
}

func (sc *ShardConn) GetReplica() (replica string) {
	sc.lock.Lock()
	defer sc.lock.Unlock()
	if sc.conn != nil {
		curRep := (len(sc.replicas) + sc.nextRep - 1) % len(sc.replicas)
		replica = sc.replicas[curRep]
	}
	return
}
func (sc *ShardConn) Close() {
	sc.lock.Lock()
	defer sc.lock.Unlock()
	if sc.conn != nil {
		sc.conn.Close()
		sc.conn = nil
	}
	if sc.writingPool != nil {
		sc.writingPool.StopWait()
	}
}

func (sc *ShardConn) NextGoodReplica(failedVer int) (db *Conn, dbVer int, err error) {
	sc.lock.Lock()
	defer sc.lock.Unlock()
	if sc.conn != nil {
		if sc.dbVer > failedVer {
			// Another goroutine has already done connection.
			// Notice: Why recording failure version instead timestamp?
			// Consider following scenario:
			// conn1 = NextGood(0); conn2 = NexGood(0); conn1.Exec failed at ts1;
			// conn3 = NextGood(ts1); conn2.Exec failed at ts2;
			// conn4 = NextGood(ts2) will close the good connection and break users.
			return sc.conn, sc.dbVer, nil
		}
		sc.conn.Close()
		sc.conn = nil
	}
	savedNextRep := sc.nextRep
	conn := Conn{
		protocol: sc.protocol,
		ctx:      context.Background(),
	}
	for i := 0; i < len(sc.replicas); i++ {
		replica := sc.replicas[sc.nextRep]
		sc.opts.Addr = []string{replica}
		sc.nextRep = (sc.nextRep + 1) % len(sc.replicas)
		if sc.protocol == clickhouse.HTTP {
			conn.db = clickhouse.OpenDB(&sc.opts)
			conn.db.SetMaxOpenConns(sc.chCfg.MaxOpenConns)
			conn.db.SetMaxIdleConns(sc.chCfg.MaxOpenConns)
			conn.db.SetConnMaxLifetime(time.Minute * 10)
		} else {
			conn.c, err = clickhouse.Open(&sc.opts)
		}
		if err != nil {
			util.Logger.Warn("clickhouse.Open failed", zap.String("replica", replica), zap.Error(err))
			continue
		}
		sc.dbVer++
		util.Logger.Info("clickhouse.Open succeeded", zap.Int("dbVer", sc.dbVer), zap.String("replica", replica))
		sc.conn = &conn
		return sc.conn, sc.dbVer, nil
	}
	err = errors.Newf("no good replica among replicas %v since %d", sc.replicas, savedNextRep)
	return nil, sc.dbVer, err
}

func InitClusterConn(chCfg *config.ClickHouseConfig) (err error) {
	lock.Lock()
	defer lock.Unlock()
	freeClusterConn()

	proto := clickhouse.Native
	if chCfg.Protocol == clickhouse.HTTP.String() {
		proto = clickhouse.HTTP
	}

	for _, replicas := range chCfg.Hosts {
		numReplicas := len(replicas)
		replicaAddrs := make([]string, numReplicas)
		for i, ip := range replicas {
			if !chCfg.Secure {
				if ips2, err := util.GetIP4Byname(ip); err == nil {
					ip = ips2[0]
				}
			}
			replicaAddrs[i] = fmt.Sprintf("%s:%d", ip, chCfg.Port)
		}
		sc := &ShardConn{
			replicas: replicaAddrs,
			chCfg:    chCfg,
			opts: clickhouse.Options{
				Auth: clickhouse.Auth{
					Database: chCfg.DB,
					Username: chCfg.Username,
					Password: chCfg.Password,
				},
				Protocol:    proto,
				DialTimeout: time.Minute * 10,
			},
			writingPool: util.NewWorkerPool(chCfg.MaxOpenConns, 1),
		}
		if chCfg.Secure {
			tlsConfig := &tls.Config{}
			tlsConfig.InsecureSkipVerify = chCfg.InsecureSkipVerify
			sc.opts.TLS = tlsConfig
		}
		if proto == clickhouse.Native {
			sc.opts.MaxOpenConns = chCfg.MaxOpenConns
			sc.opts.MaxIdleConns = chCfg.MaxOpenConns
			sc.opts.ConnMaxLifetime = time.Minute * 10
		}
		sc.protocol = proto
		if _, _, err = sc.NextGoodReplica(0); err != nil {
			return
		}
		clusterConn = append(clusterConn, sc)
	}
	return
}

func freeClusterConn() {
	for _, sc := range clusterConn {
		sc.Close()
	}
	clusterConn = []*ShardConn{}
}

func FreeClusterConn() {
	lock.Lock()
	defer lock.Unlock()
	freeClusterConn()
}

func NumShard() (cnt int) {
	lock.Lock()
	defer lock.Unlock()
	return len(clusterConn)
}

func GetShardConn(batchNum int64) (sc *ShardConn) {
	lock.Lock()
	defer lock.Unlock()
	sc = clusterConn[batchNum%int64(len(clusterConn))]
	return
}

func CloseAll() {
	FreeClusterConn()
}
