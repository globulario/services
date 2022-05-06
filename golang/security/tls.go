package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/davecourtois/Utility"
	config_ "github.com/globulario/services/golang/config"
)

var (
	Root       = config_.GetRootDir()
	ConfigPath = config_.GetConfigDir() + "/config.json"
	keyPath    = config_.GetConfigDir() + "/keys"
)

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

//////////////////////////////// Certificate Authority /////////////////////////
func InstallCertificates(domain string, port int, path string) (string, string, string, error) {
	return getCredentialConfig(path, domain, "", "", "", "", []interface{}{}, port)
}

/**
 * Return the credential configuration.
 */
func getCredentialConfig(path string, domain string, country string, state string, city string, organization string, alternateDomains []interface{}, port int) (keyPath string, certPath string, caPath string, err error) {

	log.Println("get credential config for domain: ", domain)
	// TODO Clarify the use of the password here.
	pwd := "1111"

	err = Utility.CreateDirIfNotExist(path)
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

	// Return the existing paths...
	if Utility.Exists(path) &&
		Utility.Exists(path+"/client.pem") &&
		Utility.Exists(path+"/client.crt") &&
		Utility.Exists(path+"/ca.crt") {

		local_ca_crt_checksum := Utility.CreateFileChecksum(path + "/ca.crt")
		remote_ca_crt_checksum := Utility.CreateDataChecksum([]byte(ca_crt))
		
		if local_ca_crt_checksum != remote_ca_crt_checksum {
			// Remove local and recreate new certificate...
			fmt.Println("Renew Certificates....")
			os.RemoveAll(path)
			err = Utility.CreateDirIfNotExist(path)
			if err != nil {
				log.Println(err)
				return "", "", "", err
			}
		} else {

			keyPath = path + "/client.pem"
			certPath = path + "/client.crt"
			caPath = path + "/ca.crt"
			return
		}
	}

	// Write the ca.crt file on the disk
	err = ioutil.WriteFile(path+"/ca.crt", []byte(ca_crt), 0444)
	if err != nil {
		return "", "", "", err
	}

	// Now I will generate the certificate for the client...
	// Step 1: Generate client private key.
	err = GenerateClientPrivateKey(path, pwd)
	if err != nil {
		return "", "", "", err
	}

	alternateDomains_ := make([]string, 0)
	for i := 0; i < len(alternateDomains); i++ {
		alternateDomains_ = append(alternateDomains_, alternateDomains[i].(string))
	}

	// generate the SAN file
	err = GenerateSanConfig(domain, path, country, state, city, organization, alternateDomains_)
	if err != nil {
		return "", "", "", err
	}

	// Step 2: Generate the client signing request.
	err = GenerateClientCertificateSigningRequest(path, pwd, domain)
	if err != nil {
		return "", "", "", err
	}

	// Step 3: Generate client signed certificate.
	client_csr, err := ioutil.ReadFile(path + "/client.csr")
	if err != nil {
		return "", "", "", err
	}

	// Sign the certificate from the server ca...
	client_crt, err := signCaCertificate(domain, string(client_csr), Utility.ToInt(port))
	if err != nil {
		return "", "", "", err
	}

	// Write bact the client certificate in file on the disk
	err = ioutil.WriteFile(path+"/client.crt", []byte(client_crt), 0444)
	if err != nil {
		return "", "", "", err
	}

	// Now ask the ca to sign the certificate.

	// Step 4: Convert to pem format.
	err = KeyToPem("client", path, pwd)
	if err != nil {
		return "", "", "", err
	}

	// set the credential paths.
	keyPath = path + "/client.pem"
	certPath = path + "/client.crt"
	caPath = path + "/ca.crt"
	fmt.Println("Certificate was succefully install for ", domain)
	return
}

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

// Certificate Authority trust certificate (this should be shared with users)
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
func DeletePublicKey(id string) error {
	_, err := os.ReadFile(keyPath + "/" + id + "_public")
	if err != nil {
		return err
	}

	return os.Remove(keyPath + "/" + id + "_public")
}

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

// Keep the local key in memory...
var (
	localKey = []byte{}
)

/**
 * Return the local jwt key
 */
func GetLocalKey() ([]byte, error) {
	if len(localKey) > 0 {
		return localKey, nil
	}

	macAddress, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return nil, err
	}

	// In that case the public key will be use as a token key...
	// That token will be valid on the peer itself.
	id := strings.ReplaceAll(macAddress, ":", "_")
	if !Utility.Exists(keyPath + "/" + id + "_public") {
		return nil, errors.New("no public key found at path " + keyPath + "/" + id + "_public")
	}

	localKey, err = ioutil.ReadFile(keyPath + "/" + id + "_public")

	return localKey, err
}

/**
 * Return a jwt token key for a given peer id (mac address)
 */
func GetPeerKey(id string) ([]byte, error) {

	if len(id) == 0 {
		return nil, errors.New("no peer id was given to get key")
	}

	id = strings.ReplaceAll(id, ":", "_")
	macAddress, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return nil, err
	}

	if id == strings.ReplaceAll(macAddress, ":", "_") {
		return GetLocalKey()
	}

	// If the token issuer is not the actual globule but another peer
	// I will use it public key and my private one to generate the correct key.
	err = Utility.CreateDirIfNotExist(keyPath)
	if err != nil {
		return nil, err
	}

	// Read the public key file
	file_public, err := os.Open(keyPath + "/" + id + "_public")

	if err != nil {
		return nil, err
	}

	defer file_public.Close()

	info, err := file_public.Stat()
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	buf := make([]byte, info.Size())
	file_public.Read(buf)
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
	macAddress, err = Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return nil, err
	}

	//1, open the private key file and read the content
	file_private, err := os.Open(keyPath + "/" + strings.ReplaceAll(macAddress, ":", "_") + "_private")
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	defer file_private.Close()

	info, err = file_private.Stat()
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	buf = make([]byte, info.Size())
	file_private.Read(buf)

	//2, pem decryption
	block, _ = pem.Decode(buf)

	//x509 decryption
	privb, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	a, _ := puba.Curve.ScalarMult(puba.X, puba.Y, privb.D.Bytes())

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
