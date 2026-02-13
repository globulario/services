package globular_service

import "testing"

func TestNormalizeEndpointAddress(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"localhost:10003", "127.0.0.1:10003"},
		{"127.0.0.1:10003", "127.0.0.1:10003"},
		{"10.0.0.63:10003", "10.0.0.63:10003"},
		{"localhost", "127.0.0.1"},
		{"::1:8080", "::1:8080"}, // malformed (missing brackets), leave unchanged
		{"[::1]:8080", "127.0.0.1:8080"},
	}

	for _, tt := range tests {
		if got := NormalizeEndpointAddress(tt.in); got != tt.want {
			t.Fatalf("NormalizeEndpointAddress(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
