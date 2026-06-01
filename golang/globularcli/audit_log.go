package main

// audit_log.go — Durable audit log for all repair and state mutations.
//
// Every canonicalization repair, ghost cleanup, or state mutation produces
// an audit record. Records are stored in etcd under /globular/audit/ with
// a time-ordered key. This ensures repairs never silently rewrite history.
//
// INV-10: Repair never silently rewrites history.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/globulario/services/golang/config"
)

// auditRecord is a single repair/mutation audit entry.
type auditRecord struct {
	Timestamp   string `json:"timestamp"`
	Action      string `json:"action"`       // e.g. "fix-installed", "fix-safe", "cleanup-ghost"
	Service     string `json:"service"`
	Node        string `json:"node,omitempty"`
	BeforeState string `json:"before_state,omitempty"`
	AfterState  string `json:"after_state,omitempty"`
	BuildID     string `json:"build_id,omitempty"`
	Detail      string `json:"detail,omitempty"`
	Operator    string `json:"operator"`     // "canonicalize-tool" or user
}

// writeAuditRecord persists a single audit entry to etcd.
func writeAuditRecord(ctx context.Context, rec auditRecord) {
	if rec.Timestamp == "" {
		rec.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if rec.Operator == "" {
		rec.Operator = "canonicalize-tool"
	}

	data, err := json.Marshal(rec)
	if err != nil {
		return
	}

	cli, err := config.GetEtcdClient()
	if err != nil {
		return
	}

	// Key: /globular/audit/{timestamp}_{action}_{service}
	key := fmt.Sprintf("/globular/audit/%s_%s_%s",
		time.Now().UTC().Format("20060102T150405.000"),
		rec.Action, rec.Service)

	writeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, _ = cli.Put(writeCtx, key, string(data))
}
