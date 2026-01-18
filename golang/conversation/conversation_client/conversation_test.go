package conversation_client

import (
	"log"
	"testing"
	"time"

	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/conversation/conversationpb"
	"github.com/globulario/services/golang/testutil"
	Utility "github.com/globulario/utility"
)

// testContext holds client and token for conversation tests
type testContext struct {
	client *Conversation_Client
	token  string
	uuid   string
}

// newTestContext creates clients and authenticates for testing, skipping if external services are not available.
func newTestContext(t *testing.T) *testContext {
	t.Helper()
	testutil.SkipIfNoExternalServices(t)

	addr := testutil.GetAddress()
	saUser, saPwd := testutil.GetSACredentials()

	client, err := NewConversationService_Client(addr, "conversation.ConversationService")
	if err != nil {
		t.Fatalf("NewConversationService_Client: %v", err)
	}

	authClient, err := authentication_client.NewAuthenticationService_Client(addr, "authentication.AuthenticationService")
	if err != nil {
		t.Fatalf("NewAuthenticationService_Client: %v", err)
	}

	token, err := authClient.Authenticate(saUser, saPwd)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	return &testContext{client: client, token: token}
}

// Test various function here.
func TestCreateConverstion(t *testing.T) {
	ctx := newTestContext(t)

	conversation, err := ctx.client.CreateConversation(ctx.token, "mystic.courtois", []string{"test", "converstion", "nothing"})
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("conversation was created with success!", conversation)
	ctx.uuid = conversation.GetUuid()
}

func TestGetCreatedConversation(t *testing.T) {
	ctx := newTestContext(t)

	saUser, _ := testutil.GetSACredentials()
	conversations, err := ctx.client.GetOwnedConversations(ctx.token, saUser)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("conversation retreived!", conversations)
}

func TestFindConversation(t *testing.T) {
	ctx := newTestContext(t)

	results, err := ctx.client.FindConversations(ctx.token, "nothing", "en", 0, 100, 500)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(results)
}

func testMessageListener(msg *conversationpb.Message) {
	log.Println("-------> message received! ", msg.Text)
}

func TestJoinConversation(t *testing.T) {
	ctx := newTestContext(t)
	// Note: This test requires a conversation to already exist
	t.Skip("Requires existing conversation UUID")

	conversations, err := ctx.client.JoinConversation(ctx.uuid, "__uuid_test_must_be_unique__", testMessageListener)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("-----> conversations ", conversations)
}

func TestSendConversationMessage(t *testing.T) {
	ctx := newTestContext(t)
	// Note: This test requires a conversation to already exist
	t.Skip("Requires existing conversation UUID")

	err := ctx.client.SendMessage(ctx.uuid, &conversationpb.Message{CreationTime: time.Now().Unix(), Uuid: Utility.RandomUUID(), Text: "First Message of all!", Conversation: ctx.uuid, InReplyTo: ""})
	if err != nil {
		log.Println(err)
		return
	}

}

func TestDeleteConversation(t *testing.T) {
	ctx := newTestContext(t)
	// Note: This test requires a conversation to already exist
	t.Skip("Requires existing conversation UUID")

	err := ctx.client.DeleteConversation(ctx.token, ctx.uuid)
	if err != nil {
		log.Println(err)
		return
	}

}
