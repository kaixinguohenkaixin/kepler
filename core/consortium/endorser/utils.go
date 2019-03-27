package endorser

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/vntchain/kepler/protos/common"
	pb "github.com/vntchain/kepler/protos/peer"
	"github.com/vntchain/kepler/utils"
)

type ChaincodeEventInfo struct {
	ChaincodeID string
	TxID        string
	EventName   string
	Payload     []byte
	ChannelID   string
}

func GetHeight(peerClient *PeerClient, channel string, creator []byte, signer *ecdsa.PrivateKey) (uint64, error) {
	cbi, err := GetChainInfo(peerClient, channel, creator, signer)
	if err != nil {
		err = fmt.Errorf("[consortium sdk] get chain info failed: %s", err)
		logger.Error(err)
		return 0, err
	}
	return cbi.GetHeight(), nil
}

func GetChainInfo(peerClient *PeerClient, channel string, creator []byte, signer *ecdsa.PrivateKey) (*common.BlockchainInfo, error) {
	signedProp, err := BuildGetChainInfoProposal(creator, signer, channel)
	if err != nil {
		err = fmt.Errorf("[consortium sdk] build getChaininfo proposal failed: %s", err)
		logger.Error(err)
		return nil, err
	}
	endorserClient, _ := peerClient.Endorser()
	resp, err := endorserClient.ProcessProposal(context.Background(), signedProp)
	if err != nil {
		err = fmt.Errorf("[consortium sdk] send proposal failed: %s", err)
		logger.Error(err)
		return nil, err
	}
	chainInfo, err := UnmarshalChainInfoFromProposalResponse(resp)
	if err != nil {
		err = fmt.Errorf("[consortium sdk] unmarshal chain info from proposal response failed: %s", err)
		logger.Error(err)
		return nil, err
	}
	return chainInfo, nil
}

func GetBlockByNumber(peerClient *PeerClient, channel string, number uint64, creator []byte, signer *ecdsa.PrivateKey) (*common.Block, error) {
	signedProp, err := BuildGetBlockByNumberProposal(creator, signer, channel, number)
	if err != nil {
		err = fmt.Errorf("[consortium sdk] build getBlockByNumber proposal failed: %s", err)
		logger.Error(err)
		return nil, err
	}
	endorserClient, _ := peerClient.Endorser()
	resp, err := endorserClient.ProcessProposal(context.Background(), signedProp)
	if err != nil {
		err = fmt.Errorf("[consortium sdk] process proposal failed: %s", err)
		logger.Error(err)
		return nil, err
	}
	block, err := UnmarshalBlockFromProposalResponse(resp)
	if err != nil {
		err = fmt.Errorf("[consortium sdk] unmarshal block from proposal response failed: %s", err)
		logger.Error(err)
		return nil, err
	}
	return block, nil
}

func BuildGetBlockByNumberProposal(creator []byte, signer *ecdsa.PrivateKey, channel string, number uint64) (*pb.SignedProposal, error) {
	strNum := strconv.FormatUint(number, 10)
	return buildQsccProposal(creator, signer, "GetBlockByNumber", channel, strNum)
}

func BuildQueryTxProposal(creator []byte, signer *ecdsa.PrivateKey, channel, txID string) (*pb.SignedProposal, error) {
	return buildQsccProposal(creator, signer, "GetTransactionByID", channel, txID)
}

func BuildGetChainInfoProposal(creator []byte, signer *ecdsa.PrivateKey, channel string) (*pb.SignedProposal, error) {
	return buildQsccProposal(creator, signer, "GetChainInfo", channel)
}

func buildQsccProposal(creator []byte, signer *ecdsa.PrivateKey, fcn, channel string, args ...string) (*pb.SignedProposal, error) {
	inputArgs := [][]byte{[]byte(fcn), []byte(channel)}
	for _, arg := range args {
		inputArgs = append(inputArgs, []byte(arg))
	}
	invocation := &pb.ChaincodeInvocationSpec{
		ChaincodeSpec: &pb.ChaincodeSpec{
			Type:        pb.ChaincodeSpec_Type(pb.ChaincodeSpec_Type_value["GOLANG"]),
			ChaincodeId: &pb.ChaincodeID{Name: "qscc"},
			Input:       &pb.ChaincodeInput{Args: inputArgs},
		}}

	prop, _, err := utils.CreateProposalFromCIS(common.HeaderType_ENDORSER_TRANSACTION, "", invocation, creator)
	if err != nil {
		err = fmt.Errorf("[consortium sdk] create proposal failed: %s", err)
		logger.Error(err)
		return nil, err
	}

	signedProp, err := utils.GetSignedProposal(prop, signer)
	if err != nil {
		err = fmt.Errorf("[consortium sdk] create signed proposal failed: %s", err)
		logger.Error(err)
		return nil, err
	}
	return signedProp, nil
}

func UnmarshalBlockFromProposalResponse(response *pb.ProposalResponse) (*common.Block, error) {
	block := &common.Block{}
	err := proto.Unmarshal(response.Response.Payload, block)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func UnmarshalChainInfoFromProposalResponse(response *pb.ProposalResponse) (*common.BlockchainInfo, error) {
	chainInfo := &common.BlockchainInfo{}
	err := proto.Unmarshal(response.Response.Payload, chainInfo)
	if err != nil {
		return nil, err
	}
	return chainInfo, nil
}

// getChainCodeEvents parses block events for chaincode events associated with individual transactions
func getChaincodeEvent(payloadData []byte, channelHeader *common.ChannelHeader) (event *pb.ChaincodeEvent, channelID string, err error) {
	// Chaincode events apply to endorser transaction only
	if common.HeaderType(channelHeader.Type) == common.HeaderType_ENDORSER_TRANSACTION {
		tx, err := utils.GetTransaction(payloadData)
		if err != nil {
			err = fmt.Errorf("[consortium sdk] unmarshal transaction payload failed: %s", err)
			logger.Error(err)
			return nil, "", err
		}
		chaincodeActionPayload, err := utils.GetChaincodeActionPayload(tx.Actions[0].Payload)
		if err != nil {
			err = fmt.Errorf("[consortium sdk] get chaincode action payload failed: %s", err)
			logger.Error(err)
			return nil, "", err
		}
		propRespPayload, err := utils.GetProposalResponsePayload(chaincodeActionPayload.Action.ProposalResponsePayload)
		if err != nil {
			err = fmt.Errorf("[consortium sdk] get proposal response failed: %s", err)
			logger.Error(err)
			return nil, "", err
		}
		caPayload, err := utils.GetChaincodeAction(propRespPayload.Extension)
		if err != nil {
			err = fmt.Errorf("[consortium sdk] get chaincode action failed: %s", err)
			logger.Error(err)
			return nil, "", err
		}
		ccEvent, err := utils.GetChaincodeEvents(caPayload.Events)
		if err != nil {
			err = fmt.Errorf("[consortium sdk] get chaincode event failed: %s", err)
			logger.Error(err)
			return nil, "", err
		}
		if ccEvent != nil {
			return ccEvent, channelHeader.ChannelId, nil
		}

	}
	return nil, "", nil
}
