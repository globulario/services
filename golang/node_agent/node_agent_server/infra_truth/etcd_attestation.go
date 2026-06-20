package infra_truth

import (
	"fmt"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// etcdRemediationOwner enforces "config files are artifacts, not authority": the
// fix targets the OWNER that generated the bad config (the controller's
// reconcileServiceConfigs renderer + controller-provided desired state / etcd
// membership), never a manual edit of etcd.yaml — a render overwrites it.
const etcdRemediationOwner = "Repair the owner that generated this config: the cluster-controller's etcd config renderer (reconcileServiceConfigs) and the controller-provided membership (initial-cluster / routable node IP). Do NOT hand-edit /var/lib/globular/config/etcd.yaml as the permanent fix — a render will overwrite it."

// AttestEtcdConfig runs every attestation rule against the rendered etcd config
// and returns the violations found (empty == config valid). It attests nothing
// when the config file is absent — that is a lifecycle fact (not yet rendered),
// not a config violation.
func AttestEtcdConfig(desired *InfraDesiredState, rendered *EtcdRenderedConfig) []*cluster_controllerpb.InfraViolation {
	if rendered == nil || !rendered.Present {
		return nil
	}

	var v []*cluster_controllerpb.InfraViolation
	add := func(viol *cluster_controllerpb.InfraViolation) {
		if viol != nil {
			v = append(v, viol)
		}
	}

	// The *advertise* URLs are what peers and clients dial — they must be a
	// concrete routable address, never loopback (isolates the member) nor the
	// unspecified address 0.0.0.0 (meaningless as something to dial). The
	// *listen* URLs are bind addresses: 0.0.0.0 is correct and standard (bind
	// every interface) — only loopback is forbidden there, since a member that
	// binds peer/client traffic to 127.0.0.1 alone can never be reached. The
	// renderer reflects exactly this split (advertise=node IP, listen=0.0.0.0),
	// so forbidUnspecified is true only for the advertise fields.
	add(attestEtcdURLsNoLoopback("listen-peer-urls", rendered.ListenPeerURLs, false))
	add(attestEtcdURLsNoLoopback("initial-advertise-peer-urls", rendered.InitialAdvertisePeerURLs, true))
	add(attestEtcdURLsNoLoopback("advertise-client-urls", rendered.AdvertiseClientURLs, true))
	add(attestEtcdURLsNoLoopback("listen-client-urls", rendered.ListenClientURLs, false))

	add(attestEtcdName(rendered))
	add(attestEtcdInitialClusterToken(desired, rendered))
	add(attestEtcdSelfInInitialCluster(desired, rendered))
	add(attestEtcdInitialClusterMembers(desired, rendered))
	add(attestEtcdPeerTLS(rendered))

	return v
}

// attestEtcdURLsNoLoopback flags a CRITICAL violation if any URL in the list
// resolves to a loopback host (or the unspecified address when forbidUnspecified
// is set). An empty list is not attested here — completeness is handled by the
// dedicated config rules. Returns nil when every URL is acceptable.
func attestEtcdURLsNoLoopback(field string, urls []string, forbidUnspecified bool) *cluster_controllerpb.InfraViolation {
	for _, raw := range urls {
		host := hostFromURL(raw)
		if host == "" {
			continue
		}
		switch {
		case isLoopback(host):
			return newViolation(
				"etcd.loopback_forbidden",
				SeverityCritical,
				fmt.Sprintf("%s advertises a loopback address (%s) — an etcd member must not advertise loopback; peers can never reach it", field, host),
				fmt.Sprintf("%s=%s", field, raw),
				etcdRemediationOwner,
			)
		case forbidUnspecified && isUnspecified(host):
			return newViolation(
				"etcd.loopback_forbidden",
				SeverityCritical,
				fmt.Sprintf("%s advertises the unspecified address (%s) — an etcd member must advertise a concrete routable address", field, host),
				fmt.Sprintf("%s=%s", field, raw),
				etcdRemediationOwner,
			)
		}
	}
	return nil
}

// attestEtcdName requires a non-empty member name (etcd refuses to start without).
func attestEtcdName(rendered *EtcdRenderedConfig) *cluster_controllerpb.InfraViolation {
	if rendered.Name == "" {
		return newViolation(
			"etcd.config_valid",
			SeverityError,
			"name is empty — etcd cannot identify this member in the cluster",
			"name=\"\"",
			etcdRemediationOwner,
		)
	}
	return nil
}

// attestEtcdInitialClusterToken requires a non-empty token and, when desired is
// known, that it matches — a mismatched token forms a parallel cluster.
func attestEtcdInitialClusterToken(desired *InfraDesiredState, rendered *EtcdRenderedConfig) *cluster_controllerpb.InfraViolation {
	if rendered.InitialClusterToken == "" {
		return newViolation(
			"etcd.config_valid",
			SeverityError,
			"initial-cluster-token is empty — members with no shared token will not form one cluster",
			"initial-cluster-token=\"\"",
			etcdRemediationOwner,
		)
	}
	if desired != nil && desired.ExpectedClusterName != "" && rendered.InitialClusterToken != desired.ExpectedClusterName {
		return newViolation(
			"etcd.config_valid",
			SeverityError,
			fmt.Sprintf("initial-cluster-token %q does not match desired %q — a mismatched token forms a separate cluster", rendered.InitialClusterToken, desired.ExpectedClusterName),
			fmt.Sprintf("rendered=%q desired=%q", rendered.InitialClusterToken, desired.ExpectedClusterName),
			etcdRemediationOwner,
		)
	}
	return nil
}

// attestEtcdSelfInInitialCluster verifies this member appears in its own
// initial-cluster with a non-loopback peer URL. The renderer skips a node whose
// IP is loopback/empty, so a self-absent initial-cluster means the node was
// rendered without a routable identity and will never join.
func attestEtcdSelfInInitialCluster(desired *InfraDesiredState, rendered *EtcdRenderedConfig) *cluster_controllerpb.InfraViolation {
	if len(rendered.InitialCluster) == 0 {
		return newViolation(
			"etcd.config_valid",
			SeverityError,
			"initial-cluster is empty — etcd has no membership to bootstrap or join",
			"initial-cluster=\"\"",
			etcdRemediationOwner,
		)
	}
	if rendered.Name == "" {
		return nil // name emptiness reported by attestEtcdName
	}
	peerURL, ok := rendered.InitialCluster[rendered.Name]
	if !ok {
		return newViolation(
			"etcd.config_valid",
			SeverityError,
			fmt.Sprintf("member name %q is not present in its own initial-cluster (%s) — the node was rendered without a routable identity and cannot join", rendered.Name, strings.Join(rendered.InitialClusterNames, ",")),
			fmt.Sprintf("name=%s initial-cluster=%s", rendered.Name, strings.Join(rendered.InitialClusterNames, ",")),
			etcdRemediationOwner,
		)
	}
	host := hostFromURL(peerURL)
	if host != "" && (isLoopback(host) || isUnspecified(host)) {
		return newViolation(
			"etcd.config_valid",
			SeverityError,
			fmt.Sprintf("member %q advertises a non-routable peer URL in initial-cluster (%s)", rendered.Name, peerURL),
			fmt.Sprintf("initial-cluster[%s]=%s", rendered.Name, peerURL),
			etcdRemediationOwner,
		)
	}
	// When we know this node's expected address, the self peer URL must match it.
	if desired != nil && host != "" && len(desired.ExpectedListenAddresses) > 0 {
		matched := false
		for _, exp := range desired.ExpectedListenAddresses {
			if stripQuotes(exp) == host {
				matched = true
				break
			}
		}
		if !matched {
			return newViolation(
				"etcd.config_valid",
				SeverityError,
				fmt.Sprintf("member %q peer URL host %s is not one of this node's expected addresses %s — the node would advertise the wrong identity", rendered.Name, host, strings.Join(desired.ExpectedListenAddresses, ",")),
				fmt.Sprintf("self_peer_host=%s expected=%s", host, strings.Join(desired.ExpectedListenAddresses, ",")),
				etcdRemediationOwner,
			)
		}
	}
	return nil
}

// attestEtcdInitialClusterMembers checks the membership against bootstrap intent.
// A joining node whose initial-cluster contains only itself will bootstrap an
// isolated single-member cluster instead of joining the existing ring.
func attestEtcdInitialClusterMembers(desired *InfraDesiredState, rendered *EtcdRenderedConfig) *cluster_controllerpb.InfraViolation {
	if desired == nil || desired.BootstrapIntent != BootstrapJoining {
		return nil
	}
	if len(rendered.InitialCluster) == 0 {
		return nil // emptiness reported by attestEtcdSelfInInitialCluster
	}
	self := ""
	if len(desired.ExpectedListenAddresses) > 0 {
		self = stripQuotes(desired.ExpectedListenAddresses[0])
	}
	for _, peerURL := range rendered.InitialCluster {
		if host := hostFromURL(peerURL); host != "" && host != self {
			return nil // at least one non-self member — good
		}
	}
	return newViolation(
		"etcd.config_valid",
		SeverityError,
		fmt.Sprintf("initial-cluster contains only this node (%s) but it is a joining node — it will bootstrap an isolated single-member cluster instead of joining the ring", self),
		fmt.Sprintf("initial-cluster=%s bootstrap_intent=%s", strings.Join(rendered.InitialClusterNames, ","), desired.BootstrapIntent),
		etcdRemediationOwner,
	)
}

// attestEtcdPeerTLS requires the peer transport security material. Without a
// peer trusted-ca-file, members cannot authenticate each other and replication
// fails; without cert/key the member cannot serve peer TLS at all.
func attestEtcdPeerTLS(rendered *EtcdRenderedConfig) *cluster_controllerpb.InfraViolation {
	var missing []string
	if rendered.PeerCertFile == "" {
		missing = append(missing, "cert-file")
	}
	if rendered.PeerKeyFile == "" {
		missing = append(missing, "key-file")
	}
	if rendered.PeerTrustedCA == "" {
		missing = append(missing, "trusted-ca-file")
	}
	if len(missing) == 0 {
		return nil
	}
	return newViolation(
		"etcd.config_valid",
		SeverityError,
		fmt.Sprintf("peer-transport-security is missing %s — etcd members cannot authenticate each other over mTLS and replication will fail", strings.Join(missing, ", ")),
		fmt.Sprintf("peer_tls_missing=%s", strings.Join(missing, ",")),
		etcdRemediationOwner,
	)
}
