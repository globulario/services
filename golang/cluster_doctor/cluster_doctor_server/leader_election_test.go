package main

import (
	"testing"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// TestAuthoritySourceLeader verifies that the elected leader reports "leader".
func TestAuthoritySourceLeader(t *testing.T) {
	srv := &ClusterDoctorServer{}
	srv.isAuthoritative.Store(true)
	if got := srv.authoritySource(); got != "leader" {
		t.Errorf("authoritySource() = %q, want %q", got, "leader")
	}
}

// TestAuthoritySourceFollower verifies that a non-leader reports "follower".
func TestAuthoritySourceFollower(t *testing.T) {
	srv := &ClusterDoctorServer{}
	srv.isAuthoritative.Store(false)
	if got := srv.authoritySource(); got != "follower" {
		t.Errorf("authoritySource() = %q, want %q", got, "follower")
	}
}

// TestFollowerBlocksRemediation verifies that a follower rejects
// ExecuteRemediation with FailedPrecondition.
func TestFollowerBlocksRemediation(t *testing.T) {
	srv := &ClusterDoctorServer{}
	srv.isAuthoritative.Store(false)

	_, err := srv.ExecuteRemediation(nil, &cluster_doctorpb.ExecuteRemediationRequest{
		FindingId: "test-finding",
	})
	if err == nil {
		t.Fatal("expected error for follower remediation")
	}
	if got := err.Error(); !containsStr(got, "not leader") {
		t.Errorf("error should mention 'not leader', got: %s", got)
	}
}

// TestFollowerBlocksWorkflow verifies that a follower rejects
// StartRemediationWorkflow with FailedPrecondition.
func TestFollowerBlocksWorkflow(t *testing.T) {
	srv := &ClusterDoctorServer{}
	srv.isAuthoritative.Store(false)

	_, err := srv.StartRemediationWorkflow(nil, &cluster_doctorpb.StartRemediationWorkflowRequest{
		FindingId: "test-finding",
	})
	if err == nil {
		t.Fatal("expected error for follower workflow")
	}
	if got := err.Error(); !containsStr(got, "not leader") {
		t.Errorf("error should mention 'not leader', got: %s", got)
	}
}

// TestLeaderAllowsRemediation verifies that the leader does not reject
// based on authority (it may fail for other reasons like missing finding).
func TestLeaderAllowsRemediation(t *testing.T) {
	srv := &ClusterDoctorServer{}
	srv.isAuthoritative.Store(true)

	// Will fail because finding is not in cache, but should NOT fail
	// with "not leader".
	_, err := srv.ExecuteRemediation(nil, &cluster_doctorpb.ExecuteRemediationRequest{
		FindingId: "nonexistent",
	})
	if err != nil && containsStr(err.Error(), "not leader") {
		t.Error("leader should not be rejected for authority")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
