package repository_client

import (
	"strconv"

	"context"

	"github.com/davecourtois/Utility"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"

	"bufio"
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// echo Client Service
////////////////////////////////////////////////////////////////////////////////

type Repository_Service_Client struct {
	cc *grpc.ClientConn
	c  repositorypb.PackageRepositoryClient

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

	// The client context
	ctx context.Context
}

// Create a connection to the service.
func NewRepositoryService_Client(address string, id string) (*Repository_Service_Client, error) {
	client := new(Repository_Service_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = repositorypb.NewPackageRepositoryClient(client.cc)

	return client, nil
}

func (client *Repository_Service_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Repository_Service_Client) GetCtx() context.Context {
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
func (client *Repository_Service_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Repository_Service_Client) GetAddress() string {
	return client.domain + ":" + strconv.Itoa(client.port)
}

// Return the id of the service instance
func (client *Repository_Service_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Repository_Service_Client) GetName() string {
	return client.name
}

func (client *Repository_Service_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Repository_Service_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Repository_Service_Client) SetPort(port int) {
	client.port = port
}

// Set the client instance id.
func (client *Repository_Service_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Repository_Service_Client) SetName(name string) {
	client.name = name
}

func (client *Repository_Service_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Repository_Service_Client) SetDomain(domain string) {
	client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Repository_Service_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Repository_Service_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Repository_Service_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Repository_Service_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Repository_Service_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Repository_Service_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Repository_Service_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Repository_Service_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

////////////////// Api //////////////////////

/**
 * Download bundle from a repository and return it as an object in memory.
 */
func (client *Repository_Service_Client) DownloadBundle(descriptor *resourcepb.PackageDescriptor, platform string) (*resourcepb.PackageBundle, error) {

	rqst := &repositorypb.DownloadBundleRequest{
		Descriptor_: descriptor,
		Plaform:     platform,
	}

	stream, err := client.c.DownloadBundle(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	// Here I will create the final array
	var buffer bytes.Buffer
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			// end of stream...
			break
		}
		if err != nil {
			return nil, err
		}

		_, err = buffer.Write(msg.Data)
		if err != nil {
			return nil, err
		}
	}

	// The buffer that contain the
	dec := gob.NewDecoder(&buffer)
	bundle := new(resourcepb.PackageBundle)
	err = dec.Decode(bundle)
	if err != nil {
		return nil, err
	}

	return bundle, err
}

/**
 * Upload a service bundle.
 */
func (client *Repository_Service_Client) UploadBundle(discoveryId, serviceId, publisherId, version, platform, packagePath string) error {

	// The service bundle...
	bundle := new(resourcepb.PackageBundle)
	bundle.Plaform = platform

	// Here I will find the service descriptor from the given information.
	resource_client_, err := resource_client.NewResourceService_Client(client.domain, "resource.ResourceService")
	if err != nil {
		resource_client_ = nil
		return err
	}

	descriptor, err := resource_client_.GetPackageDescriptor(serviceId, publisherId, version)
	if err != nil {
		return err
	}

	bundle.PackageDescriptor = descriptor
	if !Utility.Exists(packagePath) {
		return errors.New("No package found at path " + packagePath)
	}

	/*bundle.Binairies*/
	data, err := ioutil.ReadFile(packagePath)
	if err == nil {
		bundle.Binairies = data
	}

	return client.uploadBundle(bundle)
}

/**
 * Upload a bundle into the service repository.
 */
func (client *Repository_Service_Client) uploadBundle(bundle *resourcepb.PackageBundle) error {

	// Open the stream...
	stream, err := client.c.UploadBundle(client.GetCtx())
	if err != nil {
		return err
	}

	const BufferSize = 1024 * 5 // the chunck size.
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer) // Will write to network.
	err = enc.Encode(bundle)
	if err != nil {
		return err
	}

	for {
		var data [BufferSize]byte
		bytesread, err := buffer.Read(data[0:BufferSize])
		if bytesread > 0 {
			rqst := &repositorypb.UploadBundleRequest{
				Data: data[0:bytesread],
			}
			// send the data to the server.
			err = stream.Send(rqst)
		}

		if err == io.EOF {
			err = nil
			break
		} else if err != nil {
			return err
		}
	}

	stream.CloseAndRecv()
	return nil

}

/**
 * Create and Upload the service archive on the server.
 */
func (client *Repository_Service_Client) UploadServicePackage(user string, organization string, token string, domain string, path string, platform string) error {

	// Here I will try to read the service configuation from the path.
	configs, _ := Utility.FindFileByName(path, "config.json")
	if len(configs) == 0 {
		return errors.New("no configuration file was found")
	}

	// Find proto by name
	protos, _ := Utility.FindFileByName(path, ".proto")
	if len(protos) == 0 {
		return errors.New("No prototype file was found at path '" + path + "'")
	}

	s := make(map[string]interface{})
	data, err := ioutil.ReadFile(configs[0])
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &s)
	if err != nil {
		return err
	}

	// set the correct information inside the configuration
	publisherId := user
	if len(organization) > 0 {
		publisherId = organization
	}

	s["PublisherId"] = publisherId

	jsonStr, _ := Utility.ToJson(&s)
	ioutil.WriteFile(configs[0], []byte(jsonStr), 0644)

	md := metadata.New(map[string]string{"token": token, "domain": domain})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	// First of all I will create the archive for the service.
	// If a path is given I will take it entire content. If not
	// the proto, the config and the executable only will be taken.

	// So here I will create set the good file structure in a temp directory and
	// copy file in it that will be the bundle to be use...
	tmp_dir := strings.ReplaceAll(os.TempDir(), "\\", "/") + "/" + s["PublisherId"].(string) + "%" + s["Name"].(string) + "%" + s["Version"].(string) + "%" + s["Id"].(string) + "%" + platform
	path_ := tmp_dir + "/" + s["PublisherId"].(string) + "/" + s["Name"].(string) + "/" + s["Version"].(string) + "/" + s["Id"].(string)
	defer os.RemoveAll(tmp_dir)

	// I will create the directory
	Utility.CreateDirIfNotExist(path)

	// Now I will copy the content of the given path into it...
	err = Utility.CopyDir(path+"/.", path_)

	if err != nil {
		return err
	}

	// Now I will copy the proto file into the directory Version
	proto := strings.ReplaceAll(protos[0], "\\", "/")
	err = Utility.CopyFile(proto, tmp_dir+"/"+s["PublisherId"].(string)+"/"+s["Name"].(string)+"/"+s["Version"].(string)+"/"+proto[strings.LastIndex(proto, "/"):])
	if err != nil {
		return err
	}

	packagePath, err := client.createServicePackage(s["PublisherId"].(string), s["Name"].(string), s["Id"].(string), s["Version"].(string), platform, tmp_dir)
	if err != nil {
		return err
	}

	// Remove the file when it's transfer on the server...
	defer os.RemoveAll(packagePath)

	// Read the package data.
	packageFile, err := os.Open(packagePath)
	if err != nil {
		return err
	}
	defer packageFile.Close()

	// Now I will create the request to upload the package on the server.
	// Open the stream...
	stream, err := client.c.UploadBundle(ctx)
	if err != nil {
		return err
	}

	const chunksize = 1024 * 5 // the chunck size.
	var count int
	reader := bufio.NewReader(packageFile)
	part := make([]byte, chunksize)
	size := 0
	for {

		if count, err = reader.Read(part); err != nil {
			break
		}

		rqst := &repositorypb.UploadBundleRequest{
			Data: part[:count],
		}

		// send the data to the server.
		err = stream.Send(rqst)
		size += count

		if err == io.EOF {
			err = nil
			break
		} else if err != nil {

			return err
		}

	}

	// get the file path on the server where the package is store before being
	// publish.
	_, err = stream.CloseAndRecv()
	if err != nil {
		if err != io.EOF {
			return err
		}
	}
	return nil
}

/** Create a service package **/
func (client *Repository_Service_Client) createServicePackage(publisherId string, serviceName string, serviceId string, version string, platform string, servicePath string) (string, error) {

	// Take the information from the configuration...
	id := publisherId + "%" + serviceName + "%" + version + "%" + serviceId + "%" + platform

	// tar + gzip
	var buf bytes.Buffer
	Utility.CompressDir(servicePath, &buf)

	// write the .tar.gzip
	fileToWrite, err := os.OpenFile(os.TempDir()+string(os.PathSeparator)+id+".tar.gz", os.O_CREATE|os.O_RDWR, os.FileMode(0755))
	if err != nil {
		return "", err
	}
	// close the file.
	defer fileToWrite.Close()

	if _, err := io.Copy(fileToWrite, &buf); err != nil {
		return "", err
	}


	if err != nil {
		return "", err
	}
	return os.TempDir() + string(os.PathSeparator) + id + ".tar.gz", nil
}
