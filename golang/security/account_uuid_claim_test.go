package security

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestClaims_AccountUUID_RoundTripsAndOmitsWhenEmpty guards the additive account
// membership-identity claim (Phase 3). The uuid must survive a JWT-claims JSON
// round-trip so token readers can migrate to it, and it must be OMITTED when
// empty (service/sa and pre-migration tokens carry no account_uuid). Critically,
// it does NOT become the Subject/PrincipalID — authorization is unchanged until
// RBAC and storage dual-read.
func TestClaims_AccountUUID_RoundTripsAndOmitsWhenEmpty(t *testing.T) {
	const uid = "eb9a2dac-05b0-52ac-9002-99d8ffd35902"

	b, err := json.Marshal(&Claims{PrincipalID: "dave", AccountUUID: uid})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"account_uuid":"`+uid+`"`) {
		t.Errorf("account_uuid not serialized: %s", b)
	}

	var out Claims
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.AccountUUID != uid {
		t.Errorf("account_uuid did not round-trip: got %q want %q", out.AccountUUID, uid)
	}
	// The identity axis used by authz (PrincipalID/Subject) is untouched — the
	// account uuid is a separate, additive claim.
	if out.PrincipalID != "dave" {
		t.Errorf("PrincipalID must be unchanged by the additive claim, got %q", out.PrincipalID)
	}

	// Empty account_uuid must be omitted (omitempty): no "account_uuid" key.
	empty, _ := json.Marshal(&Claims{PrincipalID: "sa"})
	if strings.Contains(string(empty), "account_uuid") {
		t.Errorf("empty account_uuid must be omitted from the token, got: %s", empty)
	}
}
