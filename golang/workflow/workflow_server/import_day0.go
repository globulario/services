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

// day0LogPath is a var (not const) so tests can override it to point at a
// temp fixture log. Production callers leave it at the default log location.
var day0LogPath = "/var/log/globular/day0-install.jsonl"

// day0LegacyLogPath keeps workflow import backward-compatible with nodes
// installed before the Day-0 transcript moved out of /var/lib/globular.
var day0LegacyLogPath = "/var/lib/globular/day0-install.jsonl"

// day0LogLine is the on-disk shape of one Day-0 install log entry.
// Hoisted to package scope so the write seam can take []day0LogLine and
// tests can swap it.
type day0LogLine struct {
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

// day0CountExistingFn returns the count of already-imported Day-0 runs for
// (clusterID, corrID). sessOK is false when the Scylla session is not yet
// available — callers must abort silently (rule 2 of the BEST-EFFORT
// invariant). Test seam: tests override this to force the idempotent
// branch without needing a real Scylla session.
var day0CountExistingFn = func(srv *server, clusterID, corrID string) (existing int, sessOK bool) {
	sess := srv.getSession()
	if sess == nil {
		return 0, false
	}
	if err := sess.Query(
		`SELECT COUNT(*) FROM workflow_runs WHERE cluster_id=? AND correlation_id=? ALLOW FILTERING`,
		clusterID, corrID,
	).Scan(&existing); err != nil {
		// Session is alive but the COUNT failed — treat as "not imported"
		// so we attempt to write; the writes themselves will fail loudly
		// if there is a real Scylla problem.
		return 0, true
	}
	return existing, true
}

// day0WriteRunFn writes the parsed Day-0 lines as a workflow run + steps.
// Test seam: spy in tests to assert the IDEMPOTENT branch did NOT call it.
var day0WriteRunFn = func(srv *server, clusterID, corrID string, lines []day0LogLine) {
	srv.writeDay0Run(clusterID, corrID, lines)
}

// importDay0Trace reads the Day-0 install JSON log and creates a workflow
// run with steps in ScyllaDB. Called once on startup; skips if the log
// doesn't exist or was already imported (idempotent via correlation_id).
func (srv *server) importDay0Trace() {
	f, err := openDay0TraceLog()
	if err != nil {
		return // no log → nothing to import
	}
	defer f.Close()

	clusterID := srv.Domain
	if clusterID == "" {
		clusterID = "globular.internal"
	}
	corrID := "day0-install"

	existing, sessOK := day0CountExistingFn(srv, clusterID, corrID)
	if !sessOK {
		return
	}
	if existing > 0 {
		logger.Info("Day-0 trace already imported, skipping")
		return
	}

	var lines []day0LogLine
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var l day0LogLine
		if json.Unmarshal(scanner.Bytes(), &l) == nil {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return
	}

	day0WriteRunFn(srv, clusterID, corrID, lines)
}

func openDay0TraceLog() (*os.File, error) {
	f, err := os.Open(day0LogPath)
	if err == nil {
		return f, nil
	}
	return os.Open(day0LegacyLogPath)
}

// writeDay0Run is the production write path for the Day-0 trace import.
// Extracted from importDay0Trace so day0WriteRunFn can wrap it; the body
// matches the pre-refactor inline code so production behavior is unchanged.
func (srv *server) writeDay0Run(clusterID, corrID string, lines []day0LogLine) {
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

	run := &workflowpb.WorkflowRun{
		Id:            runID,
		CorrelationId: corrID,
		Context: &workflowpb.WorkflowContext{
			ClusterId:     clusterID,
			NodeId:        hostname,
			NodeHostname:  hostname,
			ComponentName: "day0-installer",
			ComponentKind: workflowpb.ComponentKind_COMPONENT_KIND_INFRASTRUCTURE,
			ReleaseKind:   "Day0Bootstrap",
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
			RunId:      resp.Id,
			Seq:        int32(l.Seq),
			StepKey:    l.Key,
			Title:      l.Title,
			Actor:      workflowpb.WorkflowActor(l.Actor),
			Phase:      workflowpb.WorkflowPhaseKind(l.Phase),
			Status:     status,
			CreatedAt:  timestamppb.New(time.UnixMilli(l.Ts)),
			StartedAt:  timestamppb.New(time.UnixMilli(l.Ts)),
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
