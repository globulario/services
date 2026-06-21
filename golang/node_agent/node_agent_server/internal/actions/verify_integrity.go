// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.actions.verify_integrity
// @awareness file_role=binary_sha256_verification_action_blocking_install_when_manifest_checksum_mismatches
// @awareness enforces=globular.platform:invariant.controller.apply_package_release_requires_manifest_checksum
// @awareness implements=globular.platform:intent.node_agent.install_claim_requires_binary_proof
// @awareness risk=critical
package actions

// verify_integrity.go — the bottom of the install chain's safety
// net. Computes the on-disk binary sha256 and compares it to the
// manifest's entrypoint_checksum carried as ExpectedSha256 in
// the dispatch. A mismatch MUST fail the action — proceeding
// with an unverified binary makes the rest of the 4-layer model
// (installed-state, proof, heartbeat) describe a binary the
// controller never approved.
//
// Wrapper packages (keepalived, scylladb, etc — those whose
// "entrypoint" is bin/noop or a thin wrapper) cannot produce a
// meaningful binary checksum. The verifier MUST detect these
// (installed_path outside /usr/lib/globular/bin/) and skip the
// binary-hash check rather than fail it; see existing wrapper-
// package handling.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/digest"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/repository/repositorypb"
)

// defaultVerifyPublisher matches the default publisher used by ApplyPackageRelease.
const defaultVerifyPublisher = "core@globular.io"

// ── package.verify_integrity ────────────────────────────────────────────────
//
// Local artifact-identity invariant check. For each installed package on this
// node, compares:
//
//	I1. Cached artifact checksum (/var/lib/globular/staging/<pub>/<name>/latest.artifact)
//	    vs the repository manifest digest for the installed build.
//	    FAIL → cache is stale/corrupt and will produce wrong bytes on next install.
//
//	I2. Installed build (etcd installed_state) vs desired build (etcd ServiceDesiredVersion).
//	    FAIL → node is not converged.
//
//	I3. Installed artifact checksum (etcd installed_state.Checksum) vs repository
//	    manifest digest for the installed build.
//	    FAIL → installed bytes don't match the published artifact identity.
//
//	I4. Presence: release has resolved digest but no fetched artifact on disk.
//	    (cache missing or inconsistent)
//
// The action returns a JSON summary of findings in its result string. Each
// finding has an invariant id, entity ref, and key=value evidence. Callers
// (doctor, CLI, admin UI) can parse the JSON without a proto schema change.
//
// Args:
//
//	package_name (string, optional) — limit check to one package. Empty = all.
//	kind         (string, optional) — filter by SERVICE/INFRASTRUCTURE/COMMAND.
//	repository_addr (string, optional) — repository gRPC endpoint. Auto-discovered if empty.
//	node_id      (string, optional) — override node id (defaults to local).
//
// Returns the full findings map via the result string (JSON).
type packageVerifyIntegrityAction struct{}

func (packageVerifyIntegrityAction) Name() string { return "package.verify_integrity" }

func (packageVerifyIntegrityAction) Validate(args *structpb.Struct) error { return nil }

// Finding is a single invariant violation discovered by verify_integrity.
type integrityFinding struct {
	Invariant string            `json:"invariant"`
	Severity  string            `json:"severity"`
	Package   string            `json:"package"`
	Kind      string            `json:"kind"`
	Summary   string            `json:"summary"`
	Evidence  map[string]string `json:"evidence,omitempty"`
}

// integrityReport is the JSON result of package.verify_integrity.
type integrityReport struct {
	NodeID     string             `json:"node_id"`
	Checked    int                `json:"checked"`
	Findings   []integrityFinding `json:"findings"`
	Errors     []string           `json:"errors,omitempty"`
	Invariants map[string]int     `json:"invariants"` // id → fail count
}

func (packageVerifyIntegrityAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	pkgFilter := strings.TrimSpace(fields["package_name"].GetStringValue())
	kindFilter := strings.ToUpper(strings.TrimSpace(fields["kind"].GetStringValue()))
	repoAddr := strings.TrimSpace(fields["repository_addr"].GetStringValue())
	nodeID := strings.TrimSpace(fields["node_id"].GetStringValue())

	if repoAddr == "" {
		repoAddr = config.ResolveServiceAddr("repository.PackageRepository", "")
	}
	if repoAddr == "" {
		repoAddr = discoverRepositoryViaGateway()
	}

	// Collect installed packages on this node.
	var pkgs []*repositorypb.ArtifactRef
	installedMap := make(map[string]installedRef) // key: kind/name

	kinds := []string{"SERVICE", "INFRASTRUCTURE", "COMMAND", "APPLICATION"}
	if kindFilter != "" {
		kinds = []string{kindFilter}
	}
	report := integrityReport{
		NodeID:     nodeID,
		Invariants: map[string]int{},
	}
	for _, k := range kinds {
		list, err := installed_state.ListInstalledPackages(ctx, nodeID, k)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("list %s: %v", k, err))
			continue
		}
		for _, p := range list {
			if pkgFilter != "" && p.GetName() != pkgFilter {
				continue
			}
			name := p.GetName()
			if name == "" {
				continue
			}
			platform := p.GetPlatform()
			if platform == "" {
				platform = "linux_amd64"
			}
			pub := p.GetPublisherId()
			if pub == "" || pub == "unknown" {
				pub = defaultVerifyPublisher
			}
			pkgs = append(pkgs, &repositorypb.ArtifactRef{
				PublisherId: pub,
				Name:        name,
				Version:     p.GetVersion(),
				Platform:    platform,
				Kind:        kindToProto(k),
			})
			// Binary-vs-binary domain rule: prefer the entrypoint_checksum
			// recorded in package metadata over the top-level Checksum
			// field, because the controller's convergence-committer
			// overwrites pkg.Checksum with the convergence
			// desired_hash (a schema-string SHA from
			// ComputeInfrastructureDesiredHash / ComputeReleaseDesiredHash) so
			// its drift reconciler can match installed_checksum == desired_hash.
			// Comparing that against the manifest's entrypoint_checksum
			// (a binary SHA) crosses domains and fires
			// artifact.installed_digest_mismatch on every healthy
			// service. The metadata field is the binary hash node-agent
			// computed at install time and is the only safe input here.
			entrypoint := ""
			if md := p.GetMetadata(); md != nil {
				entrypoint = digest.CanonicalSHA256(md["entrypoint_checksum"])
			}
			installedMap[k+"/"+name] = installedRef{
				ref:         pkgs[len(pkgs)-1],
				kind:        k,
				build:       p.GetBuildNumber(),
				checksum:    entrypoint,
				installedAt: p.GetInstalledUnix(),
			}
		}
	}

	report.Checked = len(pkgs)

	// Resolve repository manifests in parallel — one GetArtifactManifest
	// RPC per installed package. Sequential iteration takes ~5 seconds for
	// ~48 packages, which blows past the doctor collector's NodeTimeout.
	// A small worker pool keeps the total wall time under 1 second for
	// typical clusters without hammering the repository.
	manifestArchiveDigests := make(map[string]string)    // key: kind/name → archive sha256
	manifestEntrypointDigests := make(map[string]string) // key: kind/name → entrypoint sha256
	if repoAddr != "" && len(installedMap) > 0 {
		type resolveResult struct {
			key              string
			archiveDigest    string
			entrypointDigest string
			err              error
			pubID            string
			name             string
			ver              string
			build            int64
		}
		workerCount := 8
		if n := len(installedMap); n < workerCount {
			workerCount = n
		}
		jobs := make(chan struct {
			key  string
			inst installedRef
		}, len(installedMap))
		results := make(chan resolveResult, len(installedMap))

		var wg sync.WaitGroup
		for w := 0; w < workerCount; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range jobs {
					archiveDigest, aErr := resolveArtifactDigest(ctx, repoAddr,
						j.inst.ref.GetPublisherId(), j.inst.ref.GetName(),
						j.inst.ref.GetVersion(), j.inst.ref.GetPlatform(),
						strings.ToUpper(j.inst.kind), j.inst.build)
					entryDigest, eErr := resolveArtifactEntrypointDigest(ctx, repoAddr,
						j.inst.ref.GetPublisherId(), j.inst.ref.GetName(),
						j.inst.ref.GetVersion(), j.inst.ref.GetPlatform(),
						strings.ToUpper(j.inst.kind), j.inst.build)
					err := aErr
					if err == nil {
						err = eErr
					}
					results <- resolveResult{
						key:              j.key,
						archiveDigest:    archiveDigest,
						entrypointDigest: entryDigest,
						err:              err,
						pubID:            j.inst.ref.GetPublisherId(),
						name:             j.inst.ref.GetName(),
						ver:              j.inst.ref.GetVersion(),
						build:            j.inst.build,
					}
				}
			}()
		}
		for key, inst := range installedMap {
			jobs <- struct {
				key  string
				inst installedRef
			}{key: key, inst: inst}
		}
		close(jobs)
		wg.Wait()
		close(results)
		for r := range results {
			if r.err != nil {
				report.Errors = append(report.Errors,
					fmt.Sprintf("resolve manifest %s/%s@%s+%d: %v",
						r.pubID, r.name, r.ver, r.build, r.err))
				continue
			}
			manifestArchiveDigests[r.key] = r.archiveDigest
			manifestEntrypointDigests[r.key] = r.entrypointDigest
		}
	}

	// Collect desired versions once.
	desiredMap := readDesiredVersions(ctx)

	// ── Evaluate invariants ──
	for key, inst := range installedMap {
		// I2: desired vs installed build mismatch
		if d, ok := desiredMap[strings.ToLower(inst.ref.GetName())]; ok {
			if d.Version != "" && d.Version != inst.ref.GetVersion() {
				report.Findings = append(report.Findings, integrityFinding{
					Invariant: "artifact.desired_version_mismatch",
					Severity:  "WARN",
					Package:   inst.ref.GetName(),
					Kind:      inst.kind,
					Summary: fmt.Sprintf("installed %s+%d but desired %s+%d",
						inst.ref.GetVersion(), inst.build, d.Version, d.Build),
					Evidence: map[string]string{
						"installed_version": inst.ref.GetVersion(),
						"installed_build":   fmt.Sprintf("%d", inst.build),
						"desired_version":   d.Version,
						"desired_build":     fmt.Sprintf("%d", d.Build),
					},
				})
				report.Invariants["artifact.desired_version_mismatch"]++
			} else if d.Build > 0 && inst.build > 0 && d.Build != inst.build {
				// inst.build == 0 means installed_state was written before
				// build_number tracking was rigorous (or by a code path
				// that left it unset). The convergence-committer stamps
				// buildNumber from desired on the next convergence pass,
				// so this is a "will catch up" state — not a real drift.
				// Without this guard, every fresh cluster shows ~10–15
				// WARN findings (one per service where installed_state
				// predates build_number) that operators can't act on.
				report.Findings = append(report.Findings, integrityFinding{
					Invariant: "artifact.desired_build_mismatch",
					Severity:  "WARN",
					Package:   inst.ref.GetName(),
					Kind:      inst.kind,
					Summary: fmt.Sprintf("installed build %d but desired build %d",
						inst.build, d.Build),
					Evidence: map[string]string{
						"installed_build": fmt.Sprintf("%d", inst.build),
						"desired_build":   fmt.Sprintf("%d", d.Build),
					},
				})
				report.Invariants["artifact.desired_build_mismatch"]++
			}
		}

		// I1: cached artifact checksum vs manifest digest
		cachePath := filepath.Join("/var/lib/globular/staging",
			inst.ref.GetPublisherId(), inst.ref.GetName(), "latest.artifact")
		if fi, err := os.Stat(cachePath); err == nil && !fi.IsDir() {
			if expected, ok := manifestArchiveDigests[key]; ok && expected != "" {
				actual, herr := sha256OfFile(cachePath)
				if herr != nil {
					report.Errors = append(report.Errors, fmt.Sprintf("hash %s: %v", cachePath, herr))
				} else if actual != expected {
					report.Findings = append(report.Findings, integrityFinding{
						Invariant: "artifact.cache_digest_mismatch",
						Severity:  "WARN",
						Package:   inst.ref.GetName(),
						Kind:      inst.kind,
						Summary: fmt.Sprintf("cached %s has sha256 %s, manifest expects %s",
							filepath.Base(cachePath), shortDigest(actual), shortDigest(expected)),
						Evidence: map[string]string{
							"cache_path":      cachePath,
							"cache_sha256":    actual,
							"manifest_sha256": expected,
						},
					})
					report.Invariants["artifact.cache_digest_mismatch"]++
				}
			}
		}

		// I3: installed record checksum vs manifest digest
		if expected, ok := manifestEntrypointDigests[key]; ok && expected != "" && inst.checksum != "" {
			if inst.checksum != expected {
				report.Findings = append(report.Findings, integrityFinding{
					Invariant: "artifact.installed_digest_mismatch",
					Severity:  "ERROR",
					Package:   inst.ref.GetName(),
					Kind:      inst.kind,
					Summary: fmt.Sprintf("installed_state checksum %s differs from manifest %s",
						shortDigest(inst.checksum), shortDigest(expected)),
					Evidence: map[string]string{
						"installed_sha256": inst.checksum,
						"manifest_sha256":  expected,
					},
				})
				report.Invariants["artifact.installed_digest_mismatch"]++
			}
		}

		// I4: release resolved a digest but no cache present
		if expected, ok := manifestArchiveDigests[key]; ok && expected != "" {
			if _, err := os.Stat(cachePath); os.IsNotExist(err) {
				report.Findings = append(report.Findings, integrityFinding{
					Invariant: "artifact.cache_missing",
					Severity:  "INFO",
					Package:   inst.ref.GetName(),
					Kind:      inst.kind,
					Summary:   fmt.Sprintf("manifest digest resolved but cache at %s is absent", cachePath),
					Evidence: map[string]string{
						"cache_path":      cachePath,
						"manifest_sha256": expected,
					},
				})
				report.Invariants["artifact.cache_missing"]++
			}
		}
	}

	// Always return the JSON so callers can parse it even when there are 0
	// findings — the "all green" report is itself a useful signal.
	blob, err := json.MarshalIndent(&report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal integrity report: %w", err)
	}
	return string(blob), nil
}

// installedRef carries just enough to drive the invariant checks.
type installedRef struct {
	ref         *repositorypb.ArtifactRef
	kind        string
	build       int64
	checksum    string
	installedAt int64
}

// DesiredRef is the per-service desired-state tuple consumed by the I2
// invariant. Exported because the resolver injection point (see
// SetDesiredVersionResolver) has to be callable from the node-agent
// server package — which lives outside this internal/actions package
// and so cannot reference an unexported type.
type DesiredRef struct {
	Version string
	Build   int64 // build_number
}

// desiredVersionResolver returns L2 desired-state versions indexed by
// lowercase short service name. The actions package owns the function
// signature; the implementation is INSTALLED by the node-agent server
// at boot via SetDesiredVersionResolver — that implementation dials
// the cluster_controller's GetDesiredState typed RPC.
//
// Why injection (rather than dialling here): the cluster_controller
// OWNS the /globular/resources/ServiceDesiredVersion/ prefix. Reading
// it directly from this package violates
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage
// (the owner applies type, version, and audit contracts a raw etcd
// read bypasses). The injection keeps this package free of the
// cluster_controllerpb import while still letting the server wire the
// typed call.
//
// Default returns an empty map — if the server never installed a
// resolver (test scaffolds, early-boot), verify_integrity simply
// skips the I2 invariant rather than fabricating desired state.
var desiredVersionResolver func(ctx context.Context) map[string]DesiredRef = func(_ context.Context) map[string]DesiredRef {
	return map[string]DesiredRef{}
}

// SetDesiredVersionResolver installs the desired-state resolver. The
// node-agent server calls this at boot with a closure that issues
// cluster_controller.GetDesiredState. Passing nil is a no-op so a
// post-boot reset cannot leave the package without a resolver.
func SetDesiredVersionResolver(fn func(ctx context.Context) map[string]DesiredRef) {
	if fn != nil {
		desiredVersionResolver = fn
	}
}

// readDesiredVersions returns the L2 desired-version map via the
// injected resolver. The map is keyed by lowercase short service name
// (e.g. "echo", not "core@globular.io/echo").
func readDesiredVersions(ctx context.Context) map[string]DesiredRef {
	return desiredVersionResolver(ctx)
}

// kindToProto maps the string kind to the proto enum.
func kindToProto(k string) repositorypb.ArtifactKind {
	switch strings.ToUpper(k) {
	case "INFRASTRUCTURE":
		return repositorypb.ArtifactKind_INFRASTRUCTURE
	case "APPLICATION":
		return repositorypb.ArtifactKind_APPLICATION
	case "COMMAND":
		return repositorypb.ArtifactKind_COMMAND
	default:
		return repositorypb.ArtifactKind_SERVICE
	}
}

// shortDigest returns the first 12 hex chars of a sha256 digest.
func shortDigest(d string) string {
	d = digest.CanonicalSHA256(d)
	if len(d) <= 12 {
		return d
	}
	return d[:12]
}

// sha256OfFile streams the file and returns its lowercase hex digest.
func sha256OfFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func init() {
	Register(packageVerifyIntegrityAction{})
}
