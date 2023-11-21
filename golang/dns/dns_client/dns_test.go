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
	domain                    = "globule-dell.globular.cloud"
	client, _                 = NewDnsService_Client(domain, "dns.DnsService")
	authentication_client_, _ = authentication_client.NewAuthenticationService_Client(domain, "authentication.AuthenticationService")
	token, err = authentication_client_.Authenticate("sa", "adminadmin")
)


// Test various function here.
func TestSetA(t *testing.T) {

	if authentication_client_ == nil {
		log.Println("authentication_client_ is nil")
	}

	// Set ip address
	ipv4 := Utility.MyIP()
	_, err := client.SetA(token, "globular.app", ipv4, 60)
	if err != nil {
		log.Println(err)
	}

	_, err = client.SetA(token, "globular.cloud", ipv4, 60)
	if err != nil {
		log.Println(err)
	}

	// not I will set a subdomain
	_, err = client.SetA(token, "ns1.globular.io", ipv4, 60)
	if err != nil {
		log.Println("----------> fail to set A ns1.globular.io with error", err)
	}

	_, err = client.SetA(token, "ns2.globular.io", ipv4, 60)
	if err != nil {
		log.Println("----------> fail to set A ns2.globular.io with error", err)
	}

	ipv6, err  := Utility.MyIPv6()
	if err != nil {
		log.Println(err)
	}

	_, err = client.SetAAAA(token, "globular.app", ipv6, 60)
	if err != nil {
		log.Println("----------> fail to set AAAA globular.app with error", err)
	}

	_, err = client.SetAAAA(token, "ns2.globular.io", ipv6, 60)
	if err != nil {
		log.Println("----------> fail to set AAAA ns2.globular.io with error", err)
	}

	_, err = client.SetAAAA(token, "ns1.globular.io", ipv6, 60)
	if err != nil {
		log.Println("----------> fail to set AAAA ns1.globular.io with error", err)
	}

}

/*
func TestResolve(t *testing.T) {

	// Connect to the plc client.
	log.Println("---> test resolve A")
	ipv4, err := client.GetA("syno.globular.io")
	if err == nil {
		log.Println("--> your ip is ", ipv4)
	} else {
		log.Panicln(err)
	}
}
*/

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

func TestTextValue(t *testing.T) {
	// Connect to the plc client.
	log.Println("---> test set text")
	err := client.SetText(token,"_acme-challenge.globular.cloud.", []string{"toto", "titi", "tata"}, 300)
	if err != nil {
		log.Panicln(err)
	}

	log.Println("---> test get text")
	values, err := client.GetText("_acme-challenge.globular.cloud.")
	if err != nil {
		log.Panicln(err)
	}

	log.Println("--> values retreive: ", values)

	log.Println("---> test remove text")
	/*err = client.RemoveText(token, "_acme-challenge.globular.cloud.")
	if err != nil {
		log.Panicln(err)
	}*/

}

func TestNsValue(t *testing.T) {
	// id, ns, mbox string, serial, refresh, retry, expire, minttl, ttl uint32

	id := "globular.cloud."
	ns := "ns1.globular.io."
	ttl := uint32(11200)

	client.RemoveNs(token, id)
	
	err := client.SetNs(token, id, ns, ttl)
	if err != nil {
		log.Panicln(err)
	}

	ns = "ns2.globular.io."

	err = client.SetNs(token, id, ns, ttl)
	if err != nil {
		log.Panicln(err)
	}
}


func TestSoaValue(t *testing.T) {
	// id, ns, mbox string, serial, refresh, retry, expire, minttl, ttl uint32
	
	id := "globular.cloud."

	client.RemoveSoa(token, id)

	ns := "ns1.globular.io."
	mbox := "admin.globular.io."
	serial := uint32(1)
	refresh := uint32(86400)
	retry := uint32(7200)
	expire := uint32(4000000)
	ttl := uint32(11200)

	err := client.SetSoa(token, id, ns, mbox, serial, refresh, retry, expire, ttl, ttl)
	if err != nil {
		log.Panicln(err)
	}

	ns = "ns2.globular.io."
	serial = uint32(2)


	err = client.SetSoa(token, id, ns, mbox, serial, refresh, retry, expire, ttl, ttl)
	if err != nil {
		log.Panicln(err)
	}
}
