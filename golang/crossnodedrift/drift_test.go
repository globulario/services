package crossnodedrift

// drift_test.go — Phase 7 of the Diagnostic Honesty Refactor.
//
// Pins the contract of DetectDrift across every Authority value. The
// four scenarios the brief calls out:
//
//   1. one node has webroot files and others do not    (Replicated)
//   2. file exists locally but missing from objectstore (ObjectstoreBacked)
//   3. objectstore has file but local fallback serves stale content (ObjectstoreBacked)
//   4. expected authority is unclear, finding says authority_undefined

import (
	"strings"
	"testing"
)

// Helpers ─────────────────────────────────────────────────────────────

func present(node, hash string) NodeObservation {
	return NodeObservation{NodeID: node, PathClass: "x", Path: "p", SHA256: hash, Present: true}
}

func absent(node string) NodeObservation {
	return NodeObservation{NodeID: node, PathClass: "x", Path: "p", Present: false}
}

func erred(node, msg string) NodeObservation {
	return NodeObservation{NodeID: node, PathClass: "x", Path: "p", Error: msg}
}

// ─────────────────────────────────────────────────────────────────────
// Scenario 1: webroot — one node has files, four don't. Replicated.
// ─────────────────────────────────────────────────────────────────────

func TestDetectDrift_Webroot_OneNodePresentOthersAbsent_Drift(t *testing.T) {
	class := PathClass{Name: "webroot", Authority: AuthorityReplicated}
	obs := []NodeObservation{
		present("ryzen", "aaaa"),
		absent("nuc"),
		absent("dell"),
		absent("hp"),
		absent("lenovo"),
	}
	v := DetectDrift(class, "index.html", obs, AuthorityContext{})
	if v.Status != DriftStatusDrift {
		t.Fatalf("Status=%q want=%q", v.Status, DriftStatusDrift)
	}
	if v.FindingID != FindingID {
		t.Errorf("FindingID=%q want=%q", v.FindingID, FindingID)
	}
	if len(v.Drifts) != 5 {
		t.Errorf("Drifts=%v want one line per node", v.Drifts)
	}
	// At least one drift line names the present node and one names an absent one.
	foundPresent := false
	foundAbsent := false
	for _, d := range v.Drifts {
		if strings.Contains(d, "ryzen") && strings.Contains(d, "present") {
			foundPresent = true
		}
		if strings.Contains(d, "absent") {
			foundAbsent = true
		}
	}
	if !foundPresent || !foundAbsent {
		t.Errorf("drift lines must distinguish present vs absent: %v", v.Drifts)
	}
}

func TestDetectDrift_Replicated_AllPresentSameHash_Consistent(t *testing.T) {
	class := PathClass{Name: "webroot", Authority: AuthorityReplicated}
	obs := []NodeObservation{
		present("a", "aaaa"),
		present("b", "aaaa"),
		present("c", "aaaa"),
	}
	v := DetectDrift(class, "p", obs, AuthorityContext{})
	if v.Status != DriftStatusConsistent {
		t.Errorf("Status=%q want=%q (drifts=%v)", v.Status, DriftStatusConsistent, v.Drifts)
	}
	if v.FindingID != "" {
		t.Errorf("FindingID=%q want empty when consistent", v.FindingID)
	}
}

func TestDetectDrift_Replicated_AllAbsent_Consistent(t *testing.T) {
	// Deleted everywhere is a legitimate steady state — don't raise.
	class := PathClass{Name: "webroot", Authority: AuthorityReplicated}
	obs := []NodeObservation{absent("a"), absent("b"), absent("c")}
	v := DetectDrift(class, "p", obs, AuthorityContext{})
	if v.Status != DriftStatusConsistent {
		t.Errorf("all-absent must be consistent; got %q", v.Status)
	}
}

func TestDetectDrift_Replicated_HashesDiffer_Drift(t *testing.T) {
	class := PathClass{Name: "webroot", Authority: AuthorityReplicated}
	obs := []NodeObservation{
		present("a", "aaaa"),
		present("b", "bbbb"),
		present("c", "aaaa"),
	}
	v := DetectDrift(class, "p", obs, AuthorityContext{})
	if v.Status != DriftStatusDrift {
		t.Errorf("Status=%q want=drift", v.Status)
	}
	if len(v.Drifts) != 3 {
		t.Errorf("Drifts=%v want one per node", v.Drifts)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Scenario 2: file exists locally but objectstore has no such object.
// ─────────────────────────────────────────────────────────────────────

func TestDetectDrift_ObjectstoreBacked_LocalPresentObjectstoreEmpty_Drift(t *testing.T) {
	class := PathClass{Name: "webroot", Authority: AuthorityObjectstoreBacked}
	obs := []NodeObservation{present("ryzen", "deadbeef")}
	v := DetectDrift(class, "stale.html", obs, AuthorityContext{ObjectstoreHash: ""})
	if v.Status != DriftStatusDrift {
		t.Fatalf("Status=%q want=drift; drifts=%v", v.Status, v.Drifts)
	}
	if v.FindingID != FindingID {
		t.Errorf("FindingID=%q want=%q", v.FindingID, FindingID)
	}
	if !strings.Contains(v.Drifts[0], "objectstore has no such object") {
		t.Errorf("drift line must explain the discrepancy: %q", v.Drifts[0])
	}
}

// ─────────────────────────────────────────────────────────────────────
// Scenario 3: objectstore has a hash, local file disagrees (stale).
// ─────────────────────────────────────────────────────────────────────

func TestDetectDrift_ObjectstoreBacked_LocalHashDiffers_Drift(t *testing.T) {
	class := PathClass{Name: "webroot", Authority: AuthorityObjectstoreBacked}
	obs := []NodeObservation{
		present("ryzen", "deadbeef"),
		present("nuc", "1234abcd"), // matches objectstore
	}
	v := DetectDrift(class, "index.html", obs, AuthorityContext{ObjectstoreHash: "1234abcd"})
	if v.Status != DriftStatusDrift {
		t.Fatalf("Status=%q want=drift; drifts=%v", v.Status, v.Drifts)
	}
	if len(v.Drifts) != 1 {
		t.Errorf("only the diverging node should drift; got %v", v.Drifts)
	}
	if !strings.Contains(v.Drifts[0], "ryzen") {
		t.Errorf("drift line must name the diverging node: %q", v.Drifts[0])
	}
}

func TestDetectDrift_ObjectstoreBacked_LocalAbsent_OK(t *testing.T) {
	// Cache miss is legitimate — objectstore is the truth and the
	// node will fetch on demand. Not drift.
	class := PathClass{Name: "webroot", Authority: AuthorityObjectstoreBacked}
	obs := []NodeObservation{
		absent("ryzen"),
		present("nuc", "1234abcd"),
	}
	v := DetectDrift(class, "p", obs, AuthorityContext{ObjectstoreHash: "1234abcd"})
	if v.Status != DriftStatusConsistent {
		t.Errorf("Status=%q want=consistent (absent local is OK for cache class); drifts=%v",
			v.Status, v.Drifts)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Scenario 4: authority not declared — emit authority_undefined finding.
// ─────────────────────────────────────────────────────────────────────

func TestDetectDrift_AuthorityUndefined_RaisesFinding(t *testing.T) {
	class := PathClass{Name: "mystery_path", Authority: AuthorityUndefined}
	obs := []NodeObservation{present("a", "aaaa"), absent("b")}
	v := DetectDrift(class, "p", obs, AuthorityContext{})
	if v.Status != DriftStatusAuthorityUndefined {
		t.Fatalf("Status=%q want=%q", v.Status, DriftStatusAuthorityUndefined)
	}
	if v.FindingID != FindingAuthorityUndefined {
		t.Errorf("FindingID=%q want=%q", v.FindingID, FindingAuthorityUndefined)
	}
}

func TestDetectDrift_AuthorityUnknownValue_RaisesFinding(t *testing.T) {
	// A future Authority value that hasn't been wired into the switch
	// must NOT silently pass; it raises authority_undefined.
	class := PathClass{Name: "future", Authority: Authority("totally_made_up")}
	obs := []NodeObservation{present("a", "aaaa")}
	v := DetectDrift(class, "p", obs, AuthorityContext{})
	if v.Status != DriftStatusAuthorityUndefined {
		t.Errorf("Status=%q want=authority_undefined for unknown authority", v.Status)
	}
}

// ─────────────────────────────────────────────────────────────────────
// node_local: legitimate per-node — never drift.
// ─────────────────────────────────────────────────────────────────────

func TestDetectDrift_NodeLocal_DifferingNodesIsConsistent(t *testing.T) {
	class := PathClass{Name: "logs", Authority: AuthorityNodeLocal}
	obs := []NodeObservation{
		present("a", "aaaa"),
		present("b", "bbbb"),
		absent("c"),
	}
	v := DetectDrift(class, "/var/log/x", obs, AuthorityContext{})
	if v.Status != DriftStatusConsistent {
		t.Errorf("node-local class must always be consistent; got %q", v.Status)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Generated-from-* — every node must hash-match the generated hash.
// ─────────────────────────────────────────────────────────────────────

func TestDetectDrift_GeneratedFromEtcd_AllMatch_Consistent(t *testing.T) {
	class := PathClass{Name: "rendered_systemd_units", Authority: AuthorityGeneratedFromEtcd}
	obs := []NodeObservation{
		present("a", "abcd"),
		present("b", "abcd"),
	}
	v := DetectDrift(class, "globular-foo.service", obs, AuthorityContext{GeneratedHash: "abcd"})
	if v.Status != DriftStatusConsistent {
		t.Errorf("Status=%q want=consistent (drifts=%v)", v.Status, v.Drifts)
	}
}

func TestDetectDrift_GeneratedFromEtcd_OneAbsent_Drift(t *testing.T) {
	class := PathClass{Name: "rendered_systemd_units", Authority: AuthorityGeneratedFromEtcd}
	obs := []NodeObservation{
		present("a", "abcd"),
		absent("b"),
	}
	v := DetectDrift(class, "globular-foo.service", obs, AuthorityContext{GeneratedHash: "abcd"})
	if v.Status != DriftStatusDrift {
		t.Errorf("Status=%q want=drift", v.Status)
	}
	if len(v.Drifts) != 1 || !strings.Contains(v.Drifts[0], "b") {
		t.Errorf("expected one drift line naming the absent node; got %v", v.Drifts)
	}
}

func TestDetectDrift_GeneratedFromEtcd_HashMismatch_Drift(t *testing.T) {
	class := PathClass{Name: "rendered_systemd_units", Authority: AuthorityGeneratedFromEtcd}
	obs := []NodeObservation{
		present("a", "abcd"),
		present("b", "wrong"),
	}
	v := DetectDrift(class, "p", obs, AuthorityContext{GeneratedHash: "abcd"})
	if v.Status != DriftStatusDrift {
		t.Errorf("Status=%q want=drift", v.Status)
	}
	if len(v.Drifts) != 1 {
		t.Errorf("only diverging node should drift; got %v", v.Drifts)
	}
}

func TestDetectDrift_GeneratedFromEtcd_GeneratedHashMissing_Drift(t *testing.T) {
	// Missing GeneratedHash means the generation step itself never
	// ran. That is a drift in its own right — operators must know.
	class := PathClass{Name: "rendered_systemd_units", Authority: AuthorityGeneratedFromEtcd}
	obs := []NodeObservation{present("a", "abcd")}
	v := DetectDrift(class, "p", obs, AuthorityContext{GeneratedHash: ""})
	if v.Status != DriftStatusDrift {
		t.Errorf("Status=%q want=drift when GeneratedHash is missing", v.Status)
	}
	if !strings.Contains(v.Drifts[0], "generation_missing") {
		t.Errorf("expected generation_missing in drift evidence; got %v", v.Drifts)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Collection errors: never raise drift on their own — return unknown
// when EVERY observation erred, otherwise process the rest.
// ─────────────────────────────────────────────────────────────────────

func TestDetectDrift_AllErrored_Unknown(t *testing.T) {
	class := PathClass{Name: "webroot", Authority: AuthorityReplicated}
	obs := []NodeObservation{
		erred("a", "permission denied"),
		erred("b", "io error"),
	}
	v := DetectDrift(class, "p", obs, AuthorityContext{})
	if v.Status != DriftStatusUnknown {
		t.Errorf("Status=%q want=unknown when every node erred", v.Status)
	}
	if v.FindingID != "" {
		t.Errorf("FindingID=%q want empty for unknown verdict (no drift asserted)", v.FindingID)
	}
}

func TestDetectDrift_SomeErrored_ProcessRest(t *testing.T) {
	class := PathClass{Name: "webroot", Authority: AuthorityReplicated}
	obs := []NodeObservation{
		erred("a", "permission denied"),
		present("b", "abcd"),
		present("c", "abcd"),
	}
	v := DetectDrift(class, "p", obs, AuthorityContext{})
	// b and c agree; a is errored. The non-error subset is internally
	// consistent — verdict is consistent (the error is operationally
	// surfaced elsewhere; drift detection shouldn't double-flag it).
	if v.Status != DriftStatusConsistent {
		t.Errorf("Status=%q want=consistent when non-erred nodes agree; drifts=%v", v.Status, v.Drifts)
	}
}

// ─────────────────────────────────────────────────────────────────────
// known_path_classes catalog — every entry must declare an authority.
// ─────────────────────────────────────────────────────────────────────

func TestKnownPathClasses_NoneUndefined(t *testing.T) {
	for _, c := range KnownPathClasses() {
		if c.Authority == AuthorityUndefined {
			t.Errorf("path class %q has AuthorityUndefined — the catalog requires every class to declare authority", c.Name)
		}
		if c.Name == "" {
			t.Errorf("path class with empty Name in catalog: %+v", c)
		}
		if c.Description == "" {
			t.Errorf("path class %q has empty Description — operators need the one-line explanation", c.Name)
		}
	}
}

func TestLookupPathClass_KnownReturnsClass(t *testing.T) {
	got := LookupPathClass("webroot")
	if got.Authority != AuthorityObjectstoreBacked {
		t.Errorf("LookupPathClass(webroot).Authority=%q want=%q", got.Authority, AuthorityObjectstoreBacked)
	}
}

func TestLookupPathClass_UnknownReturnsUndefined(t *testing.T) {
	got := LookupPathClass("never_declared")
	if got.Authority != AuthorityUndefined {
		t.Errorf("unknown lookup must return AuthorityUndefined; got %q", got.Authority)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Sanity: FindingID values pinned against the failure_modes.yaml.
// ─────────────────────────────────────────────────────────────────────

func TestFindingID_PinsContract(t *testing.T) {
	if FindingID != "cluster.cross_node_file_drift" {
		t.Errorf("FindingID=%q must be cluster.cross_node_file_drift to match failure_modes.yaml", FindingID)
	}
	if FindingAuthorityUndefined != "cluster.authority_undefined" {
		t.Errorf("FindingAuthorityUndefined=%q must be cluster.authority_undefined", FindingAuthorityUndefined)
	}
}
