package main

import (
	"context"
	"testing"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

func TestEmitBehavioralClusterReportEmitsFindings(t *testing.T) {
	origEmit := emitBehavioralDoctorFindings
	defer func() { emitBehavioralDoctorFindings = origEmit }()

	done := make(chan struct{}, 1)
	emitBehavioralDoctorFindings = func(ctx context.Context, clusterID string, header *cluster_doctorpb.ReportHeader, findings []*cluster_doctorpb.Finding) {
		if clusterID != "cluster-1" {
			t.Errorf("cluster_id=%q", clusterID)
		}
		if header.GetSource() != "cluster-doctor" {
			t.Errorf("source=%q", header.GetSource())
		}
		if len(findings) != 1 || findings[0].GetFindingId() != "finding-1" {
			t.Errorf("unexpected findings: %+v", findings)
		}
		done <- struct{}{}
	}

	srv := &ClusterDoctorServer{clusterID: "cluster-1"}
	report := &cluster_doctorpb.ClusterReport{
		Header: &cluster_doctorpb.ReportHeader{Source: "cluster-doctor"},
		Findings: []*cluster_doctorpb.Finding{{
			FindingId: "finding-1",
		}},
	}

	srv.emitBehavioralClusterReport(report)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected doctor finding emission")
	}
}
