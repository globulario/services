package main

import (
	"context"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	"github.com/globulario/services/golang/ai_watcher/ai_watcherpb"
	"github.com/globulario/services/golang/security"
)

const (
	behavioralProject = "globular-services"
	behavioralDomain  = "cluster_operator"
)

var emitBehavioralWatcherIncident = func(ctx context.Context, incident *ai_watcherpb.Incident) {
	if incident == nil {
		return
	}
	clusterID, _ := security.GetLocalClusterID()
	bundle := observation.FromWatcherIncident(behavioralProject, api.DomainRef(behavioralDomain), clusterID, incident)
	if err := observation.RecordBundle(ctx, bundle); err != nil {
		logger.Debug("behavioral observation: watcher incident skipped", "incident", incident.GetId(), "err", err)
	}
}
