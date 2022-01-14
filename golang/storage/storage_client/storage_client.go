package storage_client

import (
	"bytes"
	"context"
	"io"

	"github.com/globulario/services/golang/config/config_client"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/storage/storagepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// storage Client Service
////////////////////////////////////////////////////////////////////////////////
const BufferSize = 1024 * 5 // the chunck size.

type Storage_Client struct {
	cc *grpc.ClientConn
	c  storagepb.StorageServiceClient

	// The id of the service
	id string

	// The mac address of the server
	mac string

	// The name of the service
	name string

	// The client domain
	domain string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

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
func NewStorageService_Client(address string, id string) (*Storage_Client, error) {
	client := new(Storage_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = storagepb.NewStorageServiceClient(client.cc)

	return client, nil
}

// The address where the client can connect.
func (client *Storage_Client) SetAddress(address string) {
	client.address = address
}

func (client *Storage_Client) GetConfiguration(address, id string) (map[string]interface{}, error) {
	client_, err := config_client.NewConfigService_Client(address, "config.ConfigService")
	if err != nil {
		return nil, err
	}
	return client_.GetServiceConfiguration(id)
}

func (client *Storage_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Storage_Client) GetCtx() context.Context {
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
func (client *Storage_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Storage_Client) GetAddress() string {
	return client.address
}

// Return the mac address
func (client *Storage_Client) GetMac() string {
	return client.mac
}

// Return the id of the service instance
func (client *Storage_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Storage_Client) GetName() string {
	return client.name
}

// must be close when no more needed.
func (client *Storage_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Storage_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *Storage_Client) GetPort() int {
	return client.port
}

// Set the client instance sevice id.
func (client *Storage_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Storage_Client) SetName(name string) {
	client.name = name
}

func (client *Storage_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Storage_Client) SetDomain(domain string) {
	client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Storage_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Storage_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Storage_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Storage_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Storage_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Storage_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Storage_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Storage_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

////////////////// Service functionnality //////////////////////

// Stop the service.
func (client *Storage_Client) StopService() {
	client.c.Stop(client.GetCtx(), &storagepb.StopRequest{})
}

func (client *Storage_Client) CreateConnection(id string, name string, connectionType float64) error {

	rqst := &storagepb.CreateConnectionRqst{
		Connection: &storagepb.Connection{
			Id:   id,
			Name: name,
			Type: storagepb.StoreType(connectionType), // Disk store (persistent)
		},
	}

	_, err := client.c.CreateConnection(client.GetCtx(), rqst)

	return err
}

func (client *Storage_Client) OpenConnection(id string, options string) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.OpenRqst{
		Id:      id,
		Options: options,
	}

	_, err := client.c.Open(client.GetCtx(), rqst)

	return err
}

func (client *Storage_Client) SetItem(connectionId string, key string, data []byte) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.SetItemRequest{
		Id:    connectionId,
		Key:   key,
		Value: data,
	}

	_, err := client.c.SetItem(client.GetCtx(), rqst)
	return err
}

func (client *Storage_Client) SetLargeItem(connectionId string, key string, value []byte) error {

	// Open the stream...
	stream, err := client.c.SetLargeItem(client.GetCtx())
	if err != nil {
		return err
	}

	buffer := bytes.NewReader(value)
	if err != nil {
		return err
	}

	for {
		var data [BufferSize]byte
		bytesread, err := buffer.Read(data[0:BufferSize])
		if bytesread > 0 {
			rqst := &storagepb.SetLargeItemRequest{
				Value: data[0:bytesread],
			}
			// send the data to the server.
			err = stream.Send(rqst)
		}

		if err == io.EOF {
			err = nil
			break
		} else if err != nil {
			return err
		}
	}

	_, err = stream.CloseAndRecv()
	if err != nil {
		return err
	}

	return nil
}

func (client *Storage_Client) GetItem(connectionId string, key string) ([]byte, error) {
	// I will execute a simple ldap search here...
	rqst := &storagepb.GetItemRequest{
		Id:  connectionId,
		Key: key,
	}

	stream, err := client.c.GetItem(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	// Here I will create the final array
	var buffer bytes.Buffer
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			// end of stream...
			break
		}
		if err != nil {
			return nil, err
		}

		_, err = buffer.Write(msg.Result)
		if err != nil {
			return nil, err
		}
	}

	return buffer.Bytes(), err

}

func (client *Storage_Client) RemoveItem(connectionId string, key string) error {
	// I will execute a simple ldap search here...
	rqst := &storagepb.RemoveItemRequest{
		Id:  connectionId,
		Key: key,
	}

	_, err := client.c.RemoveItem(client.GetCtx(), rqst)
	return err
}

func (client *Storage_Client) Clear(connectionId string) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.ClearRequest{
		Id: connectionId,
	}

	_, err := client.c.Clear(client.GetCtx(), rqst)
	return err
}

func (client *Storage_Client) Drop(connectionId string) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.DropRequest{
		Id: connectionId,
	}

	_, err := client.c.Drop(client.GetCtx(), rqst)
	return err
}

func (client *Storage_Client) CloseConnection(connectionId string) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.CloseRqst{
		Id: connectionId,
	}

	_, err := client.c.Close(client.GetCtx(), rqst)
	return err
}

func (client *Storage_Client) DeleteConnection(connectionId string) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.DeleteConnectionRqst{
		Id: connectionId,
	}

	_, err := client.c.DeleteConnection(client.GetCtx(), rqst)
	return err
}
