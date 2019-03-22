package node

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vntchain/kepler/core/consortium/endorser"
	cTxManager "github.com/vntchain/kepler/core/consortium/txmanager"
	"github.com/vntchain/kepler/core/public/sdk"
	pTxManager "github.com/vntchain/kepler/core/public/txmanager"
	pubevent "github.com/vntchain/kepler/event/public"
)

func startCmd() *cobra.Command {
	return nodeStartCmd
}

var nodeStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts the kepler node.",
	Long:  `Starts a node that interacts with the vnt network.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return serve(args)
	},
}

func serve(args []string) error {
	peers := viper.GetStringMap("consortium.peers")
	orderers := viper.GetStringMap("consortium.orderers")
	privateKey := viper.GetString("consortium.privateKey")
	cert := viper.GetString("consortium.cert")
	chaincodeID := viper.GetString("consortium.chaincodeName")
	LogCToUser := viper.GetString("consortium.LogCToUser")
	Transfered := viper.GetString("consortium.Transfered")
	RollBack := viper.GetString("consortium.RollBack")
	cTransfer := viper.GetString("consortium.cTransfer")
	keyDir := viper.GetString("public.keyDir")
	ks := sdk.NewKeyStore(keyDir)
	chainId := viper.GetInt("public.chainId")
	nodes := viper.GetStringMap("public.nodes")
	abiPath := viper.GetString("public.contract.abi")
	contractAddress := viper.GetString("public.contract.address")
	LogUserToC := viper.GetString("consortium.LogUserToC")
	pw := viper.GetString("public.keypass")
	orgName := viper.GetString("consortium.mspId")
	channelName := viper.GetString("consortium.channelName")
	chaincodeName := viper.GetString("consortium.chaincodeName")
	version := viper.GetString("consortium.version")
	queryCTransfer := viper.GetString("consortium.queryCTransfer")
	queryCApprove := viper.GetString("consortium.queryCApprove")
	cApprove := viper.GetString("consortium.cApprove")
	agreedCount := viper.GetInt("consortium.agreedCount")

	errchan := make(chan error)
	consortiumTxManager, err := cTxManager.NewConsortiumTxManager(peers, orderers, privateKey, cert, chaincodeID, LogCToUser, Transfered, RollBack, cTransfer)
	if err != nil {
		errchan <- err
	}

	publicTxManager, err := pTxManager.NewPublicTxManager(ks, keyDir, chainId, nodes, abiPath, contractAddress)

	go serveConsortiumBlockchain(errchan, publicTxManager, consortiumTxManager, chaincodeID, LogCToUser, Transfered, RollBack, cTransfer, LogUserToC, pw, orgName, channelName, chaincodeName, version, queryCTransfer, queryCApprove, cApprove, agreedCount)
	go servePublicBlockchain(publicTxManager, consortiumTxManager)

	<-errchan
	return nil
}

func serveConsortiumBlockchain(errchan chan error, publicTxManager *pTxManager.TxManager, consortiumManager *cTxManager.TxManager, chaincodeID string, LogCToUser string, Transfered string, RollBack string, cTransfer string, LogUserToC string, passwd string, orgName string, channelName string, chaincodeName string, version string, queryCTransfer string, queryCApprove string, cApprove string, agreedCount int) {
	logger.Info("[node] listening on the consortium blockchain")
	txManager := publicTxManager
	th, err := consortiumManager.RandomTxHandler(chaincodeID, LogCToUser, Transfered, RollBack, cTransfer)
	if err != nil {
		errchan <- err
	}

	var userToCChan = make(chan endorser.ChaincodeEventInfo, 1)
	th.ListenEvent(userToCChan, LogUserToC)
	for {
		userToC := <-userToCChan
		logger.Debugf("[node] consortium UserToC: %v", userToC)

		sendRawTransaction := txManager.SendRawTransaction
		nm, err := txManager.PickNodeManager()
		if err != nil {
			err = fmt.Errorf("[node] pick node manager failed: %s", err)
			logger.Error(err)
			return
		}
		getTransactionReceipt := nm.GetTransactionReceipt
		rollback := consortiumManager.RollBack
		th.HandleUserToCEvent(userToC, sendRawTransaction, getTransactionReceipt, rollback, passwd, orgName, channelName, chaincodeName, version, queryCTransfer, cTransfer, queryCApprove, cApprove, agreedCount)
	}
}

func servePublicBlockchain(publicTxManager *pTxManager.TxManager, consortiumManager *cTxManager.TxManager) {
	logger.Info("[node] listening on the public blockchain")

	EventRollback := viper.GetString("public.EventRollback")
	EventUserToC := viper.GetString("public.EventUserToC")
	EventCToUser := viper.GetString("public.EventCToUser")
	logUserToC := viper.GetString("public.LogUserToC")
	orgName := viper.GetString("consortium.mspId")
	channelName := viper.GetString("consortium.channelName")
	chaincodeName := viper.GetString("consortium.chaincodeName")
	version := viper.GetString("consortium.version")
	queryCTransfer := viper.GetString("consortium.queryCTransfer")
	cTransfer := viper.GetString("consortium.cTransfer")
	queryCApprove := viper.GetString("consortium.queryCApprove")
	cApprove := viper.GetString("consortium.cApprove")
	queryCRevert := viper.GetString("consortium.queryCRevert")
	cRevert := viper.GetString("consortium.cRevert")
	agreedCnt := viper.GetInt("consortium.agreedCount")
	chainId := viper.GetInt("public.chainId")
	CRollback := viper.GetString("public.CRollback")
	abiPath := viper.GetString("public.contract.abi")
	passwd := viper.GetString("public.keypass")
	txManager := publicTxManager

	userToCChan := make(chan *pubevent.LogUserToC, 1)
	txManager.ListenEvent(userToCChan, EventRollback, EventUserToC, EventCToUser, logUserToC)

	for {
		userToC := <-userToCChan
		txManager.HandleUserToCEvent(userToC, consortiumManager, orgName, channelName, chaincodeName, version, queryCTransfer, cTransfer, queryCApprove, cApprove, agreedCnt, queryCRevert, cRevert, chainId, CRollback, abiPath, passwd)
	}

}
