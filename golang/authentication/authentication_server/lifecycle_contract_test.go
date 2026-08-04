package main

import (
	"os"
	"strings"
	"testing"
)

func TestMainDoesNotWaitForSecondSignalAfterLifecycleStart(t *testing.T) {
	src, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	idx := strings.Index(text, "lifecycle.Start()")
	if idx < 0 {
		t.Fatal("server.go no longer calls lifecycle.Start(); update this lifecycle contract test")
	}
	after := text[idx:]
	for _, forbidden := range []string{"signal.Notify", "syscall.SIGTERM", "<-sigChan", "GracefulShutdown"} {
		if strings.Contains(after, forbidden) {
			t.Fatalf("authentication main must not wait for a second shutdown signal after lifecycle.Start(); found %q", forbidden)
		}
	}
}
