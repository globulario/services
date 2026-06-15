package infra_truth

import (
	"fmt"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// envoyRemediationOwner enforces "config files are artifacts, not authority": the
// fix targets the OWNER that generated the bad bootstrap (the gateway/xDS control
// plane that writes envoy-bootstrap.json), never a manual edit — the gateway
// rewrites it on startup, and it lives under /run (regenerated each boot).
const envoyRemediationOwner = "Repair the owner that generated this bootstrap: the gateway/xDS control plane that writes /run/globular/envoy/envoy-bootstrap.json. Do NOT hand-edit it — it is a per-boot /run artifact the gateway rewrites; the dynamic config itself comes from xDS, not the file."

// AttestEnvoyConfig runs every attestation rule against the rendered Envoy
// bootstrap and returns the violations found (empty == config valid). It attests
// nothing when the bootstrap is absent — that is a lifecycle fact (gateway has
// not written it yet), not a config violation.
//
// Note: unlike the clustered components, Envoy has NO loopback rule — the admin
// interface and the local xDS upstream are intentionally on loopback. The
// attestation is structural: can the ADS handshake happen, and can listeners and
// clusters ever load dynamically.
func AttestEnvoyConfig(desired *InfraDesiredState, rendered *EnvoyRenderedConfig) []*cluster_controllerpb.InfraViolation {
	if rendered == nil || !rendered.Present {
		return nil
	}

	var v []*cluster_controllerpb.InfraViolation
	add := func(viol *cluster_controllerpb.InfraViolation) {
		if viol != nil {
			v = append(v, viol)
		}
	}

	add(attestEnvoyADSConfig(rendered))
	add(attestEnvoyLDSConfig(rendered))
	add(attestEnvoyCDSConfig(rendered))
	add(attestEnvoyADSClusterDefined(rendered))
	add(attestEnvoyNodeID(rendered))
	add(attestEnvoyAdminAddress(rendered))

	return v
}

// attestEnvoyADSConfig requires dynamic_resources.ads_config — without it Envoy
// runs with no dynamic config at all and the mesh is dead.
func attestEnvoyADSConfig(rendered *EnvoyRenderedConfig) *cluster_controllerpb.InfraViolation {
	if !rendered.HasADSConfig {
		return newViolation(
			"envoy.config_valid",
			SeverityCritical,
			"bootstrap has no dynamic_resources.ads_config — Envoy will run with no dynamic config; the HTTP mesh is dead",
			"ads_config=absent",
			envoyRemediationOwner,
		)
	}
	return nil
}

// attestEnvoyLDSConfig requires dynamic_resources.lds_config. Without it no
// listeners ever load via xDS — the STATIC analog of the LDS wedge: port 443
// never binds and HTTP routing is dead even though the daemon is "active".
func attestEnvoyLDSConfig(rendered *EnvoyRenderedConfig) *cluster_controllerpb.InfraViolation {
	if !rendered.HasLDSConfig {
		return newViolation(
			"envoy.config_valid",
			SeverityCritical,
			"bootstrap has no dynamic_resources.lds_config — listeners will never load via xDS; port 443 stays unbound and the HTTP mesh is down (the static-config form of the LDS wedge)",
			"lds_config=absent",
			envoyRemediationOwner,
		)
	}
	return nil
}

// attestEnvoyCDSConfig requires dynamic_resources.cds_config — without it
// upstream clusters never load via xDS and all routing fails.
func attestEnvoyCDSConfig(rendered *EnvoyRenderedConfig) *cluster_controllerpb.InfraViolation {
	if !rendered.HasCDSConfig {
		return newViolation(
			"envoy.config_valid",
			SeverityError,
			"bootstrap has no dynamic_resources.cds_config — upstream clusters will never load via xDS",
			"cds_config=absent",
			envoyRemediationOwner,
		)
	}
	return nil
}

// attestEnvoyADSClusterDefined requires the cluster the ADS stream targets to be
// defined in static_resources.clusters. If it is missing, the ADS gRPC stream
// has nowhere to connect and NO dynamic config ever arrives.
func attestEnvoyADSClusterDefined(rendered *EnvoyRenderedConfig) *cluster_controllerpb.InfraViolation {
	if !rendered.HasADSConfig {
		return nil // ads-config absence already reported
	}
	want := rendered.ADSClusterName
	if want == "" {
		want = envoyExpectedADSCluster
	}
	if rendered.hasStaticCluster(want) {
		return nil
	}
	return newViolation(
		"envoy.config_valid",
		SeverityCritical,
		fmt.Sprintf("ADS targets cluster %q but it is not defined in static_resources.clusters %v — the ADS stream cannot connect to xDS and no dynamic config will arrive", want, rendered.StaticClusterNames),
		fmt.Sprintf("ads_cluster=%s static_clusters=%v", want, rendered.StaticClusterNames),
		envoyRemediationOwner,
	)
}

// attestEnvoyNodeID requires a non-empty node id — xDS keys snapshots by node id,
// so an empty id means Envoy never matches a snapshot and gets no config.
func attestEnvoyNodeID(rendered *EnvoyRenderedConfig) *cluster_controllerpb.InfraViolation {
	if rendered.NodeID == "" {
		return newViolation(
			"envoy.config_valid",
			SeverityError,
			"bootstrap node.id is empty — xDS snapshots are keyed by node id, so Envoy will never be served a config",
			"node.id=\"\"",
			envoyRemediationOwner,
		)
	}
	return nil
}

// attestEnvoyAdminAddress requires the admin address so the truth plane (and
// operators) can observe Envoy at all. Loopback is fine and expected here.
func attestEnvoyAdminAddress(rendered *EnvoyRenderedConfig) *cluster_controllerpb.InfraViolation {
	if rendered.AdminAddress == "" || rendered.AdminPort == 0 {
		return newViolation(
			"envoy.config_valid",
			SeverityError,
			"bootstrap has no admin address — Envoy cannot be observed via its native admin API",
			fmt.Sprintf("admin=%s:%d", rendered.AdminAddress, rendered.AdminPort),
			envoyRemediationOwner,
		)
	}
	return nil
}
