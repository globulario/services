package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/ldap/ldappb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	//"google.golang.org/grpc/grpclog"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/ldap/ldap_client"
	LDAP "github.com/go-ldap/ldap/v3"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

var (
	defaultPort  = 10031
	defaultProxy = 10032

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""

	// The default domain
	domain string = "localhost"

	// The ressource service to register account and create group...
	resource_client_ *resource_client.Resource_Client
)

// Keep connection information here.
type connection struct {
	Id       string // The connection id
	Host     string // can also be ipv4 addresse.
	User     string
	Password string
	Port     int32
	conn     *LDAP.Conn
}

type server struct {

	// The global attribute of the services.
	Id                 string
	Mac                string
	Name               string
	Path               string
	Proto              string
	Port               int
	Proxy              int
	Protocol           string
	AllowAllOrigins    bool
	AllowedOrigins     string // comma separated string.
	Domain             string
	Description        string
	Keywords           []string
	Repositories       []string
	Discoveries        []string
	CertAuthorityTrust string
	CertFile           string
	KeyFile            string
	Version            string
	TLS                bool
	PublisherId        string
	KeepUpToDate       bool
	KeepAlive          bool
	Permissions        []interface{} // contains the action permission for the services.
	Dependencies       []string      // The list of services needed by this services.
	Process            int
	ProxyProcess       int
	ConfigPath         string
	LastError          string

	// The grpc server.
	grpcServer *grpc.Server

	// The map of connection...
	Connections map[string]connection

	// The ldap synchronization info...
	LdapSyncInfos map[string]interface{} // Contain LdapSyncInfos...

}

// Globular services implementation...
// The id of a particular service instance.
func (server *server) GetId() string {
	return server.Id
}
func (server *server) SetId(id string) {
	server.Id = id
}

// The name of a service, must be the gRpc Service name.
func (server *server) GetName() string {
	return server.Name
}
func (server *server) SetName(name string) {
	server.Name = name
}

// The description of the service
func (server *server) GetDescription() string {
	return server.Description
}
func (server *server) SetDescription(description string) {
	server.Description = description
}

func (svr *server) GetMac() string {
	return svr.Mac
}

func (svr *server) SetMac(mac string) {
	svr.Mac = mac
}

// The list of keywords of the services.
func (server *server) GetKeywords() []string {
	return server.Keywords
}
func (server *server) SetKeywords(keywords []string) {
	server.Keywords = keywords
}

func (server *server) GetRepositories() []string {
	return server.Repositories
}
func (server *server) SetRepositories(repositories []string) {
	server.Repositories = repositories
}

func (server *server) GetDiscoveries() []string {
	return server.Discoveries
}
func (server *server) SetDiscoveries(discoveries []string) {
	server.Discoveries = discoveries
}

// Dist
func (server *server) Dist(path string) (string, error) {

	return globular.Dist(path, server)
}

func (server *server) GetDependencies() []string {

	if server.Dependencies == nil {
		server.Dependencies = make([]string, 0)
	}

	return server.Dependencies
}

func (server *server) SetDependency(dependency string) {
	if server.Dependencies == nil {
		server.Dependencies = make([]string, 0)
	}

	// Append the depency to the list.
	if !Utility.Contains(server.Dependencies, dependency) {
		server.Dependencies = append(server.Dependencies, dependency)
	}
}

func (server *server) GetPlatform() string {
	return globular.GetPlatform()
}

// The path of the executable.
func (server *server) GetPath() string {
	return server.Path
}
func (server *server) SetPath(path string) {
	server.Path = path
}

// The path of the .proto file.
func (server *server) GetProto() string {
	return server.Proto
}
func (server *server) SetProto(proto string) {
	server.Proto = proto
}

// The gRpc port.
func (server *server) GetPort() int {
	return server.Port
}
func (server *server) SetPort(port int) {
	server.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (server *server) GetProxy() int {
	return server.Proxy
}
func (server *server) SetProxy(proxy int) {
	server.Proxy = proxy
}

// Can be one of http/https/tls
func (server *server) GetProtocol() string {
	return server.Protocol
}
func (server *server) SetProtocol(protocol string) {
	server.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (server *server) GetAllowAllOrigins() bool {
	return server.AllowAllOrigins
}
func (server *server) SetAllowAllOrigins(allowAllOrigins bool) {
	server.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (server *server) GetAllowedOrigins() string {
	return server.AllowedOrigins
}

func (server *server) SetAllowedOrigins(allowedOrigins string) {
	server.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (server *server) GetDomain() string {
	return server.Domain
}
func (server *server) SetDomain(domain string) {
	server.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (server *server) GetTls() bool {
	return server.TLS
}
func (server *server) SetTls(hasTls bool) {
	server.TLS = hasTls
}

// The certificate authority file
func (server *server) GetCertAuthorityTrust() string {
	return server.CertAuthorityTrust
}
func (server *server) SetCertAuthorityTrust(ca string) {
	server.CertAuthorityTrust = ca
}

// The certificate file.
func (server *server) GetCertFile() string {
	return server.CertFile
}
func (server *server) SetCertFile(certFile string) {
	server.CertFile = certFile
}

// The key file.
func (server *server) GetKeyFile() string {
	return server.KeyFile
}
func (server *server) SetKeyFile(keyFile string) {
	server.KeyFile = keyFile
}

// The service version
func (server *server) GetVersion() string {
	return server.Version
}
func (server *server) SetVersion(version string) {
	server.Version = version
}

// The publisher id.
func (server *server) GetPublisherId() string {
	return server.PublisherId
}
func (server *server) SetPublisherId(publisherId string) {
	server.PublisherId = publisherId
}

func (server *server) GetKeepUpToDate() bool {
	return server.KeepUpToDate
}
func (server *server) SetKeepUptoDate(val bool) {
	server.KeepUpToDate = val
}

func (server *server) GetKeepAlive() bool {
	return server.KeepAlive
}
func (server *server) SetKeepAlive(val bool) {
	server.KeepAlive = val
}

func (server *server) GetPermissions() []interface{} {
	return server.Permissions
}
func (server *server) SetPermissions(permissions []interface{}) {
	server.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (server *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewLdapService_Client", ldap_client.NewLdapService_Client)

	// Get the configuration path.
	err := globular.InitService(server)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	server.grpcServer, err = globular.InitGrpcServer(server, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	return nil

}

// Save the configuration values.
func (server *server) Save() error {
	// Create the file...
	return globular.SaveService(server)
}

func (server *server) StartService() error {
	return globular.StartService(server, server.grpcServer)
}

func (server *server) StopService() error {
	return globular.StopService(server, server.grpcServer)
}

func (server *server) Stop(context.Context, *ldappb.StopRequest) (*ldappb.StopResponse, error) {
	return &ldappb.StopResponse{}, server.StopService()
}

/**
 * Connect to a ldap server...
 */
func (server *server) connect(id string, userId string, pwd string) (*LDAP.Conn, error) {

	// The info must be set before that function is call.
	info := server.Connections[id]

	conn, err := LDAP.Dial("tcp", fmt.Sprintf("%s:%d", info.Host, info.Port))
	if err != nil {
		// handle error
		return nil, err
	}

	conn.SetTimeout(time.Duration(3 * time.Second))

	// Connect with the default user...
	if len(userId) > 0 {
		if len(pwd) > 0 {
			err = conn.Bind(userId, pwd)
		} else {
			err = conn.UnauthenticatedBind(userId)
		}
		if err != nil {
			return nil, err
		}
	} else {
		if len(info.Password) > 0 {
			err = conn.Bind(info.User, info.Password)
		} else {
			err = conn.UnauthenticatedBind(info.User)
		}
		if err != nil {
			return nil, err
		}
	}

	return conn, nil
}

///////////////////// resource service functions ////////////////////////////////////
func (svr *server) getResourceClient() (*resource_client.Resource_Client, error) {
	var err error
	if resource_client_ != nil {
		return resource_client_, nil
	}

	resource_client_, err = resource_client.NewResourceService_Client(svr.Domain, "resource.ResourceService")
	if err != nil {
		resource_client_ = nil
		return nil, err
	}

	return resource_client_, nil
}

// Create an empty group.
func (svr *server) createGroup(id, name, description string) error {
	resourceClient, err := svr.getResourceClient()
	if err != nil {
		return err
	}
	return resourceClient.CreateGroup(id, name, description)
}

// Register a new user.
func (svr *server) registerAccount(domain, id, name, email, password string) error {
	resourceClient, err := svr.getResourceClient()
	if err != nil {
		return err
	}

	return resourceClient.RegisterAccount(domain, id, name, email, password, password)
}

func (svr *server) addGroupMemberAccount(groupId string, accountId string) error {
	resourceClient, err := svr.getResourceClient()
	if err != nil {
		return err
	}
	return resourceClient.AddGroupMemberAccount(groupId, accountId)
}

func (svr *server) removeGroupMemberAccount(groupId string, accountId string) error {
	resourceClient, err := svr.getResourceClient()
	if err != nil {
		return err
	}
	return resourceClient.RemoveGroupMemberAccount(groupId, accountId)
}

func (svr *server) getAccount(id string) (*resourcepb.Account, error) {
	resourceClient, err := svr.getResourceClient()
	if err != nil {
		return nil, err
	}

	return resourceClient.GetAccount(id)
}

func (svr *server) getGroup(id string) (*resourcepb.Group, error) {
	resourceClient, err := svr.getResourceClient()
	if err != nil {
		return nil, err
	}
	groups, err := resourceClient.GetGroups(`{"_id":"` + id + `"}`)
	if len(groups) > 0 {
		return groups[0], nil
	} else if err != nil {
		return nil, err
	}
	return nil, errors.New("no group found with id " + id)
}

/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// LDAP specific functionality
/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Synchronize the resource with LDAP.
func (server *server) Synchronize(ctx context.Context, rqst *ldappb.SynchronizeRequest) (*ldappb.SynchronizeResponse, error) {
	err := server.synchronize()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &ldappb.SynchronizeResponse{}, nil
}

// Append synchronize information.
// "LdapSyncInfos": {
//     "my__ldap":
//       {
//         "ConnectionId": "my__ldap",
//         "GroupSyncInfo": {
//           "Base": "OU=Access_Groups,OU=Groups,OU=MON,OU=CA,DC=UD6,DC=UF6",
//           "Id": "name",
//           "Query": "((objectClass=group))"
//         },
//         "Refresh": 1,
//         "UserSyncInfo": {
//           "Base": "OU=Users,OU=MON,OU=CA,DC=UD6,DC=UF6",
//           "Email": "mail",
//           "Id": "userPrincipalName",
//           "Query": "(|(objectClass=person)(objectClass=user))"
//         }
//       }
//  }
func (server *server) SetLdapSyncInfo(ctx context.Context, rqst *ldappb.SetLdapSyncInfoRequest) (*ldappb.SetLdapSyncInfoResponse, error) {
	info:= make(map[string] interface{}, 0)

	info["ConnectionId"] = rqst.Info.ConnectionId
	info["Refresh"] = rqst.Info.Refresh
	info["GroupSyncInfo"] = make(map[string] interface{}, 0)
	info["GroupSyncInfo"].(map[string] interface{})["Id"] = rqst.Info.GroupSyncInfo.Id
	info["GroupSyncInfo"].(map[string] interface{})["Base"] = rqst.Info.GroupSyncInfo.Base
	info["GroupSyncInfo"].(map[string] interface{})["Query"] = rqst.Info.GroupSyncInfo.Query
	info["UserSyncInfo"] = make(map[string] interface{}, 0)
	info["UserSyncInfo"].(map[string] interface{})["Id"] = rqst.Info.UserSyncInfo.Id
	info["UserSyncInfo"].(map[string] interface{})["Base"] = rqst.Info.UserSyncInfo.Base
	info["UserSyncInfo"].(map[string] interface{})["Query"] = rqst.Info.UserSyncInfo.Query

	if server.LdapSyncInfos ==nil {
		server.LdapSyncInfos = make(map[string] interface{}, 0)
	}

	// Store the info.
	server.LdapSyncInfos[ rqst.Info.ConnectionId] = info

	return &ldappb.SetLdapSyncInfoResponse{}, nil
}

// Delete synchronize information
func (server *server) DeleteLdapSyncInfo(ctx context.Context, rqst *ldappb.DeleteLdapSyncInfoRequest) (*ldappb.DeleteLdapSyncInfoResponse, error) {
	if server.LdapSyncInfos!=nil {
		if server.LdapSyncInfos[rqst.Id]!=nil {
			delete(server.LdapSyncInfos, rqst.Id)
		}
	}

	return &ldappb.DeleteLdapSyncInfoResponse{}, nil
}

// Retreive synchronize informations
func (server *server) GetLdapSyncInfo(ctx context.Context, rqst *ldappb.GetLdapSyncInfoRequest) (*ldappb.GetLdapSyncInfoResponse, error) {
	infos := make([]*ldappb.LdapSyncInfo, 0)


	for _, info := range server.LdapSyncInfos {
		info_ := new(ldappb.LdapSyncInfo)

		info_.Id = info.(map[string]interface{})["Id"].(string)
		info_.ConnectionId = info.(map[string]interface{})["ConnectionId"].(string)
		info_.Refresh = info.(map[string]interface{})["Refresh"].(int32)
		info_.GroupSyncInfo = new(ldappb.GroupSyncInfo)
		info_.GroupSyncInfo.Id = info.(map[string]interface{})["GroupSyncInfo"].(map[string]interface{})["Id"].(string)
		info_.GroupSyncInfo.Base = info.(map[string]interface{})["GroupSyncInfo"].(map[string]interface{})["Base"].(string)
		info_.GroupSyncInfo.Query = info.(map[string]interface{})["GroupSyncInfo"].(map[string]interface{})["Query"].(string)
		info_.UserSyncInfo = new(ldappb.UserSyncInfo)
		info_.UserSyncInfo.Id = info.(map[string]interface{})["UserSyncInfo"].(map[string]interface{})["Id"].(string)
		info_.UserSyncInfo.Base = info.(map[string]interface{})["UserSyncInfo"].(map[string]interface{})["Base"].(string)
		info_.UserSyncInfo.Email = info.(map[string]interface{})["UserSyncInfo"].(map[string]interface{})["Email"].(string)
		info_.UserSyncInfo.Query = info.(map[string]interface{})["UserSyncInfo"].(map[string]interface{})["Query"].(string)
		if len(rqst.Id) > 0 {
			if rqst.Id == info_.Id {
				infos = append(infos, info_)
				break
			}
		}else{
			// append all infos.
			infos = append(infos, info_)
		}
	}
	// return the 
	return &ldappb.GetLdapSyncInfoResponse{Infos: infos}, nil
}

// Authenticate a user with LDAP server.
func (server *server) Authenticate(ctx context.Context, rqst *ldappb.AuthenticateRqst) (*ldappb.AuthenticateRsp, error) {
	id := rqst.Id
	login := rqst.Login
	pwd := rqst.Pwd

	if len(id) > 0 {
		// I will made use of bind to authenticate the user.
		_, err := server.connect(id, login, pwd)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		for id, _ := range server.Connections {
			_, err := server.connect(id, login, pwd)
			if err == nil {
				return &ldappb.AuthenticateRsp{
					Result: true,
				}, nil
			}
		}
		// fail to authenticate.
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Authentication fail for user "+rqst.Login)))
	}

	return &ldappb.AuthenticateRsp{
		Result: true,
	}, nil
}

// Create a new SQL connection and store it for futur use. If the connection already
// exist it will be replace by the new one.
func (server *server) CreateConnection(ctx context.Context, rsqt *ldappb.CreateConnectionRqst) (*ldappb.CreateConnectionRsp, error) {
	fmt.Println("Try to create a new connection")
	var c connection
	var err error

	// Set the connection info from the request.
	c.Id = rsqt.Connection.Id
	c.Host = rsqt.Connection.Host
	c.Port = rsqt.Connection.Port
	c.User = rsqt.Connection.User
	c.Password = rsqt.Connection.Password

	// set or update the connection and save it in json file.
	server.Connections[c.Id] = c

	c.conn, err = server.connect(c.Id, c.User, c.Password)
	defer c.conn.Close()

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// In that case I will save it in file.
	err = server.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &ldappb.CreateConnectionRsp{
		Result: true,
	}, nil
}

// Remove a connection from the map and the file.
func (server *server) DeleteConnection(ctx context.Context, rqst *ldappb.DeleteConnectionRqst) (*ldappb.DeleteConnectionRsp, error) {

	id := rqst.GetId()
	if _, ok := server.Connections[id]; !ok {
		return &ldappb.DeleteConnectionRsp{
			Result: true,
		}, nil
	}

	if server.Connections[id].conn != nil {
		// Close the connection.
		server.Connections[id].conn.Close()
	}

	delete(server.Connections, id)

	// In that case I will save it in file.
	err := server.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// return success.
	return &ldappb.DeleteConnectionRsp{
		Result: true,
	}, nil

}

// Close connection.
func (server *server) Close(ctx context.Context, rqst *ldappb.CloseRqst) (*ldappb.CloseRsp, error) {
	id := rqst.GetId()
	if _, ok := server.Connections[id]; !ok {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Connection "+id+" dosent exist!")))
	}

	server.Connections[id].conn.Close()

	// return success.
	return &ldappb.CloseRsp{
		Result: true,
	}, nil
}

// Synchronize the list user and group whith ressources...
// Here is an exemple of ldap configuration...
// "LdapSyncInfos": {
//     "my__ldap":
//       {
//         "ConnectionId": "my__ldap",
//         "GroupSyncInfo": {
//           "Base": "OU=Access_Groups,OU=Groups,OU=MON,OU=CA,DC=UD6,DC=UF6",
//           "Id": "name",
//           "Query": "((objectClass=group))"
//         },
//         "Refresh": 1,
//         "UserSyncInfo": {
//           "Base": "OU=Users,OU=MON,OU=CA,DC=UD6,DC=UF6",
//           "Email": "mail",
//           "Id": "userPrincipalName",
//           "Query": "(|(objectClass=person)(objectClass=user))"
//         }
//       }
//  }
func (server *server) synchronize() error {

	for connectionId, syncInfo_ := range server.LdapSyncInfos {
		syncInfo := syncInfo_.(map[string]interface{})
		GroupSyncInfo := syncInfo["GroupSyncInfo"].(map[string]interface{})
		groupsInfo, err := server.search(connectionId, GroupSyncInfo["Base"].(string), GroupSyncInfo["Query"].(string), []string{GroupSyncInfo["Id"].(string), "distinguishedName"})
		if err != nil {
			fmt.Println("fail to retreive group info", err)
			return err
		}

		// Print group info.
		for i := 0; i < len(groupsInfo); i++ {
			name := groupsInfo[i][0].([]string)[0]
			id := Utility.GenerateUUID(groupsInfo[i][1].([]string)[0])
			_, err := server.getGroup(id)
			if err != nil {
				server.createGroup(id, name, "") // The group member will be set latter in that function.
			}
		}

		// Synchronize account and user info...
		UserSyncInfo := syncInfo["UserSyncInfo"].(map[string]interface{})
		accountsInfo, err := server.search(connectionId, UserSyncInfo["Base"].(string), UserSyncInfo["Query"].(string), []string{UserSyncInfo["Id"].(string), UserSyncInfo["Email"].(string), "distinguishedName", "memberOf"})
		if err != nil {
			fmt.Println("fail to retreive account info", err)
			return err
		}

		for i := 0; i < len(accountsInfo); i++ {
			// Print the list of account...
			// I will not set the password...
			name := strings.ToLower(accountsInfo[i][0].([]string)[0])

			if len(accountsInfo[i][1].([]string)) > 0 {
				email := strings.ToLower(accountsInfo[i][1].([]string)[0])

				if len(email) > 0 {
					// Generate the
					id := name //strings.ToLower(accountsInfo[i][2].([]string)[0])

					if strings.Index(id, "@") > 0 {
						id = strings.Split(id, "@")[0]
					}

					if len(id) > 0 {

						// Try to create account...
						a, err := server.getAccount(id)
						if err != nil {
							err := server.registerAccount(server.Domain, id, name, email, id)
							if err != nil {
								fmt.Println("fail to register account ", id, err)
							}
						}

						a, err = server.getAccount(id)

						// Now I will update the groups user list...
						if err == nil {
							if len(accountsInfo[i][3].([]string)) > 0 && a != nil {
								groups := accountsInfo[i][3].([]string)
								// Append not existing group...
								for j := 0; j < len(groups); j++ {
									groupId := Utility.GenerateUUID(groups[j])

									if !Utility.Contains(a.Groups, groupId) {
										// Now I will remo
										err := server.addGroupMemberAccount(groupId, a.Id)
										if err != nil {
											fmt.Println("fail to add account ", a.Id, " to ", groupId, err)
										}
									}

								}

								// Remove group that no more part of the ldap group.
								for j := 0; j < len(a.Groups); j++ {
									groupId := a.Groups[j]
									if !Utility.Contains(groups, groupId) {
										// Now I will remo
										err := server.removeGroupMemberAccount(groupId, a.Id)
										if err != nil {
											fmt.Println("fail to remove account ", a.Id, " from group ", groupId, " with error ", err)
										}
									}

								}
							}
						}

					}
				} else {
					return errors.New("account " + strings.ToLower(accountsInfo[i][2].([]string)[0]) + " has no email configured! ")
				}
			}
		}
	}

	// No error...
	return nil
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// set the logger.
	//grpclog.SetLogger(log.New(os.Stdout, "ldap_service: ", log.LstdFlags))

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// The actual server implementation.
	s_impl := new(server)
	s_impl.Connections = make(map[string]connection)
	s_impl.Name = string(ldappb.File_ldap_proto.Services().Get(0).FullName())
	s_impl.Proto = ldappb.File_ldap_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.PublisherId = "globulario"
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}
	if len(os.Args) == 2 {
		s_impl.Port, _ = strconv.Atoi(os.Args[1]) // The second argument must be the port number
	}

	// Register the echo services
	ldappb.RegisterLdapServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)
	/*
		go func() {
			s_impl.synchronize()
		}()
	*/
	// Start the service.
	s_impl.StartService()

}

/**
 * Search for a list of value over the ldap server. if the base_dn is
 * not specify the default base is use. It return a list of values. This can
 * be interpret as a tow dimensional array.
 */
func (server *server) search(id string, base_dn string, filter string, attributes []string) ([][]interface{}, error) {

	if _, ok := server.Connections[id]; !ok {
		return nil, errors.New("Connection " + id + " dosent exist!")
	}

	// create the connection.
	c := server.Connections[id]
	conn, err := server.connect(id, server.Connections[id].User, server.Connections[id].Password)
	if err != nil {
		return nil, err
	}

	c.conn = conn
	server.Connections[id] = c

	// close connection after search.
	defer c.conn.Close()

	//Now I will execute the query...
	search_request := LDAP.NewSearchRequest(
		base_dn,
		LDAP.ScopeWholeSubtree, LDAP.NeverDerefAliases, 0, 0, false,
		filter,
		attributes,
		nil)

	// Create simple search.
	sr, err := server.Connections[id].conn.Search(search_request)

	if err != nil {
		return nil, err
	}

	// Store the founded values in results...
	var results [][]interface{}
	for i := 0; i < len(sr.Entries); i++ {
		entry := sr.Entries[i]
		var row []interface{}
		for j := 0; j < len(attributes); j++ {
			attributeName := attributes[j]
			attributeValues := entry.GetAttributeValues(attributeName)
			row = append(row, attributeValues)
		}
		results = append(results, row)
	}

	return results, nil
}

// Search over LDAP server.
func (server *server) Search(ctx context.Context, rqst *ldappb.SearchRqst) (*ldappb.SearchResp, error) {
	id := rqst.Search.GetId()
	if _, ok := server.Connections[id]; !ok {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Connection "+id+" dosent exist!")))
	}

	results, err := server.search(id, rqst.Search.BaseDN, rqst.Search.Filter, rqst.Search.Attributes)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Here I got the results.
	str, err := json.Marshal(results)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &ldappb.SearchResp{
		Result: string(str),
	}, nil
}
