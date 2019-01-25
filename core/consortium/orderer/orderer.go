package orderer

import (
	"fmt"
	cb "github.com/vntchain/kepler/protos/common"
	ab "github.com/vntchain/kepler/protos/orderer"
	"google.golang.org/grpc"
)

type BroadcastClient struct {
	conn   *grpc.ClientConn
	client ab.AtomicBroadcast_BroadcastClient
}

func (s *BroadcastClient) getAck() error {
	msg, err := s.client.Recv()
	if err != nil {
		return err
	}
	if msg.Status != cb.Status_SUCCESS {
		return fmt.Errorf("Got unexpected status: %v", msg.Status)
	}
	return nil
}

//Send data to orderer
func (s *BroadcastClient) Send(env *cb.Envelope) error {
	if err := s.client.Send(env); err != nil {
		return fmt.Errorf("Could not send :%s)", err)
	}

	err := s.getAck()

	return err
}

func (s *BroadcastClient) Close() error {
	return s.conn.Close()
}

// GetBroadcastClient creates a simple instance of the BroadcastClient interface
func GetBroadcastClient(orderingEndpoint string) (common.BroadcastClient, error) {

	if len(strings.Split(orderingEndpoint, ":")) != 2 {
		return nil, fmt.Errorf("Ordering service endpoint %s is not valid or missing", orderingEndpoint)
	}

	client, err := ab.NewAtomicBroadcastClient(conn).Broadcast(context.TODO())
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("Error connecting to %s due to %s", orderingEndpoint, err)
	}

	return &broadcastClient{conn: conn, client: client}, nil
}
