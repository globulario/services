package log_client

import (
	//	"time"

	"context"
	"errors"
	"io"
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

	//  keep the last connection state of the client.
	state string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

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

	err = client.Reconnect()
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (client *Log_Client) Reconnect() error {

	var err error
	nb_try_connect := 10

	for i := 0; i < nb_try_connect; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = logpb.NewLogServiceClient(client.cc)
			break
		}

		// wait 500 millisecond before next try
		time.Sleep(500 * time.Millisecond)
	}

	return err
}

// The address where the client can connect.
func (client *Log_Client) SetAddress(address string) {
	client.address = address
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
		md := metadata.New(map[string]string{"token": string(token), "domain": client.domain, "mac": client.GetMac(), "address": client.GetAddress()})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	return client.ctx
}

// Return the ipv4 address
// Return the address
func (client *Log_Client) GetAddress() string {
	return client.address
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

// Return the last know connection state
func (client *Log_Client) GetState() string {
	return client.state
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

// Return the grpc port number
func (client *Log_Client) GetPort() int {
	return client.port
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

func (client *Log_Client) SetState(state string) {
	client.state = state
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
func (client *Log_Client) Log(application string, user string, method string, level logpb.LogLevel, message string, fileLine string, functionName string, token string) error {
	// do not log itself.
	if method == "/log.LogService/Log" {
		return errors.New("recursive function call cycle")
	}

	// Here I set a log information.
	rqst := new(logpb.LogRqst)
	info := new(logpb.LogInfo)

	info.Method = method
	info.Line = fileLine
	info.Level = level
	info.Application = application
	info.Message = message
	info.Occurences = 0

	rqst.Info = info

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.Log(ctx, rqst)

	return err
}

/**
 * Return an array of log infos.
 */
// Back-compat helper that uses the client's default context.
func (client *Log_Client) GetLog(query string) ([]*logpb.LogInfo, error) {
	return client.GetLogCtx(client.GetCtx(), query)
}

// Append a new log information with a caller-provided context.
func (client *Log_Client) LogCtx(
	ctx context.Context,
	application string,
	user string,
	method string,
	level logpb.LogLevel,
	message string,
	fileLine string,
	functionName string,
	token string,
) error {
	// do not log itself.
	if method == "/log.LogService/Log" {
		return errors.New("recursive function call cycle")
	}

	// Prepare request.
	rqst := new(logpb.LogRqst)
	info := new(logpb.LogInfo)

	info.Method = method
	info.Line = fileLine
	info.Level = level
	info.Application = application
	info.Message = message
	info.Occurences = 0

	rqst.Info = info

	// If a token is provided, override any token in the context.
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) == 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	// Use the provided ctx (so caller can inject metadata/deadlines).
	_, err := client.c.Log(ctx, rqst)
	return err
}

// Same as LogCtx but lets you include component and structured fields.
func (client *Log_Client) LogWithFieldsCtx(
	ctx context.Context,
	application, user, method string,
	level logpb.LogLevel,
	message, fileLine, functionName, component string,
	fields map[string]string,
	token string,
) error {
	if method == "/log.LogService/Log" {
		return errors.New("recursive function call cycle")
	}
	rqst := &logpb.LogRqst{
		Info: &logpb.LogInfo{
			Method:      method,
			Line:        fileLine,
			Level:       level,
			Application: application,
			Message:     message,
			Occurences:  0,
			Component:   component,
			Fields:      fields,
		},
	}

	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) == 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.Log(ctx, rqst)
	return err
}

// GetLogCtx is like GetLog, but uses the provided context (for metadata, deadlines, etc.).
func (client *Log_Client) GetLogCtx(ctx context.Context, query string) ([]*logpb.LogInfo, error) {
	rqst := &logpb.GetLogRqst{Query: query}

	stream, err := client.c.GetLog(ctx, rqst)
	if err != nil {
		return nil, err
	}

	var infos []*logpb.LogInfo
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break // end of stream
		}
		if err != nil {
			return nil, err
		}
		infos = append(infos, msg.Infos...)
	}
	return infos, nil
}

/**
 * Delete a given log.
 */
func (client *Log_Client) DeleteLog(info *logpb.LogInfo, token string) error {
	rqst := &logpb.DeleteLogRqst{
		Log: info,
	}

	var ctx context.Context = client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) == 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}


	_, err := client.c.DeleteLog(ctx, rqst)

	return err
}

/**
 * Clear all method
 */
func (client *Log_Client) ClearLog(query string, token string) error {
	rqst := &logpb.ClearAllLogRqst{
		Query: query,
	}

	var ctx context.Context = client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) == 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.ClearAllLog(ctx, rqst)

	return err
}
