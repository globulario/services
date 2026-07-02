package globular_service

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/policy"
	"google.golang.org/grpc"
)

type lifecycleTestService struct {
	grpcServer *grpc.Server
	state      string
	t          *testing.T
}

func (s *lifecycleTestService) GetId() string    { return "lifecycle-test" }
func (s *lifecycleTestService) GetName() string  { return "lifecycle_test.TestService" }
func (s *lifecycleTestService) GetPort() int     { return 12345 }
func (s *lifecycleTestService) GetState() string { return s.state }
func (s *lifecycleTestService) SetState(v string) {
	s.state = v
}
func (s *lifecycleTestService) GetGrpcServer() *grpc.Server  { return s.grpcServer }
func (s *lifecycleTestService) StopService() error           { return nil }
func (s *lifecycleTestService) SetPermissions([]interface{}) {}

func (s *lifecycleTestService) StartService() error {
	if got := policy.GlobalResolver().Resolve("/lifecycle_test.TestService/Start"); got != "lifecycle_test.start" {
		s.t.Fatalf("permission mappings must be loaded before StartService; Resolve returned %q", got)
	}
	return nil
}

func TestLifecycleStartLoadsPermissionsBeforeStartService(t *testing.T) {
	tmp := t.TempDir()
	oldAdminRoot, oldPackageRoot := policy.AdminRoot, policy.PackageRoot
	policy.AdminRoot = filepath.Join(tmp, "etc")
	policy.PackageRoot = filepath.Join(tmp, "var")
	t.Cleanup(func() {
		policy.AdminRoot = oldAdminRoot
		policy.PackageRoot = oldPackageRoot
	})

	svcDir := filepath.Join(policy.PackageRoot, "services", "lifecycle_test")
	if err := os.MkdirAll(svcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
		"schema_version": "2",
		"service": "lifecycle_test.TestService",
		"permissions": [
			{"method": "/lifecycle_test.TestService/Start", "action": "lifecycle_test.start", "permission": "admin", "resources": []}
		]
	}`
	if err := os.WriteFile(filepath.Join(svcDir, "permissions.generated.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := &lifecycleTestService{grpcServer: grpc.NewServer(), t: t}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := NewLifecycleManager(svc, logger).Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if svc.GetState() != "running" {
		t.Fatalf("state = %q, want running", svc.GetState())
	}
}
