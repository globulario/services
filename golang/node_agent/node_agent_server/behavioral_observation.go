package main

import (
	"context"
	"log"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

const (
	behavioralProject = "globular-services"
	behavioralDomain  = "cluster_operator"
)

var emitBehavioralInfraProbe = func(ctx context.Context, clusterID string, probe *cluster_controllerpb.InfraProbeResult) {
	if probe == nil {
		return
	}
	bundle := observation.FromInfraProbe(behavioralProject, api.DomainRef(behavioralDomain), clusterID, probe)
	if err := observation.RecordBundle(ctx, bundle); err != nil {
		log.Printf("behavioral observation: infra_probe skipped component=%s node=%s: %v", probe.GetComponent(), probe.GetNodeId(), err)
	}
}
