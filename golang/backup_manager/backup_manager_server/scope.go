package main

import (
	"fmt"
	"log/slog"
	"os/exec"
	"sort"
	"strings"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
)

// defaultProviderOrder defines the deterministic execution order for cluster snapshots.
var defaultProviderOrder = []string{"etcd", "scylla", "restic", "minio"}

// knownProviders is the set of valid provider names.
var knownProviders = map[string]backup_managerpb.BackupProviderType{
	"etcd":   backup_managerpb.BackupProviderType_BACKUP_PROVIDER_ETCD,
	"restic": backup_managerpb.BackupProviderType_BACKUP_PROVIDER_RESTIC,
	"minio":  backup_managerpb.BackupProviderType_BACKUP_PROVIDER_MINIO,
	"scylla": backup_managerpb.BackupProviderType_BACKUP_PROVIDER_SCYLLA,
}

// replicationProviders are always executed last (they ship the sealed capsule).
var replicationProviders = map[string]bool{
	"minio": true,
}

// ProviderAvailability holds the result of checking whether a provider can run.
type ProviderAvailability struct {
	Available bool
	Reason    string // non-empty when unavailable
}

// ResolveResult is the output of ResolveProviders.
type ResolveResult struct {
	Specs   []*backup_managerpb.BackupProviderSpec
	Names   []string
	Skipped []*backup_managerpb.SkippedProvider
}

// ResolveProviders determines which providers to run based on mode, scope, plan, and config.
// It checks availability for each provider and either skips or errors depending on strict mode.
func ResolveProviders(
	mode backup_managerpb.BackupMode,
	scope *backup_managerpb.BackupScope,
	planProviders []*backup_managerpb.BackupProviderSpec,
	srv *server,
) (*ResolveResult, error) {

	var requestedNames []string

	switch mode {
	case backup_managerpb.BackupMode_BACKUP_MODE_SERVICE, backup_managerpb.BackupMode_BACKUP_MODE_UNSPECIFIED:
		requestedNames = resolveServiceProviders(scope, planProviders)
		if len(requestedNames) == 0 {
			return nil, fmt.Errorf("SERVICE mode requires at least one provider")
		}
		if len(requestedNames) > 1 {
			return nil, fmt.Errorf("SERVICE mode requires exactly 1 provider, got %d: %v", len(requestedNames), requestedNames)
		}

	case backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER:
		requestedNames = resolveClusterProviders(scope, planProviders, srv.ClusterDefaultProviders)
		if len(requestedNames) == 0 {
			return nil, fmt.Errorf("CLUSTER mode requires providers via scope, plan, or ClusterDefaultProviders config")
		}
	}

	// Deduplicate and normalize
	requestedNames = dedupe(requestedNames)

	// Validate all names
	for _, name := range requestedNames {
		if _, ok := knownProviders[name]; !ok {
			return nil, fmt.Errorf("unknown provider %q; valid providers: etcd, restic, minio, scylla", name)
		}
	}

	// Check availability for each provider
	var activeNames []string
	var skipped []*backup_managerpb.SkippedProvider

	for _, name := range requestedNames {
		avail := checkProviderAvailability(name, srv)
		if avail.Available {
			activeNames = append(activeNames, name)
		} else {
			if srv.ClusterStrictDefaults {
				return nil, fmt.Errorf("provider %q unavailable (ClusterStrictDefaults=true): %s", name, avail.Reason)
			}
			skipped = append(skipped, &backup_managerpb.SkippedProvider{
				Name:   name,
				Reason: avail.Reason,
			})
			slog.Info("provider skipped", "provider", name, "reason", avail.Reason)
		}
	}

	if len(activeNames) == 0 && mode == backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER {
		return nil, fmt.Errorf("all providers were skipped; cannot proceed with CLUSTER backup")
	}

	// Sort in deterministic order
	orderMap := buildOrderMap(srv.ClusterProviderOrder)
	sort.Slice(activeNames, func(i, j int) bool {
		return orderMap[activeNames[i]] < orderMap[activeNames[j]]
	})

	// Enforce replication providers always last
	enforceReplicationLast(activeNames)

	// Build provider specs, inheriting options from plan if available
	planMap := make(map[backup_managerpb.BackupProviderType]*backup_managerpb.BackupProviderSpec)
	for _, p := range planProviders {
		planMap[p.Type] = p
	}

	var specs []*backup_managerpb.BackupProviderSpec
	for _, name := range activeNames {
		provType := knownProviders[name]
		if existing, ok := planMap[provType]; ok {
			cloned := &backup_managerpb.BackupProviderSpec{
				Type:           existing.Type,
				Enabled:        true,
				Options:        existing.Options,
				TimeoutSeconds: existing.TimeoutSeconds,
			}
			specs = append(specs, cloned)
		} else {
			specs = append(specs, &backup_managerpb.BackupProviderSpec{
				Type:    provType,
				Enabled: true,
			})
		}
	}

	return &ResolveResult{
		Specs:   specs,
		Names:   activeNames,
		Skipped: skipped,
	}, nil
}

// checkProviderAvailability checks whether a provider is configured and its tools are reachable.
func checkProviderAvailability(name string, srv *server) ProviderAvailability {
	switch name {
	case "etcd":
		if _, err := exec.LookPath("etcdctl"); err != nil {
			return ProviderAvailability{false, "etcdctl not found in PATH"}
		}
		return ProviderAvailability{Available: true}

	case "restic":
		if _, err := exec.LookPath("restic"); err != nil {
			return ProviderAvailability{false, "restic not found in PATH"}
		}
		if srv.ResticRepo == "" {
			return ProviderAvailability{false, "ResticRepo not configured"}
		}
		return ProviderAvailability{Available: true}

	case "scylla":
		if _, err := exec.LookPath("sctool"); err != nil {
			return ProviderAvailability{false, "sctool not found in PATH"}
		}
		if srv.ScyllaCluster == "" {
			return ProviderAvailability{false, "ScyllaCluster not configured"}
		}
		if srv.ScyllaLocation == "" && len(srv.scyllaLocations()) == 0 {
			return ProviderAvailability{false, "no ScyllaDB-compatible destinations (requires S3/GCS/Azure, not local)"}
		}
		return ProviderAvailability{Available: true}

	case "minio":
		if _, err := exec.LookPath("rclone"); err != nil {
			return ProviderAvailability{false, "rclone not found in PATH"}
		}
		if srv.RcloneRemote == "" {
			return ProviderAvailability{false, "RcloneRemote not configured"}
		}
		return ProviderAvailability{Available: true}

	default:
		return ProviderAvailability{false, fmt.Sprintf("unknown provider %q", name)}
	}
}

// buildOrderMap returns a name→position map from the given order list,
// falling back to defaultProviderOrder if empty.
func buildOrderMap(customOrder []string) map[string]int {
	order := defaultProviderOrder
	if len(customOrder) > 0 {
		order = customOrder
	}
	m := make(map[string]int)
	for i, name := range order {
		m[strings.TrimSpace(strings.ToLower(name))] = i
	}
	// Unknowns get a high index so they sort to the end
	return m
}

// enforceReplicationLast moves replication providers to the end of the slice.
func enforceReplicationLast(names []string) {
	n := len(names)
	if n <= 1 {
		return
	}
	// Partition: non-replication first, replication last
	writeIdx := 0
	var repNames []string
	for _, name := range names {
		if replicationProviders[name] {
			repNames = append(repNames, name)
		} else {
			names[writeIdx] = name
			writeIdx++
		}
	}
	for _, r := range repNames {
		names[writeIdx] = r
		writeIdx++
	}
}

func resolveServiceProviders(scope *backup_managerpb.BackupScope, planProviders []*backup_managerpb.BackupProviderSpec) []string {
	if scope != nil && len(scope.Providers) > 0 {
		return scope.Providers
	}
	return enabledProviderNames(planProviders)
}

func resolveClusterProviders(scope *backup_managerpb.BackupScope, planProviders []*backup_managerpb.BackupProviderSpec, clusterDefaults []string) []string {
	if scope != nil && len(scope.Providers) > 0 {
		return scope.Providers
	}
	if len(clusterDefaults) > 0 {
		return clusterDefaults
	}
	return enabledProviderNames(planProviders)
}

func enabledProviderNames(specs []*backup_managerpb.BackupProviderSpec) []string {
	var names []string
	for _, s := range specs {
		if s.Enabled {
			names = append(names, providerName(s.Type))
		}
	}
	return names
}

func dedupe(items []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range items {
		item = strings.TrimSpace(strings.ToLower(item))
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}
