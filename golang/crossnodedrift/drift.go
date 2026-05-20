// Package crossnodedrift is the Phase 7 primitive of the Diagnostic
// Honesty Refactor. It detects when a cluster-visible path (webroot,
// service spec, rendered systemd unit, objectstore-backed asset, etc.)
// disagrees across nodes that are supposed to be in lockstep.
//
// The webroot incident that motivates this phase ran for two days: one
// node had files, four did not. Health was green on every node because
// each only inspected its own state. Nothing in the cluster owned the
// question "are these files the same on every node?"
//
// This package is intentionally tiny: types + one pure decision
// function. It does no I/O, opens no RPCs, schedules no goroutines.
// Phase 9 (verifier) collects NodeObservations via node-agent and
// passes them through DetectDrift to produce doctor findings.
//
// Path classes (replicated / objectstore-backed / etc.) and their
// declared authority live in known_path_classes.go.
package crossnodedrift

import (
	"fmt"
	"sort"
	"strings"
)

// FindingID is the canonical doctor failure-mode id emitted whenever
// observations across nodes diverge for a path class that should be
// consistent.
const FindingID = "cluster.cross_node_file_drift"

// FindingAuthorityUndefined is the special case raised when a path
// class has no declared authority. Operators cannot tell whether drift
// is legitimate (per-node by design) or a bug (the path was supposed
// to be replicated). The brief mandates a distinct finding so the
// remediation differs: declare the authority before deciding drift.
const FindingAuthorityUndefined = "cluster.authority_undefined"

// Authority describes who decides the canonical content for a path
// class. Each class declares exactly one authority. Mixed-authority
// classes are forbidden — they were the source of the webroot bug
// (was webroot supposed to be replicated by the controller, or
// served from objectstore? The lack of a declared answer is what
// let the drift run for two days).
type Authority string

const (
	// AuthorityNodeLocal: per-node by design. Nodes may legitimately
	// differ. Drift is never raised for this class.
	AuthorityNodeLocal Authority = "node_local"

	// AuthorityReplicated: every node must hold an identical copy.
	// Drift = any node disagrees with the majority hash.
	AuthorityReplicated Authority = "replicated"

	// AuthorityObjectstoreBacked: objectstore (MinIO) is the truth.
	// Local copies are caches: if a node has the file, its hash must
	// match the objectstore hash. Missing locally is OK (cache miss);
	// present-but-different is drift.
	AuthorityObjectstoreBacked Authority = "objectstore_backed"

	// AuthorityGeneratedFromEtcd: the file is rendered from current
	// etcd state. Every node's local copy must hash-match what
	// regeneration would produce (the "generated hash" passed in as
	// context). Missing locally is drift — generation should have
	// happened.
	AuthorityGeneratedFromEtcd Authority = "generated_from_etcd"

	// AuthorityGeneratedFromRepository: the file's content is the
	// installed artifact from the repository. The expected hash is
	// the artifact's published checksum. Every node that runs the
	// package must hold the matching hash; absent or mismatched is
	// drift.
	AuthorityGeneratedFromRepository Authority = "generated_from_repository"

	// AuthorityUndefined: no authority declared. The detector emits
	// FindingAuthorityUndefined and refuses to assess drift.
	AuthorityUndefined Authority = ""
)

// PathClass describes a class of files Globular tracks across the
// cluster. Each class is declared once with its authority.
type PathClass struct {
	// Name is the stable id (e.g. "webroot", "service_specs",
	// "rendered_systemd_units"). Used in finding evidence and doctor
	// rules.
	Name string
	// Authority is who decides canonical content. AuthorityUndefined
	// is allowed but raises FindingAuthorityUndefined when used.
	Authority Authority
	// Description is a one-line human explanation for operator UIs.
	Description string
}

// NodeObservation is one node's report about one path within a class.
// Many observations may share PathClass + Path (one per node).
type NodeObservation struct {
	NodeID    string
	PathClass string
	// Path is the logical identifier within the class (e.g. for class
	// "webroot": "index.html"; for class "rendered_systemd_units":
	// "globular-foo.service").
	Path string
	// SHA256 is the file's content hash. Empty when Present=false.
	// Normalized (lower-case, no "sha256:" prefix) by the caller.
	SHA256 string
	// Size in bytes. Optional but useful for triage.
	Size int64
	// Present is whether the node has the file at all.
	Present bool
	// Error is the collection error message when the node failed to
	// inspect the path (permission denied, IO error). Treated as
	// "unknown" — does not raise drift on its own.
	Error string
}

// AuthorityContext carries optional inputs the detector needs for
// non-replicated authorities. For replicated/node_local nothing is
// required.
type AuthorityContext struct {
	// ObjectstoreHash is the truth-source hash for objectstore-backed
	// classes. Empty means "objectstore does not have this path"; the
	// detector flags any node that holds a local copy as drifted.
	ObjectstoreHash string
	// GeneratedHash is the hash regeneration from current etcd or
	// repository state would produce. Used by generated_from_*
	// authorities. Empty raises drift for every node that has a copy
	// (because generation is supposed to have run and produced it).
	GeneratedHash string
}

// DriftVerdict is the per-path verdict the detector returns.
type DriftVerdict struct {
	PathClass string
	Path      string
	// Status: consistent | drift | authority_undefined | unknown.
	Status string
	// Drifts is one short line per discrepancy (e.g.
	// "node-a: present hash=abcd; node-b: absent").
	Drifts []string
	// FindingID names the failure_modes entry to raise. Empty when
	// Status=consistent or unknown.
	FindingID string
}

const (
	DriftStatusConsistent         = "consistent"
	DriftStatusDrift              = "drift"
	DriftStatusUnknown            = "unknown"
	DriftStatusAuthorityUndefined = "authority_undefined"
)

// DetectDrift compares per-node observations for a single path within
// a path class against the authority declared by the class. Returns a
// verdict whose Status names the outcome and whose Drifts list carries
// one line per discrepancy.
//
// Decision matrix:
//
//   - AuthorityUndefined → authority_undefined finding, immediately.
//
//   - AuthorityNodeLocal → consistent (per-node is legitimate by
//     design; no drift can be raised).
//
//   - AuthorityReplicated → consistent iff every node has the same
//     hash AND every node is present (or every node is absent — the
//     "file was deleted everywhere" steady state is also consistent).
//     A mix of present and absent, or any hash mismatch, is drift.
//
//   - AuthorityObjectstoreBacked: nodes that have a local copy must
//     hash-match ObjectstoreHash. Missing locally is OK (it's a cache).
//     If ObjectstoreHash is empty (objectstore has nothing), any
//     locally-present node is drifted.
//
//   - AuthorityGeneratedFrom*: every observation must agree with
//     GeneratedHash. Missing locally is drift — generation should
//     have happened. If GeneratedHash is empty, the generation step
//     itself never ran (caller's responsibility to populate); we
//     raise drift naming "generation_missing" rather than silently
//     pass.
//
// Collection errors (NodeObservation.Error != "") never raise drift
// on their own — the result is "unknown" for that node and the
// detector continues for the rest. If every observation is in error,
// the verdict is unknown.
func DetectDrift(class PathClass, path string, observations []NodeObservation, ctx AuthorityContext) DriftVerdict {
	v := DriftVerdict{PathClass: class.Name, Path: path}

	if class.Authority == AuthorityUndefined {
		v.Status = DriftStatusAuthorityUndefined
		v.FindingID = FindingAuthorityUndefined
		v.Drifts = []string{fmt.Sprintf("path class %q has no declared authority — drift cannot be assessed", class.Name)}
		return v
	}

	// Stable order for deterministic test output.
	sorted := append([]NodeObservation(nil), observations...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].NodeID < sorted[j].NodeID })

	// Drop pure-error observations into a separate bucket; if every
	// node erred we return unknown.
	var withData []NodeObservation
	var errored []NodeObservation
	for _, o := range sorted {
		if o.Error != "" {
			errored = append(errored, o)
		} else {
			withData = append(withData, o)
		}
	}
	if len(withData) == 0 {
		v.Status = DriftStatusUnknown
		for _, o := range errored {
			v.Drifts = append(v.Drifts, fmt.Sprintf("%s: %s", o.NodeID, o.Error))
		}
		return v
	}

	switch class.Authority {
	case AuthorityNodeLocal:
		v.Status = DriftStatusConsistent
		return v

	case AuthorityReplicated:
		return verdictReplicated(v, withData)

	case AuthorityObjectstoreBacked:
		return verdictObjectstoreBacked(v, withData, ctx.ObjectstoreHash)

	case AuthorityGeneratedFromEtcd, AuthorityGeneratedFromRepository:
		return verdictGenerated(v, withData, ctx.GeneratedHash)
	}

	// Unknown authority value — treat as undefined so we never
	// silently pass.
	v.Status = DriftStatusAuthorityUndefined
	v.FindingID = FindingAuthorityUndefined
	v.Drifts = []string{fmt.Sprintf("path class %q declared unknown authority %q", class.Name, class.Authority)}
	return v
}

func verdictReplicated(v DriftVerdict, obs []NodeObservation) DriftVerdict {
	// Two acceptable steady states: every node present with the same
	// hash, or every node absent. Anything else is drift.
	allAbsent := true
	allPresent := true
	hashCounts := make(map[string]int)
	for _, o := range obs {
		if o.Present {
			allAbsent = false
			h := normalizeHash(o.SHA256)
			hashCounts[h]++
		} else {
			allPresent = false
		}
	}
	if allAbsent {
		v.Status = DriftStatusConsistent
		return v
	}
	if !allPresent {
		v.Status = DriftStatusDrift
		v.FindingID = FindingID
		for _, o := range obs {
			if !o.Present {
				v.Drifts = append(v.Drifts, fmt.Sprintf("%s: absent", o.NodeID))
			} else {
				v.Drifts = append(v.Drifts, fmt.Sprintf("%s: present hash=%s", o.NodeID, shortHash(o.SHA256)))
			}
		}
		return v
	}
	// All nodes present — check hash agreement.
	if len(hashCounts) == 1 {
		v.Status = DriftStatusConsistent
		return v
	}
	v.Status = DriftStatusDrift
	v.FindingID = FindingID
	for _, o := range obs {
		v.Drifts = append(v.Drifts, fmt.Sprintf("%s: hash=%s", o.NodeID, shortHash(o.SHA256)))
	}
	return v
}

func verdictObjectstoreBacked(v DriftVerdict, obs []NodeObservation, objectstoreHash string) DriftVerdict {
	want := normalizeHash(objectstoreHash)
	var drifts []string
	for _, o := range obs {
		if !o.Present {
			continue // cache miss is legitimate
		}
		got := normalizeHash(o.SHA256)
		if want == "" {
			drifts = append(drifts, fmt.Sprintf("%s: holds local copy hash=%s but objectstore has no such object", o.NodeID, shortHash(o.SHA256)))
			continue
		}
		if got != want {
			drifts = append(drifts, fmt.Sprintf("%s: local hash=%s != objectstore hash=%s", o.NodeID, shortHash(o.SHA256), shortHash(objectstoreHash)))
		}
	}
	if len(drifts) == 0 {
		v.Status = DriftStatusConsistent
		return v
	}
	v.Status = DriftStatusDrift
	v.FindingID = FindingID
	v.Drifts = drifts
	return v
}

func verdictGenerated(v DriftVerdict, obs []NodeObservation, generatedHash string) DriftVerdict {
	want := normalizeHash(generatedHash)
	if want == "" {
		v.Status = DriftStatusDrift
		v.FindingID = FindingID
		v.Drifts = []string{"generation_missing: caller did not supply GeneratedHash — regeneration step never ran"}
		return v
	}
	var drifts []string
	for _, o := range obs {
		if !o.Present {
			drifts = append(drifts, fmt.Sprintf("%s: absent (generation should have produced hash=%s)", o.NodeID, shortHash(generatedHash)))
			continue
		}
		got := normalizeHash(o.SHA256)
		if got != want {
			drifts = append(drifts, fmt.Sprintf("%s: hash=%s != generated hash=%s", o.NodeID, shortHash(o.SHA256), shortHash(generatedHash)))
		}
	}
	if len(drifts) == 0 {
		v.Status = DriftStatusConsistent
		return v
	}
	v.Status = DriftStatusDrift
	v.FindingID = FindingID
	v.Drifts = drifts
	return v
}

func normalizeHash(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	return strings.TrimPrefix(h, "sha256:")
}

func shortHash(h string) string {
	n := normalizeHash(h)
	if n == "" {
		return "(empty)"
	}
	if len(n) <= 8 {
		return n
	}
	return n[:8]
}
