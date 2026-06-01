// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.collector
// @awareness file_role=repository_findings_collector
// @awareness implements=globular.platform:intent.repository.identity_doctor_reports_collisions
// @awareness risk=high
package collector

// repository_finding.go — doctor-internal mirrors of repository service types.
//
// The collector imports repository/repositorypb to call the service RPCs, but
// translates the responses into the plain structs below before storing them in
// the Snapshot. The "repository.*" invariant rules consume only these structs,
// keeping the rules package free from proto imports.

// RepositoryFindingSnapshot is the doctor-side view of a single repository
// self-reported finding (from ListRepositoryFindings).
type RepositoryFindingSnapshot struct {
	Kind               string            // e.g. "REPO_FIND_PUBLISHED_MISSING_BLOB"
	Severity           string            // "INFO" | "WARN" | "ERROR" | "CRITICAL"
	ArtifactKey        string
	PublisherID        string
	Name               string
	Version            string
	Platform           string
	NodeID             string
	CurrentState       string
	ExpectedState      string
	Reason             string
	RecommendedCommand string
	Evidence           map[string]string
	ObservedAtUnix     int64
}

// RepositoryOperationalStatus is the doctor-side view of the repository
// service's live operational mode (from GetRepositoryStatus). Nil when the
// RPC was never attempted; ReachError non-nil when the RPC failed.
type RepositoryOperationalStatus struct {
	Service        string
	Mode           string // "FULL" | "DEGRADED" | "READ_ONLY" | "LOCAL_ONLY" | "UNAVAILABLE"
	Reason         string
	Dependencies   []RepoDependencyHealth
	Capabilities   []RepoCapabilityHealth
	ObservedAtUnix int64
	ReachError     error // non-nil when GetRepositoryStatus itself failed
}

// RepoDependencyHealth is the doctor-side view of a single repository
// dependency's live state.
type RepoDependencyHealth struct {
	Name                string
	Kind                string
	Status              string // "HEALTHY" | "DEGRADED" | "UNAVAILABLE" | ...
	Reason              string
	AffectsCapabilities []string
}

// RepoCapabilityHealth is the doctor-side view of one repository capability.
type RepoCapabilityHealth struct {
	Name   string
	Status string // "AVAILABLE" | "DEGRADED" | "BLOCKED" | "UNKNOWN"
	Mode   string
	Reason string
}
