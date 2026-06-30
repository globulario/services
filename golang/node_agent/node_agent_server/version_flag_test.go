package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestPrintVersionReportsEmbeddedMetadata(t *testing.T) {
	oldVersion := Version
	oldBuildTime := BuildTime
	oldGitCommit := GitCommit
	defer func() {
		Version = oldVersion
		BuildTime = oldBuildTime
		GitCommit = oldGitCommit
	}()

	Version = "9.9.9-test"
	BuildTime = "2026-06-30T00:00:00Z"
	GitCommit = "deadbeef"

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	printVersion()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	_ = r.Close()

	var payload map[string]string
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("printVersion output is not valid JSON: %v\n%s", err, buf.String())
	}
	if got := payload["service"]; got != "node-agent" {
		t.Fatalf("service = %q, want %q", got, "node-agent")
	}
	if got := payload["version"]; got != "9.9.9-test" {
		t.Fatalf("version = %q, want %q", got, "9.9.9-test")
	}
	if got := payload["build_time"]; got != "2026-06-30T00:00:00Z" {
		t.Fatalf("build_time = %q, want %q", got, "2026-06-30T00:00:00Z")
	}
	if got := payload["git_commit"]; got != "deadbeef" {
		t.Fatalf("git_commit = %q, want %q", got, "deadbeef")
	}
}
