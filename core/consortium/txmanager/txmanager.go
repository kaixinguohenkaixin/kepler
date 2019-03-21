package txmanager

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"github.com/vntchain/kepler/core/consortium/endorser"
	"github.com/vntchain/kepler/core/consortium/event"
	cevent "github.com/vntchain/kepler/event/consortium"
	pubevent "github.com/vntchain/kepler/event/public"
	"github.com/vntchain/kepler/utils"
	"math/big"
	"math/rand"
	"strconv"
	"time"
)

var logger = logging.MustGetLogger("ctxmanager")

const (
	// WaitTime = 10 * 60 * time.Second
	WaitTime                      = 100 * time.Second
	RandomSendTime                = 10
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
	creator []byte
	signer  interface{}
	// tx *endorser.TransactionHandler
	endorserConfig []*EndorserConfig
	ordererConfig  []*OrdererConfig
	ths            []*endorser.TransactionHandler
}

func NewConsortiumNode() (tm *TxManager, err error) {
	tm = &TxManager{}
	peers := viper.GetStringMap("consortium.peers")
	orderers := viper.GetStringMap("consortium.orderers")
	logger.Debugf("read the peers is %v", peers)
	for _, value := range peers {
		addr, sn, eventAddress, clientConfig, err := endorser.ConfigFromEnv(value.(map[interface{}]interface{}))
		logger.Errorf("read the addr:%s  eventAddress:%s sn:%s", addr, eventAddress, sn)
		if err != nil {
			logger.Errorf("read yaml config file err(%v)", err)
			return nil, err
		}

		grpcClient, err := endorser.NewGRPCClient(clientConfig)
		if err != nil {
			logger.Errorf("start connect to consortium blockchain err(%v)", err)
			return nil, err
		}

		e := &EndorserConfig{
			Address:      addr,
			SN:           sn,
			EventAddress: eventAddress,
			ClientConfig: clientConfig,
			GrpcClient:   grpcClient,
		}

		tm.endorserConfig = append(tm.endorserConfig, e)
	}

	for _, value := range orderers {

		ordererAddress, orderersn, _, ordererConfig, err := endorser.ConfigFromEnv(value.(map[interface{}]interface{}))
		if err != nil {
			logger.Errorf("read yaml config file err(%v)", err)
			return nil, err
		}

		ordererClient, err := endorser.NewGRPCClient(ordererConfig)
		if err != nil {
			logger.Errorf("start connect to consortium blockchain err(%v)", err)
			return nil, err
		}

		o := &OrdererConfig{
			OrdererAddress: ordererAddress,
			OrdererSN:      orderersn,
			OrdererConfig:  ordererConfig,
			OrdererClient:  ordererClient,
		}

		tm.ordererConfig = append(tm.ordererConfig, o)
	}

	logger.Debugf("the private key is %s", viper.GetString("consortium.privateKey"))
	tm.signer, err = utils.ReadPrivateKey(viper.GetString("consortium.privateKey"), []byte{})
	if err != nil {
		logger.Errorf("read private key from path err %v", err)
		return nil, err
	}

	cert, err := utils.ReadCertFile(viper.GetString("consortium.cert"))
	if err != nil {
		logger.Errorf("read cert file from path err %v", err)
		return nil, err
	}

	tm.creator, err = utils.SerializeCert(cert)
	if err != nil {
		logger.Errorf("serialize the cert err %v", err)
		return nil, err
	}
	tm.initTransactionHandlers()
	return tm, nil

}

func (tm *TxManager) initTransactionHandlers() {
	tm.ths = make([]*endorser.TransactionHandler, 0)
	for i := 0; i < RandomTransactoinHandlerCount; i++ {
		th, err := tm.RandomTxHandler()
		if err != nil {
			logger.Errorf("init random tx handler err:%v", err)
			continue
		}
		tm.ths = append(tm.ths, th)
	}
}

func (tm *TxManager) GetCreator() []byte {
	return tm.creator
}

func (tm *TxManager) GetSigner() interface{} {
	return tm.signer
}

func (tm *TxManager) PickEndorserConfig() (*EndorserConfig, error) {
	if len(tm.endorserConfig) == 0 {
		return nil, fmt.Errorf("the endorser config is empty")
	}
	i := rand.Intn(len(tm.endorserConfig))
	return tm.endorserConfig[i], nil
}

func (tm *TxManager) PickOrdererConfig() (*OrdererConfig, error) {
	if len(tm.ordererConfig) == 0 {
		return nil, fmt.Errorf("the orderer config is empty")
	}
	i := rand.Intn(len(tm.ordererConfig))
	return tm.ordererConfig[i], nil
}

//下次改成初始化一批，然后从中选取，而不是这样每次都new
func (tm *TxManager) RandomTxHandler() (*endorser.TransactionHandler, error) {

	endorserConfig, err := tm.PickEndorserConfig()
	if err != nil {
		return nil, err
	}

	ordererConfig, err := tm.PickOrdererConfig()
	if err != nil {
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
	return th, err

}

func (tm *TxManager) PickRandomTxHandler() (*endorser.TransactionHandler, error) {
	if len(tm.ths) == 0 {
		return nil, fmt.Errorf("the transaction handlers is empty")
	}
	i := rand.Intn(len(tm.ths))
	return tm.ths[i], nil
}

func (tm *TxManager) SendTransaction(th *endorser.TransactionHandler, c chan int, cc chan interface{}, chainId, chaincodeName, version, funcName, cid string, args ...string) (string, string, error) {

	prop, txid, err := th.CreateProposal(chainId, chaincodeName, version, funcName, tm.creator, args...)
	if err != nil {
		logger.Errorf("create proposal err %v", err)
		return txid, cid, err
	}

	proposalResponse, err := th.ProcessProposal(tm.signer.(*ecdsa.PrivateKey), prop)
	logger.Debugf("the proposal response is result:%s err:%v", proposalResponse, err)
	if err != nil {
		logger.Errorf("start new peerClient instance err(%v)", err)
		if proposalResponse == nil {
			// 发送proposal失败
		}
		return txid, cid, err
	}

	th.RegisterTxId(txid, c, cid, cc)

	err = th.SendTransaction(prop, tm.signer.(*ecdsa.PrivateKey), tm.creator, proposalResponse)
	if err != nil {
		logger.Errorf("send transaction to orderer err:%v", err)
		th.UnregisterTxId(txid)
		th.UnregisterCId(cid)
		return txid, cid, err
	}

	return txid, cid, nil
}

func (tm *TxManager) WaitUntilTransactionSuccess(callback func(logCToUser interface{}) bool, query func(q ...interface{}) bool, revert func(q ...interface{}), chainId, chaincodeName, version, funcName, cid string, args ...string) {
	c := make(chan int, 1)
	cc := make(chan interface{}, 1)

	th, err := tm.PickRandomTxHandler()
	if err != nil {
		// time.Sleep(time.Second)
		// continue
		logger.Errorf("transaction handler random select error!!!!!!!!!!!!!!!")
	}
	resendCount := 0

	for {

		isBreak := false

		//随机等一段时间 否则会出现大量的对一个账户操作，导致交易重合并失败
		// randomTime := rand.Intn(RandomSendTime)
		// time.Sleep(time.Duration(randomTime)*time.Second)
		txid, cid, err := tm.SendTransaction(th, c, cc, chainId, chaincodeName, version, funcName, cid, args...)
		if err != nil {
			logger.Errorf("send transaction to consortium chain err:(%s),will revert", err)
			revert(th, tm)
			return
		} else {
			timeout := time.NewTimer(WaitTime)

			for {
				isBreakLoop := false
				select {
				case txflag := <-c:
					logger.Debugf("receive the txflag is %v", txflag)
					if txflag == 1 {
						//重发交易
						logger.Debugf("transaction %s is failed and later will resend it", txid)
						isBreakLoop = true
						resendCount += 1
						if resendCount >= MaxResendCount {
							isBreak = true
							revert(th, tm)
						}
						// break
					} else if txflag == 2 {
						//交易成功
						logger.Debugf("transaction %s is successed", txid)
					}
				case eventflag := <-cc:
					switch eventflag.(type) {
					case *cevent.LogCToUser:
						//整个事件发送成功，某节点确认了
						isSuccess := callback(eventflag)
						if isSuccess {
							isBreakLoop = true
							isBreak = true
							logger.Infof("transaction:%s is approved success!!!", txid)
						}
					case *cevent.LogTransfered:
						//整个事件成功
						isSuccess := callback(eventflag)
						if isSuccess {
							isBreakLoop = true
							isBreak = true
							logger.Infof("transaction:%s is transfered success!!!", txid)
						}
					}
				case <-timeout.C:
					th.UnregisterTxId(txid)
					isBreakLoop = true
					//查询一下该交易是否已经完全成功，若成功则直接退出
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
	//出去前要把cc的消息全部消费完
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
	logger.Debugf("exit WaitUntilTransactionSuccess !!!!")
}

func (tm *TxManager) RollBack(userToC endorser.ChaincodeEventInfo) {
	// go tm.WaitUntilTransactionSuccess(chainId,chaincodeName,version,funcName,cid,args...)
	orgName := viper.GetString("consortium.mspId")
	txid := userToC.TxID
	value := big.NewInt(0)
	logUserToC := cevent.GetUserToC(userToC.Payload)
	value.SetString(logUserToC.Value, 10)
	agreedOrgs := make(map[string]*cevent.LogCToUser)
	channelName := viper.GetString("consortium.channelName")
	chaincodeName := viper.GetString("consortium.chaincodeName")
	version := viper.GetString("consortium.version")
	queryCTransfer := viper.GetString("consortium.queryCTransfer")
	cTransfer := viper.GetString("consortium.cTransfer")
	queryCApprove := viper.GetString("consortium.queryCApprove")
	cApprove := viper.GetString("consortium.cApprove")

	cApproveCallback := func(data interface{}) bool {
		logCToUser := data.(*cevent.LogCToUser)
		logger.Debugf("the callback received an agreed org %v", logCToUser)
		if logCToUser.TxId != txid {
			return false
		}
		if _, ok := agreedOrgs[logCToUser.AgreedOrg]; !ok {
			agreedOrgs[logCToUser.AgreedOrg] = logCToUser
		}
		logger.Debugf("the agreed orgs length is %d", len(agreedOrgs))
		if len(agreedOrgs) >= viper.GetInt("consortium.agreedCount") {
			//发送cTransfer交易，并监听
			go tm.WaitUntilTransactionSuccess(
				func(cdata interface{}) bool {
					return true
				},
				func(args ...interface{}) bool {
					if len(args) != 3 {
						return false
					}
					th := args[0].(*endorser.TransactionHandler)
					tm := args[1].(*TxManager)
					txid := args[2].(string)

					prop, txid, err := th.CreateProposal(channelName, chaincodeName, version, queryCTransfer, tm.GetCreator(), txid)
					if err != nil {
						logger.Errorf("create proposal in query err %v", err)
						return false
					}
					if isOk, err := strconv.ParseBool(string(prop.Payload)); err != nil {
						return false
					} else {
						return isOk
					}
				}, func(args ...interface{}) {
					// rollback in rollback, do nothing
					logger.Errorf("Rollback cTransfer failed! Do nothing!\n\n")
				}, channelName, chaincodeName, version, cTransfer, cTransfer+txid, txid)

			return true
		}
		return false
	}

	cApproveQuery := func(args ...interface{}) bool {
		if len(args) != 3 {
			return false
		}
		th := args[0].(*endorser.TransactionHandler)
		tm := args[1].(*TxManager)
		txid := args[2].(string)

		prop, txid, err := th.CreateProposal(channelName, chaincodeName, version, queryCApprove, tm.GetCreator(), txid)
		if err != nil {
			logger.Errorf("create proposal in query err %v", err)
			return false
		}

		proposalResponse, err := th.ProcessProposal(tm.GetSigner().(*ecdsa.PrivateKey), prop)
		logger.Infof("the proposal response is result:%s err:%v", proposalResponse, err)
		if proposalResponse.Response != nil {
			var logCToUsers map[string]pubevent.KVAppendValue
			if proposalResponse.Response.Payload != nil {
				err = json.Unmarshal(proposalResponse.Response.Payload, &logCToUsers)
				if err != nil {
					logger.Errorf("the query result is err:%v", err)
					return false
				}
				if _, ok := logCToUsers[orgName]; ok {
					return true
				}
			}

		}
		return false
	}

	cApproveRevert := func(args ...interface{}) {
		logger.Errorf("Rollback cApprove failed! Do nothing!\n")
	}

	go tm.WaitUntilTransactionSuccess(cApproveCallback, cApproveQuery, cApproveRevert,
		channelName, chaincodeName, version, cApprove, txid, txid, fmt.Sprintf("%d", value), logUserToC.AccountF, orgName)
}

func (tm *TxManager) GetTransactionByID(chainId, txid string) (string, error) {
	th, err := tm.RandomTxHandler()
	if err != nil {
		return "", err
	}

	generator := []byte("public chain txid")
	generator = append(generator, ([]byte("public tx public key ...."))...)

	prop, _, err := th.CreateProposalWithTxGenerator("", "qscc", "", "GetTransactionByID", tm.creator, generator, chainId, txid)
	if err != nil {
		logger.Errorf("create proposal err %v", err)
		return "", err
	}

	proposalResponse, err := th.ProcessProposal(tm.signer.(*ecdsa.PrivateKey), prop)
	logger.Infof("proposal response is result:%s err:%v", proposalResponse, err)
	if err != nil {
		return "", err
	}
	status, err := utils.UnmarshalValidateCodeFromProposalResponse(proposalResponse)

	return status, err

}

func (tm *TxManager) RetraceEvent(log chan event.ChaincodeEventInfo, config event.RetraceConf) {
	config.Signer = tm.signer
	config.Creator = tm.creator
	retracer := event.InitRetracer(config)
	go retracer.Process(log)
}
