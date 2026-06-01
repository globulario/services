// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules
// @awareness file_role=artifact_filesystem_safety_doctor_rule
// @awareness enforces=globular.platform:invariant.state.installed_not_catalog
// @awareness risk=high
package rules

import (
	"fmt"
	"os"
	"syscall"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

var (
	artifactBinDirPath         = "/usr/lib/globular/bin"
	artifactEtcdDataDirPath    = "/var/lib/globular/etcd"
	artifactCAKeyPath          = "/var/lib/globular/pki/ca.key"
	artifactInstallPolicyPath  = "/var/lib/globular/config/install-policy.json"
	artifactRequireRootCAOwner = true
)

type artifactFilesystemSafetyLocal struct{}

func (artifactFilesystemSafetyLocal) ID() string       { return "artifact.filesystem_safety_local" }
func (artifactFilesystemSafetyLocal) Category() string { return "security" }
func (artifactFilesystemSafetyLocal) Scope() string    { return "cluster" }

func (artifactFilesystemSafetyLocal) Evaluate(_ *collector.Snapshot, _ Config) []Finding {
	var findings []Finding

	// /usr/lib/globular/bin must not be group/world writable.
	if fi, err := os.Stat(artifactBinDirPath); err == nil {
		mode := fi.Mode().Perm()
		if mode&0o022 != 0 {
			findings = append(findings, fsFinding(
				artifactBinDirPath,
				cluster_doctorpb.Severity_SEVERITY_WARN,
				fmt.Sprintf("artifact bin directory is writable by group/other (mode=%#o)", mode),
			))
		}
	}

	// etcd data dir should be 0700 when present.
	if fi, err := os.Stat(artifactEtcdDataDirPath); err == nil {
		mode := fi.Mode().Perm()
		if mode != 0o700 {
			findings = append(findings, fsFinding(
				artifactEtcdDataDirPath,
				cluster_doctorpb.Severity_SEVERITY_ERROR,
				fmt.Sprintf("etcd data dir must be mode 0700 (actual=%#o)", mode),
			))
		}
	}

	// CA key must be root-owned and not accessible by group/other.
	etcdUID := uint32(0)
	if etcdFi, err := os.Stat(artifactEtcdDataDirPath); err == nil {
		if st, ok := etcdFi.Sys().(*syscall.Stat_t); ok {
			etcdUID = st.Uid
		}
	}
	if fi, err := os.Stat(artifactCAKeyPath); err == nil {
		mode := fi.Mode().Perm()
		if mode&0o077 != 0 {
			findings = append(findings, fsFinding(
				artifactCAKeyPath,
				cluster_doctorpb.Severity_SEVERITY_ERROR,
				fmt.Sprintf("CA private key is too permissive (mode=%#o, expected <=0600)", mode),
			))
		}
		if st, ok := fi.Sys().(*syscall.Stat_t); ok {
			if artifactRequireRootCAOwner && st.Uid != 0 && st.Uid != etcdUID {
				findings = append(findings, fsFinding(
					artifactCAKeyPath,
					cluster_doctorpb.Severity_SEVERITY_ERROR,
					fmt.Sprintf("CA private key owner must be root or runtime owner uid %d (actual uid=%d)", etcdUID, st.Uid),
				))
			}
		}
	}

	// install-policy is config material; must not be writable by other.
	if fi, err := os.Stat(artifactInstallPolicyPath); err == nil {
		mode := fi.Mode().Perm()
		if mode&0o002 != 0 {
			findings = append(findings, fsFinding(
				artifactInstallPolicyPath,
				cluster_doctorpb.Severity_SEVERITY_WARN,
				fmt.Sprintf("install-policy is world-writable (mode=%#o)", mode),
			))
		}
	}

	return findings
}

func fsFinding(path string, sev cluster_doctorpb.Severity, summary string) Finding {
	return Finding{
		FindingID:       FindingID("artifact.filesystem_safety_local", path, summary),
		InvariantID:     "artifact.filesystem_safety_local",
		Severity:        sev,
		Category:        "security",
		EntityRef:       path,
		Summary:         summary,
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("local_fs", "stat", map[string]string{"path": path}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, "Fix owner/mode according to artifact intent and retry doctor.", "sudo ls -l "+path),
		},
	}
}
