package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/backup_hook/backup_hookpb"
	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
	"github.com/globulario/services/golang/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// knownStatefulServices lists services that hold mutable state and should implement
// backup hooks. Used by resolveHookTargets for auto-discovery and strict validation.
var knownStatefulServices = []string{
	"resource.ResourceService",
	"catalog.CatalogService",
	"title.TitleService",
	"search.SearchService",
	"dns.DnsService",
}

// resolveHookTargets returns the effective list of hook targets for this backup.
// If HookDiscovery is enabled, it discovers services from etcd that match
// knownStatefulServices and builds targets dynamically.
// Otherwise, falls back to the static HookTargets config.
func (srv *server) resolveHookTargets(scope *backup_managerpb.BackupScope) []HookTargetConfig {
	if !srv.HookDiscovery {
		return srv.HookTargets
	}

	// Auto-discover hook targets from etcd service registry
	cfgs, err := config.GetServicesConfigurations()
	if err != nil {
		slog.Warn("hook discovery: etcd query failed, falling back to static targets", "error", err)
		return srv.HookTargets
	}

	// Build a set of stateful service names to look for
	wanted := make(map[string]bool)
	for _, name := range knownStatefulServices {
		wanted[name] = true
	}

	// If scope has specific services, only discover those
	if scope != nil && len(scope.Services) > 0 {
		wanted = make(map[string]bool)
		for _, s := range scope.Services {
			wanted[s] = true
		}
	}

	var targets []HookTargetConfig
	discovered := make(map[string]bool) // dedup by name

	for _, c := range cfgs {
		name, _ := c["Name"].(string)
		if !wanted[name] {
			continue
		}
		if discovered[name] {
			continue
		}

		var port int
		switch p := c["Port"].(type) {
		case float64:
			port = int(p)
		case int:
			port = p
		}
		if port <= 0 {
			continue
		}

		address, _ := c["Address"].(string)
		if address == "" {
			address = "localhost"
		}
		if !strings.Contains(address, ":") {
			address = fmt.Sprintf("%s:%d", address, port)
		}

		targets = append(targets, HookTargetConfig{
			Name:    name,
			Address: address,
		})
		discovered[name] = true
	}

	slog.Info("hook targets resolved", "discovered", len(targets), "mode", "auto-discovery")
	return targets
}

// validateHookCoverage checks that all known stateful services have hook targets.
// Returns an error if HookStrict is true and any are missing.
func (srv *server) validateHookCoverage(targets []HookTargetConfig) error {
	if !srv.HookStrict {
		return nil
	}

	covered := make(map[string]bool)
	for _, t := range targets {
		covered[t.Name] = true
	}

	var missing []string
	for _, name := range knownStatefulServices {
		if !covered[name] {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("hook coverage incomplete (HookStrict=true): missing hooks for %v", missing)
	}
	return nil
}

// runPrepareHooks calls PrepareBackup on all resolved hook targets in parallel.
func (srv *server) runPrepareHooks(ctx context.Context, backupID string, mode backup_managerpb.BackupMode, scope *backup_managerpb.BackupScope, labels map[string]string) []*backup_managerpb.HookResult {
	targets := srv.resolveHookTargets(scope)
	if len(targets) == 0 {
		return nil
	}

	timeout := time.Duration(srv.HookTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	rqst := &backup_hookpb.PrepareBackupRequest{
		BackupId:        backupID,
		Mode:            modeToString(mode),
		Labels:          labels,
		DeadlineSeconds: int32(srv.HookTimeoutSeconds),
	}
	if scope != nil {
		rqst.Providers = scope.Providers
		rqst.Services = scope.Services
	}

	return srv.callHooksParallel(ctx, targets, timeout, func(ctx context.Context, client backup_hookpb.BackupHookServiceClient, target HookTargetConfig) *backup_managerpb.HookResult {
		start := time.Now()
		resp, err := client.PrepareBackup(ctx, rqst)
		dur := time.Since(start).Milliseconds()

		if err != nil {
			return &backup_managerpb.HookResult{
				ServiceName: target.Name,
				Ok:          false,
				Message:     fmt.Sprintf("PrepareBackup RPC failed: %v", err),
				DurationMs:  dur,
			}
		}
		return &backup_managerpb.HookResult{
			ServiceName: target.Name,
			Ok:          resp.Ok,
			Message:     resp.Message,
			Details:     resp.Details,
			DurationMs:  dur,
		}
	})
}

// runFinalizeHooks calls FinalizeBackup on all resolved hook targets in parallel.
// This must always run, even if the backup failed.
func (srv *server) runFinalizeHooks(ctx context.Context, backupID string, mode backup_managerpb.BackupMode, scope *backup_managerpb.BackupScope, labels map[string]string, backupSucceeded bool) []*backup_managerpb.HookResult {
	targets := srv.resolveHookTargets(scope)
	if len(targets) == 0 {
		return nil
	}

	timeout := time.Duration(srv.HookTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	rqst := &backup_hookpb.FinalizeBackupRequest{
		BackupId:        backupID,
		Mode:            modeToString(mode),
		Labels:          labels,
		BackupSucceeded: backupSucceeded,
	}
	if scope != nil {
		rqst.Providers = scope.Providers
		rqst.Services = scope.Services
	}

	return srv.callHooksParallel(ctx, targets, timeout, func(ctx context.Context, client backup_hookpb.BackupHookServiceClient, target HookTargetConfig) *backup_managerpb.HookResult {
		start := time.Now()
		resp, err := client.FinalizeBackup(ctx, rqst)
		dur := time.Since(start).Milliseconds()

		if err != nil {
			return &backup_managerpb.HookResult{
				ServiceName: target.Name,
				Ok:          false,
				Message:     fmt.Sprintf("FinalizeBackup RPC failed: %v", err),
				DurationMs:  dur,
			}
		}
		return &backup_managerpb.HookResult{
			ServiceName: target.Name,
			Ok:          resp.Ok,
			Message:     resp.Message,
			Details:     resp.Details,
			DurationMs:  dur,
		}
	})
}

type hookCallFn func(ctx context.Context, client backup_hookpb.BackupHookServiceClient, target HookTargetConfig) *backup_managerpb.HookResult

// callHooksParallel calls hook targets in parallel with per-target timeout.
func (srv *server) callHooksParallel(ctx context.Context, targets []HookTargetConfig, timeout time.Duration, fn hookCallFn) []*backup_managerpb.HookResult {
	var mu sync.Mutex
	var results []*backup_managerpb.HookResult
	var wg sync.WaitGroup

	for _, target := range targets {
		wg.Add(1)
		go func(t HookTargetConfig) {
			defer wg.Done()

			hookCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			conn, err := grpc.DialContext(hookCtx, t.Address,
				append(srv.hookDialOpts(t), grpc.WithBlock())...,
			)
			if err != nil {
				mu.Lock()
				results = append(results, &backup_managerpb.HookResult{
					ServiceName: t.Name,
					Ok:          false,
					Message:     fmt.Sprintf("dial failed: %v", err),
				})
				mu.Unlock()
				slog.Warn("hook dial failed", "target", t.Name, "address", t.Address, "error", err)
				return
			}
			defer conn.Close()

			client := backup_hookpb.NewBackupHookServiceClient(conn)
			result := fn(hookCtx, client, t)

			mu.Lock()
			results = append(results, result)
			mu.Unlock()

			if result.Ok {
				slog.Info("hook succeeded", "target", t.Name, "duration_ms", result.DurationMs)
			} else {
				slog.Warn("hook failed", "target", t.Name, "message", result.Message, "duration_ms", result.DurationMs)
			}
		}(target)
	}

	wg.Wait()
	return results
}

// anyHookFailed returns true if any hook result has ok=false.
func anyHookFailed(results []*backup_managerpb.HookResult) bool {
	for _, r := range results {
		if !r.Ok {
			return true
		}
	}
	return false
}

// hookDialOpts returns gRPC dial options for connecting to a hook target.
// Uses TLS with the backup-manager's CA/cert/key when TLS is enabled,
// falls back to insecure plaintext otherwise.
func (srv *server) hookDialOpts(t HookTargetConfig) []grpc.DialOption {
	if !srv.TLS {
		return []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	}

	tlsCfg, err := srv.hookTLSConfig(t.Address)
	if err != nil {
		if srv.HookAllowInsecureFallback {
			slog.Warn("hook TLS config failed, falling back to insecure (HookAllowInsecureFallback=true)", "target", t.Name, "error", err)
			return []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
		}
		slog.Error("hook TLS config failed and insecure fallback is disabled", "target", t.Name, "error", err)
		return nil // will cause dial to fail, surfacing the TLS misconfiguration
	}

	return []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg))}
}

// hookTLSConfig builds a TLS config using the backup-manager's PKI material.
func (srv *server) hookTLSConfig(address string) (*tls.Config, error) {
	caFile := srv.CertAuthorityTrust
	if caFile == "" {
		return nil, fmt.Errorf("no CA certificate configured (CertAuthorityTrust)")
	}

	caData, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read CA %s: %w", caFile, err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caData) {
		return nil, fmt.Errorf("failed to parse CA certificate %s", caFile)
	}

	// SNI: use the host portion of the target address
	sni := address
	if idx := strings.Index(sni, ":"); idx > 0 {
		sni = sni[:idx]
	}

	cfg := &tls.Config{
		ServerName: sni,
		RootCAs:    pool,
		MinVersion: tls.VersionTLS12,
	}

	// Load client cert for mTLS if available
	if srv.CertFile != "" && srv.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(srv.CertFile, srv.KeyFile)
		if err == nil {
			cfg.Certificates = []tls.Certificate{cert}
		} else {
			slog.Warn("hook mTLS: client keypair unavailable, using server-auth only",
				"cert", srv.CertFile, "key", srv.KeyFile, "error", err)
		}
	}

	return cfg, nil
}

func modeToString(mode backup_managerpb.BackupMode) string {
	switch mode {
	case backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER:
		return "CLUSTER"
	case backup_managerpb.BackupMode_BACKUP_MODE_SERVICE:
		return "SERVICE"
	default:
		return "UNSPECIFIED"
	}
}
