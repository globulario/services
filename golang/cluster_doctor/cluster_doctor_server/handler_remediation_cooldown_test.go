package main

import (
	"testing"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

func TestAutoRemediationCooldown_AllowsFirstBlocksSecond(t *testing.T) {
	key := autoRemediationCooldownKey("finding-1", 0, cluster_doctorpb.ActionType_SYSTEMCTL_RESTART)
	autoRemediationCooldownByTarget.Delete(key)

	now := time.Now()
	ok, _ := allowAutoRemediationNow(key, now)
	if !ok {
		t.Fatal("first auto-remediation should be allowed")
	}

	ok, wait := allowAutoRemediationNow(key, now.Add(5*time.Second))
	if ok {
		t.Fatal("second auto-remediation should be blocked by cooldown")
	}
	if wait <= 0 {
		t.Fatalf("expected positive wait duration, got %s", wait)
	}
}

func TestAutoRemediationCooldown_Expires(t *testing.T) {
	key := autoRemediationCooldownKey("finding-2", 1, cluster_doctorpb.ActionType_FILE_DELETE)
	autoRemediationCooldownByTarget.Delete(key)

	now := time.Now()
	ok, _ := allowAutoRemediationNow(key, now)
	if !ok {
		t.Fatal("first auto-remediation should be allowed")
	}

	ok, _ = allowAutoRemediationNow(key, now.Add(autoRemediationCooldown+time.Second))
	if !ok {
		t.Fatal("auto-remediation after cooldown should be allowed")
	}
}
