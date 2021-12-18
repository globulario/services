package authentication_client

import (
	"log"
	"testing"
	"fmt"
	"time"
)

var (
	// Connect to the plc client.
	client, _ = NewAuthenticationService_Client("globule-dell", "authentication.AuthenticationService")
)

func BenchmarkAuthenticate(b *testing.B) {
	for n := 0; n < b.N; n++ {
		token, err := client.Authenticate("sa", "adminadmin")
		if err != nil {
			log.Println("Fail to authenticate with error ", err)
		} else {
			log.Println("Authenticate succeed", token)
		}
	}

	// Now I will test to authenticate the root...
	/*token, err = client.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println("Fail to authenticate with error ", err)
	} else {
		log.Println("Authenticate succeed", token)
	}*/
}

func TestAuthenticate(t *testing.T) {
	fmt.Println("client authenticate request ", time.Now().Unix())
	token, err := client.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println("Fail to authenticate with error ", err)
	} else {
		log.Println("Authenticate succeed", token)
	}

	// Now I will test to authenticate the root...
	/*token, err = client.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println("Fail to authenticate with error ", err)
	} else {
		log.Println("Authenticate succeed", token)
	}*/
	fmt.Println("client authenticate response ", time.Now().Unix())
}
/*

func TestSetPassword(t *testing.T) {
	token, err := client.SetPassword("dave", "400zm89Aaa", "400zm89Aaa")
	if err != nil {
		log.Println("Fail to authenticate with error ", err)
	} else {
		log.Println("Password is updated ", token)
	}
}

func TestSetRootPassword(t *testing.T) {
	token, err := client.SetRootPassword("adminadmin", "adminadmin")
	if err != nil {
		log.Println("Fail to authenticate with error ", err)
	} else {
		log.Println("Password is updated ", token)
	}
}
*/
