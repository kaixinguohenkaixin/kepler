package endorser

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/vntchain/kepler/event/consortium/consumer"
	ab "github.com/vntchain/kepler/protos/orderer"
	pb "github.com/vntchain/kepler/protos/peer"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

var (
	logger         = logging.MustGetLogger("endorser/client")
	maxRecvMsgSize = 100 * 1024 * 1024
	maxSendMsgSize = 100 * 1024 * 1024

	keepaliveOptions = &KeepaliveOptions{
		ClientInterval:    time.Duration(1) * time.Minute,  // 1 min
		ClientTimeout:     time.Duration(20) * time.Second, // 20 sec - gRPC default
		ServerInterval:    time.Duration(2) * time.Hour,    // 2 hours - gRPC default
		ServerTimeout:     time.Duration(20) * time.Second, // 20 sec - gRPC default
		ServerMinInterval: time.Duration(1) * time.Minute,  // match ClientInterval
	}
)

type GRPCClient struct {
	// TLS configuration used by the grpc.ClientConn
	tlsConfig *tls.Config
	// Options for setting up new connections
	dialOpts []grpc.DialOption
	// Duration for which to block while established a new connection
	timeout time.Duration
	// Maximum message size the client can receive
	maxRecvMsgSize int
	// Maximum message size the client can send
	maxSendMsgSize int
}

// ClientConfig defines the parameters for configuring a GRPCClient instance
type ClientConfig struct {
	// SecOpts defines the security parameters
	SecOpts *SecureOptions
	// KaOpts defines the keepalive parameters
	KaOpts *KeepaliveOptions
	// Timeout specifies how long the client will block when attempting to
	// establish a connection
	Timeout time.Duration
}

// SecureOptions defines the security parameters (e.g. TLS) for a
// GRPCServer instance
type SecureOptions struct {
	// PEM-encoded X509 public key to be used for TLS communication
	Certificate []byte
	// PEM-encoded private key to be used for TLS communication
	Key []byte
	// Set of PEM-encoded X509 certificate authorities used by clients to
	// verify server certificates
	ServerRootCAs [][]byte
	// Set of PEM-encoded X509 certificate authorities used by servers to
	// verify client certificates
	ClientRootCAs [][]byte
	// Whether or not to use TLS for communication
	UseTLS bool
	// Whether or not TLS client must present certificates for authentication
	RequireClientCert bool
	// CipherSuites is a list of supported cipher suites for TLS
	CipherSuites []uint16
}

// KeepAliveOptions is used to set the gRPC keepalive settings for both
// clients and servers
type KeepaliveOptions struct {
	// ClientInterval is the duration after which if the client does not see
	// any activity from the server it pings the server to see if it is alive
	ClientInterval time.Duration
	// ClientTimeout is the duration the client waits for a response
	// from the server after sending a ping before closing the connection
	ClientTimeout time.Duration
	// ServerInterval is the duration after which if the server does not see
	// any activity from the client it pings the client to see if it is alive
	ServerInterval time.Duration
	// ServerTimeout is the duration the server waits for a response
	// from the client after sending a ping before closing the connection
	ServerTimeout time.Duration
	// ServerMinInterval is the minimum permitted time between client pings.
	// If clients send pings more frequently, the server will disconnect them
	ServerMinInterval time.Duration
}

// AddPemToCertPool adds PEM-encoded certs to a cert pool
func AddPemToCertPool(pemCerts []byte, pool *x509.CertPool) error {
	certs, _, err := pemToX509Certs(pemCerts)
	if err != nil {
		return err
	}
	for _, cert := range certs {
		pool.AddCert(cert)
	}
	return nil
}

//utility function to parse PEM-encoded certs
func pemToX509Certs(pemCerts []byte) ([]*x509.Certificate, []string, error) {

	//it's possible that multiple certs are encoded
	certs := []*x509.Certificate{}
	subjects := []string{}
	for len(pemCerts) > 0 {
		var block *pem.Block
		block, pemCerts = pem.Decode(pemCerts)
		if block == nil {
			break
		}
		/** TODO: check why msp does not add type to PEM header
		if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			continue
		}
		*/

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, subjects, err
		} else {
			certs = append(certs, cert)
			//extract and append the subject
			subjects = append(subjects, string(cert.RawSubject))
		}
	}
	return certs, subjects, nil
}

func ConfigFromEnv(env map[interface{}]interface{}) (address, override, eventAddress string, clientConfig ClientConfig, err error) {

	address = env["address"].(string)
	override = env["sn"].(string)

	if env["eventAddress"] != nil {
		eventAddress = env["eventAddress"].(string)
	} else {
		eventAddress = ""
	}

	clientConfig = ClientConfig{}
	tls := env["tls"].(map[interface{}]interface{})
	secOpts := &SecureOptions{
		UseTLS:            tls["enabled"].(bool),
		RequireClientCert: tls["clientAuthRequired"].(bool),
	}
	if secOpts.UseTLS {
		caPEM, res := ioutil.ReadFile(tls["rootcertFile"].(string))
		if res != nil {
			err = errors.WithMessage(res,
				fmt.Sprint("unable to load rootcertFile"))
			return
		}
		secOpts.ServerRootCAs = [][]byte{caPEM}
	}
	if secOpts.RequireClientCert {
		keyPEM, res := ioutil.ReadFile(env["clientKeyFile"].(string))
		if res != nil {
			err = errors.WithMessage(res,
				fmt.Sprint("unable to load clientKeyFile"))
			return
		}
		secOpts.Key = keyPEM
		certPEM, res := ioutil.ReadFile(env["clientCertFile"].(string))
		if res != nil {
			err = errors.WithMessage(res,
				fmt.Sprint("unable to load tlsClientCertFile"))
			return
		}
		secOpts.Certificate = certPEM
	}
	clientConfig.SecOpts = secOpts
	return
}

// NewGRPCClient creates a new implementation of GRPCClient given an address
// and client configuration
func NewGRPCClient(config ClientConfig) (*GRPCClient, error) {
	client := &GRPCClient{}

	// parse secure options
	err := client.parseSecureOptions(config.SecOpts)
	if err != nil {
		return client, err
	}

	// keepalive options
	var kap keepalive.ClientParameters
	if config.KaOpts != nil {
		kap = keepalive.ClientParameters{
			Time:    config.KaOpts.ClientInterval,
			Timeout: config.KaOpts.ClientTimeout}
	} else {
		// use defaults
		kap = keepalive.ClientParameters{
			Time:    keepaliveOptions.ClientInterval,
			Timeout: keepaliveOptions.ClientTimeout}
	}
	kap.PermitWithoutStream = true
	// set keepalive and blocking
	client.dialOpts = append(client.dialOpts, grpc.WithKeepaliveParams(kap),
		grpc.WithBlock())
	client.timeout = config.Timeout
	// set send/recv message size to package defaults
	client.maxRecvMsgSize = maxRecvMsgSize
	client.maxSendMsgSize = maxSendMsgSize

	return client, nil
}

func (client *GRPCClient) parseSecureOptions(opts *SecureOptions) error {

	if opts == nil || !opts.UseTLS {
		return nil
	}
	client.tlsConfig = &tls.Config{
		MinVersion: tls.VersionTLS12} // TLS 1.2 only
	if len(opts.ServerRootCAs) > 0 {
		client.tlsConfig.RootCAs = x509.NewCertPool()
		for _, certBytes := range opts.ServerRootCAs {
			err := AddPemToCertPool(certBytes, client.tlsConfig.RootCAs)
			if err != nil {
				logger.Debugf("error adding root certificate: %v", err)
				return errors.WithMessage(err,
					"error adding root certificate")
			}
		}
	}
	if opts.RequireClientCert {
		// make sure we have both Key and Certificate
		if opts.Key != nil &&
			opts.Certificate != nil {
			cert, err := tls.X509KeyPair(opts.Certificate,
				opts.Key)
			if err != nil {
				return errors.WithMessage(err, "failed to "+
					"load client certificate")
			}
			client.tlsConfig.Certificates = append(
				client.tlsConfig.Certificates, cert)
		} else {
			return errors.New("both Key and Certificate " +
				"are required when using mutual TLS")
		}
	}
	return nil
}

// Certificate returns the tls.Certificate used to make TLS connections
// when client certificates are required by the server
func (client *GRPCClient) Certificate() tls.Certificate {
	cert := tls.Certificate{}
	if client.tlsConfig != nil && len(client.tlsConfig.Certificates) > 0 {
		cert = client.tlsConfig.Certificates[0]
	}
	return cert
}

// TLSEnabled is a flag indicating whether to use TLS for client
// connections
func (client *GRPCClient) TLSEnabled() bool {
	return client.tlsConfig != nil
}

// MutualTLSRequired is a flag indicating whether the client
// must send a certificate when making TLS connections
func (client *GRPCClient) MutualTLSRequired() bool {
	return client.tlsConfig != nil &&
		len(client.tlsConfig.Certificates) > 0
}

// SetMaxRecvMsgSize sets the maximum message size the client can receive
func (client *GRPCClient) SetMaxRecvMsgSize(size int) {
	client.maxRecvMsgSize = size
}

// SetMaxSendMsgSize sets the maximum message size the client can send
func (client *GRPCClient) SetMaxSendMsgSize(size int) {
	client.maxSendMsgSize = size
}

// SetServerRootCAs sets the list of authorities used to verify server
// certificates based on a list of PEM-encoded X509 certificate authorities
func (client *GRPCClient) SetServerRootCAs(serverRoots [][]byte) error {

	// NOTE: if no serverRoots are specified, the current cert pool will be
	// replaced with an empty one
	certPool := x509.NewCertPool()
	for _, root := range serverRoots {
		err := AddPemToCertPool(root, certPool)
		if err != nil {
			return errors.WithMessage(err, "error adding root certificate")
		}
	}
	client.tlsConfig.RootCAs = certPool
	return nil
}

// NewConnection returns a grpc.ClientConn for the target address and
// overrides the server name used to verify the hostname on the
// certificate returned by a server when using TLS
func (client *GRPCClient) NewConnection(address string, serverNameOverride string) (
	*grpc.ClientConn, error) {

	var dialOpts []grpc.DialOption
	dialOpts = append(dialOpts, client.dialOpts...)

	// set transport credentials and max send/recv message sizes
	// immediately before creating a connection in order to allow
	// SetServerRootCAs / SetMaxRecvMsgSize / SetMaxSendMsgSize
	//  to take effect on a per connection basis
	if client.tlsConfig != nil {
		client.tlsConfig.ServerName = serverNameOverride
		dialOpts = append(dialOpts,
			grpc.WithTransportCredentials(
				credentials.NewTLS(client.tlsConfig)))
	} else {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}
	dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(client.maxRecvMsgSize),
		grpc.MaxCallSendMsgSize(client.maxSendMsgSize)))

	// ctx, cancel := context.WithTimeout(context.Background(), client.timeout)
	// defer cancel()
	ctx := context.TODO()
	conn, err := grpc.DialContext(ctx, address, dialOpts...)
	if err != nil {
		return nil, errors.WithMessage(errors.WithStack(err),
			"failed to create new connection")
	}
	return conn, nil
}

type PeerClient struct {
	grpcClient      *GRPCClient
	ordererClient   *GRPCClient
	sn              string
	endorserAddress string
	eventAddress    string
	ordererAddress  string
	orderersn       string
}

func NewPeerClient(
	grpcClient *GRPCClient,
	ordererClient *GRPCClient,
	endorserAddress string,
	sn string,
	eventAddress string,
	ordererAddress string,
	orderersn string) *PeerClient {
	return &PeerClient{
		grpcClient:      grpcClient,
		ordererClient:   ordererClient,
		sn:              sn,
		endorserAddress: endorserAddress,
		eventAddress:    eventAddress,
		ordererAddress:  ordererAddress,
		orderersn:       orderersn,
	}
}

func (pc *PeerClient) SetOrdererClient(ordererClient *GRPCClient) {
	pc.ordererClient = ordererClient
}

func (pc *PeerClient) SetEndorserClient(grpcClient *GRPCClient) {
	pc.grpcClient = grpcClient
}

func (pc *PeerClient) Endorser() (pb.EndorserClient, error) {
	conn, err := pc.grpcClient.NewConnection(pc.endorserAddress, pc.sn)
	if err != nil {
		return nil, errors.WithMessage(err, fmt.Sprintf("endorser client failed to connect to %s", pc.endorserAddress))
	}

	return pb.NewEndorserClient(conn), nil
}

func (pc *PeerClient) StartEvent(adapter consumer.EventAdapter) (*consumer.EventsClient, error) {
	conn, err := pc.grpcClient.NewConnection(pc.eventAddress, pc.sn)
	if err != nil {
		return nil, errors.WithMessage(err, fmt.Sprintf("endorser event client failed to connect to %s", pc.eventAddress))
	}

	eventClient, err := consumer.NewEventsClient(pc.eventAddress, 30*time.Second, adapter)
	if err != nil {
		return nil, err
	}
	eventClient.Start(conn)
	return eventClient, nil
}

func (pc *PeerClient) Broadcast() (ab.AtomicBroadcast_BroadcastClient, error) {
	conn, err := pc.ordererClient.NewConnection(pc.ordererAddress, pc.orderersn)
	if err != nil {
		return nil, errors.WithMessage(err, fmt.Sprintf("peer failed to connect to orderer: %s", pc.ordererAddress))
	}

	return ab.NewAtomicBroadcastClient(conn).Broadcast(context.TODO())

}
