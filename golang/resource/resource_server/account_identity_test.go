package main

import (
	"testing"

	Utility "github.com/globulario/utility"
	"github.com/google/uuid"
)

// TestNewAccountUUID_IsOpaqueRandomNotDerived is the Phase-3 regression guard for
// the account MEMBERSHIP identity. The mint must be an opaque, random (v4) UUID —
// never derived from a mutable attribute (name/email/domain). The exact deviation
// this migration removes is someone "helpfully" changing the mint to
// Utility.GenerateUUID(name) (a v3/MD5 hash of a mutable string), which would make
// identity churn on rename and collide across renames.
func TestNewAccountUUID_IsOpaqueRandomNotDerived(t *testing.T) {
	a := newAccountUUID()

	// Must be a valid UUID.
	parsed, err := uuid.Parse(a)
	if err != nil {
		t.Fatalf("account uuid %q is not a valid UUID: %v", a, err)
	}

	// Must be VERSION 4 (random), not v3/v5 (which imply derivation from a name
	// input). This is the load-bearing assertion: it fails if the mint is swapped
	// to a name-derived scheme.
	if parsed.Version() != 4 {
		t.Errorf("account identity must be a random (v4) UUID; got v%d in %s — a derived (v3/v5) id is the deviation", parsed.Version(), a)
	}

	// Must be opaque/unique per mint, i.e. NOT a deterministic function of any
	// account attribute: two mints differ.
	if b := newAccountUUID(); a == b {
		t.Errorf("account uuid must be unique per mint (opaque), got identical %q twice — that is a derived, not minted, identity", a)
	}

	// Guard the deviation directly: the derived form (GenerateUUID over a mutable
	// attribute) is v3 and deterministic — prove the mint is NOT that.
	derived := Utility.GenerateUUID("some-account-name")
	if newAccountUUID() == derived {
		t.Error("account uuid must never equal a name-derived GenerateUUID value")
	}
	if p, _ := uuid.Parse(derived); p.Version() == 4 {
		t.Error("sanity: GenerateUUID is expected to be the derived (non-v4) form")
	}
}
