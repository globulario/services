package echo_client

import (
	//"encoding/json"
	"log"
	"testing"
)

// Test various function here.
func TestEcho(t *testing.T) {

	// Connect to the plc client.
	client, err := NewEchoService_Client("globule-mac-mini.globular.cloud:10602", "echo.EchoService")
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
