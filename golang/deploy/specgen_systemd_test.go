package deploy

import (
	"regexp"
	"strings"
	"testing"
)

// Background
// ----------
// The systemd unit body in specgen.go (template at top of file) historically
// emitted two `Type=` directives: a "simple" line at the top of the [Service]
// block, then a stray `Type=notify` further down — left over from an earlier
// attempt to add sd_notify-based readiness without removing the original.
// systemd takes "last directive wins", so units effectively ran as
// Type=notify. During self-upgrade the new binary forks during startup and
// sends READY=1 from a child PID that systemd does not recognise as the main
// PID, producing log spam like:
//
//   globular-node-agent.service: Got notification message from PID X,
//                                 but reception only permitted for main PID Y
//
// The hand-off then deadlocks: the old binary is never replaced and the
// running service keeps the previous version even though the package
// inventory says the upgrade succeeded. Observed cluster-wide on
// 2026-05-20 during the node-agent 1.2.63 rollout: 4 of 5 nodes were
// wedged this way; only the one where the timing happened to be lenient
// completed the swap.
//
// Decision: drop the duplicate. Keep Type=simple. Remove WatchdogSec=60
// (which is meaningless without Type=notify). The sd_notify calls in
// node_agent main.go become no-ops under Type=simple, which is harmless.
// Reliability of `systemctl restart` is more important than watchdog/notify
// semantics that we did not actually depend on.
//
// These tests pin the invariant so the regression cannot return silently.

// extractServiceSection returns the [Service] section of a generated unit
// content embedded inside a spec YAML. The template indents the unit body
// by 10 spaces under `content: |`, so we strip that for substring matching.
func extractServiceSection(t *testing.T, spec string) string {
	t.Helper()
	lines := strings.Split(spec, "\n")
	var out []string
	inService := false
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if strings.HasPrefix(trimmed, "[Service]") {
			inService = true
			continue
		}
		if inService && strings.HasPrefix(trimmed, "[") && trimmed != "[Service]" {
			break // entered [Install] or similar
		}
		if inService {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		t.Fatalf("could not locate [Service] section in spec; spec body=\n%s", spec)
	}
	return strings.Join(out, "\n")
}

// TestGenerateSpec_SystemdUnit_HasExactlyOneTypeDirective is the load-bearing
// test for this regression. It scans the [Service] section of the generated
// unit for any line beginning with `Type=` and asserts there is exactly one.
// This is what was broken before: two Type= lines in the same section.
func TestGenerateSpec_SystemdUnit_HasExactlyOneTypeDirective(t *testing.T) {
	catalogPath := findTestCatalog(t)
	cat, err := LoadCatalog(catalogPath)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	// Cover a representative cross-section: a minimal service (echo), a
	// scylla-dependent service (dns), a cap-net-bind service (dns again),
	// the affected node_agent itself, and a service touched by the
	// in-flight backup-manager work.
	services := []string{"echo", "dns", "node_agent", "backup_manager", "workflow", "rbac"}

	typeRE := regexp.MustCompile(`(?m)^Type=`)

	for _, svc := range services {
		t.Run(svc, func(t *testing.T) {
			entry, err := cat.Get(svc)
			if err != nil {
				t.Fatalf("catalog.Get(%s): %v", svc, err)
			}
			spec, err := GenerateSpec(entry)
			if err != nil {
				t.Fatalf("GenerateSpec(%s): %v", svc, err)
			}
			service := extractServiceSection(t, spec)
			matches := typeRE.FindAllString(service, -1)
			if len(matches) != 1 {
				t.Errorf("[%s] [Service] section has %d Type= directives, want exactly 1\n--- begin section ---\n%s\n--- end section ---",
					svc, len(matches), service)
			}
		})
	}
}

// TestGenerateSpec_SystemdUnit_TypeIsIntentionallySimple pins the chosen
// Type= value. If a future change wants Type=notify (or Type=exec or
// Type=forking), it must update this test deliberately — and at the same
// time ensure the unit emits no second `Type=` line (HasExactlyOneType…
// guards that). The pairing makes "silently re-introduce duplicate Type=
// by adding the new without removing the old" impossible.
func TestGenerateSpec_SystemdUnit_TypeIsIntentionallySimple(t *testing.T) {
	catalogPath := findTestCatalog(t)
	cat, err := LoadCatalog(catalogPath)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	entry, err := cat.Get("node_agent")
	if err != nil {
		t.Fatalf("catalog.Get(node_agent): %v", err)
	}
	spec, err := GenerateSpec(entry)
	if err != nil {
		t.Fatalf("GenerateSpec(node_agent): %v", err)
	}
	service := extractServiceSection(t, spec)

	if !strings.Contains(service, "Type=simple") {
		t.Errorf("[Service] section must contain `Type=simple` (intentional choice — see file header):\n%s", service)
	}
}

// TestGenerateSpec_SystemdUnit_NoWatchdogSec confirms the watchdog directive
// was removed alongside Type=notify. With Type=simple, WatchdogSec is a
// configuration error (systemd cannot deliver WATCHDOG=1 to a Type=simple
// unit), so its presence would re-create the same risk surface that
// originally allowed Type=notify to be added by accident.
func TestGenerateSpec_SystemdUnit_NoWatchdogSec(t *testing.T) {
	catalogPath := findTestCatalog(t)
	cat, err := LoadCatalog(catalogPath)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	for _, svc := range []string{"echo", "dns", "node_agent", "backup_manager"} {
		t.Run(svc, func(t *testing.T) {
			entry, err := cat.Get(svc)
			if err != nil {
				t.Fatalf("catalog.Get(%s): %v", svc, err)
			}
			spec, err := GenerateSpec(entry)
			if err != nil {
				t.Fatalf("GenerateSpec(%s): %v", svc, err)
			}
			if strings.Contains(spec, "WatchdogSec=") {
				t.Errorf("[%s] generated unit must not contain WatchdogSec= when Type=simple is used\nspec:\n%s", svc, spec)
			}
		})
	}
}

// TestGenerateSpec_SystemdUnit_NoTypeNotify guards against the specific
// regression we just fixed. If someone re-adds a Type=notify line (e.g. as
// an "improvement" alongside the existing Type=simple), this test catches
// it on its own — independent of the count-based check, which gives a
// clearer failure message.
func TestGenerateSpec_SystemdUnit_NoTypeNotify(t *testing.T) {
	catalogPath := findTestCatalog(t)
	cat, err := LoadCatalog(catalogPath)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	entry, err := cat.Get("node_agent")
	if err != nil {
		t.Fatalf("catalog.Get(node_agent): %v", err)
	}
	spec, err := GenerateSpec(entry)
	if err != nil {
		t.Fatalf("GenerateSpec(node_agent): %v", err)
	}
	service := extractServiceSection(t, spec)
	if strings.Contains(service, "Type=notify") {
		t.Errorf("[Service] section must not contain `Type=notify` (would duplicate Type=simple — see file header):\n%s", service)
	}
}
