package repository_client

import (
	"strconv"

	"context"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/discovery/discovery_client"
	globular "github.com/globulario/services/golang/globular_client"
	"google.golang.org/grpc"
	"encoding/gob"
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"log"
)

////////////////////////////////////////////////////////////////////////////////
// echo Client Service
////////////////////////////////////////////////////////////////////////////////

type Repository_Service_Client struct {
	cc *grpc.ClientConn
	c  repositorypb.PackageRepositoryClient;

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
		ctx = globular.GetClientContext(client)
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
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
 func (self *Repository_Service_Client) DownloadBundle(descriptor *resourcepb.PackageDescriptor, platform string) (*resourcepb.PackageBundle, error) {

	rqst := &repositorypb.DownloadBundleRequest{
		Descriptor_: descriptor,
		Plaform:     platform,
	}

	stream, err := self.c.DownloadBundle(globular.GetClientContext(self), rqst)
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
func (self *Repository_Service_Client) UploadBundle(discoveryId, serviceId, publisherId, platform, packagePath string) error {
	log.Println("Upload package ", packagePath)
	// The service bundle...
	bundle := new(resourcepb.PackageBundle)
	bundle.Plaform = platform

	// Here I will find the service descriptor from the given information.
	discoveryService, err := discovery_client.NewDiscoveryService_Client(discoveryId, "packages.PackageDiscovery")
	if err != nil {
		return err
	}

	descriptors, err := discoveryService.GetPackageDescriptor(serviceId, publisherId)
	if err != nil {
		return err
	}

	bundle.Descriptor_ = descriptors[0]
	if !Utility.Exists(packagePath) {
		return errors.New("No package found at path " + packagePath)
	}

	/*bundle.Binairies*/
	data, err := ioutil.ReadFile(packagePath)
	if err == nil {
		bundle.Binairies = data
	}

	return self.uploadBundle(bundle)
}

/**
 * Upload a bundle into the service repository.
 */
func (self *Repository_Service_Client) uploadBundle(bundle *resourcepb.PackageBundle) error {

	// Open the stream...
	stream, err := self.c.UploadBundle(globular.GetClientContext(self))
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