package main

import (
	"context"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

const (
	behavioralProject = "globular-services"
	behavioralDomain  = "cluster_operator"
)

var emitBehavioralDoctorFindings = func(ctx context.Context, clusterID string, header *cluster_doctorpb.ReportHeader, findings []*cluster_doctorpb.Finding) {
	for _, finding := range findings {
		if finding == nil {
			continue
		}
		bundle := observation.FromDoctorFinding(behavioralProject, api.DomainRef(behavioralDomain), clusterID, header, finding)
		if err := observation.RecordBundle(ctx, bundle); err != nil {
			logger.Debug("behavioral observation: doctor finding skipped", "finding", finding.GetFindingId(), "err", err)
		}
	}
}

func (s *ClusterDoctorServer) emitBehavioralClusterReport(report *cluster_doctorpb.ClusterReport) {
	if report == nil || len(report.GetFindings()) == 0 {
		return
	}
	go emitBehavioralDoctorFindings(context.Background(), s.clusterID, report.GetHeader(), report.GetFindings())
}
