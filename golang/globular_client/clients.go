// ==============================================
// clients.go (updated to be etcd-first, runtime-aware)
// ==============================================
package globular_client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

var (
	tokensPath = config.GetConfigDir() + "/tokens"
	clients    *sync.Map
)

// GetClient is a factory that returns (and memoizes) a client for a given
// service name at an address. fct must be the registered constructor name used
// by Utility.CallFunction (e.g., "NewFileService_Client").
// It resolves peer domains to reachable host:port when necessary.
func GetClient(address, name, fct string) (Client, error) {
	localAddress, _ := config.GetAddress()
	if localAddress != address {
		// Resolve peer address to a concrete host:port if the domain matches a peer.
		localConfig, _ := config.GetLocalConfig(true)
		if peers, ok := localConfig["Peers"].([]interface{}); ok {
			for _, pi := range peers {
				if p, ok := pi.(map[string]interface{}); ok && p["Domain"].(string) == address {
					host := p["Hostname"].(string)
					if p["Domain"].(string) != "localhost" {
						host += "." + p["Domain"].(string)
					}
					address = host + ":" + Utility.ToString(p["Port"])
					break
				}
			}
		}
	}

	if clients == nil {
		clients = new(sync.Map)
	}

	id := Utility.GenerateUUID(name + ":" + address)
	//fmt.Println("----------------> GetClient: address", address, "service", name, "id", id)

	if existing, ok := clients.Load(id); ok {
		return existing.(Client), nil
	}

	results, err := Utility.CallFunction(fct, address, name)
	if err != nil {
		slog.Error("GetClient: constructor invocation failed",
			"function", fct, "address", address, "service", name, "err", err)
		return nil, err
	}

	if !results[1].IsNil() {
		err := results[1].Interface().(error)
		slog.Error("GetClient: constructor returned error",
			"function", fct, "address", address, "service", name, "err", err)
		return nil, err
	}

	client := results[0].Interface().(Client)
	clients.Store(id, client)
	slog.Debug("GetClient: client created", "service", name, "address", address)
	return client, nil
}

// Client defines the minimal interface required by helpers in this package.
type Client interface {
	// Address including the HTTP(S) config port (not the gRPC port).
	GetAddress() string
	GetDomain() string
	GetId() string
	GetMac() string
	GetName() string
	Close()

	// gRPC port (without host).
	SetPort(int)
	GetPort() int

	SetId(string)
	SetMac(string)
	SetName(string)

	GetState() string
	SetState(string)

	SetDomain(string)
	SetAddress(string)

	// TLS configuration
	HasTLS() bool
	GetCertFile() string
	GetKeyFile() string
	GetCaFile() string
	SetTLS(bool)
	SetCertFile(string)
	SetKeyFile(string)
	SetCaFile(string)

	// Connection lifecycle
	Reconnect() error

	// Invoke a fully-qualified gRPC method on this client.
	Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error)
}

// shallow merge: b overrides a
func merge(a, b map[string]interface{}) map[string]interface{} {
	if a == nil {
		a = map[string]interface{}{}
	}
	if b == nil {
		return a
	}
	for k, v := range b {
		a[k] = v
	}
	return a
}

// InitClient initializes client metadata and TLS settings from Globular
// configuration. The given address points to the HTTP(S) control port, not the
// gRPC port; id is the service configuration identifier.
func InitClient(client Client, address string, id string) error {
	if len(address) == 0 {
		return fmt.Errorf("InitClient: no address provided (id=%s)", id)
	}
	if len(id) == 0 {
		return fmt.Errorf("InitClient: no id provided (address=%s)", address)
	}

	var cfg map[string]interface{}
	var err error

	localAddr, _ := config.GetAddress()
	localCfg, _ := config.GetLocalConfig(true)

	// Normalize address to include control port.
	if !strings.Contains(address, ":") {
		if strings.HasPrefix(localAddr, address) {
			// Local
			if localCfg["Protocol"].(string) == "https" {
				address += ":" + Utility.ToString(localCfg["PortHTTPS"])
			} else {
				address += ":" + Utility.ToString(localCfg["PortHTTP"])
			}
		} else {
			// Try peers
			if ps, ok := localCfg["Peers"].([]interface{}); ok {
				for _, pi := range ps {
					p := pi.(map[string]interface{})
					if p["Domain"].(string) == address {
						address += ":" + Utility.ToString(p["Port"])
						break
					}
				}
			}
			// Fall back to local defaults if still no port.
			if !strings.Contains(address, ":") {
				if localCfg["Protocol"].(string) == "https" {
					address += ":" + Utility.ToString(localCfg["PortHTTPS"])
				} else {
					address += ":" + Utility.ToString(localCfg["PortHTTP"])
				}
			}
		}
	}

	parts := strings.Split(address, ":")
	domain := parts[0]
	port := Utility.ToInt(parts[1])
	isLocal := localAddr == address

	// Subject Alternative Name (SAN) details used when installing TLS certs.
	var sanCountry, sanState, sanCity, sanOrg string
	var sanAltDomains []interface{}

	var globuleCfg map[string]interface{}

	if isLocal {
		globuleCfg, _ = config.GetLocalConfig(true)
		cfg, err = config.GetServiceConfigurationById(id) // etcd (desired+runtime)
	} else {
		globuleCfg, err = config.GetRemoteConfig(domain, port)
		if err == nil {
			cfg, err = config.GetRemoteServiceConfig(domain, port, id)
		}
	}

	if err != nil || cfg == nil {
		slog.Error("InitClient: failed to fetch configuration",
			"id", id, "address", address, "local", isLocal, "err", err)
		return fmt.Errorf("InitClient: failed to fetch configuration id=%s from %s: %w", id, address, err)
	}

	// Extract SAN values
	if v, ok := globuleCfg["Country"].(string); ok {
		sanCountry = v
	}
	if v, ok := globuleCfg["State"].(string); ok {
		sanState = v
	}
	if v, ok := globuleCfg["City"].(string); ok {
		sanCity = v
	}
	if v, ok := globuleCfg["Organization"].(string); ok {
		sanOrg = v
	}
	if v, ok := globuleCfg["AlternateDomains"].([]interface{}); ok {
		sanAltDomains = v
	}

	// Persist the address on the client.
	client.SetAddress(address)

	// ---- Robustness: ensure runtime overlay & fallbacks before mandatory checks ----
	// (cfg from GetServiceConfigurationById already includes runtime when local; for remote,
	//  we only have the service doc from HTTP; be tolerant.)
	if isLocal {
		// Already etcd merged; nothing to do.
	} else {
		// Remote: no runtime overlay available via HTTP; leave cfg as-is.
	}

	// Derive Mac if missing and this is the local node
	if v, ok := cfg["Mac"].(string); !ok || v == "" {
		if isLocal {
			if m, derr := config.GetMacAddress(); derr == nil && m != "" {
				cfg["Mac"] = m
			}
		}
	}
	if _, ok := cfg["State"].(string); !ok {
		cfg["State"] = "starting"
	}

	// ---------------- Mandatory attributes ----------------
	if v, ok := cfg["Id"].(string); ok && v != "" {
		client.SetId(v)
	} else {
		return fmt.Errorf("InitClient: missing service Id for %s", id)
	}
	if v, ok := cfg["Domain"].(string); ok && v != "" {
		client.SetDomain(v)
	} else {
		return fmt.Errorf("InitClient: missing service Domain for %s", id)
	}
	if v, ok := cfg["Name"].(string); ok && v != "" {
		client.SetName(v)
	} else {
		return fmt.Errorf("InitClient: missing service Name for %s", id)
	}
	if v, ok := cfg["Mac"].(string); ok && v != "" {
		client.SetMac(v)
	} else {
		return fmt.Errorf("InitClient: missing service Mac for %s", id)
	}
	if v, ok := cfg["Port"]; ok {
		client.SetPort(Utility.ToInt(v))
	} else {
		return fmt.Errorf("InitClient: missing service Port for %s", id)
	}
	if v, ok := cfg["State"].(string); ok && v != "" {
		client.SetState(v)
	} else {
		return fmt.Errorf("InitClient: missing service State for %s", id)
	}

	// TLS setup
	if enabled, _ := cfg["TLS"].(bool); enabled {
		client.SetTLS(true)
		if isLocal {
			// Translate server cert/key to client cert/key paths locally.
			certFile := strings.Replace(cfg["CertFile"].(string), "server", "client", -1)
			keyFile := strings.Replace(cfg["KeyFile"].(string), "server", "client", -1)
			client.SetKeyFile(keyFile)
			client.SetCertFile(certFile)
			client.SetCaFile(cfg["CertAuthorityTrust"].(string))
		} else {
			// Ensure remote client certificates are installed.
			path := config.GetConfigDir() + "/tls/" + domain
			keyFile, certFile, caFile, err := security.InstallClientCertificates(
				domain, port, path, sanCountry, sanState, sanCity, sanOrg, sanAltDomains,
			)
			if err != nil {
				slog.Error("InitClient: InstallClientCertificates failed",
					"domain", domain, "port", port, "err", err)
				return err
			}
			client.SetKeyFile(keyFile)
			client.SetCertFile(certFile)
			client.SetCaFile(caFile)
		}
	} else {
		client.SetTLS(false)
	}

	slog.Debug("InitClient: client initialized",
		"id", client.GetId(), "name", client.GetName(), "domain", client.GetDomain(),
		"grpc_port", client.GetPort(), "tls", client.HasTLS(), "address", client.GetAddress())
	return nil
}

// clientInterceptor attempts to transparently reinitialize/reconnect the client
// when transient connection errors occur, then retries the call.
func clientInterceptor(client_ Client) func(
	ctx context.Context,
	method string,
	rqst interface{},
	reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	return func(ctx context.Context,
		method string,
		rqst interface{},
		reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {

		err := invoker(ctx, method, rqst, reply, cc, opts...)
		if client_ != nil && err != nil {
			msg := err.Error()
			retriable := strings.HasPrefix(msg, `rpc error: code = Unavailable desc = connection error: desc = "transport: Error while dialing dial tcp`) ||
				strings.HasPrefix(msg, `rpc error: code = Unimplemented desc = unknown service`)
			if retriable {
				slog.Warn("clientInterceptor: reconnecting after error",
					"method", method, "service", client_.GetName(), "id", client_.GetId(), "err", err)

				if initErr := InitClient(client_, client_.GetAddress(), client_.GetId()); initErr == nil {
					const maxTries = 10
					for i := 0; i < maxTries; i++ {
						if recErr := client_.Reconnect(); recErr != nil {
							time.Sleep(1 * time.Second)
							continue
						}
						// Retry once reconnected
						return invoker(ctx, method, rqst, reply, cc, opts...)
					}
				} else {
					slog.Error("clientInterceptor: reinit failed",
						"service", client_.GetName(), "id", client_.GetId(), "err", initErr)
					debug.PrintStack()
				}
			}
		}
		return err
	}
}

// GetClientTlsConfig builds a tls.Config from the client's certificate files.
func GetClientTlsConfig(client Client) (*tls.Config, error) {
	certFile := client.GetCertFile()
	if certFile == "" {
		err := errors.New("GetClientTlsConfig: missing client certificate file")
		return nil, err
	}
	keyFile := client.GetKeyFile()
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("GetClientTlsConfig: load key pair failed (cert=%s, key=%s): %w", certFile, keyFile, err)
	}

	certPool := x509.NewCertPool()
	caPem, err := os.ReadFile(client.GetCaFile())
	if err != nil {
		return nil, fmt.Errorf("GetClientTlsConfig: read CA file failed: %w", err)
	}
	if ok := certPool.AppendCertsFromPEM(caPem); !ok {
		return nil, errors.New("GetClientTlsConfig: append CA certificate failed")
	}

	return &tls.Config{
		ServerName:   strings.Split(client.GetAddress(), ":")[0], // Required for TLS SNI.
		Certificates: []tls.Certificate{certificate},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		RootCAs:      certPool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// GetClientConnection dials the gRPC endpoint for a client, using TLS if enabled.
func GetClientConnection(client Client) (*grpc.ClientConn, error) {
	address := client.GetAddress()
	if strings.Contains(address, ":") {
		address = strings.Split(address, ":")[0]
	}
	target := address + ":" + Utility.ToString(client.GetPort())

	var (
		cc  *grpc.ClientConn
		err error
	)

	if client.HasTLS() {
		tcfg, err := GetClientTlsConfig(client)
		if err != nil {
			slog.Error("GetClientConnection: TLS config error", "target", target, "err", err)
			return nil, err
		}
		cc, err = grpc.Dial(target,
			grpc.WithTransportCredentials(credentials.NewTLS(tcfg)),
			grpc.WithUnaryInterceptor(clientInterceptor(client)),
		)
		if err != nil {
			slog.Error("GetClientConnection: TLS dial failed", "target", target, "err", err)
			return nil, err
		}
	} else {
		cc, err = grpc.Dial(target,
			grpc.WithInsecure(),
			grpc.WithUnaryInterceptor(clientInterceptor(client)),
		)
		if err != nil {
			slog.Error("GetClientConnection: dial failed", "target", target, "err", err)
			return nil, err
		}
	}

	slog.Debug("GetClientConnection: connected", "target", target, "tls", client.HasTLS())
	return cc, nil
}

// GetClientContext returns a context with outbound metadata (token, domain, mac)
// for the given client. If a local token is available, it is attached.
func GetClientContext(client Client) context.Context {
	if err := Utility.CreateDirIfNotExist(tokensPath); err != nil {
		slog.Warn("GetClientContext: creating token dir failed", "path", tokensPath, "err", err)
	}

	token, err := security.GetLocalToken(client.GetMac())
	address := client.GetAddress()
	if strings.Contains(address, ":") {
		address = strings.Split(address, ":")[0]
	}

	var md metadata.MD
	if err == nil {
		md = metadata.New(map[string]string{
			"token":  string(token),
			"domain": address,
			"mac":    client.GetMac(),
		})
	} else {
		// No token; still include domain/mac to aid auditing and routing.
		md = metadata.New(map[string]string{
			"token":  "",
			"domain": address,
			"mac":    client.GetMac(),
		})
	}
	return metadata.NewOutgoingContext(context.Background(), md)
}

// InvokeClientRequest invokes a gRPC method by name on a concrete client stub
// using reflection via Utility.CallMethod. method should be the fully-qualified
// RPC method; only the final segment (after the last "/") is used here.
func InvokeClientRequest(client interface{}, ctx context.Context, method string, rqst interface{}) (interface{}, error) {
	methodName := method[strings.LastIndex(method, "/")+1:]

	reply, callErr := Utility.CallMethod(client, methodName, []interface{}{ctx, rqst})
	if callErr != nil {
		// Utility.CallMethod may return either error or string.
		if reflect.TypeOf(callErr).Kind() == reflect.String {
			return nil, errors.New(callErr.(string))
		}
		return nil, callErr.(error)
	}

	return reply, nil
}
