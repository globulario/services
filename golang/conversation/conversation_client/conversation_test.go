package conversation_client

import (
	//"encoding/json"
	"log"
	"testing"

	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/conversation/conversationpb"
	"github.com/globulario/services/golang/resource/resource_client"
)

var (
	client, _ = NewConversationService_Client("hub.globular.io", "4e0408f4-9d2a-4c25-95ed-e5bdf2444eb3")
	uuid      = ""
)

// Test various function here.
func TestCreateConverstion(t *testing.T) {
	resource_client_, _ := resource_client.NewResourceService_Client("hub.globular.io", "resource.ResourceService")

	token, err := resource_client_.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println(err)
		return
	}

	conversation, err := client.CreateConversation(token, "mystic.courtois", []string{"test", "converstion", "nothing"})
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("conversation was created with success!", conversation)
	uuid = conversation.GetUuid()
}

func TestGetCreatedConversation(t *testing.T) {
	resource_client_, _ := resource_client.NewResourceService_Client("hub.globular.io", "resource.ResourceService")
	token, err := resource_client_.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println(err)
		return
	}

	conversations, err := client.GetOwnedConversations(token, "sa")
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("conversation retreived!", conversations)
}

func TestFindConversation(t *testing.T) {
	resource_client_, _ := resource_client.NewResourceService_Client("hub.globular.io", "resource.ResourceService")
	token, err := resource_client_.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println(err)
		return
	}

	results, err := client.FindConversations(token, "nothing", "en", 0, 100, 500)
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
	conversations, err := client.JoinConversation(uuid, "__uuid_test_must_be_unique__", testMessageListener)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("-----> conversations ", conversations)
}

func TestSendConversationMessage(t *testing.T) {
	err := client.SendMessage(uuid, &conversationpb.Message{CreationTime: time.Now().Unix(), Text: "First Message of all!", Conversation: uuid, InReplyTo: ""})
	if err != nil {
		log.Println(err)
		return
	}

}

func TestDeleteConversation(t *testing.T) {
	resource_client_, _ := resource_client.NewResourceService_Client("hub.globular.io", "resource.ResourceService")
	token, err := resource_client_.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println(err)
		return
	}

	err = client.DeleteConversation(token, uuid)
	if err != nil {
		log.Println(err)
		return
	}

}
