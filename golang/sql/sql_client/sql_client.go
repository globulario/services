package sql_client

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config/config_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/sql/sqlpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// //////////////////////////////////////////////////////////////////////////////
// SQL Client Service
// //////////////////////////////////////////////////////////////////////////////
type SQL_Client struct {
	cc *grpc.ClientConn
	c  sqlpb.SqlServiceClient

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
func NewSqlService_Client(address string, id string) (*SQL_Client, error) {
	client := new(SQL_Client)
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

func (client *SQL_Client) Reconnect() error {
	var err error
	nb_try_connect := 10
	
	for i:=0; i <nb_try_connect; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = sqlpb.NewSqlServiceClient(client.cc)
			break
		}
		
		// wait 500 millisecond before next try
		time.Sleep(500 * time.Millisecond)
	}
	
	return err
}

// The address where the client can connect.
func (client *SQL_Client) SetAddress(address string) {
	client.address = address
}

func (client *SQL_Client) GetConfiguration(address, id string) (map[string]interface{}, error) {
	Utility.RegisterFunction("NewConfigService_Client", config_client.NewConfigService_Client)
	client_, err := globular_client.GetClient(address, "config.ConfigService", "NewConfigService_Client")
	if err != nil {
		return nil, err
	}
	return client_.(*config_client.Config_Client).GetServiceConfiguration(id)
}

func (client *SQL_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *SQL_Client) GetCtx() context.Context {
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

// Return the last know connection state
func (client *SQL_Client) GetState() string {
	return client.state
}

// Return the domain
func (client *SQL_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *SQL_Client) GetAddress() string {
	return client.address
}

// Return the id of the service instance
func (client *SQL_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *SQL_Client) GetName() string {
	return client.name
}

func (client *SQL_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *SQL_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *SQL_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *SQL_Client) GetPort() int {
	return client.port
}

// Set the client name.
func (client *SQL_Client) SetName(name string) {
	client.name = name
}

func (client *SQL_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the client service instance id.
func (client *SQL_Client) SetId(id string) {
	client.id = id
}

// Set the domain.
func (client *SQL_Client) SetDomain(domain string) {
	client.domain = domain
}

func (client *SQL_Client) SetState(state string) {
	client.state = state
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *SQL_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *SQL_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *SQL_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *SQL_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *SQL_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *SQL_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *SQL_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *SQL_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

// //////////////////////// API ////////////////////////////
// Stop the service.
func (client *SQL_Client) StopService() {
	client.c.Stop(client.GetCtx(), &sqlpb.StopRequest{})
}

func (client *SQL_Client) CreateConnection(connectionId string, name string, driver string, user string, password string, host string, port int32, charset string, path string) error {
	

	
	// Create a new connection
	rqst := &sqlpb.CreateConnectionRqst{
		Connection: &sqlpb.Connection{
			Id:       connectionId,
			Name:     name,
			User:     user,
			Password: password,
			Port:     port,
			Host:     host,
			Driver:   driver,
			Charset:  charset,
			Path:     path,
		},
	}


	_, err := client.c.CreateConnection(client.GetCtx(), rqst)

	return err
}

func (client *SQL_Client) DeleteConnection(connectionId string) error {

	rqst := &sqlpb.DeleteConnectionRqst{
		Id: connectionId,
	}

	_, err := client.c.DeleteConnection(client.GetCtx(), rqst)

	return err
}

// Test if a connection is found
func (client *SQL_Client) Ping(connectionId interface{}) (string, error) {

	// Here I will try to ping a non-existing connection.
	rqst := &sqlpb.PingConnectionRqst{
		Id: Utility.ToString(connectionId),
	}

	rsp, err := client.c.Ping(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return rsp.Result, err
}

// That function return the json string with all element in it.
func (client *SQL_Client) QueryContext(connectionId string, query string, parameters string) (string, error) {

	// The query and all it parameters.
	rqst := &sqlpb.QueryContextRqst{
		Query: &sqlpb.Query{
			ConnectionId: connectionId,
			Query:        query,
			Parameters:   parameters,
		},
	}

	// Because number of values can be high I will use a stream.
	stream, err := client.c.QueryContext(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	// Here I will create the final array
	data := make([]interface{}, 0)
	header := make([]map[string]interface{}, 0)

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			// end of stream...
			break
		}
		if err != nil {
			return "", err
		}

		// Get the result...
		switch v := msg.Result.(type) {
		case *sqlpb.QueryContextRsp_Header:
			// Here I receive the header information.
			json.Unmarshal([]byte(v.Header), &header)
		case *sqlpb.QueryContextRsp_Rows:
			rows := make([]interface{}, 0)
			json.Unmarshal([]byte(v.Rows), &rows)
			data = append(data, rows...)
		}
	}

	// Create object result and put header and data in it.
	result := make(map[string]interface{})
	result["header"] = header
	result["data"] = data
	resultStr, _ := json.Marshal(result)
	return string(resultStr), nil
}

func (client *SQL_Client) ExecContext(connectionId interface{}, query interface{}, parameters string, tx interface{}) (string, error) {

	if tx == nil {
		tx = false
	}

	rqst := &sqlpb.ExecContextRqst{
		Query: &sqlpb.Query{
			ConnectionId: Utility.ToString(connectionId),
			Query:        Utility.ToString(query),
			Parameters:   parameters,
		},
		Tx: Utility.ToBool(tx),
	}

	rsp, err := client.c.ExecContext(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	result := make(map[string]interface{})
	result["affectRows"] = rsp.AffectedRows
	result["lastId"] = rsp.LastId
	resultStr, _ := json.Marshal(result)

	return string(resultStr), nil
}
