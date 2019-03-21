package event

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/vntchain/kepler/core/consortium/endorser"
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

func GetHeight(peerClient *endorser.PeerClient, channel string, creator []byte, signer *ecdsa.PrivateKey) (uint64, error) {
	var height uint64

	cbi, err := GetChainInfo(peerClient, channel, creator, signer)
	if err != nil {
		logger.Errorf("Failed to GetChainInfo with %s.", err.Error())
		return height, err
	}
	logger.Debugf("Current height of %s: %d.\n", channel, cbi.GetHeight())
	return cbi.GetHeight(), nil
}

func GetChainInfo(peerClient *endorser.PeerClient, channel string, creator []byte, signer *ecdsa.PrivateKey) (*common.BlockchainInfo, error) {
	signedProp, err := BuildGetChainInfoProposal(creator, signer, channel)
	if err != nil {
		return nil, fmt.Errorf("[GetChainInfo] Build getChaininfo proposal error: %s", err)
	}
	endorserClient, _ := peerClient.Endorser()
	resp, err := endorserClient.ProcessProposal(context.Background(), signedProp)
	if err != nil {
		return nil, fmt.Errorf("[GetChainInfo] Send proposal error: %s", err)
	}
	chainInfo, err := UnmarshalChainInfoFromProposalResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("[GetChainInfo] Unmarshal chain info from proposal response error: %s", err)
	}
	return chainInfo, nil
}

func GetBlockByNumber(peerClient *endorser.PeerClient, channel string, number uint64, creator []byte, signer *ecdsa.PrivateKey) (*common.Block, error) {
	signedProp, err := BuildGetBlockByNumberProposal(creator, signer, channel, number)
	if err != nil {
		return nil, fmt.Errorf("[GetBlockByNumber] Build getBlockByNumber proposal error: %s", err)
	}
	endorserClient, _ := peerClient.Endorser()
	resp, err := endorserClient.ProcessProposal(context.Background(), signedProp)
	if err != nil {
		return nil, fmt.Errorf("[GetBlockByNumber] Send proposal error: %s", err)
	}
	block, err := UnmarshalBlockFromProposalResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("[GetBlockByNumber] Unmarshal block from proposal response error: %s", err)
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
		logger.Errorf("Error creating qscc proposal : %s", err)
		return nil, err
	}

	signedProp, err := utils.GetSignedProposal(prop, signer)
	if err != nil {
		return nil, fmt.Errorf("Error creating signed proposal %s", err)
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
			return nil, "", fmt.Errorf("unmarshal transaction payload: %s", err)
		}
		chaincodeActionPayload, err := utils.GetChaincodeActionPayload(tx.Actions[0].Payload)
		if err != nil {
			return nil, "", fmt.Errorf("chaincode action payload retrieval failed: %s", err)
		}
		propRespPayload, err := utils.GetProposalResponsePayload(chaincodeActionPayload.Action.ProposalResponsePayload)
		if err != nil {
			return nil, "", fmt.Errorf("proposal response payload retrieval failed: %s", err)
		}
		caPayload, err := utils.GetChaincodeAction(propRespPayload.Extension)
		if err != nil {
			return nil, "", fmt.Errorf("chaincode action retrieval failed: %s", err)
		}
		ccEvent, err := utils.GetChaincodeEvents(caPayload.Events)

		if ccEvent != nil {
			return ccEvent, channelHeader.ChannelId, nil
		}

	}
	return nil, "", nil
}
