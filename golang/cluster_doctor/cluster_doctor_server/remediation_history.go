package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const remediationAuditEtcdPrefix = "/globular/cluster_doctor/audit/"

var listRemediationAuditsFn = listRemediationAuditsFromEtcd

type remediationActionStat struct {
	ActionType string
	Successes  int
	LastTs     int64
}

func summarizeHistoricalSuccessfulActions(ctx context.Context, invariantID, evidenceDigest string, limit int) []remediationActionStat {
	if invariantID == "" || evidenceDigest == "" {
		return nil
	}
	audits, err := listRemediationAuditsFn(ctx, limit)
	if err != nil {
		return nil
	}
	stats := map[string]remediationActionStat{}
	for _, a := range audits {
		if a.InvariantID != invariantID || a.EvidenceDigest != evidenceDigest {
			continue
		}
		if a.Rejected || !a.Executed {
			continue
		}
		current := stats[a.ActionType]
		current.ActionType = a.ActionType
		current.Successes++
		if a.Timestamp > current.LastTs {
			current.LastTs = a.Timestamp
		}
		stats[a.ActionType] = current
	}
	out := make([]remediationActionStat, 0, len(stats))
	for _, s := range stats {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Successes == out[j].Successes {
			return out[i].LastTs > out[j].LastTs
		}
		return out[i].Successes > out[j].Successes
	})
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}

func listRemediationAuditsFromEtcd(ctx context.Context, limit int) ([]RemediationAudit, error) {
	if limit <= 0 {
		limit = 200
	}
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, err
	}
	getCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := cli.Get(getCtx, remediationAuditEtcdPrefix, clientv3.WithPrefix(), clientv3.WithLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	out := make([]RemediationAudit, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var a RemediationAudit
		if err := json.Unmarshal(kv.Value, &a); err != nil {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

func historicalActionsHint(stats []remediationActionStat) string {
	if len(stats) == 0 {
		return ""
	}
	hint := "Historical successful actions for similar evidence:"
	for _, s := range stats {
		hint += fmt.Sprintf(" %s(success=%d,last=%s);", s.ActionType, s.Successes, time.Unix(s.LastTs, 0).UTC().Format(time.RFC3339))
	}
	return hint
}

func countRecentFailedActionAttempts(ctx context.Context, invariantID, evidenceDigest, actionType string, since time.Time, limit int) int {
	if invariantID == "" || evidenceDigest == "" || actionType == "" {
		return 0
	}
	audits, err := listRemediationAuditsFn(ctx, limit)
	if err != nil {
		return 0
	}
	sinceUnix := since.Unix()
	count := 0
	for _, a := range audits {
		if a.InvariantID != invariantID || a.EvidenceDigest != evidenceDigest || a.ActionType != actionType {
			continue
		}
		if a.Timestamp < sinceUnix || a.DryRun {
			continue
		}
		if !a.Executed && a.Reason != "" {
			count++
		}
	}
	return count
}
