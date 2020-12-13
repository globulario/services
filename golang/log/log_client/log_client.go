package log_client

import (
	//	"time"

	"context"
	"strconv"

	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"google.golang.org/grpc"
)

type Log_Client struct {
	cc *grpc.ClientConn
	c  resourcepb.LogServiceClient

	// The id of the service
	id string

	// The name of the service
	name string

	// The client domain
	domain string

	// The port number
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
func NewResourceService_Client(address string, id string) (*Log_Client, error) {

	client := new(Log_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {

		return nil, err
	}

	client.cc, err = globular.GetClientConnection(client)
	if err != nil {

		return nil, err
	}

	client.c = resourcepb.NewLogServiceClient(client.cc)

	return client, nil
}

func (self *Log_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(self)
	}
	return globular.InvokeClientRequest(self.c, ctx, method, rqst)
}

// Return the ipv4 address
// Return the address
func (self *Log_Client) GetAddress() string {
	return self.domain + ":" + strconv.Itoa(self.port)
}

// Return the domain
func (self *Log_Client) GetDomain() string {
	return self.domain
}

// Return the id of the service instance
func (self *Log_Client) GetId() string {
	return self.id
}

// Return the name of the service
func (self *Log_Client) GetName() string {
	return self.name
}

// must be close when no more needed.
func (self *Log_Client) Close() {
	self.cc.Close()
}

// Set grpc_service port.
func (self *Log_Client) SetPort(port int) {
	self.port = port
}

// Set the client name.
func (self *Log_Client) SetId(id string) {
	self.id = id
}

// Set the client name.
func (self *Log_Client) SetName(name string) {
	self.name = name
}

// Set the domain.
func (self *Log_Client) SetDomain(domain string) {
	self.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (self *Log_Client) HasTLS() bool {
	return self.hasTLS
}

// Get the TLS certificate file path
func (self *Log_Client) GetCertFile() string {
	return self.certFile
}

// Get the TLS key file path
func (self *Log_Client) GetKeyFile() string {
	return self.keyFile
}

// Get the TLS key file path
func (self *Log_Client) GetCaFile() string {
	return self.caFile
}

// Set the client is a secure client.
func (self *Log_Client) SetTLS(hasTls bool) {
	self.hasTLS = hasTls
}

// Set TLS certificate file path
func (self *Log_Client) SetCertFile(certFile string) {
	self.certFile = certFile
}

// Set TLS key file path
func (self *Log_Client) SetKeyFile(keyFile string) {
	self.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (self *Log_Client) SetCaFile(caFile string) {
	self.caFile = caFile
}

////////////////////////////////////////////////////////////////////////////////

// Append a new log information.
/*
func (self *Log_Client) Log(application string, user string, method string, err_ error) error {

	// Here I set a log information.
	rqst := new(resourcepb.LogRqst)
	info := new(resourcepb.LogInfo)
	info.Application = application
	info.UserName = user
	info.Method = method
	info.Date = time.Now().Unix()
	if err_ != nil {
		info.Message = err_.Error()
		info.Type = resourcepb.LogType_ERROR_MESSAGE
	} else {
		info.Type = resourcepb.LogType_INFO_MESSAGE
	}
	rqst.Info = info

	_, err := self.c.Log(globular.GetClientContext(self), rqst)

	return err
}
*/