package task

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/housepower/clickhouse_sinker/config"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

type Commit struct {
	group    string
	offsets  model.RecordMap
	wg       *sync.WaitGroup
	consumer *Consumer
}

type Consumer struct {
	sinker    *Sinker
	inputer   *input.KafkaFranz
	tasks     sync.Map
	grpConfig *config.GroupConfig
	fetchesCh chan *kgo.Fetches
	processWg sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	state     atomic.Uint32
	errCommit bool

	numFlying  int32
	mux        sync.Mutex
	commitDone *sync.Cond
}

const (
	MaxCountInBuf  = 1 << 27
	MaxParallelism = 10
)

func newConsumer(s *Sinker, gCfg *config.GroupConfig) *Consumer {
	c := &Consumer{
		sinker:    s,
		numFlying: 0,
		errCommit: false,
		grpConfig: gCfg,
		fetchesCh: make(chan *kgo.Fetches),
	}
	c.state.Store(util.StateStopped)
	c.commitDone = sync.NewCond(&c.mux)
	return c
}

func (c *Consumer) addTask(tsk *Service) {
	c.tasks.Store(tsk.taskCfg.Name, tsk)
}

func (c *Consumer) start() {
	if c.state.Load() == util.StateRunning {
		return
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.inputer = input.NewKafkaFranz()
	c.state.Store(util.StateRunning)
	if err := c.inputer.Init(c.sinker.curCfg, c.grpConfig, c.fetchesCh, c.cleanupFn); err == nil {
		go c.inputer.Run()
		go c.processFetch()
	} else {
		util.Logger.Fatal("failed to init consumer", zap.String("consumer", c.grpConfig.Name), zap.Error(err))
	}
}

func (c *Consumer) stop() {
	if c.state.Load() == util.StateStopped {
		return
	}
	c.state.Store(util.StateStopped)

	// stop the processFetch routine, make sure no more input to the commit chan & writing pool
	c.cancel()
	c.processWg.Wait()
	c.inputer.Stop()
}

func (c *Consumer) restart() {
	c.stop()
	c.start()
}

func (c *Consumer) cleanupFn() {
	// ensure the completion of writing to ck
	var wg sync.WaitGroup
	c.tasks.Range(func(key, value any) bool {
		wg.Add(1)
		go func(t *Service) {
			// drain ensure we have completeted persisting all received messages
			t.clickhouse.Drain()
			wg.Done()
		}(value.(*Service))
		return true
	})
	wg.Wait()

	// ensure the completion of offset submission
	c.mux.Lock()
	for c.numFlying != 0 {
		util.Logger.Debug("draining flying pending commits", zap.String("consumergroup", c.grpConfig.Name), zap.Int32("pending", c.numFlying))
		c.commitDone.Wait()
	}
	c.mux.Unlock()
}

func (c *Consumer) updateGroupConfig(g *config.GroupConfig) {
	if c.state.Load() == util.StateStopped {
		return
	}
	c.grpConfig = g
	// restart the processFetch routine because of potential BufferSize or FlushInterval change
	// make sure no more input to the commit chan & writing pool
	c.cancel()
	c.processWg.Wait()
	c.ctx, c.cancel = context.WithCancel(context.Background())
	go c.processFetch()
}

func (c *Consumer) processFetch() {
	c.processWg.Add(1)
	defer c.processWg.Done()
	recMap := make(model.RecordMap)
	var bufLength int

	flushFn := func() {
		if len(recMap) == 0 {
			return
		}
		var wg sync.WaitGroup
		c.tasks.Range(func(key, value any) bool {
			// flush to shard, ck
			task := value.(*Service)
			task.sharder.Flush(c.ctx, &wg, recMap[task.taskCfg.Topic])
			return true
		})
		bufLength = 0

		c.mux.Lock()
		c.numFlying++
		c.mux.Unlock()
		c.sinker.commitsCh <- &Commit{group: c.grpConfig.Name, offsets: recMap, wg: &wg, consumer: c}
		recMap = make(model.RecordMap)
	}

	bufThreshold := c.grpConfig.BufferSize * len(c.sinker.curCfg.Clickhouse.Hosts) * 4 / 5
	if bufThreshold > MaxCountInBuf {
		bufThreshold = MaxCountInBuf
	}

	ticker := time.NewTicker(time.Duration(c.grpConfig.FlushInterval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case fetches := <-c.fetchesCh:
			if c.state.Load() == util.StateStopped {
				continue
			}

			fetch := fetches.Records()
			items, done := int64(len(fetch)), int64(-1)
			var concurrency int
			if concurrency = int(items/1000) + 1; concurrency > MaxParallelism {
				concurrency = MaxParallelism
			}

			var wg sync.WaitGroup
			var err error
			wg.Add(concurrency)
			for i := 0; i < concurrency; i++ {
				go func() {
					for {
						index := atomic.AddInt64(&done, 1)
						if index >= items || c.state.Load() == util.StateStopped {
							wg.Done()
							break
						}

						rec := fetch[index]
						var prettyJSON bytes.Buffer
						err := json.Indent(&prettyJSON, rec.Value, "", "  ")
						if err != nil {
							return
						}
						var data map[string]interface{}
						err = json.Unmarshal([]byte(prettyJSON.String()), &data)
						if err != nil {
							fmt.Println("Error unmarshaling JSON:", err)
							return
						}
						if rec.Topic == "apache" || rec.Topic == "bsd_syslog" || rec.Topic == "http" {
							hostname := []string{
								"00c1ff47ea91",
								"01386986dccd",
								"013a3d45228b",
								"0554031f7833",
								"06880bf8d319",
								"076ce06672ef",
								"082c00bc249d",
								"0842a4fd0786",
								"0a9e2cfe29b2",
								"0d1b7b71f312",
								"1005ad71fd96",
								"12ca26568852",
								"13a63a8f754d",
								"179a8134654b",
								"187aeff3d2d3",
								"19d7cb80485c",
								"23429398a5f5",
								"242f64596671",
								"285db9d9cec4",
								"2cb36eec97b7",
								"2d298744911a",
								"2d44edd99782",
								"2e5fbb51f866",
								"35f51e5c8bd8",
								"36c7d513ad5c",
								"3aa01894401f",
								"3e361e23d683",
								"002bbff4b128",
								"017e2b579d4b",
								"0184a70bbc2b",
								"01f39a1fc780",
								"0219458df459",
								"029321d124e6",
								"02cb0793ae12",
								"035a012b83d0",
								"056e75f83fa7",
								"05fd0d3648ac",
								"06ac0fefc43f",
								"06b262ba9e77",
								"08497e104734",
								"0a9188cc6de8",
								"0b42c75c6945",
								"0bd852d6a30b",
								"0c45318f14c5",
								"0cbb02a208d8",
								"3e5486a2224f",
								"44b9280a30e4",
								"46d558ab745a",
								"474ba8a95866",
								"47820b5000d6",
								"49f5022c1227",
								"4b1ee19b75bd",
								"4c99fb7712c1",
								"4e36e071ed02",
								"4ed8e8e6f4aa",
								"52056b33bac5",
								"52ce82a66742",
								"537b0e8a04d3",
								"57b6b35def3f",
								"5880fbc705aa",
								"58fbe14991eb",
								"5a85e5f96a46",
								"5b3990247584",
								"5b4cf04776a9",
								"5c43f5690500",
								"5d9732abd073",
								"5e3c968129bd",
								"602c7de79bff",
								"60a61bc51d15",
								"61c8d9cd8373",
								"61f82ef2f952",
								"6aa9f00537e3",
								"6b12711f3aa3",
								"6b145d9241db",
								"6c6b62e09ee9",
								"750a42c4f53e",
								"75ef7f4bbcf1",
								"7621ee6c938d",
								"790b2b8fbdaa",
								"797bf8b9091a",
								"7a21f6f76927",
								"7a50105d0d8b",
								"7f025c678b27",
								"8304da48cb58",
								"89a13c966bd2",
								"89ae97d220c6",
								"89e54de130d6",
								"8a3ca18a6026",
								"8ab4b540b314",
								"8ade18f4ddee",
								"8b9c3f4e29f8",
								"8c4383c74087",
								"8ea6911d4e14",
								"8fbff5931328",
								"94242e2c0bb8",
								"957eadf7a565",
								"96d36894a986",
								"99b739d10fd1",
								"9aaf9b300af7",
								"9c20da6181ca",
							}
							if rec.Topic == "apache" || rec.Topic == "bsd_syslog" || rec.Topic == "http" {
								// fmt.Println("Topic", rec.Topic)
								// fmt.Println("Data", data)
								data["hostname"] = hostname[rand.Intn(len(hostname))]
								data["log_type"] = rec.Topic
								text := data["message"].(string)

								_, err := regexp.Match(".*info.*", []byte(strings.ToLower(text)))
								if err != nil {
									fmt.Println("Error:", err)
									return
								} else {
									data["log_level"] = "info"
								}

								_, err = regexp.Match(".*error.*|.*crit.*", []byte(strings.ToLower(text)))
								if err != nil {
									fmt.Println("Error:", err)
									return
								} else {
									data["log_level"] = "error"
								}

								_, err = regexp.Match(".*debug.*", []byte(strings.ToLower(text)))
								if err != nil {
									fmt.Println("Error:", err)
									return
								} else {
									data["log_level"] = "debug"
								}

								_, err = regexp.Match(".*trace.*", []byte(strings.ToLower(text)))
								if err != nil {
									fmt.Println("Error:", err)
									return
								} else {
									data["log_level"] = "trace"
								}
							}

						} else {

							var prettyJSON bytes.Buffer
							err := json.Indent(&prettyJSON, rec.Value, "", "  ")
							if err == nil {
								// fmt.Println("First aithe", prettyJSON.String())
								var data map[string]interface{}
								err := json.Unmarshal([]byte(prettyJSON.String()), &data)
								if err != nil {
									fmt.Println("Error unmarshaling JSON:", err)
									return
								}
								// fmt.Println("Extracting tags and moving to parent")
								for key, value := range data["tags"].(map[string]interface{}) {
									data[key] = value
								}

								if _, ok := data["gauge"]; ok {
									if gaugeValue, ok := data["gauge"].(map[string]interface{})["value"].(float64); ok {
										data["gauge"] = float64(gaugeValue)
									}
								}

								if _, ok := data["counter"]; ok {
									if gaugeValue, ok := data["counter"].(map[string]interface{})["value"].(float64); ok {
										data["counter"] = float64(gaugeValue)
									}
								}
								// rec.Value, err = json.Marshal(data)
							}
						}

						rec.Value, err = json.MarshalIndent(data, "", "  ")
						if err != nil {
							fmt.Println("Error marshaling JSON:", err)
							return
						} else {
							var newJSON bytes.Buffer
							err := json.Indent(&newJSON, rec.Value, "", "  ")
							if err == nil {
								// fmt.Println("check log level", newJSON.String())
							}
						}

						msg := &model.InputMessage{
							Topic:     rec.Topic,
							Partition: int(rec.Partition),
							Key:       rec.Key,
							Value:     rec.Value,
							Offset:    rec.Offset,
							Timestamp: &rec.Timestamp,
						}
						tablename := ""
						for _, it := range rec.Headers {
							if it.Key == "__table_name" {
								tablename = string(it.Value)
								break
							}
						}

						c.tasks.Range(func(key, value any) bool {
							tsk := value.(*Service)
							if (tablename != "" && tsk.clickhouse.TableName == tablename) || tsk.taskCfg.Topic == rec.Topic {
								bufLength++
								if e := tsk.Put(msg, flushFn); e != nil {
									atomic.StoreInt64(&done, items)
									err = e
									return false
								}
							}
							return true
						})
					}
				}()
			}
			wg.Wait()

			// record the latest offset in order
			// assume the c.state was reset to stopped when facing error, so that further fetch won't get processed
			if err == nil {
				for _, f := range *fetches {
					for i := range f.Topics {
						ft := &f.Topics[i]
						if recMap[ft.Topic] == nil {
							recMap[ft.Topic] = make(map[int32]*model.BatchRange)
						}
						for j := range ft.Partitions {
							fpr := ft.Partitions[j].Records
							if len(fpr) == 0 {
								continue
							}
							lastOff := fpr[len(fpr)-1].Offset
							firstOff := fpr[0].Offset

							or, ok := recMap[ft.Topic][ft.Partitions[j].Partition]
							if !ok {
								or = &model.BatchRange{Begin: math.MaxInt64, End: -1}
								recMap[ft.Topic][ft.Partitions[j].Partition] = or
							}
							if or.End < lastOff {
								or.End = lastOff
							}
							if or.Begin > firstOff {
								or.Begin = firstOff
							}
						}
					}
				}
			}

			if bufLength > bufThreshold {
				flushFn()
				ticker.Reset(time.Duration(c.grpConfig.FlushInterval) * time.Second)
			}
		case <-ticker.C:
			flushFn()
		case <-c.ctx.Done():
			util.Logger.Info("stopped processing loop", zap.String("group", c.grpConfig.Name))
			return
		}
	}
}
