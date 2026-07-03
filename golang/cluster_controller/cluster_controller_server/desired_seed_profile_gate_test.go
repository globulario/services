package main

import "testing"

// These are the regression guards for the installed≠desired root fix: an
// OBSERVED installed package must not promote to DESIRED unless an admitted node
// profile places it. The decision lives in the pure maySeedDesiredFromInstalled.
var foundingProfiles = []string{"core", "control-plane", "storage"}

// TestInstalledMediaPackageDoesNotBecomeDesiredWithoutMediaProfile: a media
// workload (catalog placement = media-server) installed on a founding node must
// NOT seed desired — it is capability residue, not intent. This is the sticky
// ffmpeg/yt-dlp/media-service bug.
func TestInstalledMediaPackageDoesNotBecomeDesiredWithoutMediaProfile(t *testing.T) {
	if maySeedDesiredFromInstalled([]string{"media-server"}, foundingProfiles) {
		t.Fatal("media package (placement=media-server) MUST NOT seed desired on a core/control-plane/storage node")
	}
	if maySeedDesiredFromInstalled([]string{"media-server", "compute"}, foundingProfiles) {
		t.Fatal("media/compute package MUST NOT seed desired when no admitted node has those profiles")
	}
}

// TestDay0SeedsOnlyFoundingProfilePackages: packages placed by a founding
// profile seed; packages placed only by non-admitted profiles do not.
func TestDay0SeedsOnlyFoundingProfilePackages(t *testing.T) {
	// Placed by an admitted profile → seeds.
	if !maySeedDesiredFromInstalled([]string{"core", "storage"}, foundingProfiles) {
		t.Fatal("a package placed on core/storage MUST seed desired on a founding node")
	}
	if !maySeedDesiredFromInstalled([]string{"control-plane"}, foundingProfiles) {
		t.Fatal("a control-plane package MUST seed desired on a control-plane node")
	}
	// Placed only by a non-admitted profile → does not seed.
	if maySeedDesiredFromInstalled([]string{"gateway"}, foundingProfiles) {
		t.Fatal("a gateway-only package MUST NOT seed desired when no admitted node has the gateway profile")
	}
}

// TestDay0DoesNotSeedDesiredFromInstalledPackages: the general law — an
// installed package not placed by any admitted profile does not become desired.
// (Uncataloged packages keep prior behavior: they seed, since placement is
// unknown; the gate only blocks CATALOGED workloads with no admitted placement.)
func TestDay0DoesNotSeedDesiredFromInstalledPackages(t *testing.T) {
	// Cataloged but unplaceable → refused.
	if maySeedDesiredFromInstalled([]string{"media-server"}, foundingProfiles) {
		t.Fatal("cataloged-but-unplaceable installed package must not seed desired")
	}
	// Empty admitted profiles (very early bootstrap): do NOT gate — refusing all
	// would break day-0. Seeding proceeds.
	if !maySeedDesiredFromInstalled([]string{"media-server"}, nil) {
		t.Fatal("with no admitted profiles yet, the gate must not block (would break day-0)")
	}
	// Uncataloged package (no placement profiles) → unchanged: seeds.
	if !maySeedDesiredFromInstalled(nil, foundingProfiles) {
		t.Fatal("uncataloged package (no catalog profiles) must retain prior seeding behavior")
	}
}

// TestObservedInstalledPackageRemainsRemovable: the removability property is a
// direct consequence of NOT seeding — an installed-but-not-desired package is an
// orphan the reconciler can remove. We assert the gate refuses to seed it, which
// is what keeps it removable (installed present, desired absent).
func TestObservedInstalledPackageRemainsRemovable(t *testing.T) {
	// The residue package is observed (installed) but the gate refuses to seed
	// desired for it → installed≠desired → removable.
	if maySeedDesiredFromInstalled([]string{"media-server"}, foundingProfiles) {
		t.Fatal("residue package must not be seeded to desired, otherwise it becomes an undead unremovable citizen")
	}
}
