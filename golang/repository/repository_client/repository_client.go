package repository_client

import (
	"context"
	"fmt"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config/config_client"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc"

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

	//  keep the last connection state of the client.
	state string

	// The client domain
	domain string

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
func NewRepositoryService_Client(address string, id string) (*Repository_Service_Client, error) {
	client := new(Repository_Service_Client)
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

func (client *Repository_Service_Client) Reconnect() error {

	var err error
	nb_try_connect := 10
	
	for i:=0; i <nb_try_connect; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = repositorypb.NewPackageRepositoryClient(client.cc)
			break
		}
		
		// wait 500 millisecond before next try
		time.Sleep(500 * time.Millisecond)
	}
	
	return err
}

// The address where the client can connect.
func (client *Repository_Service_Client) SetAddress(address string) {
	client.address = address
}

func (client *Repository_Service_Client) GetConfiguration(address, id string) (map[string]interface{}, error) {
	Utility.RegisterFunction("NewConfigService_Client", config_client.NewConfigService_Client)
	client_, err := globular_client.GetClient(address, "config.ConfigService", "NewConfigService_Client")
	if err != nil {
		return nil, err
	}
	return client_.(*config_client.Config_Client).GetServiceConfiguration(id)
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

// Return the last know connection state
func (client *Repository_Service_Client) GetState() string {
	return client.state
}

// Return the domain
func (client *Repository_Service_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Repository_Service_Client) GetAddress() string {
	return client.address
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

// Return the grpc port number
func (client *Repository_Service_Client) GetPort() int {
	return client.port
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

func (client *Repository_Service_Client) SetState(state string) {
	client.state = state
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

// ////////////////////// Resource Client ////////////////////////////////////////////
func GetResourceClient(domain string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(domain, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

/**
 * Upload a service bundle.
 */
func (client *Repository_Service_Client) UploadBundle(token, discoveryId, serviceId, publisherId, version, platform, packagePath string) (int, error) {

	// The service bundle...
	bundle := new(resourcepb.PackageBundle)
	bundle.Plaform = platform

	// Here I will find the service descriptor from the given information.
	resource_client_, err := GetResourceClient(client.address)
	if err != nil {
		resource_client_ = nil
		return -1, err
	}

	descriptor, err := resource_client_.GetPackageDescriptor(serviceId, publisherId, version)
	if err != nil {
		return -1, err
	}

	bundle.PackageDescriptor = descriptor
	if !Utility.Exists(packagePath) {
		return -1, errors.New("No package found at path " + packagePath)
	}

	/*bundle.Binairies*/
	data, err := ioutil.ReadFile(packagePath)
	if err == nil {
		bundle.Binairies = data
	}

	return client.uploadBundle(token, bundle, len(data))
}

/**
 * Upload a bundle into the service repository.
 */
func (client *Repository_Service_Client) uploadBundle(token string, bundle *resourcepb.PackageBundle, total int) (int, error) {

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	// Open the stream...
	stream, err := client.c.UploadBundle(ctx)
	if err != nil {
		return -1, err
	}

	const BufferSize = 1024 * 5 // the chunck size.
	var size int
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer) // Will write to network.
	err = enc.Encode(bundle)
	if err != nil {
		return -1, err
	}
	percent_ := 0
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

		size += bytesread
		if percent_ != int(float64(size)/float64(total)*100) {
			percent_ = int(float64(size) / float64(total) * 100)
			fmt.Println("transfert ", size, "of", total, " ", percent_, "%")
		}

		if err == io.EOF {
			err = nil
			break
		} else if err != nil {
			return -1, err
		}
	}

	_, err = stream.CloseAndRecv()
	if err != nil && err != io.EOF {
		return -1, err
	}

	return size, nil

}

/**
 * Get the rbac client.
 */
func GetRbacClient(domain string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(domain, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func GetEventClient(domain string) (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(domain, "event.EventService", "NewEventService_Client")
	if err != nil {

		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

/**
 *  Create the application bundle and push it on the server
 */
func (client *Repository_Service_Client) UploadApplicationPackage(user, organization, path, token, domain, name, version string) (int, error) {

	path = strings.ReplaceAll(path, "\\", "/")

	if len(token) > 0 {
		claims, _ := security.ValidateToken(token)
		if !strings.Contains(user, "@") {
			user += "@" + claims.UserDomain
		}
	}

	resource_client_, err := GetResourceClient(domain)
	if err != nil {
		return -1, err
	}

	// Retreive the actual application installed version.
	previousVersion, _ := resource_client_.GetApplicationVersion(name)

	publisherId := user
	if len(organization) > 0 {
		publisherId = organization
	}

	// Now I will open the data and create a archive from it.
	// The path of the archive that contain the service package.
	packagePath, err := client.createPackageArchive(publisherId, name, version, "webapp", path)
	if err != nil {
		return -1, err
	}

	defer os.RemoveAll(packagePath)

	rbac_client_, err := GetRbacClient(domain)
	if err != nil {
		return -1, err
	}

	// If the version has change I will notify current users and undate the applications.
	event_client_, err := GetEventClient(domain)
	if err != nil {
		return -1, err
	}

	applications, err := resource_client_.GetApplications(`{"_id":"` + name + `"}`)
	if err != nil {
		return -1, err
	}

	if len(applications) > 0 {
		application := applications[0]

		resource_path := application.Publisherid + "|" + application.Id + "|" + application.Name + "|" + application.Version

		if len(organization) > 0 {
			err = rbac_client_.AddResourceOwner(name+"@"+strings.Split(domain, ":")[0], "application", organization, rbacpb.SubjectType_ORGANIZATION)
			if err != nil {
				return -1, err
			}

			err = rbac_client_.AddResourceOwner(resource_path, "package", organization, rbacpb.SubjectType_ORGANIZATION)
			if err != nil {
				return -1, err
			}

		} else if len(user) > 0 {
			err = rbac_client_.AddResourceOwner(name+"@"+strings.Split(domain, ":")[0], "application", user, rbacpb.SubjectType_ACCOUNT)
			if err != nil {
				return -1, err
			}

			err = rbac_client_.AddResourceOwner(resource_path, "package", user, rbacpb.SubjectType_ACCOUNT)
			if err != nil {
				return -1, err
			}
		}

		// Send application notification...
		event_client_.Publish("update_"+strings.Split(domain, ":")[0]+"_"+name+"_evt", []byte(version))
		if previousVersion != version {
			message := `<div style="display: flex; flex-direction: column">
				  <div>A new version of <span style="font-weight: 500;">` + application.Alias + `</span> (v.` + version + `) is available.
				  </div>
				  <div>
					Press <span style="font-weight: 500;">f5</span> to refresh the page.
				  </div>
				</div>
				`

			// That service made user of persistence service.
			notification := new(resourcepb.Notification)
			notification.Id = Utility.RandomUUID()
			notification.NotificationType = resourcepb.NotificationType_APPLICATION_NOTIFICATION
			notification.Message = message
			notification.Recipient = application.Id
			notification.Date = time.Now().Unix()
			notification.Mac, _= Utility.MyMacAddr(Utility.MyIP())

			notification.Sender = `{"_id":"` + application.Id + `", "name":"` + application.Name + `","icon":"` + application.Icon + `", "alias":"` + application.Alias + `"}`

			err = resource_client_.CreateNotification(notification)
			if err != nil {
				return -1, err
			}

			var marshaler jsonpb.Marshaler
			jsonStr, err := marshaler.MarshalToString(notification)
			if err != nil {
				return -1, err
			}

			err = event_client_.Publish(application.Id+"_notification_event", []byte(jsonStr))
			if err != nil {
				return -1, err
			}
		}
	}

	// Upload the bundle to the repository server.
	return client.UploadBundle(token, domain, name, publisherId, version, "webapp", packagePath)

}

/**
 * Create the service bundle and push it on the server
 */
func (client *Repository_Service_Client) UploadServicePackage(user string, organization string, token string, domain string, path string, platform string) error {

	// Here I will try to read the service configuation from the path.
	configs, _ := Utility.FindFileByName(path, "config.json")
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

	// Find proto by name
	if !Utility.Exists(s["Proto"].(string)) {
		return errors.New("No prototype file was found at path '" + s["Proto"].(string) + "'")
	}

	// set the correct information inside the configuration
	if len(token) > 0 {
		claims, err := security.ValidateToken(token)
		if err != nil {
			return err
		}
		if !strings.Contains(user, "@") {
			user += "@" + claims.UserDomain
		}
	}

	publisherId := user
	if len(organization) > 0 {
		publisherId = organization
	}

	s["PublisherId"] = publisherId

	jsonStr, _ := Utility.ToJson(&s)
	ioutil.WriteFile(configs[0], []byte(jsonStr), 0644)

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
	proto := strings.ReplaceAll(s["Proto"].(string), "\\", "/")
	err = Utility.CopyFile(proto, tmp_dir+"/"+s["PublisherId"].(string)+"/"+s["Name"].(string)+"/"+s["Version"].(string)+"/"+proto[strings.LastIndex(proto, "/"):])
	if err != nil {
		return err
	}

	// The path of the archive that contain the service package.
	packagePath, err := client.createPackageArchive(s["PublisherId"].(string), s["Id"].(string), s["Version"].(string), platform, tmp_dir)
	if err != nil {
		return err
	}

	// Remove the file when it's transfer on the server...
	defer os.RemoveAll(packagePath)
	// fmt.Println("-------------------> ", packagePath)

	// Upload the bundle to the repository server.
	_, err = client.UploadBundle(token, domain, s["Id"].(string), s["PublisherId"].(string), s["Version"].(string), platform, packagePath)
	if err != nil {
		return err
	}

	rbac_client_, err := GetRbacClient(domain)
	if err != nil {
		return err
	}

	resource_path := s["PublisherId"].(string) + "|" + s["Id"].(string) + "|" + s["Name"].(string) + "|" + s["Version"].(string)

	if len(organization) > 0 {

		err = rbac_client_.AddResourceOwner(resource_path, "package", organization, rbacpb.SubjectType_ORGANIZATION)
		if err != nil {
			return err
		}

	} else if len(user) > 0 {
		fmt.Println("add resource owner ", resource_path, user)
		err = rbac_client_.AddResourceOwner(resource_path, "package", user, rbacpb.SubjectType_ACCOUNT)
		if err != nil {
			return err
		}
	}

	return nil
}

/** Create a service package **/
func (client *Repository_Service_Client) createPackageArchive(publisherId string, id string, version string, platform string, path string) (string, error) {

	// Take the information from the configuration...
	archive_name := id + "%" + version + "%" + id + "%" + platform

	// tar + gzip
	var buf bytes.Buffer
	Utility.CompressDir(path, &buf)

	// write the .tar.gzip
	fileToWrite, err := os.OpenFile(os.TempDir()+string(os.PathSeparator)+archive_name+".tar.gz", os.O_CREATE|os.O_RDWR, os.FileMode(0755))
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
	return os.TempDir() + string(os.PathSeparator) + archive_name + ".tar.gz", nil
}
