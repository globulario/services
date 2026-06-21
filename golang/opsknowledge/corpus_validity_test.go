package opsknowledge

import (
	"path/filepath"
	"runtime"
	"testing"
)

// repoCorpusDirs resolves the in-repo operational-knowledge corpus and the
// awareness directory it cross-references, relative to this test file
// (golang/opsknowledge/ -> repo root is ../..).
func repoCorpusDirs(t *testing.T) (corpusDir, awarenessDir string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test file path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	return filepath.Join(repoRoot, "docs", "operational-knowledge"),
		filepath.Join(repoRoot, "docs", "awareness")
}

// TestOperationalKnowledgeCorpusValidates is the CI gate that keeps the Day-0
// ops-knowledge seed shippable: it fails the build if any entry has a
// validation error (unsupported schema, invalid type, dangling awareness link,
// etc.). This is exactly what `globular ops-knowledge validate` enforces, run as
// a unit test so a drifted corpus can never reach a release (the drift that left
// a live cluster with zero seeded ops-knowledge). Warnings are allowed.
func TestOperationalKnowledgeCorpusValidates(t *testing.T) {
	corpusDir, awarenessDir := repoCorpusDirs(t)

	files, err := LoadDir(corpusDir)
	if err != nil {
		t.Fatalf("LoadDir(%s): %v", corpusDir, err)
	}
	if len(files) == 0 {
		t.Fatalf("no operational-knowledge files found under %s", corpusDir)
	}

	refs, err := LoadRefsFromAwareness(awarenessDir, corpusDir)
	if err != nil {
		t.Fatalf("LoadRefsFromAwareness(%s): %v", awarenessDir, err)
	}

	var errorFindings []string
	for _, f := range files {
		for _, finding := range Validate(f, refs) {
			if finding.Severity == SevError {
				errorFindings = append(errorFindings, finding.String())
			}
		}
	}

	if len(errorFindings) > 0 {
		t.Fatalf("operational-knowledge corpus has %d validation error(s) — run `globular ops-knowledge validate` and fix before shipping:\n  %s",
			len(errorFindings), join(errorFindings, "\n  "))
	}
}

func join(ss []string, sep string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += sep
		}
		out += s
	}
	return out
}
