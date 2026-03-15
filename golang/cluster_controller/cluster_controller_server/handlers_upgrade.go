package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/plan/versionutil"
	"github.com/globulario/services/golang/repository/repository_client"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

func (srv *server) UpgradeGlobular(ctx context.Context, req *cluster_controllerpb.UpgradeGlobularRequest) (*cluster_controllerpb.UpgradeGlobularResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if len(req.GetArtifact()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "artifact is required")
	}
	platform := strings.TrimSpace(req.GetPlatform())
	if platform == "" {
		return nil, status.Error(codes.InvalidArgument, "platform is required")
	}
	nodeID := strings.TrimSpace(req.GetNodeId())
	if nodeID == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	srv.lock("unknown")
	node := srv.state.Nodes[nodeID]
	srv.unlock()
	if node == nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	if srv.planStore == nil {
		return nil, status.Error(codes.FailedPrecondition, "plan store unavailable")
	}

	sha := strings.TrimSpace(req.GetSha256())
	if sha == "" {
		hash := sha256.Sum256(req.GetArtifact())
		sha = hex.EncodeToString(hash[:])
	} else {
		sha = strings.ToLower(sha)
	}

	planID := uuid.NewString()
	ref := &repositorypb.ArtifactRef{
		PublisherId: defaultTargetPublisher,
		Name:        defaultTargetName,
		Version:     planID,
		Platform:    platform,
		Kind:        repositorypb.ArtifactKind_SUBSYSTEM,
	}
	if err := uploadArtifact(ctx, ref, req.GetArtifact()); err != nil {
		return nil, status.Errorf(codes.Internal, "stage artifact: %v", err)
	}

	targetPath := strings.TrimSpace(req.GetTargetPath())
	if targetPath == "" {
		targetPath = os.Getenv("GLOBULAR_BINARY_PATH")
	}
	if targetPath == "" {
		targetPath = defaultBinaryPath
	}
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "target_path unavailable")
	}

	minPath := filepath.Dir(targetPath)
	fetchDest := filepath.Join(os.TempDir(), "globular-upgrade", planID, filepath.Base(targetPath))

	generation := srv.nextPlanGeneration(ctx, nodeID)
	expires := time.Now().Add(upgradePlanTTL)
	plan := buildUpgradePlan(planID, nodeID, srv.state.ClusterId, generation, expires, targetPath, fetchDest, ref, sha, req.GetProbePort(), minPath)

	if err := srv.signOrAbort(plan); err != nil {
		return nil, status.Errorf(codes.Internal, "plan signing failed: %v", err)
	}
	if err := srv.planStore.PutCurrentPlan(ctx, nodeID, plan); err != nil {
		return nil, status.Errorf(codes.Internal, "persist plan: %v", err)
	}
	if appendable, ok := srv.planStore.(interface {
		AppendHistory(ctx context.Context, nodeID string, plan *planpb.NodePlan) error
	}); ok {
		_ = appendable.AppendHistory(ctx, nodeID, plan)
	}

	opID := uuid.NewString()
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_QUEUED, "upgrade queued", 0, false, ""))
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "plan dispatched", 10, false, ""))

	status, err := srv.waitForPlanStatus(ctx, nodeID, planID, expires)
	if err != nil {
		srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_FAILED, "plan failed", 100, true, err.Error()))
		return nil, err
	}

	phase := cluster_controllerpb.OperationPhase_OP_SUCCEEDED
	msg := "plan succeeded"
	done := true
	errMsg := ""
	if status.GetState() != planpb.PlanState_PLAN_SUCCEEDED {
		phase = cluster_controllerpb.OperationPhase_OP_FAILED
		msg = "plan completed with error"
		errMsg = status.GetErrorMessage()
	}
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, phase, msg, 100, done, errMsg))

	return &cluster_controllerpb.UpgradeGlobularResponse{
		PlanId:        planID,
		Generation:    generation,
		TerminalState: planStateName(status.GetState()),
		ErrorStepId:   status.GetErrorStepId(),
		ErrorMessage:  status.GetErrorMessage(),
	}, nil
}

func buildUpgradePlan(planID, nodeID, clusterID string, generation uint64, expires time.Time, targetPath, fetchDest string, ref *repositorypb.ArtifactRef, sha string, probePort uint32, diskPath string) *planpb.NodePlan {
	if probePort == 0 {
		probePort = defaultProbePort
	}
	steps := []*planpb.PlanStep{
		planStep("check.disk_free", map[string]interface{}{
			"path":      diskPath,
			"min_bytes": float64(upgradeDiskMinBytes),
		}),
		planStep("artifact.fetch", map[string]interface{}{
			"publisher": ref.GetPublisherId(),
			"name":      ref.GetName(),
			"version":   ref.GetVersion(),
			"platform":  ref.GetPlatform(),
			"dest":      fetchDest,
		}),
		planStep("artifact.verify", map[string]interface{}{
			"path":   fetchDest,
			"sha256": sha,
		}),
		planStep("service.stop", map[string]interface{}{
			"unit": "globular.service",
		}),
		planStep("file.backup", map[string]interface{}{
			"path": targetPath,
		}),
		planStep("file.write_atomic", map[string]interface{}{
			"path": targetPath,
			"src":  fetchDest,
		}),
		planStep("file.write_atomic", map[string]interface{}{
			"path":    versionutil.MarkerPath(defaultTargetName),
			"content": ref.GetVersion(),
		}),
		planStep("service.start", map[string]interface{}{
			"unit": "globular.service",
		}),
		planStep("probe.http", map[string]interface{}{
			"url": fmt.Sprintf("http://127.0.0.1:%d/checksum", probePort),
		}),
	}
	rollback := []*planpb.PlanStep{
		planStep("file.restore_backup", map[string]interface{}{
			"path": targetPath,
		}),
		planStep("service.start", map[string]interface{}{
			"unit": "globular.service",
		}),
	}
	policy := &planpb.PlanPolicy{
		MaxRetries:       3,
		RetryBackoffMs:   2000,
		FailureMode:      planpb.FailureMode_FAILURE_MODE_ROLLBACK,
		DryRun:           false,
		MaxParallelSteps: 1,
	}
	desired := &planpb.DesiredState{
		Services: []*planpb.DesiredService{
			{
				Name:    defaultTargetName,
				Version: ref.GetVersion(),
				Unit:    "globular.service",
			},
		},
	}
	return &planpb.NodePlan{
		ApiVersion:    "globular.io/plan/v1",
		Kind:          "NodePlan",
		ClusterId:     clusterID,
		NodeId:        nodeID,
		PlanId:        planID,
		Generation:    generation,
		CreatedUnixMs: uint64(time.Now().UnixMilli()),
		ExpiresUnixMs: uint64(expires.UnixMilli()),
		IssuedBy:      "cluster-controller",
		Reason:        "update_globular",
		Locks:         []string{"node-upgrade", "service:Globular"},
		Policy:        policy,
		Spec: &planpb.PlanSpec{
			Steps:    steps,
			Rollback: rollback,
			Desired:  desired,
		},
	}
}

func planStep(action string, args map[string]interface{}) *planpb.PlanStep {
	return &planpb.PlanStep{
		Id:     fmt.Sprintf("step-%s", strings.ReplaceAll(action, ".", "-")),
		Action: action,
		Args:   structFromMap(args),
	}
}

func structFromMap(fields map[string]interface{}) *structpb.Struct {
	if len(fields) == 0 {
		return nil
	}
	s, _ := structpb.NewStruct(fields)
	return s
}

func uploadArtifact(ctx context.Context, ref *repositorypb.ArtifactRef, data []byte) error {
	addr := strings.TrimSpace(os.Getenv(repositoryAddressEnv))
	if addr == "" {
		addr = "localhost:10101"
	}
	client, err := repository_client.NewRepositoryService_Client(addr, "repository.PackageRepository")
	if err != nil {
		return err
	}
	defer client.Close()
	return client.UploadArtifact(ref, data)
}
