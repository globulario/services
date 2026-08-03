package main

import (
	"os"
	"strings"
	"testing"
)

func readSourceFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

func containsAll(src string, needles ...string) bool {
	for _, n := range needles {
		if !strings.Contains(src, n) {
			return false
		}
	}
	return true
}
