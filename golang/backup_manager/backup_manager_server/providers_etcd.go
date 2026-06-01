package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// errNoHealthyEtcdEndpoint is returned by selectEtcdSnapshotEndpoint when no
// configured endpoint passes the health probe.
var errNoHealthyEtcdEndpoint = errors.New("etcd: no healthy endpoint in configured list")

// etcdHealthProber is the function used to check whether a single etcd
// endpoint is reachable + healthy. Defined as a package-level variable so
// tests can replace it without spawning real subprocesses.
//
// Returns nil when the endpoint is healthy, otherwise a non-nil error.
var etcdHealthProber = realEtcdctlHealthProbe

// localHostMatchersFn returns the set of identifiers (hostname, FQDN, local
// IPv4s) used to decide whether an endpoint refers to this node. Defined as
// a package-level variable so tests can pin a deterministic local set
// independent of the host they run on.
var localHostMatchersFn = localHostMatchers

// realEtcdctlHealthProbe shells out to `etcdctl endpoint health --endpoints
// <one>` with the supplied TLS material. A short per-probe timeout keeps the
// selection step bounded even when an endpoint is down. Production default.
func realEtcdctlHealthProbe(ctx context.Context, endpoint, cacert, cert, key string) error {
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	args := []string{"endpoint", "health", "--endpoints", endpoint, "--command-timeout", "2s"}
	if fileExists(cacert) {
		args = append(args, "--cacert", cacert)
	}
	if fileExists(cert) {
		args = append(args, "--cert", cert)
	}
	if fileExists(key) {
		args = append(args, "--key", key)
	}
	cmd := exec.CommandContext(probeCtx, "etcdctl", args...)
	cmd.Env = append(os.Environ(), "ETCDCTL_API=3")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("etcdctl endpoint health %s: %v: %s",
			endpoint, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// splitEndpoints parses a comma-separated etcd endpoints string into a
// trimmed, non-empty slice. Whitespace and empty entries are dropped. Order
// is preserved.
func splitEndpoints(csv string) []string {
	if csv = strings.TrimSpace(csv); csv == "" {
		return nil
	}
	out := make([]string, 0, 2)
	for _, raw := range strings.Split(csv, ",") {
		if v := strings.TrimSpace(raw); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// localHostMatchers returns the set of strings that identify this host:
// short hostname, FQDN, and every non-loopback IPv4 from local interfaces.
// Used to compare against an endpoint host to decide locality.
func localHostMatchers() map[string]struct{} {
	matchers := map[string]struct{}{}
	if h, err := os.Hostname(); err == nil && h != "" {
		matchers[strings.ToLower(h)] = struct{}{}
		// Short hostname (drop domain) for the common FQDN case.
		if i := strings.IndexByte(h, '.'); i > 0 {
			matchers[strings.ToLower(h[:i])] = struct{}{}
		}
	}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() {
				continue
			}
			if v4 := ipnet.IP.To4(); v4 != nil {
				matchers[v4.String()] = struct{}{}
			}
		}
	}
	return matchers
}

// endpointHostMatchesLocal returns true when ep's host portion matches one of
// the supplied local matchers. Accepts URL forms (https://host:port) and bare
// host:port forms. Falls back to the short hostname (segment before the first
// dot) when the full host is not in the local set — covers the case where an
// endpoint URL uses a different domain than the host's own FQDN but the short
// name is still the same node (e.g. endpoint says globule-nuc.example.com,
// local hostname is globule-nuc.globular.internal).
func endpointHostMatchesLocal(ep string, localSet map[string]struct{}) bool {
	host := extractEndpointHost(ep)
	if host == "" {
		return false
	}
	if _, ok := localSet[host]; ok {
		return true
	}
	if i := strings.IndexByte(host, '.'); i > 0 {
		if _, ok := localSet[host[:i]]; ok {
			return true
		}
	}
	return false
}

// extractEndpointHost pulls the bare host from an endpoint that may be a URL
// (https://host:port/...) or a bare authority (host:port). Returns the lower-
// cased host without the port. Returns "" when nothing parseable is found.
func extractEndpointHost(ep string) string {
	ep = strings.TrimSpace(ep)
	if ep == "" {
		return ""
	}
	if strings.Contains(ep, "://") {
		if u, err := url.Parse(ep); err == nil && u.Host != "" {
			if h, _, err := net.SplitHostPort(u.Host); err == nil {
				return strings.ToLower(h)
			}
			return strings.ToLower(u.Host)
		}
	}
	if h, _, err := net.SplitHostPort(ep); err == nil {
		return strings.ToLower(h)
	}
	return strings.ToLower(ep)
}

// reorderLocalFirst returns a copy of eps with endpoints whose host matches a
// local matcher moved to the front, preserving relative order otherwise.
// Stable so two local endpoints retain their original ordering.
func reorderLocalFirst(eps []string, localSet map[string]struct{}) []string {
	out := make([]string, 0, len(eps))
	var nonLocal []string
	for _, ep := range eps {
		if endpointHostMatchesLocal(ep, localSet) {
			out = append(out, ep)
		} else {
			nonLocal = append(nonLocal, ep)
		}
	}
	return append(out, nonLocal...)
}

// selectEtcdSnapshotEndpoint picks exactly one healthy endpoint from a
// comma-separated configured list. Preference order:
//  1. an endpoint whose host matches a local IP / hostname (lower latency,
//     and snapshots from the local member are simpler to reason about);
//  2. otherwise the first endpoint that passes etcdctl endpoint health.
//
// Returns the chosen endpoint or errNoHealthyEtcdEndpoint when nothing is
// healthy. Existence of this helper is the single point that enforces
// "snapshot save receives exactly one endpoint" — it returns a string with
// no embedded commas, by construction.
//
// History: etcdctl snapshot save rejects multi-endpoint args with
// "snapshot must be requested to one selected node, not multiple". The
// earlier implementation passed the full CSV directly, breaking every
// backup against a multi-member etcd cluster.
func (srv *server) selectEtcdSnapshotEndpoint(ctx context.Context, csv, cacert, cert, key string) (string, error) {
	eps := splitEndpoints(csv)
	if len(eps) == 0 {
		return "", fmt.Errorf("etcd: no endpoints configured")
	}
	localSet := localHostMatchersFn()
	ordered := reorderLocalFirst(eps, localSet)
	var lastErr error
	for _, ep := range ordered {
		if err := etcdHealthProber(ctx, ep, cacert, cert, key); err != nil {
			slog.Info("etcd: endpoint unhealthy, trying next",
				"endpoint", ep, "error", err.Error())
			lastErr = err
			continue
		}
		// Log enough to debug a selection but not the full configured CSV —
		// the CSV is already persisted in outputs["endpoints_full"] by the
		// caller for capsule-level forensics.
		slog.Info("etcd: selected snapshot endpoint",
			"endpoint", ep,
			"configured_count", len(eps),
			"matched_local", endpointHostMatchesLocal(ep, localSet))
		return ep, nil
	}
	if lastErr == nil {
		lastErr = errNoHealthyEtcdEndpoint
	}
	return "", fmt.Errorf("%w (tried %d endpoint(s); last error: %v)",
		errNoHealthyEtcdEndpoint, len(ordered), lastErr)
}
