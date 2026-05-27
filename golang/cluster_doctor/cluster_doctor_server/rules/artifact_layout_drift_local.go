package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

var artifactStateRootPath = "/var/lib/globular"

var artifactTopLevelAllowlist = map[string]bool{
	"awareness":           true,
	"alertmanager":        true,
	"backups":             true,
	"bootstrap.enabled":   true,
	"cluster-controller":  true,
	"clustercontroller":   true,
	"clusterdoctor":       true,
	"config":              true,
	"config.json":         true,
	"data":                true,
	"domains":             true,
	"etcd":                true,
	"ingress":             true,
	"inventory":           true,
	"intent":              true,
	"keys":                true,
	"mcp":                 true,
	"minio":               true,
	"nodeagent":           true,
	"objectstore":         true,
	"operational-knowledge": true,
	"packages":            true,
	"pki":                 true,
	"policy":              true,
	"prometheus":          true,
	"release-index.json":  true,
	"repository":          true,
	"scylla-manager":      true,
	"scylla-manager-agent": true,
	"services":            true,
	"sidekick":            true,
	"staging":             true,
	"tokens":              true,
	"webroot":             true,
	"workflow":            true,
	"workflows":           true,
	"xds":                 true,
}

type artifactLayoutDriftLocal struct{}

func (artifactLayoutDriftLocal) ID() string       { return "artifact.layout_drift_local" }
func (artifactLayoutDriftLocal) Category() string { return "convergence" }
func (artifactLayoutDriftLocal) Scope() string    { return "cluster" }

func (artifactLayoutDriftLocal) Evaluate(_ *collector.Snapshot, _ Config) []Finding {
	ents, err := os.ReadDir(artifactStateRootPath)
	if err != nil {
		return nil
	}
	var unexpected []string
	for _, e := range ents {
		name := strings.TrimSpace(e.Name())
		if name == "" {
			continue
		}
		if artifactTopLevelAllowlist[name] {
			continue
		}
		if strings.HasPrefix(name, ".") {
			continue
		}
		unexpected = append(unexpected, name)
	}
	if len(unexpected) == 0 {
		return nil
	}
	sort.Strings(unexpected)
	return []Finding{
		{
			FindingID:       FindingID("artifact.layout_drift_local", artifactStateRootPath, strings.Join(unexpected, ",")),
			InvariantID:     "artifact.layout_drift_local",
			Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:        "convergence",
			EntityRef:       artifactStateRootPath,
			Summary:         fmt.Sprintf("unexpected top-level entries under %s: %s", artifactStateRootPath, strings.Join(unexpected, ", ")),
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("local_fs", "readdir", map[string]string{
					"path":       artifactStateRootPath,
					"unexpected": strings.Join(unexpected, ","),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Review whether these paths are intentional or stale leftovers from non-canonical installs.", "sudo ls -la "+filepath.Clean(artifactStateRootPath)),
			},
		},
	}
}
