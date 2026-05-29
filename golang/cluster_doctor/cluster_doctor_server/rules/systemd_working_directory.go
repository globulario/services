package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/systemdutil"
)

// systemdUnitDir is the directory the rule scans. Exposed as a var so tests
// can redirect to a tempdir.
var systemdUnitDir = "/etc/systemd/system"

// systemdWorkingDirectoryMustBeOptional asserts that every installed
// `globular-<name>.service` unit file has its `WorkingDirectory=` either
// absent or in the optional `-`-prefixed form when pointing under
// `/var/lib/globular/`.
//
// systemd evaluates `WorkingDirectory=` before `ExecStartPre=`, so a missing
// directory causes the unit to fail with `status=200/CHDIR` before any
// recovery `mkdir -p` can run. The `-` prefix makes the directory optional:
// systemd falls back to "/" if missing and ExecStartPre then recreates it.
//
// Project O.5: this rule is the regression gate after Phase 1 of Project O
// removed five WD-targeted alias dirs and broke five services. After O.1
// (template fix) + O.2 (CLI normalize parity) land, the only way bare WD
// can reappear is a fresh template regression — which this rule catches.
type systemdWorkingDirectoryMustBeOptional struct{}

func (systemdWorkingDirectoryMustBeOptional) ID() string {
	return "systemd.working_directory.must_be_optional"
}
func (systemdWorkingDirectoryMustBeOptional) Category() string { return "convergence" }
func (systemdWorkingDirectoryMustBeOptional) Scope() string    { return "cluster" }

func (r systemdWorkingDirectoryMustBeOptional) Evaluate(_ *collector.Snapshot, _ Config) []Finding {
	entries, err := os.ReadDir(systemdUnitDir)
	if err != nil {
		return nil
	}

	var bare []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "globular-") || !strings.HasSuffix(name, ".service") {
			continue
		}
		full := filepath.Join(systemdUnitDir, name)
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		if systemdutil.HasBareGlobularWorkingDirectory(data) {
			bare = append(bare, name)
		}
	}

	if len(bare) == 0 {
		return nil
	}
	sort.Strings(bare)

	summary := fmt.Sprintf(
		"systemd unit(s) have bare required WorkingDirectory under /var/lib/globular (will fail status=200/CHDIR if dir missing): %s",
		strings.Join(bare, ", "),
	)
	return []Finding{{
		FindingID:       FindingID(string(r.ID()), systemdUnitDir, strings.Join(bare, ",")),
		InvariantID:     r.ID(),
		Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
		Category:        r.Category(),
		EntityRef:       systemdUnitDir,
		Summary:         summary,
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("local_fs", "systemd_unit_audit", map[string]string{
				"path":  systemdUnitDir,
				"units": strings.Join(bare, ","),
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1,
				"Change each listed unit's WorkingDirectory= to the optional form "+
					"WorkingDirectory=-/var/lib/globular/... and re-install via the canonical package path.",
				""),
		},
	}}
}
