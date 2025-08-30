package security

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	config_ "github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
)

var (
	Root       = config_.GetGlobularExecPath()
	ConfigPath = config_.GetConfigDir() + "/config.json"
	keyPath    = config_.GetConfigDir() + "/keys"
)

/**
 * Get the ca certificate
 */
func getCaCertificate(address string, port int) (string, error) {

	// if a DNS is I will use it as CA.
	local_config, err := config_.GetLocalConfig(true)
	if err == nil && local_config != nil {
		// I will use the DNS as authority for the certificate.
		if local_config["DNS"] != nil {
			if len(local_config["DNS"].(string)) > 0 {
				address = local_config["DNS"].(string)
				port = 443
				if strings.Contains(address, ":") {
					port = Utility.ToInt(strings.Split(address, ":")[1])
					address = strings.Split(address, ":")[0]
				}
			}
		}
	}

	// try with http
	certificate, err := getCaCertificate_(address, port, "http")
	if err == nil {
		return certificate, nil
	}

	// try https
	certificate, err = getCaCertificate_(address, port, "https")
	if err == nil {
		return certificate, nil
	}

	return "", err
}

/**
 * Get the ca certificate
 */
func getCaCertificate_(address string, port int, protocol string) (string, error) {

	if len(address) == 0 {
		return "", errors.New("fail to get CA certificate no address was given")
	}

	// Here I will get the configuration information from http...
	var resp *http.Response
	var err error

	// I will firt try with http protocol...
	var caAddress = protocol + "://" + address + ":" + Utility.ToString(port) + "/get_ca_certificate"
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

	// if a DNS is defined CA.
	local_config, err := config_.GetLocalConfig(true)
	if err == nil && local_config != nil {
		if local_config["DNS"] != nil {
			if len(local_config["DNS"].(string)) > 0 {
				// I will use the DNS as authority for the certificate.
				address = local_config["DNS"].(string)
				port = 443
				if strings.Contains(address, ":") {
					port = Utility.ToInt(strings.Split(address, ":")[1])
					address = strings.Split(address, ":")[0]
				}
			}
		}
	}

	certificate, err := signCaCertificate_(address, csr, port, "http")
	if err == nil {
		return certificate, nil
	}

	certificate, err = signCaCertificate_(address, csr, port, "https")
	if err == nil {
		return certificate, nil
	}

	return "", err
}

func signCaCertificate_(address string, csr string, port int, protocol string) (string, error) {

	// try to sign the certificate with http
	if len(address) == 0 {
		return "", errors.New("fail to sign certificate no address was given")
	}

	csr_str := base64.StdEncoding.EncodeToString([]byte(csr))
	// Here I will get the configuration information from http...
	var resp *http.Response
	var err error
	var signCertificateAddress = protocol + "://" + address + ":" + Utility.ToString(port) + "/sign_ca_certificate"
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

// ////////////////////////////// Certificate Authority /////////////////////////
func InstallClientCertificates(domain string, port int, path string, country string, state string, city string, organization string, alternateDomains []interface{}) (string, string, string, error) {
	return getClientCredentialConfig(path, domain, country, state, city, organization, alternateDomains, port)
}

func InstallServerCertificates(domain string, port int, path string, country string, state string, city string, organization string, alternateDomains []interface{}) (string, string, string, error) {
	return getServerCredentialConfig(path, domain, country, state, city, organization, alternateDomains, port)
}

/**
 * Return the client credential configuration.
 */
func getClientCredentialConfig(path string, domain string, country string, state string, city string, organization string, alternateDomains []interface{}, port int) (keyPath string, certPath string, caPath string, err error) {

	// keep it as address for now.
	address := domain

	// TODO Clarify the use of the password here.
	pwd := "1111"

	err = Utility.CreateDirIfNotExist(path)
	if err != nil {
		return "", "", "", err
	}

	alternateDomains_ := make([]string, 0)
	for i := 0; i < len(alternateDomains); i++ {
		alternateDomains_ = append(alternateDomains_, alternateDomains[i].(string))
	}

	for i := 0; i < len(alternateDomains_); i++ {
		if strings.Contains(alternateDomains_[i], "*") {
			wildcard := alternateDomains_[i]
			if strings.HasSuffix(domain, wildcard[2:]) {
				domain = wildcard[2:] // trim the first part of CN...
			}
		}
	}

	// I will connect to the certificate authority of the server where the application must
	// be deployed. Certificate autority run wihtout tls.

	// Get the ca.crt certificate from the server.
	ca_crt, err := getCaCertificate(address, port)
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
	err = os.WriteFile(path+"/ca.crt", []byte(ca_crt), 0444)
	if err != nil {
		return "", "", "", err
	}

	// Now I will generate the certificate for the client...
	// Step 1: Generate client private key.
	err = GenerateClientPrivateKey(path, pwd)
	if err != nil {
		return "", "", "", err
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
	client_csr, err := os.ReadFile(path + "/client.csr")
	if err != nil {
		return "", "", "", err
	}

	// Sign the certificate from the server ca...
	client_crt, err := signCaCertificate(address, string(client_csr), Utility.ToInt(port))
	if err != nil {
		return "", "", "", err
	}

	// Write bact the client certificate in file on the disk
	err = os.WriteFile(path+"/client.crt", []byte(client_crt), 0444)
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

/**
 * Return the server credential configuration.
 */
func getServerCredentialConfig(path string, domain string, country string, state string, city string, organization string, alternateDomains []interface{}, port int) (keyPath string, certPath string, caPath string, err error) {

	// TODO Clarify the use of the password here.
	pwd := "1111"

	err = Utility.CreateDirIfNotExist(path)
	if err != nil {
		return "", "", "", err
	}

	alternateDomains_ := make([]string, 0)
	for i := 0; i < len(alternateDomains); i++ {
		alternateDomains_ = append(alternateDomains_, alternateDomains[i].(string))
	}

	for i := 0; i < len(alternateDomains_); i++ {
		if strings.Contains(alternateDomains_[i], "*") {
			wildcard := alternateDomains_[i]
			if strings.HasSuffix(domain, wildcard[2:]) {
				domain = wildcard[2:] // trim the first part of CN...
			}
		}
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
		Utility.Exists(path+"/server.pem") &&
		Utility.Exists(path+"/server.crt") &&
		Utility.Exists(path+"/ca.crt") {

		local_ca_crt_checksum := Utility.CreateFileChecksum(path + "/ca.crt")
		remote_ca_crt_checksum := Utility.CreateDataChecksum([]byte(ca_crt))

		if local_ca_crt_checksum != remote_ca_crt_checksum {
			// Remove local and recreate new certificate...
			os.RemoveAll(path)
			err = Utility.CreateDirIfNotExist(path)
			if err != nil {
				log.Println(err)
				return "", "", "", err
			}
		} else {

			keyPath = path + "/server.pem"
			certPath = path + "/server.crt"
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
	err = GenerateSeverPrivateKey(path, pwd)
	if err != nil {
		return "", "", "", err
	}

	// generate the SAN file
	err = GenerateSanConfig(domain, path, country, state, city, organization, alternateDomains_)
	if err != nil {
		return "", "", "", err
	}

	// Step 2: Generate the server signing request.
	err = GenerateServerCertificateSigningRequest(path, pwd, domain)
	if err != nil {
		return "", "", "", err
	}

	// Step 3: Generate server signed certificate.
	csr, err := os.ReadFile(path + "/server.csr")
	if err != nil {
		return "", "", "", err
	}

	// Sign the certificate from the server ca...
	crt, err := signCaCertificate(domain, string(csr), Utility.ToInt(port))
	if err != nil {
		return "", "", "", err
	}

	// Write bact the client certificate in file on the disk
	err = os.WriteFile(path+"/server.crt", []byte(crt), 0444)
	if err != nil {
		return "", "", "", err
	}

	// Now ask the ca to sign the certificate.

	// Step 4: Convert to pem format.
	err = KeyToPem("server", path, pwd)
	if err != nil {
		return "", "", "", err
	}

	// set the credential paths.
	keyPath = path + "/server.pem"
	certPath = path + "/server.crt"
	caPath = path + "/ca.crt"

	// Remove the server.csr file.
	fmt.Println("Certificate was succefully install for ", domain)

	return
}

/////////////////////// Server Keys //////////////////////////////////////////

/**
 * That function is use to generate services certificates.
 * Private ca.key, server.key, server.pem, server.crt
 * Share ca.crt (needed by the client), server.csr (needed by the CA)
 */
func GenerateServicesCertificates(pwd string, expiration_delay int, domain string, path string, country string, state string, city string, organization string, alternateDomains []interface{}) error {

	if Utility.Exists(path + "/client.crt") {
		return nil // certificate are already created.
	}

	fmt.Println("Generate Services Certificates for ", domain, alternateDomains)
	alternateDomains_ := make([]string, 0)
	for i := 0; i < len(alternateDomains); i++ {
		alternateDomains_ = append(alternateDomains_, alternateDomains[i].(string))
	}

	for i := 0; i < len(alternateDomains_); i++ {
		if strings.HasPrefix(alternateDomains_[i], "*.") {
			wildcard := alternateDomains_[i]
			if strings.HasSuffix(domain, wildcard[2:]) {
				domain = wildcard[2:] // trim the first part of CN...
			}
			if !Utility.Contains(alternateDomains_, wildcard[2:]) {
				alternateDomains = append(alternateDomains, wildcard[2:])
			}
		}
	}

	// First of all I will test if a DNS exist in the configuration file. If so I will use it to generate the certificate.
	local_config, err := config_.GetLocalConfig(true)
	if err == nil && local_config != nil {

		// I will use the DNS as authority for the certificate.
		if local_config["DNS"] != nil {
			if len(local_config["DNS"].(string)) > 0 {
				dns_address := local_config["DNS"].(string)
				port := 443
				if strings.Contains(dns_address, ":") {
					port = Utility.ToInt(strings.Split(dns_address, ":")[1])
					dns_address = strings.Split(dns_address, ":")[0]
				}

				// Be sure that the dns address is not the same as the domain.
				if dns_address != local_config["Name"].(string)+"."+local_config["Domain"].(string) {
					// Here I will generate the certificate for the server.
					_, _, _, err := getServerCredentialConfig(path, dns_address, country, state, city, organization, alternateDomains, port)
					if err != nil {
						return err
					}

					// Here I will generate the certificate for the client.
					_, _, _, err = getClientCredentialConfig(path, dns_address, country, state, city, organization, alternateDomains, port)
					if err != nil {
						return err
					}

					return nil
				}
			}

		}
	}

	// Generate the SAN configuration.
	err = GenerateSanConfig(domain, path, country, state, city, organization, alternateDomains_)
	if err != nil {
		log.Println(err)
		return err
	}

	/////////////////////////////////////////////////////////////
	// Generate the certificate authority.
	/////////////////////////////////////////////////////////////
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

	/////////////////////////////////////////////////////////////
	// Generate the server certificate.
	/////////////////////////////////////////////////////////////
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

	/////////////////////////////////////////////////////////////
	// Generate the client certificate.
	/////////////////////////////////////////////////////////////

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

// //////////////////////////////////////////////////////////////////////////////////
// Peer key generation. Diffie-Hellman
//
// https://www.youtube.com/watch?v=NmM9HA2MQGI&ab_channel=Computerphile
//
// //////////////////////////////////////////////////////////////////////////////////
func DeletePublicKey(id string) error {
	id = strings.ReplaceAll(id, ":", "_")
	if !Utility.Exists(keyPath + "/" + id + "_public") {
		fmt.Println("public key", keyPath+"/"+id+"_public dosen't exist!")
		return nil
	}

	fmt.Println("delete public key", keyPath+"/"+id+"_public")
	return os.Remove(keyPath + "/" + id + "_public")
}

/**
 * Generate keys and save it at given path.
 */
func GeneratePeerKeys(id string) error {
	if len(id) == 0 {
		return errors.New("no id was given to generate the key")
	}

	id = strings.ReplaceAll(id, ":", "_")
	var privateKey *ecdsa.PrivateKey

	// The error
	var err error

	if !Utility.Exists(keyPath + "/" + id + "_private") {

		// Use ecdsa to generate a key pair
		privateKey, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
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
		defer file.Close()

		err = pem.Encode(file, &block)
		if err != nil {
			return err
		}
	} else {
		privateKey, err = readPrivateKey(id)
		if err != nil {
			return err
		}
	}

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

	file, err := os.Create(keyPath + "/" + id + "_public")
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

	macAddress, err := config_.GetMacAddress()
	if err != nil {
		return nil, err
	}

	// In that case the public key will be use as a token key...
	// That token will be valid on the peer itself.
	id := strings.ReplaceAll(macAddress, ":", "_")
	if !Utility.Exists(keyPath + "/" + id + "_public") {
		fmt.Println("no public key found at path ", keyPath+"/"+id+"_public")
		return nil, errors.New("no public key found at path " + keyPath + "/" + id + "_public")
	}

	localKey, err = ioutil.ReadFile(keyPath + "/" + id + "_public")

	return localKey, err
}

/**
 * Read the private key.
 */
func readPrivateKey(id string) (*ecdsa.PrivateKey, error) {
	id = strings.ReplaceAll(id, ":", "_")

	//1, open the private key file and read the content
	file_private, err := os.Open(keyPath + "/" + id + "_private")
	if err != nil {
		return nil, err
	}

	defer file_private.Close()

	info, err := file_private.Stat()
	if err != nil {
		return nil, err
	}

	buf := make([]byte, info.Size())
	file_private.Read(buf)

	//2, pem decryption
	block, _ := pem.Decode(buf)
	if block == nil {
		fmt.Println("delete private key ", keyPath+"/"+id+"_private")
		os.Remove(keyPath + "/" + id + "_private")
		return nil, errors.New("Corrupted local keys was found for peer " + id + " key's was deleted. You must reconnect all your peer's to be able to connect with them.")
	}

	//x509 decryption
	return x509.ParseECPrivateKey(block.Bytes)
}

func readPublicKey(id string) (*ecdsa.PublicKey, error) {
	id = strings.ReplaceAll(id, ":", "_")

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
	if block == nil {
		os.Remove(keyPath + "/" + id + "_public")
		fmt.Println("delete public key ", keyPath+"/"+id+"_public")
		return nil, errors.New("Corrupted local keys was found for peer " + id + " key's was deleted. You must reconnect all your peer's to be able to connect with them.")
	}

	//x509
	publicStream, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	// Interface converted to public key
	puba := publicStream.(*ecdsa.PublicKey)

	return puba, nil
}

/**
 * Return a jwt token key for a given peer id (mac address)
 */
func GetPeerKey(id string) ([]byte, error) {

	if len(id) == 0 {
		return nil, errors.New("no peer id was given to get key")
	}

	id = strings.ReplaceAll(id, ":", "_")

	macAddress, err := config_.GetMacAddress()
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

	puba, err := readPublicKey(id)
	if err != nil {
		return nil, err
	}

	privb, err := readPrivateKey(macAddress)
	if err != nil {
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



///////////////////////////////////////

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

func writePEM(path string, block *pem.Block, mode os.FileMode) error {
	return os.WriteFile(path, pem.EncodeToMemory(block), mode)
}

func readPEM(path string) (*pem.Block, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("no PEM block in %s", path)
	}
	return block, nil
}

func genECDSAKeyPKCS8() (crypto.Signer, []byte, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	return priv, pkcs8, nil
}

func parseAnyPrivateKey(block *pem.Block) (crypto.Signer, error) {
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if s, ok := k.(crypto.Signer); ok {
			return s, nil
		}
	}
	if k, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	return nil, errors.New("unsupported private key format")
}

func parseSANsFromConf(path string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(path, "san.conf"))
	if err != nil {
		return nil, err
	}
	var sans []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "DNS.") {
			if i := strings.Index(line, "="); i > 0 {
				val := strings.TrimSpace(line[i+1:])
				if val != "" {
					sans = append(sans, val)
				}
			}
		}
	}
	return sans, nil
}

func serialNumber() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, limit)
}

// --- CA key/cert ---

func GenerateAuthorityPrivateKey(path string, _ string) error {
	if fileExists(path + "/ca.key") {
		return nil
	}
	_, pkcs8, err := genECDSAKeyPKCS8()
	if err != nil {
		return err
	}
	return writePEM(path+"/ca.key", &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}, 0o400)
}

func GenerateAuthorityTrustCertificate(path string, _ string, expiration_delay int, domain string) error {
	if fileExists(path + "/ca.crt") {
		return nil
	}
	b, err := readPEM(path + "/ca.key")
	if err != nil {
		return err
	}
	caSigner, err := parseAnyPrivateKey(b)
	if err != nil {
		return err
	}
	subj := pkix.Name{CommonName: domain + " Root CA"}
	serial, _ := serialNumber()
	now := time.Now()
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      subj,
		NotBefore:    now.Add(-5 * time.Minute),
		NotAfter:     now.Add(time.Duration(expiration_delay) * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:         true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, caSigner.Public(), caSigner)
	if err != nil {
		return err
	}
	return writePEM(path+"/ca.crt", &pem.Block{Type: "CERTIFICATE", Bytes: der}, 0o444)
}

// --- Server/Client keys ---

func GenerateSeverPrivateKey(path string, _ string) error {
	if fileExists(path + "/server.key") {
		return nil
	}
	_, pkcs8, err := genECDSAKeyPKCS8()
	if err != nil {
		return err
	}
	return writePEM(path+"/server.key", &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}, 0o400)
}

func GenerateClientPrivateKey(path string, _ string) error {
	if fileExists(path + "/client.key") {
		return nil
	}
	_, pkcs8, err := genECDSAKeyPKCS8()
	if err != nil {
		return err
	}
	return writePEM(path+"/client.key", &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}, 0o400)
}

// --- SAN Config ---

func GenerateSanConfig(domain, path, country, state, city, organization string, alternateDomains []string) error {
	if fileExists(path + "/san.conf") {
		return nil
	}
	cfg := fmt.Sprintf(`
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
C  = %s
ST = %s
L  = %s
O  = %s
CN = %s

[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names

[alt_names]
`, country, state, city, organization, domain)

	for i, d := range append(alternateDomains, domain) {
		cfg += fmt.Sprintf("DNS.%d = %s\n", i, d)
	}
	return os.WriteFile(path+"/san.conf", []byte(cfg), 0o644)
}

// --- CSRs ---

func GenerateClientCertificateSigningRequest(path string, _ string, domain string) error {
	if fileExists(path + "/client.csr") {
		return nil
	}
	keyBlock, err := readPEM(path + "/client.key")
	if err != nil {
		return err
	}
	signer, err := parseAnyPrivateKey(keyBlock)
	if err != nil {
		return err
	}
	sans, _ := parseSANsFromConf(path)
	tpl := &x509.CertificateRequest{Subject: pkix.Name{CommonName: domain}, DNSNames: sans}
	der, err := x509.CreateCertificateRequest(rand.Reader, tpl, signer)
	if err != nil {
		return err
	}
	return writePEM(path+"/client.csr", &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}, 0o444)
}

func GenerateServerCertificateSigningRequest(path string, _ string, domain string) error {
	if fileExists(path + "/server.csr") {
		return nil
	}
	keyBlock, err := readPEM(path + "/server.key")
	if err != nil {
		return err
	}
	signer, err := parseAnyPrivateKey(keyBlock)
	if err != nil {
		return err
	}
	sans, _ := parseSANsFromConf(path)
	tpl := &x509.CertificateRequest{Subject: pkix.Name{CommonName: domain}, DNSNames: sans}
	der, err := x509.CreateCertificateRequest(rand.Reader, tpl, signer)
	if err != nil {
		return err
	}
	return writePEM(path+"/server.csr", &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}, 0o444)
}

// --- CA-signed leaf certs ---

func signCSRWithCA(csrPath, caCrtPath, caKeyPath, outPath string, days int, isServer bool) error {
	caBlock, err := readPEM(caCrtPath)
	if err != nil {
		return err
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return err
	}
	keyBlock, _ := readPEM(caKeyPath)
	caSigner, _ := parseAnyPrivateKey(keyBlock)
	csrBlock, _ := readPEM(csrPath)
	csr, _ := x509.ParseCertificateRequest(csrBlock.Bytes)

	now := time.Now()
	serial, _ := serialNumber()
	ext := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	if isServer {
		ext = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	}
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      csr.Subject,
		NotBefore:    now,
		NotAfter:     now.Add(time.Duration(days) * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  ext,
		DNSNames:     csr.DNSNames,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, caCert, csr.PublicKey, caSigner)
	if err != nil {
		return err
	}
	return writePEM(outPath, &pem.Block{Type: "CERTIFICATE", Bytes: der}, 0o444)
}

func GenerateSignedClientCertificate(path string, _ string, expiration_delay int) error {
	if fileExists(path + "/client.crt") {
		return nil
	}
	return signCSRWithCA(path+"/client.csr", path+"/ca.crt", path+"/ca.key", path+"/client.crt", expiration_delay, false)
}

func GenerateSignedServerCertificate(path string, _ string, expiration_delay int) error {
	if fileExists(path + "/server.crt") {
		return nil
	}
	return signCSRWithCA(path+"/server.csr", path+"/ca.crt", path+"/ca.key", path+"/server.crt", expiration_delay, true)
}

// --- PEM conversion (compat) ---

func KeyToPem(name string, path string, _ string) error {
	pemPath := filepath.Join(path, name+".pem")
	if fileExists(pemPath) {
		return nil
	}
	block, err := readPEM(filepath.Join(path, name+".key"))
	if err != nil {
		return err
	}
	signer, err := parseAnyPrivateKey(block)
	if err != nil {
		return err
	}
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(signer)
	return writePEM(pemPath, &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}, 0o400)
}

// --- Validation ---

func ValidateCertificateExpiration(certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}
	cert_, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return err
	}
	if time.Now().After(cert_.NotAfter) {
		return errors.New("the certificate is expired " + cert_.NotAfter.Local().String())
	}
	return nil
}

