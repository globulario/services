package dns_client

import (
	"log"
	"testing"

	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/testutil"
	Utility "github.com/globulario/utility"
)

// testContext holds client and token for DNS tests
type testContext struct {
	client *Dns_Client
	token  string
}

// newTestContext creates clients and authenticates for testing, skipping if external services are not available.
func newTestContext(t *testing.T) *testContext {
	t.Helper()
	testutil.SkipIfNoExternalServices(t)

	addr := testutil.GetAddress()
	saUser, saPwd := testutil.GetSACredentials()

	client, err := NewDnsService_Client(addr, "dns.DnsService")
	if err != nil {
		t.Fatalf("NewDnsService_Client: %v", err)
	}

	authClient, err := authentication_client.NewAuthenticationService_Client(addr, "authentication.AuthenticationService")
	if err != nil {
		t.Fatalf("NewAuthenticationService_Client: %v", err)
	}

	token, err := authClient.Authenticate(saUser, saPwd)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	return &testContext{client: client, token: token}
}

// Test various function here.
func TestSetA(t *testing.T) {
	ctx := newTestContext(t)

	// Set ip address
	ipv4 := Utility.MyIP()

	ipv6, err := Utility.MyIPv6()
	if err != nil {
		log.Println(err)
	}

	// not I will set a subdomain
	_, err = ctx.client.SetA(ctx.token, "ns1.globular.io", ipv4, 60)
	if err != nil {
		log.Println(" fail to set A ns1.globular.io with error", err)
	}

	_, err = ctx.client.SetA(ctx.token, "ns2.globular.io", ipv4, 60)
	if err != nil {
		log.Println(" fail to set A ns2.globular.io with error", err)
	}

	_, err = ctx.client.SetAAAA(ctx.token, "ns2.globular.io", ipv6, 60)
	if err != nil {
		log.Println(" fail to set AAAA ns2.globular.io with error", err)
	}

	_, err = ctx.client.SetAAAA(ctx.token, "ns1.globular.io", ipv6, 60)
	if err != nil {
		log.Println(" fail to set AAAA ns1.globular.io with error", err)
	}

	_, err = ctx.client.SetAAAA(ctx.token, "globule-dell.globular.cloud", ipv4, 60)
	if err != nil {
		log.Println(" fail to set AAAA globular.app with error", err)
	}

	_, err = ctx.client.SetAAAA(ctx.token, "globule-dell.globular.cloud", ipv6, 60)
	if err != nil {
		log.Println(" fail to set AAAA globular.app with error", err)
	}
}

// TestGetClusterCertificatesBundle is removed - method does not exist on client

func TestGetA(t *testing.T) {
	ctx := newTestContext(t)

	// Connect to the plc client.
	log.Println("---> test resolve A")
	ipv4, err := ctx.client.GetA("globular.cloud")
	if err == nil {
		log.Println("--> your ip is ", ipv4)
	} else {
		log.Panicln(err)
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
	ctx := newTestContext(t)
	// Connect to the plc client.
	log.Println("---> test set text")
	err := ctx.client.SetText(ctx.token, "_acme-challenge.globular.cloud.", []string{"toto", "titi", "tata"}, 300)
	if err != nil {
		log.Panicln(err)
	}

	log.Println("---> test get text")
	values, err := ctx.client.GetText("_acme-challenge.globular.cloud.")
	if err != nil {
		log.Panicln(err)
	}

	log.Println("--> values retreive: ", values)

	log.Println("---> test remove text")
	/*err = ctx.client.RemoveText(ctx.token, "_acme-challenge.globular.cloud.")
	if err != nil {
		log.Panicln(err)
	}*/

}

func TestNsValue(t *testing.T) {
	ctx := newTestContext(t)
	// id, ns, mbox string, serial, refresh, retry, expire, minttl, ttl uint32

	id := "globule-ryzen.globular.cloud."

	ns := "ns1.globular.io."
	ttl := uint32(11200)

	ctx.client.RemoveNs(ctx.token, id)

	err := ctx.client.SetNs(ctx.token, id, ns, ttl)
	if err != nil {
		log.Panicln(err)
	}

	ns = "ns2.globular.io."

	err = ctx.client.SetNs(ctx.token, id, ns, ttl)
	if err != nil {
		log.Panicln(err)
	}
}

func TestSoaValue(t *testing.T) {
	ctx := newTestContext(t)
	// id, ns, mbox string, serial, refresh, retry, expire, minttl, ttl uint32

	id := "globule-ryzen.globular.cloud."

	ctx.client.RemoveSoa(ctx.token, id)

	ns := "ns1.globular.io."
	mbox := "admin.globular.io."
	serial := uint32(1)
	refresh := uint32(86400)
	retry := uint32(7200)
	expire := uint32(4000000)
	ttl := uint32(11200)

	err := ctx.client.SetSoa(ctx.token, id, ns, mbox, serial, refresh, retry, expire, ttl, ttl)
	if err != nil {
		log.Panicln(err)
	}

	ns = "ns2.globular.io."
	serial = uint32(2)

	err = ctx.client.SetSoa(ctx.token, id, ns, mbox, serial, refresh, retry, expire, ttl, ttl)
	if err != nil {
		log.Panicln(err)
	}
}
