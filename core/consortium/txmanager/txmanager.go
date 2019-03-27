package txmanager

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"github.com/op/go-logging"
	"github.com/vntchain/kepler/conf"
	"github.com/vntchain/kepler/core/consortium/endorser"
	cevent "github.com/vntchain/kepler/event/consortium"
	pubevent "github.com/vntchain/kepler/event/public"
	"github.com/vntchain/kepler/utils"
	"math/big"
	"math/rand"
	"strconv"
	"time"
)

var logger = logging.MustGetLogger("consortium_txmanager")

const (
	WaitTime                      = 100 * time.Second
	RandomTransactoinHandlerCount = 1
	MaxResendCount                = 3
)

type EndorserConfig struct {
	Address      string
	SN           string
	EventAddress string
	ClientConfig endorser.ClientConfig
	GrpcClient   *endorser.GRPCClient
}

type OrdererConfig struct {
	OrdererAddress string
	OrdererSN      string
	OrdererConfig  endorser.ClientConfig
	OrdererClient  *endorser.GRPCClient
}

type TxManager struct {
	creator        []byte
	signer         interface{}
	endorserConfig []*EndorserConfig
	ordererConfig  []*OrdererConfig
	ths            []*endorser.TransactionHandler
}

func NewConsortiumTxManager() (tm *TxManager, err error) {
	tm = &TxManager{}
	for _, p := range conf.TheConsortiumConf.Peers {
		clientConfig, err := endorser.ResolveTlsConfig(p.Tls)
		if err != nil {
			err = fmt.Errorf("[consortium txmanager] resolve peer tls config [%#v] failed: %s", p, err)
			logger.Error(err)
			return nil, err
		}
		logger.Debugf("[consortium txmanager] peer [addr: %s, event addr: %s]", p.Address, p.EventAddress)
		grpcClient, err := endorser.NewGRPCClient(clientConfig)
		if err != nil {
			err = fmt.Errorf("[consortium txmanager] create peer grpc client failed: %s", err)
			logger.Error(err)
			return nil, err
		}

		e := &EndorserConfig{
			Address:      p.Address,
			SN:           p.Sn,
			EventAddress: p.EventAddress,
			ClientConfig: clientConfig,
			GrpcClient:   grpcClient,
		}
		tm.endorserConfig = append(tm.endorserConfig, e)
	}

	for _, o := range conf.TheConsortiumConf.Orderers {
		ordererConfig, err := endorser.ResolveTlsConfig(o.Tls)
		if err != nil {
			err = fmt.Errorf("[consortium txmanager] resolve orderer tls config [%#v] failed: %s", o, err)
			logger.Error(err)
			return nil, err
		}
		ordererClient, err := endorser.NewGRPCClient(ordererConfig)
		if err != nil {
			err = fmt.Errorf("[consortium txmanager] create orderer grpc client failed: %s", err)
			logger.Error(err)
			return nil, err
		}

		o := &OrdererConfig{
			OrdererAddress: o.Address,
			OrdererSN:      o.Sn,
			OrdererConfig:  ordererConfig,
			OrdererClient:  ordererClient,
		}
		tm.ordererConfig = append(tm.ordererConfig, o)
	}

	tm.signer, err = utils.ReadPrivateKey(conf.TheConsortiumConf.PrivateKeyPath, []byte{})
	if err != nil {
		err = fmt.Errorf("[consortium txmanager] read private key from path [%s] failed: %s", conf.TheConsortiumConf.PrivateKeyPath, err)
		logger.Error(err)
		return
	}

	cert, err := utils.ReadCertFile(conf.TheConsortiumConf.CertPath)
	if err != nil {
		err = fmt.Errorf("[consortium txmanager] read cert file from path [%s] failed: %s", conf.TheConsortiumConf.CertPath, err)
		logger.Error(err)
		return
	}
	tm.creator, err = utils.SerializeCert(cert)
	if err != nil {
		err = fmt.Errorf("[consortium txmanager] serialize cert failed: %s", err)
		logger.Error(err)
		return
	}

	tm.initTransactionHandlers()
	return
}

func (tm *TxManager) initTransactionHandlers() {
	tm.ths = make([]*endorser.TransactionHandler, 0)
	for i := 0; i < RandomTransactoinHandlerCount; i++ {
		th, err := tm.RandomTxHandler()
		if err != nil {
			logger.Errorf("[consortium txmanager] init random tx handler failed: %s", err)
			continue
		}
		tm.ths = append(tm.ths, th)
	}
}

func (tm *TxManager) RandomTxHandler() (*endorser.TransactionHandler, error) {
	endorserConfig, err := tm.PickEndorserConfig()
	if err != nil {
		err := fmt.Errorf("[consortium txmanager] pick endorser config list failed: %s", err)
		logger.Error(err)
		return nil, err
	}
	ordererConfig, err := tm.PickOrdererConfig()
	if err != nil {
		err := fmt.Errorf("[consortium txmanager] pick orderer config list failed: %s", err)
		logger.Error(err)
		return nil, err
	}

	grpcClient := endorserConfig.GrpcClient
	addr := endorserConfig.Address
	sn := endorserConfig.SN
	eventAddress := endorserConfig.EventAddress
	ordererClient := ordererConfig.OrdererClient
	ordererAddress := ordererConfig.OrdererAddress
	orderersn := ordererConfig.OrdererSN
	peerClient := endorser.NewPeerClient(grpcClient, ordererClient, addr, sn, eventAddress, ordererAddress, orderersn)
	th := &endorser.TransactionHandler{PeerClient: peerClient}
	err = th.Init(tm.GetSigner(), tm.GetCreator())
	if err != nil {
		err = fmt.Errorf("[consortium txmanager] transaction handler init failed: %s", err)
		logger.Error(err)
	}
	return th, err
}

func (tm *TxManager) GetCreator() []byte {
	return tm.creator
}

func (tm *TxManager) GetSigner() interface{} {
	return tm.signer
}

func (tm *TxManager) PickEndorserConfig() (*EndorserConfig, error) {
	if len(tm.endorserConfig) == 0 {
		err := fmt.Errorf("[consortium txmanager] endorser config list is nil")
		logger.Error(err)
		return nil, err
	}
	i := rand.Intn(len(tm.endorserConfig))
	return tm.endorserConfig[i], nil
}

func (tm *TxManager) PickOrdererConfig() (*OrdererConfig, error) {
	if len(tm.ordererConfig) == 0 {
		err := fmt.Errorf("[consortium txmanager] orderer config list is nil")
		logger.Error(err)
		return nil, err
	}
	i := rand.Intn(len(tm.ordererConfig))
	return tm.ordererConfig[i], nil
}

func (tm *TxManager) PickRandomTxHandler() (*endorser.TransactionHandler, error) {
	if len(tm.ths) == 0 {
		err := fmt.Errorf("[consortium txmanager] transaction handler list is nil")
		logger.Error(err)
		return nil, err
	}
	i := rand.Intn(len(tm.ths))
	return tm.ths[i], nil
}

func (tm *TxManager) SendTransaction(th *endorser.TransactionHandler, c chan int, cc chan interface{}, chainId, chaincodeName, version, funcName, cid string, args ...string) (string, string, error) {
	prop, txid, err := th.CreateProposal(chainId, chaincodeName, version, funcName, tm.creator, args...)
	if err != nil {
		err := fmt.Errorf("[consortium txmanager] create proposal failed: %s", err)
		logger.Error(err)
		return txid, cid, err
	}

	proposalResponse, err := th.ProcessProposal(tm.signer.(*ecdsa.PrivateKey), prop)
	if err != nil {
		err := fmt.Errorf("[consortium txmanager] process proposal failed: %s", err)
		logger.Error(err)
		return txid, cid, err
	}
	logger.Debugf("[consortium txmanager] proposal response: %#v", *proposalResponse)

	th.RegisterTxId(txid, c, cid, cc)
	err = th.SendTransaction(prop, tm.signer.(*ecdsa.PrivateKey), tm.creator, proposalResponse)
	if err != nil {
		err := fmt.Errorf("[consortium txmanager] send transaction failed: %s", err)
		logger.Error(err)
		th.UnregisterTxId(txid)
		th.UnregisterCId(cid)
		return txid, cid, err
	}
	return txid, cid, nil
}

func (tm *TxManager) WaitUntilTransactionSuccess(callback func(logCToUser interface{}) bool, query func(q ...interface{}) bool, revert func(q ...interface{}), funcName, cid string, args ...string) {
	th, err := tm.PickRandomTxHandler()
	if err != nil {
		err := fmt.Errorf("[consortium txmanager] pick random transaction handler failed: %s", err)
		logger.Error(err)
		return
	}

	c := make(chan int, 1)
	cc := make(chan interface{}, 1)
	resendCount := 0
	for {
		isBreak := false
		txid, cid, err := tm.SendTransaction(
			th,
			c,
			cc,
			conf.TheConsortiumConf.ChannelName,
			conf.TheConsortiumConf.Chaincode.Name,
			conf.TheConsortiumConf.Chaincode.Version,
			funcName,
			cid,
			args...)
		if err != nil {
			err := fmt.Errorf("[consortium txmanager] send transaction failed: %s", err)
			logger.Error(err)
			revert(th, tm)
			return
		} else {
			timeout := time.NewTimer(WaitTime)
			for {
				isBreakLoop := false
				select {
				case txflag := <-c:
					if txflag == 1 {
						// 交易失败
						isBreakLoop = true
						resendCount += 1
						if resendCount >= MaxResendCount {
							isBreak = true
							revert(th, tm)
						} else {
							logger.Debugf("[consortium txmanager] transaction [%s] failed, resend it later", txid)
						}
					} else if txflag == 2 {
						// 交易成功
						logger.Infof("[consortium txmanager] transaction [%s] succeed", txid)
					}
				case eventflag := <-cc:
					switch eventflag.(type) {
					case *cevent.LogCToUser:
						isSuccess := callback(eventflag)
						if isSuccess {
							isBreakLoop = true
							isBreak = true
							logger.Infof("[consortium txmanager] approve transaction [%s] succeed!", txid)
						}
					case *cevent.LogTransfered:
						isSuccess := callback(eventflag)
						if isSuccess {
							isBreakLoop = true
							isBreak = true
							logger.Infof("[consortium txmanager] transfer transaction [%s] succeed!", txid)
						}
					}
				case <-timeout.C:
					th.UnregisterTxId(txid)
					isBreakLoop = true
					isSuccess := query(th, tm, cid)
					if isSuccess {
						isBreak = true
					}
					break
				}
				if isBreakLoop {
					break
				}
			}
		}
		th.UnregisterTxId(txid)
		if isBreak {
			break
		}
	}
	th.UnregisterCId(cid)

	// 出去前要把cc的消息全部消费完
	for {
		isBreak := false
		select {
		case <-c:
		case <-cc:
		default:
			isBreak = true
		}
		if isBreak {
			break
		}
	}
	logger.Debugf("[consortium txmanager] exit WaitUntilTransactionSuccess")
}

func (tm *TxManager) RollBack(userToC endorser.ChaincodeEventInfo) {
	txid := userToC.TxID
	value := big.NewInt(0)
	logUserToC := cevent.GetUserToC(userToC.Payload)
	value.SetString(logUserToC.Value, 10)
	agreedOrgs := make(map[string]*cevent.LogCToUser)

	cApproveCallback := func(data interface{}) bool {
		logCToUser := data.(*cevent.LogCToUser)
		logger.Debugf("[consortium txmanager] received LogCToUser: %#v", *logCToUser)
		if logCToUser.TxId != txid {
			err := fmt.Errorf("[consortium txmanager] LogCToUser event txid [%s] != [%s]", logCToUser.TxId, txid)
			logger.Error(err)
			return false
		}
		if _, ok := agreedOrgs[logCToUser.AgreedOrg]; !ok {
			agreedOrgs[logCToUser.AgreedOrg] = logCToUser
		}
		if len(agreedOrgs) >= conf.TheConsortiumConf.AgreedCount {
			go tm.WaitUntilTransactionSuccess(
				func(cdata interface{}) bool {
					return true
				},
				func(args ...interface{}) bool {
					if len(args) != 3 {
						err := fmt.Errorf("[consortium txmanager] args length [%d] != [%d]", len(args), 3)
						logger.Error(err)
						return false
					}
					th := args[0].(*endorser.TransactionHandler)
					tm := args[1].(*TxManager)
					txid := args[2].(string)
					prop, txid, err := th.CreateProposal(
						conf.TheConsortiumConf.ChannelName,
						conf.TheConsortiumConf.Chaincode.Name,
						conf.TheConsortiumConf.Chaincode.Version,
						conf.TheConsortiumConf.Chaincode.QueryCTransfer,
						tm.GetCreator(),
						txid)
					if err != nil {
						err = fmt.Errorf("[consortium txmanager] create queryTransfer proposal failed: %s", err)
						logger.Error(err)
						return false
					}
					if isOk, err := strconv.ParseBool(string(prop.Payload)); err != nil {
						err = fmt.Errorf("[consortium txmanager] proposal payload parse failed: %s", err)
						logger.Error(err)
						return false
					} else {
						return isOk
					}
				}, func(args ...interface{}) {
					logger.Errorf("[consortium txmanager] rollback cTransfer failed")
				},
				conf.TheConsortiumConf.Chaincode.CTransfer,
				conf.TheConsortiumConf.Chaincode.CTransfer+txid,
				txid)

			return true
		}
		return false
	}

	cApproveQuery := func(args ...interface{}) bool {
		if len(args) != 3 {
			err := fmt.Errorf("[consortium txmanager] args length [%d] != [%d]", len(args), 3)
			logger.Error(err)
			return false
		}
		th := args[0].(*endorser.TransactionHandler)
		tm := args[1].(*TxManager)
		txid := args[2].(string)

		prop, txid, err := th.CreateProposal(
			conf.TheConsortiumConf.ChannelName,
			conf.TheConsortiumConf.Chaincode.Name,
			conf.TheConsortiumConf.Chaincode.Version,
			conf.TheConsortiumConf.Chaincode.QueryCApprove,
			tm.GetCreator(),
			txid)
		if err != nil {
			err = fmt.Errorf("[consortium txmanager] create queryApprove proposal failed: %s", err)
			logger.Error(err)
			return false
		}
		proposalResponse, err := th.ProcessProposal(tm.GetSigner().(*ecdsa.PrivateKey), prop)
		if proposalResponse.Response != nil {
			var logCToUsers map[string]pubevent.KVAppendValue
			if proposalResponse.Response.Payload != nil {
				err = json.Unmarshal(proposalResponse.Response.Payload, &logCToUsers)
				if err != nil {
					err = fmt.Errorf("[consortium txmanager] queryApprove response unmarshal failed: %s", err)
					logger.Error(err)
					return false
				}
				if _, ok := logCToUsers[conf.TheConsortiumConf.MspId]; ok {
					return true
				}
			}
		}
		return false
	}

	cApproveRevert := func(args ...interface{}) {
		logger.Errorf("[consortium txmanager] rollback cApprove failed")
	}

	go tm.WaitUntilTransactionSuccess(
		cApproveCallback,
		cApproveQuery,
		cApproveRevert,
		conf.TheConsortiumConf.Chaincode.CApprove,
		txid,
		txid,
		fmt.Sprintf("%d", value),
		logUserToC.AccountF,
		conf.TheConsortiumConf.MspId)
}
