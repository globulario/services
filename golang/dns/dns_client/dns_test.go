package dns_client

import (
	//"encoding/json"
	"log"
	"testing"
	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/authentication/authentication_client"
)

var (
	// Try to connect to a nameserver.
	domain = "ns1.mycelius.com"
	client, _ = NewDnsService_Client(domain, "dns.DnsService")
	authentication_client_, _ = authentication_client.NewAuthenticationService_Client(domain, "authentication.AuthenticationService")
)

// Test various function here.
func TestSetA(t *testing.T) {
	log.Println("call authenticate")
	token, err := authentication_client_.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println("---> ", err)
	} else {
		log.Println("---> ", token)
	}

	// Set ip address
	domain, err := client.SetA(token, "globular.cloud", "peer0.globular.cloud", Utility.MyIP(), 60)
	if err == nil {
		log.Println(err)
	}

	log.Println("domain ", domain, "was register!")
}

func TestResolve(t *testing.T) {

	// Connect to the plc client.
	log.Println("---> test resolve A")
	ipv4, err := client.GetA("peer0.globular.cloud")
	if err == nil {
		log.Println("--> your ip is ", ipv4)
	} else {
		log.Panicln(err)
	}
}

/*func TestRemoveA(t *testing.T) {

	// Connect to the plc client.
	log.Println("---> test resolve A")
	err := client.RemoveA("toto")
	if err == nil {
		log.Println("--> your A is remove!")
	} else {
		log.Panicln(err)
	}
}*/

/*func TestTextValue(t *testing.T) {
	// Connect to the plc client.
	log.Println("---> test set text")
	err := client.SetText("toto", []string{"toto", "titi", "tata"})
	if err != nil {
		log.Panicln(err)
	}

	log.Println("---> test get text")
	values, err := client.GetText("toto")
	if err != nil {
		log.Panicln(err)
	}

	log.Println("--> values retreive: ", values)

	log.Println("---> test remove text")
	err = client.RemoveText("toto")
	if err != nil {
		log.Panicln(err)
	}
}*/
