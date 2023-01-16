package search_client

import (
	"context"
	"io"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config/config_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/search/searchpb"
	"github.com/globulario/services/golang/security"

	//"github.com/davecourtois/Utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// echo Client Service
////////////////////////////////////////////////////////////////////////////////

type Search_Client struct {
	cc *grpc.ClientConn
	c  searchpb.SearchServiceClient

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
func NewSearchService_Client(address string, id string) (*Search_Client, error) {
	client := new(Search_Client)
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

func (client *Search_Client) Reconnect() error {
	var err error

	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return err
	}

	client.c = searchpb.NewSearchServiceClient(client.cc)
	return nil
}

// The address where the client can connect.
func (client *Search_Client) SetAddress(address string) {
	client.address = address
}

func (client *Search_Client) GetConfiguration(address, id string) (map[string]interface{}, error) {
	Utility.RegisterFunction("NewConfigService_Client", config_client.NewConfigService_Client)
	client_, err := globular_client.GetClient(address, "config.ConfigService", "NewConfigService_Client")
	if err != nil {
		return nil, err
	}
	return client_.(*config_client.Config_Client).GetServiceConfiguration(id)
}

func (client *Search_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Search_Client) GetCtx() context.Context {
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
func (client *Search_Client) GetState() string {
	return client.state
}

// Return the domain
func (client *Search_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Search_Client) GetAddress() string {
	return client.address
}

// Return the id of the service instance
func (client *Search_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Search_Client) GetName() string {
	return client.name
}

func (client *Search_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Search_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Search_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *Search_Client) GetPort() int {
	return client.port
}

// Set the client service id.
func (client *Search_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Search_Client) SetName(name string) {
	client.name = name
}

func (client *Search_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Search_Client) SetDomain(domain string) {
	client.domain = domain
}

func (client *Search_Client) SetState(state string) {
	client.state = state
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Search_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Search_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Search_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Search_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Search_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Search_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Search_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Search_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

// //////////////// Api //////////////////////
// Stop the service.
func (client *Search_Client) StopService() {
	client.c.Stop(client.GetCtx(), &searchpb.StopRequest{})
}

/**
 * Return the version of the underlying search engine.
 */
func (client *Search_Client) GetVersion() (string, error) {

	rqst := &searchpb.GetEngineVersionRequest{}
	ctx := client.GetCtx()
	rsp, err := client.c.GetEngineVersion(ctx, rqst)
	if err != nil {
		return "", err
	}
	return rsp.Message, nil
}

/**
 * Index a JSON object / array
 */
func (client *Search_Client) IndexJsonObject(path string, jsonStr string, language string, id string, indexs []string, data string) error {
	rqst := &searchpb.IndexJsonObjectRequest{
		JsonStr:  jsonStr,
		Language: language,
		Id:       id,
		Indexs:   indexs,
		Data:     data,
		Path:     path,
	}
	ctx := client.GetCtx()
	_, err := client.c.IndexJsonObject(ctx, rqst)
	if err != nil {
		return err
	}
	return nil
}

/**
 * 	Execute a search over the db.
 *  -path The path of the db
 *  -query The query string
 *  -language The language of the db
 *  -fields The list of fields
 *  -offset The results offset
 *  -pageSize The number of result to be return.
 *  -snippetLength The length of the snippet.
 */
func (client *Search_Client) SearchDocuments(paths []string, query string, language string, fields []string, offset int32, pageSize int32, snippetLength int32) ([]*searchpb.SearchResult, error) {
	rqst := &searchpb.SearchDocumentsRequest{
		Paths:         paths,
		Query:         query,
		Language:      language,
		Fields:        fields,
		Offset:        offset,
		PageSize:      pageSize,
		SnippetLength: snippetLength,
	}

	ctx := client.GetCtx()
	stream, err := client.c.SearchDocuments(ctx, rqst)
	if err != nil {
		return nil, err
	}

	results := make([]*searchpb.SearchResult, 0)

	for {
		rsp, err := stream.Recv()
		if err == io.EOF {
			// end of stream...
			break
		}
		if err != nil {
			return nil, err
		}
		results = append(results, rsp.Results.GetResults()...)
	}

	return results, nil

}

/**
 * Count the number of document in a given database.
 */
func (client *Search_Client) Count(path string) (int32, error) {
	rqst := &searchpb.CountRequest{
		Path: path,
	}

	ctx := client.GetCtx()
	rsp, err := client.c.Count(ctx, rqst)

	if err != nil {
		return -1, err
	}

	return rsp.Result, nil
}

/**
 * Delete a docuement from the database.
 */
func (client *Search_Client) DeleteDocument(path string, id string) error {
	rqst := &searchpb.DeleteDocumentRequest{
		Path: path,
		Id:   id,
	}

	ctx := client.GetCtx()
	_, err := client.c.DeleteDocument(ctx, rqst)

	if err != nil {
		return err
	}

	return nil
}
