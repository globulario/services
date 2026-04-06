package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	jobKeyPrefix    = "/globular/ai/jobs/"
	approvalTimeout = 30 * time.Minute // approvals expire after 30 minutes
)

// jobStore persists remediation jobs in etcd so they survive restart.
//go:schemalint:ignore — implementation type, not schema owner
type jobStore struct {
	mu   sync.RWMutex
	jobs map[string]*ai_executorpb.Job // incident_id → job (in-memory cache)

	etcdClient *clientv3.Client
}

func newJobStore() *jobStore {
	js := &jobStore{
		jobs: make(map[string]*ai_executorpb.Job),
	}

	// Try connecting to etcd for persistence.
	go js.connectEtcd()

	return js
}

func (js *jobStore) connectEtcd() {
	// Retry etcd connection with backoff.
	for attempt := 0; attempt < 10; attempt++ {
		cli, err := config.NewEtcdClient()
		if err == nil {
			js.mu.Lock()
			js.etcdClient = cli
			js.mu.Unlock()
			logger.Info("job_store: connected to etcd")

			// Load existing jobs.
			js.loadFromEtcd()
			return
		}
		logger.Debug("job_store: etcd connect attempt failed", "attempt", attempt, "err", err)
		time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
	}
	logger.Warn("job_store: etcd unavailable, using in-memory only")
}

// createJob creates a new durable job for an incident.
func (js *jobStore) createJob(incidentID, ruleID string, tier int32, diagnosis *ai_executorpb.Diagnosis) *ai_executorpb.Job {
	js.mu.Lock()
	defer js.mu.Unlock()

	// Check for existing job (idempotency).
	if existing, ok := js.jobs[incidentID]; ok {
		return existing
	}

	now := time.Now().UnixMilli()
	job := &ai_executorpb.Job{
		IncidentId:     incidentID,
		ActionId:       Utility.RandomUUID(),
		State:          ai_executorpb.JobState_JOB_DIAGNOSED,
		Tier:           tier,
		Diagnosis:      diagnosis,
		CreatedAtMs:    now,
		UpdatedAtMs:    now,
		IdempotencyKey: fmt.Sprintf("%s:%s:%s", incidentID, diagnosis.GetRootCause(), diagnosis.GetProposedAction()),
	}

	// Set action type and target from diagnosis.
	if diagnosis != nil {
		job.ActionTarget = diagnosis.GetProposedAction()
	}

	// Set expiry for Tier 3.
	if tier == 2 {
		job.ExpiresAtMs = time.Now().Add(approvalTimeout).UnixMilli()
		job.State = ai_executorpb.JobState_JOB_AWAITING_APPROVAL
	}

	js.jobs[incidentID] = job
	go js.persistJob(job)

	return job
}

// updateState transitions a job to a new state.
func (js *jobStore) updateState(incidentID string, state ai_executorpb.JobState) (*ai_executorpb.Job, error) {
	js.mu.Lock()
	defer js.mu.Unlock()

	job, ok := js.jobs[incidentID]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", incidentID)
	}

	job.State = state
	job.UpdatedAtMs = time.Now().UnixMilli()
	go js.persistJob(job)

	return job, nil
}

// approve marks a job as approved and records who approved it.
func (js *jobStore) approve(incidentID, approvedBy string) (*ai_executorpb.Job, error) {
	js.mu.Lock()
	defer js.mu.Unlock()

	job, ok := js.jobs[incidentID]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", incidentID)
	}

	// Idempotency: if already approved, return without re-executing.
	if job.ApprovedBy != "" {
		return job, nil
	}

	if job.State != ai_executorpb.JobState_JOB_AWAITING_APPROVAL {
		return nil, fmt.Errorf("job %s not awaiting approval (state=%s)", incidentID, job.State)
	}

	// Check expiry.
	if job.ExpiresAtMs > 0 && time.Now().UnixMilli() > job.ExpiresAtMs {
		job.State = ai_executorpb.JobState_JOB_EXPIRED
		job.UpdatedAtMs = time.Now().UnixMilli()
		go js.persistJob(job)
		return nil, fmt.Errorf("job %s has expired", incidentID)
	}

	job.State = ai_executorpb.JobState_JOB_APPROVED
	job.ApprovedBy = approvedBy
	job.ApprovedAtMs = time.Now().UnixMilli()
	job.UpdatedAtMs = time.Now().UnixMilli()
	go js.persistJob(job)

	return job, nil
}

// deny marks a job as denied.
func (js *jobStore) deny(incidentID, deniedBy, reason string) (*ai_executorpb.Job, error) {
	js.mu.Lock()
	defer js.mu.Unlock()

	job, ok := js.jobs[incidentID]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", incidentID)
	}

	if job.State != ai_executorpb.JobState_JOB_AWAITING_APPROVAL {
		return nil, fmt.Errorf("job %s not awaiting approval (state=%s)", incidentID, job.State)
	}

	job.State = ai_executorpb.JobState_JOB_DENIED
	job.DeniedBy = deniedBy
	job.DeniedReason = reason
	job.UpdatedAtMs = time.Now().UnixMilli()
	go js.persistJob(job)

	return job, nil
}

// markExecuting sets a job to executing state, incrementing attempt count.
func (js *jobStore) markExecuting(incidentID string) (*ai_executorpb.Job, error) {
	js.mu.Lock()
	defer js.mu.Unlock()

	job, ok := js.jobs[incidentID]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", incidentID)
	}

	job.State = ai_executorpb.JobState_JOB_EXECUTING
	job.Attempts++
	job.LastAttemptAtMs = time.Now().UnixMilli()
	job.UpdatedAtMs = time.Now().UnixMilli()
	go js.persistJob(job)

	return job, nil
}

// markResult records the execution outcome.
func (js *jobStore) markResult(incidentID string, succeeded bool, result, errMsg string) (*ai_executorpb.Job, error) {
	js.mu.Lock()
	defer js.mu.Unlock()

	job, ok := js.jobs[incidentID]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", incidentID)
	}

	if succeeded {
		job.State = ai_executorpb.JobState_JOB_SUCCEEDED
	} else {
		job.State = ai_executorpb.JobState_JOB_FAILED
	}
	job.Result = result
	job.Error = errMsg
	job.UpdatedAtMs = time.Now().UnixMilli()
	go js.persistJob(job)

	return job, nil
}

// getJob returns a job by incident ID.
func (js *jobStore) getJob(incidentID string) *ai_executorpb.Job {
	js.mu.RLock()
	defer js.mu.RUnlock()
	return js.jobs[incidentID]
}

// listJobs returns jobs filtered by state.
func (js *jobStore) listJobs(stateFilter ai_executorpb.JobState, limit int) []*ai_executorpb.Job {
	js.mu.RLock()
	defer js.mu.RUnlock()

	var result []*ai_executorpb.Job
	for _, job := range js.jobs {
		if stateFilter != 0 && job.State != stateFilter {
			continue
		}
		result = append(result, job)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// expireStaleApprovals checks for expired approval requests.
func (js *jobStore) expireStaleApprovals() int {
	js.mu.Lock()
	defer js.mu.Unlock()

	now := time.Now().UnixMilli()
	expired := 0
	for _, job := range js.jobs {
		if job.State == ai_executorpb.JobState_JOB_AWAITING_APPROVAL && job.ExpiresAtMs > 0 && now > job.ExpiresAtMs {
			job.State = ai_executorpb.JobState_JOB_EXPIRED
			job.UpdatedAtMs = now
			go js.persistJob(job)
			expired++
			logger.Info("job expired", "incident_id", job.IncidentId)
		}
	}
	return expired
}

// persistJob writes a job to etcd (best-effort).
func (js *jobStore) persistJob(job *ai_executorpb.Job) {
	js.mu.RLock()
	cli := js.etcdClient
	js.mu.RUnlock()

	if cli == nil {
		return
	}

	data, err := json.Marshal(job)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := jobKeyPrefix + job.IncidentId
	_, err = cli.Put(ctx, key, string(data))
	if err != nil {
		logger.Debug("job_store: persist failed", "incident_id", job.IncidentId, "err", err)
	}
}

// loadFromEtcd loads all jobs from etcd on startup.
func (js *jobStore) loadFromEtcd() {
	js.mu.RLock()
	cli := js.etcdClient
	js.mu.RUnlock()

	if cli == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := cli.Get(ctx, jobKeyPrefix, clientv3.WithPrefix())
	if err != nil {
		logger.Warn("job_store: load from etcd failed", "err", err)
		return
	}

	js.mu.Lock()
	defer js.mu.Unlock()

	loaded := 0
	for _, kv := range resp.Kvs {
		var job ai_executorpb.Job
		if err := json.Unmarshal(kv.Value, &job); err != nil {
			continue
		}
		js.jobs[job.IncidentId] = &job
		loaded++
	}
	logger.Info("job_store: loaded jobs from etcd", "count", loaded)
}
