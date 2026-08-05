package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateCommand(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "service.json")
	content := `{
  "api_version":"globular.io/oci/v1alpha1",
  "kind":"OCIService",
  "metadata":{"name":"demo"},
  "spec":{"image":{"repository":"example/demo","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
}`
	if err := os.WriteFile(specPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := run([]string{"validate", "--spec", specPath}, &stdout, &stderr); err != nil {
		t.Fatalf("run() error = %v, stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"valid": true`) || !strings.Contains(stdout.String(), `"container_name": "globular-demo"`) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestMissingSpecIsRefused(t *testing.T) {
	if err := run([]string{"validate"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("run() accepted missing --spec")
	}
}
