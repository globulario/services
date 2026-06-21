package main

import (
	"context"
	"testing"
	"time"

	"github.com/globulario/services/golang/ai_watcher/ai_watcherpb"
)

func TestFireBatchEmitsBehavioralWatcherIncident(t *testing.T) {
	origEmit := emitBehavioralWatcherIncident
	defer func() { emitBehavioralWatcherIncident = origEmit }()

	observed := make(chan *ai_watcherpb.Incident, 1)
	emitBehavioralWatcherIncident = func(ctx context.Context, incident *ai_watcherpb.Incident) {
		observed <- incident
	}

	srv := &server{
		eventBatch: map[string][]string{
			"rule-1": []string{"cluster.service.failed"},
		},
		eventBatchData: map[string][]byte{},
		batchTimers:    map[string]*time.Timer{},
		lastTrigger:    map[string]time.Time{},
		incidents:      map[string]*ai_watcherpb.Incident{},
		triggerDataMap: map[string][]byte{},
	}
	rule := &ai_watcherpb.EventRule{
		Id:              "rule-1",
		Tier:            ai_watcherpb.PermissionTier_OBSERVE,
		RepeatThreshold: 1,
	}

	srv.fireBatch(rule)

	select {
	case incident := <-observed:
		if incident.GetTriggerEvent() != "cluster.service.failed" {
			t.Fatalf("trigger_event=%q", incident.GetTriggerEvent())
		}
		if incident.GetMetadata()["rule_id"] != "rule-1" {
			t.Fatalf("rule_id=%q", incident.GetMetadata()["rule_id"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected watcher incident emission")
	}
}
