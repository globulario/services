package main

import "testing"

func TestResolveDoctorEndpoint_Override(t *testing.T) {
	t.Run("adds default tls port for bare host", func(t *testing.T) {
		got, err := resolveDoctorEndpoint("doctor.globular.internal")
		if err != nil {
			t.Fatalf("resolveDoctorEndpoint returned error: %v", err)
		}
		if got != "doctor.globular.internal:443" {
			t.Fatalf("resolved=%q, want %q", got, "doctor.globular.internal:443")
		}
	})

	t.Run("rejects loopback", func(t *testing.T) {
		if _, err := resolveDoctorEndpoint("127.0.0.1:36651"); err == nil {
			t.Fatal("expected loopback endpoint to be rejected")
		}
	})
}
