package main

// profile_hygiene.go — E3: publish-time manifest↔catalog profile hygiene
// (WARN-first / detection mode).
//
// The component catalog is the placement authority (E1/E2). A package's
// manifest `profiles` field is INFORMATIONAL — runtime placement never reads it.
// But it is authored independently (in the package spec) and silently drifts
// from the catalog, which is what seeded the torrent incident. This check
// surfaces that drift at publish time.
//
// Rollout state: WARN-first / DETECTION only. It NEVER blocks a publish and
// NEVER mutates the manifest (no "derive from catalog" yet — that is a separate,
// future policy). It is not hard-enforced until contradiction→FAIL is enabled.
//
// Authority/forbidden-fix guardrails this respects:
//   - it does NOT mutate manifest.Profiles (no recompute_identity_from_secondary_source);
//   - it does NOT make manifest profiles a runtime placement input
//     (forbidden_fix: do_not_restore_manifest_profiles_as_runtime_placement_authority);
//   - the catalog remains the sole placement authority.
//
// invariant candidate: publish.manifest_profiles_must_not_contradict_catalog
// (review_only / WARN-first until promoted to FAIL).

import (
	"log"
	"sort"
	"strings"

	"github.com/globulario/services/golang/component_catalog"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

type profileHygieneStatus string

const (
	profileHygieneOK          profileHygieneStatus = "ok"
	profileHygieneSkipUnknown profileHygieneStatus = "skip_unknown" // not a catalog-tracked component
	profileHygieneMissing     profileHygieneStatus = "missing"      // manifest empty, catalog non-empty
	profileHygieneOverBroad   profileHygieneStatus = "over_broad"   // manifest ⊋ catalog
	profileHygieneUnderBroad  profileHygieneStatus = "under_broad"  // manifest ⊊ catalog
	profileHygieneMismatch    profileHygieneStatus = "mismatch"     // neither subset (disjoint/partial)
)

// normProfileSet lowercases, trims, dedups and sorts profile names WITHOUT
// inheritance expansion. We compare profile-membership sets ("which profiles
// claim this package") directly — the catalog's ProfilesForPackage and the
// manifest's declared profiles are both membership lists, so expanding
// inheritance here would muddy the comparison.
func normProfileSet(in []string) []string {
	seen := map[string]struct{}{}
	for _, p := range in {
		k := strings.ToLower(strings.TrimSpace(p))
		if k != "" {
			seen[k] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func isSubset(a, b map[string]struct{}) bool {
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func toSet(in []string) map[string]struct{} {
	m := make(map[string]struct{}, len(in))
	for _, p := range in {
		m[p] = struct{}{}
	}
	return m
}

// classifyProfileHygiene compares a package's manifest profiles against the
// component-catalog placement authority and classifies the relationship. It is
// pure (no I/O, no mutation) so it can be unit-tested against the real catalog.
//
// A package with no catalog entry is skip_unknown — "unknown to the catalog" is
// a DISTINCT condition from a profile disagreement and must not be conflated
// (same exclusion as E1/E2).
func classifyProfileHygiene(name string, manifestProfiles []string) (profileHygieneStatus, []string) {
	catalog := component_catalog.ProfilesForPackage(name)
	if len(catalog) == 0 {
		return profileHygieneSkipUnknown, nil
	}
	m := normProfileSet(manifestProfiles)
	if len(m) == 0 {
		return profileHygieneMissing, catalog
	}
	c := normProfileSet(catalog)
	mset, cset := toSet(m), toSet(c)
	switch {
	case isSubset(mset, cset) && isSubset(cset, mset):
		return profileHygieneOK, catalog
	case isSubset(cset, mset): // catalog ⊆ manifest → manifest has extras
		return profileHygieneOverBroad, catalog
	case isSubset(mset, cset): // manifest ⊆ catalog → manifest omits some
		return profileHygieneUnderBroad, catalog
	default:
		return profileHygieneMismatch, catalog
	}
}

// warnIfManifestProfilesDriftFromCatalog emits an operator-facing WARNING when a
// known component's manifest profiles disagree with the catalog placement
// authority. WARN-first: it never blocks publish and never mutates the manifest.
func warnIfManifestProfilesDriftFromCatalog(manifest *repopb.ArtifactManifest) {
	if manifest == nil || manifest.GetRef() == nil {
		return
	}
	name := strings.ToLower(strings.TrimSpace(manifest.GetRef().GetName()))
	if name == "" {
		return
	}
	status, catalog := classifyProfileHygiene(name, manifest.GetProfiles())
	switch status {
	case profileHygieneOK, profileHygieneSkipUnknown:
		return
	}
	log.Printf("WARN repository.publish: manifest profile hygiene drift (%s) for %s@%s — "+
		"manifest profiles %v vs catalog placement authority %v. "+
		"Manifest profiles are informational only (catalog is authoritative for placement); "+
		"remediation: align the package spec's metadata.profiles with the catalog %v, "+
		"or intentionally omit them until a derive-from-catalog policy exists. "+
		"[detection-mode: WARN-first; not blocking; invariant publish.manifest_profiles_must_not_contradict_catalog]",
		status, name, manifest.GetRef().GetVersion(), normProfileSet(manifest.GetProfiles()), catalog, catalog)
}
