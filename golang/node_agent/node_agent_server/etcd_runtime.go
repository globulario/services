package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/node_agent/node_agent_server/infra_truth"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// etcdObserverDialTimeout bounds the connection to the LOCAL etcd member so the
// observer can never become a new availability risk. Each native-API call gets
// its own sub-timeout from the parent probe context.
const etcdObserverDialTimeout = 1200 * time.Millisecond

// observeEtcdRuntime is the production EtcdRuntimeObserver for the infra truth
// plane. It dials ONLY the local member's advertised client URL (never the shared
// multi-member singleton) so the observed truth is THIS node's member, not a
// remote endpoint the client load-balancer happened to pick — the same
// single-host-pinning lesson as the gocql split in INC-2026-0011. All native-API
// failures are recorded as evidence on the returned state; the probe never aborts.
func observeEtcdRuntime(ctx context.Context, localClientURL string) *infra_truth.EtcdRuntimeState {
	rt := &infra_truth.EtcdRuntimeState{}

	endpoint := etcdHostPort(localClientURL)
	if endpoint == "" {
		rt.Errors = append(rt.Errors, fmt.Sprintf("could not derive host:port from local client URL %q", localClientURL))
		return rt
	}

	// TLS is mandatory for all etcd connections (the config package owns the
	// canonical client TLS material).
	tlsCfg, err := config.GetEtcdTLS()
	if err != nil {
		rt.Errors = append(rt.Errors, fmt.Sprintf("etcd TLS unavailable: %v", err))
		return rt
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{endpoint},
		DialTimeout: etcdObserverDialTimeout,
		TLS:         tlsCfg,
		Context:     ctx,
	})
	if err != nil {
		rt.Errors = append(rt.Errors, fmt.Sprintf("etcd client dial %s: %v", endpoint, err))
		return rt
	}
	defer func() { _ = cli.Close() }()

	// Endpoint status — proves the local member answers and yields leader/term/db.
	statusCtx, cancel := context.WithTimeout(ctx, etcdObserverDialTimeout)
	st, err := cli.Status(statusCtx, endpoint)
	cancel()
	if err != nil {
		rt.Errors = append(rt.Errors, fmt.Sprintf("etcd status %s: %v", endpoint, err))
		// Without status we still try member list / alarms below for evidence.
	} else {
		rt.LocalReachable = true
		rt.Version = st.Version
		rt.DBSizeBytes = st.DbSize
		rt.RaftTerm = st.RaftTerm
		rt.IsLearner = st.IsLearner
		rt.HasLeader = st.Leader != 0
		if st.Header != nil {
			rt.MemberID = fmt.Sprintf("%x", st.Header.MemberId)
			rt.IsLeader = st.Leader != 0 && st.Leader == st.Header.MemberId
		}
		if st.Leader != 0 {
			rt.LeaderID = fmt.Sprintf("%x", st.Leader)
		}
		// etcd surfaces alarms inline on the status Errors slice too.
		for _, e := range st.Errors {
			if a := normalizeEtcdAlarm(e); a != "" {
				rt.Alarms = appendUnique(rt.Alarms, a)
			}
		}
	}

	// Member list — observed membership (cluster-facing peer hosts).
	mlCtx, cancel := context.WithTimeout(ctx, etcdObserverDialTimeout)
	ml, err := cli.MemberList(mlCtx)
	cancel()
	if err != nil {
		rt.Errors = append(rt.Errors, fmt.Sprintf("etcd member list: %v", err))
	} else {
		rt.MemberCount = len(ml.Members)
		for _, m := range ml.Members {
			for _, pu := range m.PeerURLs {
				if h := etcdHost(pu); h != "" {
					rt.ObservedPeers = appendUnique(rt.ObservedPeers, h)
				}
			}
		}
	}

	// Alarm list — authoritative NOSPACE/CORRUPT signal.
	alCtx, cancel := context.WithTimeout(ctx, etcdObserverDialTimeout)
	al, err := cli.AlarmList(alCtx)
	cancel()
	if err != nil {
		rt.Errors = append(rt.Errors, fmt.Sprintf("etcd alarm list: %v", err))
	} else {
		for _, a := range al.Alarms {
			switch a.Alarm {
			case etcdserverpb.AlarmType_NOSPACE:
				rt.Alarms = appendUnique(rt.Alarms, "NOSPACE")
			case etcdserverpb.AlarmType_CORRUPT:
				rt.Alarms = appendUnique(rt.Alarms, "CORRUPT")
			}
		}
	}

	return rt
}

// etcdHostPort returns the host:port for a client URL like
// "https://10.0.0.63:2379", defaulting the port to 2379.
func etcdHostPort(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
		port := u.Port()
		if port == "" {
			port = "2379"
		}
		return net.JoinHostPort(u.Hostname(), port)
	}
	return ""
}

// etcdHost returns the bare host of a URL.
func etcdHost(raw string) string {
	if u, err := url.Parse(strings.TrimSpace(raw)); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return ""
}

// normalizeEtcdAlarm maps an inline status error string to a known alarm token.
func normalizeEtcdAlarm(s string) string {
	up := strings.ToUpper(s)
	switch {
	case strings.Contains(up, "NOSPACE"):
		return "NOSPACE"
	case strings.Contains(up, "CORRUPT"):
		return "CORRUPT"
	default:
		return ""
	}
}

func appendUnique(in []string, v string) []string {
	for _, e := range in {
		if e == v {
			return in
		}
	}
	return append(in, v)
}
