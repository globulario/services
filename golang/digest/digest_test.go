package digest

import "testing"

const (
	liveBare     = "b4429a27015b20bf573bc8bb8f13f850a68a71af92df77ed725084bf46509937"
	livePrefixed = "sha256:" + liveBare
)

func TestCanonicalSHA256(t *testing.T) {
	cases := []struct{ in, want string }{
		{"sha256:abc", "abc"},
		{"ABC", "abc"},
		{" abc ", "abc"},
		{"SHA256:ABC", "abc"},            // uppercase prefix: lowercase-before-strip
		{" sha256:ABC ", "abc"},          // whitespace + prefix + case
		{"Sha256:AbC", "abc"},            // mixed-case prefix
		{livePrefixed, liveBare},         // exact live INC case
		{"", ""},                         // empty stays empty
		{"  ", ""},                       // whitespace-only → empty
		{"not-a-digest", "not-a-digest"}, // garbage is normalized, not rejected
	}
	for _, c := range cases {
		if got := CanonicalSHA256(c.in); got != c.want {
			t.Errorf("CanonicalSHA256(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEqualSHA256(t *testing.T) {
	equal := [][2]string{
		{"sha256:abc", "abc"},
		{"ABC", "abc"},
		{" abc ", "abc"},
		{"SHA256:ABC", "abc"},
		{" sha256:ABC ", "abc"},
		{livePrefixed, liveBare}, // the false-drift case PR-18 exists to kill
	}
	for _, p := range equal {
		if !EqualSHA256(p[0], p[1]) {
			t.Errorf("EqualSHA256(%q, %q) = false, want true", p[0], p[1])
		}
	}

	notEqual := [][2]string{
		{"sha256:abc", "abd"},
		{"abc", ""},
		{liveBare, "sha256:deadbeef"},
	}
	for _, p := range notEqual {
		if EqualSHA256(p[0], p[1]) {
			t.Errorf("EqualSHA256(%q, %q) = true, want false", p[0], p[1])
		}
	}
}

// Locks the ordering contract: lowercasing must occur before prefix stripping,
// otherwise an uppercase "SHA256:" prefix would survive.
func TestCanonicalSHA256_UppercasePrefixStrippedViaLowercaseFirst(t *testing.T) {
	if got := CanonicalSHA256("SHA256:DEADBEEF"); got != "deadbeef" {
		t.Fatalf("CanonicalSHA256(\"SHA256:DEADBEEF\") = %q, want \"deadbeef\" (lowercase must precede strip)", got)
	}
}
