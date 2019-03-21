package endorser

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
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
	AttemptCount = 3
)

type TransactionHandler struct {
	PeerClient           *PeerClient
	RegisteredTxEvent    map[string]chan int
	RegisteredEventByCId map[string]chan interface{}
	retracer             *retracer
}

func (th *TransactionHandler) Init(signer interface{}, creator []byte) error {
	th.RegisteredTxEvent = make(map[string]chan int)
	th.RegisteredEventByCId = make(map[string]chan interface{})

	config := RetraceConf{
		PeerClient:      th.PeerClient,
		Channel:         viper.GetString("consortium.channelName"),
		RetraceInterval: 3 * time.Second,
		Signer:          signer,
		Creator:         creator,
	}

	logger.Errorf("the retracer init and it will process")
	th.retracer = InitRetracer(config)
	go th.retracer.Process()

	return th.waitUntilEvent()
}

func (th *TransactionHandler) CreateProposal(chainId string, chaincodeName string, chaincodeVersion string, funcName string, creator []byte, args ...string) (*pb.Proposal, string, error) {
	spec, err := utils.GetChaincodeSpecification(chaincodeName, chaincodeVersion, funcName, args...)
	if err != nil {
		return nil, "", err
	}

	//打包交易
	invocation := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}

	var prop *pb.Proposal
	prop, txid, err := utils.CreateProposalFromCIS(common.HeaderType_ENDORSER_TRANSACTION, chainId, invocation, creator)

	if err != nil {
		return nil, "", fmt.Errorf("Error creating proposal  %s: %s", funcName, err)
	}
	logger.Debugf("create proposal success txid:%s", txid)

	return prop, txid, nil
}

func (th *TransactionHandler) CreateProposalWithTxGenerator(chainId string, chaincodeName string, chaincodeVersion string, funcName string, creator []byte, generator []byte, args ...string) (*pb.Proposal, string, error) {
	spec, err := utils.GetChaincodeSpecification(chaincodeName, chaincodeVersion, funcName, args...)
	if err != nil {
		return nil, "", err
	}

	//打包交易
	invocation := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}

	var prop *pb.Proposal
	prop, txid, err := utils.CreateChaincodeProposalWithTxIDGeneratorAndTransient(common.HeaderType_ENDORSER_TRANSACTION, chainId, invocation, creator, generator, nil)

	if err != nil {
		return nil, "", fmt.Errorf("Error creating proposal  %s: %s", funcName, err)
	}
	logger.Debugf("create proposal success txid:%s", txid)

	return prop, txid, nil
}

func (th *TransactionHandler) ProcessProposal(signer *ecdsa.PrivateKey, prop *pb.Proposal) (*pb.ProposalResponse, error) {
	endorserClient, err := th.PeerClient.Endorser() //短连接，所以不需要保存
	if err != nil {
		return nil, err

	}

	var signedProp *pb.SignedProposal
	signedProp, err = utils.GetSignedProposal(prop, signer)
	if err != nil {
		return nil, fmt.Errorf("Error creating signed proposal %s", err)
	}

	//返回交易结果
	proposalResp, err := endorserClient.ProcessProposal(context.Background(), signedProp)
	return proposalResp, err
}

func (th *TransactionHandler) SendTransaction(prop *pb.Proposal, signer *ecdsa.PrivateKey, creator []byte, proposalResp *pb.ProposalResponse) error {
	if proposalResp != nil {
		// assemble a signed transaction (it's an Envelope message)
		env, err := utils.CreateSignedTx(prop, signer, creator, proposalResp)
		if err != nil {
			return fmt.Errorf("Could not assemble transaction, err %s", err)
		}

		// send the envelope for ordering
		broadcast, err := th.PeerClient.Broadcast()
		if err != nil {
			return fmt.Errorf("Error of get broadcast handler err %s", err)
		}

		if err = broadcast.Send(env); err != nil {
			return fmt.Errorf("Error sending transaction %s", err)
		}
		logger.Debugf("sendTransaction finish")
		return nil
	}
	return fmt.Errorf("proposeResponse is nil")
}

/*
	c=1 重新发送 mytx
	c=2 mytx 成功
	c=3 所有节点发送成功，整体事件成功
	c=4 回滚
	c=5 回滚成功
*/
func (th *TransactionHandler) RegisterTxId(txid string, c chan int, cid string, cc chan interface{}) {
	// th.RegisteredTxEvent[txid] = c
	th.retracer.RegisterTxId(txid, c)
	th.RegisteredEventByCId[cid] = cc
}

func (th *TransactionHandler) UnregisterTxId(txid string) {
	// delete(th.RegisteredTxEvent,txid)
	th.retracer.UnRegisterTxId(txid)

}

func (th *TransactionHandler) UnregisterCId(cid string) {
	// delete(th.RegisteredTxEvent,txid)
	delete(th.RegisteredEventByCId, cid)
	logger.Debugf("this is unregistertxid ...,cid:%s", cid)
}

func getTxPayload(tdata []byte) (*common.Payload, error) {
	if tdata == nil {
		return nil, fmt.Errorf("Cannot extract payload from nil transaction")
	}

	if env, err := utils.GetEnvelopeFromBlock(tdata); err != nil {
		return nil, fmt.Errorf("Error getting tx from block(%s)", err)
	} else if env != nil {
		// get the payload from the envelope
		payload, err := utils.GetPayload(env)
		if err != nil {
			return nil, fmt.Errorf("Could not extract payload from envelope, err %s", err)
		}
		return payload, nil
	}
	return nil, nil
}

// getChainCodeEvents parses block events for chaincode events associated with individual transactions
func getChainCodeEvents(tdata []byte) (*pb.ChaincodeEvent, error) {
	if tdata == nil {
		return nil, fmt.Errorf("Cannot extract payload from nil transaction")
	}

	if env, err := utils.GetEnvelopeFromBlock(tdata); err != nil {
		return nil, fmt.Errorf("Error getting tx from block(%s)", err)
	} else if env != nil {
		// get the payload from the envelope
		payload, err := utils.GetPayload(env)
		if err != nil {
			return nil, fmt.Errorf("Could not extract payload from envelope, err %s", err)
		}

		chdr, err := utils.UnmarshalChannelHeader(payload.Header.ChannelHeader)
		if err != nil {
			return nil, fmt.Errorf("Could not extract channel header from envelope, err %s", err)
		}

		if common.HeaderType(chdr.Type) == common.HeaderType_ENDORSER_TRANSACTION {
			tx, err := utils.GetTransaction(payload.Data)
			if err != nil {
				return nil, fmt.Errorf("Error unmarshalling transaction payload for block event: %s", err)
			}
			chaincodeActionPayload, err := utils.GetChaincodeActionPayload(tx.Actions[0].Payload)
			if err != nil {
				return nil, fmt.Errorf("Error unmarshalling transaction action payload for block event: %s", err)
			}
			propRespPayload, err := utils.GetProposalResponsePayload(chaincodeActionPayload.Action.ProposalResponsePayload)
			if err != nil {
				return nil, fmt.Errorf("Error unmarshalling proposal response payload for block event: %s", err)
			}
			caPayload, err := utils.GetChaincodeAction(propRespPayload.Extension)
			if err != nil {
				return nil, fmt.Errorf("Error unmarshalling chaincode action for block event: %s", err)
			}
			ccEvent, err := utils.GetChaincodeEvents(caPayload.Events)

			if ccEvent != nil {
				return ccEvent, nil
			}
		}
	}
	return nil, fmt.Errorf("No events found")
}

func (th *TransactionHandler) waitUntilEvent() error {

	logger.Debugf("in waitUntilEvent...")
	go func() {
		chaincodeID := viper.GetString("consortium.chaincodeName")
		logCToUserChan := make(chan ChaincodeEventInfo, 1)
		transferedChan := make(chan ChaincodeEventInfo, 1)
		rollbackChan := make(chan ChaincodeEventInfo, 1)

		LogCToUser := viper.GetString("consortium.LogCToUser")
		Transfered := viper.GetString("consortium.Transfered")
		RollBack := viper.GetString("consortium.RollBack")
		cTransfer := viper.GetString("consortium.cTransfer")

		th.retracer.RegisterEventName(LogCToUser, logCToUserChan)
		th.retracer.RegisterEventName(Transfered, transferedChan)
		th.retracer.RegisterEventName(RollBack, rollbackChan)

		for {
			select {
			case event := <-logCToUserChan:
				if len(chaincodeID) != 0 && event.ChaincodeID == chaincodeID {
					var logCToUser cevent.LogCToUser
					if err := json.Unmarshal(event.Payload, &logCToUser); err == nil {
						logger.Infof("the logCToUser is %v", logCToUser)
						cc, ok := th.RegisteredEventByCId[logCToUser.TxId]
						if ok {
							if event.EventName == LogCToUser {
								//成功发送
								//cc=3 所有节点发送成功，整体事件成功
								logger.Infof("I have received an event from consortium chain %v", event)
								cc <- &logCToUser
								continue
							}
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
							//成功回滚
							//cc=4 回滚
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

	return nil
}

func (th *TransactionHandler) ListenEvent(userToCChan chan ChaincodeEventInfo) {
	LogUserToC := viper.GetString("consortium.LogUserToC")
	th.retracer.RegisterEventName(LogUserToC, userToCChan)
	logger.Debugf("RegisterEventName %s\n", LogUserToC)
}

func (th *TransactionHandler) HandleUserToCEvent(userToC ChaincodeEventInfo,
	sendRawTransaction func(bool, string, ...interface{}) (string, error),
	getTransactionReceipt func(string) (map[string]interface{}, error),
	rollback func(ChaincodeEventInfo)) {

	logger.Debugf("the userToC is %v", userToC)
	logUserToC := cevent.GetUserToC(userToC.Payload)

	//开始向公链发送交易，并监听，若发送失败，则回退该交易
	methodName := "CTransfer"
	fTxId := userToC.TxID
	receiver := ethcom.HexToAddress(logUserToC.AccountE)
	value := new(big.Int)
	value.SetString(logUserToC.Value, 10)

	attempt := 0
	var txHash string
	var err error
	for {
		if attempt >= AttemptCount {
			break
		}
		if txHash, err = sendRawTransaction(true, methodName, fTxId, receiver, value); err != nil {
			logger.Errorf("Failed to send CTransfer to public with %s\n", err.Error())
			continue
		} else {
			//txManager.WaitUntilDeltaConfirmations() // 以太坊需要等几个块以确认交易，VNT则不用
			if receipt, err := getTransactionReceipt(txHash); err == nil {
				if receipt["status"].(string) == "0x1" { // 本节点CTransfer调用成功
					logger.Debugf("Successfully send CTransfer to public.\n")
					// CToUser检查
					return
				}
			}
		}
		attempt += 1
	}
	//FAILURE:
	// 交易失败回滚
	logger.Errorf("Failed to send CTransfer to public with %d times\n", attempt)
	rollback(userToC)
}
