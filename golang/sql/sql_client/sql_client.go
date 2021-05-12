package sql_client

import (
	"context"
	"encoding/json"
	"io"
	"strconv"

	"github.com/davecourtois/Utility"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/sql/sqlpb"
	"google.golang.org/grpc"
)

////////////////////////////////////////////////////////////////////////////////
// SQL Client Service
////////////////////////////////////////////////////////////////////////////////
type SQL_Client struct {
	cc *grpc.ClientConn
	c  sqlpb.SqlServiceClient

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
func NewSqlService_Client(address string, id string) (*SQL_Client, error) {
	client := new(SQL_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = sqlpb.NewSqlServiceClient(client.cc)

	return client, nil
}

func (sql_client *SQL_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(sql_client)
	}
	return globular.InvokeClientRequest(sql_client.c, ctx, method, rqst)
}

// Return the domain
func (sql_client *SQL_Client) GetDomain() string {
	return sql_client.domain
}

// Return the address
func (sql_client *SQL_Client) GetAddress() string {
	return sql_client.domain + ":" + strconv.Itoa(sql_client.port)
}

// Return the id of the service instance
func (sql_client *SQL_Client) GetId() string {
	return sql_client.id
}

// Return the name of the service
func (sql_client *SQL_Client) GetName() string {
	return sql_client.name
}

// must be close when no more needed.
func (sql_client *SQL_Client) Close() {
	sql_client.cc.Close()
}

// Set grpc_service port.
func (sql_client *SQL_Client) SetPort(port int) {
	sql_client.port = port
}

// Set the client name.
func (sql_client *SQL_Client) SetName(name string) {
	sql_client.name = name
}

// Set the client service instance id.
func (sql_client *SQL_Client) SetId(id string) {
	sql_client.id = id
}

// Set the domain.
func (sql_client *SQL_Client) SetDomain(domain string) {
	sql_client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (sql_client *SQL_Client) HasTLS() bool {
	return sql_client.hasTLS
}

// Get the TLS certificate file path
func (sql_client *SQL_Client) GetCertFile() string {
	return sql_client.certFile
}

// Get the TLS key file path
func (sql_client *SQL_Client) GetKeyFile() string {
	return sql_client.keyFile
}

// Get the TLS key file path
func (sql_client *SQL_Client) GetCaFile() string {
	return sql_client.caFile
}

// Set the client is a secure client.
func (sql_client *SQL_Client) SetTLS(hasTls bool) {
	sql_client.hasTLS = hasTls
}

// Set TLS certificate file path
func (sql_client *SQL_Client) SetCertFile(certFile string) {
	sql_client.certFile = certFile
}

// Set TLS key file path
func (sql_client *SQL_Client) SetKeyFile(keyFile string) {
	sql_client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (sql_client *SQL_Client) SetCaFile(caFile string) {
	sql_client.caFile = caFile
}

////////////////////////// API ////////////////////////////
// Stop the service.
func (sql_client *SQL_Client) StopService() {
	sql_client.c.Stop(globular.GetClientContext(sql_client), &sqlpb.StopRequest{})
}

func (sql_client *SQL_Client) CreateConnection(connectionId string, name string, driver string, user string, password string, host string, port int32, charset string) error {
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
		},
	}

	_, err := sql_client.c.CreateConnection(globular.GetClientContext(sql_client), rqst)

	return err
}

func (sql_client *SQL_Client) DeleteConnection(connectionId string) error {

	rqst := &sqlpb.DeleteConnectionRqst{
		Id: connectionId,
	}

	_, err := sql_client.c.DeleteConnection(globular.GetClientContext(sql_client), rqst)

	return err
}

// Test if a connection is found
func (sql_client *SQL_Client) Ping(connectionId interface{}) (string, error) {

	// Here I will try to ping a non-existing connection.
	rqst := &sqlpb.PingConnectionRqst{
		Id: Utility.ToString(connectionId),
	}

	rsp, err := sql_client.c.Ping(globular.GetClientContext(sql_client), rqst)
	if err != nil {
		return "", err
	}

	return rsp.Result, err
}

// That function return the json string with all element in it.
func (sql_client *SQL_Client) QueryContext(connectionId string, query string, parameters string) (string, error) {

	// The query and all it parameters.
	rqst := &sqlpb.QueryContextRqst{
		Query: &sqlpb.Query{
			ConnectionId: connectionId,
			Query:        query,
			Parameters:   parameters,
		},
	}

	// Because number of values can be high I will use a stream.
	stream, err := sql_client.c.QueryContext(globular.GetClientContext(sql_client), rqst)
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

func (sql_client *SQL_Client) ExecContext(connectionId interface{}, query interface{}, parameters string, tx interface{}) (string, error) {

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

	rsp, err := sql_client.c.ExecContext(globular.GetClientContext(sql_client), rqst)
	if err != nil {
		return "", err
	}

	result := make(map[string]interface{})
	result["affectRows"] = rsp.AffectedRows
	result["lastId"] = rsp.LastId
	resultStr, _ := json.Marshal(result)

	return string(resultStr), nil
}
