package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

var (
	version = "None"
	commit  = "None"
	date    = "None"
	builtBy = "None"

	cmdOps      util.CmdOptions
	httpAddr    string
	httpMetrics = promhttp.Handler()
	runner      *task.Sinker
)

const (
	HttpPortBase = 10000
)

func initCmdOptions() {
	// 1. Set options to default value.
	cmdOps = util.CmdOptions{
		LogLevel:      "info",
		LogPaths:      "stdout,clickhouse_sinker.log",
		PushInterval:  10,
		LocalCfgFile:  "/etc/clickhouse_sinker.hjson",
		NacosAddr:     "127.0.0.1:8848",
		NacosGroup:    "DEFAULT_GROUP",
		NacosUsername: "nacos",
		NacosPassword: "nacos",
	}

	util.EnvStringVar(&cmdOps.LocalCfgFile, "local-cfg-file")

	util.EnvStringVar(&cmdOps.ClickhouseUsername, "clickhouse-username")
	util.EnvStringVar(&cmdOps.ClickhousePassword, "clickhouse-password")
	util.EnvStringVar(&cmdOps.KafkaUsername, "kafka-username")
	util.EnvStringVar(&cmdOps.KafkaPassword, "kafka-password")

	flag.StringVar(&cmdOps.LogLevel, "log-level", cmdOps.LogLevel, "one of debug, info, warn, error, dpanic, panic, fatal")
	flag.StringVar(&cmdOps.LogPaths, "log-paths", cmdOps.LogPaths, "a list of comma-separated log file path. stdout means the console stdout")
	flag.IntVar(&cmdOps.PushInterval, "push-interval", cmdOps.PushInterval, "push interval in seconds")
	flag.StringVar(&cmdOps.LocalCfgFile, "local-cfg-file", cmdOps.LocalCfgFile, "local config file")

	flag.StringVar(&cmdOps.ClickhouseUsername, "clickhouse-username", cmdOps.ClickhouseUsername, "clickhouse username")
	flag.StringVar(&cmdOps.ClickhousePassword, "clickhouse-password", cmdOps.ClickhousePassword, "clickhouse password")
	flag.StringVar(&cmdOps.KafkaUsername, "kafka-username", cmdOps.KafkaUsername, "kafka username")
	flag.StringVar(&cmdOps.KafkaPassword, "kafka-password", cmdOps.KafkaPassword, "kafka password")
	flag.StringVar(&cmdOps.KafkaGSSAPIUsername, "kafka-gssapi-username", cmdOps.KafkaGSSAPIUsername, "kafka GSSAPI username")
	flag.StringVar(&cmdOps.KafkaGSSAPIPassword, "kafka-gssapi-password", cmdOps.KafkaGSSAPIPassword, "kafka GSSAPI password")

	flag.Parse()
}

func getVersion() string {
	return fmt.Sprintf("version %s, commit %s, date %s, builtBy %s, pid %v", version, commit, date, builtBy, os.Getpid())
}

func init() {
	initCmdOptions()
	logPaths := strings.Split(cmdOps.LogPaths, ",")
	util.InitLogger(logPaths)
	util.SetLogLevel(cmdOps.LogLevel)
	util.Logger.Info(getVersion())
	if cmdOps.ShowVer {
		os.Exit(0)
	}
	util.Logger.Info("parsed command options:", zap.Reflect("opts", cmdOps))
}

func main() {
	util.Run("clickhouse_sinker", func() error {
		// Initialize http server for metrics and debug
		httpPort := cmdOps.HTTPPort
		fmt.Println("httpPort:", httpPort)
		if httpPort == 0 {
			httpPort = util.GetSpareTCPPort(HttpPortBase)
		}

		httpHost := cmdOps.HTTPHost
		if httpHost == "" {
			ip, err := util.GetOutboundIP()
			if err != nil {
				return fmt.Errorf("failed to determine outbound ip: %w", err)
			}
			httpHost = ip.String()
		}

		httpAddr = fmt.Sprintf("%s:%d", httpHost, httpPort)
		listener, err := net.Listen("tcp", httpAddr)
		if err != nil {
			return fmt.Errorf("failed to listen on %q: %w", httpAddr, err)
		}

		util.Logger.Info(fmt.Sprintf("Run http server at http://%s/", httpAddr))

		go func() {
			if err := http.Serve(listener, mux); err != nil {
				util.Logger.Error("http.ListenAndServe failed", zap.Error(err))
			}
		}()

		var rcm cm.RemoteConfManager
		var properties map[string]interface{}
		logDir := "."
		logPaths := strings.Split(cmdOps.LogPaths, ",")
		for _, logPath := range logPaths {
			if logPath != "stdout" && logPath != "stderr" {
				logDir, _ = filepath.Split(logPath)
			}
		}
		logDir, _ = filepath.Abs(logDir)
		if cmdOps.NacosDataID != "" {
			util.Logger.Info(fmt.Sprintf("get config from nacos serverAddrs %s, namespaceId %s, group %s, dataId %s",
				cmdOps.NacosAddr, cmdOps.NacosNamespaceID, cmdOps.NacosGroup, cmdOps.NacosDataID))
			rcm = &cm.NacosConfManager{}
			properties = make(map[string]interface{}, 8)
			properties["serverAddrs"] = cmdOps.NacosAddr
			properties["username"] = cmdOps.NacosUsername
			properties["password"] = cmdOps.NacosPassword
			properties["namespaceId"] = cmdOps.NacosNamespaceID
			properties["group"] = cmdOps.NacosGroup
			properties["dataId"] = cmdOps.NacosDataID
			properties["serviceName"] = cmdOps.NacosServiceName
			properties["logDir"] = logDir
		} else {
			util.Logger.Info(fmt.Sprintf("get config from local file %s", cmdOps.LocalCfgFile))
		}
		if rcm != nil {
			if err := rcm.Init(properties); err != nil {
				util.Logger.Fatal("rcm.Init failed", zap.Error(err))
			}
			if cmdOps.NacosServiceName != "" {
				if err := rcm.Register(httpHost, httpPort); err != nil {
					util.Logger.Fatal("rcm.Init failed", zap.Error(err))
				}
			}
		}
		runner = task.NewSinker(rcm, httpAddr, &cmdOps)
		return runner.Init()
	}, func() error {
		runner.Run()
		return nil
	}, func() error {
		runner.Close()
		return nil
	})
}
