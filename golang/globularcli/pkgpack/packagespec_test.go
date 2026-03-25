package pkgpack

import (
	"strings"
	"testing"
)

func TestParseSpecBytes_ValidService(t *testing.T) {
	yaml := `
version: 1
metadata:
  name: authentication
  kind: service
  description: "Authentication service"
  keywords: [auth, identity]
  profiles: [core]
  priority: 100
  install_mode: repository
  managed_unit: true
  extra_binaries: [helper-tool]
  health_check:
    unit: globular-authentication.service
    port: 10101
service:
  name: authentication
  exec: authentication_server
steps:
  - id: ensure-dirs
    type: ensure_dirs
    dirs:
      - path: /var/lib/globular
  - id: install-payload
    type: install_package_payload
    install_bins: true
  - id: start-service
    type: start_services
    services:
      - globular-authentication.service
`
	spec, err := ParseSpecBytes([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("ParseSpecBytes: %v", err)
	}

	if spec.Version != 1 {
		t.Errorf("Version = %d, want 1", spec.Version)
	}
	if spec.Metadata.Name != "authentication" {
		t.Errorf("Name = %q, want %q", spec.Metadata.Name, "authentication")
	}
	if spec.Metadata.Kind != "service" {
		t.Errorf("Kind = %q, want %q", spec.Metadata.Kind, "service")
	}
	if spec.Metadata.Priority != 100 {
		t.Errorf("Priority = %d, want 100", spec.Metadata.Priority)
	}
	if len(spec.Metadata.ExtraBinaries) != 1 || spec.Metadata.ExtraBinaries[0] != "helper-tool" {
		t.Errorf("ExtraBinaries = %v, want [helper-tool]", spec.Metadata.ExtraBinaries)
	}
	if spec.Metadata.HealthCheck == nil {
		t.Fatal("HealthCheck is nil")
	}
	if spec.Metadata.HealthCheck.Port != 10101 {
		t.Errorf("HealthCheck.Port = %d, want 10101", spec.Metadata.HealthCheck.Port)
	}
	if spec.Service == nil {
		t.Fatal("Service block is nil")
	}
	if spec.Service.Exec != "authentication_server" {
		t.Errorf("Service.Exec = %q, want %q", spec.Service.Exec, "authentication_server")
	}
	if len(spec.Steps) != 3 {
		t.Fatalf("len(Steps) = %d, want 3", len(spec.Steps))
	}
	if spec.Steps[1].Type != "install_package_payload" {
		t.Errorf("Steps[1].Type = %q", spec.Steps[1].Type)
	}
	// Step args should capture type-specific fields.
	if v, ok := spec.Steps[1].Args["install_bins"]; !ok || v != true {
		t.Errorf("Steps[1].Args[install_bins] = %v, want true", v)
	}
}

func TestParseSpecBytes_MinimalCommand(t *testing.T) {
	yaml := `
version: 1
metadata:
  name: etcdctl
  kind: command
steps:
  - id: install-payload
    type: install_package_payload
    install_bins: true
`
	spec, err := ParseSpecBytes([]byte(yaml), "test.yaml")
	if err != nil {
		t.Fatalf("ParseSpecBytes: %v", err)
	}
	if spec.Service != nil {
		t.Error("Service should be nil for commands")
	}
	if spec.Metadata.Kind != "command" {
		t.Errorf("Kind = %q, want command", spec.Metadata.Kind)
	}
}

func TestValidateSpec_MissingName(t *testing.T) {
	// Use a path that can't derive a name (no recognizable filename pattern).
	spec := &PackageSpec{Version: 1, Steps: []InstallStep{{ID: "x", Type: "install_package_payload"}}}
	errs := ValidateSpec(spec, "")
	if len(errs) == 0 {
		t.Fatal("expected errors for missing name")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "metadata.name") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected metadata.name error, got: %v", errs)
	}
}

func TestValidateSpec_InvalidKind(t *testing.T) {
	spec := &PackageSpec{
		Version:  1,
		Metadata: PackageMetadata{Name: "test", Kind: "widget"},
		Steps:    []InstallStep{{ID: "x", Type: "install_package_payload"}},
	}
	errs := ValidateSpec(spec, "test.yaml")
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "not valid") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected kind validation error, got: %v", errs)
	}
}

func TestValidateSpec_DuplicateStepID(t *testing.T) {
	spec := &PackageSpec{
		Version:  1,
		Metadata: PackageMetadata{Name: "test"},
		Steps: []InstallStep{
			{ID: "step-a", Type: "ensure_dirs"},
			{ID: "step-a", Type: "install_package_payload"},
		},
	}
	errs := ValidateSpec(spec, "test.yaml")
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "duplicate id") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate id error, got: %v", errs)
	}
}

func TestValidateSpec_ServiceMissingPayloadStep(t *testing.T) {
	spec := &PackageSpec{
		Version:  1,
		Metadata: PackageMetadata{Name: "test", Kind: "service"},
		Steps:    []InstallStep{{ID: "x", Type: "ensure_dirs"}},
	}
	errs := ValidateSpec(spec, "test.yaml")
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "install_package_payload") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing install_package_payload error, got: %v", errs)
	}
}

func TestValidateSpec_InfraNoopEntrypoint(t *testing.T) {
	spec := &PackageSpec{
		Version:  1,
		Metadata: PackageMetadata{Name: "scylladb", Kind: "infrastructure", Entrypoint: "noop"},
		Steps:    []InstallStep{{ID: "x", Type: "ensure_dirs"}},
	}
	errs := ValidateSpec(spec, "test.yaml")
	for _, e := range errs {
		if strings.Contains(e.Error(), "install_package_payload") {
			t.Errorf("noop infrastructure should not require install_package_payload, got: %v", e)
		}
	}
}

func TestValidateSpec_NameFallbackToService(t *testing.T) {
	spec := &PackageSpec{
		Version: 1,
		Service: &ServiceBlock{Name: "dns"},
		Steps:   []InstallStep{{ID: "x", Type: "install_package_payload"}},
	}
	errs := ValidateSpec(spec, "test.yaml")
	for _, e := range errs {
		if strings.Contains(e.Error(), "metadata.name") {
			t.Errorf("name should fall back to service.name, got: %v", e)
		}
	}
}

func TestValidateSpec_InvalidInstallMode(t *testing.T) {
	spec := &PackageSpec{
		Version:  1,
		Metadata: PackageMetadata{Name: "test", InstallMode: "yolo"},
		Steps:    []InstallStep{{ID: "x", Type: "install_package_payload"}},
	}
	errs := ValidateSpec(spec, "test.yaml")
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "install_mode") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected install_mode error, got: %v", errs)
	}
}

func TestValidateSpec_EmptySteps(t *testing.T) {
	spec := &PackageSpec{
		Version:  1,
		Metadata: PackageMetadata{Name: "test"},
	}
	errs := ValidateSpec(spec, "test.yaml")
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "steps list is empty") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected empty steps error, got: %v", errs)
	}
}
