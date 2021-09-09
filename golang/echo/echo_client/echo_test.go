package echo_client

import (
	//"encoding/json"
	"log"
	"testing"
)

// Test various function here.
func TestEcho(t *testing.T) {

	// Connect to the plc client.
	// "0f80ed1a-5d3a-46f1-a3f6-c091ac259665",  "b94d0011-39a0-4bdb-9a5c-7e9abc23b26b"
	client, err := NewEchoService_Client("localhost", "a8aa34ac-1d57-46f9-8f44-852d73b515be")
	if err != nil {
		log.Println("17 ---> ", err)
		return
	}

	for i := 0; i < 10; i++ {
		val, err := client.Echo("", "Ceci est un test")
		if err != nil {
			log.Println("20 ---> ", err)
		} else {
			log.Println("23 ---> ", val)
		}
	}
}
