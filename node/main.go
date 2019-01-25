package main

import (
	"encoding/json"
	"github.com/vntchain/kepler/node/node"
	"github.com/op/go-logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"strings"
)

var logger = logging.MustGetLogger("main")

var format = logging.MustStringFormatter(
	`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
)

// Constants go here.
const cmdRoot = "config"

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

	// For demo purposes, create two backend for os.Stderr.
	backend1 := logging.NewLogBackend(os.Stderr, "", 0)
	backend2 := logging.NewLogBackend(os.Stderr, "", 0)

	// For messages written to backend2 we want to add some additional
	// information to the output, including the used log level and the name of
	// the function.
	backend2Formatter := logging.NewBackendFormatter(backend2, format)

	// Only errors and more severe messages should be sent to backend1
	backend1Leveled := logging.AddModuleLevel(backend1)
	backend1Leveled.SetLevel(logging.ERROR, "")

	// Set the backends to be used.
	logging.SetBackend(backend1Leveled, backend2Formatter)

	logger.Debug("Starting node.....")

	// For environment variables.
	viper.SetEnvPrefix(cmdRoot)
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	viper.SetConfigType("yaml")
	var altPath = os.Getenv("KEPLER_CFG_PATH")
	logger.Debugf("read altPath : %v", altPath)
	viper.AddConfigPath(altPath)
	viper.SetConfigName(cmdRoot)

	err := viper.ReadInConfig()
	if err != nil {
		logger.Panicf("init the config file err %v", err)
	}

	var b = []byte(viper.GetString("consortium.peers"))
	m := viper.GetStringMap("consortium.peers")
	if len(b) != 0 {
		m = make(map[string]interface{})

		err = json.Unmarshal(b, &m)
		logger.Debugf("read peers is: %s", b)
		if err != nil {
			logger.Warningf("init the config consortium.peers err %v", err)
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

	logger.Infof("read mspid is: %v", viper.GetString("consortium.mspId"))

	mainCmd.AddCommand(node.Cmd())

	// On failure Cobra prints the usage message and error string, so we only
	// need to exit with a non-0 status
	if mainCmd.Execute() != nil {
		os.Exit(1)
	}
	logger.Info("Exiting.....")
}
