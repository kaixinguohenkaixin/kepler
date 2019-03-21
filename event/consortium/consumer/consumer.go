package consumer

import (
	"crypto/ecdsa"
	"crypto/x509"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"github.com/vntchain/kepler/protos/peer"
	ehpb "github.com/vntchain/kepler/protos/peer"
	"github.com/vntchain/kepler/utils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var consumerLogger = logging.MustGetLogger("eventhub_consumer")

//EventAdapter is the interface by which a fabric event client registers interested events and
//receives messages from the fabric event Server
type EventAdapter interface {
	GetInterestedEvents() ([]*peer.Interest, error)
	Recv(msg *peer.Event) (bool, error)
	Disconnected(err error)
}

type DefaultAdapter struct {
	Notify chan *peer.Event_Block
}

//GetInterestedEvents implements consumer.EventAdapter interface for registering interested events
func (a *DefaultAdapter) GetInterestedEvents() ([]*peer.Interest, error) {
	return []*peer.Interest{{EventType: peer.EventType_BLOCK}}, nil
}

//Recv implements consumer.EventAdapter interface for receiving events
func (a *DefaultAdapter) Recv(msg *peer.Event) (bool, error) {
	if o, e := msg.Event.(*peer.Event_Block); e {
		a.Notify <- o
		return true, nil
	}
	return false, fmt.Errorf("Receive unkown type event: %v", msg)
}

//Disconnected implements consumer.EventAdapter interface for disconnecting
func (a *DefaultAdapter) Disconnected(err error) {
	consumerLogger.Errorf("adapter disconnected err:%v", err)
}

//EventsClient holds the stream and adapter for consumer to work with
type EventsClient struct {
	sync.RWMutex
	peerAddress string
	regTimeout  time.Duration
	stream      ehpb.Events_ChatClient
	adapter     EventAdapter
}

// RegistrationConfig holds the information to be used when registering for
// events from the eventhub
type RegistrationConfig struct {
	InterestedEvents []*ehpb.Interest
	Timestamp        *timestamp.Timestamp
	TlsCert          *x509.Certificate
}

//NewEventsClient Returns a new grpc.ClientConn to the configured local PEER.
func NewEventsClient(peerAddress string, regTimeout time.Duration, adapter EventAdapter) (*EventsClient, error) {
	var err error
	if regTimeout < 100*time.Millisecond {
		regTimeout = 100 * time.Millisecond
		err = fmt.Errorf("regTimeout >= 0, setting to 100 msec")
	} else if regTimeout > 60*time.Second {
		regTimeout = 60 * time.Second
		err = fmt.Errorf("regTimeout > 60, setting to 60 sec")
	}
	return &EventsClient{sync.RWMutex{}, peerAddress, regTimeout, nil, adapter}, err
}

func (ec *EventsClient) send(emsg *ehpb.Event) error {
	ec.Lock()
	defer ec.Unlock()

	creator, err := getCreator()
	if err != nil {
		return err
	}

	emsg.Creator = creator

	signer, err := utils.ReadPrivateKey(viper.GetString("consortium.privateKey"), []byte{})
	if err != nil {
		consumerLogger.Errorf("read private key from path err %v", err)
		return err
	}

	signedEvt, err := utils.GetSignedEvent(emsg, signer.(*ecdsa.PrivateKey))
	if err != nil {
		consumerLogger.Errorf("could not sign outgoing event,err: %v", err)
		return fmt.Errorf("could not sign outgoing event, err %s", err)
	}

	return ec.stream.Send(signedEvt)
}

// RegisterAsync - registers interest in a event and doesn't wait for a response
func (ec *EventsClient) RegisterAsync(config *RegistrationConfig) error {
	creator, err := getCreator()
	if err != nil {
		return fmt.Errorf("error getting creator from MSP: %s", err)
	}

	emsg := &ehpb.Event{Event: &ehpb.Event_Register{Register: &ehpb.Register{Events: config.InterestedEvents}}, Creator: creator, Timestamp: config.Timestamp}

	if config.TlsCert != nil {
		emsg.TlsCertHash = utils.ComputeSHA256(config.TlsCert.Raw)
	}
	if err = ec.send(emsg); err != nil {
		consumerLogger.Errorf("error on Register send %s\n", err)
	}
	return err
}

// register - registers interest in a event
func (ec *EventsClient) register(config *RegistrationConfig) error {
	var err error
	if err = ec.RegisterAsync(config); err != nil {
		return err
	}

	regChan := make(chan struct{})
	go func() {
		defer close(regChan)
		in, inerr := ec.stream.Recv()
		if inerr != nil {
			err = inerr
			return
		}
		switch in.Event.(type) {
		case *ehpb.Event_Register:
		case nil:
			err = fmt.Errorf("invalid nil object for register")
		default:
			err = fmt.Errorf("invalid registration object")
		}
	}()
	select {
	case <-regChan:
	case <-time.After(ec.regTimeout):
		err = fmt.Errorf("timeout waiting for registration")
	}
	return err
}

// UnregisterAsync - Unregisters interest in a event and doesn't wait for a response
func (ec *EventsClient) UnregisterAsync(ies []*ehpb.Interest) error {
	creator, err := getCreator()
	if err != nil {
		return fmt.Errorf("error getting creator from MSP: %s", err)
	}
	emsg := &ehpb.Event{Event: &ehpb.Event_Unregister{Unregister: &ehpb.Unregister{Events: ies}}, Creator: creator}

	if err = ec.send(emsg); err != nil {
		err = fmt.Errorf("error on unregister send %s\n", err)
	}

	return err
}

// Recv receives next event - use when client has not called Start
func (ec *EventsClient) Recv() (*ehpb.Event, error) {
	in, err := ec.stream.Recv()
	if err == io.EOF {
		// read done.
		if ec.adapter != nil {
			ec.adapter.Disconnected(nil)
		}
		return nil, err
	}
	if err != nil {
		if ec.adapter != nil {
			ec.adapter.Disconnected(err)
		}
		return nil, err
	}
	return in, nil
}
func (ec *EventsClient) processEvents() error {
	defer ec.stream.CloseSend()
	for {
		in, err := ec.stream.Recv()
		if err == io.EOF {
			// read done.
			if ec.adapter != nil {
				ec.adapter.Disconnected(nil)
			}
			return nil
		}
		if err != nil {
			if ec.adapter != nil {
				ec.adapter.Disconnected(err)
			}
			return err
		}
		if ec.adapter != nil {
			cont, err := ec.adapter.Recv(in)
			if !cont {
				return err
			}
		}
	}
}

//Start establishes connection with Event hub and registers interested events with it
func (ec *EventsClient) Start(conn *grpc.ClientConn) error {

	ies, err := ec.adapter.GetInterestedEvents()
	if err != nil {
		return fmt.Errorf("error getting interested events:%s", err)
	}

	if len(ies) == 0 {
		return fmt.Errorf("must supply interested events")
	}

	serverClient := ehpb.NewEventsClient(conn)
	ec.stream, err = serverClient.Chat(context.Background())
	if err != nil {
		return fmt.Errorf("could not create client conn to %s:%s", ec.peerAddress, err)
	}

	regConfig := &RegistrationConfig{InterestedEvents: ies, Timestamp: utils.CreateUtcTimestamp()}
	if err = ec.register(regConfig); err != nil {
		return err
	}

	consumerLogger.Debugf("the start of connection in event")
	go ec.processEvents()

	return nil
}

//Stop terminates connection with event hub
func (ec *EventsClient) Stop() error {
	if ec.stream == nil {
		// in case the steam/chat server has not been established earlier, we assume that it's closed, successfully
		return nil
	}
	return ec.stream.CloseSend()
}

func getCreator() ([]byte, error) {
	cert, err := utils.ReadCertFile(viper.GetString("consortium.cert"))
	if err != nil {
		consumerLogger.Errorf("read cert file from path err %v", err)
		return []byte{}, err
	}

	creator, err := utils.SerializeCert(cert)
	if err != nil {
		consumerLogger.Errorf("serialize the cert err %v", err)
		return []byte{}, err
	}
	return creator, err
}
