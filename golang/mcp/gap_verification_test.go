package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Task 6: Every status:implemented gap reports verification_status:tests_found
// ---------------------------------------------------------------------------

// TestVerifyGapTests_AllImplementedGapsHaveRealTests iterates over every
// status:implemented gap in the real agent_playbooks.yaml and asserts that
// verifyGapTests reports "tests_found" — meaning every tests_required entry
// maps to an actual func TestXxx in golang/awareness/**/*_test.go.
//
// This test will FAIL if:
//   - a gap's tests_required contains a description-style entry ("invalid_metadata")
//   - a required test function was deleted ("tests_not_found" or "tests_partial")
//   - agent_playbooks.yaml lists functions that don't start with TestXxx
func TestVerifyGapTests_AllImplementedGapsHaveRealTests(t *testing.T) {
	docsDir := selfReviewDocsDir(t)
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(docsDir, "knowledge", "agent_playbooks.yaml"))
	if err != nil {
		t.Fatalf("read agent_playbooks.yaml: %v", err)
	}

	var root struct {
		CapabilityGapPatterns []capabilityGapPattern `yaml:"capability_gap_patterns"`
	}
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse agent_playbooks.yaml: %v", err)
	}

	for _, gap := range root.CapabilityGapPatterns {
		if gap.Status != "implemented" {
			continue
		}
		if len(gap.TestsRequired) == 0 {
			// A gap with no tests_required is not verifiable — flag it.
			t.Errorf("gap %q (implemented) has empty tests_required — add at least one test function name", gap.ID)
			continue
		}

		status, note := verifyGapTests(repoRoot, gap.TestsRequired)
		if status != "tests_found" {
			t.Errorf("gap %q: verification_status = %q (want tests_found)\n  note: %s\n  tests_required: %v",
				gap.ID, status, note, gap.TestsRequired)
		}
	}
}

// ---------------------------------------------------------------------------
// Task 7: Description-style entries are detected as invalid metadata
// ---------------------------------------------------------------------------

// TestVerifyGapTests_DescriptionStyleEntryReportedAsInvalidMetadata verifies
// that a tests_required entry written as a prose description (e.g.
// "etcd NOSPACE in journalctl text → etcd failure mode matched") is NOT
// silently normalized to a nonsense function name like "etcd" and then
// reported as tests_not_found. Instead it must return "invalid_metadata"
// with a clear note, so the gap maintainer knows the entry needs fixing.
func TestVerifyGapTests_DescriptionStyleEntryReportedAsInvalidMetadata(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	cases := []struct {
		name    string
		entries []string
	}{
		{
			name:    "prose with arrow",
			entries: []string{"etcd NOSPACE in journalctl text → etcd failure mode matched"},
		},
		{
			name:    "prose lowercase",
			entries: []string{"learn_from_fix proposes a new failure mode from a verified incident"},
		},
		{
			name:    "mixed: one valid, one prose",
			entries: []string{"TestLearnFromFix_NewFailureMode", "pending_proposals lists the generated learned-fix proposal"},
		},
		{
			name:    "graph description",
			entries: []string{"Graph built from YAML A, YAML changed → freshness check reports stale=true"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, note := verifyGapTests(repoRoot, tc.entries)
			if status != "invalid_metadata" {
				t.Errorf("entries %v: got status=%q note=%q; want invalid_metadata — description-style entries must not be silently normalized",
					tc.entries, status, note)
			}
			if note == "" {
				t.Error("invalid_metadata must include a non-empty note explaining what is wrong")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isValidTestFuncName unit tests
// ---------------------------------------------------------------------------

func TestIsValidTestFuncName(t *testing.T) {
	valid := []string{
		"TestFoo",
		"TestFoo_Bar",
		"TestScanGoFile_LoopbackStringLiteral",
		"TestOfflineDiagnose_EtcdNospace",
	}
	for _, name := range valid {
		if !isValidTestFuncName(name) {
			t.Errorf("%q should be a valid test function name", name)
		}
	}

	invalid := []string{
		"testFoo",           // lowercase t
		"test_foo",          // snake_case, lowercase
		"etcd",              // not TestXxx
		"learn_from_fix proposes a new failure mode",
		"etcd NOSPACE in journalctl text → failure mode matched",
		"Graph built from YAML A",
		"pending_proposals lists the generated learned-fix proposal",
		"",                  // empty
		"Test",              // too short (< 5 chars with uppercase after Test)
		"Testfoo",           // lowercase letter after Test
	}
	for _, name := range invalid {
		if isValidTestFuncName(name) {
			t.Errorf("%q should NOT be a valid test function name", name)
		}
	}
}

// ---------------------------------------------------------------------------
// "unverified" must not be conflated with "tests_not_found"
// ---------------------------------------------------------------------------

// TestVerifyGapTests_EmptyRepoRootReturnsUnverified pins the distinction
// between "I couldn't scan" (unverified — expected on production MCP hosts
// that don't ship source) and "the test is missing" (tests_not_found —
// a real defect). Prior to this fix, health_pulse counted both as missing,
// producing a noisy false alert on every production node.
func TestVerifyGapTests_EmptyRepoRootReturnsUnverified(t *testing.T) {
	status, note := verifyGapTests("", []string{"TestSomething"})
	if status != "unverified" {
		t.Errorf("empty repoRoot must return status=unverified, got %q (note=%q)", status, note)
	}
	if note == "" {
		t.Error("unverified status must include a note explaining why")
	}
}

// TestAwarGitRoot_NotInRepoReturnsEmpty pins the rule that when awarGitRoot
// is called from a process whose cwd is not a git checkout (e.g. the MCP
// daemon running from /var/lib/globular/mcp on a production node), it
// returns "" rather than falling back to the cwd. The empty return is
// what makes integrity.CheckTestReferences classify missing-on-disk tests
// as REQUIRED_TEST_UNVERIFIED ("no repo to scan") instead of
// REQUIRED_TEST_MISSING ("scanned and didn't find it") — a critical-
// severity false positive in production.
func TestAwarGitRoot_NotInRepoReturnsEmpty(t *testing.T) {
	// Use a tmp dir that is provably not inside any git checkout.
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir to tmp: %v", err)
	}
	got := awarGitRoot()
	if got != "" {
		t.Errorf("awarGitRoot from a non-git dir (%s) must return \"\", got %q", tmp, got)
	}
}
