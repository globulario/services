package config_client

import (
	"context"
	"encoding/json"
	"strings"

	// "github.com/davecourtois/Utility"
	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/config/configpb"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// config Client Service
////////////////////////////////////////////////////////////////////////////////

type Config_Client struct {
	cc *grpc.ClientConn
	c  configpb.ConfigServiceClient

	// The id of the service
	id string

	// The name of the service
	name string

	// The client domain
	domain string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

	// The mac address of the server
	mac string

	// The port
	port int

	// is the connection is secure?
	hasTLS bool

	// Link to client key file
	keyFile string

	// Link to client certificate file.
	certFile string

	// certificate authority file
	caFile string

	// The client context
	ctx context.Context
}

// Create a connection to the service.
func NewConfigService_Client(address string, id string) (*Config_Client, error) {
	client := new(Config_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = configpb.NewConfigServiceClient(client.cc)

	return client, nil
}

// The address where the client can connect.
func (client *Config_Client) SetAddress(address string) {
	client.address = address
}

// Return the configuration from the configuration server.
func (client *Config_Client) GetConfiguration(address, id string) (map[string]interface{}, error) {
	// If the configuration is on the client domain...
	if !strings.HasPrefix(strings.ToLower(address), strings.ToLower(client.GetDomain())) {
		client_, err := NewConfigService_Client(address, id)
		if err != nil {
			return nil, err
		}
		return client_.GetServiceConfiguration(id)
	}

	// This is the only client that must be initialyse directlty from the configuration file...
	return config.GetServiceConfigurationById(id)
}

func (client *Config_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Config_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}
	token, err := security.GetLocalToken(client.GetMac())
	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": client.domain, "mac": client.GetMac()})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	return client.ctx
}

// Return the domain
func (client *Config_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Config_Client) GetAddress() string {
	return client.address
}

// Return the id of the service instance
func (client *Config_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Config_Client) GetName() string {
	return client.name
}

func (client *Config_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Config_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Config_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *Config_Client) GetPort() int {
	return client.port
}

// Set the client instance id.
func (client *Config_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Config_Client) SetName(name string) {
	client.name = name
}

func (client *Config_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Config_Client) SetDomain(domain string) {
	client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Config_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Config_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Config_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Config_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Config_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Config_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Config_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Config_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

////////////////// Api //////////////////////
// Specific config client function here.

// Get a service by it id, path or name (first with the name in that case.)
func (client *Config_Client) GetServiceConfiguration(path string) (map[string]interface{}, error) {
	rqst := new(configpb.GetServiceConfigurationRequest)
	rqst.Path = path
	rsp, err := client.c.GetServiceConfiguration(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}
	config_ := make(map[string]interface{})
	json.Unmarshal([]byte(rsp.GetConfig()), &config_)
	return config_, nil
}

// Return list of services with a given name
func (client *Config_Client) GetServicesConfigurationsByName(name string) ([]map[string]interface{}, error) {
	rqst := new(configpb.GetServicesConfigurationsByNameRequest)
	rqst.Name = name
	rsp, err := client.c.GetServicesConfigurationsByName(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	configs := make([]map[string]interface{}, 0)
	for i := 0; i < len(rsp.Configs); i++ {
		config_ := make(map[string]interface{})
		json.Unmarshal([]byte(rsp.Configs[i]), &config_)
		configs = append(configs, config_)
	}

	return configs, nil
}

// Return list of all services configuration
func (client *Config_Client) GetServicesConfigurations() ([]map[string]interface{}, error) {
	rqst := new(configpb.GetServicesConfigurationsRequest)
	rsp, err := client.c.GetServicesConfigurations(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}
	configs := make([]map[string]interface{}, 0)
	for i := 0; i < len(rsp.Configs); i++ {
		config_ := make(map[string]interface{})
		json.Unmarshal([]byte(rsp.Configs[i]), &config_)
		configs = append(configs, config_)
	}
	return configs, nil
}

// Save a service configuration
func (client *Config_Client) SetServiceConfiguration(s map[string]interface{}) error {
	rqst := new(configpb.SetServiceConfigurationRequest)

	rqst.Config = Utility.ToString(s)
	_, err := client.c.SetServiceConfiguration(client.GetCtx(), rqst)

	return err
}

////////////////////////////////////////////////////////////////////////////////////
// Configuration can be access via the configuration service or if no result found
// via the file base access function.
////////////////////////////////////////////////////////////////////////////////////

// do not use it directly...
var config_client_ *Config_Client

// A singleton to the configuration client.
func getConfigClient() (*Config_Client, error) {
	if config_client_ != nil {
		return config_client_, nil
	}

	address, err := config.GetAddress()
	if err != nil {
		return nil, err
	}

	config_client_, err = NewConfigService_Client(address, "config.ConfigService")
	if err != nil {
		return nil, err
	}

	return config_client_, nil
}

/**
 * Return a service with a given configuration id.
 */
func GetServiceConfigurationById(id string) (map[string]interface{}, error) {
	client, err := getConfigClient()
	if err == nil {
		// If a configuration client exist I will use it...
		config, err := client.GetServiceConfiguration(id)
		if err == nil {
			//fmt.Println("309 ",  config["Name"], config["State"])
			return config, nil
		}

		//fmt.Println("fail to get configuration for service with id ", id)
	}
	// I will use the synchronize file version.
	return config.GetServiceConfigurationById(id)
}

/**
 * Return a services with a given configuration name
 */
func GetServicesConfigurationsByName(name string) ([]map[string]interface{}, error) {
	client, err := getConfigClient()
	if err == nil {
		// If a configuration client exist I will use it...
		configs, err := client.GetServicesConfigurationsByName(name)
		if err == nil {
			return configs, nil
		}
	}

	// I will use the synchronize file version.
	return config.GetServicesConfigurationsByName(name)
}

/**
 * Return the list of all services configurations
 */
func GetServicesConfigurations() ([]map[string]interface{}, error) {
	client, err := getConfigClient()
	if err == nil {
		// If a configuration client exist I will use it...
		configs, err := client.GetServicesConfigurations()
		if err == nil {
			return configs, nil
		}
	}

	// I will use the synchronize file version.
	return config.GetServicesConfigurations()
}

/**
 * Save a given service configuration.
 */
func SaveServiceConfiguration(s map[string]interface{}) error {
	client, err := getConfigClient()
	if err == nil {
		// If a configuration client exist I will use it...
		//fmt.Println("361 ---------> save config ", s["Name"], s["Process"], s["State"] )
		err = client.SetServiceConfiguration(s)
		if err == nil {
			return nil
		}
	}

	// fmt.Println("368 ---------> save config ", s["Name"], s["Process"], s["State"] )
	// I will use the synchronize file version.
	return config.SaveServiceConfiguration(s)
}
