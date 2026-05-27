package audittrail

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
)

const desiredWritePrefix = "/globular/audit/desired_writes/"

// DesiredWriteRecord is the durable provenance envelope for desired-state writes.
type DesiredWriteRecord struct {
	Service       string `json:"service"`
	Actor         string `json:"actor"`
	Source        string `json:"source"`
	Action        string `json:"action"`
	Reason        string `json:"reason"`
	Timestamp     string `json:"timestamp"`
	WorkflowRunID string `json:"workflow_run_id,omitempty"`
	EtcdRevision  int64  `json:"etcd_revision,omitempty"`
}

// WriteDesiredWriteRecord appends one desired-state provenance record.
// It is read-only with respect to desired-state itself; it only writes audit keys.
func WriteDesiredWriteRecord(ctx context.Context, rec DesiredWriteRecord) error {
	rec.Service = strings.TrimSpace(rec.Service)
	rec.Actor = strings.TrimSpace(rec.Actor)
	rec.Source = strings.TrimSpace(rec.Source)
	rec.Action = strings.TrimSpace(rec.Action)
	rec.Reason = strings.TrimSpace(rec.Reason)
	if rec.Timestamp == "" {
		rec.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if err := validateDesiredWriteRecord(rec); err != nil {
		return err
	}

	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("desired write provenance: marshal: %w", err)
	}

	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("desired write provenance: etcd client: %w", err)
	}

	ts := time.Now().UTC().Format("20060102T150405.000000000")
	key := fmt.Sprintf("%s%s_%s", desiredWritePrefix, ts, rec.Service)
	wctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err = cli.Put(wctx, key, string(data))
	if err != nil {
		return fmt.Errorf("desired write provenance: put: %w", err)
	}
	return nil
}

func validateDesiredWriteRecord(rec DesiredWriteRecord) error {
	if rec.Service == "" || rec.Actor == "" || rec.Source == "" || rec.Action == "" || rec.Reason == "" {
		return fmt.Errorf("desired write provenance: service, actor, source, action, and reason are required")
	}
	return nil
}
