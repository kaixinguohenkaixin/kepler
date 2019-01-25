package txmanager

import (
	"fmt"
	"github.com/spf13/viper"
	"strings"
	"time"

	"github.com/vntchain/kepler/core/public/sdk"
	sdkutils "github.com/vntchain/kepler/core/public/sdk/common"

	pubevent "github.com/vntchain/kepler/event/public"
	"math/big"

	"github.com/vntchain/go-vnt/accounts"
	"github.com/vntchain/go-vnt/accounts/keystore"
	ethcom "github.com/vntchain/go-vnt/common"
	"github.com/vntchain/go-vnt/core/types"
)

type NodeManager struct {
	httpUrl  string
	wsPath   string
	keyDir   string
	nodeName string
	ethSDK   *sdk.EthSDK
}

func (nm *NodeManager) Init(key string, config interface{}, ks *keystore.KeyStore) (err error) {

	nm.nodeName = key

	nodeConfig := config.(map[interface{}]interface{})

	nm.httpUrl = nodeConfig["httpUrl"].(string)
	nm.wsPath = nodeConfig["wsPath"].(string)

	logger.Debugf("init node manager httpUrl:%s wsPath:%s ", nm.httpUrl, nm.wsPath)

	sdkConfig := sdk.EthConfig{HttpUrl: nm.httpUrl, WsPath: nm.wsPath}

	nm.ethSDK, err = sdk.NewEthSDK(sdkConfig, ks)

	return err
}

func (nm *NodeManager) ListenWsEvent(contractManager *ContractManager, userToCChan chan *pubevent.LogUserToC) {
	logger.Debugf("nodeManager start listening ws event")
	contractAddress := contractManager.GetContractAddress()
	log := make(chan types.Log, 1)

	nm.ethSDK.RetraceLog(sdkutils.RetraceConf{
		HttpClient:      nm.ethSDK.HttpClient,
		ContractAddress: contractAddress,
		SyncInterval:    5 * time.Second,
		RetraceInterval: 3 * time.Second,
	}, log)

	for {
		l := <-log
		data := l.Data
		topics := l.Topics
		blockNumber := l.BlockNumber
		txHash := l.TxHash
		txIndex := l.TxIndex
		removed := l.Removed
		blockHash := l.BlockHash

		if len(topics) > 0 {

			eventname := topics[1].Hex()
			logger.Debugf("read an event: name is %s", eventname)

			EventRollback := viper.GetString("public.EventRollback")
			EventUserToC := viper.GetString("public.EventUserToC")
			EventCToUser := viper.GetString("public.EventCToUser")

			if strings.Contains(eventname, EventRollback) {
				//回滚事件成功，做相应的处理
				logger.Debugf("rollback rollback")
			} else if strings.Contains(eventname, EventUserToC) {
				logger.Debugf("userToC event")
				userToC, err := contractManager.ReadUserToCEvent(data, topics, blockNumber, blockHash, txHash, txIndex, removed)
				if err != nil {
					logger.Errorf("ReadUserToCEvent err:%v", err)
					break
				}
				logger.Debugf("ReadUserToCEvent to userToC is %v", userToC)
				userToCChan <- userToC
			} else if strings.Contains(eventname, EventCToUser) {
				//CToUser，做相应的处理
				logger.Debugf("cToUser event")
			}

		}

	}
}

func (nm *NodeManager) GetEthSDK() (*sdk.EthSDK, error) {
	if nm.ethSDK == nil {
		return nil, fmt.Errorf("public chain nodeManager sdk is nil")
	}

	return nm.ethSDK, nil
}

func (nm *NodeManager) SendRawTransaction(chainId int, contractAdd ethcom.Address, data []byte, isWait bool) (string, error) {
	var nonce uint64
	var acc accounts.Account

	if len(nm.ethSDK.Keystore.Accounts()) == 0 {
		acc, _ = nm.ethSDK.Keystore.NewAccount("123456")
	} else {
		acc = nm.ethSDK.Keystore.Accounts()[0]
	}
	var err error

	if nonce, err = nm.ethSDK.GetNonce(acc.Address); err != nil {
		return "", fmt.Errorf("Failed to getNonce of %s with %s", acc.Address.Hex(), err.Error())
	}
	chainIdBig := big.NewInt(int64(chainId))
	amount := big.NewInt(0)
	price := big.NewInt(21)
	//gasLimit, _ := nm.ethSDK.EstimateGas(acc.Address, contractAdd, data, amount, price)
	gasLimit := uint64(1000000)
	if err = nm.ethSDK.Keystore.Unlock(acc, "123456"); err != nil {
		return "", fmt.Errorf("Failed to unlock acc with %s", err.Error())
	}
	accBalance, _ := nm.ethSDK.GetBalanceByHex(acc.Address.Hex())
	logger.Errorf("[public SendRawTransaction] accBalance: %#v", accBalance)
	signedData, err := nm.ethSDK.FormSignedTransaction(&acc, chainIdBig, nonce, contractAdd, amount, gasLimit, price, data)
	if err != nil {
		logger.Errorf("Failed to FormSignedData with %s", err.Error())
		return "", err
	}
	sendRes, err := nm.ethSDK.SendRawTransaction(signedData, isWait)
	if err != nil {
		logger.Errorf("Failed to SendRawTransaction with %s\n", err.Error())
		return "", err
	}
	return sendRes.Hex(), nil
}

func (nm *NodeManager) GetTransactionReceipt(txHashStr string) (map[string]interface{}, error) {
	return nm.ethSDK.GetTransactionReceipt(ethcom.HexToHash(txHashStr))
}
