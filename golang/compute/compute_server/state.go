// state.go provides etcd-backed state operations for compute objects.
//
// etcd key schema:
//   /globular/compute/definitions/{name}/{version}
//   /globular/compute/jobs/{job_id}
//   /globular/compute/jobs/{job_id}/units/{unit_id}
//   /globular/compute/jobs/{job_id}/result
package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/globulario/services/golang/compute/computepb"
	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/encoding/protojson"
)

const etcdTimeout = 5 * time.Second

// ─── Definitions ─────────────────────────────────────────────────────────────

func definitionKey(name, version string) string {
	return fmt.Sprintf("/globular/compute/definitions/%s/%s", name, version)
}

func putDefinition(ctx context.Context, def *computepb.ComputeDefinition) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	data, err := protojson.Marshal(def)
	if err != nil {
		return fmt.Errorf("marshal definition: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, etcdTimeout)
	defer cancel()
	_, err = cli.Put(tctx, definitionKey(def.Name, def.Version), string(data))
	return err
}

func getDefinition(ctx context.Context, name, version string) (*computepb.ComputeDefinition, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, etcdTimeout)
	defer cancel()
	resp, err := cli.Get(tctx, definitionKey(name, version))
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	def := &computepb.ComputeDefinition{}
	if err := protojson.Unmarshal(resp.Kvs[0].Value, def); err != nil {
		return nil, fmt.Errorf("unmarshal definition: %w", err)
	}
	return def, nil
}

func listDefinitions(ctx context.Context, prefix string) ([]*computepb.ComputeDefinition, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, etcdTimeout)
	defer cancel()
	keyPrefix := "/globular/compute/definitions/"
	if prefix != "" {
		keyPrefix += prefix
	}
	resp, err := cli.Get(tctx, keyPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	defs := make([]*computepb.ComputeDefinition, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		def := &computepb.ComputeDefinition{}
		if err := protojson.Unmarshal(kv.Value, def); err != nil {
			continue
		}
		defs = append(defs, def)
	}
	return defs, nil
}

// ─── Jobs ────────────────────────────────────────────────────────────────────

func jobKey(jobID string) string {
	return fmt.Sprintf("/globular/compute/jobs/%s", jobID)
}

func putJob(ctx context.Context, job *computepb.ComputeJob) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	data, err := protojson.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, etcdTimeout)
	defer cancel()
	_, err = cli.Put(tctx, jobKey(job.JobId), string(data))
	return err
}

func getJob(ctx context.Context, jobID string) (*computepb.ComputeJob, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, etcdTimeout)
	defer cancel()
	resp, err := cli.Get(tctx, jobKey(jobID))
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	job := &computepb.ComputeJob{}
	if err := protojson.Unmarshal(resp.Kvs[0].Value, job); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}
	return job, nil
}

func listJobs(ctx context.Context) ([]*computepb.ComputeJob, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, etcdTimeout)
	defer cancel()
	resp, err := cli.Get(tctx, "/globular/compute/jobs/", clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	jobs := make([]*computepb.ComputeJob, 0)
	for _, kv := range resp.Kvs {
		job := &computepb.ComputeJob{}
		if err := protojson.Unmarshal(kv.Value, job); err != nil {
			continue
		}
		// Only include job records (not sub-keys like units/result)
		if job.JobId != "" {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

// ─── Units ───────────────────────────────────────────────────────────────────

func unitKey(jobID, unitID string) string {
	return fmt.Sprintf("/globular/compute/jobs/%s/units/%s", jobID, unitID)
}

func putUnit(ctx context.Context, unit *computepb.ComputeUnit) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	data, err := protojson.Marshal(unit)
	if err != nil {
		return fmt.Errorf("marshal unit: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, etcdTimeout)
	defer cancel()
	_, err = cli.Put(tctx, unitKey(unit.JobId, unit.UnitId), string(data))
	return err
}

func getUnit(ctx context.Context, jobID, unitID string) (*computepb.ComputeUnit, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, etcdTimeout)
	defer cancel()
	resp, err := cli.Get(tctx, unitKey(jobID, unitID))
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	unit := &computepb.ComputeUnit{}
	if err := protojson.Unmarshal(resp.Kvs[0].Value, unit); err != nil {
		return nil, fmt.Errorf("unmarshal unit: %w", err)
	}
	return unit, nil
}

func listUnits(ctx context.Context, jobID string) ([]*computepb.ComputeUnit, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, etcdTimeout)
	defer cancel()
	prefix := fmt.Sprintf("/globular/compute/jobs/%s/units/", jobID)
	resp, err := cli.Get(tctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	units := make([]*computepb.ComputeUnit, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		unit := &computepb.ComputeUnit{}
		if err := protojson.Unmarshal(kv.Value, unit); err != nil {
			continue
		}
		units = append(units, unit)
	}
	return units, nil
}

// ─── Results ─────────────────────────────────────────────────────────────────

func resultKey(jobID string) string {
	return fmt.Sprintf("/globular/compute/jobs/%s/result", jobID)
}

func putResult(ctx context.Context, result *computepb.ComputeResult) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	data, err := protojson.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, etcdTimeout)
	defer cancel()
	_, err = cli.Put(tctx, resultKey(result.JobId), string(data))
	return err
}

func getResult(ctx context.Context, jobID string) (*computepb.ComputeResult, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, etcdTimeout)
	defer cancel()
	resp, err := cli.Get(tctx, resultKey(jobID))
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	result := &computepb.ComputeResult{}
	if err := protojson.Unmarshal(resp.Kvs[0].Value, result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}
	return result, nil
}

// ─── Partition Plans ─────────────────────────────────────────────────────────

func planKey(jobID string) string {
	return fmt.Sprintf("/globular/compute/jobs/%s/plan", jobID)
}

func putPlan(ctx context.Context, plan *computepb.ComputePartitionPlan) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	data, err := protojson.Marshal(plan)
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, etcdTimeout)
	defer cancel()
	_, err = cli.Put(tctx, planKey(plan.JobId), string(data))
	return err
}

func getPlan(ctx context.Context, jobID string) (*computepb.ComputePartitionPlan, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, etcdTimeout)
	defer cancel()
	resp, err := cli.Get(tctx, planKey(jobID))
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	plan := &computepb.ComputePartitionPlan{}
	if err := protojson.Unmarshal(resp.Kvs[0].Value, plan); err != nil {
		return nil, fmt.Errorf("unmarshal plan: %w", err)
	}
	return plan, nil
}

// ─── Leases ──────────────────────────────────────────────────────────────────

const (
	leaseKeyPrefix = "/globular/compute/leases/"
	leaseTTL       = 30 // seconds
)

func leaseKey(jobID, unitID string) string {
	return fmt.Sprintf("%s%s/%s", leaseKeyPrefix, jobID, unitID)
}

// grantUnitLease acquires an etcd TTL lease for exclusive unit ownership.
// Returns the lease ID for keep-alive renewal.
func grantUnitLease(ctx context.Context, jobID, unitID, nodeID string) (clientv3.LeaseID, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return 0, fmt.Errorf("etcd client: %w", err)
	}

	tctx, cancel := context.WithTimeout(ctx, etcdTimeout)
	defer cancel()

	grant, err := cli.Grant(tctx, leaseTTL)
	if err != nil {
		return 0, fmt.Errorf("lease grant: %w", err)
	}

	_, err = cli.Put(tctx, leaseKey(jobID, unitID), nodeID, clientv3.WithLease(grant.ID))
	if err != nil {
		cli.Revoke(context.Background(), grant.ID)
		return 0, fmt.Errorf("lease put: %w", err)
	}

	slog.Info("compute lease: granted",
		"job_id", jobID, "unit_id", unitID, "node", nodeID,
		"lease_id", grant.ID, "ttl", leaseTTL)
	return grant.ID, nil
}

// startLeaseRenewal begins automatic keep-alive for a lease. Returns a cancel
// function that stops renewal. The caller must also call revokeUnitLease on
// completion.
func startLeaseRenewal(leaseID clientv3.LeaseID) (context.CancelFunc, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}

	kaCtx, kaCancel := context.WithCancel(context.Background())
	ch, err := cli.KeepAlive(kaCtx, leaseID)
	if err != nil {
		kaCancel()
		return nil, fmt.Errorf("lease keepalive: %w", err)
	}
	// Drain the keep-alive channel in the background.
	go func() {
		for range ch {
		}
	}()
	return kaCancel, nil
}

// revokeUnitLease explicitly revokes a lease on unit completion.
func revokeUnitLease(leaseID clientv3.LeaseID) {
	if leaseID == 0 {
		return
	}
	cli, err := config.GetEtcdClient()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cli.Revoke(ctx, leaseID)
}

// isLeaseAlive checks if a unit's lease key still exists in etcd.
func isLeaseAlive(ctx context.Context, jobID, unitID string) bool {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return false
	}
	tctx, cancel := context.WithTimeout(ctx, etcdTimeout)
	defer cancel()
	resp, err := cli.Get(tctx, leaseKey(jobID, unitID))
	return err == nil && len(resp.Kvs) > 0
}

// ─── Heartbeats ──────────────────────────────────────────────────────────────

func heartbeatKey(jobID, unitID string) string {
	return fmt.Sprintf("/globular/compute/heartbeats/%s/%s", jobID, unitID)
}

// putHeartbeat writes a heartbeat marker to etcd with a short TTL.
// Auto-expires if the runner stops writing.
func putHeartbeat(ctx context.Context, jobID, unitID string, progress float64) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return err
	}
	tctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Grant a short-lived lease for the heartbeat key (15s TTL).
	grant, err := cli.Grant(tctx, 15)
	if err != nil {
		return err
	}

	val := fmt.Sprintf(`{"progress":%.2f,"observed_at":"%s"}`, progress, time.Now().UTC().Format(time.RFC3339))
	_, err = cli.Put(tctx, heartbeatKey(jobID, unitID), val, clientv3.WithLease(grant.ID))
	return err
}
