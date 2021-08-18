package dns_client

import (
	//"encoding/json"
	"log"
	"testing"

	"github.com/globulario/services/golang/authentication/authentication_client"
)

var (
	// Try to connect to a nameserver.
	token                     = ""
	domain                    = "ns1.mycelius.com"
	client, _                 = NewDnsService_Client(domain, "dns.DnsService")
	authentication_client_, _ = authentication_client.NewAuthenticationService_Client(domain, "authentication.AuthenticationService")
)

// Test various function here.
func TestSetA(t *testing.T) {
	log.Println("call authenticate")
	var err error
	token, err = authentication_client_.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println("---> ", err)
	} else {
		log.Println("---> ", token)
	}

	// Set ip address
	domain, err := client.SetA(token, "globular.cloud", "peer0.globular.cloud", "70.55.95.217", 60)
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
/*
func TestTextValue(t *testing.T) {
	// Connect to the plc client.
	log.Println("---> test set text")
	err := client.SetText(token,"key_0.cargowebserver.com.", []string{"toto", "titi", "tata"}, 300)
	if err != nil {
		log.Panicln(err)
	}

	log.Println("---> test get text")
	values, err := client.GetText("key_0.cargowebserver.com.")
	if err != nil {
		log.Panicln(err)
	}

	log.Println("--> values retreive: ", values)

	log.Println("---> test remove text")
	err = client.RemoveText(token, "toto")
	if err != nil {
		log.Panicln(err)
	}

}
*/
func TestNsValue(t *testing.T) {
	// id, ns, mbox string, serial, refresh, retry, expire, minttl, ttl uint32
	id := "globular.io."
	ns := "ns1.mycelius.com."
	ttl := uint32(11200)

	err := client.SetNs(token, id, ns, ttl)
	if err != nil {
		log.Panicln(err)
	}
}

func TestSoaValue(t *testing.T) {
	// id, ns, mbox string, serial, refresh, retry, expire, minttl, ttl uint32
	id := "globular.io."
	ns := "ns1.mycelius.com."
	mbox := "admin.cargowebserver.com."
	serial := uint32(1111111111)
	refresh := uint32(86400)
	retry := uint32(7200)
	expire := uint32(4000000)
	ttl := uint32(11200)

	err := client.SetSoa(token, id, ns, mbox, serial, refresh, retry, expire, ttl, ttl)
	if err != nil {
		log.Panicln(err)
	}
}
