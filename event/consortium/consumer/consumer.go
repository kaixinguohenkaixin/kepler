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

var consumerLogger = logging.MustGetLogger("consortium_event_consumer")

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
	err := fmt.Errorf("[consortium event consumer] receive unkown type event: %v", msg)
	consumerLogger.Error(err)
	return false, err
}

//Disconnected implements consumer.EventAdapter interface for disconnecting
func (a *DefaultAdapter) Disconnected(err error) {
	consumerLogger.Errorf("[consortium event consumer] adapter disconnected: %s", err)
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
		err = fmt.Errorf("[consortium event consumer] regTimeout >= 0, setting to 100 msec")
		consumerLogger.Error(err)
	} else if regTimeout > 60*time.Second {
		regTimeout = 60 * time.Second
		err = fmt.Errorf("[consortium event consumer] regTimeout > 60, setting to 60 sec")
		consumerLogger.Error(err)
	}
	return &EventsClient{sync.RWMutex{}, peerAddress, regTimeout, nil, adapter}, err
}

func (ec *EventsClient) send(emsg *ehpb.Event, privateKey string) error {
	ec.Lock()
	defer ec.Unlock()

	creator, err := getCreator()
	if err != nil {
		return err
	}
	emsg.Creator = creator
	signer, err := utils.ReadPrivateKey(privateKey, []byte{})
	if err != nil {
		err = fmt.Errorf("[consortium event consumer] read private key from path failed: %s", err)
		consumerLogger.Error(err)
		return err
	}

	signedEvt, err := utils.GetSignedEvent(emsg, signer.(*ecdsa.PrivateKey))
	if err != nil {
		err = fmt.Errorf("[consortium event consumer] sign outgoing event failed: %s", err)
		consumerLogger.Error(err)
		return err
	}

	return ec.stream.Send(signedEvt)
}

// RegisterAsync - registers interest in a event and doesn't wait for a response
func (ec *EventsClient) RegisterAsync(config *RegistrationConfig, privateKey string) error {
	creator, err := getCreator()
	if err != nil {
		err = fmt.Errorf("[consortium event consumer] get creator from MSP failed: %s", err)
		consumerLogger.Error(err)
		return err
	}
	emsg := &ehpb.Event{Event: &ehpb.Event_Register{Register: &ehpb.Register{Events: config.InterestedEvents}}, Creator: creator, Timestamp: config.Timestamp}
	if config.TlsCert != nil {
		emsg.TlsCertHash = utils.ComputeSHA256(config.TlsCert.Raw)
	}

	if err = ec.send(emsg, privateKey); err != nil {
		err = fmt.Errorf("[consortium event consumer] send failed: %s", err)
		consumerLogger.Error(err)
	}
	return err
}

// register - registers interest in a event
func (ec *EventsClient) register(config *RegistrationConfig, privateKey string) error {
	var err error
	if err = ec.RegisterAsync(config, privateKey); err != nil {
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
			err = fmt.Errorf("[consortium event consumer] invalid nil object for register")
			consumerLogger.Error(err)
		default:
			err = fmt.Errorf("[consortium event consumer] invalid registration object")
			consumerLogger.Error(err)
		}
	}()
	select {
	case <-regChan:
	case <-time.After(ec.regTimeout):
		err = fmt.Errorf("[consortium event consumer] timeout waiting for registration")
		consumerLogger.Error(err)
	}
	return err
}

// UnregisterAsync - Unregisters interest in a event and doesn't wait for a response
func (ec *EventsClient) UnregisterAsync(ies []*ehpb.Interest, privateKey string) error {
	creator, err := getCreator()
	if err != nil {
		err = fmt.Errorf("[consortium event consumer] gett creator from MSP failed: %s", err)
		consumerLogger.Error(err)
		return err
	}
	emsg := &ehpb.Event{Event: &ehpb.Event_Unregister{Unregister: &ehpb.Unregister{Events: ies}}, Creator: creator}

	if err = ec.send(emsg, privateKey); err != nil {
		err = fmt.Errorf("[consortium event consumer] unregister send failed: %s\n", err)
		consumerLogger.Error(err)
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
func (ec *EventsClient) Start(conn *grpc.ClientConn, privateKey string) error {
	ies, err := ec.adapter.GetInterestedEvents()
	if err != nil {
		err = fmt.Errorf("[consortium event consumer] getting interested events failed: %s", err)
		consumerLogger.Error(err)
		return err
	}

	if len(ies) == 0 {
		err = fmt.Errorf("[consortium event consumer] must supply interested events")
		consumerLogger.Error(err)
		return err
	}

	serverClient := ehpb.NewEventsClient(conn)
	ec.stream, err = serverClient.Chat(context.Background())
	if err != nil {
		err = fmt.Errorf("[consortium event consumer] create client conn to %s failed: %s", ec.peerAddress, err)
		consumerLogger.Error(err)
		return err
	}

	regConfig := &RegistrationConfig{InterestedEvents: ies, Timestamp: utils.CreateUtcTimestamp()}
	if err = ec.register(regConfig, privateKey); err != nil {
		return err
	}

	consumerLogger.Debugf("[consortium event consumer] the start of connection in event")
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
		err = fmt.Errorf("[consortium event consumer] read cert file from path failed: %s", err)
		consumerLogger.Error(err)
		return []byte{}, err
	}

	creator, err := utils.SerializeCert(cert)
	if err != nil {
		err = fmt.Errorf("[consortium event consumer] serialize the cert failed: %s", err)
		consumerLogger.Error(err)
		return []byte{}, err
	}
	return creator, err
}
