package applications_manager_client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/applications_manager/applications_managerpb"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	"github.com/polds/imgbase64"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// echo Client Service
////////////////////////////////////////////////////////////////////////////////

type Applications_Manager_Client struct {
	cc *grpc.ClientConn
	c  applications_managerpb.ApplicationManagerServiceClient

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
func NewApplicationsManager_Client(address string, id string) (*Applications_Manager_Client, error) {
	client := new(Applications_Manager_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = applications_managerpb.NewApplicationManagerServiceClient(client.cc)

	return client, nil
}

func (Applications_Manager_Client *Applications_Manager_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = Applications_Manager_Client.GetCtx()
	}
	return globular.InvokeClientRequest(Applications_Manager_Client.c, ctx, method, rqst)
}

func (client *Applications_Manager_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}

	// refresh the client as needed...
	token, err := security.GetLocalToken(client.GetDomain())
	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": client.domain, "mac": client.GetMac()})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	return client.ctx
}

// Return the domain
func (Applications_Manager_Client *Applications_Manager_Client) GetDomain() string {
	return Applications_Manager_Client.domain
}

// Return the address
func (Applications_Manager_Client *Applications_Manager_Client) GetAddress() string {
	return Applications_Manager_Client.domain + ":" + strconv.Itoa(Applications_Manager_Client.port)
}

// Return the id of the service instance
func (Applications_Manager_Client *Applications_Manager_Client) GetId() string {
	return Applications_Manager_Client.id
}

// Return the name of the service
func (Applications_Manager_Client *Applications_Manager_Client) GetName() string {
	return Applications_Manager_Client.name
}

func (Applications_Manager_Client *Applications_Manager_Client) GetMac() string {
	return Applications_Manager_Client.mac
}

// must be close when no more needed.
func (Applications_Manager_Client *Applications_Manager_Client) Close() {
	Applications_Manager_Client.cc.Close()
}

// Set grpc_service port.
func (Applications_Manager_Client *Applications_Manager_Client) SetPort(port int) {
	Applications_Manager_Client.port = port
}

// Set the client instance id.
func (Applications_Manager_Client *Applications_Manager_Client) SetId(id string) {
	Applications_Manager_Client.id = id
}

// Set the client name.
func (Applications_Manager_Client *Applications_Manager_Client) SetName(name string) {
	Applications_Manager_Client.name = name
}

func (Applications_Manager_Client *Applications_Manager_Client) SetMac(mac string) {
	Applications_Manager_Client.mac = mac
}

// Set the domain.
func (Applications_Manager_Client *Applications_Manager_Client) SetDomain(domain string) {
	Applications_Manager_Client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (Applications_Manager_Client *Applications_Manager_Client) HasTLS() bool {
	return Applications_Manager_Client.hasTLS
}

// Get the TLS certificate file path
func (Applications_Manager_Client *Applications_Manager_Client) GetCertFile() string {
	return Applications_Manager_Client.certFile
}

// Get the TLS key file path
func (Applications_Manager_Client *Applications_Manager_Client) GetKeyFile() string {
	return Applications_Manager_Client.keyFile
}

// Get the TLS key file path
func (Applications_Manager_Client *Applications_Manager_Client) GetCaFile() string {
	return Applications_Manager_Client.caFile
}

// Set the client is a secure client.
func (Applications_Manager_Client *Applications_Manager_Client) SetTLS(hasTls bool) {
	Applications_Manager_Client.hasTLS = hasTls
}

// Set TLS certificate file path
func (Applications_Manager_Client *Applications_Manager_Client) SetCertFile(certFile string) {
	Applications_Manager_Client.certFile = certFile
}

// Set TLS key file path
func (Applications_Manager_Client *Applications_Manager_Client) SetKeyFile(keyFile string) {
	Applications_Manager_Client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (Applications_Manager_Client *Applications_Manager_Client) SetCaFile(caFile string) {
	Applications_Manager_Client.caFile = caFile
}

////////////////// Api //////////////////////

/**
 * Intall a new application or update an existing one.
 */
func (client *Applications_Manager_Client) InstallApplication(token string, domain string, user string, discoveryId string, publisherId string, applicationId string, set_as_default bool) error {

	rqst := new(applications_managerpb.InstallApplicationRequest)
	rqst.DicorveryId = discoveryId
	rqst.PublisherId = publisherId
	rqst.ApplicationId = applicationId
	rqst.Domain = strings.Split(domain, ":")[0] // remove the port if one is given...
	rqst.SetAsDefault = set_as_default

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.InstallApplication(ctx, rqst)

	return err
}

/**
 * Uninstall application, if no version is given the most recent version will
 * be install.
 */
func (client *Applications_Manager_Client) UninstallApplication(token string, domain string, user string, publisherId string, applicationId string, version string) error {

	rqst := new(applications_managerpb.UninstallApplicationRequest)
	rqst.PublisherId = publisherId
	rqst.ApplicationId = applicationId
	rqst.Version = version
	rqst.Domain = strings.Split(domain, ":")[0] // remove the port if one is given...

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)
		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.UninstallApplication(ctx, rqst)

	return err
}

/**
 * Deploy the content of an application with a given name to the server.
 */
func (client *Applications_Manager_Client) DeployApplication(user string, name string, organization string, path string, token string, domain string, set_as_default bool) (int, error) {
	log.Println("deploy application", name)
	dir, err := os.Getwd()
	if err != nil {
		return -1, err
	}
	if !strings.HasPrefix(path, "/") {
		path = strings.ReplaceAll(dir, "\\", "/") + "/" + path

	}

	// Now I will open the data and create a archive from it.
	var buffer bytes.Buffer
	total, err := Utility.CompressDir(path, &buffer)
	if err != nil {
		return -1, err
	}

	// From the path I will get try to find the package.json file and get information from it...
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return -1, err
	}

	absolutePath = strings.ReplaceAll(absolutePath, "\\", "/")

	if Utility.Exists(absolutePath + "/package.json") {
		absolutePath += "/package.json"
	} else if Utility.Exists(absolutePath[0:strings.LastIndex(absolutePath, "/")] + "/package.json") {
		absolutePath = absolutePath[0:strings.LastIndex(absolutePath, "/")] + "/package.json"
	} else {
		err = errors.New("no package.config file was found")
		return -1, err
	}

	packageConfig := make(map[string]interface{})
	log.Println("read file from", absolutePath)
	data, err := ioutil.ReadFile(absolutePath)
	if err != nil {
		return -1, err
	}

	err = json.Unmarshal(data, &packageConfig)
	if err != nil {
		return -1, err
	}

	description := packageConfig["description"].(string)
	version := packageConfig["version"].(string)

	alias := name
	if packageConfig["alias"] != nil {
		alias = packageConfig["alias"].(string)
	}

	// Set keywords.
	keywords := make([]string, 0)
	if packageConfig["keywords"] != nil {
		for i := 0; i < len(packageConfig["keywords"].([]interface{})); i++ {
			keywords = append(keywords, packageConfig["keywords"].([]interface{})[i].(string))
		}
	}

	// Now The application is deploy I will set application actions from the
	// package.json file.
	log.Println("set actions")
	actions := make([]string, 0)
	if packageConfig["actions"] != nil {
		for i := 0; i < len(packageConfig["actions"].([]interface{})); i++ {
			log.Println("set action permission: ", packageConfig["actions"].([]interface{})[i].(string))
			actions = append(actions, packageConfig["actions"].([]interface{})[i].(string))
		}
	}

	// Create roles.
	log.Println("create roles")
	roles := make([]*resourcepb.Role, 0)
	if packageConfig["roles"] != nil {
		// Here I will create the roles require by the applications.
		roles_ := packageConfig["roles"].([]interface{})
		for i := 0; i < len(roles_); i++ {
			role_ := roles_[i].(map[string]interface{})
			role := new(resourcepb.Role)
			role.Id = role_["id"].(string)
			role.Name = role_["name"].(string)
			role.Actions = make([]string, 0)
			for j := 0; j < len(role_["actions"].([]interface{})); j++ {
				role.Actions = append(role.Actions, role_["actions"].([]interface{})[j].(string))
			}
			roles = append(roles, role)
		}
	}

	// Create groups.
	log.Println("create groups")
	groups := make([]*resourcepb.Group, 0)
	if packageConfig["groups"] != nil {
		groups_ := packageConfig["groups"].([]interface{})
		for i := 0; i < len(groups_); i++ {
			group_ := groups_[i].(map[string]interface{})
			group := new(resourcepb.Group)
			group.Id = group_["id"].(string)
			group.Name = group_["name"].(string)
			groups = append(groups, group)
		}
	}

	var icon string

	// Now the icon...
	if packageConfig["icon"] != nil {
		// The image icon.
		// iconPath := absolutePath[0:strings.LastIndex(absolutePath, "/")] + "/package.json"
		iconPath := strings.ReplaceAll(absolutePath, "\\", "/")
		lastIndex := strings.LastIndex(iconPath, "/")
		iconPath = iconPath[0:lastIndex] + "/" + packageConfig["icon"].(string)
		if Utility.Exists(iconPath) {
			// Convert to png before creating the data url.
			if strings.HasSuffix(strings.ToLower(iconPath), ".svg") {
				pngPath := os.TempDir() + "/output.png"
				defer os.Remove(pngPath)
				err := Utility.SvgToPng(iconPath, pngPath, 128, 128)
				if err == nil {
					iconPath = pngPath
				}
			}
			// So here I will create the b64 string
			icon, _ = imgbase64.FromLocal(iconPath)
		}

	}

	// Set the token into the context and send the request.
	md := metadata.New(map[string]string{"token": string(token), "application": name, "domain": domain, "organization": organization, "user": user})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	// Open the stream...
	stream, err := client.c.DeployApplication(ctx)
	if err != nil {
		return -1, err
	}

	const BufferSize = 1024 * 25 // the chunck size.
	var size int
	for {
		var data [BufferSize]byte
		bytesread, err := buffer.Read(data[0:BufferSize])

		if bytesread > 0 {
			rqst := &applications_managerpb.DeployApplicationRequest{
				Data:         data[0:bytesread],
				Name:         name,
				Domain:       domain,
				Organization: organization,
				User:         user,
				Version:      version,
				Description:  description,
				Keywords:     keywords,
				Actions:      actions,
				Icon:         icon,
				Alias:        alias,
				Groups:       groups,
				Roles:        roles,
				SetAsDefault: set_as_default,
			}
			// send the data to the server.
			err = stream.Send(rqst)
			if err == io.EOF {
				break
			} else if err != nil {
				return -1, err
			}
		}
		size += bytesread
		log.Println("transfert ", size, "of", total, " ", int(float64(size)/float64(total)*100), "%")
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
