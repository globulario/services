package collector

// repository_finding.go — doctor-internal mirror of repopb.RepositoryFinding.
//
// The cluster_doctor package historically avoids importing
// repository/repositorypb directly to keep its compile graph small. The
// collector dials the repository service via raw gRPC (or via an injected
// fetch function) and translates each repopb.RepositoryFinding into the
// struct below. The "repository.*" invariant rules consume these.

// RepositoryFindingSnapshot is the doctor-side, package-internal view of a
// single repository self-reported finding.
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
