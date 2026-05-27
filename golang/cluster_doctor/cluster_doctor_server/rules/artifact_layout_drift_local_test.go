package rules

import (
	"os"
	"path/filepath"
	"testing"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

func TestArtifactLayoutDriftLocal_UnexpectedEntry_Warns(t *testing.T) {
	td := t.TempDir()
	if err := os.MkdirAll(filepath.Join(td, "pki"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(td, "etcd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(td, "authentication"), 0o755); err != nil {
		t.Fatal(err)
	}

	old := artifactStateRootPath
	artifactStateRootPath = td
	t.Cleanup(func() { artifactStateRootPath = old })

	findings := (artifactLayoutDriftLocal{}).Evaluate(nil, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Fatalf("expected WARN, got %v", findings[0].Severity)
	}
}

func TestArtifactLayoutDriftLocal_AllowlistOnly_Silent(t *testing.T) {
	td := t.TempDir()
	for _, d := range []string{"pki", "etcd", "config", "services", "repository"} {
		if err := os.MkdirAll(filepath.Join(td, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	old := artifactStateRootPath
	artifactStateRootPath = td
	t.Cleanup(func() { artifactStateRootPath = old })

	findings := (artifactLayoutDriftLocal{}).Evaluate(nil, testConfig())
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(findings), findings)
	}
}
