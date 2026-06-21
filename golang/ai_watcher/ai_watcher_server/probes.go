package main

// Runtime Vigilance Probes (PR-14).
//
// The watcher's event pipeline only sees *named events* someone already chose to
// publish. It cannot see a runtime condition class that nobody emits an event
// for — which is exactly how a ScyllaDB group0 quorum loss went undetected.
//
// A probe is a governed health check that runs on a fixed interval, sources the
// *owning authority's* truth (e.g. the infra-probe truth plane), and emits a
// structured DIAGNOSTIC_CLAIM finding into behavioral-memory when it sees a bad
// condition class. Probes never run repairs and never auto-promote: they widen
// the governed afferent path, nothing more.

import (
	"context"
	"fmt"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	"github.com/globulario/services/golang/security"
)

const (
	// probeInterval is how often the vigilance probes run.
	probeInterval = 60 * time.Second
	// probeStartupDelay lets the service and its dependencies settle before the
	// first probe round, so startup transients are not reported as findings.
	probeStartupDelay = 20 * time.Second
	// probeDedupWindow bounds emission of a persistent condition so a stuck
	// failure does not flood behavioral-memory
	// (meta.diagnostic_output_must_be_bounded).
	probeDedupWindow = 5 * time.Minute
	// probeRunTimeout caps a single probe run.
	probeRunTimeout = 15 * time.Second
)

// ProbeResult is the structured outcome of one runtime vigilance probe run.
// Healthy results are not emitted. Unhealthy or indeterminate results become a
// DIAGNOSTIC_CLAIM finding in behavioral-memory (never auto-promoted, never a
// repair). The shape mirrors observation.WatcherProbeFinding.
type ProbeResult struct {
	ProbeName          string
	Component          string
	Healthy            bool
	Indeterminate      bool // probe could not reach its data source; failure is itself evidence
	Condition          string
	Observed           string
	Expected           string
	Severity           string
	Evidence           []string
	CandidateInvariant string
	RecommendedProbe   string
	EntityRef          string
	TruthPlaneRef      string
	ObservedAtUnix     int64
}

// Probe is one governed runtime health check.
type Probe interface {
	Name() string
	Component() string
	Run(ctx context.Context) ProbeResult
}

// runtimeProbes returns the set of vigilance probes this watcher runs. Add new
// probes here (etcd, envoy, minio, grpc-reflection, rbac, config-freshness …);
// each one is a small struct with an injectable acquire func for testing.
func (srv *server) runtimeProbes() []Probe {
	return []Probe{
		newScyllaGroup0Probe(srv),
	}
}

// startProbeLoop runs the runtime vigilance probes on a fixed interval. It is
// launched as a goroutine from StartService alongside the event loop.
func (srv *server) startProbeLoop() {
	time.Sleep(probeStartupDelay)
	probes := srv.runtimeProbes()
	logger.Info("runtime vigilance probes started",
		"count", len(probes), "interval", probeInterval.String())

	ticker := time.NewTicker(probeInterval)
	defer ticker.Stop()
	for {
		srv.configMu.RLock()
		enabled := srv.config.GetEnabled() && !srv.config.GetPaused()
		srv.configMu.RUnlock()
		if enabled {
			srv.runProbesOnce(context.Background(), probes)
		}
		<-ticker.C
	}
}

// runProbesOnce runs every probe once, isolating panics so one bad probe cannot
// take down the loop.
func (srv *server) runProbesOnce(ctx context.Context, probes []Probe) {
	for _, p := range probes {
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("runtime probe panicked", "probe", p.Name(), "recover", fmt.Sprintf("%v", r))
				}
			}()
			pctx, cancel := context.WithTimeout(ctx, probeRunTimeout)
			defer cancel()
			res := p.Run(pctx)
			if res.Healthy {
				return
			}
			srv.handleProbeFinding(ctx, res)
		}()
	}
}

// probeDedupKey identifies a recurring condition for rate-limiting.
func probeDedupKey(res ProbeResult) string {
	return res.Component + "|" + res.Condition + "|" + res.Severity
}

// handleProbeFinding rate-limits and emits an unhealthy probe result.
func (srv *server) handleProbeFinding(ctx context.Context, res ProbeResult) {
	key := probeDedupKey(res)
	now := time.Now()
	srv.lastProbeMu.Lock()
	last, seen := srv.lastProbe[key]
	if seen && now.Sub(last) < probeDedupWindow {
		srv.lastProbeMu.Unlock()
		return
	}
	srv.lastProbe[key] = now
	srv.lastProbeMu.Unlock()

	logger.Warn("runtime probe finding",
		"probe", res.ProbeName,
		"component", res.Component,
		"condition", res.Condition,
		"severity", res.Severity,
		"indeterminate", res.Indeterminate,
		"observed", res.Observed,
	)
	emitBehavioralProbeFinding(ctx, srv, res)
}

// emitBehavioralProbeFinding records a probe finding into behavioral-memory as a
// governed DIAGNOSTIC_CLAIM observation. It is best-effort and is a package var
// so tests can intercept it.
var emitBehavioralProbeFinding = func(ctx context.Context, srv *server, res ProbeResult) {
	clusterID, _ := security.GetLocalClusterID()
	observedAt := res.ObservedAtUnix
	if observedAt == 0 {
		observedAt = time.Now().Unix()
	}
	finding := observation.WatcherProbeFinding{
		Source:             "ai_watcher",
		ProbeName:          res.ProbeName,
		Component:          res.Component,
		Condition:          res.Condition,
		Observed:           res.Observed,
		Expected:           res.Expected,
		Severity:           res.Severity,
		Evidence:           res.Evidence,
		CandidateInvariant: res.CandidateInvariant,
		RecommendedProbe:   res.RecommendedProbe,
		EntityRef:          res.EntityRef,
		ClusterID:          clusterID,
		TruthPlaneRef:      res.TruthPlaneRef,
		ObservedAtUnix:     observedAt,
	}
	bundle := observation.FromWatcherProbe(behavioralProject, api.DomainRef(behavioralDomain), finding)
	if err := observation.RecordBundle(ctx, bundle); err != nil {
		logger.Debug("behavioral observation: probe finding skipped",
			"probe", res.ProbeName, "condition", res.Condition, "err", err)
	}
}
