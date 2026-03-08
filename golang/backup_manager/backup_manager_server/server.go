package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	defaultPort  = 10040
	defaultProxy = defaultPort + 1

	allowAllOrigins   = true
	allowedOriginsStr = ""
)

type server struct {
	// --- Service Identity ---
	Id          string
	Mac         string
	Name        string
	Domain      string
	Address     string
	Path        string
	Proto       string
	Version     string
	PublisherID string
	Description string
	Keywords    []string

	// --- Network Configuration ---
	Port     int
	Proxy    int
	Protocol string

	// --- Service Discovery ---
	Repositories []string
	Discoveries  []string

	// --- Policy & Operations ---
	AllowAllOrigins bool
	AllowedOrigins  string
	KeepUpToDate    bool
	KeepAlive       bool
	Plaform         string // typo preserved for compatibility
	Checksum        string
	Permissions     []any
	Dependencies    []string

	// --- Runtime State ---
	Process      int
	ProxyProcess int
	ConfigPath   string
	LastError    string
	State        string
	ModTime      int64

	// --- TLS Configuration ---
	TLS                bool
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	// --- gRPC Runtime ---
	grpcServer *grpc.Server
	backup_managerpb.UnimplementedBackupManagerServiceServer

	// --- Backup Manager ---
	DataDir           string              `json:"DataDir"`
	MaxConcurrentJobs int                 `json:"MaxConcurrentJobs"`
	Destinations      []DestinationConfig `json:"Destinations"`
	store             *jobStore
	sem               chan struct{}   // concurrency semaphore
	active            *runningJobs   // cancelable running jobs
	tokens            *tokenStore    // TTL-based confirmation tokens

	// Capsule compression
	CompressCapsule bool `json:"CompressCapsule"` // if true, tar.gz the capsule before replication

	// Deletion safety
	AllowResticPruneOnDelete bool `json:"AllowResticPruneOnDelete"`
	AllowRemoteDelete        bool `json:"AllowRemoteDelete"`

	// Retention policy
	RetentionKeepLastN     int    `json:"RetentionKeepLastN"`
	RetentionKeepDays      int    `json:"RetentionKeepDays"`
	RetentionMaxTotalBytes uint64 `json:"RetentionMaxTotalBytes"`

	// Retention: minimum number of restore-tested backups to keep
	MinRestoreTestedToKeep int `json:"MinRestoreTestedToKeep"`

	// Provider timeout
	ProviderTimeoutSeconds int `json:"ProviderTimeoutSeconds"`

	// Cluster defaults
	ClusterDefaultProviders []string         `json:"ClusterDefaultProviders"` // default providers for CLUSTER mode
	ClusterProviderOrder    []string         `json:"ClusterProviderOrder"`    // custom execution order (optional)
	ClusterStrictDefaults   bool             `json:"ClusterStrictDefaults"`   // if true, error when a default provider is unavailable
	HookTargets             []HookTargetConfig `json:"HookTargets"`           // services that support backup hooks
	HookTimeoutSeconds      int              `json:"HookTimeoutSeconds"`      // per-hook timeout (default 30)
	HookStrict              bool             `json:"HookStrict"`              // if true, abort on prepare hook failure
	HookDiscovery           bool             `json:"HookDiscovery"`           // auto-discover hook targets from etcd
	HookAllowInsecureFallback bool           `json:"HookAllowInsecureFallback"` // allow plaintext fallback when TLS fails

	// Provider config
	EtcdEndpoints    string `json:"EtcdEndpoints"`
	EtcdCACert       string `json:"EtcdCACert"`
	EtcdCert         string `json:"EtcdCert"`
	EtcdKey          string `json:"EtcdKey"`
	ResticRepo       string `json:"ResticRepo"`
	ResticPassword   string `json:"ResticPassword"`
	ResticPaths      string `json:"ResticPaths"`
	RcloneRemote     string `json:"RcloneRemote"`
	RcloneSource     string `json:"RcloneSource"`
	ScyllaManagerAPI string `json:"ScyllaManagerAPI"`
	ScyllaCluster    string `json:"ScyllaCluster"`
	ScyllaLocation   string `json:"ScyllaLocation"`

	// Scheduled backups
	ScheduleInterval string         `json:"ScheduleInterval"`
	stopScheduler    context.CancelFunc
	nextFireTime     atomic.Int64   // unix ms of next scheduled backup

	// MinIO connection
	MinioEndpoint  string `json:"MinioEndpoint"`
	MinioAccessKey string `json:"MinioAccessKey"`
	MinioSecretKey string `json:"MinioSecretKey"`
	MinioSecure    bool   `json:"MinioSecure"`
}

// --- Globular service contract (getters/setters) ---

func (srv *server) GetConfigurationPath() string        { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string)    { srv.ConfigPath = path }
func (srv *server) GetAddress() string                  { return srv.Address }
func (srv *server) SetAddress(address string)           { srv.Address = address }
func (srv *server) GetProcess() int                     { return srv.Process }
func (srv *server) SetProcess(pid int)                  { srv.Process = pid }
func (srv *server) GetProxyProcess() int                { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)             { srv.ProxyProcess = pid }
func (srv *server) GetState() string                    { return srv.State }
func (srv *server) SetState(state string)               { srv.State = state }
func (srv *server) GetLastError() string                { return srv.LastError }
func (srv *server) SetLastError(err string)             { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)            { srv.ModTime = modtime }
func (srv *server) GetModTime() int64                   { return srv.ModTime }
func (srv *server) GetId() string                       { return srv.Id }
func (srv *server) SetId(id string)                     { srv.Id = id }
func (srv *server) GetName() string                     { return srv.Name }
func (srv *server) SetName(name string)                 { srv.Name = name }
func (srv *server) GetDescription() string              { return srv.Description }
func (srv *server) SetDescription(description string)   { srv.Description = description }
func (srv *server) GetMac() string                      { return srv.Mac }
func (srv *server) SetMac(mac string)                   { srv.Mac = mac }
func (srv *server) GetKeywords() []string               { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)       { srv.Keywords = keywords }
func (srv *server) GetRepositories() []string           { return srv.Repositories }
func (srv *server) SetRepositories(repos []string)      { srv.Repositories = repos }
func (srv *server) GetDiscoveries() []string            { return srv.Discoveries }
func (srv *server) SetDiscoveries(disc []string)        { srv.Discoveries = disc }
func (srv *server) Dist(path string) (string, error)    { return globular.Dist(path, srv) }
func (srv *server) GetChecksum() string                 { return srv.Checksum }
func (srv *server) SetChecksum(checksum string)         { srv.Checksum = checksum }
func (srv *server) GetPlatform() string                 { return srv.Plaform }
func (srv *server) SetPlatform(platform string)         { srv.Plaform = platform }
func (srv *server) GetPath() string                     { return srv.Path }
func (srv *server) SetPath(path string)                 { srv.Path = path }
func (srv *server) GetProto() string                    { return srv.Proto }
func (srv *server) SetProto(proto string)               { srv.Proto = proto }
func (srv *server) GetPort() int                        { return srv.Port }
func (srv *server) SetPort(port int)                    { srv.Port = port }
func (srv *server) GetProxy() int                       { return srv.Proxy }
func (srv *server) SetProxy(proxy int)                  { srv.Proxy = proxy }
func (srv *server) GetGrpcServer() *grpc.Server         { return srv.grpcServer }
func (srv *server) GetProtocol() string                 { return srv.Protocol }
func (srv *server) SetProtocol(protocol string)         { srv.Protocol = protocol }
func (srv *server) GetAllowAllOrigins() bool            { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(v bool)           { srv.AllowAllOrigins = v }
func (srv *server) GetAllowedOrigins() string           { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(v string)          { srv.AllowedOrigins = v }
func (srv *server) GetDomain() string                   { return srv.Domain }
func (srv *server) SetDomain(domain string)             { srv.Domain = domain }
func (srv *server) GetTls() bool                        { return srv.TLS }
func (srv *server) SetTls(hasTls bool)                  { srv.TLS = hasTls }
func (srv *server) GetCertAuthorityTrust() string       { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string)     { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string                 { return srv.CertFile }
func (srv *server) SetCertFile(certFile string)         { srv.CertFile = certFile }
func (srv *server) GetKeyFile() string                  { return srv.KeyFile }
func (srv *server) SetKeyFile(keyFile string)           { srv.KeyFile = keyFile }
func (srv *server) GetVersion() string                  { return srv.Version }
func (srv *server) SetVersion(version string)           { srv.Version = version }
func (srv *server) GetPublisherID() string              { return srv.PublisherID }
func (srv *server) SetPublisherID(id string)            { srv.PublisherID = id }
func (srv *server) GetKeepUpToDate() bool               { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)            { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool                  { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)               { srv.KeepAlive = val }
func (srv *server) GetPermissions() []any               { return srv.Permissions }
func (srv *server) SetPermissions(permissions []any)    { srv.Permissions = permissions }

func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	return srv.Dependencies
}

func (srv *server) SetDependency(dependency string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}

func (srv *server) RolesDefault() []resourcepb.Role {
	return []resourcepb.Role{}
}

// Init initializes the service configuration, gRPC server, and job store.
func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}
	gs, err := globular.InitGrpcServer(srv)
	if err != nil {
		return err
	}
	srv.grpcServer = gs

	if srv.DataDir == "" {
		srv.DataDir = "/var/backups/globular"
	}

	// On a fresh install after a cluster wipe, restore previous backup settings
	// from <DataDir>/settings.json so the user doesn't have to re-enter everything.
	srv.restoreSettingsFromBackupDir()

	if srv.MaxConcurrentJobs < 1 {
		srv.MaxConcurrentJobs = 1
	}
	if srv.EtcdEndpoints == "" {
		srv.EtcdEndpoints = "127.0.0.1:2379"
	}
	if srv.EtcdCACert == "" {
		srv.EtcdCACert = "/var/lib/globular/pki/ca.pem"
	}
	// Prefer ca.crt if the configured path does not exist
	if !fileExists(srv.EtcdCACert) {
		dir := filepath.Dir(srv.EtcdCACert)
		alt := filepath.Join(dir, "ca.crt")
		if fileExists(alt) {
			srv.EtcdCACert = alt
		}
	}
	if srv.EtcdCert == "" {
		srv.EtcdCert = "/var/lib/globular/pki/issued/services/service.crt"
	}
	if srv.EtcdKey == "" {
		srv.EtcdKey = "/var/lib/globular/pki/issued/services/service.key"
	}
	if srv.ResticRepo == "" {
		srv.ResticRepo = "/var/backups/globular/restic"
	}
	if srv.ResticPassword == "" {
		srv.ResticPassword = "globular-backup"
	}
	if srv.ResticPaths == "" {
		srv.ResticPaths = "/var/lib/globular"
	}
	if len(srv.ClusterDefaultProviders) == 0 {
		srv.ClusterDefaultProviders = []string{"etcd", "scylla", "restic", "minio"}
	}
	// Auto-include scylla in ClusterDefaultProviders when ScyllaCluster is configured
	// but was previously missing from the list (e.g. saved before scylla was set up).
	if srv.ScyllaCluster != "" && !containsProvider(srv.ClusterDefaultProviders, "scylla") {
		srv.ClusterDefaultProviders = append(srv.ClusterDefaultProviders, "scylla")
	}
	if srv.RcloneSource == "" {
		srv.RcloneSource = "/var/lib/globular/minio/data"
	}
	if srv.ScyllaManagerAPI == "" {
		srv.ScyllaManagerAPI = "http://127.0.0.1:5080"
	}
	if srv.HookTimeoutSeconds <= 0 {
		srv.HookTimeoutSeconds = 30
	}
	if srv.MinioEndpoint == "" {
		srv.MinioEndpoint = "127.0.0.1:9000"
	}
	// Auto-load MinIO credentials from the Globular credentials file if not configured.
	if srv.MinioAccessKey == "" || srv.MinioSecretKey == "" {
		srv.tryLoadMinioCredentials()
	}
	if len(srv.Destinations) == 0 {
		srv.Destinations = []DestinationConfig{
			{Name: "local", Type: "local", Path: srv.DataDir, Primary: true},
		}
	}

	if err := os.MkdirAll(srv.DataDir, 0755); err != nil {
		return fmt.Errorf("create data dir %s: %w", srv.DataDir, err)
	}
	srv.store, err = newJobStore(srv.DataDir)
	if err != nil {
		return fmt.Errorf("init job store: %w", err)
	}

	srv.sem = make(chan struct{}, srv.MaxConcurrentJobs)
	srv.active = newRunningJobs()
	srv.tokens = newTokenStore()

	// Recover orphaned jobs that were running when the server last stopped.
	srv.recoverOrphanedJobs()

	// Auto-configure scylla-manager-agent S3 access if MinIO is configured.
	srv.ensureScyllaAgentS3Config()

	// Auto-register ScyllaDB in scylla-manager if not already done.
	// Runs in background so it doesn't block startup.
	go srv.ensureScyllaRegistered()

	// Start the backup scheduler (no-op if ScheduleInterval is empty/disabled).
	srv.stopScheduler = srv.startScheduler()

	// Recovery mode: if BACKUP_MANAGER_RECOVERY_MODE=true and a valid seed exists,
	// apply it to inject the recovery destination into runtime config.
	// This is the explicit entry point for Day 0 / bootstrap recovery.
	if os.Getenv("BACKUP_MANAGER_RECOVERY_MODE") == "true" {
		seed, seedErr := loadRecoverySeed()
		if seedErr != nil {
			slog.Warn("recovery mode enabled but no valid seed found", "error", seedErr)
		} else if !seedCredentialsAvailable(seed) {
			slog.Warn("recovery mode enabled but credentials not available", "creds_file", seed.CredsFile)
		} else {
			slog.Info("recovery mode: applying recovery seed", "destination", seed.Destination.Name)
			if _, applyErr := srv.ApplyRecoverySeed(context.Background(), &backup_managerpb.ApplyRecoverySeedRequest{Force: false}); applyErr != nil {
				slog.Warn("recovery mode: seed apply failed", "error", applyErr)
			} else {
				slog.Info("recovery mode: recovery destination applied successfully")
			}
		}
	}

	return nil
}

// recoverOrphanedJobs handles jobs left in running/queued state after a restart.
// Jobs with no BackupId are zombies (e.g. restored from a restic snapshot) and
// are deleted outright. Jobs with a BackupId are legitimate orphans from a crash
// and are marked as failed.
func (srv *server) recoverOrphanedJobs() {
	hadOrphans := false
	jobs, _, err := srv.store.ListJobs(backup_managerpb.BackupJobState_BACKUP_JOB_STATE_UNSPECIFIED, "", 0, 0)
	if err != nil {
		slog.Warn("failed to list jobs for recovery", "error", err)
		return
	}
	for _, job := range jobs {
		if job.State == backup_managerpb.BackupJobState_BACKUP_JOB_RUNNING ||
			job.State == backup_managerpb.BackupJobState_BACKUP_JOB_QUEUED {
			hadOrphans = true

			// Jobs with no BackupId never completed — they are zombies
			// (likely restored from an old restic snapshot). Delete them.
			if job.BackupId == "" {
				if err := srv.store.DeleteJob(job.JobId); err != nil {
					slog.Warn("failed to delete zombie job", "job_id", job.JobId, "error", err)
				} else {
					slog.Info("deleted zombie job (no backup_id)", "job_id", job.JobId)
				}
				continue
			}

			job.State = backup_managerpb.BackupJobState_BACKUP_JOB_FAILED
			job.Message = "server restarted while job was in progress"
			job.FinishedUnixMs = time.Now().UnixMilli()
			if err := srv.store.SaveJob(job); err != nil {
				slog.Warn("failed to recover orphaned job", "job_id", job.JobId, "error", err)
			} else {
				slog.Info("recovered orphaned job", "job_id", job.JobId)
			}
		}
	}

	// If there were orphaned jobs, force-release the cluster lock
	// in case it's still held by a stale etcd lease.
	if hadOrphans {
		srv.forceReleaseClusterLock()
	}
}

// forceReleaseClusterLock deletes the etcd cluster lock key to clear stale locks.
func (srv *server) forceReleaseClusterLock() {
	cli, err := srv.etcdClient()
	if err != nil {
		slog.Warn("cannot connect to etcd to release stale lock", "error", err)
		return
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Delete all keys under the lock prefix to clear any stale mutex entries
	resp, err := cli.Delete(ctx, clusterLockKey, clientv3.WithPrefix())
	if err != nil {
		slog.Warn("failed to delete stale cluster lock", "error", err)
	} else if resp.Deleted > 0 {
		slog.Info("released stale cluster lock", "keys_deleted", resp.Deleted)
	}
}

func (srv *server) Save() error {
	// Validate recovery destination constraints
	authCount := 0
	for _, d := range srv.Destinations {
		if d.AuthoritativeForRecovery {
			authCount++
			if d.Type == "local" {
				return fmt.Errorf("local destinations cannot be marked as recovery source for full reinstall recovery; configure an external durable destination")
			}
		}
	}
	if authCount > 1 {
		return fmt.Errorf("at most one destination may be marked AuthoritativeForRecovery (found %d)", authCount)
	}

	err := globular.SaveService(srv)
	if err == nil {
		srv.updateRecoverySeedOnConfigSave()
		srv.saveSettingsToBackupDir()
	}
	return err
}

// savedSettings is the subset of backup-manager config that matters for
// disaster recovery.  Written to <DataDir>/settings.json on every Save()
// so that after a cluster wipe + reinstall the user can recover previous
// backup settings without remembering all the values.
type savedSettings struct {
	SavedAt            string              `json:"saved_at"`
	Destinations       []DestinationConfig `json:"Destinations"`
	ResticRepo         string              `json:"ResticRepo"`
	ResticPassword     string              `json:"ResticPassword"`
	ResticPaths        string              `json:"ResticPaths"`
	RcloneRemote       string              `json:"RcloneRemote"`
	RcloneSource       string              `json:"RcloneSource"`
	ScyllaManagerAPI   string              `json:"ScyllaManagerAPI"`
	ScyllaCluster      string              `json:"ScyllaCluster"`
	ScyllaLocation     string              `json:"ScyllaLocation"`
	ScheduleInterval   string              `json:"ScheduleInterval"`
	MinioEndpoint      string              `json:"MinioEndpoint"`
	MinioAccessKey     string              `json:"MinioAccessKey"`
	MinioSecretKey     string              `json:"MinioSecretKey"`
	MinioSecure        bool                `json:"MinioSecure"`
	RetentionKeepLastN int                 `json:"RetentionKeepLastN"`
	RetentionKeepDays  int                 `json:"RetentionKeepDays"`
	ClusterDefaultProviders []string       `json:"ClusterDefaultProviders"`
}

const savedSettingsFile = "settings.json"

func (srv *server) saveSettingsToBackupDir() {
	if srv.DataDir == "" {
		return
	}
	ss := savedSettings{
		SavedAt:            time.Now().UTC().Format(time.RFC3339),
		Destinations:       srv.Destinations,
		ResticRepo:         srv.ResticRepo,
		ResticPassword:     srv.ResticPassword,
		ResticPaths:        srv.ResticPaths,
		RcloneRemote:       srv.RcloneRemote,
		RcloneSource:       srv.RcloneSource,
		ScyllaManagerAPI:   srv.ScyllaManagerAPI,
		ScyllaCluster:      srv.ScyllaCluster,
		ScyllaLocation:     srv.ScyllaLocation,
		ScheduleInterval:   srv.ScheduleInterval,
		MinioEndpoint:      srv.MinioEndpoint,
		MinioAccessKey:     srv.MinioAccessKey,
		MinioSecretKey:     srv.MinioSecretKey,
		MinioSecure:        srv.MinioSecure,
		RetentionKeepLastN: srv.RetentionKeepLastN,
		RetentionKeepDays:  srv.RetentionKeepDays,
		ClusterDefaultProviders: srv.ClusterDefaultProviders,
	}
	data, err := json.MarshalIndent(ss, "", "  ")
	if err != nil {
		slog.Warn("failed to marshal backup settings", "error", err)
		return
	}
	path := filepath.Join(srv.DataDir, savedSettingsFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0640); err != nil {
		slog.Warn("failed to write backup settings", "path", tmp, "error", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		slog.Warn("failed to rename backup settings", "error", err)
		return
	}
	slog.Info("backup settings saved to backup dir", "path", path)
}

// restoreSettingsFromBackupDir loads previously-saved settings from
// <DataDir>/settings.json on startup.  Only applied when the current
// config looks like a fresh default (no MinIO keys, no scylla cluster).
func (srv *server) restoreSettingsFromBackupDir() {
	if srv.DataDir == "" {
		return
	}
	path := filepath.Join(srv.DataDir, savedSettingsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return // no saved settings — normal on first install
	}
	var ss savedSettings
	if err := json.Unmarshal(data, &ss); err != nil {
		slog.Warn("failed to parse saved backup settings", "path", path, "error", err)
		return
	}

	// Only apply if current config looks like a fresh default
	if srv.MinioAccessKey != "" || srv.ScyllaCluster != "" {
		slog.Debug("backup settings file exists but current config is already customized, skipping restore")
		return
	}

	slog.Info("restoring backup settings from previous installation", "path", path, "saved_at", ss.SavedAt)

	if len(ss.Destinations) > 0 {
		srv.Destinations = ss.Destinations
	}
	if ss.ResticRepo != "" {
		srv.ResticRepo = ss.ResticRepo
	}
	if ss.ResticPassword != "" {
		srv.ResticPassword = ss.ResticPassword
	}
	if ss.ResticPaths != "" {
		srv.ResticPaths = ss.ResticPaths
	}
	srv.RcloneRemote = ss.RcloneRemote
	srv.RcloneSource = ss.RcloneSource
	if ss.ScyllaManagerAPI != "" {
		srv.ScyllaManagerAPI = ss.ScyllaManagerAPI
	}
	srv.ScyllaCluster = ss.ScyllaCluster
	srv.ScyllaLocation = ss.ScyllaLocation
	if ss.ScheduleInterval != "" {
		srv.ScheduleInterval = ss.ScheduleInterval
	}
	if ss.MinioEndpoint != "" {
		srv.MinioEndpoint = ss.MinioEndpoint
	}
	srv.MinioAccessKey = ss.MinioAccessKey
	srv.MinioSecretKey = ss.MinioSecretKey
	srv.MinioSecure = ss.MinioSecure
	if ss.RetentionKeepLastN > 0 {
		srv.RetentionKeepLastN = ss.RetentionKeepLastN
	}
	if ss.RetentionKeepDays > 0 {
		srv.RetentionKeepDays = ss.RetentionKeepDays
	}
	if len(ss.ClusterDefaultProviders) > 0 {
		srv.ClusterDefaultProviders = ss.ClusterDefaultProviders
	}
}

func (srv *server) Stop(ctx context.Context, _ *backup_managerpb.StopRequest) (*backup_managerpb.StopResponse, error) {
	return &backup_managerpb.StopResponse{}, srv.StopService()
}

func (srv *server) StartService() error {
	return globular.StartService(srv, srv.grpcServer)
}

func (srv *server) StopService() error {
	if srv.stopScheduler != nil {
		srv.stopScheduler()
	}
	return globular.StopService(srv, srv.grpcServer)
}

// --- Version / Build ---

var (
	Version   = "0.0.1"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

func initializeServerDefaults() *server {
	srv := new(server)

	srv.Name = string(backup_managerpb.File_backup_manager_proto.Services().Get(0).FullName())
	srv.Proto = backup_managerpb.File_backup_manager_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Version = Version
	srv.PublisherID = "localhost"
	srv.Description = "Backup orchestrator for Globular cluster components"
	srv.Keywords = []string{"backup", "restore", "snapshot", "disaster-recovery"}

	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"

	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsStr

	srv.KeepAlive = true
	srv.KeepUpToDate = true
	srv.Process = -1
	srv.ProxyProcess = -1

	srv.Repositories = make([]string, 0)
	srv.Discoveries = make([]string, 0)
	srv.Dependencies = make([]string, 0)
	srv.Permissions = make([]any, 0)

	srv.DataDir = "/var/backups/globular"
	srv.MaxConcurrentJobs = 1
	srv.AllowResticPruneOnDelete = false
	srv.AllowRemoteDelete = true
	srv.ProviderTimeoutSeconds = 1800
	srv.Destinations = []DestinationConfig{
		{Name: "local", Type: "local", Path: "/var/backups/globular", Primary: true},
	}

	srv.EtcdEndpoints = "127.0.0.1:2379"
	srv.EtcdCACert = "/var/lib/globular/pki/ca.pem"
	srv.EtcdCert = "/var/lib/globular/pki/issued/services/service.crt"
	srv.EtcdKey = "/var/lib/globular/pki/issued/services/service.key"

	srv.ResticRepo = "/var/backups/globular/restic"
	srv.ResticPassword = "globular-backup"
	srv.ResticPaths = "/var/lib/globular"

	srv.RcloneRemote = ""
	srv.RcloneSource = "/var/lib/globular/minio/data"

	srv.ScyllaManagerAPI = "http://127.0.0.1:5080"
	srv.ScyllaCluster = ""
	srv.ScyllaLocation = ""

	srv.ClusterDefaultProviders = []string{"etcd", "scylla", "restic", "minio"}

	srv.MinioEndpoint = "127.0.0.1:9000"
	srv.MinioSecure = true

	return srv
}

func setupGrpcService(srv *server) {
	backup_managerpb.RegisterBackupManagerServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)
}

func main() {
	srv := initializeServerDefaults()

	var (
		enableDebug  = flag.Bool("debug", false, "enable debug logging")
		showVersion  = flag.Bool("version", false, "print version information as JSON and exit")
		showHelp     = flag.Bool("help", false, "show usage information and exit")
		showDescribe = flag.Bool("describe", false, "print service description as JSON and exit")
		showHealth   = flag.Bool("health", false, "print service health status as JSON and exit")
	)

	flag.Usage = printUsage
	flag.Parse()

	if *enableDebug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		logger.Debug("debug logging enabled")
	}

	if *showHelp {
		printUsage()
		return
	}

	if *showVersion {
		printVersion()
		return
	}

	if *showDescribe {
		globular.HandleDescribeFlag(srv, logger)
		return
	}

	if *showHealth {
		health := map[string]interface{}{
			"service": srv.Name,
			"status":  "healthy",
			"version": srv.Version,
		}
		data, _ := json.MarshalIndent(health, "", "  ")
		fmt.Println(string(data))
		return
	}

	args := flag.Args()
	if err := globular.AllocatePortIfNeeded(srv, args); err != nil {
		logger.Error("port allocation failed", "error", err)
		os.Exit(1)
	}

	globular.ParsePositionalArgs(srv, args)
	globular.LoadRuntimeConfig(srv)

	logger.Info("starting backup manager service", "service", srv.Name, "version", srv.Version, "domain", srv.Domain)

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
	logger.Info("service initialized", "duration_ms", time.Since(start).Milliseconds())

	setupGrpcService(srv)
	logger.Debug("gRPC handlers registered")

	logger.Info("service ready", "service", srv.Name, "version", srv.Version, "port", srv.Port, "domain", srv.Domain, "startup_ms", time.Since(start).Milliseconds())

	lm := globular.NewLifecycleManager(srv, logger)
	if err := lm.Start(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Backup Manager Service - Cluster backup orchestration")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  backup_manager_server [OPTIONS] [id] [config_path]")
	fmt.Println()
	fmt.Println("OPTIONS:")
	fmt.Println("  --debug       Enable debug logging")
	fmt.Println("  --describe    Print service description as JSON and exit")
	fmt.Println("  --health      Print service health status as JSON and exit")
	fmt.Println("  --version     Print version information as JSON and exit")
	fmt.Println("  --help        Show this help message and exit")
}

func printVersion() {
	info := map[string]string{
		"service":    "backup_manager",
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(data))
}
