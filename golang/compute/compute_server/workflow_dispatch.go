// workflow_dispatch.go connects the compute service to the workflow engine.
//
// On startup, it publishes the compute workflow definitions to MinIO so the
// workflow service can find them. When a job is submitted, it calls
// WorkflowService.ExecuteWorkflow to drive the full orchestration path.
//
// The compute service passes its own address as the actor_endpoint so the
// workflow engine can call back via ExecuteAction for each step.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/compute/computepb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/workflow/workflowpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// publishWorkflowDefinitions uploads the compute workflow definitions to MinIO
// so the workflow service can load them by name. Reads from the well-known
// definitions directory. Idempotent — safe to call on every startup.
func publishWorkflowDefinitions() {
	// Locate definitions relative to the binary or in the well-known path.
	searchPaths := []string{
		"/var/lib/globular/workflows",
		filepath.Join(filepath.Dir(os.Args[0]), "..", "workflows"),
	}

	defs := []struct {
		filename string
		minioKey string
	}{
		{"compute.job.submit.yaml", "workflows/compute.job.submit.yaml"},
		{"compute.unit.execute.yaml", "workflows/compute.unit.execute.yaml"},
		{"compute.job.aggregate.yaml", "workflows/compute.job.aggregate.yaml"},
	}

	for _, d := range defs {
		var data []byte
		var err error
		for _, dir := range searchPaths {
			data, err = os.ReadFile(filepath.Join(dir, d.filename))
			if err == nil {
				break
			}
		}
		if err != nil {
			slog.Warn("compute: workflow definition not found on disk, skipping",
				"file", d.filename)
			continue
		}
		if err := config.PutClusterConfig(d.minioKey, data); err != nil {
			slog.Warn("compute: failed to publish workflow definition",
				"key", d.minioKey, "err", err)
			continue
		}
		slog.Info("compute: workflow definition published", "key", d.minioKey)
	}
}

// connectWorkflowService dials the workflow service and returns a client.
// The endpoint is resolved via etcd service discovery.
func connectWorkflowService() (workflowpb.WorkflowServiceClient, error) {
	addr := config.ResolveServiceAddr("workflow.WorkflowService", "")
	if addr == "" {
		return nil, fmt.Errorf("workflow service not found via service discovery")
	}

	dt := config.ResolveDialTarget(addr)

	certFile := "/var/lib/globular/pki/issued/services/service.crt"
	keyFile := "/var/lib/globular/pki/issued/services/service.key"
	caFile := "/var/lib/globular/pki/ca.crt"

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load cert: %w", err)
	}
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read CA: %w", err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)

	creds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		ServerName:   dt.ServerName,
	})

	token, _ := security.GetLocalToken("")
	opts := []grpc.DialOption{grpc.WithTransportCredentials(creds)}
	if token != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(runnerTokenAuth{token: token}))
	}

	conn, err := grpc.NewClient(dt.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("dial workflow service at %s: %w", dt.Address, err)
	}
	return workflowpb.NewWorkflowServiceClient(conn), nil
}

// resolveComputeEndpoint returns this compute service's address as registered
// in etcd, for use as the actor callback endpoint.
func resolveComputeEndpoint() string {
	return config.ResolveLocalServiceAddr("compute.ComputeService")
}

// executeViaWorkflow submits the compute.job.submit workflow to the workflow
// service, which orchestrates the full job lifecycle: admit → create unit →
// dispatch execution → aggregate → finalize.
func (srv *server) executeViaWorkflow(def *computepb.ComputeDefinition, job *computepb.ComputeJob, unit *computepb.ComputeUnit) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	wfClient, err := connectWorkflowService()
	if err != nil {
		slog.Error("compute: workflow service unavailable, job will not execute",
			"job_id", job.JobId, "err", err)
		srv.failJob(ctx, job, unit, fmt.Sprintf("workflow service unavailable: %v", err))
		return
	}

	// The compute service is both the orchestrator and the actor — the
	// workflow engine calls back to us via ExecuteAction for each step.
	computeEndpoint := resolveComputeEndpoint()
	if computeEndpoint == "" {
		slog.Error("compute: cannot resolve own endpoint for actor callbacks",
			"job_id", job.JobId)
		srv.failJob(ctx, job, unit, "cannot resolve compute service endpoint")
		return
	}

	inputs := map[string]any{
		"job_id":  job.JobId,
		"unit_id": unit.UnitId,
	}
	inputsJSON, _ := json.Marshal(inputs)

	clusterID := "globular.internal"
	if d, err := config.GetDomain(); err == nil && d != "" {
		clusterID = d
	}

	slog.Info("compute: dispatching job via workflow",
		"job_id", job.JobId, "workflow", "compute.job.submit",
		"callback", computeEndpoint)

	resp, err := wfClient.ExecuteWorkflow(ctx, &workflowpb.ExecuteWorkflowRequest{
		ClusterId:    clusterID,
		WorkflowName: "compute.job.submit",
		InputsJson:   string(inputsJSON),
		ActorEndpoints: map[string]string{
			"compute":          computeEndpoint,
			"workflow-service": computeEndpoint, // child workflow dispatch
		},
		CorrelationId: "compute-job-" + job.JobId,
	})
	if err != nil {
		slog.Error("compute: workflow execution failed",
			"job_id", job.JobId, "err", err)
		srv.failJob(ctx, job, unit, fmt.Sprintf("workflow execution failed: %v", err))
		return
	}

	slog.Info("compute: workflow completed",
		"job_id", job.JobId, "status", resp.Status,
		"run_id", resp.RunId)

	if resp.Status == "FAILED" {
		slog.Warn("compute: workflow reported failure",
			"job_id", job.JobId, "error", resp.Error)
	}
}

// failJob marks both the unit and job as failed.
func (srv *server) failJob(ctx context.Context, job *computepb.ComputeJob, unit *computepb.ComputeUnit, reason string) {
	unit.State = computepb.UnitState_UNIT_FAILED
	unit.FailureReason = reason
	_ = putUnit(ctx, unit)
	job.State = computepb.JobState_JOB_FAILED
	job.FailureMessage = reason
	_ = putJob(ctx, job)
}
