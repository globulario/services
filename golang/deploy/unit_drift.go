package deploy

// unit_drift.go — Phase 5b of the Diagnostic Honesty Refactor.
//
// Phase 5a (acdcb436, 2df97ebe) caught duplicate singleton directives at
// generation time — that prevented systemd from silently picking the wrong
// value of two competing Type= lines. Phase 5b answers the next-stronger
// question: given a unit we rendered and a unit systemd actually loaded
// (read via `systemctl show`), do the effective properties agree?
//
// The drift case the brief calls out:
//
//   - Effective Type differs from expected
//   - Effective ExecStart points to an unexpected binary
//   - Installed unit file on disk differs from rendered content
//
// All three are silent failures today: nothing in the convergence loop
// re-reads the unit file or asks systemd what it loaded, so an admin who
// edits /etc/systemd/system/globular-foo.service by hand, or a deploy that
// shipped against a stale spec, can be running indefinitely.
//
// This file does no I/O. It takes the rendered content the controller
// already has and the effective values node-agent already collects via
// GetServiceRuntimeProof (Phase 2), and returns a verdict.
//
// Phase 9 wires the verdict into the verifier/doctor pipeline.

import (
	"fmt"
	"sort"
	"strings"
)

// FindingUnitEffectiveConfigDrift is the failure_modes.yaml id raised by
// DetectEffectiveUnitDrift when at least one effective property disagrees
// with what we rendered.
const FindingUnitEffectiveConfigDrift = "systemd.effective_config_drift"

// EffectiveUnitProperties is the subset of `systemctl show` output the
// drift detector consumes. Field names mirror the proto on
// ServiceRuntimeProof (node_agent.proto) so consumers can construct one
// directly from the RPC reply without intermediate copying.
//
// All fields are strings — systemd emits everything as text and the
// detector compares them with whitespace-trimmed equality, with the one
// exception of ExecStart (see compareExecStart below). An empty effective
// value means "systemd did not return the property" and is treated as a
// reason-for-doubt, not a drift.
type EffectiveUnitProperties struct {
	Type             string
	ExecStart        string
	FragmentPath     string
	ActiveState      string
	SubState         string
	UnitFileSHA256   string // hash of the unit file at FragmentPath as collected by node-agent
}

// UnitDriftVerdict describes the comparison result. Status is "verified"
// when every directive systemd reported matches the rendered intent, "drift"
// when at least one disagrees, and "unknown" when the rendered content
// could not be parsed or the effective bag was empty.
//
// Drifts carries one short line per disagreement, e.g.:
//
//	"Type: rendered=simple, effective=forking"
//	"ExecStart: rendered=/usr/lib/globular/bin/x, effective=/usr/local/bin/x"
type UnitDriftVerdict struct {
	Status    string
	Drifts    []string
	FindingID string
}

const (
	UnitDriftVerified = "verified"
	UnitDriftDrift    = "drift"
	UnitDriftUnknown  = "unknown"
)

// DetectEffectiveUnitDrift compares the rendered unit content against the
// effective properties systemd reported. Returns a verdict whose Status is:
//
//   - "verified": every rendered directive systemd surfaced has a matching
//     effective value.
//   - "drift": at least one directive disagrees. Drifts lists the
//     differences; FindingID is set to systemd.effective_config_drift.
//   - "unknown": we couldn't parse the rendered content, or the effective
//     bag was empty (e.g. systemctl show failed). No drift is asserted
//     because we have nothing to compare.
func DetectEffectiveUnitDrift(rendered string, eff EffectiveUnitProperties) UnitDriftVerdict {
	if strings.TrimSpace(rendered) == "" {
		return UnitDriftVerdict{Status: UnitDriftUnknown, Drifts: []string{"rendered unit content is empty"}}
	}
	directives := ParseSystemdUnit(rendered)
	if len(directives) == 0 {
		return UnitDriftVerdict{Status: UnitDriftUnknown, Drifts: []string{"could not parse rendered unit content"}}
	}

	// If systemd returned nothing useful, we can't claim a drift — only
	// that proof is missing.
	if strings.TrimSpace(eff.Type) == "" &&
		strings.TrimSpace(eff.ExecStart) == "" &&
		strings.TrimSpace(eff.FragmentPath) == "" {
		return UnitDriftVerdict{Status: UnitDriftUnknown, Drifts: []string{"effective unit properties unavailable"}}
	}

	var drifts []string

	// Type=
	if want := getServiceDirective(directives, "Type"); want != "" {
		got := strings.TrimSpace(eff.Type)
		if got != "" && got != want {
			drifts = append(drifts,
				fmt.Sprintf("Type: rendered=%s, effective=%s", want, got))
		}
	}

	// ExecStart= — systemctl show emits a bracketed structure:
	//   { path=/usr/bin/foo ; argv[]=foo a b ; ignore_errors=no ; ... }
	// We extract the path= field and compare it to the first whitespace-
	// separated token of the rendered ExecStart line. That's enough to
	// catch "effective ExecStart points to unexpected binary" — argument
	// drift surfaces separately in the show output but is rare and noisy
	// to compare token-by-token, so we punt that to a future refinement.
	if want := getServiceDirective(directives, "ExecStart"); want != "" {
		wantPath := firstWord(want)
		gotPath := extractExecStartPath(eff.ExecStart)
		if gotPath != "" && wantPath != "" && gotPath != wantPath {
			drifts = append(drifts,
				fmt.Sprintf("ExecStart: rendered=%s, effective=%s", wantPath, gotPath))
		}
	}

	// FragmentPath= — the path systemd is loading from. We don't know the
	// install path here, but if it's empty after we asked for it, that's
	// itself a drift signal (no unit file is present).
	if strings.TrimSpace(eff.FragmentPath) == "" && strings.TrimSpace(eff.UnitFileSHA256) == "" {
		drifts = append(drifts,
			"FragmentPath: systemd has no on-disk unit file for this service")
	}

	if len(drifts) == 0 {
		return UnitDriftVerdict{Status: UnitDriftVerified}
	}
	sort.Strings(drifts) // stable order for tests + audit logs
	return UnitDriftVerdict{
		Status:    UnitDriftDrift,
		Drifts:    drifts,
		FindingID: FindingUnitEffectiveConfigDrift,
	}
}

// ParseSystemdUnit returns directives keyed as "Section.Name" → []value
// (in source order). It's deliberately minimal — section header detection,
// directive line splitting, comment/blank skipping. Multi-value directives
// (ExecStartPre, Environment, After, Wants) appear once per occurrence.
//
// This is the missing extractor the Phase 5b plan calls out: the existing
// ValidateSystemdUnit scans for duplicates but never returns parsed
// values.
func ParseSystemdUnit(content string) map[string][]string {
	out := make(map[string][]string)
	section := ""
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}
		if section == "" {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		name := strings.TrimSpace(line[:eq])
		if !isLikelyDirective(name) {
			continue
		}
		value := strings.TrimSpace(line[eq+1:])
		key := section + "." + name
		out[key] = append(out[key], value)
	}
	return out
}

// getServiceDirective returns the first value of [Service]<name>, or "" if
// absent. We deliberately pick the first occurrence (not the last) so that
// drift detection is anchored to the directive operators see at the top of
// the file. ValidateSystemdUnit already rejected duplicate singletons at
// generation time, so for well-formed units there is only one.
func getServiceDirective(directives map[string][]string, name string) string {
	if v := directives["Service."+name]; len(v) > 0 {
		return strings.TrimSpace(v[0])
	}
	return ""
}

// firstWord returns the substring of s up to the first ASCII whitespace
// character. Used for ExecStart path extraction.
func firstWord(s string) string {
	s = strings.TrimSpace(s)
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			return s[:i]
		}
	}
	return s
}

// extractExecStartPath parses systemd's `systemctl show ExecStart=` output
// format. Each ExecStart line systemd emits looks like:
//
//	{ path=/usr/bin/foo ; argv[]=foo a b ; ignore_errors=no ; start_time=... }
//
// or, if there are multiple ExecStart= lines on the unit, several of these
// joined by newlines. We extract the first `path=...` token; everything
// past the next `;` or whitespace is the argv and not relevant to drift
// detection at this granularity.
//
// If the effective string doesn't carry the bracketed structure (older
// systemd versions, or when the property happens to be returned literally),
// we fall back to firstWord so the comparison still works.
func extractExecStartPath(effective string) string {
	s := strings.TrimSpace(effective)
	if s == "" {
		return ""
	}
	const marker = "path="
	idx := strings.Index(s, marker)
	if idx < 0 {
		return firstWord(s)
	}
	tail := s[idx+len(marker):]
	// terminator is the first space or ';'
	end := len(tail)
	for i := 0; i < len(tail); i++ {
		c := tail[i]
		if c == ' ' || c == ';' || c == '\t' || c == '\n' {
			end = i
			break
		}
	}
	return strings.TrimSpace(tail[:end])
}
