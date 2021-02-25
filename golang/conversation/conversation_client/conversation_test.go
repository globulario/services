package conversation_client

import (
	//"encoding/json"
	"log"
	"testing"

	"github.com/globulario/services/golang/resource/resource_client"
)

var (
	client, _ = NewConversationService_Client("hub.globular.io", "4e0408f4-9d2a-4c25-95ed-e5bdf2444eb3")
)

// Test various function here.
/*
func TestConverstion(t *testing.T) {
	resource_client_, _ := resource_client.NewResourceService_Client("hub.globular.io", "resource.ResourceService")

	_, err := resource_client_.Authenticate("sa", "adminadmin")

	if err != nil {
		log.Println(err)
		return
	}
}
*/

func TestCreateConverstion(t *testing.T) {
	resource_client_, _ := resource_client.NewResourceService_Client("hub.globular.io", "resource.ResourceService")

	token, err := resource_client_.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println(err)
		return
	}

	conversation, err := client.createConversation(token, "mystic.courtois", []string{"test", "converstion", "nothing"})
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("conversation was created with success!", conversation)
}

func TestGetCreatedConversation(t *testing.T) {
	resource_client_, _ := resource_client.NewResourceService_Client("hub.globular.io", "resource.ResourceService")
	token, err := resource_client_.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println(err)
		return
	}

	conversations, err := client.getOwnedConversations(token, "sa")
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("conversation retreived!", conversations)
}
