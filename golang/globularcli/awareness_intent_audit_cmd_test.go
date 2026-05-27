package main

import (
	"context"
	"testing"
	"time"

	"github.com/globulario/services/golang/awareness/intentaudit"
)

type fakeRuntimeEvidenceProvider struct{}

func (fakeRuntimeEvidenceProvider) GetJSON(context.Context, string) ([]byte, error) { return nil, nil }
func (fakeRuntimeEvidenceProvider) ListKeys(context.Context, string) ([]string, error) {
	return nil, nil
}

func TestCheckFailOn_None(t *testing.T) {
	report := &intentaudit.AuditReport{
		Summary: intentaudit.AuditSummary{
			CandidateViolation: 5,
			MissingTest:        3,
			TestCoverageGap:    10,
		},
	}
	if err := checkFailOn(report, "none"); err != nil {
		t.Errorf("--fail-on none should always pass: %v", err)
	}
}

func TestCheckFailOn_Violation(t *testing.T) {
	clean := &intentaudit.AuditReport{
		Summary: intentaudit.AuditSummary{Pass: 5, TestCoverageGap: 10},
	}
	if err := checkFailOn(clean, "violation"); err != nil {
		t.Errorf("no violations should pass: %v", err)
	}

	dirty := &intentaudit.AuditReport{
		Summary: intentaudit.AuditSummary{CandidateViolation: 1},
	}
	if err := checkFailOn(dirty, "violation"); err == nil {
		t.Error("violations present should fail")
	}
}

func TestCheckFailOn_MissingTest(t *testing.T) {
	gapOnly := &intentaudit.AuditReport{
		Summary: intentaudit.AuditSummary{TestCoverageGap: 10},
	}
	if err := checkFailOn(gapOnly, "missing-test"); err != nil {
		t.Errorf("gaps alone should pass in missing-test mode: %v", err)
	}

	missing := &intentaudit.AuditReport{
		Summary: intentaudit.AuditSummary{MissingTest: 1},
	}
	if err := checkFailOn(missing, "missing-test"); err == nil {
		t.Error("missing tests should fail in missing-test mode")
	}
}

func TestCheckFailOn_Gap(t *testing.T) {
	clean := &intentaudit.AuditReport{
		Summary: intentaudit.AuditSummary{Pass: 5, AcceptedException: 2},
	}
	if err := checkFailOn(clean, "gap"); err != nil {
		t.Errorf("no gaps should pass: %v", err)
	}

	hasGap := &intentaudit.AuditReport{
		Summary: intentaudit.AuditSummary{TestCoverageGap: 1},
	}
	if err := checkFailOn(hasGap, "gap"); err == nil {
		t.Error("gaps should fail in gap mode")
	}
}

func TestRunRuntimeEvidence_UsesConfiguredTimeout(t *testing.T) {
	prevProvider := newRuntimeEvidenceProvider
	prevEval := evaluateDesiredBuildImmutability
	t.Cleanup(func() {
		newRuntimeEvidenceProvider = prevProvider
		evaluateDesiredBuildImmutability = prevEval
	})

	newRuntimeEvidenceProvider = func() (intentaudit.RuntimeEvidenceProvider, error) {
		return fakeRuntimeEvidenceProvider{}, nil
	}

	evaluateDesiredBuildImmutability = func(ctx context.Context, _ intentaudit.RuntimeEvidenceProvider) intentaudit.RuntimeResult {
		dl, ok := ctx.Deadline()
		if !ok {
			t.Fatal("expected runtime context deadline to be set")
		}
		remaining := time.Until(dl)
		if remaining < 1500*time.Millisecond || remaining > 2500*time.Millisecond {
			t.Fatalf("expected timeout near 2s, got %v remaining", remaining)
		}
		return intentaudit.RuntimeResult{Status: "pass", Detail: "ok"}
	}

	report := &intentaudit.AuditReport{}
	runRuntimeEvidence(report, "2s")
	if len(report.RuntimeEvidence) != 3 {
		t.Fatalf("expected three runtime results, got %d", len(report.RuntimeEvidence))
	}
}
