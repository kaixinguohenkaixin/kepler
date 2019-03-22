package endorser

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
	ethcom "github.com/vntchain/go-vnt/common"
	cevent "github.com/vntchain/kepler/event/consortium"
	"github.com/vntchain/kepler/protos/common"
	pb "github.com/vntchain/kepler/protos/peer"
	"github.com/vntchain/kepler/utils"
	"math/big"
	"time"
)

const (
	AttemptCount    = 3
	RetraceInterval = 3 * time.Second
)

var logger = logging.MustGetLogger("consortium_sdk")

type TransactionHandler struct {
	PeerClient           *PeerClient
	RegisteredTxEvent    map[string]chan int
	RegisteredEventByCId map[string]chan interface{}
	retracer             *retracer
}

func (th *TransactionHandler) Init(signer interface{}, creator []byte, chaincodeID string, LogCToUser string, Transfered string, RollBack string, cTransfer string) (err error) {
	th.RegisteredTxEvent = make(map[string]chan int)
	th.RegisteredEventByCId = make(map[string]chan interface{})

	config := RetraceConf{
		PeerClient:      th.PeerClient,
		Channel:         viper.GetString("consortium.channelName"),
		RetraceInterval: RetraceInterval,
		Signer:          signer,
		Creator:         creator,
	}
	th.retracer, err = InitRetracer(config)
	if err != nil {
		err = fmt.Errorf("[consortium sdk] init retracer falied: %s", err)
		logger.Error(err)
		return
	}
	go th.retracer.Process()

	th.waitUntilEvent(chaincodeID, LogCToUser, Transfered, RollBack, cTransfer)
	return
}

func (th *TransactionHandler) CreateProposal(chainId string, chaincodeName string, chaincodeVersion string, funcName string, creator []byte, args ...string) (*pb.Proposal, string, error) {
	spec, err := utils.GetChaincodeSpecification(chaincodeName, chaincodeVersion, funcName, args...)
	if err != nil {
		err = fmt.Errorf("[consortium sdk] get chaincode specification falied: %s", err)
		logger.Error(err)
		return nil, "", err
	}

	invocation := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}
	prop, txid, err := utils.CreateProposalFromCIS(common.HeaderType_ENDORSER_TRANSACTION, chainId, invocation, creator)
	if err != nil {
		err = fmt.Errorf("[consortium sdk] create proposal falied: %s", err)
		logger.Error(err)
		return nil, "", err
	}
	logger.Debugf("[consortium sdk] create proposal success [txid: %s]", txid)
	return prop, txid, nil
}

func (th *TransactionHandler) CreateProposalWithTxGenerator(chainId string, chaincodeName string, chaincodeVersion string, funcName string, creator []byte, generator []byte, args ...string) (*pb.Proposal, string, error) {
	spec, err := utils.GetChaincodeSpecification(chaincodeName, chaincodeVersion, funcName, args...)
	if err != nil {
		err = fmt.Errorf("[consortium sdk] get chaincode specification falied: %s", err)
		logger.Error(err)
		return nil, "", err
	}

	invocation := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}
	prop, txid, err := utils.CreateChaincodeProposalWithTxIDGeneratorAndTransient(common.HeaderType_ENDORSER_TRANSACTION, chainId, invocation, creator, generator, nil)
	if err != nil {
		err = fmt.Errorf("[consortium sdk] create proposal falied: %s", err)
		logger.Error(err)
		return nil, "", err
	}
	logger.Debugf("[consortium sdk] create proposal success [txid: %s]", txid)
	return prop, txid, nil
}

func (th *TransactionHandler) ProcessProposal(signer *ecdsa.PrivateKey, prop *pb.Proposal) (*pb.ProposalResponse, error) {
	endorserClient, err := th.PeerClient.Endorser()
	if err != nil {
		err = fmt.Errorf("[consortium sdk] get endorser client falied: %s", err)
		logger.Error(err)
		return nil, err
	}

	signedProp, err := utils.GetSignedProposal(prop, signer)
	if err != nil {
		err = fmt.Errorf("[consortium sdk] get signed proposal falied: %s", err)
		logger.Error(err)
		return nil, err
	}
	proposalResp, err := endorserClient.ProcessProposal(context.Background(), signedProp)
	if err != nil {
		err = fmt.Errorf("[consortium sdk] process proposal falied: %s", err)
		logger.Error(err)
		return nil, err
	}
	return proposalResp, nil
}

func (th *TransactionHandler) SendTransaction(prop *pb.Proposal, signer *ecdsa.PrivateKey, creator []byte, proposalResp *pb.ProposalResponse) error {
	if proposalResp == nil {
		err := fmt.Errorf("[consortium sdk] response is nil")
		logger.Error(err)
		return err
	}

	env, err := utils.CreateSignedTx(prop, signer, creator, proposalResp)
	if err != nil {
		err = fmt.Errorf("[consortium sdk] create signed transaction falied: %s", err)
		logger.Error(err)
		return err
	}
	broadcast, err := th.PeerClient.Broadcast()
	if err != nil {
		err = fmt.Errorf("[consortium sdk] get broadcast client falied: %s", err)
		logger.Error(err)
		return err
	}
	if err = broadcast.Send(env); err != nil {
		err = fmt.Errorf("[consortium sdk] broadcast send falied: %s", err)
		logger.Error(err)
		return err
	}
	return nil
}

/*
	c=1 重新发送 mytx
	c=2 mytx 成功
	c=3 所有节点发送成功，整体事件成功
	c=4 回滚
	c=5 回滚成功
*/
func (th *TransactionHandler) RegisterTxId(txid string, c chan int, cid string, cc chan interface{}) {
	th.retracer.RegisterTxId(txid, c)
	th.RegisteredEventByCId[cid] = cc
}

func (th *TransactionHandler) UnregisterTxId(txid string) {
	th.retracer.UnRegisterTxId(txid)
}

func (th *TransactionHandler) UnregisterCId(cid string) {
	delete(th.RegisteredEventByCId, cid)
}

func (th *TransactionHandler) waitUntilEvent(chaincodeID string, LogCToUser string, Transfered string, RollBack string, cTransfer string) {
	go func() {
		logCToUserChan := make(chan ChaincodeEventInfo, 1)
		transferedChan := make(chan ChaincodeEventInfo, 1)
		rollbackChan := make(chan ChaincodeEventInfo, 1)
		th.retracer.RegisterEventName(LogCToUser, logCToUserChan)
		th.retracer.RegisterEventName(Transfered, transferedChan)
		th.retracer.RegisterEventName(RollBack, rollbackChan)

		for {
			select {
			case event := <-logCToUserChan:
				if len(chaincodeID) != 0 && event.ChaincodeID == chaincodeID {
					var logCToUser cevent.LogCToUser
					if err := json.Unmarshal(event.Payload, &logCToUser); err == nil {
						cc, ok := th.RegisteredEventByCId[logCToUser.TxId]
						if ok && event.EventName == LogCToUser {
							logger.Debugf("[consortium sdk] received LogCToUser consortium event: %#v", event)
							cc <- &logCToUser
							continue
						}
					}
				}
			case event := <-transferedChan:
				var logTransfered cevent.LogTransfered
				if err := json.Unmarshal(event.Payload, &logTransfered); err == nil {
					cc, ok := th.RegisteredEventByCId[cTransfer+logTransfered.TxId]
					if !ok {
						continue
					}
					if event.EventName == Transfered {
						cc <- &logTransfered
					} else if event.EventName == RollBack {
						if string(event.Payload) == "helloworld" {
							cc <- 4
						}
					}
				}
			}
		}

		th.retracer.UnRegisterEventName(LogCToUser)
		th.retracer.UnRegisterEventName(Transfered)
		th.retracer.UnRegisterEventName(RollBack)
	}()
	return
}

func (th *TransactionHandler) ListenEvent(userToCChan chan ChaincodeEventInfo, LogUserToC string) {
	th.retracer.RegisterEventName(LogUserToC, userToCChan)
}

func (th *TransactionHandler) HandleUserToCEvent(userToC ChaincodeEventInfo,
	sendRawTransaction func(string, bool, string, ...interface{}) (string, error),
	getTransactionReceipt func(string) (map[string]interface{}, error),
	rollback func(ChaincodeEventInfo, ...interface{}), pw string, orgName string, channelName string, chaincodeName string, version string, queryCTransfer string, cTransfer string, queryCApprove string, cApprove string, agreedCount int) {
	logUserToC := cevent.GetUserToC(userToC.Payload)
	methodName := "CTransfer"
	fTxId := userToC.TxID
	receiver := ethcom.HexToAddress(logUserToC.AccountE)
	value := new(big.Int)
	value.SetString(logUserToC.Value, 10)

	var txHash string
	var err error
	attempt := 0
	for {
		if attempt >= AttemptCount {
			break
		}
		if txHash, err = sendRawTransaction(pw, true, methodName, fTxId, receiver, value); err != nil {
			logger.Errorf("[consortium sdk] send CTransfer public tx failed: %s", err)
			continue
		} else {
			if receipt, err := getTransactionReceipt(txHash); err == nil {
				if receipt["status"].(string) == "0x1" {
					logger.Debug("[consortium sdk] send CTransfer public tx succeed")
					return
				}
			}
		}
		attempt++
	}

	logger.Errorf("[consortium sdk] send CTransfer public tx failed with %d times\n", attempt)
	rollback(userToC, orgName, channelName, chaincodeName, version, queryCTransfer, cTransfer, queryCApprove, cApprove, agreedCount)
}
