package main

import "testing"

func TestNormalizeLoopback(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"127.0.0.1:12000", "localhost:12000"},
		{"::1:12000", "localhost:12000"},
		{"[::1]:12000", "localhost:12000"},
		{"localhost:12000", "localhost:12000"},
		{"globule-ryzen.globular.internal:12000", "globule-ryzen.globular.internal:12000"},
		{"10.0.0.63:12000", "10.0.0.63:12000"},
		{"127.0.0.1", "localhost"},
	}
	for _, tc := range cases {
		if got := normalizeLoopback(tc.in); got != tc.want {
			t.Errorf("normalizeLoopback(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
