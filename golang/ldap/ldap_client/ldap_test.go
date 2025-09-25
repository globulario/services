package ldap_client

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/globular_client"
	"github.com/go-ldap/ldap/v3"
)

// ---------------------------
// Test configuration helpers
// ---------------------------

type testCfg struct {
	Address   string // e.g. globule-ryzen.globular.cloud
	ServiceID string // e.g. ldap.LdapService

	LDAPHost string // host where the LDAP listener runs (defaults to Address's host)
	LDAPPort string // defaults to 636

	BindLogin string // e.g. "sa@globular.cloud" (for gRPC CreateConnection/Authenticate)
	BindDN    string // e.g. "cn=sa,dc=globular,dc=cloud" (for raw LDAP bind)
	BindPwd   string

	BaseDN   string // optional: for Search test
	Filter   string // optional: for Search test
	AttrsCSV string // optional: comma-separated list of attributes for Search test
}

func loadCfg() testCfg {
	addr := getenv("GLOBULAR_ADDRESS", "globule-ryzen.globular.cloud")
	service := getenv("LDAP_SERVICE_ID", "ldap.LdapService")
	ldapHost := getenv("LDAP_HOST", strings.Split(addr, ":")[0])
	ldapPort := getenv("LDAP_PORT", "636")

	return testCfg{
		Address:   addr,
		ServiceID: service,
		LDAPHost:  ldapHost,
		LDAPPort:  ldapPort,
		BindLogin: os.Getenv("LDAP_BIND_LOGIN"), // empty by default
		BindDN:    getenv("LDAP_BIND_DN", "cn=sa,dc=globular,dc=cloud"),
		BindPwd:   getenv("LDAP_BIND_PW", "adminadmin"),
		BaseDN:    os.Getenv("LDAP_BASE_DN"),
		Filter:    os.Getenv("LDAP_FILTER"),
		AttrsCSV:  os.Getenv("LDAP_ATTRS"),
	}
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// shared client for gRPC tests
var (
	cfg    = loadCfg()
	client *LDAP_Client
)

// TestMain sets up a single gRPC client used by subtests.
func TestMain(m *testing.M) {
	c, err := NewLdapService_Client(cfg.Address, cfg.ServiceID)
	if err != nil {
		fmt.Println("failed to init ldap client:", err)
		os.Exit(1)
	}
	client = c
	code := m.Run()
	client.Close()
	os.Exit(code)
}

// ---------------------------------
// Subtests
// ---------------------------------

// TestLDAP_TLS_Bind validates the raw LDAP protocol (LDAPS) using the same
// certificates/configuration that the gRPC client would use.
func TestLDAP_TLS_Bind(t *testing.T) {
	// get TLS config from running server via gRPC metadata
	tlsConfig, err := globular_client.GetClientTlsConfig(client)
	if err != nil {
		t.Fatalf("GetClientTlsConfig: %v", err)
	}

	addr := fmt.Sprintf("%s:%s", cfg.LDAPHost, cfg.LDAPPort)
	l, err := ldap.DialTLS("tcp", addr, tlsConfig)
	if err != nil {
		t.Fatalf("DialTLS(%s): %v", addr, err)
	}
	t.Cleanup(func() { _ = l.Close() })

	if err := l.Bind(cfg.BindDN, cfg.BindPwd); err != nil {
		t.Fatalf("LDAP bind failed for %s: %v", cfg.BindDN, err)
	}
}

// TestGRPC_CreateAuthenticate_Search exercises the gRPC surface.
// It is skipped unless LDAP_BIND_LOGIN, LDAP_BASE_DN and LDAP_FILTER are provided.
func TestGRPC_CreateAuthenticate_Search(t *testing.T) {
	if cfg.BindLogin == "" || cfg.BaseDN == "" || cfg.Filter == "" {
		t.Skipf("skipping: set LDAP_BIND_LOGIN, LDAP_BASE_DN and LDAP_FILTER to run this test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a temporary connection pointing to the same LDAP server
	connID := fmt.Sprintf("test_ldap_%d", time.Now().UnixNano())
	port := int32(389)
	if cfg.LDAPPort == "636" {
		port = 636
	}
	if err := client.CreateConnection(connID, cfg.BindLogin, cfg.BindPwd, cfg.LDAPHost, port); err != nil {
		t.Fatalf("CreateConnection(%s): %v", connID, err)
	}
	t.Cleanup(func() { _ = client.DeleteConnection(connID) })

	// Authenticate using that connection
	if err := client.Authenticate(connID, cfg.BindLogin, cfg.BindPwd); err != nil {
		t.Fatalf("Authenticate(%s): %v", connID, err)
	}

	// Optional search
	attrs := []string{}
	if cfg.AttrsCSV != "" {
		for _, a := range strings.Split(cfg.AttrsCSV, ",") {
			attrs = append(attrs, strings.TrimSpace(a))
		}
	}
	rows, err := client.Search(connID, cfg.BaseDN, cfg.Filter, attrs)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(rows) == 0 {
		t.Logf("Search returned 0 entries (this may be expected for the given filter)")
	}
	_ = ctx // reserved for future per-RPC deadlines when Invoke is used directly
}
