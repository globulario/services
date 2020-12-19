package echo_client

import (
	//"encoding/json"
	"log"
	"testing"
)

// Test various function here.
func TestEcho(t *testing.T) {

	// Connect to the plc client.
	client, err := NewEchoService_Client("monl580", "echo.EchoService")
	if err != nil {
		log.Println("17 ---> ", err)
		return
	}

	val, err := client.Echo("Ceci est un test")
	if err != nil {
		log.Println("23 ---> ", err)
	} else {
		log.Println("25 ---> ", val)
	}
}
