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
	// "0f80ed1a-5d3a-46f1-a3f6-c091ac259665",  "b94d0011-39a0-4bdb-9a5c-7e9abc23b26b"
	//client, err := NewEchoService_Client("globule-mac-mini.globular.cloud:10602", "echo.EchoService")
	//client, err := NewEchoService_Client("globule-ryzen.globular.cloud:10202", "echo.EchoService")
	client, err := NewEchoService_Client("globule-dell.globular.cloud", "echo.EchoService")
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
