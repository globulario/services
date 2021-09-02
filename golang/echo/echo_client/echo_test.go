package echo_client

import (
	//"encoding/json"
	"log"
	"testing"

	"github.com/globulario/services/golang/resource/resource_client"
)

// Test various function here.
func TestEcho(t *testing.T) {

	// Here I Will authenticate the user before validation...
	resource_client_, err := resource_client.NewResourceService_Client("localhost", "resource.ResourceService")
	if err != nil {
		log.Println(err)
		return
	}

	token, err := resource_client_.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println(err)
		return
	}

	// Connect to the plc client.
	// "0f80ed1a-5d3a-46f1-a3f6-c091ac259665",  "b94d0011-39a0-4bdb-9a5c-7e9abc23b26b"
	client, err := NewEchoService_Client("localhost", "b94d0011-39a0-4bdb-9a5c-7e9abc23b26b")
	if err != nil {
		log.Println("17 ---> ", err)
		return
	}
	val, err := client.Echo(token, "Ceci est un test")
	if err != nil {
		log.Println("20 ---> ", err)
	} else {
		log.Println("23 ---> ", val)
	}

	client, err := NewEchoService_Client("localhost", "8eb9a0b5-9882-40b3-9397-b5e1a8457eb8")
	if err != nil {
		log.Println("28 ---> ", err)
		return
	}

	val, err := client.Echo(token, "Ceci est un test")
	if err != nil {
		log.Println("34 ---> ", err)
	} else {
		log.Println("36 ---> ", val)
	}
}
