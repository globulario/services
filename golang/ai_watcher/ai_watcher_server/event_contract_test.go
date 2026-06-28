package main

// Event-topic contract gate.
//
// The cluster controller emits operational events; ai-watcher subscribes to
// topics and matches them against rules. Historically these two vocabularies
// were maintained independently and drifted apart — only service.failed /
// service.exited survived BOTH the subscription filter and a rule, so the
// ai-memory timeline was reduced to "a service crashed" and lost every
// reconcile/topology/quorum DECISION the controller emits.
//
// This test makes the two halves a single drift-gated contract: it extracts the
// event names the controller actually emits (from source — the single producer
// authority, not a hand-copied list) and asserts every operational one is
// consumable by (a) a SubscribeTopic AND (b) a Rule in defaultWatcherConfig.
//
// When the controller adds a new emitClusterEvent("...") topic, this test fails
// until it is either given a rule+subscription here or explicitly classified as
// non-timeline (success/steady-state/telemetry) in nonTimelineEvents with a
// justification. That is the contract: no emit topic is silently dropped.

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// controllerSrcDir is the controller server package relative to this package.
const controllerSrcDir = "../../cluster_controller/cluster_controller_server"

var emitLiteralRe = regexp.MustCompile(`emitClusterEvent\("([a-zA-Z0-9._]+)"`)

// knownDynamicEmits are operational events emitted via a variable name (the
// static regex cannot see them) but whose literal values are pinned in source.
// Keep this in sync with the non-literal emitClusterEvent call sites.
var knownDynamicEmits = map[string]string{
	"controller.invariant.disk_pressure_warn":     "workflow_invariant.go — level := \"...disk_pressure_warn\"",
	"controller.invariant.disk_pressure_critical": "workflow_invariant.go — level := \"...disk_pressure_critical\"",
	"service.exited":  "handlers_status.go — eventName := \"service.exited\"",
	"service.stopped": "handlers_status.go — eventName := \"service.stopped\"",
}

// nonTimelineEvents are controller-emitted topics intentionally NOT routed to
// the ai-watcher timeline. Each MUST carry a justification. These are success,
// recovery, steady-state, or telemetry signals — not incidents. Adding an entry
// here is a deliberate, reviewable decision; it is the only sanctioned way to
// leave a controller emit topic unconsumed.
var nonTimelineEvents = map[string]string{
	"cluster.dns_reconciled":                     "routine success",
	"cluster.health.recovered":                   "recovery (positive) — cluster.health.degraded is the incident",
	"cluster.reconcile.clean":                    "steady-state success",
	"cluster.reconcile.completed":                "terminal success",
	"cluster.reconcile.finalized":                "terminal success",
	"controller.invariant_enforcement_completed": "success — invariant_enforcement_failed is the incident",
	"controller.invariant_enforcement_report":    "periodic informational report",
	"controller.invariant.node_partition_healed": "recovery (positive) — node_partitioned is the incident",
	"controller.leader_elected":                  "routine control-plane lifecycle",
	"controller.self_update":                     "lifecycle info — self_update_apply_failed is the incident",
	"controller.self_update_pending":             "lifecycle info",
	"controller.workflows_repaired":              "recovery (positive)",
	"node.bootstrap_phase_changed":               "lifecycle info — reconcile.topology_blocked is the stuck signal",
	"node.recovery.complete":                     "recovery (positive)",
	"node.recovery.reprovision_acked":            "lifecycle info",
	"operation.restart_completed":                "success",
	"posture.gate_suppressed":                    "expected safety behavior, not an incident",
	"service.phase_changed":                      "lifecycle info",
	"service.restart_attempted":                  "lifecycle info — service.restart_failed is the incident",
	"service.restart_skipped":                    "lifecycle info",
	"workflow.backend_pressure":                  "throughput/backpressure telemetry — metrics domain, not an incident",
}

// subscriptionMatches mirrors event_server.matchesChannel (topic ".*"-suffix
// prefix matching). Kept in sync with that canonical implementation.
func subscriptionMatches(pattern, name string) bool {
	if pattern == name {
		return true
	}
	if strings.HasSuffix(pattern, ".*") {
		return strings.HasPrefix(name, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == "*"
}

func collectControllerEmits(t *testing.T) map[string]string {
	t.Helper()
	info, err := os.Stat(controllerSrcDir)
	if err != nil || !info.IsDir() {
		t.Skipf("controller source dir %q not found — contract gate runs in-repo only", controllerSrcDir)
	}
	entries, err := os.ReadDir(controllerSrcDir)
	if err != nil {
		t.Fatalf("read controller dir: %v", err)
	}
	emits := map[string]string{}
	for name, src := range knownDynamicEmits {
		emits[name] = src
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(controllerSrcDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		for _, m := range emitLiteralRe.FindAllStringSubmatch(string(b), -1) {
			if _, ok := emits[m[1]]; !ok {
				emits[m[1]] = e.Name()
			}
		}
	}
	if len(emits) == 0 {
		t.Fatal("no emitClusterEvent topics found — regex or source layout changed")
	}
	return emits
}

func consumable(cfg interface {
	GetSubscribeTopics() []string
}, rulePatterns []string, name string) (sub bool, rule bool) {
	for _, topic := range cfg.GetSubscribeTopics() {
		if subscriptionMatches(topic, name) {
			sub = true
			break
		}
	}
	for _, p := range rulePatterns {
		if matchPattern(p, name) {
			rule = true
			break
		}
	}
	return
}

// TestEventTopicContract is the drift gate: every controller-emitted operational
// topic must be consumable (subscription + rule) or explicitly allow-listed.
func TestEventTopicContract(t *testing.T) {
	cfg := defaultWatcherConfig()
	var rulePatterns []string
	for _, r := range cfg.GetRules() {
		rulePatterns = append(rulePatterns, r.GetEventPattern())
	}

	emits := collectControllerEmits(t)

	var uncovered []string
	for name := range emits {
		sub, rule := consumable(cfg, rulePatterns, name)
		_, allowed := nonTimelineEvents[name]
		if sub && rule {
			continue // consumed → reaches the timeline
		}
		if allowed {
			continue // intentionally not a timeline incident
		}
		reason := "no rule"
		if !sub {
			reason = "no subscription"
			if !rule {
				reason = "no subscription and no rule"
			}
		}
		uncovered = append(uncovered, name+" ("+reason+", emitted in "+emits[name]+")")
	}
	if len(uncovered) > 0 {
		sort.Strings(uncovered)
		t.Fatalf("controller emits %d topic(s) that ai-watcher cannot route to the timeline; add a rule (and subscription) or classify in nonTimelineEvents:\n  %s",
			len(uncovered), strings.Join(uncovered, "\n  "))
	}

	// Guard against ignore-list rot: every allow-listed name must still be emitted.
	var stale []string
	for name := range nonTimelineEvents {
		if _, ok := emits[name]; !ok {
			stale = append(stale, name)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Fatalf("nonTimelineEvents has %d stale entry/entries no longer emitted by the controller; remove them:\n  %s",
			len(stale), strings.Join(stale, "\n  "))
	}
}

// TestKeyDecisionEventsReachTimeline pins the specific signals this slice exists
// to recover: a node stuck on a topology gate, a blocked service, a quorum
// enforcement — these must become timeline incidents, not be silently dropped.
func TestKeyDecisionEventsReachTimeline(t *testing.T) {
	cfg := defaultWatcherConfig()
	var rulePatterns []string
	for _, r := range cfg.GetRules() {
		rulePatterns = append(rulePatterns, r.GetEventPattern())
	}
	mustReach := []string{
		"controller.storage_quorum_enforced",
		"cluster.reconcile.topology_blocked",
		"cluster.reconcile.item_failed",
		"service.blocked",
		"cluster.plan_blocked",
		"desired.kind_mismatch",
		"cluster.drift_detected",
	}
	for _, name := range mustReach {
		sub, rule := consumable(cfg, rulePatterns, name)
		if !sub || !rule {
			t.Errorf("%s must reach the timeline but sub=%v rule=%v", name, sub, rule)
		}
	}
}
