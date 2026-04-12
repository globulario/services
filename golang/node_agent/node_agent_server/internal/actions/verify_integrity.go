package actions

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

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/globulario/services/golang/config"
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
//   I1. Cached artifact checksum (/var/lib/globular/staging/<pub>/<name>/latest.artifact)
//       vs the repository manifest digest for the installed build.
//       FAIL → cache is stale/corrupt and will produce wrong bytes on next install.
//
//   I2. Installed build (etcd installed_state) vs desired build (etcd ServiceDesiredVersion).
//       FAIL → node is not converged.
//
//   I3. Installed artifact checksum (etcd installed_state.Checksum) vs repository
//       manifest digest for the installed build.
//       FAIL → installed bytes don't match the published artifact identity.
//
//   I4. Presence: release has resolved digest but no fetched artifact on disk.
//       (cache missing or inconsistent)
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
			installedMap[k+"/"+name] = installedRef{
				ref:         pkgs[len(pkgs)-1],
				kind:        k,
				build:       p.GetBuildNumber(),
				checksum:    normalizeSHA256Digest(p.GetChecksum()),
				installedAt: p.GetInstalledUnix(),
			}
		}
	}

	report.Checked = len(pkgs)

	// Resolve repository manifests in bulk (one call per package).
	manifestDigests := make(map[string]string) // key: kind/name → expected sha256
	for key, inst := range installedMap {
		if repoAddr == "" {
			break
		}
		digest, err := resolveArtifactDigest(ctx, repoAddr,
			inst.ref.GetPublisherId(), inst.ref.GetName(), inst.ref.GetVersion(), inst.ref.GetPlatform(),
			strings.ToUpper(inst.kind), inst.build)
		if err != nil {
			report.Errors = append(report.Errors,
				fmt.Sprintf("resolve manifest %s/%s@%s+%d: %v",
					inst.ref.GetPublisherId(), inst.ref.GetName(), inst.ref.GetVersion(), inst.build, err))
			continue
		}
		manifestDigests[key] = digest
	}

	// Collect desired versions once.
	desiredMap := readDesiredVersionsFromEtcd(ctx)

	// ── Evaluate invariants ──
	for key, inst := range installedMap {
		// I2: desired vs installed build mismatch
		if d, ok := desiredMap[strings.ToLower(inst.ref.GetName())]; ok {
			if d.version != "" && d.version != inst.ref.GetVersion() {
				report.Findings = append(report.Findings, integrityFinding{
					Invariant: "artifact.desired_version_mismatch",
					Severity:  "WARN",
					Package:   inst.ref.GetName(),
					Kind:      inst.kind,
					Summary: fmt.Sprintf("installed %s+%d but desired %s+%d",
						inst.ref.GetVersion(), inst.build, d.version, d.build),
					Evidence: map[string]string{
						"installed_version": inst.ref.GetVersion(),
						"installed_build":   fmt.Sprintf("%d", inst.build),
						"desired_version":   d.version,
						"desired_build":     fmt.Sprintf("%d", d.build),
					},
				})
				report.Invariants["artifact.desired_version_mismatch"]++
			} else if d.build > 0 && d.build != inst.build {
				report.Findings = append(report.Findings, integrityFinding{
					Invariant: "artifact.desired_build_mismatch",
					Severity:  "WARN",
					Package:   inst.ref.GetName(),
					Kind:      inst.kind,
					Summary: fmt.Sprintf("installed build %d but desired build %d",
						inst.build, d.build),
					Evidence: map[string]string{
						"installed_build": fmt.Sprintf("%d", inst.build),
						"desired_build":   fmt.Sprintf("%d", d.build),
					},
				})
				report.Invariants["artifact.desired_build_mismatch"]++
			}
		}

		// I1: cached artifact checksum vs manifest digest
		cachePath := filepath.Join("/var/lib/globular/staging",
			inst.ref.GetPublisherId(), inst.ref.GetName(), "latest.artifact")
		if fi, err := os.Stat(cachePath); err == nil && !fi.IsDir() {
			if expected, ok := manifestDigests[key]; ok && expected != "" {
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
		if expected, ok := manifestDigests[key]; ok && expected != "" && inst.checksum != "" {
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
		if expected, ok := manifestDigests[key]; ok && expected != "" {
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

// desiredRef is the per-service desired-state tuple consumed by I2.
type desiredRef struct {
	version string
	build   int64
}

// readDesiredVersionsFromEtcd fetches /globular/resources/ServiceDesiredVersion/*
// and indexes by lowercase service name.
func readDesiredVersionsFromEtcd(ctx context.Context) map[string]desiredRef {
	out := make(map[string]desiredRef)
	cli, err := config.GetEtcdClient()
	if err != nil {
		return out
	}
	resp, err := cli.Get(ctx, "/globular/resources/ServiceDesiredVersion/", clientv3.WithPrefix())
	if err != nil {
		return out
	}
	for _, kv := range resp.Kvs {
		var rec struct {
			Spec struct {
				ServiceName string `json:"service_name"`
				Version     string `json:"version"`
				BuildNumber int64  `json:"build_number"`
			} `json:"spec"`
		}
		if err := json.Unmarshal(kv.Value, &rec); err != nil {
			continue
		}
		if rec.Spec.ServiceName == "" {
			continue
		}
		out[strings.ToLower(rec.Spec.ServiceName)] = desiredRef{
			version: rec.Spec.Version,
			build:   rec.Spec.BuildNumber,
		}
	}
	return out
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

// normalizeSHA256Digest strips "sha256:" and lowercases.
func normalizeSHA256Digest(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.TrimPrefix(s, "sha256:")
}

// shortDigest returns the first 12 hex chars of a sha256 digest.
func shortDigest(d string) string {
	d = normalizeSHA256Digest(d)
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
