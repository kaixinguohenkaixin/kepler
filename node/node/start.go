package node

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	cTxManager "github.com/vntchain/kepler/core/consortium/txmanager"
	"github.com/vntchain/kepler/core/public/sdk"
	pTxManager "github.com/vntchain/kepler/core/public/txmanager"
	pubevent "github.com/vntchain/kepler/event/public"
	"github.com/vntchain/kepler/core/consortium/endorser"
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
	publicTxManager := &pTxManager.TxManager{}
	go serveConsortiumBlockchain(args, errchan, publicTxManager, consortiumTxManager)
	go servePublicBlockchain(args, errchan, publicTxManager, consortiumTxManager)

	<-errchan
	return nil
}

func serveConsortiumBlockchain(args []string, errchan chan error, publicTxManager *pTxManager.TxManager, consortiumManager *cTxManager.TxManager) {
	logger.Debug("listening the consortium blockchain")

	keyDir := viper.GetString("public.keyDir")

	ks, err := sdk.NewKeyStore(keyDir)
	if err != nil {
		logger.Errorf("Failed to NewKeyStore from %s with %s", keyDir, err.Error())
	}

	txManager := publicTxManager
	err = txManager.Init(ks)

	if err != nil {
		errchan <- err
	}

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

	keyDir := viper.GetString("public.keyDir")

	ks, err := sdk.NewKeyStore(keyDir)
	if err != nil {
		logger.Errorf("Failed to NewKeyStore from %s with %s", keyDir, err.Error())
	}

	txManager := publicTxManager
	err = txManager.Init(ks)

	if err != nil {
		logger.Errorf("Failed to init the publicTxManager err(%v)", err)
		errchan <- err
	}

	userToCChan := make(chan *pubevent.LogUserToC, 1)

	txManager.ListenEvent(userToCChan)

	for {
		userToC := <-userToCChan

		txManager.HandleUserToCEvent(userToC, consortiumManager)
	}

}
