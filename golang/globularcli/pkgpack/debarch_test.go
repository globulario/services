package pkgpack

import "testing"

// SCAR (INCIDENT 2026-07-30): bundling foreign-architecture .debs made every
// Day-1 join fail at the ScyllaDB phase on any node that had applied updates.
//
// Debian multiarch requires libfoo:i386 and libfoo:amd64 to be at the EXACT
// same version. A bundled, pinned i386 copy therefore only installs while the
// host's amd64 copy has not moved. Once it has:
//
//	package libcap2:i386 1:2.66-5ubuntu2.2 cannot be configured because
//	libcap2:amd64 is at a different version (1:2.66-5ubuntu2.4)
//
// and install-local-debs aborts the whole join. The scylladb package shipped 12
// i386 debs next to 7 amd64 ones; they are pulled in whenever the BUILD host has
// i386 multiarch enabled. ScyllaDB is amd64-only and never needed them.
func TestDebMatchesArch_DropsForeignArchKeepsNativeAndAll(t *testing.T) {
	cases := []struct {
		name   string
		goarch string
		want   bool
		why    string
	}{
		{"scylla-server_2025.3.8-0.20260223.d657044d70fb-1_amd64.deb", "amd64", true, "native"},
		{"libcap2_1%3a2.66-5ubuntu2.2_i386.deb", "amd64", false, "the exact deb that broke the join"},
		{"libsystemd0_255.4-1ubuntu8.15_i386.deb", "amd64", false, "foreign arch"},
		{"scylla-conf_2025.3.8_all.deb", "amd64", true, "architecture-independent"},
		{"foo_1.0_arm64.deb", "amd64", false, "foreign arch"},
		{"foo_1.0_arm64.deb", "arm64", true, "native on arm64"},
		{"foo_1.0_amd64.deb", "arm64", false, "foreign on arm64"},
		{"weird-name-without-arch.deb", "amd64", true, "unparseable names are kept, never silently dropped"},
	}
	for _, tc := range cases {
		if got := debMatchesArch(tc.name, tc.goarch); got != tc.want {
			t.Errorf("debMatchesArch(%q, %q) = %v, want %v — %s",
				tc.name, tc.goarch, got, tc.want, tc.why)
		}
	}
}

func TestFilterDebsForArch_RemovesEveryForeignArchDeb(t *testing.T) {
	in := []string{
		"/d/scylla-server_2025.3.8_amd64.deb",
		"/d/libcap2_2.66_i386.deb",
		"/d/libsystemd0_255.4_i386.deb",
		"/d/scylla-conf_2025.3.8_all.deb",
	}
	got := filterDebsForArch(in, "amd64")
	if len(got) != 2 {
		t.Fatalf("kept %d debs, want 2 (amd64 + all): %v", len(got), got)
	}
	for _, p := range got {
		if !debMatchesArch(p, "amd64") {
			t.Errorf("foreign-arch deb survived the filter: %s", p)
		}
	}
}
