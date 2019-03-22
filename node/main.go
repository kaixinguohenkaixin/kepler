package main

import (
	"github.com/op/go-logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vntchain/kepler/node/node"
	"os"
	"strings"
	"encoding/json"
)

var logger = logging.MustGetLogger("main")
var format = logging.MustStringFormatter(
	`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
)

const ConfigPath = "config"

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
	InitLogger()
	logger.Debug("[node] starting node ...")

	InitConfig()

	mainCmd.AddCommand(node.Cmd())
	if mainCmd.Execute() != nil {
		os.Exit(1)
	}

	logger.Info("[node] Exiting.....")
}

func InitLogger() {
	backend1 := logging.NewLogBackend(os.Stderr, "", 0)
	backend2 := logging.NewLogBackend(os.Stderr, "", 0)
	backend2Formatter := logging.NewBackendFormatter(backend2, format)
	backend1Leveled := logging.AddModuleLevel(backend1)
	backend1Leveled.SetLevel(logging.ERROR, "")
	logging.SetBackend(backend1Leveled, backend2Formatter)
}

func InitConfig() {
	viper.SetEnvPrefix(ConfigPath)
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.SetConfigType("yaml")
	var configPath = os.Getenv("KEPLER_CFG_PATH")
	logger.Debugf("[node] set KEPLER_CFG_PATH: %s", configPath)
	viper.AddConfigPath(configPath)
	viper.SetConfigName(ConfigPath)
	err := viper.ReadInConfig()
	if err != nil {
		logger.Panicf("[node] init config file failed: %s", err)
	}

	var b = []byte(viper.GetString("consortium.peers"))
	m := viper.GetStringMap("consortium.peers")
	if len(b) != 0 {
		m = make(map[string]interface{})
		err := json.Unmarshal(b, &m)
		logger.Debugf("read peers is: %s", b)
		if err != nil {
			logger.Warningf("[node] init consortium peers failed: %s", err)
		}

		newm := make(map[interface{}]interface{})
		for pkey, pvalue := range m {
			mm := make(map[interface{}]interface{})
			for k, v := range pvalue.(map[string]interface{}) {
				if k == "tls" {
					tlsm := make(map[interface{}]interface{})
					for tlsk, tlsv := range v.(map[string]interface{}) {
						tlsm[tlsk] = tlsv
					}
					mm[k] = tlsm
					continue
				}
				mm[k] = v
			}
			newm[pkey] = mm
		}
		viper.Set("consortium.peers", newm)
	}
}