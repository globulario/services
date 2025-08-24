package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	Utility "github.com/davecourtois/!utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/conversation/conversation_client"
	"github.com/globulario/services/golang/conversation/conversationpb"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	"google.golang.org/grpc/codes"

	//"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/search/search_engine"
	"github.com/globulario/services/golang/storage/storage_store"
)

// The default values.
var (
	defaultPort  = 10029
	defaultProxy = 10030

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""
)

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id              string
	Name            string
	Mac             string
	Domain          string
	Address         string
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
	Plaform         string
	Checksum        string
	KeepAlive       bool
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string
	Process         int
	ProxyProcess    int
	ConfigPath      string
	LastError       string
	State           string
	ModTime         int64

	// Specific configuration.
	Root string // Where to look for conversation data, file.. etc.

	// The port number where the sfu server will listen.
	// This work with ion-sfu, (Pion a webRTC framework), be sure sfu-v2 exec is in the path with
	// it configuration (config.toml) beside.
	PortSFU int

	TLS bool

	// svr-signed X.509 public keys for distribution
	CertFile string

	// a private RSA key to sign and authenticate the public key
	KeyFile string

	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string

	Permissions []interface{} // contains the action permission for the services.

	Dependencies []string // The list of services needed by this services.

	// The grpc server.
	grpcServer *grpc.Server

	// Use to sync conversations channel manipulation.
	actions chan map[string]interface{}

	// stop the processing loop.
	exit chan bool

	// The search engine..
	search_engine *search_engine.BleveSearchEngine

	// Store global conversation information like conversation owner's participant...
	store storage_store.Store

	// keep in map active conversation db connections.
	conversations *sync.Map
}

// The path of the configuration.
func (srv *server) GetConfigurationPath() string {
	return srv.ConfigPath
}

func (srv *server) SetConfigurationPath(path string) {
	srv.ConfigPath = path
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
	if pid == -1 && srv.store != nil {
		srv.store.Close()
	}
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

func (srv *server) GetMac() string {
	return srv.Mac
}

func (srv *server) SetMac(mac string) {
	srv.Mac = mac
}

// The description of the service
func (srv *server) GetDescription() string {
	return srv.Description
}
func (srv *server) SetDescription(description string) {
	srv.Description = description
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

// Singleton.
var (
	rbac_client_  *rbac_client.Rbac_Client
	log_client_   *log_client.Log_Client
	event_client_ *event_client.Event_Client
)

////////////////////////////////////////////////////////////////////////////////////////
// Logger function
////////////////////////////////////////////////////////////////////////////////////////
/**
 * Get the log client.
 */
func (srv *server) GetLogClient() (*log_client.Log_Client, error) {
	Utility.RegisterFunction("NewLogService_Client", log_client.NewLogService_Client)
	client, err := globular_client.GetClient(srv.Address, "log.LogService", "NewLogService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*log_client.Log_Client), nil
}

func (srv *server) logServiceInfo(method, fileLine, functionName, infos string) error {
	log_client_, err := srv.GetLogClient()
	if err != nil {
		return err
	}
	return log_client_.Log(srv.Name, srv.Domain, method, logpb.LogLevel_INFO_MESSAGE, infos, fileLine, functionName)
}

func (srv *server) logServiceError(method, fileLine, functionName, infos string) error {
	log_client_, err := srv.GetLogClient()
	if err != nil {
		return err
	}
	return log_client_.Log(srv.Name, srv.Address, method, logpb.LogLevel_ERROR_MESSAGE, infos, fileLine, functionName)
}

// /////////////////// resource service functions ////////////////////////////////////
func (srv *server) getEventClient() (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(srv.Address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

func (srv *server) publish(event string, data []byte) error {
	eventClient, err := srv.getEventClient()
	if err != nil {
		return err
	}
	return eventClient.Publish(event, data)
}

func (srv *server) subscribe(evt string, listener func(evt *eventpb.Event)) error {
	eventClient, err := srv.getEventClient()
	if err != nil {
		return err
	}

	// register a listener...
	return eventClient.Subscribe(evt, srv.Name, listener)
}

//////////////////////////////////////// RBAC Functions ///////////////////////////////////////////////
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

func (srv *server) deleteResourcePermissions(path string) error {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbac_client_.DeleteResourcePermissions(path)
}

func (srv *server) validateAccess(subject string, subjectType rbacpb.SubjectType, name string, path string) (bool, bool, error) {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return false, false, err
	}

	return rbac_client_.ValidateAccess(subject, subjectType, name, path)

}

func (srv *server) addResourceOwner(path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}

	return rbac_client_.AddResourceOwner(path, resourceType, subject, subjectType)
}

func (srv *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbac_client_.SetActionResourcesPermissions(permissions)
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

	// Initialyse the search engine.
	srv.search_engine = new(search_engine.BleveSearchEngine)

	// Create a new local store.
	srv.store = storage_store.NewBadger_store()
	return srv.store.Open(`{"path":"` + srv.Root + `", "name":"conversations"}`)
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
	srv.exit <- true
	return globular.StopService(srv, srv.grpcServer)
}

func (srv *server) Stop(context.Context, *conversationpb.StopRequest) (*conversationpb.StopResponse, error) {
	srv.exit <- true

	return &conversationpb.StopResponse{}, srv.StopService()
}

/////////////////////// Conversation specific function /////////////////////////////////

/**
 * Databases will be created in the 'conversations' directory inside the Root path
 * Each conversation will have it own leveldb database
 */
func (srv *server) getConversationConnection(id string) (*storage_store.Badger_store, error) {

	dbPath := srv.Root + "/conversations/" + id
	Utility.CreateDirIfNotExist(dbPath)

	connection, ok := srv.conversations.Load(dbPath)
	if !ok {
		connection = storage_store.NewBadger_store()
		srv.conversations.Store(dbPath, connection)
	}

	connection_ := connection.(*storage_store.Badger_store)

	return connection_, nil
}

func (srv *server) closeConversationConnection(id string) {

	dbPath := srv.Root + "/conversations/" + id
	connection, ok := srv.conversations.Load(dbPath)
	if !ok {
		return
	}

	// Close the connection.
	connection.(*storage_store.Badger_store).Close()

	defer srv.conversations.Delete(dbPath)
}

/////////////////////////// Public interfaces //////////////////////////////////

// Create a new conversation with a given name. The creator will became the
// owner of that conversation and he will be able to set permissions to
// determine who can participate to the conversation.
func (srv *server) CreateConversation(ctx context.Context, rqst *conversationpb.CreateConversationRequest) (*conversationpb.CreateConversationResponse, error) {

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	uuid := Utility.RandomUUID()

	if len(rqst.Language) == 0 {
		rqst.Language = "en"
	}

	conversation := &conversationpb.Conversation{
		Uuid:            uuid,
		Name:            rqst.Name,
		Keywords:        rqst.Keywords,
		CreationTime:    time.Now().Unix(),
		LastMessageTime: 0,
		Language:        rqst.Language,
		Participants:    []string{clientId},
		Mac:             srv.Mac,
	}

	err = srv.saveConversation(conversation)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// So here I will append the value in the index.
	err = srv.addParticipantConversation(clientId, uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.addResourceOwner(uuid, "conversation", clientId, rbacpb.SubjectType_ACCOUNT)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	fmt.Println("converstion was created: ", conversation)

	return &conversationpb.CreateConversationResponse{
		Conversation: conversation,
	}, nil
}

// Return the list of conversations created by a given user.
func (srv *server) getConversations(accountId string) (*conversationpb.Conversations, error) {

	_conversations_, err := srv.store.GetItem(accountId + "_conversations")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	_conversations := new(conversationpb.Conversations)
	_conversations.Conversations = make([]*conversationpb.Conversation, 0)

	uuids := make([]string, 0)

	json.Unmarshal(_conversations_, &uuids)
	for i := 0; i < len(uuids); i++ {
		conversation, err := srv.getConversation(uuids[i])
		if err == nil {
			_conversations.Conversations = append(_conversations.Conversations, conversation)
		}
	}

	return _conversations, nil
}

// Return the list of conversations created by a given user.
func (srv *server) GetConversations(ctx context.Context, rqst *conversationpb.GetConversationsRequest) (*conversationpb.GetConversationsResponse, error) {
	conversations, err := srv.getConversations(rqst.Creator)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &conversationpb.GetConversationsResponse{
		Conversations: conversations,
	}, nil
}

// The list of participant inside a conversation.
func (srv *server) addConversationParticipant(participant string, conversation string) error {
	c, err := srv.getConversation(conversation)
	if err != nil {
		return err
	}

	if !Utility.Contains(c.Participants, participant) {
		c.Participants = append(c.Participants, participant)
		return srv.saveConversation(c)
	}

	return nil
}

func (srv *server) removeConversationParticipant(participant string, conversation string) error {
	c, err := srv.getConversation(conversation)
	if err != nil {
		return err
	}

	if !Utility.Contains(c.Participants, participant) {
		return nil
	}

	paticipants := make([]string, 0)
	for i := 0; i < len(c.Participants); i++ {
		if c.Participants[i] != participant {
			paticipants = append(paticipants, c.Participants[i])
		}
	}

	c.Participants = paticipants
	return srv.saveConversation(c)
}

// The list of conversation of a participant.
func (srv *server) addParticipantConversation(paticipant string, conversation string) error {

	// Index owned conversation to be retreivable by it creator.
	_conversations_, err := srv.store.GetItem(paticipant + "_conversations")
	_conversations := make([]string, 0)
	if err == nil {
		err = json.Unmarshal(_conversations_, &_conversations)
		if err != nil {
			return err
		}
	}

	if Utility.Contains(_conversations, conversation) {
		return nil
	}

	// Now I will append the newly created conversation into conversation owned by
	// the client id.
	_conversations = append(_conversations, conversation)

	// Now I will save it back in the bd.
	jsonStr, err := json.Marshal(_conversations)
	if err != nil {
		return err
	}

	return srv.store.SetItem(paticipant+"_conversations", jsonStr)

}

// Remove conversation from a given participant
func (srv *server) removeParticipantConversation(paticipant string, conversation string) error {
	// Index owned conversation to be retreivable by it creator.
	jsonStr, err := srv.store.GetItem(paticipant + "_conversations")
	_conversations := make([]string, 0)
	if err == nil {
		err = json.Unmarshal(jsonStr, &_conversations)
		if err != nil {
			return err
		}
	}

	_conversations_ := make([]string, 0)
	for i := 0; i < len(_conversations); i++ {
		if _conversations[i] != conversation {
			_conversations_ = append(_conversations_, _conversations[i])
		}
	}

	jsonStr_, err := json.Marshal(_conversations_)

	if err != nil {
		return err
	}

	// save it back...
	return srv.store.SetItem(paticipant+"_conversations", jsonStr_)

}

// Kickout a user for any good reason...
func (srv *server) KickoutFromConversation(ctx context.Context, rqst *conversationpb.KickoutFromConversationRequest) (*conversationpb.KickoutFromConversationResponse, error) {

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// Get conversation if not exist I will return here.
	_, err = srv.getConversation(rqst.ConversationUuid)
	if err != nil {

		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Validate the clientId is the owner of the conversation.
	isOwner, _, err := srv.validateAccess(clientId, rbacpb.SubjectType_ACCOUNT, "owner", rqst.ConversationUuid)

	if err != nil {

		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if !isOwner {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("only the owner of the conversation can kickout a participant")))
	}

	// Here I will simply remove the converstion from the paticipant.
	err = srv.removeConversationParticipant(rqst.Account, rqst.ConversationUuid)
	if err != nil {

		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	err = srv.removeParticipantConversation(rqst.Account, rqst.ConversationUuid)
	if err != nil {

		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &conversationpb.KickoutFromConversationResponse{}, err

}

// Delete the conversation
func (srv *server) deleteConversation(clientId string, conversation *conversationpb.Conversation) error {

	err := srv.removeConversationParticipant(clientId, conversation.Uuid)
	if err != nil {
		return err
	}

	err = srv.removeParticipantConversation(clientId, conversation.Uuid)
	if err != nil {
		return err
	}

	// kickout all participants...
	for i := 0; i < len(conversation.Participants); i++ {
		srv.publish(`kickout_conversation_`+conversation.Uuid+`_evt`, []byte(conversation.Participants[i]))
	}

	// Close leveldb connection
	srv.closeConversationConnection(conversation.Uuid)

	// I will remove the conversation datastore...
	if Utility.Exists(srv.Root + "/conversations/" + conversation.Uuid) {
		err = os.RemoveAll(srv.Root + "/conversations/" + conversation.Uuid)
		if err != nil {
			return err
		}
	}

	// Now I will remove indexation.

	// Remove the connection from the search engine.
	err = srv.search_engine.DeleteDocument(srv.Root+"/conversations/search_data", conversation.Uuid)
	if err != nil {
		return err
	}

	// Remove the pending invitation.
	if conversation.Invitations != nil {
		for i := 0; i < len(conversation.Invitations.Invitations); i++ {
			srv.removeInvitation(conversation.Invitations.Invitations[i])
		}
	}

	// Remove conversation from participant conversations.
	for i := 0; i < len(conversation.Participants); i++ {
		srv.removeParticipantConversation(conversation.Participants[i], conversation.Uuid)
	}

	// Delete conversation from the store.
	err = srv.store.RemoveItem(conversation.Uuid)
	if err != nil {
		return err
	}

	// publish delete conversation event.
	srv.publish(`delete_conversation_`+conversation.Uuid+`_evt`, []byte(conversation.Uuid))

	// I will remove the conversation from the db.
	err = srv.deleteResourcePermissions(conversation.Uuid)
	// TODO find a way to remove it...
	if err != nil {
		return err
	}

	return nil
}

// Delete the conversation
func (srv *server) DeleteConversation(ctx context.Context, rqst *conversationpb.DeleteConversationRequest) (*conversationpb.DeleteConversationResponse, error) {

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// Validate the clientId is the owner of the conversation.
	_, _, err = srv.validateAccess(clientId, rbacpb.SubjectType_ACCOUNT, "owner", rqst.ConversationUuid)
	if err != nil {
		// Here I will simply remove the converstion from the paticipant.
		err := srv.removeConversationParticipant(clientId, rqst.ConversationUuid)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		err = srv.removeParticipantConversation(clientId, rqst.ConversationUuid)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		return nil, err
	}

	conversation, err := srv.getConversation(rqst.ConversationUuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Get conversation if not exist I will return here.
	err = srv.deleteConversation(clientId, conversation)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &conversationpb.DeleteConversationResponse{}, nil
}

// Retreive a conversation by keywords or name...
func (srv *server) FindConversations(ctx context.Context, rqst *conversationpb.FindConversationsRequest) (*conversationpb.FindConversationsResponse, error) {
	paths := []string{srv.Root + "/conversations/search_data"}

	results, err := srv.search_engine.SearchDocuments(paths, rqst.Language, []string{"name", "keywords"}, rqst.Query, rqst.Offset, rqst.PageSize, rqst.SnippetSize)
	if err != nil {
		return nil, err
	}

	conversations := make([]*conversationpb.Conversation, 0)
	for i := 0; i < len(results.Results); i++ {
		conversation := new(conversationpb.Conversation)
		err := protojson.Unmarshal([]byte(results.Results[i].Data), conversation)
		if err == nil {
			conversations = append(conversations, conversation)
		}

	}

	return &conversationpb.FindConversationsResponse{
		Conversations: conversations,
	}, nil
}

func (srv *server) Connect(rqst *conversationpb.ConnectRequest, stream conversationpb.ConversationService_ConnectServer) error {
	var clientId string
	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err != nil {
				return status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			if len(claims.UserDomain) == 0 {
				return status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no user domain was found in the token")))
			}
			clientId = claims.Id + "@" + claims.UserDomain
		} else {
			return errors.New("conversion Connect no token was given")
		}
	}

	action := make(map[string]interface{})
	action["action"] = "connect"
	action["stream"] = stream
	action["uuid"] = rqst.Uuid
	action["clientId"] = clientId
	action["quit"] = make(chan bool)

	srv.actions <- action

	// wait util unsbscribe or connection is close.
	<-action["quit"].(chan bool)
	return nil
}

// Close connection with the conversation srv.
func (srv *server) Disconnect(ctx context.Context, rqst *conversationpb.DisconnectRequest) (*conversationpb.DisconnectResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	quit := make(map[string]interface{})
	quit["action"] = "disconnect"
	quit["uuid"] = rqst.Uuid
	quit["clientId"] = clientId
	srv.actions <- quit

	return &conversationpb.DisconnectResponse{
		Result: true,
	}, nil
}

// Join a conversation.
func (srv *server) JoinConversation(rqst *conversationpb.JoinConversationRequest, stream conversationpb.ConversationService_JoinConversationServer) error {
	var clientId string
	var err error
	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {

			claims, err := security.ValidateToken(token)
			if err != nil {
				return status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			if len(claims.UserDomain) == 0 {
				return status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no user domain was found in the token")))
			}

			clientId = claims.Id + "@" + claims.UserDomain
		} else {
			return errors.New("JoinConversation no token was given")
		}
	}

	join := make(map[string]interface{})
	join["action"] = "join"
	join["name"] = rqst.ConversationUuid // Must be the converastion uuid...
	join["uuid"] = rqst.ConnectionUuid   // Must be the connection uuid...
	join["clientId"] = clientId

	srv.actions <- join

	// so here I will get existing convesation messages and return it in the stream.
	conn, err := srv.getConversationConnection(rqst.ConversationUuid)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.addConversationParticipant(clientId, rqst.ConversationUuid)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	conversation, err := srv.getConversation(rqst.ConversationUuid)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Retreive all message from the conversation...
	data, err := conn.GetItem(rqst.ConversationUuid + "/*")
	if err == nil {
		if data != nil {
			results := make([]interface{}, 0)
			err := json.Unmarshal(data, &results)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				stream.Send(&conversationpb.JoinConversationResponse{
					Msg:          nil,
					Conversation: conversation,
				})

				return errors.New("EOF")
			}

			for i := 0; i < len(results); i++ {

				msg, err := srv.getMessage(results[i].(map[string]interface{})["conversation"].(string), results[i].(map[string]interface{})["uuid"].(string))

				if err == nil {
					if i == 0 {
						stream.Send(&conversationpb.JoinConversationResponse{
							Msg:          msg,
							Conversation: conversation,
						})
					} else {
						stream.Send(&conversationpb.JoinConversationResponse{
							Msg:          msg,
							Conversation: nil,
						})
					}
				} else {
					return err
				}

			}
		}
	}

	return err
}

// Leave a given conversation.
func (srv *server) LeaveConversation(ctx context.Context, rqst *conversationpb.LeaveConversationRequest) (*conversationpb.LeaveConversationResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	leave := make(map[string]interface{})
	leave["action"] = "leave"
	leave["name"] = rqst.ConversationUuid
	leave["uuid"] = rqst.ConnectionUuid
	leave["clientId"] = clientId

	srv.actions <- leave

	err = srv.removeConversationParticipant(clientId, rqst.ConversationUuid)
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
	conversation, err := srv.getConversation(rqst.ConversationUuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &conversationpb.LeaveConversationResponse{
		Conversation: conversation,
	}, nil
}

// Conversation owner can invite a contact into Conversation.
func (srv *server) SendInvitation(ctx context.Context, rqst *conversationpb.SendInvitationRequest) (*conversationpb.SendInvitationResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	if clientId != rqst.Invitation.From {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Invitation must be sent by the authenticated user. You are not authenticated as "+rqst.Invitation.From)))
	}

	domain, _ := config.GetDomain()
	// Validate the clientId is the owner of the conversation.
	hasAccess, _, err := srv.validateAccess(clientId+"@"+domain, rbacpb.SubjectType_ACCOUNT, "owner", rqst.Invitation.Conversation)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("SendInvitation no token was given")))
	}

	if !hasAccess {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("you must be the owner of the conversation to invite other user")))
	}

	// Append it to the list of conversation invitations.
	conversation, err := srv.getConversation(rqst.Invitation.Conversation)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Test if the the invitation was necessary.
	if Utility.Contains(conversation.Participants, rqst.Invitation.To) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New(rqst.Invitation.To+" is already participanting to the conversation named "+rqst.Invitation.Name)))
	}

	if conversation.Invitations != nil {
		for i := 0; i < len(conversation.Invitations.Invitations); i++ {
			if conversation.Invitations.Invitations[i].From == rqst.Invitation.From && conversation.Invitations.Invitations[i].To == rqst.Invitation.To {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New(rqst.Invitation.To+" is already invited to join the conversation named "+rqst.Invitation.Name)))
			}
		}
	} else {
		conversation.Invitations = new(conversationpb.Invitations)
		conversation.Invitations.Invitations = make([]*conversationpb.Invitation, 0)
	}

	// set time from now...
	rqst.Invitation.InvitationDate = time.Now().Unix()
	// Index sent invitations
	sent_invitations_, err := srv.store.GetItem(clientId + "_sent_invitations")
	sent_invitations := new(conversationpb.Invitations)
	if err != nil {
		sent_invitations.Invitations = make([]*conversationpb.Invitation, 0)
	} else {
		err = protojson.Unmarshal(sent_invitations_, sent_invitations)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	rqst.Invitation.Mac = srv.Mac

	// I will save the invitation into the clientId invitation.
	sent_invitations.Invitations = append(sent_invitations.Invitations, rqst.Invitation)

	// Now I will save it back in the bd.
	jsonStr, err := protojson.Marshal(sent_invitations)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.store.SetItem(clientId+"_sent_invitations", []byte(jsonStr))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Index received invitations.
	received_invitations_, err := srv.store.GetItem(rqst.Invitation.To + "_received_invitations")
	received_invitations := new(conversationpb.Invitations)
	if err != nil {
		received_invitations.Invitations = make([]*conversationpb.Invitation, 0)
	} else {
		err = protojson.Unmarshal(received_invitations_, received_invitations)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// I will save the invitation into the clientId invitation.
	received_invitations.Invitations = append(received_invitations.Invitations, rqst.Invitation)

	// Now I will save it back in the bd.
	jsonStr, err = protojson.Marshal(received_invitations)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.store.SetItem(rqst.Invitation.To+"_received_invitations", []byte(jsonStr))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Here I will append the invitations to conversation and save it.
	conversation.Invitations.Invitations = append(conversation.Invitations.Invitations, rqst.Invitation)
	err = srv.saveConversation(conversation)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &conversationpb.SendInvitationResponse{}, nil
}

/**
 * Return a conversation with it given uuid
 */
func (srv *server) getConversation(uuid string) (*conversationpb.Conversation, error) {
	data, err := srv.store.GetItem(uuid)
	conversation := new(conversationpb.Conversation)
	if err != nil {
		return nil, err
	}

	err = protojson.Unmarshal(data, conversation)
	if err != nil {
		return nil, err
	}

	return conversation, nil
}

// Return a conversation with a given id.
func (srv *server) GetConversation(ctx context.Context, rqst *conversationpb.GetConversationRequest) (*conversationpb.GetConversationResponse, error) {
	convesation, err := srv.getConversation(rqst.Id)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &conversationpb.GetConversationResponse{
		Conversation: convesation,
	}, nil
}

/**
 * Save a conversations.
 */
func (srv *server) saveConversation(conversation *conversationpb.Conversation) error {

	jsonStr, err := protojson.Marshal(conversation)
	if err != nil {
		return err
	}

	// set the new one.
	err = srv.store.SetItem(conversation.Uuid, []byte(jsonStr))
	if err != nil {
		return err
	}

	// Now I will set the search information for conversations...
	err = srv.search_engine.IndexJsonObject(srv.Root+"/conversations/search_data", string(jsonStr), conversation.Language, "uuid", []string{"name", "keywords"}, string(jsonStr))
	if err != nil {
		return err
	}

	fmt.Println("conversation index ", conversation.Name, " was saved!")

	return nil
}

// Remove invitation.
func (srv *server) removeInvitation(invitation *conversationpb.Invitation) error {

	// Remove from sent invitations...
	sent_invitations_, err := srv.store.GetItem(invitation.From + "_sent_invitations")
	sent_invitations := new(conversationpb.Invitations)
	if err != nil {
		return err
	}

	err = protojson.Unmarshal(sent_invitations_, sent_invitations)
	if err != nil {
		return err
	}

	sent_invitations__ := make([]*conversationpb.Invitation, 0)
	for i := 0; i < len(sent_invitations.Invitations); i++ {
		if sent_invitations.Invitations[i].To != invitation.To {
			sent_invitations__ = append(sent_invitations__, sent_invitations.Invitations[i])
		}
	}

	// I will save the invitation into the clientId invitation.
	sent_invitations.Invitations = sent_invitations__

	// Now I will save it back in the bd.
	jsonStr, err := protojson.Marshal(sent_invitations)
	if err != nil {
		return err
	}

	err = srv.store.SetItem(invitation.From+"_sent_invitations", jsonStr)
	if err != nil {
		return err
	}

	// Remove it from received invitations...
	received_invitations_, err := srv.store.GetItem(invitation.To + "_received_invitations")
	received_invitations := new(conversationpb.Invitations)
	if err != nil {
		return err
	}

	err = protojson.Unmarshal(received_invitations_, received_invitations)
	if err != nil {
		return err
	}

	received_invitations__ := make([]*conversationpb.Invitation, 0)
	for i := 0; i < len(received_invitations.Invitations); i++ {
		if received_invitations.Invitations[i].To != invitation.To {
			received_invitations__ = append(received_invitations__, received_invitations.Invitations[i])
		}
	}

	// I will save the invitation into the clientId invitation.
	received_invitations.Invitations = received_invitations__

	// Now I will save it back in the bd.
	jsonStr, err = protojson.Marshal(received_invitations)
	if err != nil {
		return err
	}

	err = srv.store.SetItem(invitation.To+"_received_invitations", []byte(jsonStr))
	if err != nil {
		return err
	}

	// Now I will remove invitation from the conversation itsrv.
	conversation, err := srv.getConversation(invitation.Conversation)
	if err != nil {
		return err
	}

	invitations__ := make([]*conversationpb.Invitation, 0)
	for i := 0; i < len(conversation.Invitations.Invitations); i++ {
		if conversation.Invitations.Invitations[i].To != invitation.To && conversation.Invitations.Invitations[i].From != invitation.From {
			invitations__ = append(invitations__, conversation.Invitations.Invitations[i])
		}
	}

	conversation.Invitations.Invitations = invitations__

	return srv.saveConversation(conversation)

}

// Accept invitation response.
func (srv *server) AcceptInvitation(ctx context.Context, rqst *conversationpb.AcceptInvitationRequest) (*conversationpb.AcceptInvitationResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// Validate the user id.
	if clientId != rqst.Invitation.To {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Wrong account id your not authenticated as "+rqst.Invitation.To)))
	}

	err = srv.removeInvitation(rqst.Invitation)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.addParticipantConversation(rqst.Invitation.To, rqst.Invitation.Conversation)

	return &conversationpb.AcceptInvitationResponse{}, nil
}

// Decline invitation response.
func (srv *server) DeclineInvitation(ctx context.Context, rqst *conversationpb.DeclineInvitationRequest) (*conversationpb.DeclineInvitationResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// Validate the user id.
	if clientId != rqst.Invitation.To {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Wrong account id your not authenticated as "+rqst.Invitation.To)))
	}

	err = srv.removeInvitation(rqst.Invitation)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &conversationpb.DeclineInvitationResponse{}, nil
}

// Revoke invitation.
func (srv *server) RevokeInvitation(ctx context.Context, rqst *conversationpb.RevokeInvitationRequest) (*conversationpb.RevokeInvitationResponse, error) {

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// Validate the user id.
	if clientId != rqst.Invitation.From {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("wrong account id your not authenticated as "+rqst.Invitation.From)))
	}

	err = srv.removeInvitation(rqst.Invitation)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &conversationpb.RevokeInvitationResponse{}, nil
}

// Get the list of received invitations request.
func (srv *server) GetReceivedInvitations(ctx context.Context, rqst *conversationpb.GetReceivedInvitationsRequest) (*conversationpb.GetReceivedInvitationsResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// Validate the user id.
	if clientId != rqst.Account {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Wrong account id your not authenticated as "+rqst.Account)))
	}

	received_invitations_, err := srv.store.GetItem(clientId + "_received_invitations")
	received_invitations := new(conversationpb.Invitations)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = protojson.Unmarshal(received_invitations_, received_invitations)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Return the retreived invitations.
	return &conversationpb.GetReceivedInvitationsResponse{Invitations: received_invitations}, nil
}

// Get the list of sent invitations request.
func (srv *server) GetSentInvitations(ctx context.Context, rqst *conversationpb.GetSentInvitationsRequest) (*conversationpb.GetSentInvitationsResponse, error) {

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// Validate the user id.
	if clientId != rqst.Account {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Wrong account id your not authenticated as "+rqst.Account)))
	}

	// Index sent invitations
	sent_invitations_, err := srv.store.GetItem(clientId + "_sent_invitations")
	sent_invitations := new(conversationpb.Invitations)
	if err != nil {
		sent_invitations.Invitations = make([]*conversationpb.Invitation, 0)
	} else {
		err = protojson.Unmarshal(sent_invitations_, sent_invitations)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// Return the retreived invitations.
	return &conversationpb.GetSentInvitationsResponse{Invitations: sent_invitations}, nil
}

/**
 * Create/Update message and send it back on the conversation channel.
 */
func (srv *server) sendMessage(msg *conversationpb.Message) error {

	// Save the message in the database...
	conn, err := srv.getConversationConnection(msg.Conversation)
	if err != nil {
		return err
	}

	// Here I will save the message.
	jsonStr_, err := protojson.Marshal(msg)
	if err != nil {
		return err
	}

	// The key will be composed of the conversation id and message uuid...
	err = conn.SetItem(msg.Conversation+"/"+msg.Uuid, []byte(jsonStr_))
	if err != nil {
		return err
	}

	// Now I will index the message in the search engine...
	Utility.CreateDirIfNotExist(srv.Root + "/conversations/" + msg.Conversation + "/search_data")
	srv.search_engine.IndexJsonObject(srv.Root+"/conversations/"+msg.Conversation+"/search_data", string(jsonStr_), msg.Language, "uuid", []string{"text"}, string(jsonStr_))

	// set the conversation time...
	conversation, err := srv.getConversation(msg.Conversation)
	if err != nil {
		return err
	}

	conversation.LastMessageTime = time.Now().Unix()

	err = srv.saveConversation(conversation)
	if err != nil {
		return err
	}

	// Send the message on the network...
	send_message := make(map[string]interface{})
	send_message["action"] = "send_message"
	send_message["name"] = msg.Conversation
	send_message["message"] = msg

	// publish the message.
	srv.actions <- send_message
	return nil
}

// Send a message
func (srv *server) SendMessage(ctx context.Context, rqst *conversationpb.SendMessageRequest) (*conversationpb.SendMessageResponse, error) {

	// Save the message in the database...
	err := srv.sendMessage(rqst.Msg)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &conversationpb.SendMessageResponse{}, nil
}

// Delete message.
func (srv *server) DeleteMessage(ctx context.Context, rqst *conversationpb.DeleteMessageRequest) (*conversationpb.DeleteMessageResponse, error) {

	err := srv.deleteMessges(rqst.Conversation, rqst.Uuid)
	if err != nil {
		return nil, err
	}

	return &conversationpb.DeleteMessageResponse{}, nil
}

// Retreive a conversation by keywords or name...
func (srv *server) FindMessages(rqst *conversationpb.FindMessagesRequest, stream conversationpb.ConversationService_FindMessagesServer) error {

	return nil
}

/**
 * Get message.
 */
func (srv *server) getMessage(conversation string, uuid string) (*conversationpb.Message, error) {
	conn, err := srv.getConversationConnection(conversation)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	data, err := conn.GetItem(conversation + "/" + uuid)
	msg := new(conversationpb.Message)
	if err != nil {
		return nil, err
	}

	err = protojson.Unmarshal(data, msg)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func (srv *server) deleteMessges(conversation string, uuid string) error {
	conn, err := srv.getConversationConnection(conversation)
	if err != nil {
		return err
	}

	return conn.RemoveItem(conversation + "/" + uuid + "*")
}

// append a like message
func (srv *server) LikeMessage(ctx context.Context, rqst *conversationpb.LikeMessageRqst) (*conversationpb.LikeMessageResponse, error) {

	// Get the message by it id.
	msg, err := srv.getMessage(rqst.Conversation, rqst.Message)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	/** Authors cannot like it own message...*/
	if msg.Author == rqst.Account {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("to be humble is not to think less of onesvr, but to think of onesvr less")))
	}

	/** Append only if the msg is not likes. */

	if !Utility.Contains(msg.Likes, rqst.Account) {
		msg.Dislikes = Utility.RemoveString(msg.Dislikes, rqst.Account)
		msg.Likes = append(msg.Likes, rqst.Account)
	} else {
		msg.Likes = Utility.RemoveString(msg.Likes, rqst.Account)
	}

	/** Send message */
	err = srv.sendMessage(msg)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &conversationpb.LikeMessageResponse{}, nil
}

// dislike message
func (srv *server) DislikeMessage(ctx context.Context, rqst *conversationpb.DislikeMessageRqst) (*conversationpb.DislikeMessageResponse, error) {
	// Get the message by it id.
	msg, err := srv.getMessage(rqst.Conversation, rqst.Message)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	/** Authors cannot like it own message...*/
	if msg.Author == rqst.Account {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("low svr-esteem is like driving through life with your hand-brake on")))
	}

	/** Append only if the msg is not likes. */
	if !Utility.Contains(msg.Dislikes, rqst.Account) {
		msg.Likes = Utility.RemoveString(msg.Likes, rqst.Account)
		msg.Dislikes = append(msg.Dislikes, rqst.Account)
	} else {
		msg.Dislikes = Utility.RemoveString(msg.Dislikes, rqst.Account)
	}

	/** Send message */
	err = srv.sendMessage(msg)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &conversationpb.DislikeMessageResponse{}, nil
}

// set message as read
func (srv *server) SetMessageRead(ctx context.Context, rqst *conversationpb.SetMessageReadRqst) (*conversationpb.SetMessageReadResponse, error) {
	return nil, nil
}

// That function process channel operation and run in it own go routine.
func (srv *server) run() {

	log.Println("start conversation service")
	channels := make(map[string][]string)
	clientIds := make(map[string]string)
	streams := make(map[string]conversationpb.ConversationService_ConnectServer)
	quits := make(map[string]chan bool)

	// Here will create the action channel.
	srv.actions = make(chan map[string]interface{})

	for {
		select {
		case <-srv.exit:
			return
		case a := <-srv.actions:

			action := a["action"].(string)
			if action == "connect" {
				streams[a["uuid"].(string)] = a["stream"].(conversationpb.ConversationService_ConnectServer)
				clientIds[a["uuid"].(string)] = a["clientId"].(string)
				quits[a["uuid"].(string)] = a["quit"].(chan bool)
			} else if action == "join" {
				if channels[a["name"].(string)] == nil {
					channels[a["name"].(string)] = make([]string, 0)
				}
				if !Utility.Contains(channels[a["name"].(string)], a["uuid"].(string)) {
					channels[a["name"].(string)] = append(channels[a["name"].(string)], a["uuid"].(string))
				}
			} else if action == "send_message" {
				if channels[a["name"].(string)] != nil {
					toDelete := make([]string, 0)
					for i := 0; i < len(channels[a["name"].(string)]); i++ {
						uuid := channels[a["name"].(string)][i]
						stream := streams[uuid]
						msg := a["message"].(*conversationpb.Message)
						if stream != nil {
							// Here I will send data to stream.
							err := stream.Send(&conversationpb.ConnectResponse{
								Msg: msg,
							})

							// In case of error I will remove the subscriber
							// from the list.
							if err != nil {
								// append to channle list to be close.
								toDelete = append(toDelete, uuid)
							}
						} else {
							log.Println("connection stream with ", uuid, "is nil!")
						}
					}

					// remove closed channel
					for i := 0; i < len(toDelete); i++ {
						uuid := toDelete[i]
						clientId := clientIds[uuid]
						// remove uuid from all channels.
						for name, channel := range channels {
							uuids := make([]string, 0)
							for i := 0; i < len(channel); i++ {
								if uuid != channel[i] {
									uuids = append(uuids, channel[i])
								}
							}
							channels[name] = uuids
							srv.removeConversationParticipant(clientId, uuid)
						}
						// return from OnEvent
						quits[uuid] <- true
						// remove the channel from the map.
						delete(quits, uuid)
					}
				}
			} else if action == "leave" {
				uuids := make([]string, 0)
				for i := 0; i < len(channels[a["name"].(string)]); i++ {
					if a["uuid"].(string) != channels[a["name"].(string)][i] {
						uuids = append(uuids, channels[a["name"].(string)][i])
					}
				}
				channels[a["name"].(string)] = uuids
			} else if action == "disconnect" {
				// remove uuid from all channels.
				for name, channel := range channels {
					uuids := make([]string, 0)
					for i := 0; i < len(channel); i++ {
						if a["uuid"].(string) != channel[i] {
							uuids = append(uuids, channel[i])
						}
					}
					channels[name] = uuids
				}
				// return from connect
				quits[a["uuid"].(string)] <- true
				// remove the channel from the map.
				delete(quits, a["uuid"].(string))
			}
		}
	}
}

func (srv *server) deleteAccountListener(evt *eventpb.Event) {
	accountId := string(evt.Data)
	fmt.Println("Remove conversation for account ", accountId)
	conversations, err := srv.getConversations(accountId)
	if err == nil {
		for i := 0; i < len(conversations.GetConversations()); i++ {
			conversation := conversations.GetConversations()[i]
			srv.deleteConversation(accountId, conversation)
		}
	}
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// set the logger.
	//grpclog.SetLogger(log.New(os.Stdout, "Conversation_service: ", log.LstdFlags))

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Initialyse service with default values.
	s_impl := new(server)
	s_impl.Name = string(conversationpb.File_conversation_proto.Services().Get(0).FullName())
	s_impl.Proto = conversationpb.File_conversation_proto.Path()
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "localhost"
	s_impl.Description = "A way to communicate with other member of an organization"
	s_impl.Keywords = []string{"Conversation", "Chat", "Messenger"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = []string{"rbac.RbacService"}
	s_impl.Permissions = make([]interface{}, 2)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.PortSFU = 5551
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins

	// Register the client function, so it can be use for dynamic routing, (ex: ["GetFile", "round-robin"])
	Utility.RegisterFunction("NewConversationService_Client", conversation_client.NewConversationService_Client)

	// Set the root path if is pass as argument.
	if len(s_impl.Root) == 0 {
		s_impl.Root = os.TempDir()
	}

	// Give base info to retreive it configuration.
	if len(os.Args) == 2 {
		s_impl.Id = os.Args[1] // The second argument must be the port number
	} else if len(os.Args) == 3 {
		s_impl.Id = os.Args[1]         // The second argument must be the port number
		s_impl.ConfigPath = os.Args[2] // The second argument must be the port number
	}

	// Set permission
	s_impl.Permissions[0] = map[string]interface{}{"action": "/conversation.ConversationService/DeleteConversation", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}
	s_impl.Permissions[1] = map[string]interface{}{"action": "/conversation.ConversationService/KickoutFromConversation", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}

	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
	}

	// The search engine use to search into message, file and conversation.
	s_impl.search_engine = new(search_engine.BleveSearchEngine)

	// The map of db connections.
	s_impl.conversations = new(sync.Map)

	// Open the connetion with the store.
	Utility.CreateDirIfNotExist(s_impl.Root + "/conversations")
	s_impl.store.Open(`{"path":"` + s_impl.Root + "/conversations" + `", "name":"index"}`)

	// Register the Conversation services
	conversationpb.RegisterConversationServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start listen for event...
	go func() {
		// subscribe to account delete event events
		s_impl.subscribe("delete_account_evt", s_impl.deleteAccountListener)
	}()

	// Here I will make a signal hook to interrupt to exit cleanly.
	go s_impl.run()

	// Start the service.
	s_impl.StartService()

}
