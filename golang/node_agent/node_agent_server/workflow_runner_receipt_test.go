package main

import (
	"context"
	"errors"
	"testing"
)

// An active unit must NOT satisfy a join install when installed-state is
// missing. That shortcut is what left units permanently unprovable after a
// wipe-and-rejoin: the receipt-stamping install path never ran, so
// cluster-doctor raised unit_receipt_drift.installed_state_missing_or_unproven
// and it never cleared.
//
// The regression is behavioural, so the test pins the decision predicate
// rather than the whole action: given no installed-state record, "unit is
// active" must not be sufficient to skip.
func TestActiveUnitAloneDoesNotSatisfyInstall(t *testing.T) {
	alwaysActive := func(ctx context.Context, unit string) (bool, error) { return true, nil }

	// No installed-state record at all — the wipe-and-rejoin case.
	if skipIfAlreadyInstalled(context.Background(), "ldap", nil, alwaysActive) {
		t.Error("skipped install with NO installed-state record — an active unit is " +
			"an observation, not an owner-produced install receipt " +
			"(installed_state_requires_successful_owner_install_receipt)")
	}
}

// A probe failure must never be read as "installed". Fail-closed: if we cannot
// tell whether the unit is active, we must not skip the install.
func TestUnitProbeErrorDoesNotSatisfyInstall(t *testing.T) {
	probeFails := func(ctx context.Context, unit string) (bool, error) {
		return false, errors.New("systemctl unavailable")
	}
	if skipIfAlreadyInstalled(context.Background(), "ldap", nil, probeFails) {
		t.Error("skipped install when the unit probe failed — unknown must not " +
			"default to satisfied")
	}
}
