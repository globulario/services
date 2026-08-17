package domain

import "github.com/globulario/services/golang/ai_memory/behavioral/api"

const (
	// AlwaysConditionID is the technical runtime sentinel used to make a
	// promoted principle's forbidden moves reachable without a caller-declared
	// situational condition.
	AlwaysConditionID = "condition.always"

	// RuntimeAlwaysReachMetadataKey records the upgrade-only case where an
	// already-promoted seed principle predates AlwaysConditionID. The migration
	// also adds the sentinel to canonical AppliesWhen so explanation, runtime
	// lookup and revocation all observe one scope. This marker preserves why that
	// technical condition was added and distinguishes migration from approval.
	RuntimeAlwaysReachMetadataKey = "runtime_reach.condition_always"

	// RuntimeAlwaysReachSeedMigration is the only currently supported source of
	// migrated always reach. Keeping the value explicit makes future migration
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
