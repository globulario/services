package conversation_client

import (
	"context"
	"fmt"
	"io"
	"time"

	//""github.com/globulario/utility""

	"github.com/globulario/services/golang/conversation/conversationpb"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
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

	// The mac address of the server
	mac string

	// The client domain
	domain string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

	//  keep the last connection state of the client.
	state string

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

	// The event channel.
	actions chan map[string]interface{}

	// A unique uuid use for authenticate with the server.
	uuid string
}

// Create a connection to the service.
func NewConversationService_Client(address string, id string) (*Conversation_Client, error) {
	client := new(Conversation_Client)

	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}

	// The channel where data will be exchange.
	client.actions = make(chan map[string]interface{})

	// Create a random uuid.
	client.uuid = Utility.RandomUUID()

	err = client.Reconnect()
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (client *Conversation_Client) Reconnect() error {

	var err error
	nb_try_connect := 10

	for i := 0; i < nb_try_connect; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = conversationpb.NewConversationServiceClient(client.cc)
			break
		}

		// wait 500 millisecond before next try
		time.Sleep(500 * time.Millisecond)
	}

	return err

}

/**
 * Process event from the server. Only one stream is needed between the server
 * and the client. Local handler are kept in a map with a unique uuid, so many
 * handler can exist for a single event.
 */
func (client *Conversation_Client) run() error {

	// Create the channel.
	data_channel := make(chan *conversationpb.Message)

	// start listenting to events from the server...
	err := client.connect(client.uuid, data_channel)
	if err != nil {
		return err
	}

	// the map that will contain the event handler.
	handlers := make(map[string]map[string]func(*conversationpb.Message))

	for {
		select {
		case msg := <-data_channel:
			// So here I received a message, I will dispatch it to it conversation.
			handlers_ := handlers[msg.Conversation]
			for _, fct := range handlers_ {
				// Call the handler.
				fct(msg)
			}

		case action := <-client.actions:
			if action["action"].(string) == "join" {
				if handlers[action["name"].(string)] == nil {
					handlers[action["name"].(string)] = make(map[string]func(*conversationpb.Message))
				}
				// Set it handler.
				handlers[action["name"].(string)][action["uuid"].(string)] = action["fct"].(func(*conversationpb.Message))
			} else if action["action"].(string) == "leave" {
				// Now I will remove the handler...
				for _, handler := range handlers {
					if handler[action["uuid"].(string)] != nil {
						delete(handler, action["uuid"].(string))
					}
				}
			} else if action["action"].(string) == "stop" {
				return nil
			}
		}
	}
}

// The address where the client can connect.
func (client *Conversation_Client) SetAddress(address string) {
	client.address = address
}

func (client *Conversation_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Conversation_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}

	token, err := security.GetLocalToken(client.GetMac())
	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": client.domain, "mac": client.GetMac(), "address": client.GetAddress()})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	return client.ctx
}

// Return the domain
func (client *Conversation_Client) GetDomain() string {
	return client.domain
}

// Return the last know connection state
func (client *Conversation_Client) GetState() string {
	return client.state
}

// Return the address
func (client *Conversation_Client) GetAddress() string {
	return client.address
}

// Return the id of the service instance
func (client *Conversation_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Conversation_Client) GetName() string {
	return client.name
}

func (client *Conversation_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Conversation_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Conversation_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *Conversation_Client) GetPort() int {
	return client.port
}

// Set the client instance id.
func (client *Conversation_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Conversation_Client) SetName(name string) {
	client.name = name
}

func (client *Conversation_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Conversation_Client) SetDomain(domain string) {
	client.domain = domain
}

func (client *Conversation_Client) SetState(state string) {
	client.state = state
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Conversation_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Conversation_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Conversation_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Conversation_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Conversation_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Conversation_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Conversation_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Conversation_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

// //////////////// Api //////////////////////
// Stop the service.
func (client *Conversation_Client) StopService() {
	client.c.Stop(client.GetCtx(), &conversationpb.StopRequest{})
}

////////////////////////////////////////////////////////////////////////////////
// Conversation specific function here.
////////////////////////////////////////////////////////////////////////////////

// Create a new conversation with a given name and a list of keywords for retreive it latter.
func (client *Conversation_Client) CreateConversation(token string, name string, keywords []string) (*conversationpb.Conversation, error) {

	rqst := &conversationpb.CreateConversationRequest{
		Name:     name,
		Keywords: keywords,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	/** Create the conversation on the server side and get it uuid as response. */
	rsp, err := client.c.CreateConversation(ctx, rqst)

	if err != nil {
		return nil, err
	}

	return rsp.Conversation, nil
}

// Return the list of owned conversations.
func (client *Conversation_Client) GetOwnedConversations(token string, creator string) (*conversationpb.Conversations, error) {
	rqst := &conversationpb.GetConversationsRequest{
		Creator: creator,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	rsp, err := client.c.GetConversations(ctx, rqst)

	if err != nil {
		return nil, err
	}

	return rsp.GetConversations(), nil
}

// Delete a conversation
func (client *Conversation_Client) DeleteConversation(token string, conversationUuid string) error {
	rqst := new(conversationpb.DeleteConversationRequest)
	rqst.ConversationUuid = conversationUuid

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.DeleteConversation(ctx, rqst)

	if err != nil {
		return err
	}

	return nil
}

/**
 * Find a conversations.
 */
func (client *Conversation_Client) FindConversations(token string, query string, language string, offset int32, pageSize int32, snippetSize int32) ([]*conversationpb.Conversation, error) {
	rqst := new(conversationpb.FindConversationsRequest)
	rqst.Query = query
	rqst.Language = language
	rqst.Offset = offset
	rqst.PageSize = pageSize
	rqst.SnippetSize = snippetSize

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	results, err := client.c.FindConversations(ctx, rqst)

	if err != nil {
		return nil, err
	}

	return results.Conversations, nil
}

/**
 * Open a new connection with the conversation server.
 */
func (client *Conversation_Client) connect(uuid string, data_channel chan *conversationpb.Message) error {

	rqst := &conversationpb.ConnectRequest{
		Uuid: uuid,
	}

	stream, err := client.c.Connect(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	// Run in it own goroutine.
	go func() {
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				// end of stream...
				break
			}
			if err != nil {
				break
			}

			// Get the result...
			data_channel <- msg.GetMsg()
		}
	}()

	// Wait for subscriber uuid and return it to the function caller.
	return nil
}

func (client *Conversation_Client) JoinConversation(conversation_uuid string, listener_uuid string, fct func(msg *conversationpb.Message)) (*conversationpb.Conversations, error) {
	/** Connect to a given conversation */
	rqst := &conversationpb.JoinConversationRequest{
		ConversationUuid: conversation_uuid,
		ConnectionUuid:   client.uuid,
	}

	stream, err := client.c.JoinConversation(client.GetCtx(), rqst)
	if err != nil {
		fmt.Println("fail to join conversation ", conversation_uuid, err)
		return nil, err
	}

	var conversations *conversationpb.Conversations
	if stream != nil {
		// TODO get stream and init the conversations object here...
		fmt.Println("Get existing messages...")
	}

	action := make(map[string]interface{})
	action["action"] = "join"
	action["uuid"] = listener_uuid
	action["name"] = conversation_uuid
	action["fct"] = fct

	// set the action.
	client.actions <- action

	// Return the list of message already in the database...

	return conversations, nil
}

// Exit event channel.
func (client *Conversation_Client) Leave(conversation_uuid string, listener_uuid string) error {

	// Unsubscribe from the event channel.
	rqst := &conversationpb.LeaveConversationRequest{
		ConversationUuid: conversation_uuid,
		ConnectionUuid:   client.uuid,
	}

	_, err := client.c.LeaveConversation(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	action := make(map[string]interface{})
	action["action"] = "leave"
	action["uuid"] = listener_uuid
	action["name"] = conversation_uuid

	// set the action.
	client.actions <- action
	return nil
}

// Publish and event over the network
func (client *Conversation_Client) SendMessage(conversation_uuid string, msg *conversationpb.Message) error {
	rqst := &conversationpb.SendMessageRequest{
		Msg: msg,
	}

	_, err := client.c.SendMessage(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	return nil
}
