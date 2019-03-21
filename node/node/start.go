package node

import (
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
	errchan := make(chan error)
	consortiumTxManager, err := cTxManager.NewConsortiumNode()
	if err != nil {
		errchan <- err
	}

	keyDir := viper.GetString("public.keyDir")
	ks := sdk.NewKeyStore(keyDir)
	chainId := viper.GetInt("public.chainId")
	nodes := viper.GetStringMap("public.nodes")
	abiPath := viper.GetString("public.contract.abi")
	contractAddress := viper.GetString("public.contract.address")
	publicTxManager, err := pTxManager.NewPublicTxManager(ks, keyDir, chainId, nodes, abiPath, contractAddress)

	go serveConsortiumBlockchain(args, errchan, publicTxManager, consortiumTxManager)
	go servePublicBlockchain(args, errchan, publicTxManager, consortiumTxManager)

	<-errchan
	return nil
}

func serveConsortiumBlockchain(args []string, errchan chan error, publicTxManager *pTxManager.TxManager, consortiumManager *cTxManager.TxManager) {
	logger.Debug("listening the consortium blockchain")

	txManager := publicTxManager

	th, err := consortiumManager.RandomTxHandler()
	if err != nil {
		errchan <- err
	}
	var userToCChan = make(chan endorser.ChaincodeEventInfo, 1)
	th.ListenEvent(userToCChan)
	for {
		userToC := <-userToCChan
		logger.Debugf("Consortium the userToC is %v", userToC)

		sendRawTransaction := txManager.SendRawTransaction
		getTransactionReceipt := txManager.RandomNode().GetTransactionReceipt
		rollback := consortiumManager.RollBack
		th.HandleUserToCEvent(userToC, sendRawTransaction, getTransactionReceipt, rollback)
	}
}

func servePublicBlockchain(args []string, errchan chan error, publicTxManager *pTxManager.TxManager, consortiumManager *cTxManager.TxManager) {
	logger.Debug("listening the public blockchain")

	txManager := publicTxManager

	userToCChan := make(chan *pubevent.LogUserToC, 1)

	EventRollback := viper.GetString("public.EventRollback")
	EventUserToC := viper.GetString("public.EventUserToC")
	EventCToUser := viper.GetString("public.EventCToUser")
	logUserToC := viper.GetString("public.LogUserToC")
	txManager.ListenEvent(userToCChan, EventRollback, EventUserToC, EventCToUser, logUserToC)


	orgName := viper.GetString("consortium.mspId")
	channelName := viper.GetString("consortium.channelName")
	chaincodeName := viper.GetString("consortium.chaincodeName")
	version := viper.GetString("consortium.version")
	queryCTransfer := viper.GetString("consortium.queryCTransfer")
	cTransfer := viper.GetString("consortium.cTransfer")
	queryCApprove := viper.GetString("consortium.queryCApprove")
	cApprove := viper.GetString("consortium.cApprove")
	for {
		userToC := <-userToCChan

		txManager.HandleUserToCEvent(userToC, consortiumManager, orgName, channelName, chaincodeName, version, queryCTransfer, cTransfer, queryCApprove, cApprove)
	}

}
