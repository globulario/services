package log_client

import (
	//	"time"

	"context"
	"errors"
	"io"
	"log"
	"strconv"
	"time"

	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type Log_Client struct {
	cc *grpc.ClientConn
	c  logpb.LogServiceClient

	// The id of the service
	id string

	// The mac address of the server
	mac string

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

	// The client context
	ctx context.Context
}

// Create a connection to the service.
func NewLogService_Client(address string, id string) (*Log_Client, error) {

	client := new(Log_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {

		return nil, err
	}

	client.cc, err = globular.GetClientConnection(client)
	if err != nil {

		return nil, err
	}

	client.c = logpb.NewLogServiceClient(client.cc)

	return client, nil
}

// Return the configuration from the configuration server.
func (client *Log_Client) GetConfiguration(address string) (map[string]interface{}, error) {
	return nil, errors.New("no implemented...")
}

func (client *Log_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Log_Client) GetCtx() context.Context {
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

// Return the ipv4 address
// Return the address
func (client *Log_Client) GetAddress() string {
	return client.domain + ":" + strconv.Itoa(client.port)
}

// Return the domain
func (client *Log_Client) GetDomain() string {
	return client.domain
}

// Return the id of the service instance
func (client *Log_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Log_Client) GetName() string {
	return client.name
}

func (client *Log_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Log_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Log_Client) SetPort(port int) {
	client.port = port
}

// Set the client name.
func (client *Log_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Log_Client) SetName(name string) {
	client.name = name
}

func (client *Log_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Log_Client) SetDomain(domain string) {
	client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Log_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Log_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Log_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Log_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Log_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Log_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Log_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Log_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

////////////////////////////////////////////////////////////////////////////////

// Append a new log information.
func (client *Log_Client) Log(application string, user string, method string, level logpb.LogLevel, message string, fileLine string, functionName string) error {
	// do not log itself.
	if method == "/log.LogService/Log" {
		return errors.New("recursive function call cycle")
	}

	// Here I set a log information.
	rqst := new(logpb.LogRqst)
	info := new(logpb.LogInfo)
	occurence := new(logpb.Occurence)

	occurence.Application = application
	occurence.UserName = user
	occurence.Date = time.Now().Unix()

	info.Method = method
	info.FunctionName = functionName
	info.Line = fileLine
	info.Level = level
	info.Message = message

	rqst.Info = info
	rqst.Occurence = occurence

	_, err := client.c.Log(client.GetCtx(), rqst)

	log.Println(application, user, method, level, message)

	return err
}

/**
 * Return an array of log infos.
 */
func (client *Log_Client) GetLog(query string) ([]*logpb.LogInfo, error) {
	rqst := &logpb.GetLogRqst{
		Query: query,
	}

	stream, err := client.c.GetLog(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	infos := make([]*logpb.LogInfo, 0)
	// Here I will create the final array

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			// end of stream...
			break
		}
		if err != nil {
			return nil, err
		}

		infos = append(infos, msg.Infos...)
	}

	// The buffer that contain the
	return infos, nil
}

/**
 * Delete a given log.
 */
func (client *Log_Client) DeleteLog(info *logpb.LogInfo) error {
	rqst := &logpb.DeleteLogRqst{
		Log: info,
	}

	_, err := client.c.DeleteLog(client.GetCtx(), rqst)

	return err
}

/**
 * Clear all method
 */
func (client *Log_Client) ClearLog(query string) error {
	rqst := &logpb.ClearAllLogRqst{
		Query: query,
	}

	_, err := client.c.ClearAllLog(client.GetCtx(), rqst)

	return err
}
