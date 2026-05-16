package main

import (
	"testing"

	"github.com/globulario/awareness/assurance"
)

func TestCriticalOrphanCount(t *testing.T) {
	report := &assurance.CoverageReport{
		PerFailureMode: []assurance.FailureModeCoverage{
			{ID: "fm1", State: "ORPHAN", Severity: "critical"},
			{ID: "fm2", State: "ORPHAN", Severity: "high"},
			{ID: "fm3", State: "ENFORCED", Severity: "critical"},
		},
	}
	if got := criticalOrphanCount(report); got != 1 {
		t.Fatalf("criticalOrphanCount=%d want 1", got)
	}
}

func TestRepresentativeFailureModePrefersOrphan(t *testing.T) {
	report := &assurance.CoverageReport{
		PerFailureMode: []assurance.FailureModeCoverage{
			{ID: "fm-enf", State: "ENFORCED"},
			{ID: "fm-orphan", State: "ORPHAN"},
			{ID: "fm-partial", State: "PARTIAL"},
		},
	}
	got := representativeFailureMode(report)
	if got == nil || got.ID != "fm-orphan" {
		t.Fatalf("representativeFailureMode=%v want fm-orphan", got)
	}
}

func TestLifecycleCounts(t *testing.T) {
	report := &assurance.CoverageReport{
		PerFailureMode: []assurance.FailureModeCoverage{
			{State: "ORPHAN"},
			{State: "ORPHAN"},
			{State: "ENFORCED"},
			{State: "PARTIAL"},
		},
	}
	got := lifecycleCounts(report)
	if got["ORPHAN"] != 2 || got["ENFORCED"] != 1 || got["PARTIAL"] != 1 {
		t.Fatalf("unexpected lifecycle counts: %+v", got)
	}
}

func TestClassifyOrphanRowExtractionOrphan(t *testing.T) {
	row := classifyOrphanRow(assurance.FailureModeCoverage{
		ID:              "fm.x",
		Title:           "fm x",
		Severity:        "critical",
		Mitigations:     0,
		Tests:           0,
		Detectors:       0,
		LearningEntries: 1,
		DecisionPaths:   0,
		State:           "ORPHAN",
	}, map[string]int{"fm x": 1}, false)
	if row.LikelyCause != "extraction_orphan" {
		t.Fatalf("likely cause=%s want extraction_orphan", row.LikelyCause)
	}
}

func TestClassifyOrphanRowPossibleDuplicate(t *testing.T) {
	row := classifyOrphanRow(assurance.FailureModeCoverage{
		ID:       "fm.dup",
		Title:    "same title",
		State:    "ORPHAN",
		Severity: "high",
	}, map[string]int{"same title": 2}, false)
	if row.LikelyCause != "possible_duplicate" {
		t.Fatalf("likely cause=%s want possible_duplicate", row.LikelyCause)
	}
}

func TestClassifyOrphanRowSourceRefsMarksExtractionOrphan(t *testing.T) {
	row := classifyOrphanRow(assurance.FailureModeCoverage{
		ID:       "fm.src",
		Title:    "src",
		State:    "ORPHAN",
		Severity: "high",
	}, map[string]int{"src": 1}, true)
	if row.LikelyCause != "extraction_orphan" {
		t.Fatalf("likely cause=%s want extraction_orphan", row.LikelyCause)
	}
}

// TestDetectedCount counts failure_modes whose lifecycle is DETECTED or
// ENFORCED. Other states (PARTIAL, TESTED, ORPHAN, etc.) must not contribute.
func TestDetectedCount(t *testing.T) {
	report := &assurance.CoverageReport{
		PerFailureMode: []assurance.FailureModeCoverage{
			{ID: "a", State: "DETECTED"},
			{ID: "b", State: "ENFORCED"},
			{ID: "c", State: "TESTED"},
			{ID: "d", State: "PARTIAL"},
			{ID: "e", State: "ORPHAN"},
			{ID: "f", State: ""},
		},
	}
	if got := detectedCount(report); got != 2 {
		t.Errorf("detectedCount=%d, want 2", got)
	}
}

// TestExitOnMetaCheckGate_MinWellCovered: the well_covered ratchet fires when
// the current count is below the floor. Above the floor must not gate.
func TestExitOnMetaCheckGate_MinWellCovered(t *testing.T) {
	c := &assurance.CoverageReport{
		WellCoveredCount: 5,
		PerFailureMode:   []assurance.FailureModeCoverage{},
	}
	s := &assurance.Staleness{}

	// Reset every flag we touch so cross-test pollution doesn't bite.
	defer func() { awarenessMetaCheckCfg.minWellCovered = 0 }()

	awarenessMetaCheckCfg.minWellCovered = 0
	if exitOnMetaCheckGate(c, s) {
		t.Errorf("gate should not fire when --min-well-covered is unset")
	}

	awarenessMetaCheckCfg.minWellCovered = 5
	if exitOnMetaCheckGate(c, s) {
		t.Errorf("gate should not fire when count == floor")
	}

	awarenessMetaCheckCfg.minWellCovered = 6
	if !exitOnMetaCheckGate(c, s) {
		t.Errorf("gate must fire when well_covered_count (5) drops below floor (6)")
	}
}

// TestExitOnMetaCheckGate_MinDetected: the detected ratchet fires when the
// current DETECTED+ENFORCED count is below the floor.
func TestExitOnMetaCheckGate_MinDetected(t *testing.T) {
	c := &assurance.CoverageReport{
		PerFailureMode: []assurance.FailureModeCoverage{
			{ID: "a", State: "DETECTED"},
			{ID: "b", State: "ENFORCED"},
			{ID: "c", State: "TESTED"},
		},
	}
	s := &assurance.Staleness{}

	defer func() { awarenessMetaCheckCfg.minDetected = 0 }()

	awarenessMetaCheckCfg.minDetected = 2
	if exitOnMetaCheckGate(c, s) {
		t.Errorf("gate should not fire when count (2) >= floor (2)")
	}

	awarenessMetaCheckCfg.minDetected = 3
	if !exitOnMetaCheckGate(c, s) {
		t.Errorf("gate must fire when detected_count (2) < floor (3)")
	}
}
