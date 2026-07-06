// rbac_subject_flip_test.go: Phase-3 subject flip (Path B) regression guards.
// The RBAC canonical subject key for a real account is its opaque uuid; the
// built-in superadmin stays name-keyed (carve-out). Grants written by name are
// stored uuid-canonical so a uuid subject matches — and deny still overrides
// (security.deny_overrides_allow must not silently weaken across the flip).

package main

import (
	"testing"

	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resourcepb"
	"google.golang.org/protobuf/encoding/protojson"
)

// seedAccount pre-loads an account into the RBAC getAccount cache under BOTH its
// id and its uuid, so accountExist/getAccount resolve it either way with no
// resource-service connection.
func seedAccount(t *testing.T, srv *server, acc *resourcepb.Account) {
	t.Helper()
	data, err := protojson.Marshal(acc)
	if err != nil {
		t.Fatalf("marshal account: %v", err)
	}
	if err := srv.cache.SetItem(acc.Id, data); err != nil {
		t.Fatalf("cache set id: %v", err)
	}
	if acc.Uuid != "" {
		if err := srv.cache.SetItem(acc.Uuid, data); err != nil {
			t.Fatalf("cache set uuid: %v", err)
		}
	}
}

// TestSubjectFlip_AccountExistCanonicalizesToUUID: real accounts key on the
// opaque uuid (resolvable by name OR uuid); sa is always "sa" (never its minted
// uuid — the permanent service-principal carve-out).
func TestSubjectFlip_AccountExistCanonicalizesToUUID(t *testing.T) {
	srv := newDenyTestServer(t)
	const uid = "aaaaaaaa-1111-2222-3333-444444444444"
	seedAccount(t, srv, &resourcepb.Account{Id: "dave", Uuid: uid})

	if exist, k := srv.accountExist("dave"); !exist || k != uid {
		t.Errorf("accountExist(name) = (%v,%q), want (true, uuid)", exist, k)
	}
	if exist, k := srv.accountExist(uid); !exist || k != uid {
		t.Errorf("accountExist(uuid) = (%v,%q), want (true, uuid)", exist, k)
	}
	if exist, k := srv.accountExist("sa"); !exist || k != "sa" {
		t.Errorf("accountExist(sa) = (%v,%q), want (true,\"sa\") — carve-out", exist, k)
	}
	if exist, k := srv.accountExist("sa@test.local"); !exist || k != "sa" {
		t.Errorf("accountExist(sa@domain) = (%v,%q), want (true,\"sa\")", exist, k)
	}
}

// TestSubjectFlip_GrantByNameMatchedByUUID: a permission granted (and denied) by
// account NAME is stored uuid-canonical, so a request whose subject is the
// account's uuid matches — and deny overrides allow for that uuid subject.
func TestSubjectFlip_GrantByNameMatchedByUUID(t *testing.T) {
	srv := newDenyTestServer(t)
	const uid = "bbbbbbbb-5555-6666-7777-888888888888"
	seedAccount(t, srv, &resourcepb.Account{Id: "alice", Uuid: uid})

	// ALLOW read for alice, written by NAME.
	if err := srv.setResourcePermissions("/docs/x", "file", &rbacpb.Permissions{
		Allowed: []*rbacpb.Permission{{Name: "read", Accounts: []string{"alice"}}},
	}); err != nil {
		t.Fatalf("setResourcePermissions(allow): %v", err)
	}
	has, denied, err := srv.validateAccess(uid, rbacpb.SubjectType_ACCOUNT, "read", "/docs/x")
	if err != nil {
		t.Fatalf("validateAccess(allow): %v", err)
	}
	if !has || denied {
		t.Errorf("uuid subject must match the name-granted allow: has=%v denied=%v", has, denied)
	}
	// The stored list is canonicalized to the uuid (not the name).
	if perms, _ := srv.getResourcePermissions("/docs/x"); perms == nil ||
		len(perms.GetAllowed()) == 0 || len(perms.GetAllowed()[0].GetAccounts()) == 0 ||
		perms.GetAllowed()[0].GetAccounts()[0] != uid {
		t.Errorf("stored Allowed.Accounts must be uuid, got %v", perms.GetAllowed())
	}

	// DENY overrides: allow + deny read for alice by name; the uuid subject is denied.
	if err := srv.setResourcePermissions("/docs/y", "file", &rbacpb.Permissions{
		Allowed: []*rbacpb.Permission{{Name: "read", Accounts: []string{"alice"}}},
		Denied:  []*rbacpb.Permission{{Name: "read", Accounts: []string{"alice"}}},
	}); err != nil {
		t.Fatalf("setResourcePermissions(deny): %v", err)
	}
	has, denied, err = srv.validateAccess(uid, rbacpb.SubjectType_ACCOUNT, "read", "/docs/y")
	if err != nil {
		t.Fatalf("validateAccess(deny): %v", err)
	}
	if has || !denied {
		t.Errorf("deny must override for the uuid subject: has=%v denied=%v", has, denied)
	}
}

// TestSubjectFlip_GroupCanonicalizesToUUID extends the flip to group principals:
// groupExist canonicalizes a group to its uuid (resolvable by name or uuid), and
// a permission granted by group NAME is stored uuid-canonical so a group-uuid
// subject matches. Organizations and applications use the identical resolver.
func TestSubjectFlip_GroupCanonicalizesToUUID(t *testing.T) {
	srv := newDenyTestServer(t)
	const guid = "cccccccc-1111-2222-3333-444444444444"
	g := &resourcepb.Group{Id: "admins", Uuid: guid}
	data, err := protojson.Marshal(g)
	if err != nil {
		t.Fatalf("marshal group: %v", err)
	}
	// seed under both id and uuid so getGroup resolves either way
	_ = srv.cache.SetItem("admins", data)
	_ = srv.cache.SetItem(guid, data)

	if exist, k := srv.groupExist("admins"); !exist || k != guid {
		t.Errorf("groupExist(name) = (%v,%q), want (true, uuid)", exist, k)
	}
	if exist, k := srv.groupExist(guid); !exist || k != guid {
		t.Errorf("groupExist(uuid) = (%v,%q), want (true, uuid)", exist, k)
	}

	// grant read to the group BY NAME → stored uuid-canonical → group-uuid matches
	if err := srv.setResourcePermissions("/g/x", "file", &rbacpb.Permissions{
		Allowed: []*rbacpb.Permission{{Name: "read", Accounts: []string{}, Groups: []string{"admins"}}},
	}); err != nil {
		t.Fatalf("setResourcePermissions: %v", err)
	}
	has, denied, err := srv.validateAccess(guid, rbacpb.SubjectType_GROUP, "read", "/g/x")
	if err != nil {
		t.Fatalf("validateAccess: %v", err)
	}
	if !has || denied {
		t.Errorf("group-uuid subject must match name-granted allow: has=%v denied=%v", has, denied)
	}
	if perms, _ := srv.getResourcePermissions("/g/x"); perms == nil ||
		len(perms.GetAllowed()) == 0 || len(perms.GetAllowed()[0].GetGroups()) == 0 ||
		perms.GetAllowed()[0].GetGroups()[0] != guid {
		t.Errorf("stored Allowed.Groups must be uuid, got %v", perms.GetAllowed())
	}
}
