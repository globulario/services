package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDoctorCLIHasNoHardcodedInsecureTLS ensures doctor CLI dials through the
// shared secure dial path instead of hardcoding InsecureSkipVerify=true.
func TestDoctorCLIHasNoHardcodedInsecureTLS(t *testing.T) {
	files := []string{
		"doctor_report_cmd.go",
		"doctor_remediate_cmd.go",
	}
	for _, name := range files {
		path := filepath.Join(".", name)
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		src := string(b)
		if strings.Contains(src, "InsecureSkipVerify: true") {
			t.Fatalf("%s contains hardcoded InsecureSkipVerify=true; use dialGRPC + --insecure policy", name)
		}
		if !strings.Contains(src, "dialGRPC(") {
			t.Fatalf("%s must use dialGRPC for centralized TLS policy", name)
		}
	}
}

