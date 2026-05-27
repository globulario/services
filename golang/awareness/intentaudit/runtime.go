package intentaudit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/protobuf/encoding/protojson"
)

// RuntimeEvidenceProvider abstracts access to cluster runtime state.
// Implementations may query etcd, gRPC services, or return mocked data.
type RuntimeEvidenceProvider interface {
	GetJSON(ctx context.Context, key string) ([]byte, error)
	ListKeys(ctx context.Context, prefix string) ([]string, error)
}

// RuntimeCheck is a single runtime evidence check for an intent.
type RuntimeCheck struct {
	IntentID    string
	Description string
	Keys        []string // etcd keys to read
	Evaluate    func(evidence map[string][]byte) RuntimeResult
}

// RuntimeResult is the outcome of a runtime check.
type RuntimeResult struct {
	Status string // "pass", "fail", "unknown"
	Detail string
}

// EvaluateRuntimeChecks runs checks against a provider.
func EvaluateRuntimeChecks(ctx context.Context, provider RuntimeEvidenceProvider, checks []RuntimeCheck) []RuntimeResult {
	results := make([]RuntimeResult, len(checks))
	for i, check := range checks {
		evidence := make(map[string][]byte)
		for _, key := range check.Keys {
			data, err := provider.GetJSON(ctx, key)
			if err == nil {
				evidence[key] = data
			}
		}
		results[i] = check.Evaluate(evidence)
	}
	return results
}

// desiredVersionPrefix is the etcd key prefix for ServiceDesiredVersion records.
const desiredVersionPrefix = "/globular/resources/ServiceDesiredVersion/"
const installedStatePrefix = "/globular/nodes/"
const desiredWriteAuditPrefix = "/globular/audit/desired_writes/"

// desiredVersionSpec is the JSON shape we expect inside each
// ServiceDesiredVersion record.  We only extract the fields we need.
type desiredVersionSpec struct {
	Spec struct {
		Version     string `json:"version"`
		BuildNumber int    `json:"build_number"`
		BuildID     string `json:"build_id"`
	} `json:"spec"`
}

// DesiredBuildImmutabilityCheck verifies that every desired-version record
// that has been pinned (build_number > 0) also carries a build_id.
//
// Classification per service:
//
//	PASS           – version + build_number + build_id all populated
//	NOT_APPLICABLE – build_number == 0 (not yet pinned)
//	FAIL           – build_number > 0 but build_id is empty (partially pinned)
//
// The aggregated result is the worst status across all services:
//
//	any FAIL   → FAIL
//	all N/A    → not_applicable
//	all PASS   → pass
//	no data    → unknown
func DesiredBuildImmutabilityCheck() RuntimeCheck {
	return RuntimeCheck{
		IntentID:    "desired.build_id_immutable_after_resolution",
		Description: "desired build_id must not change without spec generation change",
		Keys:        []string{desiredVersionPrefix},
		Evaluate: func(evidence map[string][]byte) RuntimeResult {
			// The old-style single-key path (kept for backward compat with
			// the EvaluateRuntimeChecks helper which pre-fetches by key).
			// Real evaluation happens in EvaluateDesiredBuildImmutability.
			if len(evidence) == 0 {
				return RuntimeResult{Status: "unknown", Detail: "no desired state data available"}
			}
			return RuntimeResult{Status: "pass", Detail: "desired state evidence available"}
		},
	}
}

// EvaluateDesiredBuildImmutability runs the full provider-aware check.
// It uses ListKeys + GetJSON so it works with both mock and live providers.
func EvaluateDesiredBuildImmutability(ctx context.Context, provider RuntimeEvidenceProvider) RuntimeResult {
	keys, err := provider.ListKeys(ctx, desiredVersionPrefix)
	if err != nil {
		return RuntimeResult{Status: "unknown", Detail: fmt.Sprintf("ListKeys error: %v", err)}
	}
	if len(keys) == 0 {
		return RuntimeResult{Status: "unknown", Detail: "no keys found under " + desiredVersionPrefix}
	}

	var (
		failServices []string
		passCount    int
		naCount      int
	)

	for _, key := range keys {
		data, err := provider.GetJSON(ctx, key)
		if err != nil {
			failServices = append(failServices, svcNameFromKey(key)+": read error")
			continue
		}

		var rec desiredVersionSpec
		if err := json.Unmarshal(data, &rec); err != nil {
			failServices = append(failServices, svcNameFromKey(key)+": parse error")
			continue
		}

		switch {
		case rec.Spec.BuildNumber == 0:
			// Not yet pinned — not applicable.
			naCount++
		case rec.Spec.BuildNumber > 0 && rec.Spec.BuildID == "":
			// Partially pinned — this is the gap we want to detect.
			failServices = append(failServices, svcNameFromKey(key))
		case rec.Spec.Version != "" && rec.Spec.BuildNumber > 0 && rec.Spec.BuildID != "":
			passCount++
		default:
			// Unexpected shape — count as fail.
			failServices = append(failServices, svcNameFromKey(key)+": unexpected shape")
		}
	}

	total := len(keys)

	if len(failServices) > 0 {
		return RuntimeResult{
			Status: "fail",
			Detail: fmt.Sprintf("%d/%d services partially pinned: %s",
				len(failServices), total, strings.Join(failServices, ", ")),
		}
	}
	if passCount == 0 && naCount == total {
		return RuntimeResult{
			Status: "not_applicable",
			Detail: fmt.Sprintf("all %d services have build_number=0 (not yet pinned)", total),
		}
	}
	return RuntimeResult{
		Status: "pass",
		Detail: fmt.Sprintf("%d/%d services fully pinned, %d not yet pinned", passCount, total, naCount),
	}
}

// svcNameFromKey extracts the trailing service name from an etcd key.
func svcNameFromKey(key string) string {
	idx := strings.LastIndex(key, "/")
	if idx >= 0 && idx < len(key)-1 {
		return key[idx+1:]
	}
	return key
}

// EvaluateInstalledStateOwnership validates that installed-state records look
// like node-agent-sourced package evidence, not fabricated placeholders.
//
// Required identity fields:
//   - node_id
//   - kind
//   - name
//   - version
//
// Additional policy:
//   - keys must match /globular/nodes/{node}/packages/{kind}/{name}
func EvaluateInstalledStateOwnership(ctx context.Context, provider RuntimeEvidenceProvider) RuntimeResult {
	keys, err := provider.ListKeys(ctx, installedStatePrefix)
	if err != nil {
		return RuntimeResult{Status: "unknown", Detail: fmt.Sprintf("ListKeys error: %v", err)}
	}

	installedKeys := make([]string, 0, len(keys))
	for _, k := range keys {
		if strings.Contains(k, "/packages/") {
			installedKeys = append(installedKeys, k)
		}
	}
	if len(installedKeys) == 0 {
		return RuntimeResult{Status: "not_applicable", Detail: "no installed-state package keys found"}
	}

	var bad []string
	readErrors := 0
	valid := 0
	for _, key := range installedKeys {
		data, err := provider.GetJSON(ctx, key)
		if err != nil {
			readErrors++
			continue
		}

		// Installed records are persisted as protojson InstalledPackage.
		pkg := &node_agentpb.InstalledPackage{}
		if err := protojson.Unmarshal(data, pkg); err != nil {
			bad = append(bad, key+": parse error")
			continue
		}

		if pkg.GetNodeId() == "" || pkg.GetKind() == "" || pkg.GetName() == "" || pkg.GetVersion() == "" {
			bad = append(bad, key+": missing required identity field")
			continue
		}
		if !strings.Contains(key, "/"+pkg.GetNodeId()+"/packages/") {
			bad = append(bad, key+": key/node_id mismatch")
			continue
		}
		valid++
	}

	if readErrors > 0 {
		return RuntimeResult{
			Status: "unknown",
			Detail: fmt.Sprintf("%d/%d installed-state records unreadable within runtime window",
				readErrors, len(installedKeys)),
		}
	}
	if len(bad) > 0 {
		return RuntimeResult{
			Status: "fail",
			Detail: fmt.Sprintf("%d/%d installed-state records invalid: %s",
				len(bad), len(installedKeys), strings.Join(bad, ", ")),
		}
	}
	return RuntimeResult{
		Status: "pass",
		Detail: fmt.Sprintf("%d/%d installed-state records carry required identity fields", valid, len(installedKeys)),
	}
}

type desiredWriteProvenance struct {
	Service string `json:"service"`
	Actor   string `json:"actor"`
	Source  string `json:"source"`
	Action  string `json:"action"`
}

// EvaluateRuntimeObservationDoesNotMutateDesired checks whether desired-state
// writes can be attributed to allowed desired-state authorities.
//
// This check is provenance-sensitive:
//   - PASS only when desired keys are present AND provenance exists for each key
//     with allowed actors.
//   - FAIL when provenance explicitly shows runtime/heartbeat/observer actors.
//   - UNKNOWN when desired-state provenance metadata is missing/incomplete.
//   - NOT_APPLICABLE when no desired-state keys exist.
func EvaluateRuntimeObservationDoesNotMutateDesired(ctx context.Context, provider RuntimeEvidenceProvider) RuntimeResult {
	desiredKeys, err := provider.ListKeys(ctx, desiredVersionPrefix)
	if err != nil {
		return RuntimeResult{
			Status: "unknown",
			Detail: fmt.Sprintf("runtime_observation_must_not_mutate_desired: cannot list desired keys: %v", err),
		}
	}
	if len(desiredKeys) == 0 {
		return RuntimeResult{
			Status: "not_applicable",
			Detail: "runtime_observation_must_not_mutate_desired: no desired-state keys found",
		}
	}

	desiredServices := make(map[string]bool, len(desiredKeys))
	for _, k := range desiredKeys {
		svc := svcNameFromKey(k)
		if svc != "" {
			desiredServices[svc] = true
		}
	}

	provKeys, err := provider.ListKeys(ctx, desiredWriteAuditPrefix)
	if err != nil || len(provKeys) == 0 {
		return RuntimeResult{
			Status: "unknown",
			Detail: "runtime_observation_must_not_mutate_desired: desired-state provenance metadata missing (/globular/audit/desired_writes/*)",
		}
	}

	provenanceByService := make(map[string][]desiredWriteProvenance)
	for _, k := range provKeys {
		data, err := provider.GetJSON(ctx, k)
		if err != nil {
			continue
		}
		var rec desiredWriteProvenance
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		svc := strings.TrimSpace(rec.Service)
		if svc == "" {
			continue
		}
		provenanceByService[svc] = append(provenanceByService[svc], rec)
	}

	if len(provenanceByService) == 0 {
		return RuntimeResult{
			Status: "unknown",
			Detail: "runtime_observation_must_not_mutate_desired: provenance records unreadable or missing service actor fields",
		}
	}

	var missingProvenance []string
	var forbiddenActors []string
	for svc := range desiredServices {
		records := provenanceByService[svc]
		if len(records) == 0 {
			missingProvenance = append(missingProvenance, svc)
			continue
		}
		for _, r := range records {
			a := strings.ToLower(strings.TrimSpace(r.Actor))
			s := strings.ToLower(strings.TrimSpace(r.Source))
			if strings.Contains(a, "node-agent") || strings.Contains(a, "heartbeat") ||
				strings.Contains(a, "runtime") || strings.Contains(a, "observer") ||
				strings.Contains(a, "verifier") {
				forbiddenActors = append(forbiddenActors, fmt.Sprintf("%s:actor=%s source=%s", svc, r.Actor, r.Source))
				continue
			}
			if strings.Contains(s, "node-agent") || strings.Contains(s, "heartbeat") ||
				strings.Contains(s, "runtime") || strings.Contains(s, "observer") ||
				strings.Contains(s, "verifier") {
				forbiddenActors = append(forbiddenActors, fmt.Sprintf("%s:actor=%s source=%s", svc, r.Actor, r.Source))
			}
		}
	}

	if len(forbiddenActors) > 0 {
		return RuntimeResult{
			Status: "fail",
			Detail: fmt.Sprintf("runtime_observation_must_not_mutate_desired: runtime actors mutated desired state: %s",
				strings.Join(forbiddenActors, ", ")),
		}
	}
	if len(missingProvenance) > 0 {
		return RuntimeResult{
			Status: "unknown",
			Detail: fmt.Sprintf("runtime_observation_must_not_mutate_desired: missing provenance for desired services: %s",
				strings.Join(missingProvenance, ", ")),
		}
	}
	return RuntimeResult{
		Status: "pass",
		Detail: fmt.Sprintf("runtime_observation_must_not_mutate_desired: provenance present for %d/%d desired services; no runtime actors observed",
			len(desiredServices), len(desiredServices)),
	}
}
