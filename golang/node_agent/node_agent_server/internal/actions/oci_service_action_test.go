package actions

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/globulario/services/golang/oci"
	ocilifecycle "github.com/globulario/services/golang/oci/lifecycle"
	"google.golang.org/protobuf/types/known/structpb"
)

var decoratorTestID atomic.Uint64

type fakeHandler struct {
	name    string
	applied int
}

func (f *fakeHandler) Name() string                    { return f.name }
func (f *fakeHandler) Validate(*structpb.Struct) error { return nil }
func (f *fakeHandler) Apply(context.Context, *structpb.Struct) (string, error) {
	f.applied++
	return "base applied", nil
}

func TestDecoratorIsRegistrationOrderIndependent(t *testing.T) {
	name := fmt.Sprintf("test.decorated.action.%d", decoratorTestID.Add(1))
	Decorate(name, func(next Handler) Handler { return markerHandler{next: next, marker: "wrapped"} })
	base := &fakeHandler{name: name}
	Register(base)
	result, err := Get(name).Apply(context.Background(), &structpb.Struct{})
	if err != nil {
		t.Fatal(err)
	}
	if result != "base applied; wrapped" {
		t.Fatalf("result=%q", result)
	}
}

type captureHandler struct {
	name   string
	fields map[string]*structpb.Value
}

func (h *captureHandler) Name() string                    { return h.name }
func (h *captureHandler) Validate(*structpb.Struct) error { return nil }
func (h *captureHandler) Apply(_ context.Context, args *structpb.Struct) (string, error) {
	h.fields = args.GetFields()
	return "captured", nil
}

type markerHandler struct {
	next   Handler
	marker string
}

func (h markerHandler) Name() string                      { return h.next.Name() }
func (h markerHandler) Validate(a *structpb.Struct) error { return h.next.Validate(a) }
func (h markerHandler) Apply(ctx context.Context, a *structpb.Struct) (string, error) {
	s, e := h.next.Apply(ctx, a)
	if e != nil {
		return "", e
	}
	return s + "; " + h.marker, nil
}

func TestOCIServiceActionsInstallVerifyAndUninstall(t *testing.T) {
	root := t.TempDir()
	oldBin, oldState, oldConfig, oldSystemd, oldSkip, oldReload, oldSocket := ActionBinDir, ActionStateDir, ActionConfigDir, ActionSystemdDir, ActionSkipSystemd, ActionSkipDaemonReload, ActionOCIDockerSocket
	snapshotActionRegistry(t)
	defer func() {
		ActionBinDir, ActionStateDir, ActionConfigDir, ActionSystemdDir, ActionSkipSystemd, ActionSkipDaemonReload, ActionOCIDockerSocket = oldBin, oldState, oldConfig, oldSystemd, oldSkip, oldReload, oldSocket
	}()
	ActionBinDir = filepath.Join(root, "bin")
	ActionStateDir = filepath.Join(root, "state")
	ActionConfigDir = filepath.Join(root, "etc")
	ActionSystemdDir = filepath.Join(root, "systemd")
	ActionSkipSystemd = true
	ActionSkipDaemonReload = true
	ActionOCIDockerSocket = "unix:///var/run/docker.sock"

	artifact := filepath.Join(root, "demo.tgz")
	writeOCIActionArchive(t, artifact, "demo")
	installBase := &fakeHandler{name: "service.install_payload"}
	Register(installBase)
	installArgs, _ := structpb.NewStruct(map[string]interface{}{"service": "demo", "artifact_path": artifact, "version": "1.0.0"})
	if err := Get("service.install_payload").Validate(installArgs); err != nil {
		t.Fatal(err)
	}
	result, err := Get("service.install_payload").Apply(context.Background(), installArgs)
	if err != nil {
		t.Fatal(err)
	}
	if installBase.applied != 1 || !strings.Contains(result, "OCI runtime materialized") {
		t.Fatalf("applied=%d result=%q", installBase.applied, result)
	}
	manager := actionOCIManager()
	receipt, err := manager.ReadReceipt("demo")
	if err != nil {
		t.Fatal(err)
	}
	reportBase := &captureHandler{name: "package.report_state"}
	Register(reportBase)
	reportArgs, _ := structpb.NewStruct(map[string]interface{}{"node_id": "node-1", "name": "demo", "version": "1.0.0", "kind": "SERVICE"})
	if _, err := Get("package.report_state").Apply(context.Background(), reportArgs); err != nil {
		t.Fatal(err)
	}
	if got := reportBase.fields["runtime_kind"].GetStringValue(); got != "oci" {
		t.Fatalf("runtime_kind=%q", got)
	}
	if got := reportBase.fields["oci_spec_digest"].GetStringValue(); got != receipt.SpecDigest {
		t.Fatalf("oci_spec_digest=%q want %q", got, receipt.SpecDigest)
	}

	observedDir := filepath.Join(ActionStateDir, "oci", "demo", "default")
	if err := os.MkdirAll(observedDir, 0o750); err != nil {
		t.Fatal(err)
	}
	observed := oci.ObservedState{ServiceName: "demo", Instance: "default", Image: receipt.Image, SpecDigest: receipt.SpecDigest, Phase: oci.PhaseReady, Running: true, Ready: true}
	b, _ := json.Marshal(observed)
	if err := os.WriteFile(filepath.Join(observedDir, "observed.json"), b, 0o640); err != nil {
		t.Fatal(err)
	}

	verifyBase := &fakeHandler{name: "package.verify"}
	Register(verifyBase)
	verifyArgs, _ := structpb.NewStruct(map[string]interface{}{"name": "demo", "kind": "SERVICE"})
	verified, err := Get("package.verify").Apply(context.Background(), verifyArgs)
	if err != nil {
		t.Fatal(err)
	}
	if verifyBase.applied != 0 || !strings.Contains(verified, "OCI package demo verified") {
		t.Fatalf("base=%d result=%q", verifyBase.applied, verified)
	}

	uninstallBase := &fakeHandler{name: "package.uninstall"}
	Register(uninstallBase)
	uninstallArgs, _ := structpb.NewStruct(map[string]interface{}{"name": "demo", "kind": "SERVICE"})
	removed, err := Get("package.uninstall").Apply(context.Background(), uninstallArgs)
	if err != nil {
		t.Fatal(err)
	}
	if uninstallBase.applied != 1 || !strings.Contains(removed, "persistent data retained") {
		t.Fatalf("base=%d result=%q", uninstallBase.applied, removed)
	}
	if _, err := os.Stat(filepath.Join(observedDir, "observed.json")); err != nil {
		t.Fatalf("observed deleted: %v", err)
	}
}

// snapshotActionRegistry restores the process-global action registry after the
// test. Register has no inverse, so a test that registers a fake under a real
// action name leaves that fake resolvable for every test that runs after it:
// TestServiceUninstall_* passed alone but failed in a full package run because
// package.uninstall still resolved to a fake registered here. Entries are
// restored directly rather than through Register so decorators already baked
// into the saved handlers are not applied a second time.
func snapshotActionRegistry(t *testing.T) {
	t.Helper()
	saved := make(map[string]Handler, len(registry))
	for name, handler := range registry {
		saved[name] = handler
	}
	t.Cleanup(func() {
		for name := range registry {
			delete(registry, name)
		}
		for name, handler := range saved {
			registry[name] = handler
		}
	})
}

func TestNativeServiceActionRemainsUnchanged(t *testing.T) {
	snapshotActionRegistry(t)
	root := t.TempDir()
	artifact := filepath.Join(root, "native.tgz")
	writeActionArchive(t, artifact, map[string]string{"bin/demo": "native"})
	base := &fakeHandler{name: "service.install_payload"}
	Register(base)
	args, _ := structpb.NewStruct(map[string]interface{}{"service": "demo", "artifact_path": artifact})
	result, err := Get("service.install_payload").Apply(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if base.applied != 1 || result != "base applied" {
		t.Fatalf("base=%d result=%q", base.applied, result)
	}
}

func writeOCIActionArchive(t *testing.T, p, service string) {
	t.Helper()
	contract := ocilifecycle.PackageContract{APIVersion: ocilifecycle.PackageAPIVersionV1Alpha1, Kind: ocilifecycle.KindOCIServicePackage, ServiceName: service, Instance: "default", ServiceSpec: ocilifecycle.DefaultSpecPath(service)}
	spec := oci.ServiceSpec{APIVersion: oci.APIVersionV1Alpha1, Kind: oci.KindOCIService, Metadata: oci.Metadata{Name: service, Instance: "default"}, Spec: oci.OCIServiceSpec{Image: oci.ImageSpec{Repository: "registry.example.com/team/demo", Digest: "sha256:" + strings.Repeat("b", 64)}}}
	cb, _ := json.Marshal(contract)
	sb, _ := json.Marshal(spec)
	writeActionArchive(t, p, map[string]string{ocilifecycle.ContractPath(service): string(cb), ocilifecycle.DefaultSpecPath(service): string(sb)})
}
func writeActionArchive(t *testing.T, p string, files map[string]string) {
	t.Helper()
	f, e := os.Create(p)
	if e != nil {
		t.Fatal(e)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for n, b := range files {
		if e := tw.WriteHeader(&tar.Header{Name: n, Mode: 0o644, Size: int64(len(b))}); e != nil {
			t.Fatal(e)
		}
		if _, e := tw.Write([]byte(b)); e != nil {
			t.Fatal(e)
		}
	}
	if e := tw.Close(); e != nil {
		t.Fatal(e)
	}
	if e := gz.Close(); e != nil {
		t.Fatal(e)
	}
	if e := f.Close(); e != nil {
		t.Fatal(e)
	}
}
