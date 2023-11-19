package globular_client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"runtime"
	"runtime/debug"
	"time"

	//"log"
	"reflect"
	"strings"
	"sync"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

var (
	tokensPath = config.GetConfigDir() + "/tokens"
	clients    *sync.Map
)

// Factory method.
func GetClient(address, name, fct string) (Client, error) {

	// so here I will test if the domain is contain in peers...
	localAddress, _ := config.GetAddress()
	if localAddress != address {
		// Here I will test if the domain is contain in peers...
		localConfig, _ := config.GetLocalConfig( true)
		peers, _ := localConfig["Peers"].([]interface{})
		for i := 0; i < len(peers); i++ {
			p := peers[i].(map[string]interface{})
			if p["Domain"].(string) == address {
				if p["ExternalIpAddress"] == Utility.MyIP() {
					address = p["ExternalIpAddress"].(string)
				} else {
					address = p["LocalIpAddress"].(string)
				}
				break
			}
		}
	}

	if clients == nil {
		clients = new(sync.Map)
	}

	id := Utility.GenerateUUID(name + ":" + address)
	existing_client_, ok := clients.Load(id)
	if ok {
		return existing_client_.(Client), nil
	}

	results, err := Utility.CallFunction(fct, address, name)
	if err != nil {

		fmt.Println("fail to call function ", fct, "with params", address, name, "error:", err)

		b := make([]byte, 2048) // adjust buffer size to be larger than expected stack
		n := runtime.Stack(b, false)
		s := string(b[:n])
		fmt.Println(s)

		return nil, err
	}

	if !results[1].IsNil() {
		err := results[1].Interface().(error)
		fmt.Println("fail to call function ", fct, "with params", address, name, "error:", err)
		b := make([]byte, 2048) // adjust buffer size to be larger than expected stack
		n := runtime.Stack(b, false)
		s := string(b[:n])
		fmt.Println(s)
		return nil, err
	}

	client := results[0].Interface().(Client)
	clients.Store(id, client)

	return client, nil
}

// The client service interface.
type Client interface {

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

	// Return the state of the client at connection time.
	GetState() string

	SetState(string)

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

	// Connect or reconnect...
	Reconnect() error

	// Invoque a request on the client and return it grpc reply.
	Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error)
}

/**
 * Initialyse the client security and set it port to
 */
func InitClient(client Client, address string, id string) error {

	if len(address) == 0 {
		return errors.New("no address was given for client id " + id)
	}

	if len(id) == 0 {
		return errors.New("no id was given for client address " + address)
	}

	var config_ map[string]interface{}
	var err error

	// Here the address must contain the port where the configuration can be found on the
	// http server. If not given thas mean if it's local (on the same domain) I will retreive
	// it from the local configuration. Otherwize if it's remove the port 80 will be taken.
	address_, _ := config.GetAddress()
	localConfig, _ := config.GetLocalConfig(true)

	if !strings.Contains(address, ":") {
		if strings.HasPrefix(address_, address) {
			// this is local
			if localConfig["Protocol"].(string) == "https" {
				address += ":" + Utility.ToString(localConfig["PortHttps"])
			} else {
				address += ":" + Utility.ToString(localConfig["PortHttp"])
			}
		} else {

			// so here I will test if the domain is contain in peers...
			if localConfig["Peers"] != nil {
				peers := localConfig["Peers"].([]interface{})
				for i := 0; i < len(peers); i++ {
					p := peers[i].(map[string]interface{})
					if p["Domain"].(string) == address {
						address += ":" + Utility.ToString(p["Port"])
						break
					}
				}
			}

			if !strings.Contains(address, ":") {
				address += ":80"
			}
		}
	}

	values := strings.Split(address, ":")
	domain := values[0]
	port := Utility.ToInt(values[1])
	isLocal := address_ == address

	// San Certificate informations
	var san_country string
	var san_state string
	var san_city string
	var san_organization string
	san_alternateDomains := make([]interface{}, 0)

	if isLocal {

		// get san values from the globule itself...
		globule_config, _ := config.GetLocalConfig(true)
		if globule_config["Country"] != nil {
			san_country = globule_config["Country"].(string)
		}
		if globule_config["State"] != nil {
			san_state = globule_config["State"].(string)
		}
		if globule_config["City"] != nil {
			san_city = globule_config["City"].(string)
		}
		if globule_config["Organization"] != nil {
			san_organization = globule_config["Organization"].(string)
		}
		if globule_config["AlternateDomains"] != nil {
			san_alternateDomains = globule_config["AlternateDomains"].([]interface{})
		}

		// Local client configuration
		config_, err = config.GetServiceConfigurationById(id)
		
	} else {
		// so here I try to get more information from peers...
		var globule_config map[string]interface{}
		globule_config, err = config.GetRemoteConfig(domain, port)
		if err == nil {
			config_, err = config.GetRemoteServiceConfig(domain, port, id)
		}

		// set san values
		if globule_config["Country"] != nil {
			san_country = globule_config["Country"].(string)
		}
		if globule_config["State"] != nil {
			san_state = globule_config["State"].(string)
		}
		if globule_config["City"] != nil {
			san_city = globule_config["City"].(string)
		}
		if globule_config["Organization"] != nil {
			san_organization = globule_config["Organization"].(string)
		}
		if globule_config["AlternateDomains"] != nil {
			san_alternateDomains = globule_config["AlternateDomains"].([]interface{})
		}

	}

	// fmt.Println("try to retreive configuration", id, "at address ", address, " is local ", isLocal, " given local address is ", address_)
	if err != nil {
		fmt.Println("fail to initialyse client", id, "with error", err)
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
		client.SetName(config_["Name"].(string))
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

	if config_["State"] != nil {
		client.SetState(config_["State"].(string))
	} else {
		return errors.New("no state found for service " + id)
	}

	// Set security values.
	if config_["TLS"].(bool) {
		client.SetTLS(true)
		if isLocal {
			// Change server cert to client cert and do the same for key because we are at client side...
			certificateFile := strings.Replace(config_["CertFile"].(string), "server", "client", -1)
			keyFile := strings.Replace(config_["KeyFile"].(string), "server", "client", -1)
			client.SetKeyFile(keyFile)
			client.SetCertFile(certificateFile)
			client.SetCaFile(config_["CertAuthorityTrust"].(string))

		} else {

			// The address is not the local address so I want to get remote configuration value.
			// Here I will retreive the credential or create it if not exist.
			path := config.GetConfigDir() + "/tls/" + domain

			// install tls certificates if needed.
			keyFile, certificateFile, caFile, err := security.InstallCertificates(domain, port, path, san_country, san_state, san_city, san_organization, san_alternateDomains)
			if err != nil {
				return err
			}

			client.SetKeyFile(keyFile)
			client.SetCertFile(certificateFile)
			client.SetCaFile(caFile)
		}

	} else {
		client.SetTLS(false)
	}

	return nil
}

/**
 * That function is use to intercept all grpc client call for each client
 * if the connection is close a new connection will be made and the configuration
 * will be updated to set correct information.
 */
func clientInterceptor(client_ Client) func(
	ctx context.Context,
	method string,
	req interface{},
	reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	return func(ctx context.Context,
		method string,
		req interface{},
		reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {

		// Calls the invoker to execute RPC
		err := invoker(ctx, method, req, reply, cc, opts...)
		// Logic after invoking the invoker

		if client_ != nil && err != nil {
			if strings.HasPrefix(err.Error(), `rpc error: code = Unavailable desc = connection error: desc = "transport: Error while dialing dial tcp`) || strings.HasPrefix(err.Error(), `rpc error: code = Unimplemented desc = unknown service`) {

				// Here I will test if the process his the same...
				//fmt.Println("fail to connect to client ", client_.GetName()+":"+client_.GetId(), "at address", client_.GetAddress(), "with error", err)
				err := InitClient(client_, client_.GetAddress(), client_.GetId())
				if err == nil {
					nbTry := 10
					for i := 0; i < nbTry; i++ {
						err := client_.Reconnect()
						if err != nil {
							nbTry--
							time.Sleep(1 * time.Second)
						} else {
							return invoker(ctx, method, req, reply, cc, opts...)
						}
					}

				} else {

					fmt.Println("fail to initialyse client ", client_.GetName()+":"+client_.GetId(), err)
					debug.PrintStack()

				}

			}
		}
		return err
	}
}

/*
func withClientUnaryInterceptor() grpc.DialOption {
	return grpc.WithUnaryInterceptor(clientInterceptor)
}
*/

/**
 * Get the client connection. The token is require to control access to resource
 */
func GetClientConnection(client Client) (*grpc.ClientConn, error) {
	// initialyse the client values.
	var cc *grpc.ClientConn
	var err error

	address := client.GetAddress()
	if strings.Contains(address, ":") {
		address = strings.Split(address, ":")[0]
	}

	// The grpc address
	address += ":" + Utility.ToString(client.GetPort())

	//fmt.Println("get client connection ", address)
	if client.HasTLS() {
		//fmt.Println("client connection use tls")
		// Setup the login/pass simple test...
		if len(client.GetKeyFile()) == 0 {
			err := errors.New("no key file is available for client ")
			fmt.Println(err)
			return nil, err
		}

		certFile := client.GetCertFile()
		if len(certFile) == 0 {
			err = errors.New("no certificate file is available for client")
			fmt.Println(err)
			return nil, errors.New("no certificate file is available for client")
		}

		keyFile := client.GetKeyFile()

		certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}

		// Create a certificate pool from the certificate authority
		certPool := x509.NewCertPool()

		ca, err := ioutil.ReadFile(client.GetCaFile())
		if err != nil {
			err = errors.New("fail to read ca certificate")
			fmt.Println(err)
			return nil, err
		}

		// Append the certificates from the CA
		if ok := certPool.AppendCertsFromPEM(ca); !ok {
			err = errors.New("failed to append ca certs")
			fmt.Println(err)
			return nil, err
		}

		domain := address
		if strings.Contains(address, ":") {
			domain = strings.Split(address, ":")[0]
		}

		creds := credentials.NewTLS(&tls.Config{
			ServerName:   domain, // NOTE: this is required!
			Certificates: []tls.Certificate{certificate},
			ClientAuth:   tls.RequireAndVerifyClientCert,
			ClientCAs:    certPool,
			RootCAs:      certPool,
		})

		// Create a connection with the TLS credentials
		cc, err = grpc.Dial(address, grpc.WithTransportCredentials(creds), grpc.WithUnaryInterceptor(clientInterceptor(client)))
		if err != nil {
			fmt.Println("fail to dial address ", err)
			return nil, err
		} else if cc != nil {
			return cc, nil
		}
	} else {
		//fmt.Println("client connection not use tls")
		cc, err = grpc.Dial(address, grpc.WithInsecure(), grpc.WithUnaryInterceptor(clientInterceptor(client)))
		if err != nil {
			return nil, err
		} else if cc != nil {
			return cc, nil
		}
	}

	return nil, errors.New("fail to connect to grpc service")

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
		fmt.Println("fail to create token dir ", tokensPath)
	}

	// Get the last valid token if it exist
	token, err := security.GetLocalToken(client.GetMac())
	macAddress, _ := Utility.MyMacAddr(Utility.MyLocalIP())
	address := client.GetAddress()
	if strings.Contains(address, ":") {
		address = strings.Split(address, ":")[0]
	}

	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": address, "mac": macAddress})
		ctx = metadata.NewOutgoingContext(context.Background(), md)
		return ctx
	}

	md := metadata.New(map[string]string{"token": "", "domain": address, "mac": macAddress})
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
