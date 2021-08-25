package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
)

var (
	Root       = config.GetRootDir()
	ConfigPath = config.GetConfigDir() + "/config.json"
	keyPath    = config.GetConfigDir() + "/keys"
)

// That function will be access via http so event server or client will be able
// to get particular service configuration.
func GetClientConfig(address string, name string, port int, path string) (map[string]interface{}, error) {

	log.Println("get client configuration for ", name, address)
	var serverConfig map[string]interface{}
	var config map[string]interface{}
	var err error
	if len(address) == 0 {
		err := errors.New("no address was given for service name " + name)
		return nil, err
	}

	// In case of local service I will get the service value directly from
	// the configuration file.
	serverConfig, err = getLocalConfig()
	isLocal := true
	if err == nil {
		domain := serverConfig["Domain"].(string)
		if len(serverConfig["Name"].(string)) > 0 {
			domain = serverConfig["Name"].(string) + "." + domain
		}
		if domain != address {
			isLocal = false
		}
	} else {
		isLocal = false
	}

	if !isLocal {
		// First I will retreive the server configuration.
		log.Println("get remote client configuration for ", address, port)
		serverConfig, err = getRemoteConfig(address, port)
		if err != nil {
			return nil, err
		}
	}

	// get service by id or by name... (take the first service with a given name in case of name.
	for _, s := range serverConfig["Services"].(map[string]interface{}) {
		if s.(map[string]interface{})["Name"].(string) == name || s.(map[string]interface{})["Id"].(string) == name {
			config = s.(map[string]interface{})
			break
		}
	}

	// No service with name or id was found...
	if config == nil {
		return nil, errors.New("No service found whit name " + name + " exist on the server.")
	}

	// Set the config tls...
	config["TLS"] = serverConfig["Protocol"].(string) == "https"
	config["Domain"] = address

	// get / init credential values.
	if config["TLS"] == false {
		// set the credential function here
		config["KeyFile"] = ""
		config["CertFile"] = ""
		config["CertAuthorityTrust"] = ""
	} else {
		// Here I will retreive the credential or create it if not exist.
		var country string
		if serverConfig["Country"] != nil {
			country = serverConfig["Country"].(string)
		}

		var state string
		if serverConfig["State"] != nil {
			state = serverConfig["State"].(string)
		}

		var city string
		if serverConfig["City"] != nil {
			city = serverConfig["City"].(string)
		}

		var organization string
		if serverConfig["Organization"] != nil {
			state = serverConfig["Organization"].(string)
		}

		var alternateDomains []interface{}
		if serverConfig["AlternateDomains"] != nil {
			alternateDomains = serverConfig["AlternateDomains"].([]interface{})
		}

		if !isLocal {
			domain := serverConfig["Domain"].(string)
			if len(serverConfig["Name"].(string)) > 0 {
				domain = serverConfig["Name"].(string) + "." + domain
			}
			keyPath, certPath, caPath, err := getCredentialConfig(path, domain, country, state, city, organization, alternateDomains, port)
			if err != nil {
				log.Println("Fail to retreive credential configuration with error ", err)
				return nil, err
			}

			// set the credential function here
			config["KeyFile"] = keyPath
			config["CertFile"] = certPath
			config["CertAuthorityTrust"] = caPath
		}
	}
	return config, nil
}

func InstallCertificates(domain string, port int, path string) (string, string, string, error) {
	return getCredentialConfig(path, domain, "", "", "", "", []interface{}{}, port)
}

/**
 * Return the server local configuration if one exist.
 */
func getLocalConfig() (map[string]interface{}, error) {

	if !Utility.Exists(ConfigPath) {
		return nil, errors.New("no local Globular configuration found")
	}

	config := make(map[string]interface{})
	data, err := ioutil.ReadFile(ConfigPath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	// Now I will read the services configurations...
	config["Services"] = make(map[string]interface{})

	// use the GLOBULAR_SERVICES_ROOT path if it set... or the Root (/usr/local/share/globular)
	serviceDir := os.Getenv("GLOBULAR_SERVICES_ROOT")
	if len(serviceDir) == 0 {
		serviceDir = Root
	}

	filepath.Walk(serviceDir, func(path string, info os.FileInfo, err error) error {
		path = strings.ReplaceAll(path, "\\", "/")
		if info == nil {
			return nil
		}

		if err == nil && info.Name() == "config.json" {
			// So here I will read the content of the file.
			s := make(map[string]interface{})
			data, err := ioutil.ReadFile(path)
			if err == nil {
				// Read the config file.
				err := json.Unmarshal(data, &s)
				if err == nil {
					config["Services"].(map[string]interface{})[s["Id"].(string)] = s
				}
			}
		}
		return nil
	})

	return config, nil
}

/**
 * Get the remote client configuration.
 */
func getRemoteConfig(address string, port int) (map[string]interface{}, error) {

	// Here I will get the configuration information from http...
	var resp *http.Response
	var err error
	var configAddress = "http://" + address + ":" + Utility.ToString(port) + "/config"
	resp, err = http.Get(configAddress)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var config map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

/**
 * Get the ca certificate
 */
func getCaCertificate(address string, port int) (string, error) {

	if len(address) == 0 {
		return "", errors.New("no address was given")
	}

	// Here I will get the configuration information from http...
	var resp *http.Response
	var err error
	var caAddress = "http://" + address + ":" + Utility.ToString(port) + "/get_ca_certificate"

	resp, err = http.Get(caAddress)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {

		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}

		return string(bodyBytes), nil
	}

	return "", errors.New("fail to retreive ca certificate with error " + Utility.ToString(resp.StatusCode))
}

func signCaCertificate(address string, csr string, port int) (string, error) {

	if len(address) == 0 {
		return "", errors.New("no address was given")
	}

	csr_str := base64.StdEncoding.EncodeToString([]byte(csr))
	// Here I will get the configuration information from http...
	var resp *http.Response
	var err error
	var signCertificateAddress = "http://" + address + ":" + Utility.ToString(port) + "/sign_ca_certificate"

	resp, err = http.Get(signCertificateAddress + "?csr=" + csr_str)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}

		return string(bodyBytes), nil
	}

	return "", errors.New("fail to sign ca certificate with error " + Utility.ToString(resp.StatusCode))
}

/**
 * Return the credential configuration.
 */
func getCredentialConfig(basePath string, domain string, country string, state string, city string, organization string, alternateDomains []interface{}, port int) (keyPath string, certPath string, caPath string, err error) {

	log.Println("get credential config for domain ", domain)
	// TODO Clarify the use of the password here.
	pwd := "1111"

	// use the temp dir to store the certificate in that case.
	path := basePath + "/config/tls"
	// must have write access of file.
	_, err = ioutil.ReadFile(path + "/" + domain + "/client.pem")
	if err != nil {
		path = basePath + "/config/tls"
		err = nil
	}

	// Create a new directory to put the credential.
	creds := path + "/" + domain

	// Return the existing paths...
	if Utility.Exists(creds) &&
		Utility.Exists(creds+"/client.pem") &&
		Utility.Exists(creds+"/client.crt") &&
		Utility.Exists(creds+"/ca.crt") {
		info, _ := os.Stat(creds)

		// test if the certificate are older than 5 mount.
		if info.ModTime().Add(24*30*5*time.Hour).Unix() < time.Now().Unix() {
			os.RemoveAll(creds)
		} else {

			keyPath = creds + "/client.pem"
			certPath = creds + "/client.crt"
			caPath = creds + "/ca.crt"
			return
		}
	}

	err = Utility.CreateDirIfNotExist(creds)
	if err != nil {
		log.Println(err)
		return "", "", "", err
	}

	// I will connect to the certificate authority of the server where the application must
	// be deployed. Certificate autority run wihtout tls.

	// Get the ca.crt certificate.
	ca_crt, err := getCaCertificate(domain, port)
	if err != nil {
		return "", "", "", err
	}

	// Write the ca.crt file on the disk
	err = ioutil.WriteFile(creds+"/ca.crt", []byte(ca_crt), 0444)
	if err != nil {
		return "", "", "", err
	}

	// Now I will generate the certificate for the client...
	// Step 1: Generate client private key.
	err = GenerateClientPrivateKey(creds, pwd)
	if err != nil {
		return "", "", "", err
	}

	alternateDomains_ := make([]string, 0)
	for i := 0; i < len(alternateDomains); i++ {
		alternateDomains_ = append(alternateDomains_, alternateDomains[i].(string))
	}

	// generate the SAN file
	err = GenerateSanConfig(domain, creds, country, state, city, organization, alternateDomains_)
	if err != nil {
		return "", "", "", err
	}

	// Step 2: Generate the client signing request.
	err = GenerateClientCertificateSigningRequest(creds, pwd, domain)
	if err != nil {
		return "", "", "", err
	}

	// Step 3: Generate client signed certificate.
	client_csr, err := ioutil.ReadFile(creds + "/client.csr")
	if err != nil {
		return "", "", "", err
	}

	// Sign the certificate from the server ca...
	client_crt, err := signCaCertificate(domain, string(client_csr), Utility.ToInt(port))
	if err != nil {
		return "", "", "", err
	}

	// Write bact the client certificate in file on the disk
	err = ioutil.WriteFile(creds+"/client.crt", []byte(client_crt), 0444)
	if err != nil {
		return "", "", "", err
	}

	// Now ask the ca to sign the certificate.

	// Step 4: Convert to pem format.
	err = KeyToPem("client", creds, pwd)
	if err != nil {
		return "", "", "", err
	}

	// set the credential paths.
	keyPath = creds + "/client.pem"
	certPath = creds + "/client.crt"
	caPath = creds + "/ca.crt"

	return
}

//////////////////////////////// Certificate Authority /////////////////////////

// Generate the Certificate Authority private key file (this shouldn't be shared in real life)
func GenerateAuthorityPrivateKey(path string, pwd string) error {
	if Utility.Exists(path + "/ca.key") {
		return nil
	}

	cmd := "openssl"
	args := make([]string, 0)
	args = append(args, "genrsa")
	args = append(args, "-passout")
	args = append(args, "pass:"+pwd)
	args = append(args, "-des3")
	args = append(args, "-out")
	args = append(args, path+"/ca.key")
	args = append(args, "4096")

	err := exec.Command(cmd, args...).Run()
	if err != nil || !Utility.Exists(path+"/ca.key") {
		if err == nil {
			err = errors.New("fail to generate the Authority private key " + path + "/ca.key")
		}

		return err
	}
	return nil
}

// Certificate Authority trust certificate (this should be shared whit users)
func GenerateAuthorityTrustCertificate(path string, pwd string, expiration_delay int, domain string) error {
	if Utility.Exists(path + "/ca.crt") {
		return nil
	}

	cmd := "openssl"
	args := make([]string, 0)
	args = append(args, "req")
	args = append(args, "-passin")
	args = append(args, "pass:"+pwd)
	args = append(args, "-new")
	args = append(args, "-x509")
	args = append(args, "-days")
	args = append(args, strconv.Itoa(expiration_delay))
	args = append(args, "-key")
	args = append(args, path+"/ca.key")
	args = append(args, "-out")
	args = append(args, path+"/ca.crt")
	args = append(args, "-subj")
	args = append(args, "/CN=Root CA")

	err := exec.Command(cmd, args...).Run()
	if err != nil || !Utility.Exists(path+"/ca.crt") {
		if err == nil {
			err = errors.New("fail to generate the trust certificate " + path + "/ca.crt")
		}

		return err
	}

	return nil
}

/////////////////////// Server Keys //////////////////////////////////////////

// Server private key, password protected (this shoudn't be shared)
func GenerateSeverPrivateKey(path string, pwd string) error {
	if Utility.Exists(path + "/server.key") {
		return nil
	}
	cmd := "openssl"
	args := make([]string, 0)
	args = append(args, "genrsa")
	args = append(args, "-passout")
	args = append(args, "pass:"+pwd)
	args = append(args, "-des3")
	args = append(args, "-out")
	args = append(args, path+"/server.key")
	args = append(args, "4096")

	err := exec.Command(cmd, args...).Run()
	if err != nil || !Utility.Exists(path+"/server.key") {
		if err == nil {
			err = errors.New("fail to generate server private key " + path + "/server.key")
		}

		return err
	}
	return nil
}

// Generate client private key and certificate.
func GenerateClientPrivateKey(path string, pwd string) error {
	if Utility.Exists(path + "/client.key") {
		return nil
	}

	cmd := "openssl"
	args := make([]string, 0)
	args = append(args, "genrsa")
	args = append(args, "-passout")
	args = append(args, "pass:"+pwd)
	args = append(args, "-des3")
	args = append(args, "-out")
	args = append(args, path+"/client.pass.key")
	args = append(args, "4096")

	err := exec.Command(cmd, args...).Run()
	if err != nil || !Utility.Exists(path+"/client.pass.key") {
		if err == nil {
			err = errors.New("fail to generate client private key " + path + "/client.pass.key")
		}

		return err
	}

	args = make([]string, 0)
	args = append(args, "rsa")
	args = append(args, "-passin")
	args = append(args, "pass:"+pwd)
	args = append(args, "-in")
	args = append(args, path+"/client.pass.key")
	args = append(args, "-out")
	args = append(args, path+"/client.key")

	err = exec.Command(cmd, args...).Run()
	if err != nil || !Utility.Exists(path+"/client.key") {
		if err == nil {
			err = errors.New("fail to generate client private key " + path + "/client.key")
		}

		return err
	}

	// Remove the file.
	err = os.Remove(path + "/client.pass.key")
	if err != nil {
		return errors.New("fail to remove intermediate key client.pass.key")
	}
	return nil
}

func GenerateClientCertificateSigningRequest(path string, pwd string, domain string) error {
	if Utility.Exists(path + "/client.csr") {
		return nil
	}

	cmd := "openssl"
	args := make([]string, 0)
	args = append(args, "req")
	args = append(args, "-new")
	args = append(args, "-key")
	args = append(args, path+"/client.key")
	args = append(args, "-out")
	args = append(args, path+"/client.csr")
	args = append(args, "-subj")
	args = append(args, "/CN="+domain)
	args = append(args, "-config")
	args = append(args, path+"/san.conf")

	err := exec.Command(cmd, args...).Run()

	if err != nil || !Utility.Exists(path+"/client.csr") {
		if err == nil {
			err = errors.New("fail to generate client certificate signing request " + path + "/client.key")
		}

		return err
	}

	return nil
}

func GenerateSignedClientCertificate(path string, pwd string, expiration_delay int) error {

	if Utility.Exists(path + "/client.crt") {
		return nil
	}

	cmd := "openssl"
	args := make([]string, 0)
	args = append(args, "x509")
	args = append(args, "-req")
	args = append(args, "-passin")
	args = append(args, "pass:"+pwd)
	args = append(args, "-days")
	args = append(args, strconv.Itoa(expiration_delay))
	args = append(args, "-in")
	args = append(args, path+"/client.csr")
	args = append(args, "-CA")
	args = append(args, path+"/ca.crt")
	args = append(args, "-CAkey")
	args = append(args, path+"/ca.key")
	args = append(args, "-set_serial")
	args = append(args, "01")
	args = append(args, "-out")
	args = append(args, path+"/client.crt")
	args = append(args, "-extfile")
	args = append(args, path+"/san.conf")
	args = append(args, "-extensions")
	args = append(args, "v3_req")

	err := exec.Command(cmd, args...).Run()
	if err != nil || !Utility.Exists(path+"/client.crt") {
		if err == nil {
			err = errors.New("fail to get the signed server certificate " + path + "/client.key")
		}

		return err
	}

	return nil
}

func GenerateSanConfig(domain, path, country, state, city, organization string, alternateDomains []string) error {

	config := fmt.Sprintf(`
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
C = %s
ST =  %s
L =  %s
O	=  %s
CN =  %s

[v3_req]
# Extensions to add to a certificate request
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names

[alt_names]
`, country, state, city, organization, domain)
	// TODO filter wild card domains here ...
	if !Utility.Contains(alternateDomains, domain) {
		alternateDomains = append(alternateDomains, domain)
	}

	// set alternate domain
	for i := 0; i < len(alternateDomains); i++ {
		config += fmt.Sprintf("DNS.%d = %s \n", i, alternateDomains[i])
	}

	if Utility.Exists(path + "/san.conf") {
		return nil
	}

	f, err := os.Create(path + "/san.conf")
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(config)

	return err
}

// Server certificate signing request (this should be shared with the CA owner)
func GenerateServerCertificateSigningRequest(path string, pwd string, domain string) error {

	if Utility.Exists(path + "/server.crs") {
		return nil
	}

	cmd := "openssl"
	args := make([]string, 0)
	args = append(args, "req")
	args = append(args, "-passin")
	args = append(args, "pass:"+pwd)
	args = append(args, "-new")
	args = append(args, "-key")
	args = append(args, path+"/server.key")
	args = append(args, "-out")
	args = append(args, path+"/server.csr")
	args = append(args, "-subj")
	args = append(args, "/CN="+domain)
	args = append(args, "-config")
	args = append(args, path+"/san.conf")

	err := exec.Command(cmd, args...).Run()
	if err != nil || !Utility.Exists(path+"/server.csr") {
		if err == nil {
			err = errors.New("fail to generate server certificate signing request" + path + "/client.key")
		}

		return err
	}

	return nil
}

// Server certificate signed by the CA (this would be sent back to the client by the CA owner)
func GenerateSignedServerCertificate(path string, pwd string, expiration_delay int) error {

	if Utility.Exists(path + "/server.crt") {
		return nil
	}

	cmd := "openssl"
	args := make([]string, 0)
	args = append(args, "x509")
	args = append(args, "-req")
	args = append(args, "-passin")
	args = append(args, "pass:"+pwd)
	args = append(args, "-days")
	args = append(args, strconv.Itoa(expiration_delay))
	args = append(args, "-in")
	args = append(args, path+"/server.csr")
	args = append(args, "-CA")
	args = append(args, path+"/ca.crt")
	args = append(args, "-CAkey")
	args = append(args, path+"/ca.key")
	args = append(args, "-set_serial")
	args = append(args, "01")
	args = append(args, "-out")
	args = append(args, path+"/server.crt")
	args = append(args, "-extfile")
	args = append(args, path+"/san.conf")
	args = append(args, "-extensions")
	args = append(args, "v3_req")

	err := exec.Command(cmd, args...).Run()
	if err != nil || !Utility.Exists(path+"/server.crt") {
		if err == nil {
			err = errors.New("fail to get the signed server certificate" + path + "/server.key")
		}

		return err

	}

	return nil
}

// Conversion of server.key into a format gRpc likes (this shouldn't be shared)
func KeyToPem(name string, path string, pwd string) error {
	if Utility.Exists(path + "/" + name + ".pem") {
		return nil
	}

	cmd := "openssl"
	args := make([]string, 0)
	args = append(args, "pkcs8")
	args = append(args, "-topk8")
	args = append(args, "-nocrypt")
	args = append(args, "-passin")
	args = append(args, "pass:"+pwd)
	args = append(args, "-in")
	args = append(args, path+"/"+name+".key")
	args = append(args, "-out")
	args = append(args, path+"/"+name+".pem")

	err := exec.Command(cmd, args...).Run()
	if err != nil || !Utility.Exists(path+"/"+name+".key") {
		if err == nil {
			err = errors.New("Fail to generate " + name + ".pem key from " + name + ".key")
		}

		return err
	}

	return nil
}

/**
 * That function is use to generate services certificates.
 * Private ca.key, server.key, server.pem, server.crt
 * Share ca.crt (needed by the client), server.csr (needed by the CA)
 */
func GenerateServicesCertificates(pwd string, expiration_delay int, domain string, path string, country string, state string, city string, organization string, alternateDomains []interface{}) error {
	if Utility.Exists(path + "/client.crt") {
		return nil // certificate are already created.
	}
	alternateDomains_ := make([]string, 0)
	for i := 0; i < len(alternateDomains); i++ {
		alternateDomains_ = append(alternateDomains_, alternateDomains[i].(string))
	}

	// Generate the SAN configuration.
	err := GenerateSanConfig(domain, path, country, state, city, organization, alternateDomains_)
	if err != nil {
		log.Println(err)
		return err
	}
	err = GenerateAuthorityPrivateKey(path, pwd)
	if err != nil {
		log.Println(err)
		return err
	}
	err = GenerateAuthorityTrustCertificate(path, pwd, expiration_delay, domain)
	if err != nil {
		log.Println(err)
		return err
	}
	err = GenerateSeverPrivateKey(path, pwd)
	if err != nil {
		log.Println(err)
		return err
	}
	err = GenerateServerCertificateSigningRequest(path, pwd, domain)
	if err != nil {
		log.Println(err)
		return err
	}
	err = GenerateSignedServerCertificate(path, pwd, expiration_delay)
	if err != nil {
		log.Println(err)
		return err
	}
	err = KeyToPem("server", path, pwd)
	if err != nil {
		log.Println(err)
		return err
	}
	err = GenerateClientPrivateKey(path, pwd)
	if err != nil {
		log.Println(err)
		return err
	}
	err = GenerateClientCertificateSigningRequest(path, pwd, domain)
	if err != nil {
		log.Println(err)
		return err
	}
	err = GenerateSignedClientCertificate(path, pwd, expiration_delay)
	if err != nil {
		log.Println(err)
		return err
	}
	err = KeyToPem("client", path, pwd)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////////
// Peer key generation. Diffie-Hellman
//
// https://www.youtube.com/watch?v=NmM9HA2MQGI&ab_channel=Computerphile
//
////////////////////////////////////////////////////////////////////////////////////

/**
 * Generate keys and save it at given path.
 */
func GeneratePeerKeys(id string) error {

	id = strings.ReplaceAll(id, ":", "_")

	if Utility.Exists(keyPath + "/" + id + "_private") {
		return nil // not realy an error the key already exist.
	}

	// Use ecdsa to generate a key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return err
	}

	// Use 509
	private, err := x509.MarshalECPrivateKey(privateKey) //here
	if err != nil {
		return err
	}

	//pem
	block := pem.Block{
		Type:  "esdsa private key",
		Bytes: private,
	}

	err = Utility.CreateDirIfNotExist(keyPath)
	if err != nil {
		return err
	}

	file, err := os.Create(keyPath + "/" + id + "_private")
	if err != nil {
		return err
	}
	err = pem.Encode(file, &block)
	if err != nil {
		return err
	}

	defer file.Close()

	// Handle the public key
	public := privateKey.PublicKey

	//x509 serialization
	publicKey, err := x509.MarshalPKIXPublicKey(&public)
	if err != nil {
		return err
	}

	//pem
	public_block := pem.Block{
		Type:  "ecdsa public key",
		Bytes: publicKey,
	}

	file, err = os.Create(keyPath + "/" + id + "_public")
	if err != nil {
		return err
	}

	//pem encoding
	err = pem.Encode(file, &public_block)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Return the local jwt key
 */
func GetLocalKey() ([]byte, error) {
	// In that case the public key will be use as a token key...
	// That token will be valid on the peer itself.
	id := strings.ReplaceAll(Utility.MyMacAddr(), ":", "_")
	return ioutil.ReadFile(keyPath + "/" + id + "_public")
}

/**
 * Return a jwt token key for a given peer id (mac address)
 */
func GetPeerKey(id string) ([]byte, error) {
	id = strings.ReplaceAll(id, ":", "_")

	if id == strings.ReplaceAll(Utility.MyMacAddr(), ":", "_"){
		return GetLocalKey()
	}


	fmt.Println("Get peer key, ", keyPath+"/"+id+"_public")

	// If the token issuer is not the actual globule but another peer
	// I will use it public key and my private one to generate the correct key.
	err := Utility.CreateDirIfNotExist(keyPath)
	if err != nil {
		return nil, err
	}

	// Read the public key file
	file, err := os.Open(keyPath + "/" + id + "_public")
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	buf := make([]byte, info.Size())
	file.Read(buf)
	//pem decoding
	block, _ := pem.Decode(buf)

	//x509
	publicStream, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// Interface converted to public key
	puba := publicStream.(*ecdsa.PublicKey)

	//1, open the private key file and read the content
	file, err = os.Open(keyPath + "/" + Utility.MyMacAddr() + "_private")
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	info, err = file.Stat()
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	buf = make([]byte, info.Size())
	file.Read(buf)

	//2, pem decryption
	block, _ = pem.Decode(buf)

	//x509 decryption
	privb, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	a, _ := puba.Curve.ScalarMult(puba.X, puba.Y, privb.D.Bytes())

	fmt.Println("key is ", a.String())
	// The same value will be generated other peers...
	return []byte(a.String()), nil
}

/**
 * The key must be formated as pem.
 */
func SetPeerPublicKey(id, encPub string) error {
	id = strings.ReplaceAll(id, ":", "_")
	fmt.Println("save file ", keyPath+"/"+id+"_public")
	err := ioutil.WriteFile(keyPath+"/"+id+"_public", []byte(encPub), 0644)
	if err != nil {
		return err
	}

	return nil
}
