package repository_client

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"github.com/schollz/progressbar/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
)

////////////////////////////////////////////////////////////////////////////////
// Repository Client
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
	if err := globular_client.InitClient(client, address, id); err != nil {
		return nil, err
	}
	if err := client.Reconnect(); err != nil {
		return nil, err
	}
	return client, nil
}

func (client *Repository_Service_Client) Reconnect() error {
	var err error
	const nbTry = 10
	for i := 0; i < nbTry; i++ {
		client.cc, err = globular_client.GetClientConnection(client)
		if err == nil {
			client.c = repositorypb.NewPackageRepositoryClient(client.cc)
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

// The address where the client can connect.
func (client *Repository_Service_Client) SetAddress(address string) { client.address = address }

func (client *Repository_Service_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular_client.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Repository_Service_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular_client.GetClientContext(client)
	}
	if token, err := security.GetLocalToken(client.GetMac()); err == nil {
		md := metadata.New(map[string]string{
			"token":   string(token),
			"domain":  client.domain,
			"mac":     client.GetMac(),
			"address": client.GetAddress(),
		})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	return client.ctx
}

// Return the last know connection state
func (client *Repository_Service_Client) GetState() string   { return client.state }
func (client *Repository_Service_Client) GetDomain() string  { return client.domain }
func (client *Repository_Service_Client) GetAddress() string { return client.address }
func (client *Repository_Service_Client) GetId() string      { return client.id }
func (client *Repository_Service_Client) GetName() string    { return client.name }
func (client *Repository_Service_Client) GetMac() string     { return client.mac }

// must be close when no more needed.
func (client *Repository_Service_Client) Close() { client.cc.Close() }

// Set grpc_service port.
func (client *Repository_Service_Client) SetPort(port int) { client.port = port }

// Return the grpc port number
func (client *Repository_Service_Client) GetPort() int { return client.port }

// Set the client instance id.
func (client *Repository_Service_Client) SetId(id string)         { client.id = id }
func (client *Repository_Service_Client) SetName(name string)     { client.name = name }
func (client *Repository_Service_Client) SetMac(mac string)       { client.mac = mac }
func (client *Repository_Service_Client) SetDomain(domain string) { client.domain = domain }
func (client *Repository_Service_Client) SetState(state string)   { client.state = state }

////////////////// TLS ///////////////////

func (client *Repository_Service_Client) HasTLS() bool                { return client.hasTLS }
func (client *Repository_Service_Client) GetCertFile() string         { return client.certFile }
func (client *Repository_Service_Client) GetKeyFile() string          { return client.keyFile }
func (client *Repository_Service_Client) GetCaFile() string           { return client.caFile }
func (client *Repository_Service_Client) SetTLS(hasTls bool)          { client.hasTLS = hasTls }
func (client *Repository_Service_Client) SetCertFile(certFile string) { client.certFile = certFile }
func (client *Repository_Service_Client) SetKeyFile(keyFile string)   { client.keyFile = keyFile }
func (client *Repository_Service_Client) SetCaFile(caFile string)     { client.caFile = caFile }

////////////////// Api //////////////////////

/**
 * Download bundle from a repository and return it as an object in memory.
 */
func (client *Repository_Service_Client) DownloadBundle(descriptor *resourcepb.PackageDescriptor, platform string) (*resourcepb.PackageBundle, error) {
	rqst := &repositorypb.DownloadBundleRequest{
		Descriptor_: descriptor,
		Platform:    platform,
	}
	stream, err := client.c.DownloadBundle(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if _, err = buffer.Write(msg.Data); err != nil {
			return nil, err
		}
	}

	dec := gob.NewDecoder(&buffer)
	bundle := new(resourcepb.PackageBundle)
	if err := dec.Decode(bundle); err != nil {
		return nil, err
	}
	return bundle, nil
}

// ////////////////////// Resource Client ////////////////////////////////////////////
func getResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

/**
 * Upload a service bundle.
 */
func (client *Repository_Service_Client) UploadBundle(token, discoveryId, serviceId, PublisherID, version, platform, packagePath string) (int, error) {
	bundle := new(resourcepb.PackageBundle)
	bundle.Plaform = platform

	resource_client_, err := getResourceClient(client.address)
	if err != nil {
		return -1, err
	}

	descriptor, err := resource_client_.GetPackageDescriptor(serviceId, PublisherID, version)
	if err != nil {
		return -1, err
	}
	bundle.PackageDescriptor = descriptor

	if !Utility.Exists(packagePath) {
		return -1, errors.New("No package found at path " + packagePath)
	}

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

	stream, err := client.c.UploadBundle(ctx)
	if err != nil {
		return -1, err
	}

	const BufferSize = 1024 * 5
	var size int
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	if err := enc.Encode(bundle); err != nil {
		return -1, err
	}

	percent_ := 0
	bar := progressbar.Default(100)
	for {
		var data [BufferSize]byte
		bytesread, rerr := buffer.Read(data[0:BufferSize])
		if bytesread > 0 {
			if err := stream.Send(&repositorypb.UploadBundleRequest{Data: data[0:bytesread]}); err != nil {
				return -1, err
			}
		}
		size += bytesread
		if total > 0 {
			next := int(float64(size) / float64(total) * 100)
			if next != percent_ {
				percent_ = next
				bar.Set(percent_)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return -1, rerr
		}
	}

	if _, err := stream.CloseAndRecv(); err != nil && err != io.EOF {
		return -1, err
	}
	return size, nil
}

func (client *Repository_Service_Client) DownloadArtifact(ref *repositorypb.ArtifactRef) ([]byte, error) {
	if ref == nil {
		return nil, errors.New("artifact ref required")
	}
	stream, err := client.c.DownloadArtifact(client.GetCtx(), &repositorypb.DownloadArtifactRequest{Ref: ref})
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if _, err := buf.Write(msg.GetData()); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func (client *Repository_Service_Client) UploadArtifact(ref *repositorypb.ArtifactRef, data []byte) error {
	if ref == nil {
		return errors.New("artifact ref required")
	}
	stream, err := client.c.UploadArtifact(client.GetCtx())
	if err != nil {
		return err
	}
	if err := stream.Send(&repositorypb.UploadArtifactRequest{
		Ref:  ref,
		Data: data,
	}); err != nil {
		return err
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return err
	}
	if resp == nil || !resp.GetResult() {
		return fmt.Errorf("artifact upload failed")
	}
	return nil
}

/**
 * Get the rbac client.
 */
func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func getEventClient(address string) (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

/**
 *  Create the application bundle and push it on the server
 */
func (client *Repository_Service_Client) UploadApplicationPackage(user, organization, path, token, address, name, version string) (int, error) {
	path = strings.ReplaceAll(path, "\\", "/")

	if !strings.Contains(user, "@") {
		user += "@" + client.GetDomain()
	}

	resource_client_, err := getResourceClient(address)
	if err != nil {
		return -1, err
	}

	PublisherID := user
	if len(organization) > 0 {
		PublisherID = organization
	}

	packagePath, err := client.createPackageArchive(PublisherID, name, version, "webapp", path)
	if err != nil {
		return -1, err
	}
	defer os.RemoveAll(packagePath)

	rbac_client_, err := GetRbacClient(address)
	if err != nil {
		return -1, err
	}

	resource_path := PublisherID + "|" + name + "|" + version

	if len(organization) > 0 {
		if err := rbac_client_.AddResourceOwner(token, resource_path, organization, "package", rbacpb.SubjectType_ORGANIZATION); err != nil {
			return -1, err
		}
	} else if len(user) > 0 {
		if err := rbac_client_.AddResourceOwner(token, resource_path, user, "package", rbacpb.SubjectType_ACCOUNT); err != nil {
			return -1, err
		}
	}

	applicationId := Utility.GenerateUUID(PublisherID + "%" + name + "%" + version)
	applications, _ := resource_client_.GetApplications(`{"_id":"` + applicationId + `"}`)

	if len(applications) > 0 {
		event_client_, err := getEventClient(address)
		if err != nil {
			return -1, err
		}

		application := applications[0]
		previousVersion, _ := resource_client_.GetApplicationVersion(name)

		event_client_.Publish("update_"+applicationId+"_evt", []byte(version))

		if previousVersion != version {
			message := `<div style="display: flex; flex-direction: column">
				  <div>A new version of <span style="font-weight: 500;">` + application.Alias + `</span> (v.` + version + `) is available.</div>
				  <div>Press <span style="font-weight: 500;">f5</span> to refresh the page.</div>
				</div>`

			notification := new(resourcepb.Notification)
			notification.Id = Utility.RandomUUID()
			notification.NotificationType = resourcepb.NotificationType_APPLICATION_NOTIFICATION
			notification.Message = message
			notification.Recipient = application.Name
			notification.Date = time.Now().Unix()
			notification.Mac, _ = config.GetMacAddress()
			notification.Sender = `{"_id":"` + application.Id + `", "name":"` + application.Name + `","icon":"` + application.Icon + `", "alias":"` + application.Alias + `"}`

			if err := resource_client_.CreateNotification(notification); err != nil {
				return -1, err
			}
			if jsonStr, err := protojson.Marshal(notification); err == nil {
				if err := event_client_.Publish(application.Id+"_notification_event", []byte(jsonStr)); err != nil {
					return -1, err
				}
			} else {
				return -1, err
			}
		}
	}

	return client.UploadBundle(token, address, name, PublisherID, version, "webapp", packagePath)
}

/**
 * Create the service bundle and push it on the server
 *
 * `path` can be either:
 *   - a service Id or Name present in etcd (preferred), or
 *   - a legacy folder path (fallback) containing the service files.
 */
func (client *Repository_Service_Client) UploadServicePackage(user, organization, token, domain, path string, platform string) error {
	path = strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")

	// Resolve service configuration from etcd first
	var s map[string]interface{}
	if cfg, err := config.GetServiceConfigurationById(path); err == nil && cfg != nil {
		s = cfg
	} else if Utility.Exists(path) {
		// Legacy fallback: use a directory path; we still need enough fields in `s`
		// Try to infer from directory name when possible
		base := filepath.Base(path)
		s = map[string]interface{}{
			"Id":          base,
			"Name":        base,
			"Version":     "0.0.0",
			"PublisherID": "",
			"Path":        path, // we'll treat this as a folder root below
		}
	} else {
		return errors.New("service not found in etcd and path does not exist")
	}

	// Ensure required fields
	id := Utility.ToString(s["Id"])
	name := Utility.ToString(s["Name"])
	version := Utility.ToString(s["Version"])
	execPath := Utility.ToString(s["Path"])
	protoPath := Utility.ToString(s["Proto"])

	// PublisherID resolution (etcd may already have it)
	pubID := Utility.ToString(s["PublisherID"])
	if pubID == "" {
		if len(organization) > 0 {
			pubID = organization
		} else {
			if !strings.Contains(user, "@") {
				if len(token) > 0 {
					claims, err := security.ValidateToken(token)
					if err != nil {
						return err
					}
					if claims.UserDomain == "" {
						return errors.New("no user domain found in token")
					}
					user += "@" + claims.UserDomain
				}
			}
			pubID = user
		}
	}

	// Determine source directory to package
	var srcDir string
	info, err := os.Stat(path)
	if Utility.Exists(path) && err == nil && info.IsDir() {
		srcDir = path
	} else if execPath != "" && Utility.Exists(execPath) {
		srcDir = filepath.Dir(execPath)
	} else {
		return errors.New("cannot determine service source directory to package")
	}

	// Validate proto (try absolute first; if missing, try same dir by name guess)
	if protoPath == "" || !Utility.Exists(protoPath) {
		guess := filepath.Join(srcDir, name+".proto")
		if Utility.Exists(guess) {
			protoPath = guess
		} else {
			return errors.New("proto file not found; set 'Proto' in etcd or place " + name + ".proto in service directory")
		}
	}

	// Temp bundle layout: <tmp>/<pub>/<name>/<version>/<id>
	tmpDir := strings.ReplaceAll(os.TempDir(), "\\", "/") + "/" + pubID + "%" + name + "%" + version + "%" + id + "%" + platform
	destRoot := tmpDir + "/" + pubID + "/" + name + "/" + version + "/" + id
	defer os.RemoveAll(tmpDir)

	if err := Utility.CreateDirIfNotExist(destRoot); err != nil {
		return err
	}

	// Copy service folder contents
	if err := Utility.CopyDir(srcDir+"/.", destRoot); err != nil {
		return err
	}

	// Copy proto under <pub>/<name>/<version>/
	protoDstDir := filepath.Join(tmpDir, pubID, name, version)
	if err := Utility.CreateDirIfNotExist(strings.ReplaceAll(protoDstDir, "\\", "/")); err != nil {
		return err
	}
	if err := Utility.CopyFile(protoPath, filepath.Join(protoDstDir, filepath.Base(protoPath))); err != nil {
		return err
	}

	// Create archive and upload
	packagePath, err := client.createPackageArchive(pubID, id, version, platform, tmpDir)
	if err != nil {
		return err
	}
	defer os.RemoveAll(packagePath)

	if _, err := client.UploadBundle(token, domain, id, pubID, version, platform, packagePath); err != nil {
		return err
	}

	// RBAC ownership
	rbac_client_, err := GetRbacClient(domain)
	if err != nil {
		return err
	}
	resourcePath := pubID + "|" + id + "|" + name + "|" + version
	if organization != "" {
		if err := rbac_client_.AddResourceOwner(token, resourcePath, organization, "package", rbacpb.SubjectType_ORGANIZATION); err != nil {
			return err
		}
	} else if user != "" {
		if err := rbac_client_.AddResourceOwner(token, resourcePath, user, "package", rbacpb.SubjectType_ACCOUNT); err != nil {
			return err
		}
	}
	return nil
}

/** Create a service package **/
func (client *Repository_Service_Client) createPackageArchive(PublisherID string, id string, version string, platform string, path string) (string, error) {
	archive_name := id + "%" + version + "%" + id + "%" + platform

	var buf bytes.Buffer
	Utility.CompressDir(path, &buf)

	outPath := os.TempDir() + string(os.PathSeparator) + archive_name + ".tar.gz"
	fileToWrite, err := os.OpenFile(outPath, os.O_CREATE|os.O_RDWR, os.FileMode(0755))
	if err != nil {
		return "", err
	}
	defer fileToWrite.Close()

	if _, err := io.Copy(fileToWrite, &buf); err != nil {
		return "", err
	}
	return outPath, nil
}
