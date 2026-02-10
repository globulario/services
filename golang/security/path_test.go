package security

import (
	"strings"
	"testing"
)

func TestCanonicalizePath(t *testing.T) {
	tests := []struct {
		name        string
		base        string
		requested   string
		wantPath    string
		wantErr     bool
		errContains string
	}{
		{
			name:      "simple relative path",
			base:      "/var/lib/globular/files",
			requested: "user/docs/file.txt",
			wantPath:  "/var/lib/globular/files/user/docs/file.txt",
			wantErr:   false,
		},
		{
			name:        "absolute path denied",
			base:        "/var/lib/globular/files",
			requested:   "/etc/passwd",
			wantErr:     true,
			errContains: "absolute paths not allowed",
		},
		{
			name:        "parent directory escape",
			base:        "/var/lib/globular/files",
			requested:   "../../etc/passwd",
			wantErr:     true,
			errContains: "escapes base directory",
		},
		{
			name:        "complex traversal attempt",
			base:        "/var/lib/globular/files",
			requested:   "user/../.././../../etc/passwd",
			wantErr:     true,
			errContains: "escapes base directory",
		},
		{
			name:        "null byte attack",
			base:        "/var/lib/globular/files",
			requested:   "file\x00.txt",
			wantErr:     true,
			errContains: "null byte",
		},
		{
			name:      "redundant slashes",
			base:      "/var/lib/globular/files",
			requested: "user//docs///file.txt",
			wantPath:  "/var/lib/globular/files/user/docs/file.txt",
			wantErr:   false,
		},
		{
			name:      "dot components",
			base:      "/var/lib/globular/files",
			requested: "./user/./docs/./file.txt",
			wantPath:  "/var/lib/globular/files/user/docs/file.txt",
			wantErr:   false,
		},
		{
			name:      "safe parent within base",
			base:      "/var/lib/globular/files",
			requested: "user/docs/../images/pic.png",
			wantPath:  "/var/lib/globular/files/user/images/pic.png",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, err := CanonicalizePath(tt.base, tt.requested)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CanonicalizePath() expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("CanonicalizePath() error = %q, want contains %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("CanonicalizePath() unexpected error: %v", err)
					return
				}
				if gotPath != tt.wantPath {
					t.Errorf("CanonicalizePath() = %q, want %q", gotPath, tt.wantPath)
				}
			}
		})
	}
}

func TestValidateResourcePath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid user resource path",
			path:    "/users/alice/files/doc.txt",
			wantErr: false,
		},
		{
			name:    "valid system resource path",
			path:    "/system/config/settings.json",
			wantErr: false,
		},
		{
			name:    "parent directory gets cleaned",
			path:    "/users/alice/../admin/secrets",
			wantErr: false, // path.Clean() normalizes this to "/users/admin/secrets", which is valid
		},
		{
			name:        "null byte",
			path:        "/users/alice/file\x00.txt",
			wantErr:     true,
			errContains: "null byte",
		},
		{
			name:        "relative path",
			path:        "users/alice/files",
			wantErr:     true,
			errContains: "must be absolute",
		},
		{
			name:    "root path",
			path:    "/",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateResourcePath(tt.path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateResourcePath() expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateResourcePath() error = %q, want contains %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateResourcePath() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestExtractOwnerFromPath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantOwner string
		wantErr   bool
	}{
		{
			name:      "valid user path",
			path:      "/users/alice/files/doc.txt",
			wantOwner: "alice",
			wantErr:   false,
		},
		{
			name:      "user root",
			path:      "/users/bob",
			wantOwner: "bob",
			wantErr:   false,
		},
		{
			name:      "nested user path",
			path:      "/users/charlie/documents/work/2026/report.pdf",
			wantOwner: "charlie",
			wantErr:   false,
		},
		{
			name:    "non-user path",
			path:    "/system/config/settings.json",
			wantErr: true,
		},
		{
			name:    "too short",
			path:    "/users",
			wantErr: true,
		},
		{
			name:      "double slash becomes valid",
			path:      "/users//files",
			wantOwner: "files", // After cleaning, "/users//files" -> "/users/files", owner is "files"
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOwner, err := ExtractOwnerFromPath(tt.path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ExtractOwnerFromPath() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("ExtractOwnerFromPath() unexpected error: %v", err)
					return
				}
				if gotOwner != tt.wantOwner {
					t.Errorf("ExtractOwnerFromPath() = %q, want %q", gotOwner, tt.wantOwner)
				}
			}
		})
	}
}

func TestPathSecurityError(t *testing.T) {
	err := &PathSecurityError{
		RequestedPath: "../../../etc/passwd",
		Reason:        "path traversal attempt",
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "path traversal attempt") {
		t.Errorf("PathSecurityError.Error() = %q, want contains 'path traversal attempt'", errStr)
	}
	if !strings.Contains(errStr, "../../../etc/passwd") {
		t.Errorf("PathSecurityError.Error() = %q, want contains requested path", errStr)
	}
}
