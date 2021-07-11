package storage_client

import (
	"bytes"
	"context"
	"io"
	"strconv"

	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/storage/storagepb"
	"google.golang.org/grpc"
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

func (storage_client *Storage_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(storage_client)
	}
	return globular.InvokeClientRequest(storage_client.c, ctx, method, rqst)
}

// Return the domain
func (storage_client *Storage_Client) GetDomain() string {
	return storage_client.domain
}

// Return the address
func (storage_client *Storage_Client) GetAddress() string {
	return storage_client.domain + ":" + strconv.Itoa(storage_client.port)
}

// Return the mac address
func (storage_client *Storage_Client) GetMac() string {
	return storage_client.mac
}

// Return the id of the service instance
func (storage_client *Storage_Client) GetId() string {
	return storage_client.id
}

// Return the name of the service
func (storage_client *Storage_Client) GetName() string {
	return storage_client.name
}

// must be close when no more needed.
func (storage_client *Storage_Client) Close() {
	storage_client.cc.Close()
}

// Set grpc_service port.
func (storage_client *Storage_Client) SetPort(port int) {
	storage_client.port = port
}

// Set the client instance sevice id.
func (storage_client *Storage_Client) SetId(id string) {
	storage_client.id = id
}

// Set the client name.
func (storage_client *Storage_Client) SetName(name string) {
	storage_client.name = name
}

func (storage_client *Storage_Client) SetMac(mac string) {
	storage_client.mac = mac
}


// Set the domain.
func (storage_client *Storage_Client) SetDomain(domain string) {
	storage_client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (storage_client *Storage_Client) HasTLS() bool {
	return storage_client.hasTLS
}

// Get the TLS certificate file path
func (storage_client *Storage_Client) GetCertFile() string {
	return storage_client.certFile
}

// Get the TLS key file path
func (storage_client *Storage_Client) GetKeyFile() string {
	return storage_client.keyFile
}

// Get the TLS key file path
func (storage_client *Storage_Client) GetCaFile() string {
	return storage_client.caFile
}

// Set the client is a secure client.
func (storage_client *Storage_Client) SetTLS(hasTls bool) {
	storage_client.hasTLS = hasTls
}

// Set TLS certificate file path
func (storage_client *Storage_Client) SetCertFile(certFile string) {
	storage_client.certFile = certFile
}

// Set TLS key file path
func (storage_client *Storage_Client) SetKeyFile(keyFile string) {
	storage_client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (storage_client *Storage_Client) SetCaFile(caFile string) {
	storage_client.caFile = caFile
}

////////////////// Service functionnality //////////////////////

// Stop the service.
func (storage_client *Storage_Client) StopService() {
	storage_client.c.Stop(globular.GetClientContext(storage_client), &storagepb.StopRequest{})
}

func (storage_client *Storage_Client) CreateConnection(id string, name string, connectionType float64) error {

	rqst := &storagepb.CreateConnectionRqst{
		Connection: &storagepb.Connection{
			Id:   id,
			Name: name,
			Type: storagepb.StoreType(connectionType), // Disk store (persistent)
		},
	}

	_, err := storage_client.c.CreateConnection(globular.GetClientContext(storage_client), rqst)

	return err
}

func (storage_client *Storage_Client) OpenConnection(id string, options string) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.OpenRqst{
		Id:      id,
		Options: options,
	}

	_, err := storage_client.c.Open(globular.GetClientContext(storage_client), rqst)

	return err
}

func (storage_client *Storage_Client) SetItem(connectionId string, key string, data []byte) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.SetItemRequest{
		Id:    connectionId,
		Key:   key,
		Value: data,
	}

	_, err := storage_client.c.SetItem(globular.GetClientContext(storage_client), rqst)
	return err
}

func (storage_client *Storage_Client) SetLargeItem(connectionId string, key string, value []byte) error {

	// Open the stream...
	stream, err := storage_client.c.SetLargeItem(globular.GetClientContext(storage_client))
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

func (storage_client *Storage_Client) GetItem(connectionId string, key string) ([]byte, error) {
	// I will execute a simple ldap search here...
	rqst := &storagepb.GetItemRequest{
		Id:  connectionId,
		Key: key,
	}

	stream, err := storage_client.c.GetItem(globular.GetClientContext(storage_client), rqst)
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

func (storage_client *Storage_Client) RemoveItem(connectionId string, key string) error {
	// I will execute a simple ldap search here...
	rqst := &storagepb.RemoveItemRequest{
		Id:  connectionId,
		Key: key,
	}

	_, err := storage_client.c.RemoveItem(globular.GetClientContext(storage_client), rqst)
	return err
}

func (storage_client *Storage_Client) Clear(connectionId string) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.ClearRequest{
		Id: connectionId,
	}

	_, err := storage_client.c.Clear(globular.GetClientContext(storage_client), rqst)
	return err
}

func (storage_client *Storage_Client) Drop(connectionId string) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.DropRequest{
		Id: connectionId,
	}

	_, err := storage_client.c.Drop(globular.GetClientContext(storage_client), rqst)
	return err
}

func (storage_client *Storage_Client) CloseConnection(connectionId string) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.CloseRqst{
		Id: connectionId,
	}

	_, err := storage_client.c.Close(globular.GetClientContext(storage_client), rqst)
	return err
}

func (storage_client *Storage_Client) DeleteConnection(connectionId string) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.DeleteConnectionRqst{
		Id: connectionId,
	}

	_, err := storage_client.c.DeleteConnection(globular.GetClientContext(storage_client), rqst)
	return err
}
