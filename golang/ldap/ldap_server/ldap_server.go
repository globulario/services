package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/ldap/ldappb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	//"google.golang.org/grpc/grpclog"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/go-ldap/ldap/v3"
	"github.com/vjeantet/ldapserver"

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
)

// Keep connection information here.
type connection struct {
	Id       string // The connection id
	Host     string // can also be ipv4 addresse.
	User     string
	Password string
	Port     int32
	conn     *ldap.Conn
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
	Address            string
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
	Plaform            string
	Checksum           string
	KeepAlive          bool
	Permissions        []interface{} // contains the action permission for the services.
	Dependencies       []string      // The list of services needed by this services.
	Process            int
	ProxyProcess       int
	ConfigPath         string
	LastError          string
	ModTime            int64
	State              string

	// The grpc server.
	grpcServer *grpc.Server

	// The map of connection...
	Connections map[string]connection

	// The ldap synchronization info...
	LdapSyncInfos map[string]interface{} // Contain LdapSyncInfos...

}

// The http address where the configuration can be found /config
func (srv *server) GetAddress() string {
	return srv.Address
}

func (srv *server) SetAddress(address string) {
	srv.Address = address
}

func (srv *server) GetProcess() int {
	return srv.Process
}

func (srv *server) SetProcess(pid int) {
	srv.Process = pid
}

func (srv *server) GetProxyProcess() int {
	return srv.ProxyProcess
}

func (srv *server) SetProxyProcess(pid int) {
	srv.ProxyProcess = pid
}

// The current service state
func (srv *server) GetState() string {
	return srv.State
}

func (srv *server) SetState(state string) {
	srv.State = state
}

// The last error
func (srv *server) GetLastError() string {
	return srv.LastError
}

func (srv *server) SetLastError(err string) {
	srv.LastError = err
}

// The modeTime
func (srv *server) SetModTime(modtime int64) {
	srv.ModTime = modtime
}
func (srv *server) GetModTime() int64 {
	return srv.ModTime
}

// Globular services implementation...
// The id of a particular service instance.
func (srv *server) GetId() string {
	return srv.Id
}
func (srv *server) SetId(id string) {
	srv.Id = id
}

// The name of a service, must be the gRpc Service name.
func (srv *server) GetName() string {
	return srv.Name
}
func (srv *server) SetName(name string) {
	srv.Name = name
}

// The description of the service
func (srv *server) GetDescription() string {
	return srv.Description
}
func (srv *server) SetDescription(description string) {
	srv.Description = description
}

func (srv *server) GetMac() string {
	return srv.Mac
}

func (srv *server) SetMac(mac string) {
	srv.Mac = mac
}

// The list of keywords of the services.
func (srv *server) GetKeywords() []string {
	return srv.Keywords
}
func (srv *server) SetKeywords(keywords []string) {
	srv.Keywords = keywords
}

func (srv *server) GetRepositories() []string {
	return srv.Repositories
}
func (srv *server) SetRepositories(repositories []string) {
	srv.Repositories = repositories
}

func (srv *server) GetDiscoveries() []string {
	return srv.Discoveries
}
func (srv *server) SetDiscoveries(discoveries []string) {
	srv.Discoveries = discoveries
}

// Dist
func (srv *server) Dist(path string) (string, error) {

	return globular.Dist(path, srv)
}

func (srv *server) GetDependencies() []string {

	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}

	return srv.Dependencies
}

func (srv *server) SetDependency(dependency string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}

	// Append the depency to the list.
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}
func (srv *server) GetChecksum() string {

	return srv.Checksum
}

func (srv *server) SetChecksum(checksum string) {
	srv.Checksum = checksum
}

func (srv *server) GetPlatform() string {
	return srv.Plaform
}

func (srv *server) SetPlatform(platform string) {
	srv.Plaform = platform
}

// The path of the executable.
func (srv *server) GetPath() string {
	return srv.Path
}
func (srv *server) SetPath(path string) {
	srv.Path = path
}

// The path of the .proto file.
func (srv *server) GetProto() string {
	return srv.Proto
}
func (srv *server) SetProto(proto string) {
	srv.Proto = proto
}

// The gRpc port.
func (srv *server) GetPort() int {
	return srv.Port
}
func (srv *server) SetPort(port int) {
	srv.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (srv *server) GetProxy() int {
	return srv.Proxy
}
func (srv *server) SetProxy(proxy int) {
	srv.Proxy = proxy
}

// Can be one of http/https/tls
func (srv *server) GetProtocol() string {
	return srv.Protocol
}
func (srv *server) SetProtocol(protocol string) {
	srv.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (srv *server) GetAllowAllOrigins() bool {
	return srv.AllowAllOrigins
}
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) {
	srv.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (srv *server) GetAllowedOrigins() string {
	return srv.AllowedOrigins
}

func (srv *server) SetAllowedOrigins(allowedOrigins string) {
	srv.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (srv *server) GetDomain() string {
	return srv.Domain
}
func (srv *server) SetDomain(domain string) {
	srv.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (srv *server) GetTls() bool {
	return srv.TLS
}
func (srv *server) SetTls(hasTls bool) {
	srv.TLS = hasTls
}

// The certificate authority file
func (srv *server) GetCertAuthorityTrust() string {
	return srv.CertAuthorityTrust
}
func (srv *server) SetCertAuthorityTrust(ca string) {
	srv.CertAuthorityTrust = ca
}

// The certificate file.
func (srv *server) GetCertFile() string {
	return srv.CertFile
}
func (srv *server) SetCertFile(certFile string) {
	srv.CertFile = certFile
}

// The key file.
func (srv *server) GetKeyFile() string {
	return srv.KeyFile
}
func (srv *server) SetKeyFile(keyFile string) {
	srv.KeyFile = keyFile
}

// The service version
func (srv *server) GetVersion() string {
	return srv.Version
}
func (srv *server) SetVersion(version string) {
	srv.Version = version
}

// The publisher id.
func (srv *server) GetPublisherId() string {
	return srv.PublisherId
}
func (srv *server) SetPublisherId(publisherId string) {
	srv.PublisherId = publisherId
}

func (srv *server) GetKeepUpToDate() bool {
	return srv.KeepUpToDate
}
func (srv *server) SetKeepUptoDate(val bool) {
	srv.KeepUpToDate = val
}

func (srv *server) GetKeepAlive() bool {
	return srv.KeepAlive
}
func (srv *server) SetKeepAlive(val bool) {
	srv.KeepAlive = val
}

func (srv *server) GetPermissions() []interface{} {
	return srv.Permissions
}
func (srv *server) SetPermissions(permissions []interface{}) {
	srv.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (srv *server) Init() error {

	// Get the configuration path.
	err := globular.InitService(srv)
	if err != nil {
		return err
	}

	// Initialyse GRPC srv.
	srv.grpcServer, err = globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	return nil

}

// Save the configuration values.
func (srv *server) Save() error {
	// Create the file...
	return globular.SaveService(srv)
}

func (srv *server) StartService() error {
	return globular.StartService(srv, srv.grpcServer)
}

func (srv *server) StopService() error {
	return globular.StopService(srv, srv.grpcServer)
}

func (srv *server) Stop(context.Context, *ldappb.StopRequest) (*ldappb.StopResponse, error) {
	return &ldappb.StopResponse{}, srv.StopService()
}

/**
 * Connect to a ldap srv...
 */
func (srv *server) connect(id string, userId string, pwd string) (*ldap.Conn, error) {

	// The info must be set before that function is call.
	info := srv.Connections[id]

	conn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", info.Host, info.Port))
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

// /////////////////// resource service functions ////////////////////////////////////
func (srv *server) getResourceClient() (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(srv.Address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

// Create an empty group.
func (srv *server) createGroup(token, id, name, description string) error {
	resourceClient, err := srv.getResourceClient()
	if err != nil {
		return err
	}
	return resourceClient.CreateGroup(token, id, name, description)
}

// Register a new user.
func (srv *server) registerAccount(domain, id, name, email, password string) error {
	resourceClient, err := srv.getResourceClient()
	if err != nil {
		return err
	}

	return resourceClient.RegisterAccount(domain, id, name, email, password, password)
}

func (srv *server) addGroupMemberAccount(token, groupId string, accountId string) error {
	resourceClient, err := srv.getResourceClient()
	if err != nil {
		return err
	}
	return resourceClient.AddGroupMemberAccount(token, groupId, accountId)
}

func (srv *server) removeGroupMemberAccount(token, groupId string, accountId string) error {
	resourceClient, err := srv.getResourceClient()
	if err != nil {
		return err
	}
	return resourceClient.RemoveGroupMemberAccount(token, groupId, accountId)
}

func (srv *server) getAccount(id string) (*resourcepb.Account, error) {
	resourceClient, err := srv.getResourceClient()
	if err != nil {
		return nil, err
	}

	return resourceClient.GetAccount(id)
}

func (srv *server) getGroup(id string) (*resourcepb.Group, error) {
	resourceClient, err := srv.getResourceClient()
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
func (srv *server) Synchronize(ctx context.Context, rqst *ldappb.SynchronizeRequest) (*ldappb.SynchronizeResponse, error) {
	err := srv.synchronize()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &ldappb.SynchronizeResponse{}, nil
}

// Append synchronize information.
//
//	"LdapSyncInfos": {
//	    "my__ldap":
//	      {
//	        "ConnectionId": "my__ldap",
//	        "GroupSyncInfo": {
//	          "Base": "OU=Access_Groups,OU=Groups,OU=MON,OU=CA,DC=UD6,DC=UF6",
//	          "Id": "name",
//	          "Query": "((objectClass=group))"
//	        },
//	        "Refresh": 1,
//	        "UserSyncInfo": {
//	          "Base": "OU=Users,OU=MON,OU=CA,DC=UD6,DC=UF6",
//	          "Email": "mail",
//	          "Id": "userPrincipalName",
//	          "Query": "(|(objectClass=person)(objectClass=user))"
//	        }
//	      }
//	 }
func (srv *server) SetLdapSyncInfo(ctx context.Context, rqst *ldappb.SetLdapSyncInfoRequest) (*ldappb.SetLdapSyncInfoResponse, error) {
	info := make(map[string]interface{}, 0)

	info["ConnectionId"] = rqst.Info.ConnectionId
	info["Refresh"] = rqst.Info.Refresh
	info["GroupSyncInfo"] = make(map[string]interface{}, 0)
	info["GroupSyncInfo"].(map[string]interface{})["Id"] = rqst.Info.GroupSyncInfo.Id
	info["GroupSyncInfo"].(map[string]interface{})["Base"] = rqst.Info.GroupSyncInfo.Base
	info["GroupSyncInfo"].(map[string]interface{})["Query"] = rqst.Info.GroupSyncInfo.Query
	info["UserSyncInfo"] = make(map[string]interface{}, 0)
	info["UserSyncInfo"].(map[string]interface{})["Id"] = rqst.Info.UserSyncInfo.Id
	info["UserSyncInfo"].(map[string]interface{})["Base"] = rqst.Info.UserSyncInfo.Base
	info["UserSyncInfo"].(map[string]interface{})["Query"] = rqst.Info.UserSyncInfo.Query

	if srv.LdapSyncInfos == nil {
		srv.LdapSyncInfos = make(map[string]interface{}, 0)
	}

	// Store the info.
	srv.LdapSyncInfos[rqst.Info.ConnectionId] = info

	return &ldappb.SetLdapSyncInfoResponse{}, nil
}

// Delete synchronize information
func (srv *server) DeleteLdapSyncInfo(ctx context.Context, rqst *ldappb.DeleteLdapSyncInfoRequest) (*ldappb.DeleteLdapSyncInfoResponse, error) {
	if srv.LdapSyncInfos != nil {
		if srv.LdapSyncInfos[rqst.Id] != nil {
			delete(srv.LdapSyncInfos, rqst.Id)
		}
	}

	return &ldappb.DeleteLdapSyncInfoResponse{}, nil
}

// Retreive synchronize informations
func (srv *server) GetLdapSyncInfo(ctx context.Context, rqst *ldappb.GetLdapSyncInfoRequest) (*ldappb.GetLdapSyncInfoResponse, error) {
	infos := make([]*ldappb.LdapSyncInfo, 0)

	for _, info := range srv.LdapSyncInfos {
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
		} else {
			// append all infos.
			infos = append(infos, info_)
		}
	}
	// return the
	return &ldappb.GetLdapSyncInfoResponse{Infos: infos}, nil
}

// Authenticate a user with LDAP srv.
func (srv *server) Authenticate(ctx context.Context, rqst *ldappb.AuthenticateRqst) (*ldappb.AuthenticateRsp, error) {
	id := rqst.Id
	login := rqst.Login
	pwd := rqst.Pwd

	if len(id) > 0 {
		// I will made use of bind to authenticate the user.
		_, err := srv.connect(id, login, pwd)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		for id, _ := range srv.Connections {
			_, err := srv.connect(id, login, pwd)
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
func (srv *server) CreateConnection(ctx context.Context, rsqt *ldappb.CreateConnectionRqst) (*ldappb.CreateConnectionRsp, error) {
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
	srv.Connections[c.Id] = c

	c.conn, err = srv.connect(c.Id, c.User, c.Password)
	defer c.conn.Close()

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// In that case I will save it in file.
	err = srv.Save()
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
func (srv *server) DeleteConnection(ctx context.Context, rqst *ldappb.DeleteConnectionRqst) (*ldappb.DeleteConnectionRsp, error) {

	id := rqst.GetId()
	if _, ok := srv.Connections[id]; !ok {
		return &ldappb.DeleteConnectionRsp{
			Result: true,
		}, nil
	}

	if srv.Connections[id].conn != nil {
		// Close the connection.
		srv.Connections[id].conn.Close()
	}

	delete(srv.Connections, id)

	// In that case I will save it in file.
	err := srv.Save()
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
func (srv *server) Close(ctx context.Context, rqst *ldappb.CloseRqst) (*ldappb.CloseRsp, error) {
	id := rqst.GetId()
	if _, ok := srv.Connections[id]; !ok {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Connection "+id+" dosent exist!")))
	}

	srv.Connections[id].conn.Close()

	// return success.
	return &ldappb.CloseRsp{
		Result: true,
	}, nil
}

// Synchronize the list user and group whith resources...
// Here is an exemple of ldap configuration...
//
//	"LdapSyncInfos": {
//	    "my__ldap":
//	      {
//	        "ConnectionId": "my__ldap",
//	        "GroupSyncInfo": {
//	          "Base": "OU=Access_Groups,OU=Groups,OU=MON,OU=CA,DC=UD6,DC=UF6",
//	          "Id": "name",
//	          "Query": "((objectClass=group))"
//	        },
//	        "Refresh": 1,
//	        "UserSyncInfo": {
//	          "Base": "OU=Users,OU=MON,OU=CA,DC=UD6,DC=UF6",
//	          "Email": "mail",
//	          "Id": "userPrincipalName",
//	          "Query": "(|(objectClass=person)(objectClass=user))"
//	        }
//	      }
//	 }
func (srv *server) synchronize() error {

	// Here I will get the local token to generate the groups
	token, err := security.GetLocalToken(srv.Mac)
	if err != nil {
		return err
	}

	for connectionId, syncInfo_ := range srv.LdapSyncInfos {
		syncInfo := syncInfo_.(map[string]interface{})
		GroupSyncInfo := syncInfo["GroupSyncInfos"].(map[string]interface{})
		groupsInfo, err := srv.search(connectionId, GroupSyncInfo["Base"].(string), GroupSyncInfo["Query"].(string), []string{GroupSyncInfo["Id"].(string), "distinguishedName"})
		if err != nil {
			fmt.Println("fail to retreive group info", err)
			return err
		}

		// Print group info.
		for i := 0; i < len(groupsInfo); i++ {
			name := groupsInfo[i][0].([]string)[0]
			id := Utility.GenerateUUID(groupsInfo[i][1].([]string)[0])
			_, err := srv.getGroup(id)
			if err != nil {
				srv.createGroup(token, id, name, "") // The group member will be set latter in that function.
			}
		}

		// Synchronize account and user info...
		UserSyncInfo := syncInfo["UserSyncInfos"].(map[string]interface{})
		accountsInfo, err := srv.search(connectionId, UserSyncInfo["Base"].(string), UserSyncInfo["Query"].(string), []string{UserSyncInfo["Id"].(string), UserSyncInfo["Email"].(string), "distinguishedName", "memberOf"})
		if err != nil {
			fmt.Println("fail to retreive account info", err)
			return err
		}

		for i := 0; i < len(accountsInfo); i++ {
			// Print the list of account...
			// I will not set the password...
			if len(accountsInfo[i]) > 0 {
				if len(accountsInfo[i][0].([]string)) > 0 {
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
								a, err := srv.getAccount(id)
								if err != nil {
									err := srv.registerAccount(srv.Domain, id, name, email, id)
									if err != nil {
										fmt.Println("fail to register account ", id, err)
									}
								}

								a, err = srv.getAccount(id)

								// Now I will update the groups user list...
								if err == nil {
									if len(accountsInfo[i][3].([]string)) > 0 && a != nil {
										groups := accountsInfo[i][3].([]string)
										// Append not existing group...
										for j := 0; j < len(groups); j++ {
											groupId := Utility.GenerateUUID(groups[j])

											if !Utility.Contains(a.Groups, groupId) {
												// Now I will remo
												err := srv.addGroupMemberAccount(token, groupId, a.Id)
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
												err := srv.removeGroupMemberAccount(token, groupId, a.Id)
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
		}
	}

	// No error...
	return nil
}

// The path of the configuration.
func (srv *server) GetConfigurationPath() string {
	return srv.ConfigPath
}

func (srv *server) SetConfigurationPath(path string) {
	srv.ConfigPath = path
}

/**
 * Search for a list of value over the ldap srv. if the base_dn is
 * not specify the default base is use. It return a list of values. This can
 * be interpret as a tow dimensional array.
 */
func (srv *server) search(id string, base_dn string, filter string, attributes []string) ([][]interface{}, error) {

	if _, ok := srv.Connections[id]; !ok {
		return nil, errors.New("Connection " + id + " dosent exist!")
	}

	// create the connection.
	c := srv.Connections[id]
	conn, err := srv.connect(id, srv.Connections[id].User, srv.Connections[id].Password)
	if err != nil {
		return nil, err
	}

	c.conn = conn
	srv.Connections[id] = c

	// close connection after search.
	defer c.conn.Close()

	//Now I will execute the query...
	search_request := ldap.NewSearchRequest(
		base_dn,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		attributes,
		nil)

	// Create simple search.
	sr, err := srv.Connections[id].conn.Search(search_request)

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

// Search over LDAP srv.
func (srv *server) Search(ctx context.Context, rqst *ldappb.SearchRqst) (*ldappb.SearchResp, error) {
	id := rqst.Search.GetId()
	if _, ok := srv.Connections[id]; !ok {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Connection "+id+" dosent exist!")))
	}

	results, err := srv.search(id, rqst.Search.BaseDN, rqst.Search.Filter, rqst.Search.Attributes)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Here I got the results.
	str, err := Utility.ToJson(results)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &ldappb.SearchResp{
		Result: string(str),
	}, nil
}

///////////////////////////////////////////////////////////////
// LDAP server handler
///////////////////////////////////////////////////////////////

// handleBind return Success
func handleBind(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	r := m.GetBindRequest()
	res := ldapserver.NewBindResponse(ldapserver.LDAPResultSuccess)

	if string(r.Name()) == "myLogin" {
		w.Write(res)
		return
	}

	// Parse the DN string
	parsedDN, err := ldap.ParseDN(string(r.Name()))
	if err != nil {
		fmt.Println("Error parsing DN:", err)
		return
	}

	// Access individual components of the DN
	for _, val := range parsedDN.RDNs {
		for _, av := range val.Attributes {
			fmt.Printf("DN: %s = %s\n", av.Type, av.Value)
		}
	}

	fmt.Printf("Bind failed User=%s, Pass=%s", string(r.Name()), string(r.AuthenticationSimple()))

	res.SetResultCode(ldap.LDAPResultInvalidCredentials)
	res.SetDiagnosticMessage("invalid credentials")
	w.Write(res)
}

// handleSearch return Success
func handleSearch(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	r := m.GetSearchRequest()

	res := ldapserver.NewSearchResultDoneResponse(ldapserver.LDAPResultSuccess)

	fmt.Printf("Search: BaseDN=%s, Filter=%s", r.BaseObject(), r.Filter())

	w.Write(res)
}

// handleAdd return Success
func handleAdd(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	r := m.GetAddRequest()

	res := ldapserver.NewAddResponse(ldapserver.LDAPResultSuccess)

	fmt.Println("Add: DN=%s, Attributes=%v", r.Entry(), r.Attributes())

	w.Write(res)
}

// handleModify return Success
func handleModify(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	r := m.GetModifyRequest()

	res := ldapserver.NewModifyResponse(ldapserver.LDAPResultSuccess)

	fmt.Println("Modify: DN=%s, Changes=%v", r.Object(), r.Changes())

	w.Write(res)
}

// handleDelete return Success
func handleDelete(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	r := m.GetDeleteRequest()

	res := ldapserver.NewDeleteResponse(ldapserver.LDAPResultSuccess)

	fmt.Println("Delete: DN=%s", r)

	w.Write(res)
}

// handleAbandon return Success
func handleAbandon(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	r := m.GetAbandonRequest()

	res := ldapserver.NewResponse(ldapserver.LDAPResultSuccess)

	fmt.Println("Abandon: MessageID=%d", r)

	w.Write(res)
}

// handleExtendedRequest return Success
func handleExtendedRequest(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	r := m.GetExtendedRequest()

	res := ldapserver.NewExtendedResponse(ldapserver.LDAPResultSuccess)

	fmt.Println("ExtendedRequest: OID=%s, Value=%s", r.RequestName(), r.RequestValue())

	w.Write(res)
}

// handleCompare return Success
func handleCompare(w ldapserver.ResponseWriter, m *ldapserver.Message) {
	r := m.GetCompareRequest()

	res := ldapserver.NewCompareResponse(ldapserver.LDAPResultSuccess)

	fmt.Println("Compare response", r)

	w.Write(res)
}

// Start the ldap server.
func (srv *server) startLdapServer() {

	//ldap logger
	ldapserver.Logger = log.New(os.Stdout, "[server] ", log.LstdFlags)

	//Create a new LDAP Server
	server := ldapserver.NewServer()

	routes := ldapserver.NewRouteMux()

	// Attach routes to server
	routes.Bind(handleBind)
	routes.Search(handleSearch)
	routes.Add(handleAdd)
	routes.Modify(handleModify)
	routes.Delete(handleDelete)
	routes.Abandon(handleAbandon)
	routes.Extended(handleExtendedRequest)
	routes.Compare(handleCompare)

	// Attach route mux to LDAP Server
	server.Handle(routes)

	// if TLS is enabled
	if srv.TLS {

		secureConn := func(s *ldapserver.Server) {
			config := globular_service.GetTLSConfig(srv.GetKeyFile(), srv.GetCertFile(), srv.GetCertAuthorityTrust())
			s.Listener = tls.NewListener(s.Listener, config)
		}

		go server.ListenAndServe("0.0.0.0:636", secureConn)

	} else {
		go server.ListenAndServe("0.0.0.0:389")
	}

	// When CTRL+C, SIGINT and SIGTERM signal occurs
	// Then stop server gracefully
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	close(ch)

	server.Stop()
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
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.PublisherId = "localhost"
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true

	// Give base info to retreive it configuration.
	if len(os.Args) == 2 {
		s_impl.Id = os.Args[1] // The second argument must be the port number
	} else if len(os.Args) == 3 {
		s_impl.Id = os.Args[1]         // The second argument must be the port number
		s_impl.ConfigPath = os.Args[2] // The second argument must be the port number
	}

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}

	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
	}

	// Register the echo services
	ldappb.RegisterLdapServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	/*
		go func() {
			s_impl.synchronize()
		}()
	*/

	// Start the ldap server.
	go func() {
		s_impl.startLdapServer()
	}()

	// Start the service.
	s_impl.StartService()

}
