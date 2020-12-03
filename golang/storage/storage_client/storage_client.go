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

func (self *Storage_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(self)
	}
	return globular.InvokeClientRequest(self.c, ctx, method, rqst)
}

// Return the domain
func (self *Storage_Client) GetDomain() string {
	return self.domain
}

// Return the address
func (self *Storage_Client) GetAddress() string {
	return self.domain + ":" + strconv.Itoa(self.port)
}

// Return the id of the service instance
func (self *Storage_Client) GetId() string {
	return self.id
}

// Return the name of the service
func (self *Storage_Client) GetName() string {
	return self.name
}

// must be close when no more needed.
func (self *Storage_Client) Close() {
	self.cc.Close()
}

// Set grpc_service port.
func (self *Storage_Client) SetPort(port int) {
	self.port = port
}

// Set the client instance sevice id.
func (self *Storage_Client) SetId(id string) {
	self.id = id
}

// Set the client name.
func (self *Storage_Client) SetName(name string) {
	self.name = name
}

// Set the domain.
func (self *Storage_Client) SetDomain(domain string) {
	self.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (self *Storage_Client) HasTLS() bool {
	return self.hasTLS
}

// Get the TLS certificate file path
func (self *Storage_Client) GetCertFile() string {
	return self.certFile
}

// Get the TLS key file path
func (self *Storage_Client) GetKeyFile() string {
	return self.keyFile
}

// Get the TLS key file path
func (self *Storage_Client) GetCaFile() string {
	return self.caFile
}

// Set the client is a secure client.
func (self *Storage_Client) SetTLS(hasTls bool) {
	self.hasTLS = hasTls
}

// Set TLS certificate file path
func (self *Storage_Client) SetCertFile(certFile string) {
	self.certFile = certFile
}

// Set TLS key file path
func (self *Storage_Client) SetKeyFile(keyFile string) {
	self.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (self *Storage_Client) SetCaFile(caFile string) {
	self.caFile = caFile
}

////////////////// Service functionnality //////////////////////

// Stop the service.
func (self *Storage_Client) StopService() {
	self.c.Stop(globular.GetClientContext(self), &storagepb.StopRequest{})
}

func (self *Storage_Client) CreateConnection(id string, name string, connectionType float64) error {

	rqst := &storagepb.CreateConnectionRqst{
		Connection: &storagepb.Connection{
			Id:   id,
			Name: name,
			Type: storagepb.StoreType(connectionType), // Disk store (persistent)
		},
	}

	_, err := self.c.CreateConnection(globular.GetClientContext(self), rqst)

	return err
}

func (self *Storage_Client) OpenConnection(id string, options string) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.OpenRqst{
		Id:      id,
		Options: options,
	}

	_, err := self.c.Open(globular.GetClientContext(self), rqst)

	return err
}

func (self *Storage_Client) SetItem(connectionId string, key string, data []byte) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.SetItemRequest{
		Id:    connectionId,
		Key:   key,
		Value: data,
	}

	_, err := self.c.SetItem(globular.GetClientContext(self), rqst)
	return err
}

func (self *Storage_Client) SetLargeItem(connectionId string, key string, value []byte) error {

	// Open the stream...
	stream, err := self.c.SetLargeItem(globular.GetClientContext(self))
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

func (self *Storage_Client) GetItem(connectionId string, key string) ([]byte, error) {
	// I will execute a simple ldap search here...
	rqst := &storagepb.GetItemRequest{
		Id:  connectionId,
		Key: key,
	}

	stream, err := self.c.GetItem(globular.GetClientContext(self), rqst)
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

func (self *Storage_Client) RemoveItem(connectionId string, key string) error {
	// I will execute a simple ldap search here...
	rqst := &storagepb.RemoveItemRequest{
		Id:  connectionId,
		Key: key,
	}

	_, err := self.c.RemoveItem(globular.GetClientContext(self), rqst)
	return err
}

func (self *Storage_Client) Clear(connectionId string) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.ClearRequest{
		Id: connectionId,
	}

	_, err := self.c.Clear(globular.GetClientContext(self), rqst)
	return err
}

func (self *Storage_Client) Drop(connectionId string) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.DropRequest{
		Id: connectionId,
	}

	_, err := self.c.Drop(globular.GetClientContext(self), rqst)
	return err
}

func (self *Storage_Client) CloseConnection(connectionId string) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.CloseRqst{
		Id: connectionId,
	}

	_, err := self.c.Close(globular.GetClientContext(self), rqst)
	return err
}

func (self *Storage_Client) DeleteConnection(connectionId string) error {

	// I will execute a simple ldap search here...
	rqst := &storagepb.DeleteConnectionRqst{
		Id: connectionId,
	}

	_, err := self.c.DeleteConnection(globular.GetClientContext(self), rqst)
	return err
}
