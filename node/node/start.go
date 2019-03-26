package node

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/vntchain/kepler/conf"
	"github.com/vntchain/kepler/core/consortium/endorser"
	cTxManager "github.com/vntchain/kepler/core/consortium/txmanager"
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
		return serve()
	},
}

func serve() error {
	errchan := make(chan error)
	consortiumTxManager, err := cTxManager.NewConsortiumTxManager()
	if err != nil {
		err = fmt.Errorf("[node] create consortium tx manager falied: %s", err)
		logger.Error(err)
		errchan <- err
	}

	publicTxManager, err := pTxManager.NewPublicTxManager()
	if err != nil {
		errchan <- err
	}

	go serveConsortiumBlockchain(errchan, publicTxManager, consortiumTxManager)
	go servePublicBlockchain(publicTxManager, consortiumTxManager)

	<-errchan
	return nil
}

func serveConsortiumBlockchain(errchan chan error, publicTxManager *pTxManager.TxManager, consortiumManager *cTxManager.TxManager) {
	logger.Info("[node] listening on the consortium blockchain")
	txManager := publicTxManager
	th, err := consortiumManager.RandomTxHandler()
	if err != nil {
		errchan <- err
	}

	var userToCChan = make(chan endorser.ChaincodeEventInfo, 1)
	th.ListenEvent(userToCChan, conf.TheConsortiumConf.Chaincode.LogUserToC)
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
		th.HandleUserToCEvent(userToC, sendRawTransaction, getTransactionReceipt, rollback)
	}
}

func servePublicBlockchain(publicTxManager *pTxManager.TxManager, consortiumManager *cTxManager.TxManager) {
	logger.Info("[node] listening on the public blockchain")
	userToCChan := make(chan *pubevent.LogUserToC, 1)
	publicTxManager.ListenEvent(userToCChan)

	for {
		userToC := <-userToCChan
		publicTxManager.HandleUserToCEvent(userToC, consortiumManager)
	}

}
