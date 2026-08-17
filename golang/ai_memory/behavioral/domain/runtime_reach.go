package domain

import "github.com/globulario/services/golang/ai_memory/behavioral/api"

const (
	// AlwaysConditionID is the technical runtime sentinel used to make a
	// promoted principle's forbidden moves reachable without a caller-declared
	// situational condition.
	AlwaysConditionID = "condition.always"

	// RuntimeAlwaysReachMetadataKey records the upgrade-only case where an
	// already-promoted seed principle predates AlwaysConditionID. The semantic
	// AppliesWhen approved on the principle is deliberately left unchanged;
	// this marker is canonical migration state for the extra runtime index reach.
	RuntimeAlwaysReachMetadataKey = "runtime_reach.condition_always"

	// RuntimeAlwaysReachSeedMigration is the only currently supported source of
	// synthetic always reach. Keeping the value explicit makes future migration
	// formats distinguishable rather than treating an arbitrary truthy string as
	// authority-affecting runtime state.
	RuntimeAlwaysReachSeedMigration = "seed_migration_v1"
)

// HasMigratedAlwaysReach reports whether a principle carries the canonical
// upgrade marker for technical condition.always reach.
func HasMigratedAlwaysReach(p *api.Principle) bool {
	return p != nil && p.Metadata != nil &&
		p.Metadata[RuntimeAlwaysReachMetadataKey] == RuntimeAlwaysReachSeedMigration
}

func principleDeclaresCondition(p *api.Principle, want api.ConditionRef) bool {
	if p == nil {
		return false
	}
	for _, c := range p.AppliesWhen {
		if c == want {
			return true
		}
	}
	return false
}

// RuntimeIndexedPrinciple returns the exact condition set that must be present
// in the active principles_by_condition index. For ordinary principles it is a
// copy of the persisted semantic AppliesWhen. For a migrated principle it also
// contains condition.always, derived from the persisted migration marker.
//
// The input principle is never mutated.
func RuntimeIndexedPrinciple(p *api.Principle) api.Principle {
	if p == nil {
		return api.Principle{}
	}
	cp := *p
	cp.AppliesWhen = append([]api.ConditionRef(nil), p.AppliesWhen...)
	always := api.ConditionRef(AlwaysConditionID)
	if HasMigratedAlwaysReach(p) && !principleDeclaresCondition(p, always) {
		cp.AppliesWhen = append(cp.AppliesWhen, always)
	}
	return cp
}
