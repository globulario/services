package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/globulario/services/golang/workflow/workflowpb"
	"github.com/gocql/gocql"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const day0LogPath = "/var/lib/globular/day0-install.jsonl"

// importDay0Trace reads the Day-0 install JSON log and creates a workflow
// run with steps in ScyllaDB. Called once on startup; skips if the log
// doesn't exist or was already imported (idempotent via correlation_id).
func (srv *server) importDay0Trace() {
	f, err := os.Open(day0LogPath)
	if err != nil {
		return // no log → nothing to import
	}
	defer f.Close()

	// Check if already imported.
	clusterID := srv.Domain
	if clusterID == "" {
		clusterID = "globular.internal"
	}
	corrID := "day0-install"
	var existing int
	if err := srv.session.Query(
		`SELECT COUNT(*) FROM workflow_runs WHERE cluster_id=? AND correlation_id=? ALLOW FILTERING`,
		clusterID, corrID,
	).Scan(&existing); err == nil && existing > 0 {
		logger.Info("Day-0 trace already imported, skipping")
		return
	}

	// Parse log lines.
	type logLine struct {
		Type     string `json:"type"`
		Seq      int    `json:"seq"`
		Key      string `json:"key"`
		Title    string `json:"title"`
		Status   string `json:"status"`
		Phase    int    `json:"phase"`
		Actor    int    `json:"actor"`
		Ts       int64  `json:"ts"`
		Dur      int64  `json:"dur"`
		Hostname string `json:"hostname"`
		Msg      string `json:"msg"`
	}

	var lines []logLine
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var l logLine
		if json.Unmarshal(scanner.Bytes(), &l) == nil {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return
	}

	// Find hostname and timestamps.
	hostname := "unknown"
	var runStart, runEnd int64
	finalStatus := workflowpb.RunStatus_RUN_STATUS_SUCCEEDED
	var finalMsg string

	for _, l := range lines {
		if l.Type == "run_start" {
			hostname = l.Hostname
			runStart = l.Ts
		}
		if l.Type == "run_finish" {
			runEnd = l.Ts
			if l.Status != "ok" {
				finalStatus = workflowpb.RunStatus_RUN_STATUS_FAILED
			}
			finalMsg = l.Msg
		}
	}
	if runStart == 0 {
		runStart = time.Now().UnixMilli()
	}
	if runEnd == 0 {
		runEnd = time.Now().UnixMilli()
	}

	runID := gocql.TimeUUID().String()
	startTime := time.UnixMilli(runStart)

	// Create the run.
	run := &workflowpb.WorkflowRun{
		Id:            runID,
		CorrelationId: corrID,
		Context: &workflowpb.WorkflowContext{
			ClusterId:    clusterID,
			NodeId:       hostname,
			NodeHostname: hostname,
			ComponentName: "day0-installer",
			ComponentKind: workflowpb.ComponentKind_COMPONENT_KIND_INFRASTRUCTURE,
			ReleaseKind:  "Day0Bootstrap",
		},
		TriggerReason: workflowpb.TriggerReason_TRIGGER_REASON_BOOTSTRAP,
		Status:        finalStatus,
		CurrentActor:  workflowpb.WorkflowActor_ACTOR_INSTALLER,
		Summary:       finalMsg,
		StartedAt:     timestamppb.New(startTime),
		UpdatedAt:     timestamppb.New(time.UnixMilli(runEnd)),
		FinishedAt:    timestamppb.New(time.UnixMilli(runEnd)),
	}

	resp, err := srv.StartRun(context.Background(), &workflowpb.StartRunRequest{Run: run})
	if err != nil {
		logger.Error("Day-0 import: StartRun failed", "err", err)
		return
	}

	// Create steps.
	var stepCount int
	for _, l := range lines {
		if l.Seq == 0 {
			continue // skip run_start/run_finish markers
		}

		status := workflowpb.StepStatus_STEP_STATUS_SUCCEEDED
		if l.Status == "failed" {
			status = workflowpb.StepStatus_STEP_STATUS_FAILED
		} else if l.Status == "running" {
			status = workflowpb.StepStatus_STEP_STATUS_RUNNING
		}

		step := &workflowpb.WorkflowStep{
			RunId:     resp.Id,
			Seq:       int32(l.Seq),
			StepKey:   l.Key,
			Title:     l.Title,
			Actor:     workflowpb.WorkflowActor(l.Actor),
			Phase:     workflowpb.WorkflowPhaseKind(l.Phase),
			Status:    status,
			CreatedAt: timestamppb.New(time.UnixMilli(l.Ts)),
			StartedAt: timestamppb.New(time.UnixMilli(l.Ts)),
			DurationMs: l.Dur,
		}
		if status != workflowpb.StepStatus_STEP_STATUS_RUNNING {
			step.FinishedAt = timestamppb.New(time.UnixMilli(l.Ts))
		}

		srv.RecordStep(context.Background(), &workflowpb.RecordStepRequest{
			ClusterId: clusterID,
			Step:      step,
		})
		stepCount++
	}

	logger.Info("Day-0 trace imported",
		"run_id", resp.Id, "steps", stepCount, "hostname", hostname,
		"status", finalStatus.String())
}
