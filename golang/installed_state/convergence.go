package installed_state

// Package installed_state — convergence.go
//
// A ConvergenceResult is the per-package outcome record written by the
// node-agent after every install attempt (success, failure, or skip).
// The controller reads this to learn the actual installed version and then
// commits the authoritative installed-state record to etcd.
//
// etcd key schema:
//
//	/globular/nodes/{node_id}/convergence/{kind}/{name}
//
// This splits the write ownership cleanly:
//   - node-agent WRITES convergence results   ← "what happened"
//   - controller READS convergence, WRITES installed state ← "what is authoritative"
//
// Schema:
//
// +globular:schema:key="/globular/nodes/{node_id}/convergence/{kind}/{name}"
// +globular:schema:writer="globular-node-agent"
// +globular:schema:readers="globular-cluster-controller,globular-cluster-doctor"
// +globular:schema:description="Per-package convergence outcome written by node-agent after every install attempt."
// +globular:schema:invariants="Status MUST be one of SUCCEEDED|FAILED|SKIPPED; NodeID, PackageName, PackageKind are required; CommittedUnix must be non-zero."

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Convergence status constants. The controller gate uses these to decide
// whether to commit the installed-state record or open an incident.
const (
	ConvergenceStatusSucceeded = "SUCCEEDED"
	ConvergenceStatusFailed    = "FAILED"
	ConvergenceStatusSkipped   = "SKIPPED"
)

// ConvergenceResult is the outcome record for a single install attempt.
// It is written by the node-agent and consumed by the cluster controller.
type ConvergenceResult struct {
	// NodeID identifies the node where the install was attempted.
	NodeID string `json:"node_id"`
	// PackageName is the package name (e.g. "gateway", "etcd").
	PackageName string `json:"package_name"`
	// PackageKind is the normalised kind: SERVICE, INFRASTRUCTURE, or COMMAND.
	PackageKind string `json:"package_kind"`
	// AttemptedVersion is the version the workflow requested.
	AttemptedVersion string `json:"attempted_version"`
	// ActualVersion is the version that is installed on disk now.
	// For SKIPPED this equals the already-installed version (may differ from
	// AttemptedVersion when the request targeted the same version that was
	// already present — they should match — or when an older binary is
	// already live and the skip was not warranted).
	ActualVersion string `json:"actual_version"`
	// BuildID is the exact artifact identity (UUIDv7) of what is on disk.
	BuildID string `json:"build_id,omitempty"`
	// Status is one of ConvergenceStatusSucceeded, ConvergenceStatusFailed,
	// or ConvergenceStatusSkipped.
	Status string `json:"status"`
	// Reason is a human-readable explanation, required for SKIPPED and FAILED.
	Reason string `json:"reason,omitempty"`
	// OperationID is the workflow operation_id that triggered this attempt.
	// The controller uses it to correlate convergence results with workflow runs.
	OperationID string `json:"operation_id,omitempty"`
	// WorkflowRunID is the ID of the workflow run that triggered this attempt.
	WorkflowRunID string `json:"workflow_run_id,omitempty"`
	// CommittedUnix is the Unix timestamp when the node-agent wrote this record.
	CommittedUnix int64 `json:"committed_unix"`
}

// convergenceKeyPrefix shares the /globular/nodes/ root with installed-state keys.
const convergenceKeyPrefix = "/globular/nodes/"

// convergenceKey returns the etcd key for a convergence result.
// Format: /globular/nodes/{node_id}/convergence/{KIND}/{name}
func convergenceKey(nodeID, kind, name string) string {
	return convergenceKeyPrefix + nodeID + "/convergence/" + strings.ToUpper(kind) + "/" + name
}

// nodeConvergencePrefix returns the etcd prefix for all convergence results on a node.
func nodeConvergencePrefix(nodeID string) string {
	return convergenceKeyPrefix + nodeID + "/convergence/"
}

// nodeConvergenceKindPrefix returns the etcd prefix for convergence results of a specific kind.
func nodeConvergenceKindPrefix(nodeID, kind string) string {
	return convergenceKeyPrefix + nodeID + "/convergence/" + strings.ToUpper(kind) + "/"
}

// ParseConvergenceKey extracts nodeID, kind, and name from an etcd convergence key.
// Returns an error when the key does not match the expected schema.
func ParseConvergenceKey(key string) (nodeID, kind, name string, err error) {
	rest := strings.TrimPrefix(key, convergenceKeyPrefix)
	if rest == key {
		return "", "", "", fmt.Errorf("convergence key %q missing prefix %q", key, convergenceKeyPrefix)
	}
	// rest = "{node_id}/convergence/{KIND}/{name}"
	parts := strings.SplitN(rest, "/", 4)
	if len(parts) != 4 || parts[1] != "convergence" || parts[2] == "" || parts[3] == "" {
		return "", "", "", fmt.Errorf("convergence key %q has unexpected shape (want node/convergence/KIND/name)", key)
	}
	return parts[0], parts[2], parts[3], nil
}

// WriteConvergenceResult writes a ConvergenceResult to etcd using the
// StateCommitWrite policy (30 s timeout, 6 retries, jittered backoff).
// The controller MUST NOT commit installed-state until this write succeeds.
func WriteConvergenceResult(ctx context.Context, result *ConvergenceResult) error {
	if err := validateConvergenceResult(result); err != nil {
		return err
	}
	if result.CommittedUnix == 0 {
		result.CommittedUnix = time.Now().Unix()
	}
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("convergence_result: marshal: %w", err)
	}
	key := convergenceKey(result.NodeID, result.PackageKind, result.PackageName)
	return config.PutRuntimeWithClass(ctx, key, data, config.StateCommitWrite)
}

// GetConvergenceResult reads a single ConvergenceResult from etcd.
// Returns (nil, nil) when the key does not exist.
func GetConvergenceResult(ctx context.Context, nodeID, kind, name string) (*ConvergenceResult, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("convergence_result: etcd client: %w", err)
	}
	key := convergenceKey(nodeID, kind, name)
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	resp, err := cli.Get(tctx, key)
	if err != nil {
		return nil, fmt.Errorf("convergence_result: get %q: %w", key, err)
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	return unmarshalConvergenceResult(resp.Kvs[0].Value)
}

// DeleteConvergenceResult removes a ConvergenceResult from etcd.
// Used by the controller after it has committed the installed-state record.
func DeleteConvergenceResult(ctx context.Context, nodeID, kind, name string) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("convergence_result: etcd client: %w", err)
	}
	key := convergenceKey(nodeID, kind, name)
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	_, err = cli.Delete(tctx, key)
	if err != nil {
		return fmt.Errorf("convergence_result: delete %q: %w", key, err)
	}
	return nil
}

// ListConvergenceResults returns all ConvergenceResults for a node, optionally
// filtered to a specific kind. Returns an empty slice when none exist.
func ListConvergenceResults(ctx context.Context, nodeID, kind string) ([]*ConvergenceResult, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("convergence_result: etcd client: %w", err)
	}
	prefix := nodeConvergencePrefix(nodeID)
	if kind != "" {
		prefix = nodeConvergenceKindPrefix(nodeID, kind)
	}
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	resp, err := cli.Get(tctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("convergence_result: list %q: %w", prefix, err)
	}
	results := make([]*ConvergenceResult, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		r, err := unmarshalConvergenceResult(kv.Value)
		if err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func validateConvergenceResult(r *ConvergenceResult) error {
	if r == nil {
		return fmt.Errorf("convergence_result: nil result")
	}
	if r.NodeID == "" {
		return fmt.Errorf("convergence_result: node_id is required")
	}
	if r.PackageName == "" {
		return fmt.Errorf("convergence_result: package_name is required")
	}
	if r.PackageKind == "" {
		return fmt.Errorf("convergence_result: package_kind is required")
	}
	switch r.Status {
	case ConvergenceStatusSucceeded, ConvergenceStatusFailed, ConvergenceStatusSkipped:
	default:
		return fmt.Errorf("convergence_result: status %q must be SUCCEEDED|FAILED|SKIPPED", r.Status)
	}
	return nil
}

func unmarshalConvergenceResult(data []byte) (*ConvergenceResult, error) {
	r := &ConvergenceResult{}
	if err := json.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("convergence_result: unmarshal: %w", err)
	}
	return r, nil
}
