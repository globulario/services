// Package backup_hook provides a shared backup hook handler with optional write-gate
// for Globular services that implement the BackupHookService contract.
package backup_hook

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/backup_hook/backup_hookpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FlushFunc is called during PrepareBackup to flush pending state.
// It should complete quickly (< 3s target, 10s max).
// Return details map with counters like "flushed_items", "synced_indexes", etc.
type FlushFunc func(ctx context.Context, backupID string) (details map[string]string, err error)

// ResumeFunc is called during FinalizeBackup to resume normal operation.
type ResumeFunc func(ctx context.Context, backupID string) error

// HookHandler implements BackupHookServiceServer with write-gate support.
type HookHandler struct {
	ServiceName string

	// Callbacks
	OnFlush  FlushFunc
	OnResume ResumeFunc

	// Write-gate state
	writeGateEnabled bool
	writeBlocked     atomic.Bool
	mu               sync.Mutex
	currentBackupID  string
	blockedSince     time.Time
}

// NewHookHandler creates a hook handler for the given service.
// If writeGate is true, PrepareBackup in CLUSTER mode will block write RPCs.
func NewHookHandler(serviceName string, writeGate bool, flush FlushFunc, resume ResumeFunc) *HookHandler {
	return &HookHandler{
		ServiceName:      serviceName,
		OnFlush:          flush,
		OnResume:         resume,
		writeGateEnabled: writeGate,
	}
}

// PrepareBackup flushes pending state and optionally blocks writes.
func (h *HookHandler) PrepareBackup(ctx context.Context, rqst *backup_hookpb.PrepareBackupRequest) (*backup_hookpb.PrepareBackupResponse, error) {
	slog.Info("backup hook: PrepareBackup called", "service", h.ServiceName, "backup_id", rqst.BackupId, "mode", rqst.Mode)

	details := make(map[string]string)

	// Flush pending state
	if h.OnFlush != nil {
		flushDetails, err := h.OnFlush(ctx, rqst.BackupId)
		if err != nil {
			slog.Warn("backup hook: flush failed", "service", h.ServiceName, "error", err)
			return &backup_hookpb.PrepareBackupResponse{
				Ok:      false,
				Message: fmt.Sprintf("flush failed: %v", err),
				Details: details,
			}, nil
		}
		for k, v := range flushDetails {
			details[k] = v
		}
	}

	// Enable write-gate for CLUSTER mode if configured
	if h.writeGateEnabled && rqst.Mode == "CLUSTER" {
		h.mu.Lock()
		h.currentBackupID = rqst.BackupId
		h.blockedSince = time.Now()
		h.writeBlocked.Store(true)
		h.mu.Unlock()
		details["write_gate"] = "enabled"
		slog.Info("backup hook: write-gate enabled", "service", h.ServiceName)
	}

	return &backup_hookpb.PrepareBackupResponse{
		Ok:      true,
		Message: "prepared",
		Details: details,
	}, nil
}

// FinalizeBackup resumes normal operation and unblocks writes.
func (h *HookHandler) FinalizeBackup(ctx context.Context, rqst *backup_hookpb.FinalizeBackupRequest) (*backup_hookpb.FinalizeBackupResponse, error) {
	slog.Info("backup hook: FinalizeBackup called", "service", h.ServiceName, "backup_id", rqst.BackupId, "succeeded", rqst.BackupSucceeded)

	details := make(map[string]string)

	// Unblock writes (always, even if already unblocked — idempotent)
	if h.writeBlocked.Load() {
		h.writeBlocked.Store(false)
		h.mu.Lock()
		dur := time.Since(h.blockedSince)
		h.currentBackupID = ""
		h.mu.Unlock()
		details["write_gate"] = "disabled"
		details["blocked_duration_ms"] = fmt.Sprintf("%d", dur.Milliseconds())
		slog.Info("backup hook: write-gate disabled", "service", h.ServiceName, "blocked_ms", dur.Milliseconds())
	}

	// Resume workers/background jobs
	if h.OnResume != nil {
		if err := h.OnResume(ctx, rqst.BackupId); err != nil {
			slog.Warn("backup hook: resume failed", "service", h.ServiceName, "error", err)
			return &backup_hookpb.FinalizeBackupResponse{
				Ok:      false,
				Message: fmt.Sprintf("resume failed: %v", err),
				Details: details,
			}, nil
		}
	}

	return &backup_hookpb.FinalizeBackupResponse{
		Ok:      true,
		Message: "finalized",
		Details: details,
	}, nil
}

// IsWriteBlocked returns whether writes are currently blocked by a backup.
func (h *HookHandler) IsWriteBlocked() bool {
	return h.writeBlocked.Load()
}

// WriteGateInterceptor returns a gRPC unary server interceptor that rejects
// write RPCs while a backup is in progress (write-gate enabled).
// writeMethods is a list of method name prefixes that are considered writes,
// e.g. ["Create", "Update", "Delete", "Set", "Save", "Put", "Remove"].
func WriteGateInterceptor(h *HookHandler, writeMethods []string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if h.IsWriteBlocked() && isWriteMethod(info.FullMethod, writeMethods) {
			return nil, status.Errorf(codes.FailedPrecondition, "backup in progress: writes temporarily blocked")
		}
		return handler(ctx, req)
	}
}

// isWriteMethod checks if the gRPC method name matches any write prefix.
// FullMethod looks like "/package.Service/MethodName".
func isWriteMethod(fullMethod string, prefixes []string) bool {
	// Extract method name after the last "/"
	parts := strings.Split(fullMethod, "/")
	method := parts[len(parts)-1]
	for _, prefix := range prefixes {
		if strings.HasPrefix(method, prefix) {
			return true
		}
	}
	return false
}

// Register registers the BackupHookServiceServer on the given gRPC server.
func Register(gs *grpc.Server, h *HookHandler) {
	backup_hookpb.RegisterBackupHookServiceServer(gs, h)
}
