package lifecycle

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/oci"
)

type fakeSupervisor struct {
	reloads   int
	stops     []string
	disables  []string
	reloadErr error
}

func (f *fakeSupervisor) DaemonReload(context.Context) error { f.reloads++; return f.reloadErr }
func (f *fakeSupervisor) Stop(_ context.Context, u string) error {
	f.stops = append(f.stops, u)
	return nil
}
func (f *fakeSupervisor) Disable(_ context.Context, u string) error {
	f.disables = append(f.disables, u)
	return nil
}

func TestInspectArtifactDistinguishesNativeAndValidatesOCI(t *testing.T) {
	dir := t.TempDir()
	native := filepath.Join(dir, "native.tgz")
	writeArchive(t, native, map[string]string{"bin/demo": "hello"})
	if _, found, err := InspectArtifact(native, "demo"); err != nil || found {
		t.Fatalf("native found=%t err=%v", found, err)
	}

	artifact := filepath.Join(dir, "oci.tgz")
	writeArchive(t, artifact, validFiles(t, "demo"))
	inspection, found, err := InspectArtifact(artifact, "demo")
	if err != nil || !found {
		t.Fatalf("found=%t err=%v", found, err)
	}
	if got := inspection.ServiceSpec.CanonicalImageReference(); !strings.Contains(got, "@sha256:") {
		t.Fatalf("image=%q", got)
	}
}

func TestInspectArtifactRejectsPackageOwnedUnitAndTraversal(t *testing.T) {
	dir := t.TempDir()
	files := validFiles(t, "demo")
	files["systemd/globular-demo.service"] = "[Service]"
	artifact := filepath.Join(dir, "unit.tgz")
	writeArchive(t, artifact, files)
	if _, found, err := InspectArtifact(artifact, "demo"); !found || err == nil || !strings.Contains(err.Error(), "may not ship a systemd unit") {
		t.Fatalf("found=%t err=%v", found, err)
	}

	artifact = filepath.Join(dir, "traversal.tgz")
	writeRawArchive(t, artifact, []archiveEntry{{name: ContractPath("demo"), body: validFiles(t, "demo")[ContractPath("demo")]}, {name: "data/oci/../../escape", body: "x"}, {name: DefaultSpecPath("demo"), body: validFiles(t, "demo")[DefaultSpecPath("demo")]}})
	if _, found, err := InspectArtifact(artifact, "demo"); !found || err == nil {
		t.Fatalf("found=%t err=%v", found, err)
	}
}

func TestInspectArtifactRejectsHostMutationPayloads(t *testing.T) {
	for _, forbidden := range []string{"bin/evil", "scripts/post-install.sh", "debs/evil.deb"} {
		t.Run(forbidden, func(t *testing.T) {
			files := validFiles(t, "demo")
			files[forbidden] = "payload"
			artifact := filepath.Join(t.TempDir(), "forbidden.tgz")
			writeArchive(t, artifact, files)
			if _, found, err := InspectArtifact(artifact, "demo"); !found || err == nil {
				t.Fatalf("found=%t err=%v", found, err)
			}
		})
	}
}

func TestManagerInstallVerifyAndUninstallPreservesObservedState(t *testing.T) {
	root := t.TempDir()
	layout := Layout{ConfigRoot: filepath.Join(root, "etc/services"), SystemdRoot: filepath.Join(root, "systemd"), StateRoot: filepath.Join(root, "state"), RunnerPath: filepath.Join(root, "bin/runner"), PolicyPath: filepath.Join(root, "etc/policy.json"), DockerSocket: "unix:///var/run/docker.sock"}
	artifact := filepath.Join(root, "oci.tgz")
	writeArchive(t, artifact, validFiles(t, "demo"))
	inspection, _, err := InspectArtifact(artifact, "demo")
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeSupervisor{}
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	manager := Manager{Layout: layout, Supervisor: fake, Now: func() time.Time { return now }}
	receipt, err := manager.Install(context.Background(), inspection, oci.DefaultPolicy(), "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if fake.reloads != 1 {
		t.Fatalf("reloads=%d", fake.reloads)
	}
	unitBytes, err := os.ReadFile(filepath.Join(layout.SystemdRoot, "globular-demo.service"))
	if err != nil {
		t.Fatal(err)
	}
	unit := string(unitBytes)
	for _, needle := range []string{"Type=notify", "docker.service", layout.RunnerPath, "Restart=on-failure", "ReadWritePaths="} {
		if !strings.Contains(unit, needle) {
			t.Fatalf("unit missing %q\n%s", needle, unit)
		}
	}
	if strings.Contains(unit, "docker run") {
		t.Fatal("unit shells out to docker")
	}
	if receipt.SpecDigest == "" || receipt.Image == "" {
		t.Fatalf("receipt=%+v", receipt)
	}

	observedDir := filepath.Join(layout.StateRoot, "demo", "default")
	if err := os.MkdirAll(observedDir, 0o750); err != nil {
		t.Fatal(err)
	}
	observed := oci.ObservedState{ServiceName: "demo", Instance: "default", Image: receipt.Image, SpecDigest: receipt.SpecDigest, Phase: oci.PhaseReady, Running: true, Ready: true, UpdatedAt: now}
	b, _ := json.Marshal(observed)
	if err := os.WriteFile(filepath.Join(observedDir, "observed.json"), b, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Verify("demo"); err != nil {
		t.Fatal(err)
	}

	managed, err := manager.Uninstall(context.Background(), "demo")
	if err != nil || !managed {
		t.Fatalf("managed=%t err=%v", managed, err)
	}
	if len(fake.stops) != 1 || fake.stops[0] != "globular-demo.service" {
		t.Fatalf("stops=%v", fake.stops)
	}
	if _, err := os.Stat(filepath.Join(observedDir, "observed.json")); err != nil {
		t.Fatalf("observed state deleted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(layout.SystemdRoot, "globular-demo.service")); !os.IsNotExist(err) {
		t.Fatalf("unit remains err=%v", err)
	}
}

func TestManagerRollsBackMaterializationWhenDaemonReloadFails(t *testing.T) {
	root := t.TempDir()
	layout := Layout{ConfigRoot: filepath.Join(root, "etc/services"), SystemdRoot: filepath.Join(root, "systemd"), StateRoot: filepath.Join(root, "state"), RunnerPath: filepath.Join(root, "bin/runner"), PolicyPath: filepath.Join(root, "etc/policy.json"), DockerSocket: "unix:///var/run/docker.sock"}
	artifact := filepath.Join(root, "oci.tgz")
	writeArchive(t, artifact, validFiles(t, "demo"))
	inspection, _, _ := InspectArtifact(artifact, "demo")
	unitPath := filepath.Join(layout.SystemdRoot, "globular-demo.service")
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unitPath, []byte("old-unit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &fakeSupervisor{reloadErr: errors.New("reload failed")}
	manager := Manager{Layout: layout, Supervisor: fake}
	if _, err := manager.Install(context.Background(), inspection, oci.DefaultPolicy(), "1.0.0"); err == nil {
		t.Fatal("expected failure")
	}
	b, err := os.ReadFile(unitPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "old-unit\n" {
		t.Fatalf("unit=%q", b)
	}
	if _, err := os.Stat(filepath.Join(layout.StateRoot, "package-receipts", "demo.json")); !os.IsNotExist(err) {
		t.Fatalf("receipt exists: %v", err)
	}
}

func validFiles(t *testing.T, service string) map[string]string {
	t.Helper()
	contract := PackageContract{APIVersion: PackageAPIVersionV1Alpha1, Kind: KindOCIServicePackage, ServiceName: service, Instance: "default", ServiceSpec: DefaultSpecPath(service)}
	spec := oci.ServiceSpec{APIVersion: oci.APIVersionV1Alpha1, Kind: oci.KindOCIService, Metadata: oci.Metadata{Name: service, Instance: "default"}, Spec: oci.OCIServiceSpec{Image: oci.ImageSpec{Repository: "registry.example.com/team/demo", Digest: "sha256:" + strings.Repeat("a", 64)}}}
	cb, _ := json.Marshal(contract)
	sb, _ := json.Marshal(spec)
	return map[string]string{ContractPath(service): string(cb), DefaultSpecPath(service): string(sb)}
}

type archiveEntry struct{ name, body string }

func writeArchive(t *testing.T, path string, files map[string]string) {
	t.Helper()
	entries := make([]archiveEntry, 0, len(files))
	for n, b := range files {
		entries = append(entries, archiveEntry{n, b})
	}
	writeRawArchive(t, path, entries)
}
func writeRawArchive(t *testing.T, p string, entries []archiveEntry) {
	t.Helper()
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.body))}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
