package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeConfigPath(t *testing.T) {
	// Create a temporary base directory for testing
	baseDir := t.TempDir()

	// Create a subdirectory
	subDir := filepath.Join(baseDir, "subdir")
	if err := os.MkdirAll(subDir, 0750); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		baseDir     string
		wantErr     bool
		errContains string
	}{
		{
			name:    "simple filename in base dir",
			path:    "config.yaml",
			baseDir: baseDir,
			wantErr: false,
		},
		{
			name:    "file in subdirectory",
			path:    "subdir/config.yaml",
			baseDir: baseDir,
			wantErr: false,
		},
		{
			name:        "path traversal with ..",
			path:        "../etc/passwd",
			baseDir:     baseDir,
			wantErr:     true,
			errContains: "path traversal detected",
		},
		{
			name:        "path traversal multiple ..",
			path:        "../../etc/passwd",
			baseDir:     baseDir,
			wantErr:     true,
			errContains: "path traversal detected",
		},
		{
			name:        "path traversal hidden in path",
			path:        "subdir/../../etc/passwd",
			baseDir:     baseDir,
			wantErr:     true,
			errContains: "path traversal detected",
		},
		{
			name:        "absolute path outside base",
			path:        "/etc/passwd",
			baseDir:     baseDir,
			wantErr:     true,
			errContains: "path traversal detected",
		},
		{
			name:    "absolute path inside base",
			path:    filepath.Join(baseDir, "config.yaml"),
			baseDir: baseDir,
			wantErr: false,
		},
		{
			name:    "absolute path to subdirectory",
			path:    filepath.Join(subDir, "config.yaml"),
			baseDir: baseDir,
			wantErr: false,
		},
		{
			name:    "dot path stays in base",
			path:    "./config.yaml",
			baseDir: baseDir,
			wantErr: false,
		},
		{
			name:        "only dot dot",
			path:        "..",
			baseDir:     baseDir,
			wantErr:     true,
			errContains: "path traversal detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sanitizeConfigPath(tt.path, tt.baseDir)

			if tt.wantErr {
				if err == nil {
					t.Errorf("sanitizeConfigPath(%q, %q) expected error containing %q, got nil",
						tt.path, tt.baseDir, tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("sanitizeConfigPath(%q, %q) error = %q, want error containing %q",
						tt.path, tt.baseDir, err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("sanitizeConfigPath(%q, %q) unexpected error: %v",
					tt.path, tt.baseDir, err)
				return
			}

			// Verify the result is within base directory
			absBase, _ := filepath.Abs(tt.baseDir)
			relPath, err := filepath.Rel(absBase, result)
			if err != nil || (len(relPath) >= 2 && relPath[:2] == "..") {
				t.Errorf("sanitizeConfigPath(%q, %q) = %q, which is outside base directory",
					tt.path, tt.baseDir, result)
			}
		})
	}
}

func TestSanitizeConfigPath_EdgeCases(t *testing.T) {
	baseDir := t.TempDir()

	// Test with trailing slashes
	t.Run("base dir with trailing slash", func(t *testing.T) {
		_, err := sanitizeConfigPath("config.yaml", baseDir+"/")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	// Test with empty path
	t.Run("empty path", func(t *testing.T) {
		result, err := sanitizeConfigPath("", baseDir)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Empty path should resolve to base directory
		absBase, _ := filepath.Abs(baseDir)
		if result != absBase {
			t.Errorf("expected %q, got %q", absBase, result)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
