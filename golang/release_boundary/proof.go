package release_boundary

// proof.go — the pure release-boundary verdict engine (PR-16 Phase 1).
//
// Evaluate() takes the four already-fetched truth structs for one service on
// one node and returns a Report whose overall Verdict is computed
// conservatively from five first-class assertions (A0..A4). It performs no
// I/O of any kind — see doc.go for the governance rationale.

// Verdict is the outcome of an assertion or of the overall report.
type Verdict string

const (
	// VerdictProven means every required assertion passed against reachable
	// evidence.
	VerdictProven Verdict = "PROVEN"
	// VerdictFailed means a reachable truth source proved a mismatch or an
	// invalid state.
	VerdictFailed Verdict = "FAILED"
	// VerdictIndeterminate means a required truth source was missing,
	// unreachable, ambiguous, or incomplete. A tool/source failure is
	// evidence, not silence — it can never be upgraded to PROVEN.
	VerdictIndeterminate Verdict = "INDETERMINATE"
	// VerdictNotApplicable is reserved for wrapper / unhashable packages that
	// are intentionally unverifiable. It is never FAILED.
	VerdictNotApplicable Verdict = "NOT_APPLICABLE"
)

// AssertionID identifies one link in the release-boundary chain.
type AssertionID string

const (
	// AssertionRepositoryArtifactIntact (A0): the repository artifact is intact.
	AssertionRepositoryArtifactIntact AssertionID = "A0"
	// AssertionDesiredPublished (A1): desired binds the expected build and the
	// manifest is PUBLISHED.
	AssertionDesiredPublished AssertionID = "A1"
	// AssertionInstalledMatches (A2): installed equals published.
	AssertionInstalledMatches AssertionID = "A2"
	// AssertionRuntimeMatches (A3): the running executable equals published.
	AssertionRuntimeMatches AssertionID = "A3"
	// AssertionRestartAfterInstall (A4): the running process started after the
	// artifact was installed.
	AssertionRestartAfterInstall AssertionID = "A4"
)

const publishStatePublished = "PUBLISHED"

// AssertionReport is the structured outcome of a single assertion.
type AssertionReport struct {
	ID       AssertionID
	Name     string
	Verdict  Verdict
	Reason   string
	Evidence map[string]string
}

// Report is the full release-boundary verdict for one service on one node.
type Report struct {
	ServiceName string
	NodeName    string
	BuildID     string
	Checksum    string
	Verdict     Verdict
	Assertions  []AssertionReport
}

// RepositoryEvidence represents the result of repository artifact
// verification (the would-be output of VerifyArtifact). A nil pointer or
// Present=false both mean "no usable evidence" -> INDETERMINATE.
type RepositoryEvidence struct {
	Present  bool
	Verified bool
	Reason   string
}

// ManifestEvidence represents the published artifact manifest for build B.
type ManifestEvidence struct {
	BuildID            string
	PublishState       string
	EntrypointChecksum string
	ProvenanceGitSHA   string
}

// InstalledEvidence represents the node's installed-package record.
type InstalledEvidence struct {
	BuildID            string
	EntrypointChecksum string
	InstalledUnix      int64
}

// RuntimeEvidence represents the node's runtime proof for the live process.
type RuntimeEvidence struct {
	Running          bool
	PID              int
	RunningExeSHA256 string
	ProcessStartUnix int64
}

// Inputs is the complete set of already-fetched truth for one (service, node).
// Pointer fields are nil when that truth source was never collected; presence
// booleans inside the evidence structs distinguish "collected but absent" from
// "collected and present".
type Inputs struct {
	ServiceName string
	NodeName    string

	// DesiredBuildID is the build the desired state binds this service to (B).
	DesiredBuildID string

	Manifest   *ManifestEvidence
	Repository *RepositoryEvidence
	Installed  *InstalledEvidence
	Runtime    *RuntimeEvidence

	// PackageKind and Unhashable identify wrapper / unverifiable packages.
	PackageKind string
	Unhashable  bool
}

// isWrapper reports whether the package is intentionally unverifiable
// (wrapper / bin-noop entrypoint). Such packages are NOT_APPLICABLE, never
// FAILED — they have no real binary to hash.
func (in Inputs) isWrapper() bool {
	if in.Unhashable {
		return true
	}
	switch in.PackageKind {
	case "wrapper", "bin/noop", "noop":
		return true
	}
	return false
}

// Evaluate computes the release-boundary Report for one service on one node.
// It is pure: the verdict is a deterministic function of inputs only.
func Evaluate(in Inputs) Report {
	b := in.DesiredBuildID
	ec := ""
	if in.Manifest != nil {
		ec = in.Manifest.EntrypointChecksum
	}

	report := Report{
		ServiceName: in.ServiceName,
		NodeName:    in.NodeName,
		BuildID:     b,
		Checksum:    ec,
	}

	// Wrapper / unhashable packages short-circuit: they are intentionally
	// unverifiable, so we neither evaluate A0..A4 nor pretend evidence exists.
	if in.isWrapper() {
		report.Verdict = VerdictNotApplicable
		report.Assertions = wrapperAssertions()
		return report
	}

	// Evaluate every assertion from supplied evidence — never short-circuit on
	// an early failure, so the report always exposes the full boundary state.
	report.Assertions = []AssertionReport{
		evalA0(in),
		evalA1(in, b),
		evalA2(in, b, ec),
		evalA3(in, ec),
		evalA4(in),
	}
	report.Verdict = aggregate(report.Assertions)
	return report
}

// aggregate combines assertion verdicts conservatively. Order matters: a
// single FAILED dominates INDETERMINATE, which in turn blocks PROVEN.
func aggregate(assertions []AssertionReport) Verdict {
	allProven := true
	anyIndeterminate := false
	for _, a := range assertions {
		switch a.Verdict {
		case VerdictFailed:
			return VerdictFailed
		case VerdictIndeterminate:
			anyIndeterminate = true
			allProven = false
		case VerdictProven:
			// keep scanning
		default:
			allProven = false
		}
	}
	if anyIndeterminate {
		return VerdictIndeterminate
	}
	if allProven {
		return VerdictProven
	}
	return VerdictIndeterminate
}

// evalA0 — repository artifact is intact.
func evalA0(in Inputs) AssertionReport {
	r := AssertionReport{
		ID:       AssertionRepositoryArtifactIntact,
		Name:     "repository artifact intact",
		Evidence: map[string]string{},
	}
	if in.Repository == nil || !in.Repository.Present {
		r.Verdict = VerdictIndeterminate
		r.Reason = "repository verification evidence missing"
		return r
	}
	r.Evidence["verified"] = boolStr(in.Repository.Verified)
	if in.Repository.Reason != "" {
		r.Evidence["repository_reason"] = in.Repository.Reason
	}
	if in.Repository.Verified {
		r.Verdict = VerdictProven
		r.Reason = "repository verification reports the artifact intact"
		return r
	}
	r.Verdict = VerdictFailed
	r.Reason = "repository verification reports the artifact broken"
	if in.Repository.Reason != "" {
		r.Reason += ": " + in.Repository.Reason
	}
	return r
}

// evalA1 — desired binds the expected build and the manifest is PUBLISHED.
func evalA1(in Inputs, b string) AssertionReport {
	r := AssertionReport{
		ID:       AssertionDesiredPublished,
		Name:     "desired binds published build",
		Evidence: map[string]string{},
	}
	if b == "" {
		r.Verdict = VerdictIndeterminate
		r.Reason = "desired-state evidence missing (no desired build_id)"
		return r
	}
	if in.Manifest == nil {
		r.Verdict = VerdictIndeterminate
		r.Reason = "artifact manifest evidence missing"
		return r
	}
	r.Evidence["desired_build_id"] = b
	r.Evidence["manifest_build_id"] = in.Manifest.BuildID
	r.Evidence["publish_state"] = in.Manifest.PublishState
	if in.Manifest.BuildID != b {
		r.Verdict = VerdictFailed
		r.Reason = "manifest build_id does not match desired build_id"
		return r
	}
	if in.Manifest.PublishState != publishStatePublished {
		r.Verdict = VerdictFailed
		r.Reason = "manifest publish_state is not PUBLISHED"
		return r
	}
	r.Verdict = VerdictProven
	r.Reason = "desired build is bound and the manifest is PUBLISHED"
	return r
}

// evalA2 — installed equals published.
func evalA2(in Inputs, b, ec string) AssertionReport {
	r := AssertionReport{
		ID:       AssertionInstalledMatches,
		Name:     "installed equals published",
		Evidence: map[string]string{},
	}
	if in.Installed == nil {
		r.Verdict = VerdictIndeterminate
		r.Reason = "installed-package evidence missing"
		return r
	}
	r.Evidence["installed_build_id"] = in.Installed.BuildID
	r.Evidence["installed_checksum"] = in.Installed.EntrypointChecksum
	r.Evidence["expected_checksum"] = ec

	// A build_id mismatch is provable regardless of whether we know EC.
	if b != "" && in.Installed.BuildID != b {
		r.Verdict = VerdictFailed
		r.Reason = "installed build_id does not match desired build_id"
		return r
	}
	if ec == "" {
		r.Verdict = VerdictIndeterminate
		r.Reason = "expected entrypoint_checksum unknown (manifest evidence absent)"
		return r
	}
	if in.Installed.EntrypointChecksum == "" {
		r.Verdict = VerdictIndeterminate
		r.Reason = "installed entrypoint_checksum metadata absent"
		return r
	}
	if in.Installed.EntrypointChecksum != ec {
		r.Verdict = VerdictFailed
		r.Reason = "installed entrypoint_checksum does not match manifest"
		return r
	}
	r.Verdict = VerdictProven
	r.Reason = "installed build and checksum match the published artifact"
	return r
}

// evalA3 — the running executable equals published.
func evalA3(in Inputs, ec string) AssertionReport {
	r := AssertionReport{
		ID:       AssertionRuntimeMatches,
		Name:     "runtime equals published",
		Evidence: map[string]string{},
	}
	if in.Runtime == nil {
		r.Verdict = VerdictIndeterminate
		r.Reason = "runtime proof missing"
		return r
	}
	if !in.Runtime.Running {
		r.Verdict = VerdictIndeterminate
		r.Reason = "process is not running"
		return r
	}
	if in.Runtime.RunningExeSHA256 == "" {
		r.Verdict = VerdictIndeterminate
		r.Reason = "runtime executable checksum unavailable"
		return r
	}
	r.Evidence["running_exe_sha256"] = in.Runtime.RunningExeSHA256
	r.Evidence["expected_checksum"] = ec
	if ec == "" {
		r.Verdict = VerdictIndeterminate
		r.Reason = "expected entrypoint_checksum unknown (manifest evidence absent)"
		return r
	}
	if in.Runtime.RunningExeSHA256 != ec {
		r.Verdict = VerdictFailed
		r.Reason = "running executable checksum does not match the published artifact"
		return r
	}
	r.Verdict = VerdictProven
	r.Reason = "running executable matches the published artifact"
	return r
}

// evalA4 — the running process started after the artifact was installed.
//
// Conservative rule: process_start_unix <= installed_unix never proves.
// Strictly older -> FAILED; equal (or any ambiguous tie) -> INDETERMINATE.
func evalA4(in Inputs) AssertionReport {
	r := AssertionReport{
		ID:       AssertionRestartAfterInstall,
		Name:     "restart after install",
		Evidence: map[string]string{},
	}
	if in.Runtime == nil || !in.Runtime.Running || in.Runtime.ProcessStartUnix == 0 {
		r.Verdict = VerdictIndeterminate
		r.Reason = "process start time unavailable"
		return r
	}
	if in.Installed == nil || in.Installed.InstalledUnix == 0 {
		r.Verdict = VerdictIndeterminate
		r.Reason = "installed time unavailable"
		return r
	}
	r.Evidence["process_start_unix"] = int64Str(in.Runtime.ProcessStartUnix)
	r.Evidence["installed_unix"] = int64Str(in.Installed.InstalledUnix)
	switch {
	case in.Runtime.ProcessStartUnix < in.Installed.InstalledUnix:
		r.Verdict = VerdictFailed
		r.Reason = "process started before the artifact was installed (stale process)"
	case in.Runtime.ProcessStartUnix == in.Installed.InstalledUnix:
		r.Verdict = VerdictIndeterminate
		r.Reason = "process start time ties installed time at ambiguous precision"
	default:
		r.Verdict = VerdictProven
		r.Reason = "process started after the artifact was installed"
	}
	return r
}

// wrapperAssertions returns the five assertions marked NOT_APPLICABLE for a
// wrapper / unhashable package, preserving the report shape without inventing
// evidence.
func wrapperAssertions() []AssertionReport {
	const reason = "wrapper package is intentionally unverifiable (no hashable binary)"
	ids := []struct {
		id   AssertionID
		name string
	}{
		{AssertionRepositoryArtifactIntact, "repository artifact intact"},
		{AssertionDesiredPublished, "desired binds published build"},
		{AssertionInstalledMatches, "installed equals published"},
		{AssertionRuntimeMatches, "runtime equals published"},
		{AssertionRestartAfterInstall, "restart after install"},
	}
	out := make([]AssertionReport, 0, len(ids))
	for _, x := range ids {
		out = append(out, AssertionReport{
			ID:      x.id,
			Name:    x.name,
			Verdict: VerdictNotApplicable,
			Reason:  reason,
		})
	}
	return out
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func int64Str(v int64) string {
	// Small dependency-free int64 -> decimal string.
	if v == 0 {
		return "0"
	}
	neg := v < 0
	var buf [20]byte
	i := len(buf)
	u := v
	if neg {
		u = -u
	}
	for u > 0 {
		i--
		buf[i] = byte('0' + u%10)
		u /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
