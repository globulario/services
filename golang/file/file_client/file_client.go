package file_client

import (
	"io"
	"log"
	"os"

	"context"

	"github.com/globulario/services/golang/config/config_client"
	"github.com/globulario/services/golang/file/filepb"
	globular "github.com/globulario/services/golang/globular_client"

	"github.com/davecourtois/Utility"
	"google.golang.org/grpc"
)

////////////////////////////////////////////////////////////////////////////////
// File Client Service
////////////////////////////////////////////////////////////////////////////////

type File_Client struct {
	cc *grpc.ClientConn
	c  filepb.FileServiceClient

	// The id of the service
	id string

	// The name of the service
	name string

	// The mac address of the server
	mac string

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

	// The connection context
	ctx context.Context
}

// Create a connection to the service.
func NewFileService_Client(address string, id string) (*File_Client, error) {
	client := new(File_Client)
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

func (client *File_Client) Reconnect () error{
	var err error
	
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return  err
	}

	client.c = filepb.NewFileServiceClient(client.cc)
	return nil
}

// The address where the client can connect.
func (client *File_Client) SetAddress(address string) {
	client.address = address
}

// Return the configuration from the configuration server.
func (client *File_Client) GetConfiguration(address, id string) (map[string]interface{}, error) {
	client_, err := config_client.NewConfigService_Client(address, "config.ConfigService")
	if err != nil {
		return nil, err
	}
	return client_.GetServiceConfiguration(id)
}

func (client *File_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *File_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = client.GetCtx()
	}
	return client.ctx
}

// Return the domain
func (client *File_Client) GetDomain() string {
	return client.domain
}

// Return the last know connection state
func (client *File_Client) GetState() string {
	return client.state
}

// Return the address
func (client *File_Client) GetAddress() string {
	return client.address
}

// Return the id of the service instance
func (client *File_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *File_Client) GetName() string {
	return client.name
}

func (client *File_Client) SetState(state string) {
	client.state = state
}

func (client *File_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *File_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *File_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *File_Client) GetPort() int {
	return client.port
}

// Set the client instance id.
func (client *File_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *File_Client) SetName(name string) {
	client.name = name
}

func (client *File_Client) SetMac(mac string) {
	client.name = mac
}

// Set the domain.
func (client *File_Client) SetDomain(domain string) {
	client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *File_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *File_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *File_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *File_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *File_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *File_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *File_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *File_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

///////////////////// API //////////////////////

// Stop the service.
func (client *File_Client) StopService() {
	client.c.Stop(client.GetCtx(), &filepb.StopRequest{})
}

// Read the content of a dir and return it info.
func (client *File_Client) ReadDir(path interface{}, recursive interface{}, thumbnailHeight interface{}, thumbnailWidth interface{}) (string, error) {

	// Create a new client service...
	rqst := &filepb.ReadDirRequest{
		Path:           Utility.ToString(path),
		Recursive:      Utility.ToBool(recursive),
		ThumnailHeight: int32(Utility.ToInt(thumbnailHeight)),
		ThumnailWidth:  int32(Utility.ToInt(thumbnailWidth)),
	}

	stream, err := client.c.ReadDir(client.GetCtx(), rqst)
	if err != nil {
		log.Println("---> 181 ", err)
		return "", err
	}

	// Here I will create the final array
	data := make([]byte, 0)
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			// end of stream...
			break
		} else if err != nil {
			return "", err
		}

		data = append(data, msg.Data...)

	}

	return string(data), nil
}

/**
 * Create a new directory on the server.
 */
func (client *File_Client) CreateDir(path interface{}, name interface{}) error {

	rqst := &filepb.CreateDirRequest{
		Path: Utility.ToString(path),
		Name: Utility.ToString(name),
	}

	_, err := client.c.CreateDir(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Read file data
 */
func (client *File_Client) ReadFile(path interface{}) ([]byte, error) {

	rqst := &filepb.ReadFileRequest{
		Path: Utility.ToString(path),
	}

	stream, err := client.c.ReadFile(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	// Here I will create the final array
	data := make([]byte, 0)
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			// end of stream...
			break
		} else if err != nil {
			return nil, err
		}

		data = append(data, msg.Data...)

	}

	return data, err
}

/**
 * Rename a directory.
 */
func (client *File_Client) RenameDir(path interface{}, oldname interface{}, newname interface{}) error {

	rqst := &filepb.RenameRequest{
		Path:    Utility.ToString(path),
		OldName: Utility.ToString(oldname),
		NewName: Utility.ToString(newname),
	}

	_, err := client.c.Rename(client.GetCtx(), rqst)

	return err
}

/**
 * Delete a directory
 */
func (client *File_Client) DeleteDir(path string) error {
	rqst := &filepb.DeleteDirRequest{
		Path: Utility.ToString(path),
	}

	_, err := client.c.DeleteDir(client.GetCtx(), rqst)
	return err
}

/**
 * Get a single file info.
 */
func (client *File_Client) GetFileInfo(path interface{}, recursive interface{}, thumbnailHeight interface{}, thumbnailWidth interface{}) (string, error) {

	rqst := &filepb.GetFileInfoRequest{
		Path:           Utility.ToString(path),
		ThumnailHeight: int32(Utility.ToInt(thumbnailHeight)),
		ThumnailWidth:  int32(Utility.ToInt(thumbnailWidth)),
	}

	rsp, err := client.c.GetFileInfo(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return string(rsp.Data), nil
}

/**
 * That function move a file from a directory to another... (mv) in unix.
 */
func (client *File_Client) MoveFile(path interface{}, dest interface{}) error {

	// Open the stream...
	stream, err := client.c.SaveFile(client.GetCtx())
	if err != nil {
		return err
	}

	err = stream.Send(&filepb.SaveFileRequest{
		File: &filepb.SaveFileRequest_Path{
			Path: Utility.ToString(dest), // Where the file will be save...
		},
	})

	if err != nil {
		return err
	}

	// Where the file is read from.
	file, err := os.Open(Utility.ToString(path))
	if err != nil {
		return err
	}

	// close the file when done.
	defer file.Close()

	const BufferSize = 1024 * 5 // the chunck size.
	buffer := make([]byte, BufferSize)
	for {
		bytesread, err := file.Read(buffer)
		if bytesread > 0 {
			rqst := &filepb.SaveFileRequest{
				File: &filepb.SaveFileRequest_Data{
					Data: buffer[:bytesread],
				},
			}
			err = stream.Send(rqst)
		}

		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}
	}

	_, err = stream.CloseAndRecv()
	if err != nil {
		return err
	}

	return nil
}

/**
 * Delete a file with a given path.
 */
func (client *File_Client) DeleteFile(path string) error {

	rqst := &filepb.DeleteFileRequest{
		Path: Utility.ToString(path),
	}

	_, err := client.c.DeleteFile(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	return err
}

// Read the content of a dir and return all images as thumbnails.
func (client *File_Client) GetThumbnails(path interface{}, recursive interface{}, thumbnailHeight interface{}, thumbnailWidth interface{}) (string, error) {

	// Create a new client service...
	rqst := &filepb.GetThumbnailsRequest{
		Path:           Utility.ToString(path),
		Recursive:      Utility.ToBool(recursive),
		ThumnailHeight: int32(Utility.ToInt(thumbnailHeight)),
		ThumnailWidth:  int32(Utility.ToInt(thumbnailWidth)),
	}

	stream, err := client.c.GetThumbnails(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	// Here I will create the final array
	data := make([]byte, 0)
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			// end of stream...
			break
		} else if err != nil {
			return "", err
		}

		data = append(data, msg.Data...)
		if err != nil {
			return "", err
		}
	}

	return string(data), nil
}

func (client *File_Client) HtmlToPdf(html string) ([]byte, error) {
	rqst := &filepb.HtmlToPdfRqst{
		Html: html,
	}

	rsp, err := client.c.HtmlToPdf(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}
	return rsp.Pdf, nil
}
