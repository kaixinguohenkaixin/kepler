package main

import (
	"github.com/op/go-logging"
	"github.com/spf13/cobra"
	"github.com/vntchain/kepler/conf"
	"github.com/vntchain/kepler/node/node"
	"os"
)

const (
	DefaultLogLevel = logging.INFO
)

var logger = logging.MustGetLogger("main")
var format = logging.MustStringFormatter(
	`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
)

var mainCmd = &cobra.Command{
	Use: "node",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.HelpFunc()(cmd, args)
	},
}

func main() {
	backend := InitLogger()
	logger.Debug("[node] starting node ...")

	conf.InitConfig()
	ResetLogger(backend, conf.GetLogLevel())

	mainCmd.AddCommand(node.Cmd())
	if mainCmd.Execute() != nil {
		os.Exit(1)
	}

	logger.Info("[node] exiting.....")
}

func InitLogger() logging.LeveledBackend {
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	backendLeveled := logging.AddModuleLevel(backendFormatter)
	backendLeveled.SetLevel(DefaultLogLevel, "")
	logging.SetBackend(backendLeveled)
	return backendLeveled
}

var LogLevelMap = map[string]logging.Level{
	"error":   logging.ERROR,
	"warning": logging.WARNING,
	"info":    logging.INFO,
	"debug":   logging.DEBUG,
}

func ResetLogger(backend logging.LeveledBackend, level string) {
	if level != "" {
		if logLevel, ok := LogLevelMap[level]; ok {
			backend.SetLevel(logLevel, "")
			logging.SetBackend(backend)
			logger.Infof("[node] set logging level to %s", level)
		}
	}
}
