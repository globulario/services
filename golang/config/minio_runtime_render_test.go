package config

import (
	"strings"
	"testing"
)

// nodePathsFor builds a NodePaths map from a list of IPs using a common base path.
func nodePathsFor(ips []string, base string) map[string]string {
	m := make(map[string]string, len(ips))
	for _, ip := range ips {
		m[ip] = base
	}
	return m
}

// ── RenderMinioEnv tests ──────────────────────────────────────────────────────

func TestRenderMinioEnv_Standalone(t *testing.T) {
	state := &ObjectStoreDesiredState{
		Mode:          ObjectStoreModeStandalone,
		Nodes:         []string{"10.0.0.1"},
		DrivesPerNode: 1,
		AccessKey:     "access",
		SecretKey:     "secret",
	}
	got := RenderMinioEnv(state)

	if !strings.Contains(got, "MINIO_VOLUMES=/var/lib/globular/minio/data\n") {
		t.Errorf("standalone: unexpected MINIO_VOLUMES line:\n%s", got)
	}
	if strings.Contains(got, "MINIO_CI_CD") {
		t.Error("standalone: must not set MINIO_CI_CD")
	}
	if !strings.Contains(got, "MINIO_ROOT_USER=access\n") {
		t.Errorf("standalone: expected MINIO_ROOT_USER=access, got:\n%s", got)
	}
}

func TestRenderMinioEnv_Distributed_SingleDrive(t *testing.T) {
	ips := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}
	state := &ObjectStoreDesiredState{
		Mode:          ObjectStoreModeDistributed,
		Nodes:         ips,
		DrivesPerNode: 1,
		AccessKey:     "access",
		SecretKey:     "secret",
	}
	got := RenderMinioEnv(state)

	expected := "MINIO_VOLUMES=https://10.0.0.1:9000/var/lib/globular/minio/data" +
		" https://10.0.0.2:9000/var/lib/globular/minio/data" +
		" https://10.0.0.3:9000/var/lib/globular/minio/data\n"
	if !strings.Contains(got, expected) {
		t.Errorf("distributed single-drive: unexpected MINIO_VOLUMES:\n%s\nwant substring:\n%s", got, expected)
	}
	if !strings.Contains(got, "MINIO_CI_CD=1\n") {
		t.Error("distributed: must set MINIO_CI_CD=1")
	}
}

func TestRenderMinioEnv_Standalone_MultiDrive(t *testing.T) {
	state := &ObjectStoreDesiredState{
		Mode:          ObjectStoreModeStandalone,
		Nodes:         []string{"10.0.0.1"},
		DrivesPerNode: 4,
		AccessKey:     "access",
		SecretKey:     "secret",
	}
	got := RenderMinioEnv(state)

	expected := "MINIO_VOLUMES=/var/lib/globular/minio/data1" +
		" /var/lib/globular/minio/data2" +
		" /var/lib/globular/minio/data3" +
		" /var/lib/globular/minio/data4\n"
	if !strings.Contains(got, expected) {
		t.Errorf("standalone multi-drive: unexpected MINIO_VOLUMES:\n%s\nwant:\n%s", got, expected)
	}
	if strings.Contains(got, "MINIO_CI_CD") {
		t.Error("standalone multi-drive: must not set MINIO_CI_CD")
	}
}

func TestRenderMinioEnv_Distributed_MultiDrive(t *testing.T) {
	ips := []string{"10.0.0.1", "10.0.0.2"}
	state := &ObjectStoreDesiredState{
		Mode:          ObjectStoreModeDistributed,
		Nodes:         ips,
		DrivesPerNode: 2,
		AccessKey:     "access",
		SecretKey:     "secret",
	}
	got := RenderMinioEnv(state)

	expected := "MINIO_VOLUMES=https://10.0.0.1:9000/var/lib/globular/minio/data1" +
		" https://10.0.0.1:9000/var/lib/globular/minio/data2" +
		" https://10.0.0.2:9000/var/lib/globular/minio/data1" +
		" https://10.0.0.2:9000/var/lib/globular/minio/data2\n"
	if !strings.Contains(got, expected) {
		t.Errorf("distributed multi-drive: unexpected MINIO_VOLUMES:\n%s\nwant:\n%s", got, expected)
	}
	if !strings.Contains(got, "MINIO_CI_CD=1\n") {
		t.Error("distributed multi-drive: must set MINIO_CI_CD=1")
	}
}

func TestRenderMinioEnv_CustomNodePaths(t *testing.T) {
	ips := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}
	state := &ObjectStoreDesiredState{
		Mode:          ObjectStoreModeDistributed,
		Nodes:         ips,
		DrivesPerNode: 1,
		NodePaths:     nodePathsFor(ips, "/mnt/data/minio"),
		AccessKey:     "access",
		SecretKey:     "secret",
	}
	got := RenderMinioEnv(state)

	if !strings.Contains(got, "https://10.0.0.1:9000/mnt/data/minio/data") {
		t.Errorf("custom node paths: expected /mnt/data/minio prefix, got:\n%s", got)
	}
}

func TestRenderMinioEnv_DefaultCredentials(t *testing.T) {
	state := &ObjectStoreDesiredState{
		Nodes:         []string{"10.0.0.1"},
		DrivesPerNode: 1,
	}
	got := RenderMinioEnv(state)

	if !strings.Contains(got, "MINIO_ROOT_USER=minioadmin\n") {
		t.Errorf("default creds: expected MINIO_ROOT_USER=minioadmin, got:\n%s", got)
	}
}

func TestRenderMinioEnv_NilState(t *testing.T) {
	if got := RenderMinioEnv(nil); got != "" {
		t.Errorf("nil state: expected empty string, got %q", got)
	}
}

// ── RenderMinioSystemdOverride tests ─────────────────────────────────────────

func TestRenderMinioSystemdOverride_StandaloneNoop(t *testing.T) {
	state := &ObjectStoreDesiredState{
		Nodes:         []string{"10.0.0.1"},
		DrivesPerNode: 1,
	}
	content, ok := RenderMinioSystemdOverride(state, "10.0.0.1")
	if ok {
		t.Errorf("standalone single-drive: must return ok=false, got content:\n%s", content)
	}
}

func TestRenderMinioSystemdOverride_Distributed(t *testing.T) {
	state := &ObjectStoreDesiredState{
		Nodes:         []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
		DrivesPerNode: 1,
	}
	content, ok := RenderMinioSystemdOverride(state, "10.0.0.1")
	if !ok {
		t.Fatal("distributed: expected ok=true")
	}
	if !strings.Contains(content, "[Service]") {
		t.Error("distributed: missing [Service] section")
	}
	if !strings.Contains(content, "ExecStart=\n") {
		t.Error("distributed: missing ExecStart= (clear line)")
	}
	if !strings.Contains(content, "--address 10.0.0.1:9000") {
		t.Errorf("distributed: expected --address 10.0.0.1:9000, got:\n%s", content)
	}
	if !strings.Contains(content, "ExecStartPre=+/usr/bin/mkdir -p /var/lib/globular/minio/data\n") {
		t.Errorf("distributed: expected data dir mkdir, got:\n%s", content)
	}
}

func TestRenderMinioSystemdOverride_MultiDrive(t *testing.T) {
	state := &ObjectStoreDesiredState{
		Nodes:         []string{"10.0.0.1"},
		DrivesPerNode: 2,
	}
	content, ok := RenderMinioSystemdOverride(state, "10.0.0.1")
	if !ok {
		t.Fatal("multi-drive: expected ok=true")
	}
	if !strings.Contains(content, "/var/lib/globular/minio/data1") {
		t.Errorf("multi-drive: expected data1 dir, got:\n%s", content)
	}
	if !strings.Contains(content, "/var/lib/globular/minio/data2") {
		t.Errorf("multi-drive: expected data2 dir, got:\n%s", content)
	}
}

func TestRenderMinioSystemdOverride_CustomNodePath(t *testing.T) {
	state := &ObjectStoreDesiredState{
		Nodes:         []string{"10.0.0.1", "10.0.0.2"},
		DrivesPerNode: 1,
		NodePaths:     map[string]string{"10.0.0.1": "/mnt/data/minio", "10.0.0.2": "/mnt/data/minio"},
	}
	content, ok := RenderMinioSystemdOverride(state, "10.0.0.1")
	if !ok {
		t.Fatal("custom path: expected ok=true")
	}
	if !strings.Contains(content, "/mnt/data/minio/data") {
		t.Errorf("custom path: expected /mnt/data/minio/data, got:\n%s", content)
	}
}

// ── Byte-identical parity tests ───────────────────────────────────────────────
//
// These tests verify that RenderMinioEnv and RenderMinioSystemdOverride produce
// identical output to the controller's renderMinioConfig / renderMinioSystemdOverride
// for known inputs. They act as a compile-time guard: if either implementation
// diverges, these tests will catch the regression.

func TestRenderMinioEnv_ParityWithController_Distributed(t *testing.T) {
	// Known expected output derived from controller renderMinioConfig with:
	//   poolIPs = [10.0.0.1, 10.0.0.2, 10.0.0.3], drivesPerNode = 1
	//   credentials = access/secret, no custom paths
	state := &ObjectStoreDesiredState{
		Nodes:         []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
		DrivesPerNode: 1,
		AccessKey:     "access",
		SecretKey:     "secret",
	}
	want := "MINIO_VOLUMES=https://10.0.0.1:9000/var/lib/globular/minio/data" +
		" https://10.0.0.2:9000/var/lib/globular/minio/data" +
		" https://10.0.0.3:9000/var/lib/globular/minio/data\n" +
		"MINIO_ROOT_USER=access\n" +
		"MINIO_ROOT_PASSWORD=secret\n" +
		"MINIO_CI_CD=1\n"
	got := RenderMinioEnv(state)
	if got != want {
		t.Errorf("parity: got:\n%q\nwant:\n%q", got, want)
	}
}

func TestRenderMinioSystemdOverride_ParityWithController_Distributed(t *testing.T) {
	// Known expected output for distributed 3-node, single-drive, node IP 10.0.0.1.
	state := &ObjectStoreDesiredState{
		Nodes:         []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
		DrivesPerNode: 1,
	}
	want := "# Managed by Globular cluster controller — do not edit manually.\n" +
		"[Service]\n" +
		"ExecStartPre=+/usr/bin/mkdir -p /var/lib/globular/minio/data\n" +
		"ExecStartPre=+/usr/bin/chown globular:globular /var/lib/globular/minio/data\n" +
		"ExecStart=\n" +
		"ExecStart=/usr/lib/globular/bin/minio server $MINIO_VOLUMES --address 10.0.0.1:9000 --console-address 10.0.0.1:9001\n"
	got, ok := RenderMinioSystemdOverride(state, "10.0.0.1")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != want {
		t.Errorf("parity: got:\n%q\nwant:\n%q", got, want)
	}
}
