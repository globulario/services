package search_client

import (
	"strconv"

	"context"
	"io"

	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/search/searchpb"

	//"github.com/davecourtois/Utility"
	"google.golang.org/grpc"
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
func NewSearchService_Client(address string, id string) (*Search_Client, error) {
	client := new(Search_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = searchpb.NewSearchServiceClient(client.cc)

	return client, nil
}

func (search_client *Search_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(search_client)
	}
	return globular.InvokeClientRequest(search_client.c, ctx, method, rqst)
}

// Return the domain
func (search_client *Search_Client) GetDomain() string {
	return search_client.domain
}

// Return the address
func (search_client *Search_Client) GetAddress() string {
	return search_client.domain + ":" + strconv.Itoa(search_client.port)
}

// Return the id of the service instance
func (search_client *Search_Client) GetId() string {
	return search_client.id
}

// Return the name of the service
func (search_client *Search_Client) GetName() string {
	return search_client.name
}

func (search_client *Search_Client) GetMac() string {
	return search_client.mac
}

// must be close when no more needed.
func (search_client *Search_Client) Close() {
	search_client.cc.Close()
}

// Set grpc_service port.
func (search_client *Search_Client) SetPort(port int) {
	search_client.port = port
}

// Set the client service id.
func (search_client *Search_Client) SetId(id string) {
	search_client.id = id
}

// Set the client name.
func (search_client *Search_Client) SetName(name string) {
	search_client.name = name
}

func (search_client *Search_Client) SetMac(mac string) {
	search_client.mac = mac
}

// Set the domain.
func (search_client *Search_Client) SetDomain(domain string) {
	search_client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (search_client *Search_Client) HasTLS() bool {
	return search_client.hasTLS
}

// Get the TLS certificate file path
func (search_client *Search_Client) GetCertFile() string {
	return search_client.certFile
}

// Get the TLS key file path
func (search_client *Search_Client) GetKeyFile() string {
	return search_client.keyFile
}

// Get the TLS key file path
func (search_client *Search_Client) GetCaFile() string {
	return search_client.caFile
}

// Set the client is a secure client.
func (search_client *Search_Client) SetTLS(hasTls bool) {
	search_client.hasTLS = hasTls
}

// Set TLS certificate file path
func (search_client *Search_Client) SetCertFile(certFile string) {
	search_client.certFile = certFile
}

// Set TLS key file path
func (search_client *Search_Client) SetKeyFile(keyFile string) {
	search_client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (search_client *Search_Client) SetCaFile(caFile string) {
	search_client.caFile = caFile
}

////////////////// Api //////////////////////
// Stop the service.
func (search_client *Search_Client) StopService() {
	search_client.c.Stop(globular.GetClientContext(search_client), &searchpb.StopRequest{})
}

/**
 * Return the version of the underlying search engine.
 */
func (search_client *Search_Client) GetVersion() (string, error) {

	rqst := &searchpb.GetEngineVersionRequest{}
	ctx := globular.GetClientContext(search_client)
	rsp, err := search_client.c.GetEngineVersion(ctx, rqst)
	if err != nil {
		return "", err
	}
	return rsp.Message, nil
}

/**
 * Index a JSON object / array
 */
func (search_client *Search_Client) IndexJsonObject(path string, jsonStr string, language string, id string, indexs []string, data string) error {
	rqst := &searchpb.IndexJsonObjectRequest{
		JsonStr:  jsonStr,
		Language: language,
		Id:       id,
		Indexs:   indexs,
		Data:     data,
		Path:     path,
	}
	ctx := globular.GetClientContext(search_client)
	_, err := search_client.c.IndexJsonObject(ctx, rqst)
	if err != nil {
		return err
	}
	return nil
}

/**
 * Index a text file.
 * -dbPath the database path.
 * -filePath the file path must be reachable from the server.
 */
func (search_client *Search_Client) IndexFile(dbPath string, filePath string, language string) error {
	rqst := &searchpb.IndexFileRequest{
		DbPath:   dbPath,
		FilePath: filePath,
		Language: language,
	}

	ctx := globular.GetClientContext(search_client)
	_, err := search_client.c.IndexFile(ctx, rqst)
	if err != nil {
		return err
	}
	return nil
}

/**
 * Index a text file.
 * -dbPath the database path.
 * -dirPath the file path must be reachable from the server.
 */
func (search_client *Search_Client) IndexDir(dbPath string, dirPath string, language string) error {
	rqst := &searchpb.IndexDirRequest{
		DbPath:   dbPath,
		DirPath:  dirPath,
		Language: language,
	}

	ctx := globular.GetClientContext(search_client)
	_, err := search_client.c.IndexDir(ctx, rqst)
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
func (search_client *Search_Client) SearchDocuments(paths []string, query string, language string, fields []string, offset int32, pageSize int32, snippetLength int32) ([]*searchpb.SearchResult, error) {
	rqst := &searchpb.SearchDocumentsRequest{
		Paths:         paths,
		Query:         query,
		Language:      language,
		Fields:        fields,
		Offset:        offset,
		PageSize:      pageSize,
		SnippetLength: snippetLength,
	}

	ctx := globular.GetClientContext(search_client)
	stream, err := search_client.c.SearchDocuments(ctx, rqst)
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
func (search_client *Search_Client) Count(path string) (int32, error) {
	rqst := &searchpb.CountRequest{
		Path: path,
	}

	ctx := globular.GetClientContext(search_client)
	rsp, err := search_client.c.Count(ctx, rqst)

	if err != nil {
		return -1, err
	}

	return rsp.Result, nil
}

/**
 * Delete a docuement from the database.
 */
func (search_client *Search_Client) DeleteDocument(path string, id string) error {
	rqst := &searchpb.DeleteDocumentRequest{
		Path: path,
		Id:   id,
	}

	ctx := globular.GetClientContext(search_client)
	_, err := search_client.c.DeleteDocument(ctx, rqst)

	if err != nil {
		return err
	}

	return nil
}
