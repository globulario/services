package discovery_client

import (
	"context"
	"strconv"

	"encoding/json"
	"errors"
	"io/ioutil"
	"log"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/discovery/discoverypb"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// echo Client Service
////////////////////////////////////////////////////////////////////////////////

type Dicovery_Client struct {
	cc *grpc.ClientConn
	c  discoverypb.PackageDiscoveryClient

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
func NewDiscoveryService_Client(address string, id string) (*Dicovery_Client, error) {
	client := new(Dicovery_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}

	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}

	client.c = discoverypb.NewPackageDiscoveryClient(client.cc)

	return client, nil
}

func (client *Dicovery_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(client)
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

// Return the domain
func (client *Dicovery_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Dicovery_Client) GetAddress() string {
	return client.domain + ":" + strconv.Itoa(client.port)
}

// Return the id of the service instance
func (client *Dicovery_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Dicovery_Client) GetName() string {
	return client.name
}

// must be close when no more needed.
func (client *Dicovery_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Dicovery_Client) SetPort(port int) {
	client.port = port
}

// Set the client instance id.
func (client *Dicovery_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Dicovery_Client) SetName(name string) {
	client.name = name
}

// Set the domain.
func (client *Dicovery_Client) SetDomain(domain string) {
	client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Dicovery_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Dicovery_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Dicovery_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Dicovery_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Dicovery_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Dicovery_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Dicovery_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Dicovery_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

////////////////// Api //////////////////////

/**
 * Create and Upload the service archive on the server.
 */
/*
func (admin_client *Admin_Client) UploadServicePackage(user string, organization string, token string, domain string, path string, platform string) (string, int, error) {

	// Here I will try to read the service configuation from the path.
	configs, _ := Utility.FindFileByName(path, "config.json")
	if len(configs) == 0 {
		return "", 0, errors.New("no configuration file was found")
	}

	// Find proto by name
	protos, _ := Utility.FindFileByName(path, ".proto")
	if len(protos) == 0 {
		return "", 0, errors.New("No prototype file was found at path '" + path + "'")
	}

	s := make(map[string]interface{})
	data, err := ioutil.ReadFile(configs[0])
	if err != nil {
		return "", 0, err
	}

	err = json.Unmarshal(data, &s)
	if err != nil {
		return "", 0, err
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
		return "", 0, err
	}

	// Now I will copy the proto file into the directory Version
	proto := strings.ReplaceAll(protos[0], "\\", "/")
	err = Utility.CopyFile(proto, tmp_dir+"/"+s["PublisherId"].(string)+"/"+s["Name"].(string)+"/"+s["Version"].(string)+"/"+proto[strings.LastIndex(proto, "/"):])
	if err != nil {
		log.Println(err)
		return "", 0, err
	}

	packagePath, err := admin_client.createServicePackage(s["PublisherId"].(string), s["Name"].(string), s["Id"].(string), s["Version"].(string), platform, tmp_dir)
	if err != nil {
		return "", 0, err
	}

	// Remove the file when it's transfer on the server...
	defer os.RemoveAll(packagePath)

	// Read the package data.
	packageFile, err := os.Open(packagePath)
	if err != nil {
		return "", 0, err
	}
	defer packageFile.Close()

	// Now I will create the request to upload the package on the server.
	// Open the stream...
	stream, err := admin_client.c.UploadServicePackage(ctx)
	if err != nil {
		return "", 0, err
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

		rqst := &adminpb.UploadServicePackageRequest{
			User:         user,
			Organization: organization,
			Data:         part[:count],
		}

		// send the data to the server.
		err = stream.Send(rqst)
		size += count

		if err == io.EOF {
			err = nil
			break
		} else if err != nil {

			return "", 0, err
		}

	}

	// get the file path on the server where the package is store before being
	// publish.
	rsp, err := stream.CloseAndRecv()
	if err != nil {
		if err != io.EOF {
			return "", 0, err
		}
	}
	return rsp.Path, size, nil
}
*/

/**
 * Publish a service from a runing globular server.
 */
func (Services_Manager_Client *Dicovery_Client) PublishService(user, organization, token, domain, configPath, platform string) error {

	// Here I will try to read the service configuation from the path.
	configs, _ := Utility.FindFileByName(configPath, "config.json")
	if len(configs) == 0 {
		return errors.New("no configuration file was found")
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

	keywords := make([]string, 0)
	if s["Keywords"] != nil {
		for i := 0; i < len(s["Keywords"].([]interface{})); i++ {
			keywords = append(keywords, s["Keywords"].([]interface{})[i].(string))
		}
	}
	if s["Repositories"] == nil {
		s["Repositories"] = []interface{}{"localhost"}
	}
	repositories := s["Repositories"].([]interface{})
	if len(repositories) == 0 {
		repositories = []interface{}{"localhost"}

	}

	if s["Discoveries"] == nil {
		return errors.New("no discovery was set on that server")
	}

	discoveries := s["Discoveries"].([]interface{})

	for i := 0; i < len(discoveries); i++ {
		rqst := new(discoverypb.PublishServiceRequest)
		rqst.User = user
		rqst.Organization = organization
		rqst.Description = s["Description"].(string)
		rqst.DicorveryId = discoveries[i].(string)
		rqst.RepositoryId = repositories[0].(string)
		rqst.Keywords = keywords
		rqst.Version = s["Version"].(string)
		rqst.ServiceId = s["Id"].(string)
		rqst.ServiceName = s["Name"].(string)
		rqst.Platform = platform

		// Set the token into the context and send the request.
		ctx := globular.GetClientContext(Services_Manager_Client)
		if len(token) > 0 {
			md, _ := metadata.FromOutgoingContext(ctx)

			if len(md.Get("token")) != 0 {
				md.Set("token", token)
			}
			ctx = metadata.NewOutgoingContext(context.Background(), md)
		}

		_, err = Services_Manager_Client.c.PublishService(ctx, rqst)
		if err != nil {
			log.Println("fail to publish service at ", discoveries[i], err)
		}
	}

	return nil
}

/**
 * Publish an application on the server.
 */
func (client *Dicovery_Client) PublishApplication(user, organization, path, name, domain, version, description, icon, alias, repositoryId, discoveryId string, actions, keywords []string, roles []*resourcepb.Role, groups []*resourcepb.Group) error {
	// TODO upload the package and publish the application after see old admin client code bundle from the path...

	rqst := &discoverypb.PublishApplicationRequest{
		User:         user,
		Organization: organization,
		Name:         name,
		Domain:       domain,
		Version:      version,
		Description:  description,
		Icon:         icon,
		Alias:        alias,
		Repository:   repositoryId,
		Discovery:    discoveryId,
		Actions:      actions,
		Keywords:     keywords,
		Roles:        roles,
		Path:         path,
		Groups: 	  groups,
	}

	_, err := client.c.PublishApplication(globular.GetClientContext(client), rqst)

	return err
}
