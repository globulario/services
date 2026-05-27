package intentaudit

import (
	"os"
	"path/filepath"
	"testing"
)

func writeGoFile(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name)
	parent := filepath.Dir(full)
	if err := os.MkdirAll(parent, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestASTScan_OsGetenv(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "config/loader.go", `package config

import "os"

func Load() string {
	return os.Getenv("MY_VAR")
}
`)
	findings, err := ScanGoAST(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range findings {
		if f.Pattern == "os.Getenv" && f.IntentID == "etcd.is_source_of_truth" && f.Source == "ast" {
			found = true
			if f.Line == 0 {
				t.Error("expected non-zero line number")
			}
		}
	}
	if !found {
		t.Errorf("expected os.Getenv finding, got %v", findings)
	}
}

func TestASTScan_SkipsTestFiles(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "config/loader_test.go", `package config

import (
	"os"
	"testing"
)

func TestLoader(t *testing.T) {
	os.Getenv("TEST_VAR")
}
`)
	findings, err := ScanGoAST(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if f.Pattern == "os.Getenv" {
			t.Errorf("should skip test files, but found %+v", f)
		}
	}
}

func TestASTScan_SkipsGeneratedFiles(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "api/service.pb.go", `package api

import "os"

func init() {
	os.Getenv("GEN_VAR")
}
`)
	writeGoFile(t, dir, "api/types_generated.go", `package api

import "os"

func init() {
	os.Getenv("GEN_VAR2")
}
`)
	findings, err := ScanGoAST(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if f.Pattern == "os.Getenv" {
			t.Errorf("should skip generated files, but found %+v", f)
		}
	}
}

func TestASTScan_ExecCommand(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "controller/run.go", `package controller

import "os/exec"

func Run() {
	exec.Command("ls", "-la")
}
`)
	findings, err := ScanGoAST(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	intentIDs := make(map[string]bool)
	for _, f := range findings {
		if f.Pattern == "exec.Command" && f.Source == "ast" {
			intentIDs[f.IntentID] = true
		}
	}
	if !intentIDs["controller.decides_but_does_not_execute_leaf_work"] {
		t.Error("missing intent controller.decides_but_does_not_execute_leaf_work")
	}
	if !intentIDs["workflow.source_of_operational_truth"] {
		t.Error("missing intent workflow.source_of_operational_truth")
	}
}

func TestASTScan_ExecCommandContext(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "svc/worker.go", `package svc

import (
	"context"
	"os/exec"
)

func Work(ctx context.Context) {
	exec.CommandContext(ctx, "date")
}
`)
	findings, err := ScanGoAST(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range findings {
		if f.Pattern == "exec.CommandContext" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected exec.CommandContext finding, got %v", findings)
	}
}

func TestASTScan_SkipsNodeAgent(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "node_agent/supervisor/exec.go", `package supervisor

import "os/exec"

func Run() {
	exec.Command("systemctl", "restart", "foo")
}
`)
	findings, err := ScanGoAST(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if f.Pattern == "exec.Command" || f.Pattern == "exec.CommandContext" {
			t.Errorf("should skip node_agent, but found %+v", f)
		}
	}
}

func TestASTScan_ExceptionSuppresses(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "security/bootstrap.go", `package security

import "os"

func Bootstrap() string {
	return os.Getenv("BOOTSTRAP_TOKEN")
}
`)
	exceptions := []Exception{
		{
			Name:        "bootstrap_tls",
			ID:          "exc-bootstrap",
			Description: "Bootstrap bypass",
			Files:       []string{"security/bootstrap.go"},
		},
	}
	findings, err := ScanGoAST(dir, exceptions)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if f.Pattern == "os.Getenv" {
			t.Errorf("exception should suppress finding, but found %+v", f)
		}
	}
}

func TestASTScan_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	findings, err := ScanGoAST(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty dir, got %d", len(findings))
	}
}
