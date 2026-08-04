package main

import (
	"os"
	"strings"
	"testing"
)

func readWorkflowSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

func containsStr(src, needle string) bool { return strings.Contains(src, needle) }

// stripLineComments removes // comments so a structural ratchet inspects CODE
// and is not tripped by prose that quotes the very pattern it forbids.
func stripLineComments(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}
