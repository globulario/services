// Immutability layer for operational-knowledge seed entries.
//
// Entries shipped via the operational-knowledge seed (see
// docs/operational-knowledge/ and golang/opsknowledge) are stamped with
// metadata.source="seed" + metadata.immutable="true" by the seed CLI.
// They form the baseline knowledge an AI agent has at Day-0/Day-1, before
// any human or agent has had a chance to record runtime experience.
//
// To prevent silent drift — an agent or stale workflow rewriting an entry
// to no longer match the on-disk YAML — Update and Delete reject mutations
// against protected seed entries, with one explicit override:
//
//   - Subject "sa" (super-admin) MAY mutate seed entries. This is the
//     escape hatch the seed CLI itself uses for drift correction
//     (operator runs `globular auth login --user sa` before re-seeding),
//     and the only path a human has to surgically fix a bad seed entry
//     in production without re-deploying the awareness bundle.
//
// Anything else (random AI agents, application service accounts, mTLS
// peers other than sa) gets a permission-denied error.
// @awareness namespace=globular.platform
// @awareness component=platform_ai_memory
// @awareness file_role=ai_memory_immutability_enforcement
// @awareness implements=globular.platform:intent.missing_state_is_not_delete_intent
// @awareness risk=high
package main

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/security"
)

// Metadata keys the seed CLI stamps on every entry it ingests. Kept here
// (not imported from golang/opsknowledge) to avoid a server→CLI dep
// cycle; both packages must agree on these strings.
const (
	seedMetadataSourceKey      = "source"
	seedMetadataSourceValue    = "seed"
	seedMetadataImmutableKey   = "immutable"
	seedMetadataImmutableValue = "true"

	// seedAdminSubject is the only principal allowed to mutate a
	// protected seed entry. Matches the cluster super-admin identity.
	seedAdminSubject = "sa"
)

// isProtectedSeed reports whether an existing memory was loaded from the
// operational-knowledge seed and is marked immutable.
func isProtectedSeed(m *ai_memorypb.Memory) bool {
	if m == nil {
		return false
	}
	md := m.GetMetadata()
	if md == nil {
		return false
	}
	return md[seedMetadataSourceKey] == seedMetadataSourceValue &&
		md[seedMetadataImmutableKey] == seedMetadataImmutableValue
}

// authorizedSeedMutator reports whether the caller may mutate a protected
// seed entry. Today only "sa" qualifies; broaden carefully — every
// principal added here can rewrite Day-0 baseline knowledge.
func authorizedSeedMutator(ctx context.Context) bool {
	authCtx := security.FromContext(ctx)
	if authCtx == nil {
		return false
	}
	return authCtx.Subject == seedAdminSubject
}

// guardSeedMutation returns a non-nil error if the caller is attempting
// to mutate a protected seed entry without the right principal. The
// caller (Update/Delete) must invoke this AFTER fetching the existing
// memory and BEFORE issuing the write.
func guardSeedMutation(ctx context.Context, existing *ai_memorypb.Memory, op string) error {
	if !isProtectedSeed(existing) {
		return nil
	}
	if authorizedSeedMutator(ctx) {
		return nil
	}
	subject := "<anonymous>"
	if a := security.FromContext(ctx); a != nil && a.Subject != "" {
		subject = a.Subject
	}
	return fmt.Errorf(
		"%s denied: memory %q is a protected operational-knowledge seed entry "+
			"(metadata.source=seed, metadata.immutable=true); only subject %q may mutate it, "+
			"but caller is %q",
		op, existing.GetId(), seedAdminSubject, subject,
	)
}
