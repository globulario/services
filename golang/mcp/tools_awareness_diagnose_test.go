package main

import (
	"errors"
	"strings"
	"testing"

	awarenesspb "github.com/globulario/awareness-graph/golang/pb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// helpers ───────────────────────────────────────────────────────────────

func mkHints(symptom string) requestHints {
	return requestHints{
		symptom:  symptom,
		keywords: extractKeywords(symptom),
		mode:     "compact",
	}
}

func mkFinding(id, invariantID, sev, summary, entityRef string) *cluster_doctorpb.Finding {
	var s cluster_doctorpb.Severity
	switch sev {
	case "critical":
		s = cluster_doctorpb.Severity_SEVERITY_CRITICAL
	case "error":
		s = cluster_doctorpb.Severity_SEVERITY_ERROR
	case "warn":
		s = cluster_doctorpb.Severity_SEVERITY_WARN
	default:
		s = cluster_doctorpb.Severity_SEVERITY_INFO
	}
	return &cluster_doctorpb.Finding{
		FindingId:   id,
		InvariantId: invariantID,
		Severity:    s,
		Summary:     summary,
		EntityRef:   entityRef,
	}
}

func mustField(t *testing.T, m map[string]interface{}, key string) interface{} {
	t.Helper()
	v, ok := m[key]
	if !ok {
		t.Fatalf("expected key %q in response, got keys: %v", key, mapKeys(m))
	}
	return v
}

func mapKeys(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ────────────────────────────────────────────────────────────────────────
// 1. All backends down — degrades cleanly, forbidden_conclusions still present
// ────────────────────────────────────────────────────────────────────────

func TestDiagnose_AllBackendsDown(t *testing.T) {
	h := mkHints("service desired but not running on globule-ryzen")
	c := &collectedSources{
		briefingErr: errors.New("awareness-graph: dial failed"),
		doctorErr:   errors.New("doctor: dial failed"),
		driftErr:    errors.New("controller: dial failed"),
	}

	resp := buildDiagnoseResponse(h, c)

	if got := mustField(t, resp, "status").(string); got != "degraded" {
		t.Errorf("status: want degraded, got %q", got)
	}
	fc, ok := mustField(t, resp, "forbidden_conclusions").([]string)
	if !ok || len(fc) < 3 {
		t.Errorf("forbidden_conclusions missing or short: %v", fc)
	}
	bs := mustField(t, resp, "blind_spots").([]string)
	wantSubstrings := []string{"awareness-graph", "cluster_doctor", "drift"}
	for _, want := range wantSubstrings {
		found := false
		for _, b := range bs {
			if strings.Contains(b, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("blind_spots missing entry containing %q: %v", want, bs)
		}
	}
	tf := mustField(t, resp, "tool_failures").([]map[string]interface{})
	if len(tf) != 3 {
		t.Errorf("tool_failures: want 3 (one per source), got %d", len(tf))
	}
	// Authored context should be present-but-empty, not absent.
	authored := mustField(t, resp, "authored_context").(map[string]interface{})
	if authored["coverage_status"] != "degraded" {
		t.Errorf("authored.coverage_status: want degraded, got %v", authored["coverage_status"])
	}
}

// ────────────────────────────────────────────────────────────────────────
// 2. Only awareness available — runtime sections empty + labeled
// ────────────────────────────────────────────────────────────────────────

func TestDiagnose_AwarenessOnlyAvailable(t *testing.T) {
	h := mkHints("reconcile loop seems stuck")
	c := &collectedSources{
		briefing: &awarenesspb.BriefingResponse{
			Prose:         "briefing prose here",
			ReferencedIds: []string{"invariant:reconciler.resolution_must_match_spec"},
			Status:        awarenesspb.BriefingStatus_BRIEFING_STATUS_OK,
		},
		doctorErr: errors.New("doctor unreachable"),
		driftErr:  errors.New("drift unreachable"),
	}

	resp := buildDiagnoseResponse(h, c)
	if got := resp["status"].(string); got != "partial" {
		t.Errorf("status: want partial, got %q", got)
	}
	authored := resp["authored_context"].(map[string]interface{})
	if authored["coverage_status"] != "ok" {
		t.Errorf("authored.coverage_status: want ok, got %v", authored["coverage_status"])
	}
	runtime := resp["runtime_evidence"].(map[string]interface{})
	if df := runtime["doctor_findings"].([]map[string]interface{}); len(df) != 0 {
		t.Errorf("doctor_findings: want 0, got %d", len(df))
	}
	freshness := runtime["freshness"].(map[string]interface{})
	if doc, ok := freshness["doctor"].(map[string]interface{}); !ok || doc["status"] != "unavailable" {
		t.Errorf("doctor freshness: want {status:unavailable}, got %v", freshness["doctor"])
	}
}

// ────────────────────────────────────────────────────────────────────────
// 3. Strong invariant correlation — invariant_id overlap = high confidence
// ────────────────────────────────────────────────────────────────────────

func TestDiagnose_StrongInvariantCorrelation(t *testing.T) {
	h := mkHints("desired build_id not converging")
	invariantID := "controller.apply_package_release_requires_manifest_checksum"
	c := &collectedSources{
		briefing: &awarenesspb.BriefingResponse{
			Prose:         "briefing",
			ReferencedIds: []string{"invariant:" + invariantID, "intent:something_else"},
			Status:        awarenesspb.BriefingStatus_BRIEFING_STATUS_OK,
		},
		doctorReport: &cluster_doctorpb.ClusterReport{
			Findings: []*cluster_doctorpb.Finding{
				mkFinding("F-1", invariantID, "critical", "missing expected_sha256", "release/service/x"),
				mkFinding("F-2", "unrelated.invariant.id", "warn", "different thing", "release/service/y"),
			},
		},
	}

	resp := buildDiagnoseResponse(h, c)
	cf := resp["correlated_findings"].([]map[string]interface{})
	if len(cf) != 1 {
		t.Fatalf("correlated_findings: want exactly 1, got %d", len(cf))
	}
	got := cf[0]
	if got["confidence"] != "high" {
		t.Errorf("confidence: want high, got %v", got["confidence"])
	}
	if got["match_reason"] != "invariant_id_overlap" {
		t.Errorf("match_reason: want invariant_id_overlap, got %v", got["match_reason"])
	}
	if got["matched_awareness_id"] != "invariant:"+invariantID {
		t.Errorf("matched_awareness_id: want %q, got %v", "invariant:"+invariantID, got["matched_awareness_id"])
	}
	// F-2 should NOT appear in correlated (no invariant match, no hint).
	for _, c := range cf {
		if c["finding_id"] == "F-2" {
			t.Errorf("F-2 should not be correlated (no overlap)")
		}
	}
}

// ────────────────────────────────────────────────────────────────────────
// 4. Keyword-only overlap — NEVER goes into correlated_findings
// ────────────────────────────────────────────────────────────────────────

func TestDiagnose_KeywordOnlyNoFabrication(t *testing.T) {
	h := mkHints("manifest checksum verification problem")
	c := &collectedSources{
		briefing: &awarenesspb.BriefingResponse{
			Prose:         "briefing",
			ReferencedIds: []string{"invariant:totally.unrelated.thing"},
			Status:        awarenesspb.BriefingStatus_BRIEFING_STATUS_OK,
		},
		doctorReport: &cluster_doctorpb.ClusterReport{
			Findings: []*cluster_doctorpb.Finding{
				// Different invariant id, no hint match — only keyword overlap on
				// "manifest" and "checksum".
				mkFinding("F-K", "some.other.invariant", "warn",
					"manifest checksum mismatch detected", ""),
			},
		},
	}

	resp := buildDiagnoseResponse(h, c)
	cf := resp["correlated_findings"].([]map[string]interface{})
	for _, c := range cf {
		if c["finding_id"] == "F-K" {
			t.Errorf("F-K appeared in correlated_findings — keyword-only must NOT be correlated: %v", c)
		}
	}
	// Should land in possible_related_evidence instead.
	pr := resp["possible_related_evidence"].([]map[string]interface{})
	if len(pr) == 0 {
		t.Fatalf("possible_related_evidence empty; expected at least F-K")
	}
	found := false
	for _, p := range pr {
		if p["finding_id"] == "F-K" {
			found = true
			if p["confidence"] != "low" {
				t.Errorf("F-K confidence: want low, got %v", p["confidence"])
			}
			if p["match_reason"] != "keyword_overlap_only" {
				t.Errorf("F-K match_reason: want keyword_overlap_only, got %v", p["match_reason"])
			}
			if p["causal_implication"] != false {
				t.Errorf("F-K causal_implication must be false")
			}
		}
	}
	if !found {
		t.Errorf("F-K not in possible_related_evidence: %v", pr)
	}
}

// ────────────────────────────────────────────────────────────────────────
// 5. Bounded output — many findings, top-N by severity
// ────────────────────────────────────────────────────────────────────────

func TestDiagnose_BoundedOutput(t *testing.T) {
	h := mkHints("everything is broken")
	c := &collectedSources{
		doctorReport: &cluster_doctorpb.ClusterReport{},
	}
	// Inject 25 findings of mixed severity.
	for i := 0; i < 25; i++ {
		sev := "info"
		if i%5 == 0 {
			sev = "critical"
		} else if i%3 == 0 {
			sev = "error"
		}
		fid := "F-" + string(rune('A'+(i%26)))
		c.doctorReport.Findings = append(c.doctorReport.Findings,
			mkFinding(fid, "", sev, "summary "+fid, ""))
	}

	resp := buildDiagnoseResponse(h, c)
	runtime := resp["runtime_evidence"].(map[string]interface{})
	df := runtime["doctor_findings"].([]map[string]interface{})
	if len(df) != maxDoctorFindings {
		t.Errorf("doctor_findings size: want %d, got %d", maxDoctorFindings, len(df))
	}
	// First entry must be critical (top severity after sort).
	if df[0]["severity"] != "critical" {
		t.Errorf("first finding severity after sort: want critical, got %v", df[0]["severity"])
	}
	bs := resp["blind_spots"].([]string)
	foundTruncation := false
	for _, b := range bs {
		if strings.Contains(b, "only top") && strings.Contains(b, "by severity") {
			foundTruncation = true
			break
		}
	}
	if !foundTruncation {
		t.Errorf("blind_spots must surface truncation: %v", bs)
	}
}

// ────────────────────────────────────────────────────────────────────────
// 6. Forbidden conclusions ALWAYS present — regression guard
// ────────────────────────────────────────────────────────────────────────

func TestDiagnose_ForbiddenConclusionsAlwaysPresent(t *testing.T) {
	cases := []struct {
		name string
		c    *collectedSources
	}{
		{"all-down", &collectedSources{
			briefingErr: errors.New("x"), doctorErr: errors.New("x"), driftErr: errors.New("x"),
		}},
		{"all-ok-empty", &collectedSources{
			briefing:     &awarenesspb.BriefingResponse{},
			doctorReport: &cluster_doctorpb.ClusterReport{},
			driftReport:  &cluster_doctorpb.DriftReport{},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := buildDiagnoseResponse(mkHints("anything"), tc.c)
			fc := resp["forbidden_conclusions"].([]string)
			if len(fc) < 3 {
				t.Errorf("forbidden_conclusions must always have ≥3 entries, got %d", len(fc))
			}
		})
	}
}

// ────────────────────────────────────────────────────────────────────────
// 7. extractKeywords / keywordOverlap unit tests
// ────────────────────────────────────────────────────────────────────────

func TestExtractKeywords(t *testing.T) {
	got := extractKeywords("Service X failed to apply on node Y — desired state present but applied missing")
	want := map[string]bool{
		"service": true, "failed": true, "apply": true,
		"node": false, // too short (4-char min, "node" is exactly 4 so should be true)
		"desired": true, "state": true, "present": true,
		"applied": true, "missing": true,
	}
	want["node"] = true // exactly 4 chars — included
	got_set := map[string]bool{}
	for _, k := range got {
		got_set[k] = true
	}
	for w := range want {
		if want[w] && !got_set[w] {
			t.Errorf("extractKeywords missing keyword %q: %v", w, got)
		}
	}
	// Single-letter "X" / "Y" / "to" / "on" should not appear.
	for _, k := range got {
		if len(k) < keywordMinLen {
			t.Errorf("keyword %q is shorter than keywordMinLen=%d", k, keywordMinLen)
		}
	}
}

func TestKeywordOverlap_ScoreThreshold(t *testing.T) {
	keywords := extractKeywords("manifest checksum verification problem")
	// Has both "manifest" and "checksum" — 2 matches.
	overlap := keywordOverlap(keywords, "manifest checksum mismatch")
	if len(overlap) < keywordMinTermsForLo {
		t.Errorf("expected ≥%d overlaps, got %v", keywordMinTermsForLo, overlap)
	}
	// Only "verification" — 1 match (below threshold).
	overlap1 := keywordOverlap(keywords, "verification step failed")
	if len(overlap1) >= keywordMinTermsForLo {
		t.Errorf("expected <%d overlaps, got %v", keywordMinTermsForLo, overlap1)
	}
}

// ────────────────────────────────────────────────────────────────────────
// 8. splitClassID — handles malformed and normal inputs
// ────────────────────────────────────────────────────────────────────────

func TestSplitClassID(t *testing.T) {
	cases := []struct {
		in        string
		wantBare  string
		wantClass string
	}{
		{"invariant:foo.bar", "foo.bar", "invariant"},
		{"failure_mode:x.y.z", "x.y.z", "failure_mode"},
		{"intent:reconciler.foo", "reconciler.foo", "intent"},
		{"malformed", "", ""},
		{":no-class", "", ""},
		{"no-id:", "", ""},
		{"", "", ""},
	}
	for _, tc := range cases {
		bare, class := splitClassID(tc.in)
		if bare != tc.wantBare || class != tc.wantClass {
			t.Errorf("splitClassID(%q): got (%q, %q), want (%q, %q)",
				tc.in, bare, class, tc.wantBare, tc.wantClass)
		}
	}
}
