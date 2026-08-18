package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The corpus is labelled from real code found in the 2026-08-14 fail-open audit.
// Each KNOWN-BAD is a shape that actually exists in this repository; each
// KNOWN-GOOD is a shape that also exists and must NOT be reported.
//
// The good cases matter more than the bad ones. A scanner that flags everything
// is indistinguishable from one that is broken, and the cost of a false positive
// here is someone "fixing" correct fail-safe code.
func TestScanner_LabelledCorpus(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("testdata", "corpus.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "corpus.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}

	findings, err := scan(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	byFunc := map[string]Finding{}
	for _, f := range findings {
		if prev, dup := byFunc[f.Func]; dup {
			t.Errorf("duplicate finding for %s: %s and %s", f.Func, prev.Rule, f.Rule)
		}
		byFunc[f.Func] = f
	}

	wantBad := map[string]string{
		"badConsumerCollapse":  "uncertainty-collapsed-to-zero",
		"badErrorErased":       "error-erased-on-return",
		"badPermissiveDefault": "uncertainty-collapsed-to-zero",
		"badUnjustifiedOptOut": "unjustified-optout",
	}
	wantGood := []string{
		"goodDeniesOnError",
		"goodNoVerdictDownstream",
		"goodDeclaredFailsafe",
		"goodCentralisedElsewhere",
	}

	for fn, rule := range wantBad {
		got, ok := byFunc[fn]
		if !ok {
			t.Errorf("MISSED known-bad %s (expected rule %s)", fn, rule)
			continue
		}
		if got.Rule != rule {
			t.Errorf("%s: got rule %q, want %q", fn, got.Rule, rule)
		}
	}

	for _, fn := range wantGood {
		if got, ok := byFunc[fn]; ok {
			t.Errorf("FALSE POSITIVE on known-good %s: [%s] %s", fn, got.Rule, got.Message)
		}
	}

	if len(findings) != len(wantBad) {
		t.Errorf("finding count = %d, want %d", len(findings), len(wantBad))
		for _, f := range findings {
			t.Logf("  %s:%d [%s] %s", filepath.Base(f.File), f.Line, f.Rule, f.Func)
		}
	}
}

// The sink requirement is what makes this scanner high-signal rather than a
// generic "swallowed error" linter. Without it, every logged-and-continued
// error in the codebase would be a finding.
func TestScanner_SinkRequirementIsLoadBearing(t *testing.T) {
	dir := t.TempDir()
	src := `package p
func noSink() {
	x, err := load()
	if err != nil {
		x = nil
	}
	store(x)
}
func withSink() bool {
	x, err := load()
	if err != nil {
		x = nil
	}
	return authorize(x)
}
`
	if err := os.WriteFile(filepath.Join(dir, "s.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := scan(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 {
		t.Fatalf("want exactly 1 finding (only the one reaching a verdict), got %d", len(findings))
	}
	if findings[0].Func != "withSink" {
		t.Errorf("flagged %s; the no-sink case must not be reported", findings[0].Func)
	}
}

// An error branch that propagates the error is the correct shape and must never
// be reported, however it is written.
func TestScanner_PropagatedErrorIsNeverFlagged(t *testing.T) {
	dir := t.TempDir()
	src := `package p
func propagates() ([]string, error) {
	v, err := load()
	if err != nil {
		return nil, err
	}
	return authorize(v), nil
}
func wrapsAndReturns() ([]string, error) {
	v, err := load()
	if err != nil {
		return nil, wrap(err)
	}
	return authorize(v), nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := scan(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("propagated errors must not be flagged, got %d: %+v", len(findings), findings)
	}
}
