package media_client

import (
	//"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"
)

// Test various function here.
func TestMedia(t *testing.T) {

	// Connect to the plc client.
	client, err := NewMediaService_Client("globule-ryzen.globular.cloud", "media.MediaService")
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
