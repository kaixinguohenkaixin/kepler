package conf

import (
	"encoding/json"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"os"
	"strings"
)

const (
	ConfigPath    = "config"
	KeplerCfgPath = "KEPLER_CFG_PATH"
)

var logger = logging.MustGetLogger("config")

type CryptoConf struct {
	Hash     string
	Security int
}

type ChaincodeConf struct {
	Name           string
	Version        string
	CTransfer      string
	QueryCTransfer string
	CRevert        string
	QueryCRevert   string
	CApprove       string
	QueryCApprove  string
	LogCToUser     string
	LogUserToC     string
	Transfered     string
	RollBack       string
}

type TLSConf struct {
	Enabled            bool
	ClientAuthRequired bool
	ClientKeyFile      string
	ClientCertFile     string
	RootcertFile       string
}

type PeerConf struct {
	Address      string
	Sn           string
	EventAddress string
	Tls          TLSConf
}

type OrdererConf struct {
	Address string
	Sn      string
	Tls     TLSConf
}

type ConsortiumConf struct {
	Crypto CryptoConf

	PrivateKeyPath string
	CertPath       string
	MspId          string

	ChannelName string
	Chaincode   ChaincodeConf

	AgreedCount int
	Peers       map[string]PeerConf
	Orderers    map[string]OrdererConf
}

type ContractConf struct {
	Address       string
	AbiPath       string
	EventUserToC  string
	EventCToUser  string
	EventRollback string
	LogUserToC    string
	CRollback     string
}

type NodeConf struct {
	HttpUrl string
	WsPath  string
}

type PublicConf struct {
	ChainId  int
	KeyPath  string
	Password string

	Contract ContractConf

	Nodes map[string]NodeConf
}

var TheConsortiumConf ConsortiumConf
var ThePublicConf PublicConf

func InitConfig() {
	viper.SetEnvPrefix(ConfigPath)
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	viper.SetConfigType("yaml")
	var configPath = os.Getenv(KeplerCfgPath)
	logger.Debugf("[node] set %s: %s", KeplerCfgPath, configPath)
	viper.AddConfigPath(configPath)
	viper.SetConfigName(ConfigPath)
	err := viper.ReadInConfig()
	if err != nil {
		logger.Panicf("[config] init config file failed: %s", err)
	}

	err = viper.UnmarshalKey("consortium.crypto", &TheConsortiumConf.Crypto)
	if err != nil {
		logger.Panicf("[config] get consortium crypto config failed: %s", err)
	}
	TheConsortiumConf.PrivateKeyPath = viper.GetString("consortium.privateKeyPath")
	TheConsortiumConf.CertPath = viper.GetString("consortium.certPath")
	TheConsortiumConf.MspId = viper.GetString("consortium.mspId")
	TheConsortiumConf.ChannelName = viper.GetString("consortium.channelName")
	err = viper.UnmarshalKey("consortium.chaincode", &TheConsortiumConf.Chaincode)
	if err != nil {
		logger.Panicf("[config] get consortium chaincode config failed: %s", err)
	}
	TheConsortiumConf.AgreedCount = viper.GetInt("consortium.agreedCount")
	if envPeers := viper.GetString("consortium.peers"); envPeers != "" {
		err = json.Unmarshal([]byte(envPeers), &TheConsortiumConf.Peers)
		if err != nil {
			logger.Panicf("[config] get consortium peers env config failed: %s", err)
		}
	} else {
		err = viper.UnmarshalKey("consortium.peers", &TheConsortiumConf.Peers)
		if err != nil {
			logger.Panicf("[config] get consortium peers config failed: %s", err)
		}
	}
	err = viper.UnmarshalKey("consortium.orderers", &TheConsortiumConf.Orderers)
	if err != nil {
		logger.Panicf("[config] get consortium orderers config failed: %s", err)
	}
	logger.Infof("[Debug] [config] consortium config: %#v", TheConsortiumConf)

	ThePublicConf.ChainId = viper.GetInt("public.chainId")
	ThePublicConf.KeyPath = viper.GetString("public.keyPath")
	ThePublicConf.Password = viper.GetString("public.password")
	err = viper.UnmarshalKey("public.contract", &ThePublicConf.Contract)
	if err != nil {
		logger.Panicf("[config] get public contract config failed: %s", err)
	}
	err = viper.UnmarshalKey("public.nodes", &ThePublicConf.Nodes)
	if err != nil {
		logger.Panicf("[config] get public nodes config failed: %s", err)
	}
	logger.Infof("[Debug] [config] public config: %#v", ThePublicConf)
}

func GetLogLevel() string {
	return viper.GetString("logging.level")
}
