package main

import "testing"

func TestVersionCheckDecision(t *testing.T) {
	cases := []struct {
		name                                                     string
		desiredVer, desiredBID, installedVer, installedBID       string
		hasInstalled                                             bool
		wantOK                                                   bool
		wantReason                                               string
	}{
		{
			name:         "build_ids match -> OK",
			desiredVer:   "1.2.0",
			desiredBID:   "80ab89b1-72b5-4b2e-ba01-bbebba1cffbd",
			installedVer: "1.2.0",
			installedBID: "80ab89b1-72b5-4b2e-ba01-bbebba1cffbd",
			hasInstalled: true,
			wantOK:       true,
			wantReason:   "",
		},
		{
			name:         "build_ids differ but versions match -> OK with build-drift reason",
			desiredVer:   "1.2.0",
			desiredBID:   "80ab89b1-72b5-4b2e-ba01-bbebba1cffbd",
			installedVer: "1.2.0",
			installedBID: "ffffffff-1111-2222-3333-444444444444",
			hasInstalled: true,
			wantOK:       true,
			wantReason:   "build drift: 1.2.0 installed ffffffff, desired 80ab89b1",
		},
		{
			name:         "build_ids differ AND versions differ -> FAIL",
			desiredVer:   "1.2.0",
			desiredBID:   "80ab89b1-72b5-4b2e-ba01-bbebba1cffbd",
			installedVer: "1.1.5",
			installedBID: "ffffffff-1111-2222-3333-444444444444",
			hasInstalled: true,
			wantOK:       false,
			wantReason:   "installed 1.1.5, desired 1.2.0",
		},
		{
			name:         "no installed record -> FAIL not installed",
			desiredVer:   "1.1.5",
			desiredBID:   "",
			installedVer: "",
			installedBID: "",
			hasInstalled: false,
			wantOK:       false,
			wantReason:   "not installed (desired 1.1.5)",
		},
		{
			name:         "version fallback match (no build_id either side) -> OK",
			desiredVer:   "1.2.0",
			desiredBID:   "",
			installedVer: "1.2.0",
			installedBID: "",
			hasInstalled: true,
			wantOK:       true,
			wantReason:   "",
		},
		{
			name:         "version fallback mismatch (no build_id either side) -> FAIL",
			desiredVer:   "1.2.0",
			desiredBID:   "",
			installedVer: "1.1.5",
			installedBID: "",
			hasInstalled: true,
			wantOK:       false,
			wantReason:   "installed 1.1.5, desired 1.2.0",
		},
		{
			name:         "desired build_id but installed missing -> falls back to version match OK",
			desiredVer:   "1.2.0",
			desiredBID:   "80ab89b1-72b5-4b2e-ba01-bbebba1cffbd",
			installedVer: "1.2.0",
			installedBID: "",
			hasInstalled: true,
			wantOK:       true,
			wantReason:   "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, reason := versionCheckDecision(tc.desiredVer, tc.desiredBID, tc.installedVer, tc.installedBID, tc.hasInstalled)
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
			if reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", reason, tc.wantReason)
			}
		})
	}
}

func TestShortBuildID(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"abc", "abc"},
		{"80ab89b1-72b5-4b2e-ba01-bbebba1cffbd", "80ab89b1"},
		{"12345678", "12345678"},
		{"123456789", "12345678"},
	}
	for _, tc := range cases {
		if got := shortBuildID(tc.in); got != tc.want {
			t.Errorf("shortBuildID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
