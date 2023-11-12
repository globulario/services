package echo_client

import (
	//"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"
)

// Test various function here.
func TestEcho(t *testing.T) {

	// Connect to the plc client.
	//client, err := NewEchoService_Client("globule-mac-mini.globular.cloud:10602", "echo.EchoService")
	//client, err := NewEchoService_Client("globule-ryzen.globular.cloud:10202", "echo.EchoService")
	client, err := NewEchoService_Client("globule-ryzen.globular.cloud", "echo.EchoService")
	if err != nil {
		log.Println("17 ---> ", err)
		return
	}

	for i := 0; i < 10; i++ {
		
		val, err := client.Echo("", "Ceci est un test")
		err_ := client.Reconnect()
		fmt.Println("===> ", err_)
		if err != nil {
			log.Println("20 ---> ", err)
		} else {
			log.Println("23 ---> ", val)
		}
		time.Sleep(1 * time.Second)
	}
}
