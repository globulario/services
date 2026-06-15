package infra_truth

import (
	"fmt"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// minioRemediationOwner enforces "config files are artifacts, not authority": the
// fix targets the OWNER that generated the bad config (config.RenderMinioEnv,
// driven by the controller-published ObjectStoreDesiredState), never a manual
// edit of minio.env — a render overwrites it.
const minioRemediationOwner = "Repair the owner that generated this config: the controller-published ObjectStoreDesiredState (mode/nodes/drives_per_node) and config.RenderMinioEnv. Do NOT hand-edit /var/lib/globular/minio/minio.env as the permanent fix — a render will overwrite it, and a wrong MINIO_VOLUMES topology risks a format.json reformat."

// AttestMinioConfig runs every attestation rule against the rendered MinIO env
// and returns the violations found (empty == config valid). It attests nothing
// when the config file is absent — that is a lifecycle fact (not yet rendered),
// not a config violation.
func AttestMinioConfig(desired *MinioDesired, rendered *MinioRenderedConfig) []*cluster_controllerpb.InfraViolation {
	if rendered == nil || !rendered.Present {
		return nil
	}

	var v []*cluster_controllerpb.InfraViolation
	add := func(viol *cluster_controllerpb.InfraViolation) {
		if viol != nil {
			v = append(v, viol)
		}
	}

	add(attestMinioVolumesPresent(rendered))
	add(attestMinioVolumesNoLoopback(rendered))
	add(attestMinioTopologyMatchesDesired(desired, rendered))
	add(attestMinioSelfInPool(desired, rendered))

	return v
}

// attestMinioVolumesPresent requires a non-empty MINIO_VOLUMES — without it the
// daemon has nothing to serve.
func attestMinioVolumesPresent(rendered *MinioRenderedConfig) *cluster_controllerpb.InfraViolation {
	if rendered.VolumeCount == 0 {
		return newViolation(
			"minio.config_valid",
			SeverityError,
			"MINIO_VOLUMES is empty — MinIO has no drives/endpoints to serve",
			"MINIO_VOLUMES=\"\"",
			minioRemediationOwner,
		)
	}
	return nil
}

// attestMinioVolumesNoLoopback flags a CRITICAL violation if any distributed
// volume endpoint resolves to a loopback/unspecified host — peers can never
// reach a pool member advertised on loopback.
func attestMinioVolumesNoLoopback(rendered *MinioRenderedConfig) *cluster_controllerpb.InfraViolation {
	for _, vol := range rendered.Volumes {
		host := volumeHost(vol)
		if host == "" {
			continue // local path (standalone) — no host to check
		}
		if isLoopback(host) || isUnspecified(host) {
			return newViolation(
				"minio.loopback_forbidden",
				SeverityCritical,
				fmt.Sprintf("a distributed MINIO_VOLUMES endpoint advertises a non-routable host (%s) — pool peers can never reach this member", host),
				fmt.Sprintf("volume=%s", vol),
				minioRemediationOwner,
			)
		}
	}
	return nil
}

// attestMinioTopologyMatchesDesired is the format.json blast-radius guard. A
// desired-distributed pool rendered as standalone (a single local path) means the
// node would form an ISOLATED single-node store and diverge from the pool; a
// rendered volume count that disagrees with desired nodes×drives means the node
// would format a different drive topology. Both are CRITICAL because MinIO can
// silently reformat drives when the topology changes underneath it.
func attestMinioTopologyMatchesDesired(desired *MinioDesired, rendered *MinioRenderedConfig) *cluster_controllerpb.InfraViolation {
	if desired == nil {
		return nil
	}
	if desired.Mode == MinioModeDistributed && rendered.Mode == MinioModeStandalone {
		return newViolation(
			"minio.topology_matches_desired",
			SeverityCritical,
			fmt.Sprintf("desired topology is distributed (%d nodes) but the rendered MINIO_VOLUMES is standalone (local path) — this node would form an isolated single-node store and diverge from the pool (split-brain)", len(desired.ExpectedPeers)),
			fmt.Sprintf("desired_mode=distributed rendered_mode=standalone volumes=%s", strings.Join(rendered.Volumes, " ")),
			minioRemediationOwner,
		)
	}
	if exp := desired.ExpectedVolumeCount(); exp > 0 && rendered.VolumeCount > 0 && rendered.VolumeCount != exp {
		return newViolation(
			"minio.topology_matches_desired",
			SeverityCritical,
			fmt.Sprintf("rendered MINIO_VOLUMES has %d entries but desired topology expects %d (%d nodes × %d drives) — a drive-count mismatch risks a format.json reformat and erasure-set divergence", rendered.VolumeCount, exp, len(desired.ExpectedPeers), desired.effectiveDrives()),
			fmt.Sprintf("rendered_count=%d expected_count=%d", rendered.VolumeCount, exp),
			minioRemediationOwner,
		)
	}
	return nil
}

// attestMinioSelfInPool verifies that, in distributed mode, this node's expected
// address appears among the rendered volume endpoints — i.e. the node is actually
// part of the pool it rendered. A self-absent pool means the env was rendered for
// the wrong node identity.
func attestMinioSelfInPool(desired *MinioDesired, rendered *MinioRenderedConfig) *cluster_controllerpb.InfraViolation {
	if desired == nil || rendered.Mode != MinioModeDistributed || len(rendered.Endpoints) == 0 {
		return nil
	}
	if len(desired.ExpectedListenAddresses) == 0 {
		return nil
	}
	self := stripQuotes(desired.ExpectedListenAddresses[0])
	for _, ep := range rendered.Endpoints {
		if ep == self {
			return nil
		}
	}
	return newViolation(
		"minio.config_valid",
		SeverityError,
		fmt.Sprintf("this node (%s) does not appear among the rendered distributed volume endpoints %s — the env was rendered for the wrong node identity", self, strings.Join(rendered.Endpoints, ",")),
		fmt.Sprintf("self=%s endpoints=%s", self, strings.Join(rendered.Endpoints, ",")),
		minioRemediationOwner,
	)
}
