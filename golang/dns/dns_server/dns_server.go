// Package main implements the DNS gRPC service used by Globular. It provides a
// storage-backed API to manage DNS records (A/AAAA/TXT/NS/MX/SOA/CNAME/URI/AFSDB/CAA)
// and a UDP DNS responder (via miekg/dns). All operational events are logged through
// the central log service so this service can serve as a reference for structured logs.
package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/dns/dns_client"
	"github.com/globulario/services/golang/dns/dnspb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/storage/storage_store"
	Utility "github.com/globulario/utility"
	"github.com/miekg/dns"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// Default network configuration. (gRPC and reverse proxy)
var (
	defaultPort  = 10033
	defaultProxy = 10034

	// By default all origins are allowed.
	allow_all_origins = true
	// Comma-separated values for explicitly allowed origins when AllowAllOrigins is false.
	allowed_origins string

	// Global pointer to the server implementation used by the UDP DNS handler.
	s *server
)

// server is the concrete implementation of the DNS service. It also implements
// Globular's service interface to be managed alongside other services.
type server struct {
	// Generic service metadata required by Globular.
	Id                 string
	Mac                string
	Name               string
	Path               string
	Proto              string
	Port               int
	Proxy              int
	AllowAllOrigins    bool
	AllowedOrigins     string
	Protocol           string
	Domain             string // Service domain (host) used by other services to reach us.
	Address            string // Full address (host[:port]) where this service can be reached.
	Description        string
	Keywords           []string
	Repositories       []string
	Discoveries        []string
	State              string
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string
	TLS                bool
	Version            string
	PublisherID        string
	KeepUpToDate       bool
	Checksum           string
	Plaform            string // Note: kept as-is for backward compatibility.
	KeepAlive          bool
	Permissions        []interface{}
	Dependencies       []string
	Process            int
	ProxyProcess       int
	ConfigPath         string
	LastError          string
	ModTime            int64
	Root               string // Root path for the local storage (badger).

	// The gRPC server instance.
	grpcServer *grpc.Server

	// DNS-specific configuration/state.
	DnsPort           int      // UDP port for the DNS server.
	Domains           []string // List of managed (authoritative) domains.
	ReplicationFactor int      // Reserved for storage backends that need it.

	// Persistent storage for records.
	store storage_store.Store

	connection_is_open bool
}

/* =========================
   Globular service methods
   ========================= */

// GetConfigurationPath returns the path where the service configuration is stored.
func (srv *server) GetConfigurationPath() string { return srv.ConfigPath }

// SetConfigurationPath sets the path where the service configuration is stored.
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

// GetAddress returns the HTTP address where the configuration can be fetched (/config).
func (srv *server) GetAddress() string { return srv.Address }

// SetAddress sets the HTTP address where the configuration can be fetched (/config).
func (srv *server) SetAddress(address string) { srv.Address = address }

// GetProcess returns the PID of the running process (or -1 if not running).
func (srv *server) GetProcess() int { return srv.Process }

// SetProcess sets the PID of the running process. If pid == -1, it closes the storage connection.
func (srv *server) SetProcess(pid int) {
	if pid == -1 {
		if srv.store != nil {
			_ = srv.store.Close()
		}
		srv.connection_is_open = false
	}
	srv.Process = pid
}

// GetProxyProcess returns the PID of the proxy process.
func (srv *server) GetProxyProcess() int { return srv.ProxyProcess }

// SetProxyProcess sets the PID of the proxy process.
func (srv *server) SetProxyProcess(pid int) { srv.ProxyProcess = pid }

// GetState returns the current service state.
func (srv *server) GetState() string { return srv.State }

// SetState sets the current service state.
func (srv *server) SetState(state string) { srv.State = state }

// GetLastError returns the last error message.
func (srv *server) GetLastError() string { return srv.LastError }

// SetLastError sets the last error message.
func (srv *server) SetLastError(err string) { srv.LastError = err }

// SetModTime sets the last modification time.
func (srv *server) SetModTime(modtime int64) { srv.ModTime = modtime }

// GetModTime gets the last modification time.
func (srv *server) GetModTime() int64 { return srv.ModTime }

// GetId returns the service instance id.
func (srv *server) GetId() string { return srv.Id }

// SetId sets the service instance id.
func (srv *server) SetId(id string) { srv.Id = id }

// GetName returns the gRPC service name.
func (srv *server) GetName() string { return srv.Name }

// SetName sets the gRPC service name.
func (srv *server) SetName(name string) { srv.Name = name }

// GetMac returns the service host MAC address.
func (srv *server) GetMac() string { return srv.Mac }

// SetMac sets the service host MAC address.
func (srv *server) SetMac(mac string) { srv.Mac = mac }

// GetDescription returns the service description.
func (srv *server) GetDescription() string { return srv.Description }

// SetDescription sets the service description.
func (srv *server) SetDescription(description string) { srv.Description = description }

// GetKeywords returns the service keywords.
func (srv *server) GetKeywords() []string { return srv.Keywords }

// SetKeywords sets the service keywords.
func (srv *server) SetKeywords(keywords []string) { srv.Keywords = keywords }

// Dist packages the service for distribution at the given path.
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

// GetDependencies returns the list of required services.
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	return srv.Dependencies
}

// SetDependency adds a service dependency if it is not already present.
func (srv *server) SetDependency(dependency string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}

// GetChecksum returns the service checksum.
func (srv *server) GetChecksum() string { return srv.Checksum }

// SetChecksum sets the service checksum.
func (srv *server) SetChecksum(checksum string) { srv.Checksum = checksum }

// GetPlatform returns the platform (OS/Arch).
func (srv *server) GetPlatform() string { return srv.Plaform }

// SetPlatform sets the platform (OS/Arch).
func (srv *server) SetPlatform(platform string) { srv.Plaform = platform }

// GetRepositories returns the list of repositories.
func (srv *server) GetRepositories() []string { return srv.Repositories }

// SetRepositories sets the list of repositories.
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }

// GetDiscoveries returns the list of discovery endpoints.
func (srv *server) GetDiscoveries() []string { return srv.Discoveries }

// SetDiscoveries sets the list of discovery endpoints.
func (srv *server) SetDiscoveries(discoveries []string) { srv.Discoveries = discoveries }

// GetPath returns the path of the executable.
func (srv *server) GetPath() string { return srv.Path }

// SetPath sets the path of the executable.
func (srv *server) SetPath(path string) { srv.Path = path }

// GetProto returns the path of the .proto file.
func (srv *server) GetProto() string { return srv.Proto }

// SetProto sets the path of the .proto file.
func (srv *server) SetProto(proto string) { srv.Proto = proto }

// GetPort returns the gRPC port.
func (srv *server) GetPort() int { return srv.Port }

// SetPort sets the gRPC port.
func (srv *server) SetPort(port int) { srv.Port = port }

// GetProxy returns the reverse-proxy port (gRPC-Web).
func (srv *server) GetProxy() int { return srv.Proxy }

// SetProxy sets the reverse-proxy port (gRPC-Web).
func (srv *server) SetProxy(proxy int) { srv.Proxy = proxy }

// GetProtocol returns the transport protocol (http/https/tls/grpc).
func (srv *server) GetProtocol() string { return srv.Protocol }

// SetProtocol sets the transport protocol (http/https/tls/grpc).
func (srv *server) SetProtocol(protocol string) { srv.Protocol = protocol }

// GetAllowAllOrigins indicates whether all origins are allowed.
func (srv *server) GetAllowAllOrigins() bool { return srv.AllowAllOrigins }

// SetAllowAllOrigins sets whether all origins are allowed.
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) { srv.AllowAllOrigins = allowAllOrigins }

// GetAllowedOrigins returns the comma-separated list of allowed origins.
func (srv *server) GetAllowedOrigins() string { return srv.AllowedOrigins }

// SetAllowedOrigins sets the comma-separated list of allowed origins.
func (srv *server) SetAllowedOrigins(allowedOrigins string) { srv.AllowedOrigins = allowedOrigins }

// GetDomain returns the configured service domain (host).
func (srv *server) GetDomain() string { return srv.Domain }

// SetDomain sets the configured service domain (host).
func (srv *server) SetDomain(domain string) { srv.Domain = domain }

// GetTls returns true if TLS is enabled.
func (srv *server) GetTls() bool { return srv.TLS }

// SetTls enables or disables TLS.
func (srv *server) SetTls(hasTls bool) { srv.TLS = hasTls }

// GetCertAuthorityTrust returns the CA trust file path.
func (srv *server) GetCertAuthorityTrust() string { return srv.CertAuthorityTrust }

// SetCertAuthorityTrust sets the CA trust file path.
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }

// GetCertFile returns the TLS certificate file path.
func (srv *server) GetCertFile() string { return srv.CertFile }

// SetCertFile sets the TLS certificate file path.
func (srv *server) SetCertFile(certFile string) { srv.CertFile = certFile }

// GetKeyFile returns the TLS private key file path.
func (srv *server) GetKeyFile() string { return srv.KeyFile }

// SetKeyFile sets the TLS private key file path.
func (srv *server) SetKeyFile(keyFile string) { srv.KeyFile = keyFile }

// GetVersion returns the service version.
func (srv *server) GetVersion() string { return srv.Version }

// SetVersion sets the service version.
func (srv *server) SetVersion(version string) { srv.Version = version }

// GetPublisherID returns the publisher id.
func (srv *server) GetPublisherID() string { return srv.PublisherID }

// SetPublisherID sets the publisher id.
func (srv *server) SetPublisherID(PublisherID string) { srv.PublisherID = PublisherID }

// GetKeepUpToDate returns true if auto-update is enabled.
func (srv *server) GetKeepUpToDate() bool { return srv.KeepUpToDate }

// SetKeepUptoDate sets auto-update behavior.
func (srv *server) SetKeepUptoDate(val bool) { srv.KeepUpToDate = val }

// GetKeepAlive returns true if keep-alive is enabled.
func (srv *server) GetKeepAlive() bool { return srv.KeepAlive }

// SetKeepAlive sets keep-alive behavior.
func (srv *server) SetKeepAlive(val bool) { srv.KeepAlive = val }

// GetPermissions returns the action/resource permissions for the service.
func (srv *server) GetPermissions() []interface{} { return srv.Permissions }

// SetPermissions sets the action/resource permissions for the service.
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }

// GetRbacClient returns an RBAC client bound to this service's address.
func (srv *server) GetRbacClient() (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(srv.Address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

// createPermission sets the caller (from ctx) as the owner of the given resource path.
func (srv *server) createPermission(ctx context.Context, path string) error {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return err
	}
	rbac_client_, err := srv.GetRbacClient()
	if err != nil {
		return err
	}
	return rbac_client_.AddResourceOwner(path, "domain", clientId, rbacpb.SubjectType_ACCOUNT)
}

// Init initializes the service configuration and gRPC server. Must be called before StartService.
func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}
	var err error
	srv.grpcServer, err = globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}
	if len(srv.Root) == 0 {
		fmt.Println("The value StorageDataPath in the configuration must be provided. You can use /tmp (on Linux) if you don't want to keep values permanently.")
	}
	s = srv
	return nil
}

// Save persists the current configuration to disk.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService starts the gRPC server for this DNS service.
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

// StopService gracefully stops the gRPC server for this DNS service.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// Stop is the gRPC endpoint that stops the service. It returns when the stop request is scheduled.
func (srv *server) Stop(context.Context, *dnspb.StopRequest) (*dnspb.StopResponse, error) {
	return &dnspb.StopResponse{}, srv.StopService()
}

/* =========================
   Storage / connection util
   ========================= */

// openConnection opens the persistent store if not already open.
func (srv *server) openConnection() error {
	if srv.connection_is_open {
		return nil
	}
	srv.store = storage_store.NewBadger_store()
	if err := srv.store.Open(`{"path":"` + srv.Root + `", "name":"dns"}`); err != nil {
		return err
	}
	srv.connection_is_open = true
	return nil
}

// isManaged returns true if the given domain (or subdomain) is within a managed zone.
func (srv *server) isManaged(domain string) bool {
	for i := range srv.Domains {
		if strings.HasSuffix(domain, srv.Domains[i]) {
			return true
		}
	}
	return false
}

/* =========================
   A (IPv4) record endpoints
   ========================= */

// SetA stores (or appends) an IPv4 address for the given domain. It also sets the TTL for that record key.
func (srv *server) SetA(ctx context.Context, rqst *dnspb.SetARequest) (*dnspb.SetAResponse, error) {
	if !srv.isManaged(rqst.Domain) {
		err := fmt.Errorf("the domain %s is not managed by this DNS", rqst.Domain)
		_ = srv.logServiceError("SetA", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	domain := strings.ToLower(rqst.Domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	uuid := Utility.GenerateUUID("A:" + domain)

	values := make([]string, 0)

	// Merge new value with existing list (if any).
	if data, err := srv.store.GetItem(uuid); err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			_ = srv.logServiceError("SetA", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if !Utility.Contains(values, rqst.A) {
		values = append(values, rqst.A)
	}

	data, err := json.Marshal(values)
	if err != nil {
		_ = srv.logServiceError("SetA", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if err := srv.store.SetItem(uuid, data); err != nil {
		_ = srv.logServiceError("SetA", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Persist TTL and log success.
	_ = srv.setTtl(uuid, rqst.Ttl)
	_ = srv.logServiceInfo("SetA", Utility.FileLine(), Utility.FunctionName(),
		fmt.Sprintf("A record set: domain=%s uuid=%s ipv4=%s ttl=%d", domain, uuid, rqst.A, rqst.Ttl))

	return &dnspb.SetAResponse{Message: domain}, nil
}

// RemoveA removes a specific IPv4 address from the A record list. If no value remains, the key is deleted.
func (srv *server) RemoveA(ctx context.Context, rqst *dnspb.RemoveARequest) (*dnspb.RemoveAResponse, error) {
	if !srv.isManaged(rqst.Domain) {
		err := fmt.Errorf("the domain %s is not managed by this DNS", rqst.Domain)
		_ = srv.logServiceError("RemoveA", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	domain := strings.ToLower(rqst.Domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	uuid := Utility.GenerateUUID("A:" + domain)

	data, err := srv.store.GetItem(uuid)
	if err != nil {
		_ = srv.logServiceError("RemoveA", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	values := make([]string, 0)
	if err := json.Unmarshal(data, &values); err != nil {
		_ = srv.logServiceError("RemoveA", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if Utility.Contains(values, rqst.A) {
		values = Utility.RemoveString(values, rqst.A)
	}

	if len(values) == 0 {
		if err := srv.store.RemoveItem(uuid); err != nil {
			_ = srv.logServiceError("RemoveA", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if rbac_client_, err := srv.GetRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(domain)
		}
		_ = srv.logServiceInfo("RemoveA", Utility.FileLine(), Utility.FunctionName(),
			fmt.Sprintf("A record deleted: domain=%s uuid=%s ipv4=%s", domain, uuid, rqst.A))
	} else {
		data, err = json.Marshal(values)
		if err != nil {
			_ = srv.logServiceError("RemoveA", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if err := srv.store.SetItem(uuid, data); err != nil {
			_ = srv.logServiceError("RemoveA", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		_ = srv.logServiceInfo("RemoveA", Utility.FileLine(), Utility.FunctionName(),
			fmt.Sprintf("A record value removed: domain=%s uuid=%s ipv4=%s remaining=%d", domain, uuid, rqst.A, len(values)))
	}

	return &dnspb.RemoveAResponse{Result: true}, nil
}

// get_ipv4 returns all IPv4 addresses and TTL for a given domain.
func (srv *server) get_ipv4(domain string) ([]string, uint32, error) {
	domain = strings.ToLower(domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	uuid := Utility.GenerateUUID("A:" + domain)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return nil, 0, err
	}
	values := make([]string, 0)
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, 0, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return values, srv.getTtl(uuid), nil
}

// orderIPsByPrivacy places private IPs first while keeping relative order within the privacy group.
func orderIPsByPrivacy(ips []string) []string {
	sort.Slice(ips, func(i, j int) bool {
		ip1 := net.ParseIP(ips[i])
		ip2 := net.ParseIP(ips[j])
		isPrivate1 := ip1 != nil && ip1.IsPrivate()
		isPrivate2 := ip2 != nil && ip2.IsPrivate()
		if isPrivate1 == isPrivate2 {
			return i < j
		}
		return isPrivate1
	})
	return ips
}

// GetA returns the list of IPv4 addresses associated with a domain.
func (srv *server) GetA(ctx context.Context, rqst *dnspb.GetARequest) (*dnspb.GetAResponse, error) {
	domain := strings.ToLower(rqst.Domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	uuid := Utility.GenerateUUID("A:" + domain)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0)
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	values = orderIPsByPrivacy(values)
	return &dnspb.GetAResponse{A: values}, nil
}

/* ===========================
   AAAA (IPv6) record endpoints
   =========================== */

// SetAAAA stores (or appends) an IPv6 address for the given domain and sets its TTL.
func (srv *server) SetAAAA(ctx context.Context, rqst *dnspb.SetAAAARequest) (*dnspb.SetAAAAResponse, error) {
	if !srv.isManaged(rqst.Domain) {
		err := fmt.Errorf("the domain %s is not managed by this DNS", rqst.Domain)
		_ = srv.logServiceError("SetAAAA", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	domain := strings.ToLower(rqst.Domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	uuid := Utility.GenerateUUID("AAAA:" + domain)

	values := make([]string, 0)
	if data, err := srv.store.GetItem(uuid); err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			_ = srv.logServiceError("SetAAAA", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if !Utility.Contains(values, rqst.Aaaa) {
		values = append(values, rqst.Aaaa)
	}
	data, err := json.Marshal(values)
	if err != nil {
		_ = srv.logServiceError("SetAAAA", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.store.SetItem(uuid, data); err != nil {
		_ = srv.logServiceError("SetAAAA", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.setTtl(uuid, rqst.Ttl)
	_ = srv.logServiceInfo("SetAAAA", Utility.FileLine(), Utility.FunctionName(),
		fmt.Sprintf("AAAA record set: domain=%s uuid=%s ipv6=%s ttl=%d", domain, uuid, rqst.Aaaa, rqst.Ttl))

	return &dnspb.SetAAAAResponse{Message: domain}, nil
}

// RemoveAAAA removes a specific IPv6 address from the AAAA record list (or deletes the key if no values remain).
func (srv *server) RemoveAAAA(ctx context.Context, rqst *dnspb.RemoveAAAARequest) (*dnspb.RemoveAAAAResponse, error) {
	domain := strings.ToLower(rqst.Domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	if !srv.isManaged(rqst.Domain) {
		err := fmt.Errorf("the domain %s is not managed by this DNS", rqst.Domain)
		_ = srv.logServiceError("RemoveAAAA", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("AAAA:" + domain)
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			_ = srv.logServiceError("RemoveAAAA", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if Utility.Contains(values, rqst.Aaaa) {
		values = Utility.RemoveString(values, rqst.Aaaa)
	}

	if len(values) == 0 {
		if err := srv.store.RemoveItem(uuid); err != nil {
			_ = srv.logServiceError("RemoveAAAA", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if rbac_client_, err := srv.GetRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(domain)
		}
		_ = srv.logServiceInfo("RemoveAAAA", Utility.FileLine(), Utility.FunctionName(),
			fmt.Sprintf("AAAA record deleted: domain=%s uuid=%s ipv6=%s", domain, uuid, rqst.Aaaa))
	} else {
		data, err = json.Marshal(values)
		if err != nil {
			_ = srv.logServiceError("RemoveAAAA", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if err := srv.store.SetItem(uuid, data); err != nil {
			_ = srv.logServiceError("RemoveAAAA", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		_ = srv.logServiceInfo("RemoveAAAA", Utility.FileLine(), Utility.FunctionName(),
			fmt.Sprintf("AAAA record value removed: domain=%s uuid=%s ipv6=%s remaining=%d", domain, uuid, rqst.Aaaa, len(values)))
	}

	return &dnspb.RemoveAAAAResponse{Result: true}, nil
}

// get_ipv6 returns all IPv6 addresses and TTL for a given domain.
func (srv *server) get_ipv6(domain string) ([]string, uint32, error) {
	domain = strings.ToLower(domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	uuid := Utility.GenerateUUID("AAAA:" + domain)
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, 0, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		return nil, 0, err
	}
	if len(values) == 0 {
		return nil, 0, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no value found for domain "+domain)))
	}
	return values, srv.getTtl(uuid), nil
}

// GetAAAA returns the list of IPv6 addresses associated with a domain.
func (srv *server) GetAAAA(ctx context.Context, rqst *dnspb.GetAAAARequest) (*dnspb.GetAAAAResponse, error) {
	domain := strings.ToLower(rqst.Domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	uuid := Utility.GenerateUUID("AAAA:" + domain)
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(values) == 0 {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no value found for domain "+domain)))
	}
	return &dnspb.GetAAAAResponse{Aaaa: values}, nil
}

/* ===============
   TXT record API
   =============== */

// SetText appends TXT values for an identifier and stores TTL.
func (srv *server) SetText(ctx context.Context, rqst *dnspb.SetTextRequest) (*dnspb.SetTextResponse, error) {
	values, err := json.Marshal(rqst.Values)
	if err != nil {
		_ = srv.logServiceError("SetText", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("TXT:" + id)

	// Merge with existing values (if any).
	if data, err := srv.store.GetItem(uuid); err == nil {
		values_ := make([]string, 0)
		if err := json.Unmarshal(data, &values_); err != nil {
			_ = srv.logServiceError("SetText", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		values_ = append(values_, rqst.Values...)
		values, err = json.Marshal(values_)
		if err != nil {
			_ = srv.logServiceError("SetText", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if err := srv.store.SetItem(uuid, values); err != nil {
		_ = srv.logServiceError("SetText", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.setTtl(uuid, rqst.Ttl)
	_ = srv.logServiceInfo("SetText", Utility.FileLine(), Utility.FunctionName(),
		fmt.Sprintf("TXT set: id=%s uuid=%s values=%d ttl=%d", id, uuid, len(rqst.Values), rqst.Ttl))

	return &dnspb.SetTextResponse{Result: true}, nil
}

// getText returns TXT values and TTL for an identifier.
func (srv *server) getText(id string) ([]string, uint32, error) {
	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("TXT:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, 0, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		return nil, 0, err
	}

	_ = srv.logServiceInfo("getText", Utility.FileLine(), Utility.FunctionName(),
		fmt.Sprintf("TXT get: id=%s uuid=%s values=%d", id, uuid, len(values)))

	return values, srv.getTtl(uuid), nil
}

// GetText returns TXT values for an identifier.
func (srv *server) GetText(ctx context.Context, rqst *dnspb.GetTextRequest) (*dnspb.GetTextResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("TXT:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	return &dnspb.GetTextResponse{Values: values}, nil
}

// RemoveText deletes all TXT values for an identifier.
func (srv *server) RemoveText(ctx context.Context, rqst *dnspb.RemoveTextRequest) (*dnspb.RemoveTextResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("TXT:" + id)
	if err := srv.store.RemoveItem(uuid); err != nil {
		_ = srv.logServiceError("RemoveText", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if rbac_client_, err := srv.GetRbacClient(); err == nil {
		_ = rbac_client_.DeleteResourcePermissions(rqst.Id)
	}
	_ = srv.logServiceInfo("RemoveText", Utility.FileLine(), Utility.FunctionName(),
		fmt.Sprintf("TXT removed: id=%s uuid=%s", id, uuid))

	return &dnspb.RemoveTextResponse{Result: true}, nil
}

/* ==============
   NS record API
   ============== */

// SetNs appends a nameserver (NS) value for an identifier and stores TTL.
func (srv *server) SetNs(ctx context.Context, rqst *dnspb.SetNsRequest) (*dnspb.SetNsResponse, error) {
	id := strings.ToLower(rqst.Id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	uuid := Utility.GenerateUUID("NS:" + id)

	values := make([]string, 0)
	if data, err := srv.store.GetItem(uuid); err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			_ = srv.logServiceError("SetNs", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	ns := strings.ToLower(rqst.Ns)
	if !strings.HasSuffix(ns, ".") {
		ns += "."
	}
	if !Utility.Contains(values, ns) {
		values = append(values, ns)
	}

	data, err := json.Marshal(values)
	if err != nil {
		_ = srv.logServiceError("SetNs", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.store.SetItem(uuid, data); err != nil {
		_ = srv.logServiceError("SetNs", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.setTtl(uuid, rqst.Ttl)
	_ = srv.logServiceInfo("SetNs", Utility.FileLine(), Utility.FunctionName(),
		fmt.Sprintf("NS set: id=%s uuid=%s ns=%s ttl=%d", id, uuid, ns, rqst.Ttl))

	return &dnspb.SetNsResponse{Result: true}, nil
}

// getNs returns NS values and TTL for a domain, walking up to parent if none at exact label.
func (srv *server) getNs(id string) ([]string, uint32, error) {
	id = strings.ToLower(id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	uuid := Utility.GenerateUUID("NS:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, 0, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		parts := strings.Split(id, ".")
		if len(parts) > 2 {
			id = strings.Join(parts[1:], ".")
			return srv.getNs(id)
		}
		return nil, 0, err
	}
	return values, srv.getTtl(uuid), nil
}

// GetNs returns NS values for a domain identifier.
func (srv *server) GetNs(ctx context.Context, rqst *dnspb.GetNsRequest) (*dnspb.GetNsResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("NS:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	return &dnspb.GetNsResponse{Ns: values}, nil
}

// RemoveNs removes a specific NS value (or deletes the key if no values remain).
func (srv *server) RemoveNs(ctx context.Context, rqst *dnspb.RemoveNsRequest) (*dnspb.RemoveNsResponse, error) {
	id := strings.ToLower(rqst.Id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	uuid := Utility.GenerateUUID("NS:" + id)

	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			_ = srv.logServiceError("RemoveNs", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if len(values) == 0 {
		err := errors.New("no value found for domain " + id)
		_ = srv.logServiceError("RemoveNs", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	ns := strings.ToLower(rqst.Ns)
	if !strings.HasSuffix(ns, ".") {
		ns += "."
	}
	if Utility.Contains(values, ns) {
		values = Utility.RemoveString(values, ns)
	}

	if len(values) == 0 {
		if err := srv.store.RemoveItem(uuid); err != nil {
			_ = srv.logServiceError("RemoveNs", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if rbac_client_, err := srv.GetRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(rqst.Id)
		}
		_ = srv.logServiceInfo("RemoveNs", Utility.FileLine(), Utility.FunctionName(),
			fmt.Sprintf("NS record deleted: id=%s uuid=%s ns=%s", id, uuid, ns))
	} else {
		data, err = json.Marshal(values)
		if err != nil {
			_ = srv.logServiceError("RemoveNs", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if err := srv.store.SetItem(uuid, data); err != nil {
			_ = srv.logServiceError("RemoveNs", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		_ = srv.logServiceInfo("RemoveNs", Utility.FileLine(), Utility.FunctionName(),
			fmt.Sprintf("NS value removed: id=%s uuid=%s ns=%s remaining=%d", id, uuid, ns, len(values)))
	}

	return &dnspb.RemoveNsResponse{Result: true}, nil
}

/* =================
   CNAME record API
   ================= */

// SetCName stores a CNAME target for an identifier and sets TTL.
func (srv *server) SetCName(ctx context.Context, rqst *dnspb.SetCNameRequest) (*dnspb.SetCNameResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CName:" + id)
	if err := srv.store.SetItem(uuid, []byte(rqst.Cname)); err != nil {
		_ = srv.logServiceError("SetCName", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.setTtl(uuid, rqst.Ttl)
	_ = srv.logServiceInfo("SetCName", Utility.FileLine(), Utility.FunctionName(),
		fmt.Sprintf("CNAME set: id=%s uuid=%s target=%s ttl=%d", id, uuid, rqst.Cname, rqst.Ttl))
	return &dnspb.SetCNameResponse{Result: true}, nil
}

// getCName returns the CNAME target and TTL for an identifier.
func (srv *server) getCName(id string) (string, uint32, error) {
	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("CName:" + id)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return "", 0, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return string(data), srv.getTtl(uuid), nil
}

// GetCName returns the stored CNAME target for an identifier.
func (srv *server) GetCName(ctx context.Context, rqst *dnspb.GetCNameRequest) (*dnspb.GetCNameResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CName:" + id)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &dnspb.GetCNameResponse{Cname: string(data)}, nil
}

// RemoveCName deletes the CNAME record for an identifier.
func (srv *server) RemoveCName(ctx context.Context, rqst *dnspb.RemoveCNameRequest) (*dnspb.RemoveCNameResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CName:" + id)
	if err := srv.store.RemoveItem(uuid); err != nil {
		_ = srv.logServiceError("RemoveCName", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if rbac_client_, err := srv.GetRbacClient(); err == nil {
		_ = rbac_client_.DeleteResourcePermissions(rqst.Id)
	}
	_ = srv.logServiceInfo("RemoveCName", Utility.FileLine(), Utility.FunctionName(),
		fmt.Sprintf("CNAME removed: id=%s uuid=%s", id, uuid))
	return &dnspb.RemoveCNameResponse{Result: true}, nil
}

/* =============
   MX record API
   ============= */

// SetMx appends/updates an MX record for a domain and stores TTL.
func (srv *server) SetMx(ctx context.Context, rqst *dnspb.SetMxRequest) (*dnspb.SetMxResponse, error) {
	id := strings.ToLower(rqst.Id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	if !strings.HasSuffix(rqst.Mx.Mx, ".") {
		rqst.Mx.Mx += "."
	}

	uuid := Utility.GenerateUUID("MX:" + id)
	values := make([]*dnspb.MX, 0)

	if data, err := srv.store.GetItem(uuid); err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			_ = srv.logServiceError("SetMx", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	found := false
	for i := range values {
		if values[i].Mx == rqst.Mx.Mx {
			values[i] = rqst.Mx
			found = true
			break
		}
	}
	if !found && rqst.Mx != nil {
		values = append(values, rqst.Mx)
	}

	data, err := json.Marshal(values)
	if err != nil {
		_ = srv.logServiceError("SetMx", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.store.SetItem(uuid, data); err != nil {
		_ = srv.logServiceError("SetMx", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	_ = srv.setTtl(uuid, rqst.Ttl)
	_ = srv.logServiceInfo("SetMx", Utility.FileLine(), Utility.FunctionName(),
		fmt.Sprintf("MX set: id=%s uuid=%s host=%s pref=%d ttl=%d", id, uuid, rqst.Mx.Mx, rqst.Mx.Preference, rqst.Ttl))

	return &dnspb.SetMxResponse{Result: true}, nil
}

// getMx returns MX records and TTL for a domain. If mx is provided, it filters to that host.
func (srv *server) getMx(id, mx string) ([]*dnspb.MX, uint32, error) {
	id = strings.ToLower(id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	mx = strings.ToLower(mx)
	if len(mx) > 0 && !strings.HasSuffix(mx, ".") {
		mx += "."
	}

	uuid := Utility.GenerateUUID("MX:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.MX, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, 0, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(values) > 0 && len(mx) > 0 {
		for i := range values {
			if values[i].Mx == mx {
				return []*dnspb.MX{values[i]}, srv.getTtl(uuid), nil
			}
		}
	}
	return values, srv.getTtl(uuid), nil
}

// GetMx returns all MX records for a domain, or a specific one if rqst.Mx is set.
func (srv *server) GetMx(ctx context.Context, rqst *dnspb.GetMxRequest) (*dnspb.GetMxResponse, error) {
	id := strings.ToLower(rqst.Id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	uuid := Utility.GenerateUUID("MX:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.MX, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(rqst.Mx) > 0 {
		for i := range values {
			if values[i].Mx == rqst.Mx {
				return &dnspb.GetMxResponse{Result: []*dnspb.MX{values[i]}}, nil
			}
		}
	}
	return &dnspb.GetMxResponse{Result: values}, nil
}

// RemoveMx removes a specific MX entry (or deletes the key if no values remain).
func (srv *server) RemoveMx(ctx context.Context, rqst *dnspb.RemoveMxRequest) (*dnspb.RemoveMxResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("MX:" + id)

	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.MX, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			_ = srv.logServiceError("RemoveMx", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if len(values) == 0 {
		err := errors.New("no value found for domain " + id)
		_ = srv.logServiceError("RemoveMx", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	for i := range values {
		if values[i].Mx == rqst.Mx {
			values = append(values[:i], values[i+1:]...)
			break
		}
	}

	data, err = json.Marshal(values)
	if err != nil {
		_ = srv.logServiceError("RemoveMx", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(values) > 0 {
		if err := srv.store.SetItem(uuid, data); err != nil {
			_ = srv.logServiceError("RemoveMx", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		_ = srv.logServiceInfo("RemoveMx", Utility.FileLine(), Utility.FunctionName(),
			fmt.Sprintf("MX value removed: id=%s uuid=%s host=%s remaining=%d", id, uuid, rqst.Mx, len(values)))
	} else {
		if err := srv.store.RemoveItem(uuid); err != nil {
			_ = srv.logServiceError("RemoveMx", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if rbac_client_, err := srv.GetRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(rqst.Id)
		}
		_ = srv.logServiceInfo("RemoveMx", Utility.FileLine(), Utility.FunctionName(),
			fmt.Sprintf("MX record deleted: id=%s uuid=%s host=%s", id, uuid, rqst.Mx))
	}

	return &dnspb.RemoveMxResponse{Result: true}, nil
}

/* =============
   SOA record API
   ============= */

// SetSoa appends/updates a SOA value for a domain and sets TTL. Multiple SOAs can be stored (by NS).
func (srv *server) SetSoa(ctx context.Context, rqst *dnspb.SetSoaRequest) (*dnspb.SetSoaResponse, error) {
	id := strings.ToLower(rqst.Id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	if !strings.HasSuffix(rqst.Soa.Ns, ".") {
		rqst.Soa.Ns += "."
	}
	uuid := Utility.GenerateUUID("SOA:" + id)

	values := make([]*dnspb.SOA, 0)
	if data, err := srv.store.GetItem(uuid); err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			_ = srv.logServiceError("SetSoa", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	for i := range values {
		ns := strings.ToLower(values[i].Ns)
		if !strings.HasSuffix(ns, ".") {
			ns += "."
		}
		if ns == rqst.Soa.Ns {
			values[i] = rqst.Soa
			rqst.Soa = nil
			break
		}
	}
	if rqst.Soa != nil {
		values = append(values, rqst.Soa)
	}

	data, err := json.Marshal(values)
	if err != nil {
		_ = srv.logServiceError("SetSoa", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.store.SetItem(uuid, data); err != nil {
		_ = srv.logServiceError("SetSoa", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.setTtl(uuid, rqst.Ttl)
	_ = srv.logServiceInfo("SetSoa", Utility.FileLine(), Utility.FunctionName(),
		fmt.Sprintf("SOA set: id=%s uuid=%s ns=%s ttl=%d", id, uuid, rqst.Soa.GetNs(), rqst.Ttl))

	return &dnspb.SetSoaResponse{Result: true}, nil
}

// getSoa returns SOA records and TTL for a domain. If ns is provided, it filters to that NS.
// It walks up the domain tree to find a parent SOA if the exact label has none.
func (srv *server) getSoa(id, ns string) ([]*dnspb.SOA, uint32, error) {
	id = strings.ToLower(id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	uuid := Utility.GenerateUUID("SOA:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.SOA, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, 0, err
		}
	} else {
		parts := strings.Split(id, ".")
		if len(parts) > 2 {
			id = strings.Join(parts[1:], ".")
			return srv.getSoa(id, ns)
		}
		return nil, 0, err
	}

	if len(ns) > 0 {
		for i := range values {
			if values[i].Ns == ns {
				return []*dnspb.SOA{values[i]}, srv.getTtl(uuid), nil
			}
		}
	}
	return values, srv.getTtl(uuid), nil
}

// GetSoa returns all SOA records for a domain, or a specific one if rqst.Ns is set.
func (srv *server) GetSoa(ctx context.Context, rqst *dnspb.GetSoaRequest) (*dnspb.GetSoaResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("SOA:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.SOA, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(rqst.Ns) > 0 {
		for i := range values {
			if values[i].Ns == rqst.Ns {
				return &dnspb.GetSoaResponse{Result: []*dnspb.SOA{values[i]}}, nil
			}
		}
	}
	return &dnspb.GetSoaResponse{Result: values}, nil
}

// RemoveSoa removes a specific SOA by NS (or deletes the key if none remain).
func (srv *server) RemoveSoa(ctx context.Context, rqst *dnspb.RemoveSoaRequest) (*dnspb.RemoveSoaResponse, error) {
	id := strings.ToLower(rqst.Id)
	if !strings.HasSuffix(id, ".") {
		id += "."
	}
	uuid := Utility.GenerateUUID("SOA:" + id)

	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.SOA, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			_ = srv.logServiceError("RemoveSoa", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(values) == 0 {
		err := errors.New("no value found for domain " + id)
		_ = srv.logServiceError("RemoveSoa", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if !strings.HasSuffix(rqst.Ns, ".") {
		rqst.Ns += "."
	}
	for i := range values {
		ns := strings.ToLower(values[i].Ns)
		if !strings.HasSuffix(ns, ".") {
			ns += "."
		}
		if ns == rqst.Ns {
			values = append(values[:i], values[i+1:]...)
			break
		}
	}

	data, err = json.Marshal(values)
	if err != nil {
		_ = srv.logServiceError("RemoveSoa", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(values) > 0 {
		if err := srv.store.SetItem(uuid, data); err != nil {
			_ = srv.logServiceError("RemoveSoa", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		_ = srv.logServiceInfo("RemoveSoa", Utility.FileLine(), Utility.FunctionName(),
			fmt.Sprintf("SOA value removed: id=%s uuid=%s ns=%s remaining=%d", id, uuid, rqst.Ns, len(values)))
	} else {
		if err := srv.store.RemoveItem(uuid); err != nil {
			_ = srv.logServiceError("RemoveSoa", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if rbac_client_, err := srv.GetRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(rqst.Id)
		}
		_ = srv.logServiceInfo("RemoveSoa", Utility.FileLine(), Utility.FunctionName(),
			fmt.Sprintf("SOA record deleted: id=%s uuid=%s ns=%s", id, uuid, rqst.Ns))
	}

	return &dnspb.RemoveSoaResponse{Result: true}, nil
}

/* =============
   URI record API
   ============= */

// SetUri upserts a URI record (by Target) for an identifier and sets TTL.
func (srv *server) SetUri(ctx context.Context, rqst *dnspb.SetUriRequest) (*dnspb.SetUriResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("URI:" + id)

	values := make([]*dnspb.URI, 0)
	if data, err := srv.store.GetItem(uuid); err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			_ = srv.logServiceError("SetUri", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	for i := range values {
		if values[i].Target == rqst.Uri.Target {
			values[i] = rqst.Uri
			rqst.Uri = nil
			break
		}
	}
	if rqst.Uri != nil {
		values = append(values, rqst.Uri)
	}

	data, err := json.Marshal(values)
	if err != nil {
		_ = srv.logServiceError("SetUri", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.store.SetItem(uuid, data); err != nil {
		_ = srv.logServiceError("SetUri", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.setTtl(uuid, rqst.Ttl)
	_ = srv.logServiceInfo("SetUri", Utility.FileLine(), Utility.FunctionName(),
		fmt.Sprintf("URI set: id=%s uuid=%s target=%s priority=%d weight=%d ttl=%d",
			id, uuid, rqst.Uri.GetTarget(), rqst.Uri.GetPriority(), rqst.Uri.GetWeight(), rqst.Ttl))
	return &dnspb.SetUriResponse{Result: true}, nil
}

// getUri returns URI records and TTL for an identifier. If target is provided, it filters to that target.
func (srv *server) getUri(id, target string) ([]*dnspb.URI, uint32, error) {
	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("URI:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.URI, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, 0, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(target) > 0 {
		for i := range values {
			if values[i].Target == target {
				return []*dnspb.URI{values[i]}, srv.getTtl(uuid), nil
			}
		}
	}
	return values, srv.getTtl(uuid), nil
}

// GetUri returns all URI records for an identifier, or a specific one if rqst.Target is set.
func (srv *server) GetUri(ctx context.Context, rqst *dnspb.GetUriRequest) (*dnspb.GetUriResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("URI:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.URI, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(rqst.Target) > 0 {
		for i := range values {
			if values[i].Target == rqst.Target {
				return &dnspb.GetUriResponse{Result: []*dnspb.URI{values[i]}}, nil
			}
		}
	}
	return &dnspb.GetUriResponse{Result: values}, nil
}

// RemoveUri removes a single URI record (by target) or deletes the key if no values remain.
func (srv *server) RemoveUri(ctx context.Context, rqst *dnspb.RemoveUriRequest) (*dnspb.RemoveUriResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("URI:" + id)

	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.URI, 0)
	if err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			_ = srv.logServiceError("RemoveUri", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(values) == 0 {
		err := errors.New("no value found for domain " + id)
		_ = srv.logServiceError("RemoveUri", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	for i := range values {
		if values[i].Target == rqst.Target {
			values = append(values[:i], values[i+1:]...)
			break
		}
	}

	data, err = json.Marshal(values)
	if err != nil {
		_ = srv.logServiceError("RemoveUri", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(values) > 0 {
		if err := srv.store.SetItem(uuid, data); err != nil {
			_ = srv.logServiceError("RemoveUri", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		_ = srv.logServiceInfo("RemoveUri", Utility.FileLine(), Utility.FunctionName(),
			fmt.Sprintf("URI value removed: id=%s uuid=%s target=%s remaining=%d", id, uuid, rqst.Target, len(values)))
	} else {
		if err := srv.store.RemoveItem(uuid); err != nil {
			_ = srv.logServiceError("RemoveUri", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if rbac_client_, err := srv.GetRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(rqst.Id)
		}
		_ = srv.logServiceInfo("RemoveUri", Utility.FileLine(), Utility.FunctionName(),
			fmt.Sprintf("URI record deleted: id=%s uuid=%s target=%s", id, uuid, rqst.Target))
	}

	return &dnspb.RemoveUriResponse{Result: true}, nil
}

/* ==============
   AFSDB record API
   ============== */

// SetAfsdb sets an AFSDB record and TTL for an identifier.
func (srv *server) SetAfsdb(ctx context.Context, rqst *dnspb.SetAfsdbRequest) (*dnspb.SetAfsdbResponse, error) {
	values, err := json.Marshal(rqst.Afsdb)
	if err != nil {
		_ = srv.logServiceError("SetAfsdb", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("AFSDB:" + id)
	if err := srv.store.SetItem(uuid, values); err != nil {
		_ = srv.logServiceError("SetAfsdb", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.setTtl(uuid, rqst.Ttl)
	_ = srv.logServiceInfo("SetAfsdb", Utility.FileLine(), Utility.FunctionName(),
		fmt.Sprintf("AFSDB set: id=%s uuid=%s subtype=%d host=%s ttl=%d", id, uuid, rqst.Afsdb.GetSubtype(), rqst.Afsdb.GetHostname(), rqst.Ttl))
	return &dnspb.SetAfsdbResponse{Result: true}, nil
}

// getAfsdb returns an AFSDB record and TTL for an identifier.
func (srv *server) getAfsdb(id string) (*dnspb.AFSDB, uint32, error) {
	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("AFSDB:" + id)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return nil, 0, err
	}
	afsdb := new(dnspb.AFSDB)
	if err := json.Unmarshal(data, afsdb); err != nil {
		return nil, 0, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return afsdb, srv.getTtl(uuid), nil
}

// GetAfsdb returns the AFSDB record for an identifier.
func (srv *server) GetAfsdb(ctx context.Context, rqst *dnspb.GetAfsdbRequest) (*dnspb.GetAfsdbResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("AFSDB:" + id)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	afsdb := new(dnspb.AFSDB)
	if err := json.Unmarshal(data, afsdb); err != nil {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &dnspb.GetAfsdbResponse{Result: afsdb}, nil
}

// RemoveAfsdb deletes the AFSDB record for an identifier.
func (srv *server) RemoveAfsdb(ctx context.Context, rqst *dnspb.RemoveAfsdbRequest) (*dnspb.RemoveAfsdbResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("AFSDB:" + id)
	if err := srv.store.RemoveItem(uuid); err != nil {
		_ = srv.logServiceError("RemoveAfsdb", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if rbac_client_, err := srv.GetRbacClient(); err == nil {
		_ = rbac_client_.DeleteResourcePermissions(rqst.Id)
	}
	_ = srv.logServiceInfo("RemoveAfsdb", Utility.FileLine(), Utility.FunctionName(),
		fmt.Sprintf("AFSDB removed: id=%s uuid=%s", id, uuid))
	return &dnspb.RemoveAfsdbResponse{Result: true}, nil
}

/* =============
   CAA record API
   ============= */

// SetCaa upserts a CAA value (by domain) for an identifier and sets TTL.
func (srv *server) SetCaa(ctx context.Context, rqst *dnspb.SetCaaRequest) (*dnspb.SetCaaResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CAA:" + id)

	values := make([]*dnspb.CAA, 0)
	if data, err := srv.store.GetItem(uuid); err == nil {
		if err := json.Unmarshal(data, &values); err != nil {
			_ = srv.logServiceError("SetCaa", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	for i := range values {
		if values[i].Domain == rqst.Caa.Domain {
			values[i] = rqst.Caa
			rqst.Caa = nil
			break
		}
	}
	if rqst.Caa != nil {
		values = append(values, rqst.Caa)
	}

	data, err := json.Marshal(values)
	if err != nil {
		_ = srv.logServiceError("SetCaa", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.store.SetItem(uuid, data); err != nil {
		_ = srv.logServiceError("SetCaa", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.setTtl(uuid, rqst.Ttl)
	_ = srv.logServiceInfo("SetCaa", Utility.FileLine(), Utility.FunctionName(),
		fmt.Sprintf("CAA set: id=%s uuid=%s tag=%s value=%s ttl=%d", id, uuid, rqst.Caa.GetTag(), rqst.Caa.GetDomain(), rqst.Ttl))
	return &dnspb.SetCaaResponse{Result: true}, nil
}

// getCaa returns CAA records and TTL for an identifier. If domain is provided, it filters to that domain.
func (srv *server) getCaa(id, domain string) ([]*dnspb.CAA, uint32, error) {
	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("CAA:" + id)
	data, err := srv.store.GetItem(uuid)
	caa := make([]*dnspb.CAA, 0)
	if err == nil {
		if err := json.Unmarshal(data, &caa); err != nil {
			return nil, 0, err
		}
	} else {
		return nil, 0, err
	}

	if len(domain) > 0 {
		for i := range caa {
			if caa[i].Domain == domain {
				return []*dnspb.CAA{caa[i]}, srv.getTtl(uuid), nil
			}
		}
	}
	return caa, srv.getTtl(uuid), nil
}

// GetCaa returns all CAA records for an identifier, or a specific one if rqst.Domain is set.
func (srv *server) GetCaa(ctx context.Context, rqst *dnspb.GetCaaRequest) (*dnspb.GetCaaResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CAA:" + id)
	data, err := srv.store.GetItem(uuid)
	caa := make([]*dnspb.CAA, 0)
	if err == nil {
		if err := json.Unmarshal(data, &caa); err != nil {
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(rqst.Domain) > 0 {
		for i := range caa {
			if caa[i].Domain == rqst.Domain {
				return &dnspb.GetCaaResponse{Result: []*dnspb.CAA{caa[i]}}, nil
			}
		}
	}
	return &dnspb.GetCaaResponse{Result: caa}, nil
}

// RemoveCaa removes a single CAA value (by domain) or deletes the key if none remain.
func (srv *server) RemoveCaa(ctx context.Context, rqst *dnspb.RemoveCaaRequest) (*dnspb.RemoveCaaResponse, error) {
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CAA:" + id)

	data, err := srv.store.GetItem(uuid)
	caa := make([]*dnspb.CAA, 0)
	if err == nil {
		if err := json.Unmarshal(data, &caa); err != nil {
			_ = srv.logServiceError("RemoveCaa", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	if len(caa) == 0 {
		err := errors.New("no value found for domain " + id)
		_ = srv.logServiceError("RemoveCaa", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	for i := range caa {
		if caa[i].Domain == rqst.Domain {
			caa = append(caa[:i], caa[i+1:]...)
			break
		}
	}

	data, err = json.Marshal(caa)
	if err != nil {
		_ = srv.logServiceError("RemoveCaa", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(caa) > 0 {
		if err := srv.store.SetItem(uuid, data); err != nil {
			_ = srv.logServiceError("RemoveCaa", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		_ = srv.logServiceInfo("RemoveCaa", Utility.FileLine(), Utility.FunctionName(),
			fmt.Sprintf("CAA value removed: id=%s uuid=%s domain=%s remaining=%d", id, uuid, rqst.Domain, len(caa)))
	} else {
		if err := srv.store.RemoveItem(uuid); err != nil {
			_ = srv.logServiceError("RemoveCaa", Utility.FileLine(), Utility.FunctionName(), err.Error())
			return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if rbac_client_, err := srv.GetRbacClient(); err == nil {
			_ = rbac_client_.DeleteResourcePermissions(rqst.Id)
		}
		_ = srv.logServiceInfo("RemoveCaa", Utility.FileLine(), Utility.FunctionName(),
			fmt.Sprintf("CAA record deleted: id=%s uuid=%s domain=%s", id, uuid, rqst.Domain))
	}

	return &dnspb.RemoveCaaResponse{Result: true}, nil
}

/* ==========================
   UDP DNS responder (miekg)
   ========================== */

type handler struct{}

// ServeDNS handles incoming DNS queries from UDP and writes responses using
// data stored by the service. Errors are logged via the log service.
func (hd *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	switch r.Question[0].Qtype {
	case dns.TypeA:
		msg := dns.Msg{}
		msg.SetReply(r)
		domain := msg.Question[0].Name
		msg.Authoritative = true
		addresses, ttl, err := s.get_ipv4(domain)
		if err == nil {
			for _, address := range addresses {
				msg.Answer = append(msg.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
					A:   net.ParseIP(address),
				})
			}
		}
		if err := w.WriteMsg(&msg); err != nil {
			_ = s.logServiceError("ServeDNS(TypeA)", Utility.FileLine(), Utility.FunctionName(), err.Error())
		}

	case dns.TypeAAAA:
		msg := dns.Msg{}
		msg.SetReply(r)
		msg.Authoritative = true
		domain := msg.Question[0].Name
		addresses, ttl, err := s.get_ipv6(domain)
		if err == nil {
			for _, address := range addresses {
				msg.Answer = append(msg.Answer, &dns.AAAA{
					Hdr:  dns.RR_Header{Name: domain, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: ttl},
					AAAA: net.ParseIP(address),
				})
			}
		}
		if err := w.WriteMsg(&msg); err != nil {
			_ = s.logServiceError("ServeDNS(TypeAAAA)", Utility.FileLine(), Utility.FunctionName(), err.Error())
		}

	case dns.TypeAFSDB:
		msg := dns.Msg{}
		msg.SetReply(r)
		msg.Authoritative = true
		afsdb, ttl, err := s.getAfsdb(msg.Question[0].Name)
		if err == nil {
			msg.Answer = append(msg.Answer, &dns.AFSDB{
				Hdr:      dns.RR_Header{Name: msg.Question[0].Name, Rrtype: dns.TypeAFSDB, Class: dns.ClassINET, Ttl: ttl},
				Subtype:  uint16(afsdb.Subtype),
				Hostname: afsdb.Hostname,
			})
		}
		if err := w.WriteMsg(&msg); err != nil {
			_ = s.logServiceError("ServeDNS(TypeAFSDB)", Utility.FileLine(), Utility.FunctionName(), err.Error())
		}

	case dns.TypeCAA:
		msg := dns.Msg{}
		msg.SetReply(r)
		msg.Authoritative = true
		name := msg.Question[0].Name
		domain := ""
		if len(msg.Question) > 1 {
			domain = msg.Question[1].Name
		}
		values, ttl, err := s.getCaa(name, domain)
		if err == nil {
			for _, caa := range values {
				msg.Answer = append(msg.Answer, &dns.CAA{
					Hdr:   dns.RR_Header{Name: name, Rrtype: dns.TypeCAA, Class: dns.ClassINET, Ttl: ttl},
					Flag:  uint8(caa.Flag),
					Tag:   caa.Tag,
					Value: caa.Domain,
				})
			}
		}
		if err := w.WriteMsg(&msg); err != nil {
			_ = s.logServiceError("ServeDNS(TypeCAA)", Utility.FileLine(), Utility.FunctionName(), err.Error())
		}

	case dns.TypeCNAME:
		msg := dns.Msg{}
		msg.SetReply(r)
		cname, ttl, err := s.getCName(msg.Question[0].Name)
		if err == nil {
			msg.Answer = append(msg.Answer, &dns.CNAME{
				Hdr:    dns.RR_Header{Name: msg.Question[0].Name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: ttl},
				Target: cname,
			})
		}
		if err := w.WriteMsg(&msg); err != nil {
			_ = s.logServiceError("ServeDNS(TypeCNAME)", Utility.FileLine(), Utility.FunctionName(), err.Error())
		}

	case dns.TypeTXT:
		values, ttl, err := s.getText(r.Question[0].Name)
		if err == nil {
			msg := new(dns.Msg)
			msg.SetReply(r)
			for _, txtValue := range values {
				msg.Answer = append(msg.Answer, &dns.TXT{
					Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: ttl},
					Txt: []string{txtValue},
				})
			}
			if err := w.WriteMsg(msg); err != nil {
				_ = s.logServiceError("ServeDNS(TypeTXT)", Utility.FileLine(), Utility.FunctionName(), err.Error())
			}
		}

	case dns.TypeNS:
		values, ttl, err := s.getNs(r.Question[0].Name)
		msg := new(dns.Msg)
		msg.SetReply(r)
		if err == nil {
			for _, ns := range values {
				msg.Answer = append(msg.Answer, &dns.NS{
					Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: ttl},
					Ns:  ns,
				})
			}
			if err := w.WriteMsg(msg); err != nil {
				_ = s.logServiceError("ServeDNS(TypeNS)", Utility.FileLine(), Utility.FunctionName(), err.Error())
			}
		}

	case dns.TypeMX:
		msg := dns.Msg{}
		msg.SetReply(r)
		mx := ""
		if len(msg.Question) > 1 {
			mx = msg.Question[1].Name
		}
		values, ttl, err := s.getMx(msg.Question[0].Name, mx)
		if err == nil {
			for _, mxr := range values {
				msg.Answer = append(msg.Answer, &dns.MX{
					Hdr:        dns.RR_Header{Name: msg.Question[0].Name, Rrtype: dns.TypeMX, Class: dns.ClassINET, Ttl: ttl},
					Preference: uint16(mxr.Preference),
					Mx:         mxr.Mx,
				})
			}
		}
		if err := w.WriteMsg(&msg); err != nil {
			_ = s.logServiceError("ServeDNS(TypeMX)", Utility.FileLine(), Utility.FunctionName(), err.Error())
		}

	case dns.TypeSOA:
		msg := dns.Msg{}
		msg.SetReply(r)
		ns := ""
		if len(msg.Question) > 1 {
			ns = msg.Question[1].Name
		}
		values, ttl, err := s.getSoa(msg.Question[0].Name, ns)
		if err == nil {
			domain := strings.ToLower(msg.Question[0].Name)
			if !strings.HasSuffix(domain, ".") {
				domain += "."
			}
			for _, soa := range values {
				if !strings.HasSuffix(soa.Mbox, ".") {
					soa.Mbox += "."
				}
				msg.Answer = append(msg.Answer, &dns.SOA{
					Hdr:     dns.RR_Header{Name: domain, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: ttl},
					Ns:      soa.Ns,
					Mbox:    soa.Mbox,
					Serial:  soa.Serial,
					Refresh: soa.Refresh,
					Retry:   soa.Retry,
					Expire:  soa.Expire,
					Minttl:  soa.Minttl,
				})
			}
		}
		if err := w.WriteMsg(&msg); err != nil {
			_ = s.logServiceError("ServeDNS(TypeSOA)", Utility.FileLine(), Utility.FunctionName(), err.Error())
		}

	case dns.TypeURI:
		msg := dns.Msg{}
		msg.SetReply(r)
		target := ""
		if len(msg.Question) > 1 {
			target = msg.Question[1].Name
		}
		values, ttl, err := s.getUri(msg.Question[0].Name, target)
		if err == nil {
			for _, uri := range values {
				msg.Answer = append(msg.Answer, &dns.URI{
					Hdr:      dns.RR_Header{Name: msg.Question[0].Name, Rrtype: dns.TypeURI, Class: dns.ClassINET, Ttl: ttl},
					Priority: uint16(uri.Priority),
					Weight:   uint16(uri.Weight),
					Target:   uri.Target,
				})
			}
		}
		if err := w.WriteMsg(&msg); err != nil {
			_ = s.logServiceError("ServeDNS(TypeURI)", Utility.FileLine(), Utility.FunctionName(), err.Error())
		}
	}
}

// ServeDns starts the UDP DNS server on the specified port.
func ServeDns(port int) error {
	srv := &dns.Server{Addr: ":" + strconv.Itoa(port), Net: "udp"}
	srv.Handler = &handler{}
	if err := srv.ListenAndServe(); err != nil {
		_ = s.logServiceError("ServeDns", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return err
	}
	return nil
}

/* ===============
   TTL persistence
   =============== */

// setTtl persists the TTL for a given record UUID.
func (srv *server) setTtl(uuid string, ttl uint32) error {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, ttl)
	uuid = Utility.GenerateUUID("TTL:" + uuid)
	return srv.store.SetItem(uuid, data)
}

// getTtl retrieves the TTL for a given record UUID, returning a default (60s) if none is set.
func (srv *server) getTtl(uuid string) uint32 {
	uuid = Utility.GenerateUUID("TTL:" + uuid)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return 60
	}
	return binary.LittleEndian.Uint32(data)
}

/* ======================
   Log service integration
   ====================== */

// GetLogClient returns a Log service client bound to this service's address.
func (srv *server) GetLogClient() (*log_client.Log_Client, error) {
	Utility.RegisterFunction("NewLogService_Client", log_client.NewLogService_Client)
	client, err := globular_client.GetClient(srv.Address, "log.LogService", "NewLogService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*log_client.Log_Client), nil
}

// logServiceInfo logs an informational event for this service via the log service.
func (srv *server) logServiceInfo(method, fileLine, functionName, infos string) error {
	log_client_, err := srv.GetLogClient()
	if err != nil {
		return err
	}
	return log_client_.Log(srv.Name, srv.Domain, method, logpb.LogLevel_INFO_MESSAGE, infos, fileLine, functionName)
}

// logServiceError logs an error event for this service via the log service.
func (srv *server) logServiceError(method, fileLine, functionName, infos string) error {
	log_client_, err := srv.GetLogClient()
	if err != nil {
		return err
	}
	// Use domain (not address) consistently to group service logs by domain.
	return log_client_.Log(srv.Name, srv.Domain, method, logpb.LogLevel_ERROR_MESSAGE, infos, fileLine, functionName)
}

/* ======================
   RBAC helper operations
   ====================== */

// GetRbacClient returns an RBAC client at a given address.
func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

// setActionResourcesPermissions sets permissions for action/resources on the RBAC service.
func (srv *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbac_client_.SetActionResourcesPermissions(permissions)
}

/* =====
   main
   ===== */

// main wires up the DNS service, starts the UDP DNS responder, and serves the gRPC API.
// Public API signatures (protobuf-generated service) are preserved exactly.
func main() {
	// Concrete server implementation.
	s_impl := new(server)
	Utility.RegisterType(s_impl)
	s_impl.Name = string(dnspb.File_dns_proto.Services().Get(0).FullName())
	s_impl.Proto = dnspb.File_dns_proto.Path()
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.DnsPort = 5353
	s_impl.PublisherID = "localhost"
	s_impl.Permissions = make([]interface{}, 6)
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true
	Utility.RegisterFunction("NewDnsService_Client", dns_client.NewDnsService_Client)

	// Local storage root.
	s_impl.Root = config.GetDataDir()
	Utility.CreateDirIfNotExist(s_impl.Root)

	// Default permissions for DNS operations on a given domain.
	s_impl.Permissions[0] = map[string]interface{}{"action": "/dns.DnsService/SetA", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[1] = map[string]interface{}{"action": "/dns.DnsService/SetAAAA", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[5] = map[string]interface{}{"action": "/dns.DnsService/SetText", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[2] = map[string]interface{}{"action": "/dns.DnsService/RemoveA", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}
	s_impl.Permissions[3] = map[string]interface{}{"action": "/dns.DnsService/RemoveAAAA", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}
	s_impl.Permissions[4] = map[string]interface{}{"action": "/dns.DnsService/RemoveText", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}

	// Parse optional args (id, config path).
	if len(os.Args) == 2 {
		s_impl.Id = os.Args[1]
	} else if len(os.Args) == 3 {
		s_impl.Id = os.Args[1]
		s_impl.ConfigPath = os.Args[2]
	}

	// Initialize service and gRPC server.
	if err := s_impl.Init(); err != nil {
		fmt.Printf("Fail to initialize service %s: %s", s_impl.Name, s_impl.Id)
	}

	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
	}

	// Start the UDP DNS responder.
	go func() {
		if err := ServeDns(s_impl.DnsPort); err != nil {
			_ = s_impl.logServiceError("ServeDns", Utility.FileLine(), Utility.FunctionName(), err.Error())
		}
	}()

	// Register gRPC endpoints and reflection.
	dnspb.RegisterDnsServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Open persistent store.
	if err := s_impl.openConnection(); err != nil {
		_ = s_impl.logServiceError("openConnection", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return
	}

	// Start the gRPC service.
	if err := s_impl.StartService(); err != nil {
		_ = s_impl.logServiceError("StartService", Utility.FileLine(), Utility.FunctionName(), err.Error())
	}
}
