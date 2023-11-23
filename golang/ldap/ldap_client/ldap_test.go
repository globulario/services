package ldap_client

import (

	"testing"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	"github.com/go-ldap/ldap/v3"

)

var (
	// Connect to the plc client.
	address = "globule-ryzen.globular.cloud"
	client, _ = NewLdapService_Client("globule-ryzen.globular.cloud", "ldap.LdapService")
	ldapAddress = address+":636"
	localConfig, _ = config.GetLocalConfig(true)
	certPath = config.GetConfigDir() + "/" + localConfig["Certificate"].(string)

)

// Test ldap server via ldap protocol via tls.

func TestLDAPBind(t *testing.T) {
	// Replace these values with your LDAP server details
	bindUsername := "cn=sa,dc=globular,dc=cloud"
	bindPassword := "adminadmin"

	// get the tls client config
	tlsConfig, err := globular_client.GetClientTlsConfig(client)
	if err != nil {
		t.Fatalf("Failed to get client TLS config: %v", err)
	}

	// Create an LDAP connection with TLS
	l, err := ldap.DialTLS("tcp", ldapAddress, 	tlsConfig)

	if err != nil {
		t.Fatalf("Failed to connect to the LDAP server: %v", err)
	}
	defer l.Close()

	// Bind to the LDAP server with the provided username and password
	err = l.Bind(bindUsername, bindPassword)
	if err != nil {
		t.Fatalf("LDAP bind failed: %v", err)
	}

	// If the bind is successful, you can perform additional LDAP operations here

	// Output a success message
	t.Logf("LDAP bind successful for user %s", bindUsername)
}

// First test create a fresh new connection...
/*
func TestCreateConnection(t *testing.T) {
	fmt.Println("Connection creation test.")

	err := client.CreateConnection("test_ldap", "mrmfct037@UD6.UF6", "Dowty123", "mon-dc-p01.UD6.UF6", 389)
	if err != nil {
		log.Println(err)
	}
	log.Println("Connection created!")
}
*/

// Test a ldap query.
/*
func TestSearch(t *testing.T) {

	// I will execute a simple ldap search here...
	results, err := client.Search("test_ldap", "OU=Users,OU=MON,OU=CA,DC=UD6,DC=UF6", "(&(!(givenName=Machine*))(objectClass=user))", []string{"sAMAccountName", "mail", "memberOf"})
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("results found: ", len(results))
	for i := 0; i < len(results); i++ {
		log.Println(results[i])
	}
}
*/

// Test a ldap query.
/*
func TestDeleteConnection(t *testing.T) {
	err := client.DeleteConnection("test_ldap")
	if err != nil {
		log.Println(err)
	}
	log.Println("Connection deleted!")
}
*/
