package main

import (
	"context"
	//	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/davecourtois/Utility"
	"github.com/globulario/Globular/Interceptors"
	"github.com/globulario/services/golang/conversation/conversation_client"
	"github.com/globulario/services/golang/conversation/conversationpb"
	globular "github.com/globulario/services/golang/globular_service"
	"google.golang.org/grpc"

	"google.golang.org/grpc/codes"

	//"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"errors"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/jsonpb"

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

	// Specific configuration.
	Root string // Where to look for conversation data, file.. etc.

	TLS bool

	// self-signed X.509 public keys for distribution
	CertFile string

	// a private RSA key to sign and authenticate the public key
	KeyFile string

	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string

	Permissions []interface{} // contains the action permission for the services.

	// The grpc server.
	grpcServer *grpc.Server

	// The search engine..
	search_engine *search_engine.XapianEngine

	// Store global conversation information like conversation owner's participant...
	store *storage_store.LevelDB_store

	// keep in map active conversation db connections.
	conversations *sync.Map

	// The rbac client
	rbac_client_ *rbac_client.Rbac_Client
}

// Globular services implementation...
// The id of a particular service instance.
func (self *server) GetId() string {
	return self.Id
}
func (self *server) SetId(id string) {
	self.Id = id
}

// The name of a service, must be the gRpc Service name.
func (self *server) GetName() string {
	return self.Name
}
func (self *server) SetName(name string) {
	self.Name = name
}

// The description of the service
func (self *server) GetDescription() string {
	return self.Description
}
func (self *server) SetDescription(description string) {
	self.Description = description
}

// The list of keywords of the services.
func (self *server) GetKeywords() []string {
	return self.Keywords
}
func (self *server) SetKeywords(keywords []string) {
	self.Keywords = keywords
}

func (self *server) GetRepositories() []string {
	return self.Repositories
}
func (self *server) SetRepositories(repositories []string) {
	self.Repositories = repositories
}

func (self *server) GetDiscoveries() []string {
	return self.Discoveries
}
func (self *server) SetDiscoveries(discoveries []string) {
	self.Discoveries = discoveries
}

// Dist
func (self *server) Dist(path string) (string, error) {

	return globular.Dist(path, self)
}

func (self *server) GetPlatform() string {
	return globular.GetPlatform()
}

// The path of the executable.
func (self *server) GetPath() string {
	return self.Path
}
func (self *server) SetPath(path string) {
	self.Path = path
}

// The path of the .proto file.
func (self *server) GetProto() string {
	return self.Proto
}
func (self *server) SetProto(proto string) {
	self.Proto = proto
}

// The gRpc port.
func (self *server) GetPort() int {
	return self.Port
}
func (self *server) SetPort(port int) {
	self.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (self *server) GetProxy() int {
	return self.Proxy
}
func (self *server) SetProxy(proxy int) {
	self.Proxy = proxy
}

// Can be one of http/https/tls
func (self *server) GetProtocol() string {
	return self.Protocol
}
func (self *server) SetProtocol(protocol string) {
	self.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (self *server) GetAllowAllOrigins() bool {
	return self.AllowAllOrigins
}
func (self *server) SetAllowAllOrigins(allowAllOrigins bool) {
	self.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (self *server) GetAllowedOrigins() string {
	return self.AllowedOrigins
}

func (self *server) SetAllowedOrigins(allowedOrigins string) {
	self.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (self *server) GetDomain() string {
	return self.Domain
}
func (self *server) SetDomain(domain string) {
	self.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (self *server) GetTls() bool {
	return self.TLS
}
func (self *server) SetTls(hasTls bool) {
	self.TLS = hasTls
}

// The certificate authority file
func (self *server) GetCertAuthorityTrust() string {
	return self.CertAuthorityTrust
}
func (self *server) SetCertAuthorityTrust(ca string) {
	self.CertAuthorityTrust = ca
}

// The certificate file.
func (self *server) GetCertFile() string {
	return self.CertFile
}
func (self *server) SetCertFile(certFile string) {
	self.CertFile = certFile
}

// The key file.
func (self *server) GetKeyFile() string {
	return self.KeyFile
}
func (self *server) SetKeyFile(keyFile string) {
	self.KeyFile = keyFile
}

// The service version
func (self *server) GetVersion() string {
	return self.Version
}
func (self *server) SetVersion(version string) {
	self.Version = version
}

// The publisher id.
func (self *server) GetPublisherId() string {
	return self.PublisherId
}
func (self *server) SetPublisherId(publisherId string) {
	self.PublisherId = publisherId
}

func (self *server) GetKeepUpToDate() bool {
	return self.KeepUpToDate
}
func (self *server) SetKeepUptoDate(val bool) {
	self.KeepUpToDate = val
}

func (self *server) GetKeepAlive() bool {
	return self.KeepAlive
}
func (self *server) SetKeepAlive(val bool) {
	self.KeepAlive = val
}

func (self *server) GetPermissions() []interface{} {
	return self.Permissions
}
func (self *server) SetPermissions(permissions []interface{}) {
	self.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (self *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewConversationService_Client", conversation_client.NewConversationService_Client)

	// Get the configuration path.
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	err := globular.InitService(dir+"/config.json", self)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	self.grpcServer, err = globular.InitGrpcServer(self, Interceptors.ServerUnaryInterceptor, Interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	// Initialyse the search engine.
	self.search_engine = new(search_engine.XapianEngine)

	// Create a new local store.
	self.store = storage_store.NewLevelDB_store()

	return nil

}

// Save the configuration values.
func (self *server) Save() error {
	// Create the file...
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	return globular.SaveService(dir+"/config.json", self)
}

func (self *server) StartService() error {
	return globular.StartService(self, self.grpcServer)
}

func (self *server) StopService() error {
	return globular.StopService(self, self.grpcServer)
}

func (self *server) Stop(context.Context, *conversationpb.StopRequest) (*conversationpb.StopResponse, error) {
	return &conversationpb.StopResponse{}, self.StopService()
}

/////////////////////// Conversation specific function /////////////////////////////////

/**
 * Databases will be created in the 'conversations' directory inside the Root path
 * Each conversation will have it own leveldb database
 */
func (self *server) getConversationConnection(id string) (*storage_store.LevelDB_store, error) {

	dbPath := self.Root + "/conversations/" + id
	if !Utility.Exists(dbPath) {
		log.Println(dbPath)
	}

	connection, ok := self.conversations.Load(dbPath)
	if !ok {
		connection = storage_store.NewLevelDB_store()
		err := connection.(*storage_store.LevelDB_store).Open(`{"path":"` + dbPath + `", "name":"store_data"}`)
		if err != nil {
			return nil, err
		}
	}

	return connection.(*storage_store.LevelDB_store), nil
}

func (self *server) closeConversationConnection(id string) {

	dbPath := self.Root + "/conversations/" + id
	if !Utility.Exists(dbPath) {
		log.Println(dbPath)
	}

	connection, ok := self.conversations.Load(dbPath)
	if !ok {
		return
	}

	// Close the connection.
	connection.(*storage_store.LevelDB_store).Close()
}

/////////////////////////// Public interfaces //////////////////////////////////

// Create a new conversation with a given name. The creator will became the
// owner of that conversation and he will be able to set permissions to
// determine who can participate to the conversation.
func (self *server) CreateConversation(ctx context.Context, rqst *conversationpb.CreateConversationRequest) (*conversationpb.CreateConversationResponse, error) {
	var clientId string
	var err error

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {

			clientId, _, _, err = Interceptors.ValidateToken(token)
			if err != nil {
				log.Println("token validation fail with error: ", err)
				return nil, err
			}

		} else {
			errors.New("No token was given!")
		}
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
	}

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(conversation)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = self.store.SetItem(uuid, []byte(jsonStr))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will set the search information for conversations...
	err = self.search_engine.IndexJsonObject(self.Root+"/conversations/search_data", jsonStr, rqst.Language, "uuid", []string{"name", "keywords"}, jsonStr)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// So here I will append the value in the index.

	// Index owned conversation to be retreivable by it creator.
	owned_conversations_, err := self.store.GetItem(clientId + "_owned")
	owned_conversations := new(conversationpb.Conversations)
	if err != nil {
		owned_conversations.Conversations = make([]*conversationpb.Conversation, 0)
	} else {
		err = jsonpb.UnmarshalString(string(owned_conversations_), owned_conversations)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// Now I will append the newly created conversation into conversation owned by
	// the client id.
	owned_conversations.Conversations = append(owned_conversations.Conversations, conversation)

	// Now I will save it back in the bd.
	jsonStr, err = marshaler.MarshalToString(owned_conversations)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = self.store.SetItem(clientId+"_owned", []byte(jsonStr))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will set it in the rbac as ressource owner...
	permissions := &rbacpb.Permissions{
		Allowed: []*rbacpb.Permission{},
		Denied:  []*rbacpb.Permission{},
		Owners: &rbacpb.Permission{
			Name:          "owner", // The name is informative in that particular case.
			Applications:  []string{},
			Accounts:      []string{clientId},
			Groups:        []string{},
			Peers:         []string{},
			Organizations: []string{},
		},
	}

	err = self.rbac_client_.SetResourcePermissions(uuid, permissions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &conversationpb.CreateConversationResponse{
		Conversation: conversation,
	}, nil
}

// Return the list of conversations created by a given user.
func (self *server) GetCreatedConversations(ctx context.Context, rqst *conversationpb.GetCreatedConversationsRequest) (*conversationpb.GetCreatedConversationsResponse, error) {

	owned_conversations_, err := self.store.GetItem(rqst.Creator + "_owned")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	owned_conversations := new(conversationpb.Conversations)
	jsonpb.UnmarshalString(string(owned_conversations_), owned_conversations)

	return &conversationpb.GetCreatedConversationsResponse{
		Conversations: owned_conversations,
	}, nil
}

// Delete the conversation
func (self *server) DeleteConversation(ctx context.Context, rqst *conversationpb.DeleteConversationRequest) (*conversationpb.DeleteConversationResponse, error) {
	var clientId string
	var err error
	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {

			clientId, _, _, err = Interceptors.ValidateToken(token)
			if err != nil {
				log.Println("token validation fail with error: ", err)
				return nil, err
			}

		} else {
			errors.New("No token was given!")
		}
	}

	//TODO First of all I will quick out connected pepole...

	// Close leveldb connection
	self.closeConversationConnection(rqst.ConversationUuid)

	// I will remove the conversation datastore...
	if Utility.Exists(self.Root + "/conversations/" + rqst.ConversationUuid) {
		err = os.RemoveAll(self.Root + "/conversations/" + rqst.ConversationUuid)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// I will remove the conversation from the db.
	err = self.rbac_client_.DeleteResourcePermissions(rqst.ConversationUuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will remove indexation.

	// Index owned conversation to be retreivable by it creator.
	jsonStr, err := self.store.GetItem(clientId + "_owned")
	owned_conversations := new(conversationpb.Conversations)
	if err == nil {
		err = jsonpb.UnmarshalString(string(jsonStr), owned_conversations)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	conversations := make([]*conversationpb.Conversation, 0)
	for i := 0; i < len(owned_conversations.Conversations); i++ {
		c := owned_conversations.Conversations[i]
		if c.Uuid != rqst.ConversationUuid {
			conversations = append(conversations, c)
		}
	}

	owned_conversations.Conversations = conversations
	var marshaler jsonpb.Marshaler
	jsonStr_, err := marshaler.MarshalToString(owned_conversations)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// save it back...
	err = self.store.SetItem(clientId+"_owned", []byte(jsonStr_))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Remove the connection from the search engine.
	err = self.search_engine.DeleteDocument(self.Root+"/conversations/search_data", rqst.ConversationUuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Remove it from the store.
	err = self.store.RemoveItem(rqst.ConversationUuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &conversationpb.DeleteConversationResponse{}, nil
}

// Retreive a conversation by keywords or name...
func (self *server) FindConversation(ctx context.Context, rqst *conversationpb.FindConversationRequest) (*conversationpb.FindConversationResponse, error) {
	paths := []string{self.Root + "/conversations/search_data"}

	results, err := self.search_engine.SearchDocuments(paths, rqst.Language, []string{"name", "keywords"}, rqst.Query, rqst.Offset, rqst.PageSize, rqst.SnippetSize)
	if err != nil {
		return nil, err
	}

	conversations := make([]*conversationpb.Conversation, 0)
	for i := 0; i < len(results.Results); i++ {

		conversation := new(conversationpb.Conversation)
		err := jsonpb.UnmarshalString(results.Results[i].Data, conversation)
		if err == nil {
			conversations = append(conversations, conversation)
		} else {
			log.Println(err)
		}

	}

	return &conversationpb.FindConversationResponse{
		Conversations: conversations,
	}, nil
}

// Join a conversation.
func (self *server) JoinConversation(ctx *conversationpb.JoinConversationRequest, stream conversationpb.ConversationService_JoinConversationServer) error {
	return nil
}

// Leave a given conversation.
func (self *server) LeaveConversation(ctx context.Context, rqst *conversationpb.LeaveConversationRequest) (*conversationpb.LeaveConversationResponse, error) {
	return nil, nil
}

// Stop new message to be insert...
func (self *server) SuspendConversation(ctx context.Context, rqst *conversationpb.SuspendConversationRequest) (*conversationpb.SuspendConversationResponse, error) {
	return nil, nil
}

// Resume the conversation
func (self *server) ResumeConversation(ctx context.Context, rqst *conversationpb.ResumeConversationRequest) (*conversationpb.ResumeConversationResponse, error) {
	return nil, nil
}

// Send a message
func (self *server) SendMessage(ctx context.Context, rqst *conversationpb.SendMessageRequest) (*conversationpb.SendMessageResponse, error) {
	return nil, nil
}

// Revoke a message from the list.
func (self *server) RevokeMessage(ctx context.Context, rqst *conversationpb.RevokeMessageRequest) (*conversationpb.RevokeMessageResponse, error) {

	return nil, nil
}

// Retreive a conversation by keywords or name...
func (self *server) FindMessage(ctx context.Context, rqst *conversationpb.FindMessageRequest) (*conversationpb.FindMessageResponse, error) {

	return nil, nil
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
	s_impl.Name = string(conversationpb.File_proto_conversation_proto.Services().Get(0).FullName())
	s_impl.Proto = conversationpb.File_proto_conversation_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario"
	s_impl.Description = "A way to communicate with other member of an organization"
	s_impl.Keywords = []string{"Conversation", "Chat", "Messenger"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Permissions = make([]interface{}, 0)

	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins

	// Set the root path if is pass as argument.
	if len(s_impl.Root) == 0 {
		s_impl.Root = os.TempDir()
	}

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id, err)
	}

	if len(os.Args) == 2 {
		s_impl.Port, _ = strconv.Atoi(os.Args[1]) // The second argument must be the port number
	}

	// The search engine use to search into message, file and conversation.
	s_impl.search_engine = new(search_engine.XapianEngine)

	// The map of db connections.
	s_impl.conversations = new(sync.Map)

	// Open the connetion with the store.
	Utility.CreateDirIfNotExist(s_impl.Root + "/conversations")
	s_impl.store.Open(`{"path":"` + s_impl.Root + "/conversations" + `", "name":"index"}`)

	// Register the Conversation services
	conversationpb.RegisterConversationServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Init the Role Based Access Control client.
	s_impl.rbac_client_, _ = rbac_client.NewRbacService_Client(s_impl.Domain, "rbac.RbacService")

	// Start the service.
	s_impl.StartService()

}
