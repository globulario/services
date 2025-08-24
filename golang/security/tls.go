package security

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
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
	"time"

	config_ "github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
)

var (
	Root       = config_.GetGlobularExecPath()
	ConfigPath = config_.GetConfigDir() + "/config.json"
	keyPath    = config_.GetConfigDir() + "/keys"
)

func runCmd(name string, args []string, wait chan bool) error {

	cmd := exec.Command(name, args...)
	cmd.Dir = os.TempDir()

	pid := -1

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	fmt.Println("run command: ", name, args)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output := make(chan string)
	done := make(chan bool)

	// Process message util the command is done.
	go func() {
		for {
			select {
			case <-done:
				fmt.Println(name, "is done")
				wait <- true // unblock it...
				break

			case result := <-output:
				if cmd.Process != nil {
					pid = cmd.Process.Pid
				}
				fmt.Println(name+":", pid, result)

			}
		}
	}()

	// Start reading the output
	go Utility.ReadOutput(output, stdout)
	err = cmd.Run()
	if err != nil {
		cmd_str := name
		for i := 0; i < len(args); i++ {
			cmd_str += " " + args[i]
		}
		return errors.New(cmd_str + " </br> " + fmt.Sprint(err) + ": " + stderr.String())
	}

	// Close the output.
	stdout.Close()
	done <- true

	return nil
}

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
	wait := make(chan bool)
	err := runCmd(cmd, args, wait)
	if err == nil {
		<-wait
	}

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

	wait := make(chan bool)
	err := runCmd(cmd, args, wait)
	if err == nil {
		<-wait
	}

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
	wait := make(chan bool)
	err := runCmd(cmd, args, wait)
	if err == nil {
		<-wait
	}

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
	wait := make(chan bool)
	err := runCmd(cmd, args, wait)
	if err == nil {
		<-wait
	}

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
	err = runCmd(cmd, args, wait)
	if err == nil {
		<-wait
	}

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

	wait := make(chan bool)
	err := runCmd(cmd, args, wait)
	if err == nil {
		<-wait
	}

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

	wait := make(chan bool)
	err := runCmd(cmd, args, wait)
	if err == nil {
		<-wait
	}

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

	// I will test if the domain is part of a wild card domain. if so I will change the domain to the wild card domain.
	for i := 0; i < len(alternateDomains); i++ {
		if strings.Contains(alternateDomains[i], "*") {
			if strings.HasSuffix(domain, alternateDomains[i][2:]) {
				domain = alternateDomains[i] // I will use the wild card domain.
			}
		}
	}

	// I will add the domain to the alternate domains.
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

	wait := make(chan bool)
	err := runCmd(cmd, args, wait)
	if err == nil {
		<-wait
	}

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

	wait := make(chan bool)
	err := runCmd(cmd, args, wait)
	if err == nil {
		<-wait
	}

	if err != nil || !Utility.Exists(path+"/server.crt") {
		if err == nil {
			err = errors.New("fail to get the signed server certificate" + path + "/server.key")
		}

		return err

	}

	return nil
}

// Conversion of srv.key into a format gRpc likes (this shouldn't be shared)
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

	wait := make(chan bool)
	err := runCmd(cmd, args, wait)
	if err == nil {
		<-wait
	}

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

/**
 * Load and return certificate file.
 */
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
