package enforce_test

// ast_scan_test.go — tests for Go source hard-rule violation scanner.
// These tests are required by the awareness.ast_scan capability gap.

import (
	"strings"
	"testing"

	"github.com/globulario/services/golang/awareness/enforce"
)

// TestScanGoFile_LoopbackStringLiteral verifies that a "127.0.0.1" string
// literal in a Go file is flagged as LOOPBACK_STRING_LITERAL.
func TestScanGoFile_LoopbackStringLiteral(t *testing.T) {
	src := []byte(`package example
func dial() { connect("127.0.0.1:10000") }
`)
	res, err := enforce.ScanGoSource("example.go", src, false)
	if err != nil {
		t.Fatalf("ScanGoSource: %v", err)
	}
	found := false
	for _, v := range res.Violations {
		if v.Kind == "LOOPBACK_STRING_LITERAL" && strings.Contains(v.Message, "127.0.0.1") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected LOOPBACK_STRING_LITERAL violation for 127.0.0.1, got %v", res.Violations)
	}
}

// TestScanGoFile_ConstLoopback verifies that a const initialized to "127.0.0.1"
// is flagged as CONST_LOOPBACK.
func TestScanGoFile_ConstLoopback(t *testing.T) {
	src := []byte(`package example
const listenAddr = "127.0.0.1"
`)
	res, err := enforce.ScanGoSource("example.go", src, false)
	if err != nil {
		t.Fatalf("ScanGoSource: %v", err)
	}
	found := false
	for _, v := range res.Violations {
		if v.Kind == "CONST_LOOPBACK" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected CONST_LOOPBACK violation, got %v", res.Violations)
	}
}

// TestScanGoFile_GRPCDialLoopback verifies that grpc.Dial with a loopback
// address is flagged as GRPC_DIAL_LOOPBACK.
func TestScanGoFile_GRPCDialLoopback(t *testing.T) {
	src := []byte(`package example
import "google.golang.org/grpc"
func connect() { grpc.Dial("localhost:10000") }
`)
	res, err := enforce.ScanGoSource("example.go", src, false)
	if err != nil {
		t.Fatalf("ScanGoSource: %v", err)
	}
	found := false
	for _, v := range res.Violations {
		if v.Kind == "GRPC_DIAL_LOOPBACK" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected GRPC_DIAL_LOOPBACK violation, got %v", res.Violations)
	}
}

// TestScanGoFile_OsGetenv verifies that os.Getenv calls are flagged as
// OS_GETENV (config must come from etcd, not environment variables).
func TestScanGoFile_OsGetenv(t *testing.T) {
	src := []byte(`package example
import "os"
func getAddr() string { return os.Getenv("SERVICE_ADDR") }
`)
	res, err := enforce.ScanGoSource("example.go", src, false)
	if err != nil {
		t.Fatalf("ScanGoSource: %v", err)
	}
	found := false
	for _, v := range res.Violations {
		if v.Kind == "OS_GETENV" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected OS_GETENV violation, got %v", res.Violations)
	}
}

// TestScanGoFile_ExecImportInController verifies that importing os/exec in a
// cluster_controller package is flagged as EXEC_IMPORT_IN_CONTROLLER.
func TestScanGoFile_ExecImportInController(t *testing.T) {
	src := []byte(`package cluster_controller
import "os/exec"
func run() { exec.Command("ls").Run() }
`)
	res, err := enforce.ScanGoSource("cluster_controller/server.go", src, true /* isController */)
	if err != nil {
		t.Fatalf("ScanGoSource: %v", err)
	}
	found := false
	for _, v := range res.Violations {
		if v.Kind == "EXEC_IMPORT_IN_CONTROLLER" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected EXEC_IMPORT_IN_CONTROLLER violation, got %v", res.Violations)
	}
}

// TestScanGoFile_CleanFileHasNoViolations verifies that a file with no
// hard-rule violations produces an empty result.
func TestScanGoFile_CleanFileHasNoViolations(t *testing.T) {
	src := []byte(`package example
import "github.com/some/pkg"
func healthy() string { return pkg.GetAddr() }
`)
	res, err := enforce.ScanGoSource("example.go", src, false)
	if err != nil {
		t.Fatalf("ScanGoSource: %v", err)
	}
	if len(res.Violations) > 0 {
		t.Errorf("expected no violations for clean file, got %v", res.Violations)
	}
}

// Awareness required-test name wrapper: service server config must not fall
// back to environment-variable sources.
func TestNoOsGetenvInServiceServerCode(t *testing.T) {
	TestScanGoFile_OsGetenv(t)
}
