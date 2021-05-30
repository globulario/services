package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/davecourtois/Utility"
	"encoding/json"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/persistence/persistence_store"
	"google.golang.org/protobuf/reflect/protoreflect"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The default values.
var (
	defaultPort  = 10029
	defaultProxy = 10030

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""

	domain string = "localhost"
)

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id              string
	Name            string
	Domain          string
	Path            string
	Proto           string
	Port            int
	Proxy           int
	AllowAllOrigins bool
	AllowedOrigins  string // comma separated string.
	Protocol        string
	Version         string
	PublisherId     string
	KeepUpToDate    bool
	KeepAlive       bool
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string

	TLS bool

	// svr-signed X.509 public keys for distribution
	CertFile string

	// a private RSA key to sign and authenticate the public key
	KeyFile string

	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string

	Permissions []interface{} // contains the action permission for the services.

	// The backend infos.
	Backend_address  string
	Backend_port     int64
	Backend_user     string
	Backend_password string

	// The session time out.
	SessionTimeout time.Duration

	jwtKey []byte // This is the client secret.

	// Data store where account, role ect are keep...
	store persistence_store.Store

	// The grpc server.
	grpcServer *grpc.Server
}

// Globular services implementation...
// The id of a particular service instance.
func (svr *server) GetId() string {
	return svr.Id
}
func (svr *server) SetId(id string) {
	svr.Id = id
}

// The name of a service, must be the gRpc Service name.
func (svr *server) GetName() string {
	return svr.Name
}
func (svr *server) SetName(name string) {
	svr.Name = name
}

// The description of the service
func (svr *server) GetDescription() string {
	return svr.Description
}
func (svr *server) SetDescription(description string) {
	svr.Description = description
}

// The list of keywords of the services.
func (svr *server) GetKeywords() []string {
	return svr.Keywords
}
func (svr *server) SetKeywords(keywords []string) {
	svr.Keywords = keywords
}

func (svr *server) GetRepositories() []string {
	return svr.Repositories
}
func (svr *server) SetRepositories(repositories []string) {
	svr.Repositories = repositories
}

func (svr *server) GetDiscoveries() []string {
	return svr.Discoveries
}
func (svr *server) SetDiscoveries(discoveries []string) {
	svr.Discoveries = discoveries
}

// Dist
func (svr *server) Dist(path string) (string, error) {

	return globular.Dist(path, svr)
}

func (svr *server) GetPlatform() string {
	return globular.GetPlatform()
}

// The path of the executable.
func (svr *server) GetPath() string {
	return svr.Path
}
func (svr *server) SetPath(path string) {
	svr.Path = path
}

// The path of the .proto file.
func (svr *server) GetProto() string {
	return svr.Proto
}
func (svr *server) SetProto(proto string) {
	svr.Proto = proto
}

// The gRpc port.
func (svr *server) GetPort() int {
	return svr.Port
}
func (svr *server) SetPort(port int) {
	svr.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (svr *server) GetProxy() int {
	return svr.Proxy
}
func (svr *server) SetProxy(proxy int) {
	svr.Proxy = proxy
}

// Can be one of http/https/tls
func (svr *server) GetProtocol() string {
	return svr.Protocol
}
func (svr *server) SetProtocol(protocol string) {
	svr.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (svr *server) GetAllowAllOrigins() bool {
	return svr.AllowAllOrigins
}
func (svr *server) SetAllowAllOrigins(allowAllOrigins bool) {
	svr.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (svr *server) GetAllowedOrigins() string {
	return svr.AllowedOrigins
}

func (svr *server) SetAllowedOrigins(allowedOrigins string) {
	svr.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (svr *server) GetDomain() string {
	return svr.Domain
}
func (svr *server) SetDomain(domain string) {
	svr.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (svr *server) GetTls() bool {
	return svr.TLS
}
func (svr *server) SetTls(hasTls bool) {
	svr.TLS = hasTls
}

// The certificate authority file
func (svr *server) GetCertAuthorityTrust() string {
	return svr.CertAuthorityTrust
}
func (svr *server) SetCertAuthorityTrust(ca string) {
	svr.CertAuthorityTrust = ca
}

// The certificate file.
func (svr *server) GetCertFile() string {
	return svr.CertFile
}
func (svr *server) SetCertFile(certFile string) {
	svr.CertFile = certFile
}

// The key file.
func (svr *server) GetKeyFile() string {
	return svr.KeyFile
}
func (svr *server) SetKeyFile(keyFile string) {
	svr.KeyFile = keyFile
}

// The service version
func (svr *server) GetVersion() string {
	return svr.Version
}
func (svr *server) SetVersion(version string) {
	svr.Version = version
}

// The publisher id.
func (svr *server) GetPublisherId() string {
	return svr.PublisherId
}
func (svr *server) SetPublisherId(publisherId string) {
	svr.PublisherId = publisherId
}

func (svr *server) GetKeepUpToDate() bool {
	return svr.KeepUpToDate
}
func (svr *server) SetKeepUptoDate(val bool) {
	svr.KeepUpToDate = val
}

func (svr *server) GetKeepAlive() bool {
	return svr.KeepAlive
}
func (svr *server) SetKeepAlive(val bool) {
	svr.KeepAlive = val
}

func (svr *server) GetPermissions() []interface{} {
	return svr.Permissions
}
func (svr *server) SetPermissions(permissions []interface{}) {
	svr.Permissions = permissions
}

// Use rbac client here...
func (svr *server) addResourceOwner(path string, subject string, subjectType rbacpb.SubjectType) error {
	return nil
}

func (svr *server) getActionResourcesPermissions(action string) ([]*rbacpb.ResourceInfos, error) {

	return nil, nil
}

func (svr *server) validateAction(method string, subject string, subjectType rbacpb.SubjectType, infos []*rbacpb.ResourceInfos) (bool, error) {
	return false, nil
}

func (svr *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	return errors.New("not implemented")
}

// Create the configuration file if is not already exist.
func (svr *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewresourceService_Client", resource_client.NewResourceService_Client)

	// Get the configuration path.
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	err := globular.InitService(dir+"/config.json", svr)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	svr.grpcServer, err = globular.InitGrpcServer(svr, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	return nil

}

// Save the configuration values.
func (svr *server) Save() error {
	// Create the file...
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	return globular.SaveService(dir+"/config.json", svr)
}

func (svr *server) StartService() error {
	return globular.StartService(svr, svr.grpcServer)
}

func (svr *server) StopService() error {
	return globular.StopService(svr, svr.grpcServer)
}

/**
 * Connection to mongo db local store.
 */
func (svr *server) getPersistenceStore() (persistence_store.Store, error) {
	// That service made user of persistence service.
	if svr.store == nil {
		svr.store = new(persistence_store.MongoStore)
		err := svr.store.Connect("local_resource", svr.Backend_address, int32(svr.Backend_port), svr.Backend_user, svr.Backend_password, "local_resource", 5000, "")
		if err != nil {
			return nil, err
		}
	}
	return svr.store, nil
}

/**
 *  hashPassword return the bcrypt hash of the password.
 */
func (resource_server *server) hashPassword(password string) (string, error) {
	haspassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(haspassword), nil
}

func (resource_server *server) registerAccount(id string, name string, email string, password string, organizations []string, contacts []string, roles []string, groups []string) error {

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	// first of all the Persistence service must be active.
	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Accounts", `{"$or":[{"_id":"`+id+`"},{"name":"`+id+`"} ]}`, "")
	if err != nil {
		return err
	}

	// one account already exist for the name.
	if count == 1 {
		return errors.New("account with name " + name + " already exist!")
	}

	// set the account object and set it basic roles.
	account := make(map[string]interface{}, 0)
	account["_id"] = id
	account["name"] = name
	account["email"] = email
	account["password"], _ = resource_server.hashPassword(password) // hide the password...

	// List of aggregation.
	account["roles"] = make([]interface{}, 0)
	account["groups"] = make([]interface{}, 0)
	account["organizations"] = make([]interface{}, 0)

	// append guest role if not already exist.
	if !Utility.Contains(roles, "guest") {
		roles = append(roles, "guest")
	}

	// Here I will insert the account in the database.
	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Accounts", account, "")
	if err != nil {
		return err
	}

	// replace @ and . by _
	name = strings.ReplaceAll(strings.ReplaceAll(name, "@", "_"), ".", "_")

	// Each account will have their own database and a use that can read and write
	// into it.
	// Here I will wrote the script for mongoDB...
	createUserScript := fmt.Sprintf("db=db.getSiblingDB('%s_db');db.createCollection('user_data');db=db.getSiblingDB('admin');db.createUser({user: '%s', pwd: '%s',roles: [{ role: 'dbOwner', db: '%s_db' }]});", name, name, password, name)

	// I will execute the sript with the admin function.
	err = p.RunAdminCmd(context.Background(), "local_resource", resource_server.Backend_user, resource_server.Backend_password, createUserScript)
	if err != nil {
		return err
	}

	/* TODO create connection...
	err = p.CreateConnection(name+"_db", name+"_db", address, float64(port), 0, name, password, 5000, "", false)
	if err != nil {
		return errors.New("no persistence service are available to store resource information")
	}
	*/

	// Now I will set the reference for
	// Contact...
	for i := 0; i < len(contacts); i++ {
		resource_server.addAccountContact(id, contacts[i])
	}

	// Organizations
	for i := 0; i < len(organizations); i++ {
		resource_server.createCrossReferences(organizations[i], "Organizations", "accounts", id, "Accounts", "organizations")
	}

	// Roles
	for i := 0; i < len(roles); i++ {
		resource_server.createCrossReferences(roles[i], "Roles", "members", id, "Accounts", "roles")
	}

	// Groups
	for i := 0; i < len(groups); i++ {
		resource_server.createCrossReferences(groups[i], "Groups", "members", id, "Accounts", "groups")
	}

	return nil

}

func (resource_server *server) deleteReference(p persistence_store.Store, refId, targetId, targetField, targetCollection string) error {

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", targetCollection, `{"_id":"`+targetId+`"}`, ``)
	if err != nil {
		return err
	}

	target := values.(map[string]interface{})

	if target[targetField] == nil {
		return errors.New("No field named " + targetField + " was found in object with id " + targetId + "!")
	}

	references := []interface{}(target[targetField].(primitive.A))
	references_ := make([]interface{}, 0)
	for j := 0; j < len(references); j++ {
		if references[j].(map[string]interface{})["$id"] != refId {
			references_ = append(references_, references[j])
		}
	}

	target[targetField] = references_
	jsonStr := serialyseObject(target)

	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", targetCollection, `{"_id":"`+targetId+`"}`, jsonStr, ``)
	if err != nil {
		return err
	}

	return nil
}

func (resource_server *server) createReference(p persistence_store.Store, id, sourceCollection, field, targetId, targetCollection string) error {
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", sourceCollection, `{"_id":"`+id+`"}`, ``)
	if err != nil {
		return err
	}

	log.Println("create reference ", id, " source ", sourceCollection, " field ", field, " field ", targetId, " target ", targetCollection)
	source := values.(map[string]interface{})
	references := make([]interface{}, 0)
	if source[field] != nil {
		references = []interface{}(source[field].(primitive.A))
	}

	for j := 0; j < len(references); j++ {
		if references[j].(map[string]interface{})["$id"] == targetId {
			return errors.New(" named " + targetId + " aleready exist in  " + field + "!")
		}
	}

	// append the account.
	source[field] = append(references, map[string]interface{}{"$ref": targetCollection, "$id": targetId, "$db": "local_resource"})
	jsonStr := serialyseObject(source)

	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", sourceCollection, `{"_id":"`+id+`"}`, jsonStr, ``)
	if err != nil {
		return err
	}

	return nil
}

func (resource_server *server) createCrossReferences(sourceId, sourceCollection, sourceField, targetId, targetCollection, targetField string) error {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	err = resource_server.createReference(p, targetId, targetCollection, targetField, sourceId, sourceCollection)
	if err != nil {
		//return err
		log.Println(err)
	}

	err = resource_server.createReference(p, sourceId, sourceCollection, sourceField, targetId, targetCollection)

	return err

}


//////////////////////////// Loggin info ///////////////////////////////////////
func (resource_server *server) validateActionRequest(rqst interface{}, method string, subject string, subjectType rbacpb.SubjectType) (bool, error) {

	infos, err := resource_server.getActionResourcesPermissions(method)
	if err != nil {
		infos = make([]*rbacpb.ResourceInfos, 0)
	} else {
		// Here I will get the params...
		val, _ := Utility.CallMethod(rqst, "ProtoReflect", []interface{}{})
		rqst_ := val.(protoreflect.Message)
		if rqst_.Descriptor().Fields().Len() > 0 {
			for i := 0; i < len(infos); i++ {
				// Get the path value from retreive infos.
				param := rqst_.Descriptor().Fields().Get(Utility.ToInt(infos[i].Index))
				path, _ := Utility.CallMethod(rqst, "Get"+strings.ToUpper(string(param.Name())[0:1])+string(param.Name())[1:], []interface{}{})
				infos[i].Path = path.(string)
			}
		}
	}

	return resource_server.validateAction(method, subject, rbacpb.SubjectType_ACCOUNT, infos)
}

// That function is necessary to serialyse reference and kept field orders
func serialyseObject(obj map[string]interface{}) string {
	// Here I will save the role.
	jsonStr, _ := Utility.ToJson(obj)
	jsonStr = strings.ReplaceAll(jsonStr, `"$ref"`, `"__a__"`)
	jsonStr = strings.ReplaceAll(jsonStr, `"$id"`, `"__b__"`)
	jsonStr = strings.ReplaceAll(jsonStr, `"$db"`, `"__c__"`)

	obj_ := make(map[string]interface{}, 0)

	json.Unmarshal([]byte(jsonStr), &obj_)
	jsonStr, _ = Utility.ToJson(obj_)
	jsonStr = strings.ReplaceAll(jsonStr, `"__a__"`, `"$ref"`)
	jsonStr = strings.ReplaceAll(jsonStr, `"__b__"`, `"$id"`)
	jsonStr = strings.ReplaceAll(jsonStr, `"__c__"`, `"$db"`)

	return jsonStr
}


// unaryInterceptor calls authenticateClient with current context
func (resource_server *server) unaryResourceInterceptor(ctx context.Context, rqst interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	method := info.FullMethod

	// The token and the application id.
	var token string
	var application string

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		application = strings.Join(md["application"], "")
		token = strings.Join(md["token"], "")
	}

	hasAccess := false

	// If the address is local I will give the permission.
	/* Test only
	peer_, _ := peer.FromContext(ctx)
	address := peer_.Addr.String()
	address = address[0:strings.Index(address, ":")]
	if Utility.IsLocal(address) {
		hasAccess = true
	}*/

	var err error
	// Here some method are accessible by default.
	if method == "/admin.adminService/GetConfig" ||
		method == "/admin.adminService/DownloadGlobular" ||
		method == "/resource.ResourceService/GetAllActions" ||
		method == "/resource.ResourceService/RegisterAccount" ||
		method == "/resource.ResourceService/GetAccounts" ||
		method == "/resource.ResourceService/RegisterPeer" ||
		method == "/resource.ResourceService/GetPeers" ||
		method == "/resource.ResourceService/AccountExist" ||
		method == "/resource.ResourceService/Authenticate" ||
		method == "/resource.ResourceService/RefreshToken" ||
		method == "/resource.ResourceService/GetAllFilesInfo" ||
		method == "/resource.ResourceService/GetAllApplicationsInfo" ||
		method == "/resource.ResourceService/ValidateToken" ||
		method == "/rbac.RbacService/ValidateAction" ||
		method == "/rbac.RbacService/ValidateAccess" ||
		method == "/rbac.RbacService/GetResourcePermissions" ||
		method == "/rbac.RbacService/GetResourcePermission" ||
		method == "/log.LogService/Log" ||
		method == "/log.LogService/DeleteLog" ||
		method == "/log.LogService/GetLog" ||
		method == "/log.LogService/ClearAllLog" {
		hasAccess = true
	}

	var clientId string

	// Test if the user has access to execute the method
	if len(token) > 0 && !hasAccess {
		var expiredAt int64
		var err error

		/*clientId*/
		clientId, _, _, expiredAt, err = interceptors.ValidateToken(token)
		if err != nil {
			return nil, err
		}

		if expiredAt < time.Now().Unix() {
			return nil, errors.New("the token is expired")
		}

		hasAccess = clientId == "sa"
		if !hasAccess {
			hasAccess, _ = resource_server.validateActionRequest(rqst, method, clientId, rbacpb.SubjectType_ACCOUNT)
		}
	}

	// Test if the application has access to execute the method.
	if len(application) > 0 && !hasAccess {
		// TODO validate rpc method access
		hasAccess, _ = resource_server.validateActionRequest(rqst, method, application, rbacpb.SubjectType_APPLICATION)
	}

	if !hasAccess {
		err := errors.New("Permission denied to execute method " + method + " user:" + clientId + " application:" + application)
		return nil, err
	}

	// Execute the action.
	result, err := handler(ctx, rqst)
	return result, err
}

// Stream interceptor.
func (resource_server *server) streamResourceInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {

	err := handler(srv, stream)
	if err != nil {
		return err
	}

	return nil
}

func (resource_server *server) addAccountContact(accountId string, contactId string) error {

	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	err = resource_server.createReference(p, accountId, "Accounts", "contacts", contactId, "Accounts")
	if err != nil {
		return err
	}

	return nil
}

func (resource_server *server) createGroup(id, name string, members []string) error {
	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	// Here I will first look if a peer with a same name already exist on the
	// resources...
	count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Groups", `{"_id":"`+id+`"}`, "")
	if count > 0 {
		return errors.New("Group with name '" + id + "' already exist!")
	}

	// No authorization exist for that peer I will insert it.
	// Here will create the new peer.
	g := make(map[string]interface{}, 0)
	g["_id"] = id
	g["name"] = name

	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Groups", g, "")
	if err != nil {
		return err
	}

	// Create references.
	for i := 0; i < len(members); i++ {
		err := resource_server.createCrossReferences(id, "Groups", "members", members[i], "Accounts", "groups")
		if err != nil {
			log.Println(err)
		}
	}

	return nil
}

func (resource_server *server) createRole(id string, name string, actions []string) error {
	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	// Here I will test if a newer token exist for that user if it's the case
	// I will not refresh that token.
	_, err = p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+id+`"}`, ``)
	if err == nil {
		return errors.New("Role named " + name + "already exist!")
	}

	// Here will create the new role.
	role := make(map[string]interface{}, 0)
	role["_id"] = id
	role["name"] = name
	role["actions"] = actions

	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Roles", role, "")
	if err != nil {
		return err
	}

	return nil
}

/**
 * Delete application Data from the backend.
 */
 func (resource_server *server) deleteApplication(applicationId string) error {

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	// Here I will test if a newer token exist for that user if it's the case
	// I will not refresh that token.
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+applicationId+`"}`, ``)
	if err != nil {
		return err
	}

	application := values.(map[string]interface{})

	// I will remove it from organization...
	if application["organizations"] != nil {
		organizations := []interface{}(application["organizations"].(primitive.A))

		for i := 0; i < len(organizations); i++ {
			organizationId := organizations[i].(map[string]interface{})["$id"].(string)
			resource_server.deleteReference(p, applicationId, organizationId, "applications", "Applications")
		}
	}

	// I will remove the directory.
	err = os.RemoveAll(application["path"].(string))
	if err != nil {
		return err
	}

	// Now I will remove the database create for the application.
	err = p.DeleteDatabase(context.Background(), "local_resource", applicationId+"_db")
	if err != nil {
		return err
	}

	// Finaly I will remove the entry in  the table.
	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+applicationId+`"}`, "")
	if err != nil {
		return err
	}

	// Delete permissions
	err = p.Delete(context.Background(), "local_resource", "local_resource", "Permissions", `{"owner":"`+applicationId+`"}`, "")
	if err != nil {
		return err
	}

	// Drop the application user.
	// Here I will drop the db user.
	dropUserScript := fmt.Sprintf(
		`db=db.getSiblingDB('admin');db.dropUser('%s', {w: 'majority', wtimeout: 4000})`,
		applicationId)

	// I will execute the sript with the admin function.
	err = p.RunAdminCmd(context.Background(), "local_resource", resource_server.Backend_user, resource_server.Backend_password, dropUserScript)
	if err != nil {
		return err
	}

	return nil
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Initialyse service with default values.
	s_impl := new(server)
	s_impl.Name = string(resourcepb.File_resource_proto.Services().Get(0).FullName())
	s_impl.Proto = resourcepb.File_resource_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario"
	s_impl.Description = "Resource manager service. Resources are Group, Account, Role, Organization and Peer."
	s_impl.Keywords = []string{"Resource"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins

	// Backend informations.
	s_impl.Backend_address = "localhost"
	s_impl.Backend_port = 27017
	s_impl.Backend_user = "sa"
	s_impl.Backend_password = "adminadmin"

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("fail to initialyse service %s: %s", s_impl.Name, s_impl.Id, err)
	}
	if len(os.Args) == 2 {
		s_impl.Port, _ = strconv.Atoi(os.Args[1]) // The second argument must be the port number
	}

	// Register the resource services
	resourcepb.RegisterResourceServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	s_impl.setActionResourcesPermissions(map[string]interface{}{"action": "/resource.ResourceService/DeletePermissions", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}})
	s_impl.setActionResourcesPermissions(map[string]interface{}{"action": "/resource.ResourceService/SetResourceOwner", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}})
	s_impl.setActionResourcesPermissions(map[string]interface{}{"action": "/resource.ResourceService/DeleteResourceOwner", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}})

	// Start the service.
	s_impl.StartService()

}
