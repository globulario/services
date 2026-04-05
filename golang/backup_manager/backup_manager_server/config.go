package main

import (
	"github.com/globulario/services/golang/globular_service"
)

// Config holds the Backup Manager service configuration.
type Config struct {
	ID          string   `json:"Id"`
	Name        string   `json:"Name"`
	Domain      string   `json:"Domain"`
	Address     string   `json:"Address"`
	Description string   `json:"Description"`
	Version     string   `json:"Version"`
	PublisherID string   `json:"PublisherId"`
	Keywords    []string `json:"Keywords"`

	Port     int    `json:"Port"`
	Proxy    int    `json:"Proxy"`
	Protocol string `json:"Protocol"`

	Repositories []string `json:"Repositories"`
	Discoveries  []string `json:"Discoveries"`
	Dependencies []string `json:"Dependencies"`

	AllowAllOrigins bool   `json:"AllowAllOrigins"`
	AllowedOrigins  string `json:"AllowedOrigins"`

	KeepAlive    bool `json:"KeepAlive"`
	KeepUpToDate bool `json:"KeepUpToDate"`

	TLS struct {
		Enabled            bool   `json:"TLS"`
		CertFile           string `json:"CertFile"`
		KeyFile            string `json:"KeyFile"`
		CertAuthorityTrust string `json:"CertAuthorityTrust"`
	} `json:"TLS"`

	ConfigPath string `json:"ConfigPath"`
	Plaform    string `json:"Plaform"` // typo preserved for compatibility

	// Backup Manager specific
	DataDir           string              `json:"DataDir"`
	MaxConcurrentJobs int                 `json:"MaxConcurrentJobs"`
	Destinations      []DestinationConfig `json:"Destinations"` // where to store backups (replicate to all)

	// etcd provider
	EtcdEndpoints string `json:"EtcdEndpoints"` // comma-separated, default "127.0.0.1:2379"
	EtcdCACert    string `json:"EtcdCACert"`
	EtcdCert      string `json:"EtcdCert"`
	EtcdKey       string `json:"EtcdKey"`

	// Capsule compression
	CompressCapsule bool `json:"CompressCapsule"` // if true, tar.gz capsule before replication

	// Deletion safety
	AllowResticPruneOnDelete bool `json:"AllowResticPruneOnDelete"` // allow restic forget/prune on delete (default false)
	AllowRemoteDelete        bool `json:"AllowRemoteDelete"`        // allow deletion of replicated copies (default true)

	// Retention policy
	RetentionKeepLastN    int    `json:"RetentionKeepLastN"`    // keep at most N recent backups (0 = unlimited)
	RetentionKeepDays     int    `json:"RetentionKeepDays"`     // keep backups from last N days (0 = unlimited)
	RetentionMaxTotalBytes uint64 `json:"RetentionMaxTotalBytes"` // max total backup size in bytes (0 = unlimited)

	// Retention: minimum number of restore-tested backups to keep
	MinRestoreTestedToKeep int `json:"MinRestoreTestedToKeep"`

	// Cluster defaults
	ClusterDefaultProviders []string         `json:"ClusterDefaultProviders"` // default providers for CLUSTER mode
	ClusterProviderOrder    []string         `json:"ClusterProviderOrder"`    // custom execution order (optional, overrides hardcoded)
	ClusterStrictDefaults   bool             `json:"ClusterStrictDefaults"`   // if true, error when a default provider is unavailable; false = SKIP
	HookTargets             []HookTargetConfig `json:"HookTargets"`           // services that support backup hooks
	HookTimeoutSeconds      int              `json:"HookTimeoutSeconds"`      // per-hook timeout (default 30)
	HookStrict              bool             `json:"HookStrict"`              // abort on prepare hook failure
	HookDiscovery           bool             `json:"HookDiscovery"`           // auto-discover hook targets from etcd registry
	HookAllowInsecureFallback bool           `json:"HookAllowInsecureFallback"` // allow plaintext fallback when TLS config fails

	// Provider timeout (seconds, 0 = no timeout beyond context)
	ProviderTimeoutSeconds int `json:"ProviderTimeoutSeconds"`

	// restic provider
	ResticRepo     string `json:"ResticRepo"`     // e.g. "/var/backups/globular/restic" or "s3:..."
	ResticPassword string `json:"ResticPassword"` // repository password
	ResticPaths    string `json:"ResticPaths"`     // comma-separated paths to back up

	// minio/rclone provider
	RcloneRemote string `json:"RcloneRemote"` // rclone remote:path for destination
	RcloneSource string `json:"RcloneSource"` // local path to sync, default minio data dir

	// scylla provider
	ScyllaManagerAPI string `json:"ScyllaManagerAPI"` // scylla-manager API, default "http://127.0.0.1:5080"
	ScyllaCluster    string `json:"ScyllaCluster"`    // cluster name in scylla-manager
	ScyllaLocation   string `json:"ScyllaLocation"`   // backup location, e.g. "s3:scylla-backups"

	// Scheduled backups
	ScheduleInterval string `json:"ScheduleInterval"` // e.g. "6h", "24h", "daily", "weekly", "0"=disabled

	// MinIO connection (shared by minio provider + bucket management)
	MinioEndpoint  string `json:"MinioEndpoint"`  // DNS name served by cluster DNS (minio.<domain>:9000)
	MinioAccessKey string `json:"MinioAccessKey"` // access key
	MinioSecretKey string `json:"MinioSecretKey"` // secret key
	MinioSecure    bool   `json:"MinioSecure"`    // use HTTPS
}

// DestinationConfig defines a storage location for backup artifacts.
//
//	Type: "local", "minio", "nfs", "s3", "rclone"
//	Path: depends on type:
//	  local/nfs: filesystem path (e.g. "/mnt/backups")
//	  minio:     bucket/prefix (e.g. "globular-backups/cluster-01")
//	  s3:        bucket/prefix (e.g. "my-bucket/backups")
//	  rclone:    remote:path (e.g. "myremote:backups/cluster-01")
//	Options: type-specific (endpoint, access_key, secret_key, region, etc.)
// HookTargetConfig defines a service that supports backup hooks (PrepareBackup/FinalizeBackup).
type HookTargetConfig struct {
	Name    string `json:"Name"`    // service name
	Address string `json:"Address"` // gRPC address (host:port)
}

type DestinationConfig struct {
	Name                     string            `json:"Name"`
	Type                     string            `json:"Type"`    // local, minio, nfs, s3, rclone
	Path                     string            `json:"Path"`
	Options                  map[string]string `json:"Options"` // endpoint, access_key, secret_key, region, etc.
	Primary                  bool              `json:"Primary"` // if true, this is the primary storage location
	AuthoritativeForRecovery bool              `json:"AuthoritativeForRecovery,omitempty"` // if true, recovery seed is written from this destination
}

func DefaultConfig() *Config {
	cfg := &Config{
		Name:        "backup_manager.BackupManagerService",
		Port:        defaultPort,
		Proxy:       defaultProxy,
		Protocol:    "grpc",
		Version:     "0.0.1",
		PublisherID: "localhost",
		Description: "Backup orchestrator for Globular cluster components",
		Keywords:    []string{"backup", "restore", "snapshot", "disaster-recovery"},

		Repositories: make([]string, 0),
		Discoveries:  make([]string, 0),
		Dependencies: make([]string, 0),

		AllowAllOrigins: allowAllOrigins,
		AllowedOrigins:  allowedOriginsStr,
		KeepAlive:       true,
		KeepUpToDate:    true,

		DataDir:           "/var/backups/globular",
		MaxConcurrentJobs: 1,
		AllowResticPruneOnDelete: false,
		AllowRemoteDelete:        true,
		ProviderTimeoutSeconds:   1800, // 30 minutes default
		Destinations: []DestinationConfig{
			{
				Name:    "local",
				Type:    "local",
				Path:    "/var/backups/globular",
				Primary: true,
			},
		},

		EtcdEndpoints: "127.0.0.1:2379",
		EtcdCACert:    "/var/lib/globular/pki/ca.pem",
		EtcdCert:      "/var/lib/globular/pki/issued/services/service.crt",
		EtcdKey:       "/var/lib/globular/pki/issued/services/service.key",

		ResticRepo:     "/var/backups/globular/restic",
		ResticPassword: "globular-backup",
		ResticPaths:    "/var/lib/globular",

		RcloneRemote: "",
		RcloneSource: "/var/lib/globular/minio/data",

		ScyllaManagerAPI: "http://127.0.0.1:5080",
		ScyllaCluster:    "",
		ScyllaLocation:   "",

		ScheduleInterval: "daily",

		// Populated from etcd at runtime — do not hardcode addresses.
		MinioEndpoint:  "",
		MinioAccessKey: "",
		MinioSecretKey: "",
		MinioSecure:    true,
	}

	cfg.Domain, cfg.Address = globular_service.GetDefaultDomainAddress(cfg.Port)
	return cfg
}

func (c *Config) Validate() error {
	return globular_service.ValidateCommonFields(c.Name, c.Port, c.Proxy, c.Protocol, c.Version)
}
