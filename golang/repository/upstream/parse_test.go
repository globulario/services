package upstream

import (
	"strings"
	"testing"
)

func TestParseRepoURL_OwnerRepo(t *testing.T) {
	o, r, err := ParseRepoURL("globulario/services")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o != "globulario" || r != "services" {
		t.Fatalf("got %s/%s, want globulario/services", o, r)
	}
}

func TestParseRepoURL_FullHTTPS(t *testing.T) {
	o, r, err := ParseRepoURL("https://github.com/globulario/services")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o != "globulario" || r != "services" {
		t.Fatalf("got %s/%s", o, r)
	}
}

func TestParseRepoURL_WithDotGit(t *testing.T) {
	o, r, err := ParseRepoURL("https://github.com/globulario/services.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o != "globulario" || r != "services" {
		t.Fatalf("got %s/%s", o, r)
	}
}

func TestParseRepoURL_BareGithubDomain(t *testing.T) {
	o, r, err := ParseRepoURL("github.com/globulario/services")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o != "globulario" || r != "services" {
		t.Fatalf("got %s/%s", o, r)
	}
}

func TestParseRepoURL_Invalid(t *testing.T) {
	for _, input := range []string{"", "just-one-part", "a/b/c", "https://gitlab.com/a/b"} {
		_, _, err := ParseRepoURL(input)
		if err == nil {
			t.Errorf("expected error for %q", input)
		}
	}
}

func TestRedactAssetURL_StripsQueryParams(t *testing.T) {
	raw := "https://github.com/a/b/releases/download/v1/file.tgz?token=SECRET123"
	got := RedactAssetURL(raw)
	if strings.Contains(got, "SECRET") {
		t.Fatalf("token leaked: %s", got)
	}
	if !strings.Contains(got, "file.tgz") {
		t.Fatalf("path lost: %s", got)
	}
}

func TestRedactAssetURL_PlainURL(t *testing.T) {
	raw := "https://github.com/a/b/releases/download/v1/file.tgz"
	got := RedactAssetURL(raw)
	if got != raw {
		t.Fatalf("plain URL should be unchanged: got %q", got)
	}
}

func TestDeriveIndexURL(t *testing.T) {
	got := DeriveIndexURL("globulario", "packages")
	want := "https://github.com/globulario/packages/releases/download/{tag}/release-index.json"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestValidateIndexURLTemplate(t *testing.T) {
	tests := []struct {
		name  string
		url   string
		valid bool
	}{
		{
			name:  "valid github template",
			url:   "https://github.com/globulario/services/releases/download/{tag}/release-index.json",
			valid: true,
		},
		{
			name:  "missing placeholder",
			url:   "https://github.com/globulario/services/releases/download/v1.2.37/release-index.json",
			valid: false,
		},
		{
			name:  "trailing stray brace",
			url:   "https://github.com/globulario/services/releases/download/{tag}/release-index.json}",
			valid: false,
		},
		{
			name:  "unbalanced brace",
			url:   "https://github.com/globulario/services/releases/download/{tag/release-index.json",
			valid: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateIndexURLTemplate(tc.url)
			if tc.valid && err != nil {
				t.Fatalf("expected valid template, got error: %v", err)
			}
			if !tc.valid && err == nil {
				t.Fatalf("expected validation error for %q", tc.url)
			}
		})
	}
}
