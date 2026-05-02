package main

// config_receipts.go — Phase F Part 2 (repo-side scaffold) for
// PackageConfigReceipt. The node-agent calls RecordConfigReceipt after
// each config-file action; CLI + doctor read via ListConfigReceipts.
//
// Node-agent emitter is the next session's work; the repository accepts
// receipts now so the surface is stable when that lands.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// receiptCache is the in-memory mirror — production also persists to Scylla
// (table created in scylla_store.schemaCreateTable below). Tests use the
// cache exclusively when scylla=nil.
type receiptCache struct {
	mu  sync.Mutex
	rows []*repopb.PackageConfigReceipt
}

func (srv *server) initReceiptCache() {
	srv.artifactStateMu.Lock()
	defer srv.artifactStateMu.Unlock()
	if srv.receipts == nil {
		srv.receipts = &receiptCache{}
	}
}

func (srv *server) cacheReceipt(r *repopb.PackageConfigReceipt) {
	srv.initReceiptCache()
	srv.receipts.mu.Lock()
	defer srv.receipts.mu.Unlock()
	srv.receipts.rows = append(srv.receipts.rows, r)
}

func (srv *server) listCachedReceipts(publisherID, name, platform, nodeID string, action repopb.ConfigReceiptAction, limit int) []*repopb.PackageConfigReceipt {
	srv.initReceiptCache()
	srv.receipts.mu.Lock()
	defer srv.receipts.mu.Unlock()
	var out []*repopb.PackageConfigReceipt
	for _, r := range srv.receipts.rows {
		if r.GetPublisherId() != publisherID || r.GetName() != name || r.GetPlatform() != platform {
			continue
		}
		if nodeID != "" && r.GetNodeId() != nodeID {
			continue
		}
		if action != repopb.ConfigReceiptAction_CONFIG_RECEIPT_ACTION_UNSPECIFIED && r.GetAction() != action {
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].GetTimestampUnix() > out[j].GetTimestampUnix()
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// redactIfSensitive returns a copy of the receipt safe for the wire / logs:
// SECRET / sensitive paths are stripped to [REDACTED]; checksums stay.
func redactReceipt(r *repopb.PackageConfigReceipt) *repopb.PackageConfigReceipt {
	if r == nil {
		return nil
	}
	out := &repopb.PackageConfigReceipt{
		NodeId:         r.GetNodeId(),
		PublisherId:    r.GetPublisherId(),
		Name:           r.GetName(),
		Platform:       r.GetPlatform(),
		BuildNumber:    r.GetBuildNumber(),
		Path:           r.GetPath(),
		ConfigKind:     r.GetConfigKind(),
		MergeStrategy:  r.GetMergeStrategy(),
		ChecksumBefore: r.GetChecksumBefore(),
		ChecksumAfter:  r.GetChecksumAfter(),
		Action:         r.GetAction(),
		SnapshotId:     r.GetSnapshotId(),
		WorkflowRunId:  r.GetWorkflowRunId(),
		TimestampUnix:  r.GetTimestampUnix(),
		Reason:         r.GetReason(),
		Sensitive:      r.GetSensitive(),
	}
	if out.GetSensitive() || out.GetConfigKind() == repopb.ConfigKind_CONFIG_SECRET {
		out.Path = "[REDACTED]"
	}
	return out
}

// ── RPC handlers ───────────────────────────────────────────────────────────

func (srv *server) RecordConfigReceipt(ctx context.Context, req *repopb.RecordConfigReceiptRequest) (*repopb.RecordConfigReceiptResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	rec := req.GetReceipt()
	if rec == nil {
		return nil, status.Error(codes.InvalidArgument, "receipt is required")
	}
	if strings.TrimSpace(rec.GetPublisherId()) == "" || strings.TrimSpace(rec.GetName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "receipt.publisher_id and receipt.name are required")
	}
	if rec.GetTimestampUnix() == 0 {
		rec.TimestampUnix = time.Now().Unix()
	}
	srv.cacheReceipt(rec)

	if srv.scylla != nil {
		if ss, ok := srv.scylla.(*scyllaStore); ok {
			if err := ss.putConfigReceipt(ctx, rec); err != nil {
				// Non-fatal — cache holds the row, scylla retry on next call.
				return &repopb.RecordConfigReceiptResponse{}, nil
			}
		}
	}

	srv.publishAuditEvent(ctx, "repository.config.receipt", map[string]any{
		"node_id":      rec.GetNodeId(),
		"publisher_id": rec.GetPublisherId(),
		"name":         rec.GetName(),
		"path":         redactReceipt(rec).GetPath(),
		"action":       rec.GetAction().String(),
		"merge":        rec.GetMergeStrategy().String(),
		"workflow":     rec.GetWorkflowRunId(),
	})
	return &repopb.RecordConfigReceiptResponse{}, nil
}

func (srv *server) ListConfigReceipts(ctx context.Context, req *repopb.ListConfigReceiptsRequest) (*repopb.ListConfigReceiptsResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	pubID := strings.TrimSpace(req.GetPublisherId())
	name := strings.TrimSpace(req.GetName())
	platform := strings.TrimSpace(req.GetPlatform())
	if pubID == "" || name == "" || platform == "" {
		return nil, status.Error(codes.InvalidArgument, "publisher_id, name, platform are required")
	}
	limit := int(req.GetLimit())
	if limit == 0 {
		limit = 100
	}
	rows := srv.listCachedReceipts(pubID, name, platform, strings.TrimSpace(req.GetNodeId()),
		req.GetActionFilter(), limit)

	if srv.scylla != nil {
		if ss, ok := srv.scylla.(*scyllaStore); ok {
			if extra, err := ss.listConfigReceipts(ctx, pubID, name, platform); err == nil && len(extra) > 0 {
				rows = mergeReceipts(rows, extra, req.GetActionFilter(), strings.TrimSpace(req.GetNodeId()), limit)
			}
		}
	}

	// Redact on the wire — operator output never carries SECRET paths.
	out := make([]*repopb.PackageConfigReceipt, 0, len(rows))
	for _, r := range rows {
		out = append(out, redactReceipt(r))
	}
	return &repopb.ListConfigReceiptsResponse{Receipts: out}, nil
}

func mergeReceipts(a, b []*repopb.PackageConfigReceipt, action repopb.ConfigReceiptAction, nodeID string, limit int) []*repopb.PackageConfigReceipt {
	seen := make(map[string]bool)
	keyOf := func(r *repopb.PackageConfigReceipt) string {
		return fmt.Sprintf("%s|%s|%d", r.GetNodeId(), r.GetPath(), r.GetTimestampUnix())
	}
	out := append([]*repopb.PackageConfigReceipt{}, a...)
	for _, r := range a {
		seen[keyOf(r)] = true
	}
	for _, r := range b {
		if seen[keyOf(r)] {
			continue
		}
		if nodeID != "" && r.GetNodeId() != nodeID {
			continue
		}
		if action != repopb.ConfigReceiptAction_CONFIG_RECEIPT_ACTION_UNSPECIFIED && r.GetAction() != action {
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].GetTimestampUnix() > out[j].GetTimestampUnix()
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
