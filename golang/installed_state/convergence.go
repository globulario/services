package installed_state

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/encoding/protojson"
)

// ConvergenceOutcome is the persisted convergence verdict.
type ConvergenceOutcome string

const (
	OutcomeSuccessCommitted          ConvergenceOutcome = "SUCCESS_COMMITTED"
	OutcomeSuccessLocalPendingSync   ConvergenceOutcome = "SUCCESS_LOCAL_PENDING_SYNC"
	OutcomeBlockedMissingNativeDep   ConvergenceOutcome = "BLOCKED_MISSING_NATIVE_DEP"
	OutcomeBlockedCriticalKeyMissing ConvergenceOutcome = "BLOCKED_CRITICAL_KEY_MISSING"
	OutcomeBlockedNodeUnreachable    ConvergenceOutcome = "BLOCKED_NODE_UNREACHABLE"
	OutcomeFailedTransient           ConvergenceOutcome = "FAILED_TRANSIENT"
	OutcomeFailedPermanent           ConvergenceOutcome = "FAILED_PERMANENT"
	OutcomeDegradedRetrying          ConvergenceOutcome = "DEGRADED_RETRYING"
	OutcomeStaleInstalledState       ConvergenceOutcome = "STALE_INSTALLED_STATE"
)

const (
	convergenceActionPrefix = "/globular/convergence/actions/"
	convergenceLatestPrefix = "/globular/convergence/nodes/"
)

// ConvergenceResultV1 is the authoritative persisted action outcome contract.
type ConvergenceResultV1 struct {
	ActionID       string            `json:"action_id"`
	WorkflowID     string            `json:"workflow_id"`
	Package        string            `json:"package"`
	NodeID         string            `json:"node_id"`
	DesiredVersion string            `json:"desired_version"`
	DesiredBuildID string            `json:"desired_build_id"`
	DesiredHash    string            `json:"desired_hash"`
	LocalVersion   string            `json:"local_version"`
	LocalBuildID   string            `json:"local_build_id"`
	LocalHash      string            `json:"local_hash"`
	Outcome        ConvergenceOutcome`json:"outcome"`
	ReasonCode     string            `json:"reason_code"`
	RetryPolicy    string            `json:"retry_policy"`
	UnblockPolicy  string            `json:"unblock_policy"`
	Evidence       map[string]string `json:"evidence,omitempty"`
	CommittedAt    int64             `json:"committed_at"`
	LastAttemptAt  int64             `json:"last_attempt_at"`
	AttemptCount   int32             `json:"attempt_count"`
	SourceComponent string           `json:"source_component"`
}

func ConvergenceActionKey(actionID string) string {
	return convergenceActionPrefix + actionID
}

func ConvergenceLatestKey(nodeID, pkg string) string {
	return convergenceLatestPrefix + nodeID + "/packages/" + pkg + "/latest"
}

func ParseConvergenceLatestKey(key string) (nodeID, pkg string, err error) {
	rest := strings.TrimPrefix(key, convergenceLatestPrefix)
	if rest == key {
		return "", "", fmt.Errorf("convergence latest key %q missing prefix %q", key, convergenceLatestPrefix)
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 4 || parts[1] != "packages" || parts[3] != "latest" || parts[0] == "" || parts[2] == "" {
		return "", "", fmt.Errorf("convergence latest key %q has unexpected shape (want node/packages/pkg/latest)", key)
	}
	return parts[0], parts[2], nil
}

// WriteConvergenceResult writes both action and latest records via StateCommitWrite.
func WriteConvergenceResult(ctx context.Context, result *ConvergenceResultV1) error {
	if err := validateConvergenceResult(result); err != nil {
		return err
	}
	now := time.Now().Unix()
	if result.CommittedAt == 0 {
		result.CommittedAt = now
	}
	if result.LastAttemptAt == 0 {
		result.LastAttemptAt = now
	}
	if result.AttemptCount == 0 {
		result.AttemptCount = 1
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("convergence_result: marshal: %w", err)
	}
	if err := config.PutRuntimeWithClass(ctx, ConvergenceActionKey(result.ActionID), data, config.StateCommitWrite); err != nil {
		return err
	}
	return config.PutRuntimeWithClass(ctx, ConvergenceLatestKey(result.NodeID, result.Package), data, config.StateCommitWrite)
}

func GetConvergenceResult(ctx context.Context, actionID string) (*ConvergenceResultV1, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("convergence_result: etcd client: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	resp, err := cli.Get(tctx, ConvergenceActionKey(actionID))
	if err != nil {
		return nil, fmt.Errorf("convergence_result: get action %q: %w", actionID, err)
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	return unmarshalConvergenceResult(resp.Kvs[0].Value)
}

func GetLatestConvergenceResult(ctx context.Context, nodeID, pkg string) (*ConvergenceResultV1, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("convergence_result: etcd client: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	resp, err := cli.Get(tctx, ConvergenceLatestKey(nodeID, pkg))
	if err != nil {
		return nil, fmt.Errorf("convergence_result: get latest %q/%q: %w", nodeID, pkg, err)
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	return unmarshalConvergenceResult(resp.Kvs[0].Value)
}

func DeleteConvergenceResult(ctx context.Context, actionID string) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("convergence_result: etcd client: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	_, err = cli.Delete(tctx, ConvergenceActionKey(actionID))
	if err != nil {
		return fmt.Errorf("convergence_result: delete action %q: %w", actionID, err)
	}
	return nil
}

func ListConvergenceResults(ctx context.Context, nodeID string) ([]*ConvergenceResultV1, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("convergence_result: etcd client: %w", err)
	}
	prefix := convergenceLatestPrefix + nodeID + "/packages/"
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	resp, err := cli.Get(tctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("convergence_result: list latest %q: %w", nodeID, err)
	}
	results := make([]*ConvergenceResultV1, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		r, err := unmarshalConvergenceResult(kv.Value)
		if err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

func validateConvergenceResult(r *ConvergenceResultV1) error {
	if r == nil {
		return fmt.Errorf("convergence_result: nil result")
	}
	if r.ActionID == "" {
		return fmt.Errorf("convergence_result: action_id is required")
	}
	if r.WorkflowID == "" {
		return fmt.Errorf("convergence_result: workflow_id is required")
	}
	if r.Package == "" {
		return fmt.Errorf("convergence_result: package is required")
	}
	if r.NodeID == "" {
		return fmt.Errorf("convergence_result: node_id is required")
	}
	if r.SourceComponent == "" {
		return fmt.Errorf("convergence_result: source_component is required")
	}
	switch r.Outcome {
	case OutcomeSuccessCommitted,
		OutcomeSuccessLocalPendingSync,
		OutcomeBlockedMissingNativeDep,
		OutcomeBlockedCriticalKeyMissing,
		OutcomeBlockedNodeUnreachable,
		OutcomeFailedTransient,
		OutcomeFailedPermanent,
		OutcomeDegradedRetrying,
		OutcomeStaleInstalledState:
	default:
		return fmt.Errorf("convergence_result: outcome %q is invalid", r.Outcome)
	}
	return nil
}

func unmarshalConvergenceResult(data []byte) (*ConvergenceResultV1, error) {
	r := &ConvergenceResultV1{}
	if err := json.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("convergence_result: unmarshal: %w", err)
	}
	return r, nil
}

// CommitConvergenceWithInstall atomically commits three etcd operations in a
// single Txn, preventing partial-write states that would leave installed-state
// and convergence keys inconsistent after a controller crash:
//
//  1. Put: installed-package key   ← authoritative installed state
//  2. Put: convergence latest key  ← promoted to OutcomeSuccessCommitted
//  3. Delete: convergence action key ← cleanup, idempotent on retry
//
// The result's Outcome is promoted to OutcomeSuccessCommitted and
// SourceComponent set to "cluster-controller" before the write.
//
// Validation mirrors CommitInstalledPackage — call this from the convergence
// committer instead of calling CommitInstalledPackage + WriteConvergenceResult
// + DeleteConvergenceResult separately.
func CommitConvergenceWithInstall(
	ctx context.Context,
	pkg *node_agentpb.InstalledPackage,
	result *ConvergenceResultV1,
) error {
	if pkg.GetNodeId() == "" {
		return fmt.Errorf("convergence_txn: node_id is required")
	}
	if pkg.GetName() == "" {
		return fmt.Errorf("convergence_txn: name is required")
	}
	if pkg.GetKind() == "" {
		return fmt.Errorf("convergence_txn: kind is required")
	}
	if result == nil {
		return fmt.Errorf("convergence_txn: result is required")
	}
	if result.ActionID == "" {
		return fmt.Errorf("convergence_txn: result.action_id is required")
	}
	if result.NodeID == "" {
		return fmt.Errorf("convergence_txn: result.node_id is required")
	}
	if result.Package == "" {
		return fmt.Errorf("convergence_txn: result.package is required")
	}

	now := time.Now().Unix()
	if pkg.UpdatedUnix == 0 {
		pkg.UpdatedUnix = now
	}
	if pkg.InstalledUnix == 0 {
		pkg.InstalledUnix = now
	}
	if pkg.Status == "" {
		pkg.Status = "installed"
	}

	pkgData, err := protojson.Marshal(pkg)
	if err != nil {
		return fmt.Errorf("convergence_txn: marshal pkg: %w", err)
	}

	// Promote outcome to committed before writing.
	promoted := *result
	promoted.Outcome = OutcomeSuccessCommitted
	promoted.SourceComponent = "cluster-controller"
	promoted.CommittedAt = now

	resultData, err := json.Marshal(&promoted)
	if err != nil {
		return fmt.Errorf("convergence_txn: marshal result: %w", err)
	}

	installedKey := packageKey(pkg.GetNodeId(), pkg.GetKind(), pkg.GetName())
	latestKey := ConvergenceLatestKey(result.NodeID, result.Package)
	actionKey := ConvergenceActionKey(result.ActionID)

	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("convergence_txn: etcd client: %w", err)
	}

	tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err = cli.Txn(tctx).Then(
		clientv3.OpPut(installedKey, string(pkgData)),
		clientv3.OpPut(latestKey, string(resultData)),
		clientv3.OpDelete(actionKey),
	).Commit()
	if err != nil {
		return fmt.Errorf("convergence_txn: commit: %w", err)
	}
	return nil
}
