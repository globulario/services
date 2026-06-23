package main

import (
	"io/fs"
	"os"
	"strings"
	"testing"
)

// fakeDirEntry is a minimal os.DirEntry for exercising minioResticExcludes
// without touching the filesystem.
type fakeDirEntry struct {
	name string
	dir  bool
}

func (f fakeDirEntry) Name() string               { return f.name }
func (f fakeDirEntry) IsDir() bool                { return f.dir }
func (f fakeDirEntry) Type() fs.FileMode          { return 0 }
func (f fakeDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

func dirEntries(names ...string) []os.DirEntry {
	out := make([]os.DirEntry, 0, len(names))
	for _, n := range names {
		out = append(out, fakeDirEntry{name: n, dir: true})
	}
	return out
}

func TestMinioResticExcludes_OffNodeReplicated_ExcludesWholeDir(t *testing.T) {
	got := minioResticExcludes(true, dirEntries("globular-backups", "user-photos", ".minio.sys"))
	if len(got) != 1 || got[0] != minioDataDir {
		t.Fatalf("off-node replication should exclude the whole minio data dir, got %v", got)
	}
}

func TestMinioResticExcludes_KeepsScyllaBucketAndSystemDir(t *testing.T) {
	got := minioResticExcludes(false, dirEntries(scyllaBackupBucket, ".minio.sys", "user-photos", "user-docs"))
	// The scylla backup bucket and .minio.sys must NOT be excluded — they are
	// required for a wipe-survivable, restorable backup.
	for _, mustKeep := range []string{
		minioDataDir + "/" + scyllaBackupBucket,
		minioDataDir + "/.minio.sys",
	} {
		for _, ex := range got {
			if ex == mustKeep {
				t.Errorf("excluded a path that must be captured: %q (excludes=%v)", mustKeep, got)
			}
		}
	}
	// User buckets must be excluded to keep the snapshot lean.
	sliceContainsAll(t, got, []string{
		minioDataDir + "/user-photos",
		minioDataDir + "/user-docs",
	}, "user buckets excluded")
	if len(got) != 2 {
		t.Errorf("expected exactly the 2 user buckets excluded, got %v", got)
	}
}

func TestMinioResticExcludes_NilEntries(t *testing.T) {
	if got := minioResticExcludes(false, nil); len(got) != 0 {
		t.Errorf("no entries (minio data dir absent) should yield no excludes, got %v", got)
	}
}

// helper: assert a slice contains every element of want, in any position.
func sliceContainsAll(t *testing.T, got, want []string, label string) {
	t.Helper()
	set := map[string]bool{}
	for _, s := range got {
		set[s] = true
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("%s: missing %q in %v", label, w, got)
		}
	}
}

// helper: count occurrences of pair "--exclude X" in argv.
func excludeArgs(argv []string) []string {
	var out []string
	for i := 0; i < len(argv)-1; i++ {
		if argv[i] == "--exclude" {
			out = append(out, argv[i+1])
		}
	}
	return out
}

func TestBuildResticBackupArgs_IncludesDefaultExcludes(t *testing.T) {
	got := buildResticBackupArgs(
		"/var/backups/globular/restic",
		"/var/lib/globular/backups",
		[]string{"/var/lib/globular"},
		nil,
	)
	excl := excludeArgs(got)
	sliceContainsAll(t, excl, defaultResticExcludes, "default excludes")
	// dataDir + repo also present (they're now both in dataDir/repo args AND
	// included via defaults — dedupe in buildResticBackupArgs prevents
	// listing the same string twice).
	sliceContainsAll(t, excl, []string{
		"/var/backups/globular/restic",
		"/var/lib/globular/backups",
	}, "dataDir + repo excludes")
}

func TestBuildResticBackupArgs_AcceptsExtraExcludes(t *testing.T) {
	extras := []string{"/custom/path/a", "/custom/path/b"}
	got := buildResticBackupArgs(
		"/var/backups/globular/restic",
		"/var/lib/globular/backups",
		[]string{"/var/lib/globular"},
		extras,
	)
	excl := excludeArgs(got)
	sliceContainsAll(t, excl, extras, "extra excludes")
	// Defaults still present alongside extras.
	sliceContainsAll(t, excl, defaultResticExcludes, "defaults still present with extras")
}

func TestBuildResticBackupArgs_NoExtraExcludesWhenEmpty(t *testing.T) {
	got := buildResticBackupArgs("repo", "data", []string{"/p"}, nil)
	if excl := excludeArgs(got); len(excl) == 0 {
		t.Fatal("expected at least dataDir+repo+defaults excludes")
	}
	// Empty extras + whitespace-only entries must be filtered out: build
	// with []string{"", "   "} and assert exclude count unchanged from nil.
	baseline := excludeArgs(buildResticBackupArgs("repo", "data", []string{"/p"}, nil))
	withEmpty := excludeArgs(buildResticBackupArgs("repo", "data", []string{"/p"}, []string{"", "   "}))
	if len(withEmpty) != len(baseline) {
		t.Fatalf("empty/whitespace extras should produce no additional excludes: baseline=%d withEmpty=%d",
			len(baseline), len(withEmpty))
	}
}

func TestBuildResticBackupArgs_PathsAppendedAfterFlags(t *testing.T) {
	paths := []string{"/var/lib/globular", "/var/log/globular"}
	got := buildResticBackupArgs("repo", "data", paths, nil)

	// The last len(paths) elements must be exactly the source paths, in order.
	if len(got) < len(paths) {
		t.Fatalf("argv shorter than path list: %v", got)
	}
	tail := got[len(got)-len(paths):]
	for i, p := range paths {
		if tail[i] != p {
			t.Errorf("trailing arg[%d] = %q, want %q (paths must be at the end, positional)", i, tail[i], p)
		}
	}
	// First two args must be the subcommand and JSON flag.
	if got[0] != "backup" || got[1] != "--json" {
		t.Errorf("argv head = %v, want [backup --json ...]", got[:2])
	}
}

// realistic stderr fragment that mirrors what restic actually produced
// during the 2026-05-20 backup attempt: JSON-per-line, mixed scan/archival.
const sampleResticStderrOptionalOnly = `
{"message_type":"error","error":{"message":"openfile for readdirnames failed: open /var/lib/globular/staging/ai-executor/extract-x: permission denied"},"during":"scan","item":"/var/lib/globular/staging/ai-executor/extract-x"}
{"message_type":"error","error":{"message":"open /var/lib/globular/nodeagent/state.json: permission denied"},"during":"scan","item":"/var/lib/globular/nodeagent/state.json"}
{"message_type":"error","error":{"message":"open /var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml.bak.1779291412: permission denied"},"during":"scan","item":"/var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml.bak.1779291412"}
{"message_type":"error","error":{"message":"open /var/lib/globular/nodeagent/plan-abc.json: permission denied"},"during":"scan","item":"/var/lib/globular/nodeagent/plan-abc.json"}
{"message_type":"exit_error","code":3,"message":"Warning: at least one source file could not be read"}
`

const sampleResticStderrRequiredMissing = `
{"message_type":"error","error":{"message":"open /var/lib/globular/staging/ai/extract-xyz: permission denied"},"during":"scan","item":"/var/lib/globular/staging/ai/extract-xyz"}
{"message_type":"error","error":{"message":"open /var/lib/globular/keys/00_1f_c6_9c_d3_34_XYS7J0vcsNLq9BCd_private: permission denied"},"during":"archival","item":"/var/lib/globular/keys/00_1f_c6_9c_d3_34_XYS7J0vcsNLq9BCd_private"}
{"message_type":"error","error":{"message":"open /var/lib/globular/ingress/spec-last-known-good.json: permission denied"},"during":"archival","item":"/var/lib/globular/ingress/spec-last-known-good.json"}
{"message_type":"exit_error","code":3,"message":"Warning: at least one source file could not be read"}
`

func TestClassifyResticWarnings_OnlyOptional(t *testing.T) {
	configuredExcludes := append([]string{"/var/lib/globular/backups", "/var/backups/globular/restic"}, defaultResticExcludes...)
	missing, allOptional := classifyResticWarnings(sampleResticStderrOptionalOnly, configuredExcludes)
	if !allOptional {
		t.Errorf("expected allOptional=true, got false; missing=%v", missing)
	}
	if len(missing) != 0 {
		t.Errorf("expected no required-missing, got %v", missing)
	}
}

func TestClassifyResticWarnings_RequiredMissing(t *testing.T) {
	configuredExcludes := append([]string{"/var/lib/globular/backups", "/var/backups/globular/restic"}, defaultResticExcludes...)
	missing, allOptional := classifyResticWarnings(sampleResticStderrRequiredMissing, configuredExcludes)
	if allOptional {
		t.Errorf("expected allOptional=false (required paths present)")
	}
	wantContains := []string{
		"/var/lib/globular/keys/00_1f_c6_9c_d3_34_XYS7J0vcsNLq9BCd_private",
		"/var/lib/globular/ingress/spec-last-known-good.json",
	}
	for _, w := range wantContains {
		found := false
		for _, m := range missing {
			if m == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected missing list to contain %q; got %v", w, missing)
		}
	}
	// The staging error is an optional (excluded) path; it must NOT be in
	// the missing-required list.
	for _, m := range missing {
		if strings.Contains(m, "/staging/") {
			t.Errorf("staging path %q must be optional, not required-missing", m)
		}
	}
}

func TestClassifyResticWarnings_IgnoresUnrelatedErrors(t *testing.T) {
	// Lines that don't parse as JSON or aren't message_type=error must be
	// ignored. Includes a "verbose" line and a malformed line.
	stderr := `
not-json informational line
{"message_type":"verbose_status","action":"start"}
{"message_type":"status","seconds_remaining":10}
{"message_type":"error","error":{"message":"unrelated noise"},"during":"scan","item":""}
{"message_type":"exit_error","code":3,"message":"Warning: ..."}
`
	missing, allOptional := classifyResticWarnings(stderr, defaultResticExcludes)
	if !allOptional {
		t.Errorf("expected allOptional=true when stderr has no error+item rows, got missing=%v", missing)
	}
	if len(missing) != 0 {
		t.Errorf("expected zero missing; got %v", missing)
	}
}

// TestRunResticBackup_RequiredMissingFailsClean exercises the exact decision
// runResticBackup makes on restic exit-3 — without spawning real restic.
// We test evaluateResticResult because that's the load-bearing helper called
// from runResticBackup; refactoring runResticBackup to be fully unit-testable
// would require stubbing exec.Command, the capsule write, and several layers
// of glue. The classifier + evaluator together cover the new logic; the
// outer function just translates their verdict into failResult(...).
func TestRunResticBackup_RequiredMissingFailsClean(t *testing.T) {
	configuredExcludes := append([]string{"/var/lib/globular/backups", "/var/backups/globular/restic"}, defaultResticExcludes...)
	outputs := map[string]string{}
	ok, failMsg := evaluateResticResult(3, sampleResticStderrRequiredMissing, configuredExcludes, outputs)
	if ok {
		t.Fatal("expected failure when required path is unreadable")
	}
	if !strings.Contains(failMsg, "restic backup failed: required path(s) unreadable") {
		t.Errorf("failMsg should declare the failure class clearly, got: %q", failMsg)
	}
	if !strings.Contains(failMsg, "user=globular") {
		t.Errorf("failMsg should name the backup_manager user, got: %q", failMsg)
	}
	if outputs["unreadable_required"] == "" {
		t.Errorf("outputs[unreadable_required] must be populated on failure")
	}
	if !strings.Contains(outputs["unreadable_required"], "/keys/") {
		t.Errorf("outputs[unreadable_required] should list the keys path; got %q", outputs["unreadable_required"])
	}
}

func TestEvaluateResticResult_Exit0Success(t *testing.T) {
	ok, msg := evaluateResticResult(0, "", defaultResticExcludes, map[string]string{})
	if !ok || msg != "" {
		t.Errorf("exit 0 must be unconditional success; got ok=%v msg=%q", ok, msg)
	}
}

func TestEvaluateResticResult_Exit3AllOptional(t *testing.T) {
	configuredExcludes := append([]string{"/var/lib/globular/backups", "/var/backups/globular/restic"}, defaultResticExcludes...)
	outputs := map[string]string{}
	ok, msg := evaluateResticResult(3, sampleResticStderrOptionalOnly, configuredExcludes, outputs)
	if !ok || msg != "" {
		t.Errorf("exit 3 with all-optional warnings must be success; got ok=%v msg=%q", ok, msg)
	}
	if _, set := outputs["unreadable_required"]; set {
		t.Errorf("outputs[unreadable_required] must NOT be set when all warnings are optional")
	}
}

func TestEvaluateResticResult_OtherExitDefersToCaller(t *testing.T) {
	// Non-3 non-0 exit codes return (false, "") so the caller can apply its
	// own fail-with-stderr handling (preserving the existing behaviour for
	// restic exit codes 1 and any future codes).
	ok, msg := evaluateResticResult(1, "fatal: cannot reach repo", defaultResticExcludes, map[string]string{})
	if ok {
		t.Errorf("exit 1 must not be success")
	}
	if msg != "" {
		t.Errorf("exit 1 must not produce a failMsg from evaluateResticResult; caller handles it. got %q", msg)
	}
}

// TestMatchesExcludePattern_Glob covers the glob semantics specifically.
// Anchors the load-bearing classifier behaviour: the stale yaml backup
// files restic complained about in production must classify as optional.
func TestMatchesExcludePattern_Glob(t *testing.T) {
	cases := []struct {
		path    string
		pattern string
		want    bool
	}{
		// directory prefix (no glob)
		{"/var/lib/globular/staging/ai/extract-x", "/var/lib/globular/staging", true},
		{"/var/lib/globular/staging", "/var/lib/globular/staging", true},
		{"/var/lib/globular/etcd/wal/file", "/var/lib/globular/etcd", true},
		// non-match prefix
		{"/var/lib/globular/state.json", "/var/lib/globular/staging", false},
		// glob: yaml backup files
		{"/var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml.bak.1779291412",
			"/var/lib/globular/scylla-manager-agent/*.bak.*", true},
		{"/var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml.before-token-fix.1779295343",
			"/var/lib/globular/scylla-manager-agent/*.before-*", true},
		// glob: plan files
		{"/var/lib/globular/nodeagent/plan-2024.json",
			"/var/lib/globular/nodeagent/plan-*.json", true},
		// glob: tmp files via basename
		{"/some/where/x.tmp", "*.tmp", true},
		// glob: required path NOT covered by any pattern
		{"/var/lib/globular/keys/X_private", "/var/lib/globular/staging", false},
	}
	for _, c := range cases {
		if got := matchesExcludePattern(c.path, c.pattern); got != c.want {
			t.Errorf("matchesExcludePattern(%q, %q) = %v, want %v",
				c.path, c.pattern, got, c.want)
		}
	}
}

// TestResticSnapshotsToValidate_CoversEveryNode locks in the AWG re-audit fix
// (meta.assertions_must_carry_their_scope): deep validation of a cluster restic
// backup must verify EVERY node's snapshot, not just the top-level (first
// node's) snapshot_id, before the artifact is promoted to VALIDATED.
func TestResticSnapshotsToValidate_CoversEveryNode(t *testing.T) {
	// A cluster fan-out: top-level snapshot_id is node-a's; per-node IDs recorded.
	outputs := map[string]string{
		"snapshot_id":         "aaa",
		"snapshot_id_node-a":  "aaa",
		"snapshot_id_node-b":  "bbb",
		"snapshot_id_node-c":  "ccc",
		"node_count":          "3",
	}
	got := resticSnapshotsToValidate(outputs)
	if len(got) != 3 {
		t.Fatalf("expected all 3 per-node snapshots to validate, got %d: %v", len(got), got)
	}
	for _, node := range []string{"node-a", "node-b", "node-c"} {
		if got[node] == "" {
			t.Errorf("missing snapshot to validate for %s", node)
		}
	}
}

// TestResticSnapshotsToValidate_SingleNodeBackwardCompat — a pre-fan-out backup
// records only the top-level snapshot_id; it must still be validated.
func TestResticSnapshotsToValidate_SingleNodeBackwardCompat(t *testing.T) {
	got := resticSnapshotsToValidate(map[string]string{"snapshot_id": "solo"})
	if len(got) != 1 || got[""] != "solo" {
		t.Fatalf("single-node backup must validate its one snapshot; got %v", got)
	}
}

// TestResticSnapshotsToValidate_None — no snapshot recorded → nothing to
// validate (caller emits the missing-id warning).
func TestResticSnapshotsToValidate_None(t *testing.T) {
	if got := resticSnapshotsToValidate(map[string]string{"node_count": "2"}); len(got) != 0 {
		t.Fatalf("expected no snapshots to validate, got %v", got)
	}
}
