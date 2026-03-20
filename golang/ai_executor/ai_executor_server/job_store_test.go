package main

import (
	"testing"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
)

func testJobStore() *jobStore {
	return &jobStore{
		jobs: make(map[string]*ai_executorpb.Job),
		// No etcd — pure in-memory for tests.
	}
}

func testDiagnosis(rootCause, action string) *ai_executorpb.Diagnosis {
	return &ai_executorpb.Diagnosis{
		IncidentId:     "test-incident",
		Summary:        "test diagnosis",
		RootCause:      rootCause,
		ProposedAction: action,
		Confidence:     0.8,
	}
}

// --- Approval triggers execution ---

func TestApprovalTriggersExecution(t *testing.T) {
	js := testJobStore()
	diag := testDiagnosis("service_crash", "restart_service:echo")

	// Create Tier 2 job — should start in AWAITING_APPROVAL.
	job := js.createJob("inc-1", "service-crash", 2, diag)
	if job.State != ai_executorpb.JobState_JOB_AWAITING_APPROVAL {
		t.Fatalf("expected AWAITING_APPROVAL, got %s", job.State)
	}
	if job.ExpiresAtMs == 0 {
		t.Fatal("expected expiry to be set for Tier 2 job")
	}

	// Approve.
	approved, err := js.approve("inc-1", "admin")
	if err != nil {
		t.Fatalf("approve failed: %v", err)
	}
	if approved.State != ai_executorpb.JobState_JOB_APPROVED {
		t.Fatalf("expected APPROVED, got %s", approved.State)
	}
	if approved.ApprovedBy != "admin" {
		t.Fatalf("expected approvedBy=admin, got %s", approved.ApprovedBy)
	}

	// Mark executing.
	executing, err := js.markExecuting("inc-1")
	if err != nil {
		t.Fatalf("markExecuting failed: %v", err)
	}
	if executing.State != ai_executorpb.JobState_JOB_EXECUTING {
		t.Fatalf("expected EXECUTING, got %s", executing.State)
	}
	if executing.Attempts != 1 {
		t.Fatalf("expected attempts=1, got %d", executing.Attempts)
	}

	// Mark succeeded.
	succeeded, err := js.markResult("inc-1", true, "restarted", "")
	if err != nil {
		t.Fatalf("markResult failed: %v", err)
	}
	if succeeded.State != ai_executorpb.JobState_JOB_SUCCEEDED {
		t.Fatalf("expected SUCCEEDED, got %s", succeeded.State)
	}
}

// --- Denial prevents execution ---

func TestDenialPreventsExecution(t *testing.T) {
	js := testJobStore()
	diag := testDiagnosis("service_crash", "restart_service:echo")

	js.createJob("inc-2", "service-crash", 2, diag)

	denied, err := js.deny("inc-2", "operator", "not safe right now")
	if err != nil {
		t.Fatalf("deny failed: %v", err)
	}
	if denied.State != ai_executorpb.JobState_JOB_DENIED {
		t.Fatalf("expected DENIED, got %s", denied.State)
	}
	if denied.DeniedBy != "operator" {
		t.Fatalf("expected deniedBy=operator, got %s", denied.DeniedBy)
	}
	if denied.DeniedReason != "not safe right now" {
		t.Fatalf("expected reason, got %s", denied.DeniedReason)
	}

	// Attempting to approve a denied job should fail.
	_, err = js.approve("inc-2", "admin")
	if err == nil {
		t.Fatal("expected error approving denied job")
	}
}

// --- Expiry prevents execution ---

func TestExpiryPreventsExecution(t *testing.T) {
	js := testJobStore()
	diag := testDiagnosis("service_crash", "restart_service:echo")

	job := js.createJob("inc-3", "service-crash", 2, diag)

	// Force expiry by setting it to the past.
	js.mu.Lock()
	job.ExpiresAtMs = time.Now().Add(-1 * time.Minute).UnixMilli()
	js.mu.Unlock()

	// Approve should fail with expiry.
	_, err := js.approve("inc-3", "admin")
	if err == nil {
		t.Fatal("expected error approving expired job")
	}

	// Job should be marked EXPIRED.
	j := js.getJob("inc-3")
	if j.State != ai_executorpb.JobState_JOB_EXPIRED {
		t.Fatalf("expected EXPIRED, got %s", j.State)
	}
}

// --- Expiry via background checker ---

func TestExpireStaleApprovals(t *testing.T) {
	js := testJobStore()
	diag := testDiagnosis("service_crash", "restart_service:echo")

	job := js.createJob("inc-4", "service-crash", 2, diag)

	// Force expiry.
	js.mu.Lock()
	job.ExpiresAtMs = time.Now().Add(-1 * time.Minute).UnixMilli()
	js.mu.Unlock()

	expired := js.expireStaleApprovals()
	if expired != 1 {
		t.Fatalf("expected 1 expired, got %d", expired)
	}

	j := js.getJob("inc-4")
	if j.State != ai_executorpb.JobState_JOB_EXPIRED {
		t.Fatalf("expected EXPIRED, got %s", j.State)
	}
}

// --- Idempotency ---

func TestIdempotency(t *testing.T) {
	js := testJobStore()
	diag := testDiagnosis("service_crash", "restart_service:echo")

	job1 := js.createJob("inc-5", "service-crash", 0, diag)
	job2 := js.createJob("inc-5", "service-crash", 0, diag)

	// Same pointer — second call returns cached job.
	if job1 != job2 {
		t.Fatal("expected same job instance for duplicate incident ID")
	}

	// Only one job in store.
	jobs := js.listJobs(0, 100)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
}

// --- Failure handling ---

func TestFailureHandling(t *testing.T) {
	js := testJobStore()
	diag := testDiagnosis("service_crash", "restart_service:echo")

	js.createJob("inc-6", "service-crash", 1, diag)

	js.markExecuting("inc-6")

	failed, err := js.markResult("inc-6", false, "", "systemctl restart failed: unit not found")
	if err != nil {
		t.Fatalf("markResult failed: %v", err)
	}
	if failed.State != ai_executorpb.JobState_JOB_FAILED {
		t.Fatalf("expected FAILED, got %s", failed.State)
	}
	if failed.Error != "systemctl restart failed: unit not found" {
		t.Fatalf("expected error message, got %s", failed.Error)
	}
}

// --- Tier 0 observe only ---

func TestTier0ObserveOnly(t *testing.T) {
	js := testJobStore()
	diag := testDiagnosis("service_crash", "observe_and_record")

	job := js.createJob("inc-7", "service-crash", 0, diag)

	// Tier 0 starts as DIAGNOSED (not AWAITING_APPROVAL).
	if job.State != ai_executorpb.JobState_JOB_DIAGNOSED {
		t.Fatalf("expected DIAGNOSED for Tier 0, got %s", job.State)
	}

	// Mark succeeded directly (observe records and closes).
	js.updateState("inc-7", ai_executorpb.JobState_JOB_SUCCEEDED)
	j := js.getJob("inc-7")
	if j.State != ai_executorpb.JobState_JOB_SUCCEEDED {
		t.Fatalf("expected SUCCEEDED, got %s", j.State)
	}
}

// --- List jobs with state filter ---

func TestListJobsFilter(t *testing.T) {
	js := testJobStore()

	js.createJob("a", "r1", 0, testDiagnosis("c1", "a1"))
	js.createJob("b", "r2", 2, testDiagnosis("c2", "a2"))
	js.createJob("c", "r3", 0, testDiagnosis("c3", "a3"))

	// "b" should be AWAITING_APPROVAL (Tier 2).
	awaiting := js.listJobs(ai_executorpb.JobState_JOB_AWAITING_APPROVAL, 10)
	if len(awaiting) != 1 {
		t.Fatalf("expected 1 awaiting, got %d", len(awaiting))
	}
	if awaiting[0].IncidentId != "b" {
		t.Fatalf("expected incident b, got %s", awaiting[0].IncidentId)
	}

	// All jobs.
	all := js.listJobs(0, 10)
	if len(all) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(all))
	}
}

// --- Approve idempotency ---

func TestApproveIdempotency(t *testing.T) {
	js := testJobStore()
	diag := testDiagnosis("service_crash", "restart_service:echo")

	js.createJob("inc-8", "service-crash", 2, diag)

	// First approve.
	j1, err := js.approve("inc-8", "admin")
	if err != nil {
		t.Fatalf("first approve failed: %v", err)
	}

	// Second approve — should return same job without error.
	j2, err := js.approve("inc-8", "other-admin")
	if err != nil {
		t.Fatalf("second approve failed: %v", err)
	}

	// ApprovedBy should still be "admin" (first approver).
	if j2.ApprovedBy != "admin" {
		t.Fatalf("expected approvedBy=admin (first approver), got %s", j2.ApprovedBy)
	}
	if j1.ActionId != j2.ActionId {
		t.Fatal("expected same job")
	}
}
