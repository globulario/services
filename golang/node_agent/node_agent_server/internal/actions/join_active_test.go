package actions

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsJoinActive_DefaultFalse(t *testing.T) {
	old := joinActiveFunc
	defer func() { joinActiveFunc = old }()
	joinActiveFunc = nil

	if IsJoinActive() {
		t.Error("IsJoinActive must return false when no callback is set")
	}
}

func TestSetJoinActiveFunc_Wired(t *testing.T) {
	old := joinActiveFunc
	defer func() { joinActiveFunc = old }()

	SetJoinActiveFunc(func() bool { return true })
	if !IsJoinActive() {
		t.Error("IsJoinActive must return true when callback returns true")
	}

	SetJoinActiveFunc(func() bool { return false })
	if IsJoinActive() {
		t.Error("IsJoinActive must return false when callback returns false")
	}
}

func TestRunPostInstallScript_PassesJoinActiveEnv(t *testing.T) {
	old := joinActiveFunc
	defer func() { joinActiveFunc = old }()

	// Case 1: join active → env var should be "true".
	dir1 := t.TempDir()
	outFile1 := filepath.Join(t.TempDir(), "out.txt")
	os.WriteFile(filepath.Join(dir1, "post-install.sh"),
		[]byte("#!/bin/bash\necho \"JOIN_ACTIVE=${GLOBULAR_JOIN_ACTIVE:-unset}\" > "+outFile1+"\n"), 0755)

	SetJoinActiveFunc(func() bool { return true })
	if err := runPostInstallScript(context.Background(), dir1, t.TempDir()); err != nil {
		t.Fatalf("post-install with join active: %v", err)
	}
	got1, _ := os.ReadFile(outFile1)
	if !strings.Contains(string(got1), "JOIN_ACTIVE=true") {
		t.Errorf("expected GLOBULAR_JOIN_ACTIVE=true, got: %s", string(got1))
	}

	// Case 2: join not active → env var should be unset.
	dir2 := t.TempDir()
	outFile2 := filepath.Join(t.TempDir(), "out2.txt")
	os.WriteFile(filepath.Join(dir2, "post-install.sh"),
		[]byte("#!/bin/bash\necho \"JOIN_ACTIVE=${GLOBULAR_JOIN_ACTIVE:-unset}\" > "+outFile2+"\n"), 0755)

	SetJoinActiveFunc(func() bool { return false })
	if err := runPostInstallScript(context.Background(), dir2, t.TempDir()); err != nil {
		t.Fatalf("post-install without join active: %v", err)
	}
	got2, _ := os.ReadFile(outFile2)
	if !strings.Contains(string(got2), "JOIN_ACTIVE=unset") {
		t.Errorf("expected GLOBULAR_JOIN_ACTIVE unset, got: %s", string(got2))
	}
}
