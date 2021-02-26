package conversation_client

import (
	"strconv"

	"context"

	//"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/conversation/conversationpb"
	globular "github.com/globulario/services/golang/globular_client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// conversation Client Service
////////////////////////////////////////////////////////////////////////////////

type Conversation_Client struct {
	cc *grpc.ClientConn
	c  conversationpb.ConversationServiceClient

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
func NewConversationService_Client(address string, id string) (*Conversation_Client, error) {
	client := new(Conversation_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = conversationpb.NewConversationServiceClient(client.cc)

	return client, nil
}

func (self *Conversation_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(self)
	}
	return globular.InvokeClientRequest(self.c, ctx, method, rqst)
}

// Return the domain
func (self *Conversation_Client) GetDomain() string {
	return self.domain
}

// Return the address
func (self *Conversation_Client) GetAddress() string {
	return self.domain + ":" + strconv.Itoa(self.port)
}

// Return the id of the service instance
func (self *Conversation_Client) GetId() string {
	return self.id
}

// Return the name of the service
func (self *Conversation_Client) GetName() string {
	return self.name
}

// must be close when no more needed.
func (self *Conversation_Client) Close() {
	self.cc.Close()
}

// Set grpc_service port.
func (self *Conversation_Client) SetPort(port int) {
	self.port = port
}

// Set the client instance id.
func (self *Conversation_Client) SetId(id string) {
	self.id = id
}

// Set the client name.
func (self *Conversation_Client) SetName(name string) {
	self.name = name
}

// Set the domain.
func (self *Conversation_Client) SetDomain(domain string) {
	self.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (self *Conversation_Client) HasTLS() bool {
	return self.hasTLS
}

// Get the TLS certificate file path
func (self *Conversation_Client) GetCertFile() string {
	return self.certFile
}

// Get the TLS key file path
func (self *Conversation_Client) GetKeyFile() string {
	return self.keyFile
}

// Get the TLS key file path
func (self *Conversation_Client) GetCaFile() string {
	return self.caFile
}

// Set the client is a secure client.
func (self *Conversation_Client) SetTLS(hasTls bool) {
	self.hasTLS = hasTls
}

// Set TLS certificate file path
func (self *Conversation_Client) SetCertFile(certFile string) {
	self.certFile = certFile
}

// Set TLS key file path
func (self *Conversation_Client) SetKeyFile(keyFile string) {
	self.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (self *Conversation_Client) SetCaFile(caFile string) {
	self.caFile = caFile
}

////////////////// Api //////////////////////
// Stop the service.
func (self *Conversation_Client) StopService() {
	self.c.Stop(globular.GetClientContext(self), &conversationpb.StopRequest{})
}

////////////////////////////////////////////////////////////////////////////////
// Conversation specific function here.
////////////////////////////////////////////////////////////////////////////////

// Create a new conversation with a given name and a list of keywords for retreive it latter.
func (self *Conversation_Client) createConversation(token string, name string, keywords []string) (*conversationpb.Conversation, error) {

	rqst := &conversationpb.CreateConversationRequest{
		Name:     name,
		Keywords: keywords,
	}

	ctx := globular.GetClientContext(self)
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	/** Create the conversation on the server side and get it uuid as response. */
	rsp, err := self.c.CreateConversation(ctx, rqst)

	if err != nil {
		return nil, err
	}

	return rsp.Conversation, nil
}

// Return the list of owned conversations.
func (self *Conversation_Client) getOwnedConversations(token string, creator string) (*conversationpb.Conversations, error) {
	rqst := &conversationpb.GetCreatedConversationsRequest{
		Creator: creator,
	}

	ctx := globular.GetClientContext(self)
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	rsp, err := self.c.GetCreatedConversations(ctx, rqst)

	if err != nil {
		return nil, err
	}

	return rsp.GetConversations(), nil
}

// Delete a conversation
func (self *Conversation_Client) deleteConversation(token string, conversationUuid string) error {
	rqst := new(conversationpb.DeleteConversationRequest)
	rqst.ConversationUuid = conversationUuid

	ctx := globular.GetClientContext(self)
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := self.c.DeleteConversation(ctx, rqst)

	if err != nil {
		return err
	}

	return nil
}

/**
 * Find a conversations.
 */
func (self *Conversation_Client) findConversations(token string, query string, language string, offset int32, pageSize int32, snippetSize int32) ([]*conversationpb.Conversation, error) {
	rqst := new(conversationpb.FindConversationRequest)
	rqst.Query = query
	rqst.Language = language
	rqst.Offset = offset
	rqst.PageSize = pageSize
	rqst.SnippetSize = snippetSize

	ctx := globular.GetClientContext(self)
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	results, err := self.c.FindConversation(ctx, rqst)

	if err != nil {
		return nil, err
	}

	return results.Conversations, nil
}
