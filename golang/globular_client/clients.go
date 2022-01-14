package globular_client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"log"

	//"log"
	"reflect"
	"strings"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

var (
	tokensPath = config.GetConfigDir() + "/tokens"
)

// The client service interface.
type Client interface {

	// Return the configuration from the configuration server.
	GetConfiguration(address, id string) (map[string]interface{}, error)

	// Return the address with the port where configuration can be found...
	GetAddress() string

	// Get Domain return the client domain.
	GetDomain() string

	// Return the id of the service.
	GetId() string

	// Return the mac address of a client.
	GetMac() string

	// Return the name of the service.
	GetName() string

	// Close the client.
	Close()

	// Contain the grpc port number, to http port is contain the address
	SetPort(int)

	// Return the grpc port
	GetPort() int

	// Set the id of the client
	SetId(string)

	// Set the mac address
	SetMac(string)

	// Set the name of the client
	SetName(string)

	// Set the domain of the client
	SetDomain(string)

	// Set the address of the client
	SetAddress(string)

	////////////////// TLS ///////////////////

	//if the client is secure.
	HasTLS() bool

	// Get the TLS certificate file path
	GetCertFile() string

	// Get the TLS key file path
	GetKeyFile() string

	// Get the TLS key file path
	GetCaFile() string

	// Set the client is a secure client.
	SetTLS(bool)

	// Set TLS certificate file path
	SetCertFile(string)

	// Set TLS key file path
	SetKeyFile(string)

	// Set TLS authority trust certificate file path
	SetCaFile(string)

	// Invoque a request on the client and return it grpc reply.
	Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error)
}

/**
 * Initialyse the client security and set it port to
 */
func InitClient(client Client, address string, id string) error {

	var config_ map[string]interface{}
	var err error
	address_, _ := config.GetAddress()
	port := 80
	domain := address
	values := strings.Split(address, ":")
	if len(values) == 2 {
		domain = values[0]
		port = Utility.ToInt(values[1])
	}

	if address_ == address {
		// Local client configuration
		config_, err = client.GetConfiguration(address, id)
	} else {
		// Remote client configuration
		config_, err = config.GetRemoteConfig(domain, port)
	}

	if err != nil {
		return err
	}

	// Keep the client address...
	client.SetAddress(address)

	// Set client attributes.
	if config_["Id"] != nil {
		client.SetId(config_["Id"].(string))
	} else {
		return errors.New("no id found for service " + id)
	}

	if config_["Domain"] != nil {
		client.SetDomain(config_["Domain"].(string))
	} else {
		return errors.New("no domain found for service " + id)
	}

	if config_["Name"] != nil {
		client.SetId(config_["Name"].(string))
	} else {
		return errors.New("no name found for service " + id)
	}

	if config_["Mac"] != nil {
		client.SetMac(config_["Mac"].(string))
	} else {
		return errors.New("no mac address found for service " + id)
	}

	if config_["Port"] != nil {
		client.SetPort(Utility.ToInt(config_["Port"]))
	} else {
		return errors.New("no port found for service " + id)
	}

	// Set security values.
	if config_["TLS"].(bool) {
		address_, _ := config.GetAddress()
		if address_ == address {
			// Change server cert to client cert and do the same for key because we are at client side...
			certificateFile := strings.Replace(config_["CertFile"].(string), "server", "client", -1)
			keyFile := strings.Replace(config_["KeyFile"].(string), "server", "client", -1)
			client.SetKeyFile(keyFile)
			client.SetCertFile(certificateFile)
			client.SetCaFile(config_["CertAuthorityTrust"].(string))
			client.SetTLS(config_["TLS"].(bool))
		} else {

			// The address is not the local address so I want to get remote configuration value.
			// Here I will retreive the credential or create it if not exist.
			path := config.GetConfigDir() + "/tls/" + domain

			// install tls certificates if needed.
			keyFile, certificateFile, caFile, err := security.InstallCertificates(domain, port, path)
			if err != nil {
				return err
			}

			client.SetKeyFile(keyFile)
			client.SetCertFile(certificateFile)
			client.SetCaFile(caFile)
			client.SetTLS(config_["TLS"].(bool))

		}

	} else {
		client.SetTLS(false)
	}

	return nil
}

/**
 * Get the client connection. The token is require to control access to resource
 */
func GetClientConnection(client Client) (*grpc.ClientConn, error) {
	// initialyse the client values.
	var cc *grpc.ClientConn
	var err error
	if cc == nil {
		// The grpc address
		address := client.GetDomain() + ":" + Utility.ToString(client.GetPort())
		if client.HasTLS() {

			// Setup the login/pass simple test...
			if len(client.GetKeyFile()) == 0 {
				err := errors.New("no key file is available for client ")
				log.Println(err)
				return nil, err
			}

			certFile := client.GetCertFile()
			if len(certFile) == 0 {
				err = errors.New("no certificate file is available for client")
				log.Println(err)
				return nil, errors.New("no certificate file is available for client")
			}

			keyFile := client.GetKeyFile()

			certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return nil, err
			}
			log.Println("190 Get cert file")
			// Create a certificate pool from the certificate authority
			certPool := x509.NewCertPool()

			ca, err := ioutil.ReadFile(client.GetCaFile())
			if err != nil {
				err = errors.New("fail to read ca certificate")
				log.Println(err)
				return nil, err
			}

			// Append the certificates from the CA
			if ok := certPool.AppendCertsFromPEM(ca); !ok {
				err = errors.New("failed to append ca certs")
				log.Println(err)
				return nil, err
			}

			creds := credentials.NewTLS(&tls.Config{
				ServerName:   client.GetDomain(), // NOTE: this is required!
				Certificates: []tls.Certificate{certificate},
				ClientAuth:   tls.RequireAndVerifyClientCert,
				ClientCAs:    certPool,
				RootCAs:      certPool,
			})
			// Create a connection with the TLS credentials
			cc, err = grpc.Dial(address, grpc.WithTransportCredentials(creds))
			if err != nil {
				log.Println("fail to dial address ", err)
				return nil, err
			}
		} else {
			cc, err = grpc.Dial(address, grpc.WithInsecure())
			if err != nil {
				return nil, err
			}
		}
	}

	return cc, nil
}

/**
 * That function is use to get the client context. If a token is found in the
 * tmp directory for the client domain it's set in the metadata.
 */
func GetClientContext(client Client) context.Context {

	var ctx context.Context

	// if the address is local.
	err := Utility.CreateDirIfNotExist(tokensPath)
	if err != nil {
		log.Panicln("fail to create token dir ", tokensPath)
	}

	// Get the last valid token if it exist
	token, err := security.GetLocalToken(client.GetMac())
	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": client.GetAddress(), "mac": Utility.MyMacAddr()})
		ctx = metadata.NewOutgoingContext(context.Background(), md)
		return ctx
	}

	md := metadata.New(map[string]string{"token": "", "domain": client.GetAddress(), "mac": Utility.MyMacAddr()})
	ctx = metadata.NewOutgoingContext(context.Background(), md)

	return ctx

}

/**
 * Invoke a method on a client. The client is
 * ctx is the client request context.
 * method is the rpc method to run.
 * rqst is the request to run.
 */
func InvokeClientRequest(client interface{}, ctx context.Context, method string, rqst interface{}) (interface{}, error) {
	methodName := method[strings.LastIndex(method, "/")+1:]
	var err error
	reply, err_ := Utility.CallMethod(client, methodName, []interface{}{ctx, rqst})
	if err_ != nil {
		if reflect.TypeOf(err_).Kind() == reflect.String {
			err = errors.New(err_.(string))
		} else {
			err = err_.(error)
		}
	}

	return reply, err
}
