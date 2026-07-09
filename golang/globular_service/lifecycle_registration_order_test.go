package globular_service

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/policy"
	"google.golang.org/grpc"
)

// ordMock is a minimal LifecycleService that records what the RBAC action
// resolver returned for a method AT THE MOMENT serving began (StartService).
type ordMock struct {
	name            string
	grpc            *grpc.Server
	state           string
	startCalled     bool
	resolvedAtStart string
}

func (m *ordMock) GetId() string                { return "ord-id" }
func (m *ordMock) GetName() string              { return m.name }
func (m *ordMock) GetPort() int                 { return 0 }
func (m *ordMock) GetState() string             { return m.state }
func (m *ordMock) SetState(s string)            { m.state = s }
func (m *ordMock) StopService() error           { return nil }
func (m *ordMock) GetGrpcServer() *grpc.Server  { return m.grpc }
func (m *ordMock) SetPermissions(_ []interface{}) {}

// StartService stands in for the real (blocking) serve loop. It records what the
// global resolver returns for a known method — proving whether registration
// already happened before serving began.
func (m *ordMock) StartService() error {
	m.startCalled = true
	m.resolvedAtStart = policy.GlobalResolver().Resolve("/ordtest.OrdTest/Ping")
	return nil
}

// TestPermissionsRegisteredBeforeStartService is the regression guard for the
// v1.2.267 empty-resolver root cause: globular.StartService() blocks until a
// termination signal, so LoadAndRegisterPermissions MUST run before it —
// otherwise it is dead code and the serving process resolves raw method paths
// and denies every role-based call (repository/workflow symptom). The mock
// records what the resolver returned at the instant serving began; if
// registration ran first, the method already resolves to its fine action key.
func TestPermissionsRegisteredBeforeStartService(t *testing.T) {
	tmp := t.TempDir()
	oldRoot := policy.PackageRoot
	policy.PackageRoot = tmp
	t.Cleanup(func() { policy.PackageRoot = oldRoot })

	dir := filepath.Join(tmp, "services", "ordtest")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	perm := `{"schema_version":"2","service":"ordtest.OrdTest","permissions":[` +
		`{"method":"/ordtest.OrdTest/Ping","action":"ordtest.ping.read","permission":"read"}]}`
	if err := os.WriteFile(filepath.Join(dir, "permissions.generated.json"), []byte(perm), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &ordMock{name: "ordtest.OrdTest", grpc: grpc.NewServer()}
	lm := NewLifecycleManager(m, slog.Default())
	if err := lm.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if !m.startCalled {
		t.Fatal("StartService was never called")
	}
	if m.resolvedAtStart != "ordtest.ping.read" {
		t.Fatalf("resolver returned %q when serving began — registration ran AFTER the blocking "+
			"StartService (the empty-resolver bug). Want the fine action key %q.",
			m.resolvedAtStart, "ordtest.ping.read")
	}
}
