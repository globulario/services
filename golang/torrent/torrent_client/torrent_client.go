package torrent_client

import (
	"context"
	"errors"
	"io"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config/config_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/torrent/torrentpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// Torrent Client Service
////////////////////////////////////////////////////////////////////////////////
type Torrent_Client struct {
	cc *grpc.ClientConn
	c  torrentpb.TorrentServiceClient

	// The id of the service
	id string

	// The name of the service
	name string

	// The client domain
	domain string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

	// The mac address of the server
	mac string

	//  keep the last connection state of the client.
	state string

	// The port
	port int

	// is the connection is secure?
	hasTLS bool

	// Link to client key file
	keyFile string

	// Link to client certificate file.
	certFile string

	// certificate authority file
	caFile string

	// The client context
	ctx context.Context
}

// Create a connection to the service.
func NewTorrentService_Client(address string, id string) (*Torrent_Client, error) {
	client := new(Torrent_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}

	err = client.Reconnect()
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (client *Torrent_Client) Reconnect () error{
	var err error
	
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return  err
	}

	client.c = torrentpb.NewTorrentServiceClient(client.cc)
	return nil
}


// The address where the client can connect.
func (client *Torrent_Client) SetAddress(address string) {
	client.address = address
}

// Return the configuration from the configuration server.
func (client *Torrent_Client) GetConfiguration(address, id string) (map[string]interface{}, error) {
	client_, err := globular_client.GetClient(address, "config.ConfigService", "config_client.NewConfigService_Client")
	if err != nil {
		return nil, err
	}
	return client_.(*config_client.Config_Client).GetServiceConfiguration(id)
}

func (client *Torrent_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Torrent_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}
	token, err := security.GetLocalToken(client.GetMac())
	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": client.domain, "mac": client.GetMac()})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	return client.ctx
}

// Return the domain
func (client *Torrent_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Torrent_Client) GetAddress() string {
	return client.address
}

// Return the id of the service instance
func (client *Torrent_Client) GetId() string {
	return client.id
}

// Return the last know connection state
func (client *Torrent_Client) GetState() string {
	return client.state
}

// Return the name of the service
func (client *Torrent_Client) GetName() string {
	return client.name
}

func (client *Torrent_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Torrent_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Torrent_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *Torrent_Client) GetPort() int {
	return client.port
}

// Set the client instance id.
func (client *Torrent_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Torrent_Client) SetName(name string) {
	client.name = name
}

func (client *Torrent_Client) SetMac(mac string) {
	client.mac = mac
}

func (client *Torrent_Client) SetState(state string) {
	client.state = state
}

// Set the domain.
func (client *Torrent_Client) SetDomain(domain string) {
	client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Torrent_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Torrent_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Torrent_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Torrent_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Torrent_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Torrent_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Torrent_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Torrent_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

////////////////// Api //////////////////////

/**
 * Append a link to list of file to be downloaded... It return the uuid
 * of the torrent.
 */
func (client *Torrent_Client) DowloadTorrent(link, dest string, seed bool ) (error) {

	if !Utility.Exists(dest){
		return  errors.New("dir " + dest + " not exist")
	}

	rqst := new(torrentpb.DownloadTorrentRequest)
	rqst.Dest = dest
	rqst.Link = link
	rqst.Seed = seed

	_, err := client.c.DownloadTorrent(client.GetCtx(), rqst)
	return err
}

/**
 * Return the list of all active torrent on the server.
 */ 
func (client *Torrent_Client) GetTorrentInfos(callback func([]*torrentpb.TorrentInfo)) (error){
	
	rqst := new(torrentpb.GetTorrentInfosRequest)
	
	stream, err := client.c.GetTorrentInfos(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			// end of stream...
			break
		}

		if err != nil {
			return err
		}

		// Callback...
		callback(msg.Infos)
	}

	return nil
}