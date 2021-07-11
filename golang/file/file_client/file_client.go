package file_client

import (
	"io"
	"log"
	"os"
	"strconv"

	"context"

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
func NewFileService_Client(address string, id string) (*File_Client, error) {
	client := new(File_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = filepb.NewFileServiceClient(client.cc)

	return client, nil
}

func (file_client *File_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(file_client)
	}
	return globular.InvokeClientRequest(file_client.c, ctx, method, rqst)
}

// Return the domain
func (file_client *File_Client) GetDomain() string {
	return file_client.domain
}

// Return the address
func (file_client *File_Client) GetAddress() string {
	return file_client.domain + ":" + strconv.Itoa(file_client.port)
}

// Return the id of the service instance
func (file_client *File_Client) GetId() string {
	return file_client.id
}

// Return the name of the service
func (file_client *File_Client) GetName() string {
	return file_client.name
}

func (file_client *File_Client) GetMac() string {
	return file_client.mac
}


// must be close when no more needed.
func (file_client *File_Client) Close() {
	file_client.cc.Close()
}

// Set grpc_service port.
func (file_client *File_Client) SetPort(port int) {
	file_client.port = port
}

// Set the client instance id.
func (file_client *File_Client) SetId(id string) {
	file_client.id = id
}

// Set the client name.
func (file_client *File_Client) SetName(name string) {
	file_client.name = name
}

func (file_client *File_Client) SetMac(mac string) {
	file_client.name = mac
}


// Set the domain.
func (file_client *File_Client) SetDomain(domain string) {
	file_client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (file_client *File_Client) HasTLS() bool {
	return file_client.hasTLS
}

// Get the TLS certificate file path
func (file_client *File_Client) GetCertFile() string {
	return file_client.certFile
}

// Get the TLS key file path
func (file_client *File_Client) GetKeyFile() string {
	return file_client.keyFile
}

// Get the TLS key file path
func (file_client *File_Client) GetCaFile() string {
	return file_client.caFile
}

// Set the client is a secure client.
func (file_client *File_Client) SetTLS(hasTls bool) {
	file_client.hasTLS = hasTls
}

// Set TLS certificate file path
func (file_client *File_Client) SetCertFile(certFile string) {
	file_client.certFile = certFile
}

// Set TLS key file path
func (file_client *File_Client) SetKeyFile(keyFile string) {
	file_client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (file_client *File_Client) SetCaFile(caFile string) {
	file_client.caFile = caFile
}

///////////////////// API //////////////////////

// Stop the service.
func (file_client *File_Client) StopService() {
	file_client.c.Stop(globular.GetClientContext(file_client), &filepb.StopRequest{})
}

// Read the content of a dir and return it info.
func (file_client *File_Client) ReadDir(path interface{}, recursive interface{}, thumbnailHeight interface{}, thumbnailWidth interface{}) (string, error) {

	// Create a new client service...
	rqst := &filepb.ReadDirRequest{
		Path:           Utility.ToString(path),
		Recursive:      Utility.ToBool(recursive),
		ThumnailHeight: int32(Utility.ToInt(thumbnailHeight)),
		ThumnailWidth:  int32(Utility.ToInt(thumbnailWidth)),
	}

	stream, err := file_client.c.ReadDir(globular.GetClientContext(file_client), rqst)
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
func (file_client *File_Client) CreateDir(path interface{}, name interface{}) error {

	rqst := &filepb.CreateDirRequest{
		Path: Utility.ToString(path),
		Name: Utility.ToString(name),
	}

	_, err := file_client.c.CreateDir(globular.GetClientContext(file_client), rqst)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Read file data
 */
func (file_client *File_Client) ReadFile(path interface{}) ([]byte, error) {

	rqst := &filepb.ReadFileRequest{
		Path: Utility.ToString(path),
	}

	stream, err := file_client.c.ReadFile(globular.GetClientContext(file_client), rqst)
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
func (file_client *File_Client) RenameDir(path interface{}, oldname interface{}, newname interface{}) error {

	rqst := &filepb.RenameRequest{
		Path:    Utility.ToString(path),
		OldName: Utility.ToString(oldname),
		NewName: Utility.ToString(newname),
	}

	_, err := file_client.c.Rename(globular.GetClientContext(file_client), rqst)

	return err
}

/**
 * Delete a directory
 */
func (file_client *File_Client) DeleteDir(path string) error {
	rqst := &filepb.DeleteDirRequest{
		Path: Utility.ToString(path),
	}

	_, err := file_client.c.DeleteDir(globular.GetClientContext(file_client), rqst)
	return err
}

/**
 * Get a single file info.
 */
func (file_client *File_Client) GetFileInfo(path interface{}, recursive interface{}, thumbnailHeight interface{}, thumbnailWidth interface{}) (string, error) {

	rqst := &filepb.GetFileInfoRequest{
		Path:           Utility.ToString(path),
		ThumnailHeight: int32(Utility.ToInt(thumbnailHeight)),
		ThumnailWidth:  int32(Utility.ToInt(thumbnailWidth)),
	}

	rsp, err := file_client.c.GetFileInfo(globular.GetClientContext(file_client), rqst)
	if err != nil {
		return "", err
	}

	return string(rsp.Data), nil
}

/**
 * That function move a file from a directory to another... (mv) in unix.
 */
func (file_client *File_Client) MoveFile(path interface{}, dest interface{}) error {

	// Open the stream...
	stream, err := file_client.c.SaveFile(globular.GetClientContext(file_client))
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
 * Delete a file whit a given path.
 */
func (file_client *File_Client) DeleteFile(path string) error {

	rqst := &filepb.DeleteFileRequest{
		Path: Utility.ToString(path),
	}

	_, err := file_client.c.DeleteFile(globular.GetClientContext(file_client), rqst)
	if err != nil {
		return err
	}

	return err
}

// Read the content of a dir and return all images as thumbnails.
func (file_client *File_Client) GetThumbnails(path interface{}, recursive interface{}, thumbnailHeight interface{}, thumbnailWidth interface{}) (string, error) {

	// Create a new client service...
	rqst := &filepb.GetThumbnailsRequest{
		Path:           Utility.ToString(path),
		Recursive:      Utility.ToBool(recursive),
		ThumnailHeight: int32(Utility.ToInt(thumbnailHeight)),
		ThumnailWidth:  int32(Utility.ToInt(thumbnailWidth)),
	}

	stream, err := file_client.c.GetThumbnails(globular.GetClientContext(file_client), rqst)
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

func (file_client *File_Client) HtmlToPdf(html string) ([]byte, error) {
	rqst := &filepb.HtmlToPdfRqst{
		Html: html,
	}

	rsp, err := file_client.c.HtmlToPdf(globular.GetClientContext(file_client), rqst)
	if err != nil {
		return nil, err
	}
	return rsp.Pdf, nil
}
