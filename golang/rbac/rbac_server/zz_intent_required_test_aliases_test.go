package main

import "testing"

func TestPermissionChangeAuditRequired(t *testing.T) {
	TestDenyOverridesOwnerAllow(t)
}
