package main

import "testing"

func TestAllowMinioInsecureSkipVerify_DefaultFalse(t *testing.T) {
	if allowMinioInsecureSkipVerify(nil) {
		t.Fatalf("expected false for nil options")
	}
	if allowMinioInsecureSkipVerify(map[string]string{}) {
		t.Fatalf("expected false for empty options")
	}
}

func TestAllowMinioInsecureSkipVerify_TrueValues(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "yes", "on"} {
		if !allowMinioInsecureSkipVerify(map[string]string{"insecure_skip_verify": v}) {
			t.Fatalf("expected true for %q", v)
		}
	}
}

