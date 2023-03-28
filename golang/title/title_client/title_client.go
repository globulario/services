package title_client

import (
	"context"
	"errors"
	"io"
	"time"

	//"github.com/davecourtois/Utility"
	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config/config_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/title/titlepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// title Client Service
////////////////////////////////////////////////////////////////////////////////

type Title_Client struct {
	cc *grpc.ClientConn
	c  titlepb.TitleServiceClient

	// The id of the service
	id string

	// The name of the service
	name string

	// The client domain
	domain string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

	// The mac address of the server
	mac string

	//  keep the last connection state of the client.
	state string

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
func NewTitleService_Client(address string, id string) (*Title_Client, error) {
	client := new(Title_Client)
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

func (client *Title_Client) Reconnect() error {

	var err error
	nb_try_connect := 10
	
	for i:=0; i <nb_try_connect; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = titlepb.NewTitleServiceClient(client.cc)
			break
		}
		
		// wait 500 millisecond before next try
		time.Sleep(500 * time.Millisecond)
	}
	
	return err
}

// The address where the client can connect.
func (client *Title_Client) SetAddress(address string) {
	client.address = address
}

// Return the configuration from the configuration server.
func (client *Title_Client) GetConfiguration(address, id string) (map[string]interface{}, error) {
	Utility.RegisterFunction("NewConfigService_Client", config_client.NewConfigService_Client)
	client_, err := globular_client.GetClient(address, "config.ConfigService", "NewConfigService_Client")
	if err != nil {
		return nil, err
	}
	return client_.(*config_client.Config_Client).GetServiceConfiguration(id)
}

func (client *Title_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Title_Client) GetCtx() context.Context {
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
func (client *Title_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Title_Client) GetAddress() string {
	return client.address
}

// Return the id of the service instance
func (client *Title_Client) GetId() string {
	return client.id
}

// Return the last know connection state
func (client *Title_Client) GetState() string {
	return client.state
}

// Return the name of the service
func (client *Title_Client) GetName() string {
	return client.name
}

func (client *Title_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Title_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Title_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *Title_Client) GetPort() int {
	return client.port
}

// Set the client instance id.
func (client *Title_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Title_Client) SetName(name string) {
	client.name = name
}

func (client *Title_Client) SetMac(mac string) {
	client.mac = mac
}

func (client *Title_Client) SetState(state string) {
	client.state = state
}

// Set the domain.
func (client *Title_Client) SetDomain(domain string) {
	client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Title_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Title_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Title_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Title_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Title_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Title_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Title_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Title_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

// //////////////// Api //////////////////////
func (client *Title_Client) CreateTitle(token, path string, title *titlepb.Title) error {

	rqst := &titlepb.CreateTitleRequest{
		Title:     title,
		IndexPath: path,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
	}

	_, err := client.c.CreateTitle(ctx, rqst)

	return err
}

func (client *Title_Client) CreateAudio(token, path string, track *titlepb.Audio) error {

	rqst := &titlepb.CreateAudioRequest{
		Audio:     track,
		IndexPath: path,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
	}

	_, err := client.c.CreateAudio(ctx, rqst)

	return err
}

/**
 * Return the list of title asscociated with a given path.
 */
func (client *Title_Client) GetTitleFiles(indexPath, titleId string) ([]string, error) {
	rqst := &titlepb.GetTitleFilesRequest{IndexPath: indexPath, TitleId: titleId}

	rsp, err := client.c.GetTitleFiles(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	return rsp.FilePaths, nil
}

func (client *Title_Client) GetFileTitles(indexPath, path string) ([]*titlepb.Title, error) {
	rqst := &titlepb.GetFileTitlesRequest{IndexPath: indexPath, FilePath: path}

	rsp, err := client.c.GetFileTitles(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	if len(rsp.Titles.Titles) == 0 {
		return nil, errors.New("no titles found")
	}

	return rsp.Titles.Titles, nil
}

func (client *Title_Client) GetFileVideos(indexPath, path string) ([]*titlepb.Video, error) {
	rqst := &titlepb.GetFileVideosRequest{IndexPath: indexPath, FilePath: path}

	rsp, err := client.c.GetFileVideos(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	if len(rsp.Videos.Videos) == 0 {
		return nil, errors.New("no videos found")
	}

	return rsp.Videos.Videos, nil
}

// Return the list of audios for a given path...
func (client *Title_Client) GetFileAudios(indexPath, path string) ([]*titlepb.Audio, error) {
	rqst := &titlepb.GetFileAudiosRequest{IndexPath: indexPath, FilePath: path}

	rsp, err := client.c.GetFileAudios(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	if len(rsp.Audios.Audios) == 0 {
		return nil, errors.New("no audios found")
	}

	return rsp.Audios.Audios, nil
}

/**
 * Get the title by it id.
 */
func (client *Title_Client) GetTitleById(path, id string) (*titlepb.Title, []string, error) {
	rqst := &titlepb.GetTitleByIdRequest{
		IndexPath: path,
		TitleId:   id,
	}

	rsp, err := client.c.GetTitleById(client.GetCtx(), rqst)
	if err != nil {
		return nil, nil, err
	}

	// Return a title.
	return rsp.GetTitle(), rsp.GetFilesPaths(), nil
}

/**
 * Get the video by it id.
 */
func (client *Title_Client) GetVideoById(path, id string) (*titlepb.Video, []string, error) {
	rqst := &titlepb.GetVideoByIdRequest{
		IndexPath: path,
		VidoeId:   id,
	}

	rsp, err := client.c.GetVideoById(client.GetCtx(), rqst)
	if err != nil {
		return nil, nil, err
	}

	// Return a title.
	return rsp.GetVideo(), rsp.GetFilesPaths(), nil
}

/**
 * Search titles with a query, title, genre etc...
 */
func (client *Title_Client) SearchTitle(path, query string, fields []string) (*titlepb.SearchSummary, []*titlepb.SearchHit, *titlepb.SearchFacets, error) {
	rqst := &titlepb.SearchTitlesRequest{
		IndexPath: path,
		Query:     query,
		Fields:    fields,
	}

	stream, err := client.c.SearchTitles(client.GetCtx(), rqst)
	if err != nil {
		return nil, nil, nil, err
	}

	hits := make([]*titlepb.SearchHit, 0)
	var summary *titlepb.SearchSummary
	var facets *titlepb.SearchFacets

	for {
		rsp, err := stream.Recv()
		if err == io.EOF {
			// end of stream...
			break
		}
		if err != nil {
			return nil, nil, nil, err
		}

		switch v := rsp.Result.(type) {
		case *titlepb.SearchTitlesResponse_Hit:
			hits = append(hits, v.Hit)
		case *titlepb.SearchTitlesResponse_Summary:
			summary = v.Summary
		case *titlepb.SearchTitlesResponse_Facets:
			facets = v.Facets
		}
	}

	// return the results...
	return summary, hits, facets, nil
}

/**
 * Delete a given title from the indexation.
 */
func (client *Title_Client) DeleteTitle(path, id string) error {
	rqst := &titlepb.DeleteTitleRequest{
		IndexPath: path,
		TitleId:   id,
	}

	_, err := client.c.DeleteTitle(client.GetCtx(), rqst)
	return err
}

/**
 * Asscociate a title with a given file.
 */
func (client *Title_Client) AssociateFileWithTitle(indexPath, titleId, filePath string) error {
	rqst := &titlepb.AssociateFileWithTitleRequest{
		IndexPath: indexPath,
		TitleId:   titleId,
		FilePath:  filePath,
	}

	_, err := client.c.AssociateFileWithTitle(client.GetCtx(), rqst)
	return err
}

/**
 * Dissociate file from it title.
 */
func (client *Title_Client) DissociateFileWithTitle(indexPath, titleId, filePath string) error {
	rqst := &titlepb.DissociateFileWithTitleRequest{
		IndexPath: indexPath,
		TitleId:   titleId,
		FilePath:  filePath,
	}

	_, err := client.c.DissociateFileWithTitle(client.GetCtx(), rqst)
	return err
}

/**
 * Create video
 */
func (client *Title_Client) CreateVideo(token, path string, video *titlepb.Video) error {

	rqst := &titlepb.CreateVideoRequest{
		Video:     video,
		IndexPath: path,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.CreateVideo(ctx, rqst)

	return err
}

/**
 * Create a person (video participant, cast)
 */
func (client *Title_Client) CreatePerson(token, path string, p *titlepb.Person) error {

	rqst := &titlepb.CreatePersonRequest{
		Person:    p,
		IndexPath: path,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
	}

	_, err := client.c.CreatePerson(ctx, rqst)

	return err
}

func (client *Title_Client) CreatePublisher(token, path string, p *titlepb.Publisher) error {

	rqst := &titlepb.CreatePublisherRequest{
		Publisher: p,
		IndexPath: path,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
	}

	_, err := client.c.CreatePublisher(ctx, rqst)

	return err
}
