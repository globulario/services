package oci

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func validSpec() ServiceSpec {
	return ServiceSpec{
		APIVersion: APIVersionV1Alpha1,
		Kind:       KindOCIService,
		Metadata: Metadata{
			Name: "demo",
		},
		Spec: OCIServiceSpec{
			Image: ImageSpec{
				Repository: "registry.example.com/team/demo",
				Digest:     testDigest,
			},
			Security: SecuritySpec{
				NoNewPrivileges: true,
			},
		},
	}
}

func TestValidateAcceptsMinimalImmutableSpec(t *testing.T) {
	policy := DefaultPolicy()
	policy.AllowedRegistryHosts = []string{"registry.example.com"}
	if err := Validate(validSpec(), policy); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsUnsafeSurface(t *testing.T) {
	spec := validSpec()
	spec.Spec.Image.Digest = "latest"
	spec.Spec.Network.Mode = NetworkHost
	spec.Spec.Security.Privileged = true
	spec.Spec.Security.AddCapabilities = []string{"SYS_ADMIN"}
	spec.Spec.Mounts = []Mount{{Source: "/", Target: "/host"}, {Source: "/var/run/docker.sock", Target: "/var/run/docker.sock"}}
	err := Validate(spec, DefaultPolicy())
	if err == nil {
		t.Fatal("Validate() succeeded for unsafe spec")
	}
	for _, want := range []string{"immutable sha256", "host network", "privileged", "added Linux capabilities", "Docker socket", "host or container root"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Validate() error %q missing %q", err, want)
		}
	}
}

func TestSpecDigestStableAcrossMapOrder(t *testing.T) {
	a := validSpec()
	a.Metadata.Labels = map[string]string{"z": "last", "a": "first"}
	a.Spec.Security.DropCapabilities = []string{"NET_RAW", "MKNOD"}
	b := validSpec()
	b.Metadata.Labels = map[string]string{"a": "first", "z": "last"}
	b.Spec.Security.DropCapabilities = []string{"MKNOD", "NET_RAW"}
	da, err := a.SpecDigest()
	if err != nil {
		t.Fatal(err)
	}
	db, err := b.SpecDigest()
	if err != nil {
		t.Fatal(err)
	}
	if da != db {
		t.Fatalf("equivalent spec digests differ: %s != %s", da, db)
	}
}

func TestLoadSpecRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.json")
	payload := map[string]any{
		"api_version": APIVersionV1Alpha1,
		"kind":        KindOCIService,
		"metadata":    map[string]any{"name": "demo"},
		"spec": map[string]any{
			"image":    map[string]any{"repository": "example/demo", "digest": testDigest},
			"surprise": true,
		},
	}
	b, _ := json.Marshal(payload)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSpec(path); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("LoadSpec() error = %v, want unknown field", err)
	}
}

func TestReadSecretFileRequiresOwnerOnlyRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := os.WriteFile(path, []byte("token\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readSecretFile(path); err == nil {
		t.Fatal("readSecretFile() accepted broad permissions")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readSecretFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "token" {
		t.Fatalf("readSecretFile() = %q, want token", got)
	}
}

func TestValidateRejectsSecretOutsideNodeOwnedRoots(t *testing.T) {
	spec := validSpec()
	spec.Spec.Environment = []Environment{{Name: "TOKEN", ValueFile: "/root/.ssh/id_rsa"}}
	policy := DefaultPolicy()
	policy.AllowedRegistryHosts = []string{"registry.example.com"}
	err := Validate(spec, policy)
	if err == nil || !strings.Contains(err.Error(), "outside allowed secret roots") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPathPolicyRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if pathWithinAny(filepath.Join(link, "data"), []string{root}) {
		t.Fatal("pathWithinAny() admitted symlink escape")
	}
}
