package globular_service

import (
	"encoding/json"
	"testing"

	"github.com/globulario/services/golang/resource/resourcepb"
)

// minimalService is a lightweight stub that satisfies the Service interface for testing.
type minimalService struct {
	id      string
	name    string
	port    int
	domain  string
	address string
	version string
	// all other fields zero/nil — the interface requires setters/getters
}

func (s *minimalService) GetId() string                      { return s.id }
func (s *minimalService) SetId(v string)                     { s.id = v }
func (s *minimalService) GetName() string                    { return s.name }
func (s *minimalService) SetName(v string)                   { s.name = v }
func (s *minimalService) GetPort() int                       { return s.port }
func (s *minimalService) SetPort(v int)                      { s.port = v }
func (s *minimalService) GetDomain() string                  { return s.domain }
func (s *minimalService) SetDomain(v string)                 { s.domain = v }
func (s *minimalService) GetAddress() string                 { return s.address }
func (s *minimalService) SetAddress(v string)                { s.address = v }
func (s *minimalService) GetVersion() string                 { return s.version }
func (s *minimalService) SetVersion(v string)                { s.version = v }
func (s *minimalService) GetDescription() string             { return "" }
func (s *minimalService) SetDescription(string)              {}
func (s *minimalService) GetProxy() int                      { return 0 }
func (s *minimalService) SetProxy(int)                       {}
func (s *minimalService) GetProtocol() string                { return "" }
func (s *minimalService) SetProtocol(string)                 {}
func (s *minimalService) GetTls() bool                       { return false }
func (s *minimalService) SetTls(bool)                        {}
func (s *minimalService) GetCertFile() string                { return "" }
func (s *minimalService) SetCertFile(string)                 {}
func (s *minimalService) GetKeyFile() string                 { return "" }
func (s *minimalService) SetKeyFile(string)                  {}
func (s *minimalService) GetCertAuthorityTrust() string      { return "" }
func (s *minimalService) SetCertAuthorityTrust(string)       {}
func (s *minimalService) GetPublisherID() string             { return "" }
func (s *minimalService) SetPublisherID(string)              {}
func (s *minimalService) GetMac() string                     { return "" }
func (s *minimalService) SetMac(string)                      {}
func (s *minimalService) GetProcess() int                    { return 0 }
func (s *minimalService) SetProcess(int)                     {}
func (s *minimalService) GetProxyProcess() int               { return 0 }
func (s *minimalService) SetProxyProcess(int)                {}
func (s *minimalService) GetState() string                   { return "" }
func (s *minimalService) SetState(string)                    {}
func (s *minimalService) GetLastError() string               { return "" }
func (s *minimalService) SetLastError(string)                {}
func (s *minimalService) GetModTime() int64                  { return 0 }
func (s *minimalService) SetModTime(int64)                   {}
func (s *minimalService) GetKeepAlive() bool                 { return false }
func (s *minimalService) SetKeepAlive(bool)                  {}
func (s *minimalService) GetKeepUpToDate() bool              { return false }
func (s *minimalService) SetKeepUptoDate(bool)               {}
func (s *minimalService) GetAllowAllOrigins() bool           { return false }
func (s *minimalService) SetAllowAllOrigins(bool)            {}
func (s *minimalService) GetAllowedOrigins() string          { return "" }
func (s *minimalService) SetAllowedOrigins(string)           {}
func (s *minimalService) GetChecksum() string                { return "" }
func (s *minimalService) SetChecksum(string)                 {}
func (s *minimalService) GetPlatform() string                { return "" }
func (s *minimalService) SetPlatform(string)                 {}
func (s *minimalService) GetPath() string                    { return "" }
func (s *minimalService) SetPath(string)                     {}
func (s *minimalService) GetProto() string                   { return "" }
func (s *minimalService) SetProto(string)                    {}
func (s *minimalService) GetKeywords() []string              { return nil }
func (s *minimalService) SetKeywords([]string)               {}
func (s *minimalService) GetRepositories() []string          { return nil }
func (s *minimalService) SetRepositories([]string)           {}
func (s *minimalService) GetDiscoveries() []string           { return nil }
func (s *minimalService) SetDiscoveries([]string)            {}
func (s *minimalService) GetDependencies() []string          { return nil }
func (s *minimalService) SetDependency(string)               {}
func (s *minimalService) GetPermissions() []any              { return nil }
func (s *minimalService) SetPermissions([]any)               {}
func (s *minimalService) GetConfigurationPath() string       { return "" }
func (s *minimalService) SetConfigurationPath(string)        {}
func (s *minimalService) GetGrpcServer() interface{}      { return nil }
func (s *minimalService) Save() error                     { return nil }
func (s *minimalService) Init() error                     { return nil }
func (s *minimalService) StartService() error             { return nil }
func (s *minimalService) StopService() error              { return nil }
func (s *minimalService) Dist(string) (string, error)     { return "", nil }
func (s *minimalService) RolesDefault() []resourcepb.Role { return nil }

// TestEnsureDescribeID verifies that ensureDescribeID:
//   - assigns a non-empty deterministic UUID when Id is empty
//   - produces the same UUID on repeat calls with the same state
//   - preserves a pre-set Id
func TestEnsureDescribeID(t *testing.T) {
	t.Run("assigns stable UUID when empty", func(t *testing.T) {
		svc := &minimalService{name: "echo.EchoService", port: 10000, domain: "localhost"}
		ensureDescribeID(svc)
		id1 := svc.GetId()
		if id1 == "" {
			t.Fatal("Id must not be empty after ensureDescribeID")
		}
		// Reset and call again — must produce the same UUID.
		svc2 := &minimalService{name: "echo.EchoService", port: 10000, domain: "localhost"}
		ensureDescribeID(svc2)
		if svc2.GetId() != id1 {
			t.Errorf("Id not stable: first=%q second=%q", id1, svc2.GetId())
		}
	})

	t.Run("preserves existing Id", func(t *testing.T) {
		svc := &minimalService{id: "preset-id", name: "echo.EchoService"}
		ensureDescribeID(svc)
		if svc.GetId() != "preset-id" {
			t.Errorf("Id changed from preset: got %q", svc.GetId())
		}
	})
}

// TestDescribeMapRequiredFields verifies the --describe contract fields.
func TestDescribeMapRequiredFields(t *testing.T) {
	svc := &minimalService{name: "echo.EchoService", port: 10000, version: "0.0.1", domain: "localhost"}

	m := DescribeMap(svc)

	// Required installer contract.
	if id, _ := m["Id"].(string); id == "" {
		t.Error("DescribeMap: Id must be non-empty")
	}
	if dp, _ := m["DefaultPort"].(int); dp == 0 {
		// JSON round-trip uses float64; just check it's present.
		raw, _ := json.Marshal(m)
		var parsed map[string]any
		_ = json.Unmarshal(raw, &parsed)
		if parsed["DefaultPort"] == nil {
			t.Error("DescribeMap: DefaultPort must be present")
		}
	}
	if pr := m["PortRange"]; pr == nil {
		t.Error("DescribeMap: PortRange must be present")
	}
	if n, _ := m["Name"].(string); n == "" {
		t.Error("DescribeMap: Name must be non-empty")
	}
}

func TestNormalizeEndpointAddress(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"localhost:10003", "127.0.0.1:10003"},
		{"127.0.0.1:10003", "127.0.0.1:10003"},
		{"10.0.0.63:10003", "10.0.0.63:10003"},
		{"localhost", "127.0.0.1"},
		{"::1:8080", "::1:8080"}, // malformed (missing brackets), leave unchanged
		{"[::1]:8080", "127.0.0.1:8080"},
	}

	for _, tt := range tests {
		if got := NormalizeEndpointAddress(tt.in); got != tt.want {
			t.Fatalf("NormalizeEndpointAddress(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
