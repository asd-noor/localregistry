package assets

import (
	"io/fs"
	"testing"
)

func TestReadFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{
			name:     "read existing yaml file",
			filename: "traefik.yaml",
			wantErr:  false,
		},
		{
			name:     "read default config yaml",
			filename: "default_config.yaml",
			wantErr:  false,
		},
		{
			name:     "read non-existent file",
			filename: "nonexistent.txt",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := ReadFile(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadFile(%q) error = %v, wantErr %v", tt.filename, err, tt.wantErr)
				return
			}
			if !tt.wantErr && data == nil {
				t.Errorf("ReadFile(%q) returned nil data", tt.filename)
			}
		})
	}
}

func TestOpen(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		file, err := Open("traefik.yaml")
		if err != nil {
			t.Fatalf("Open(traefik.yaml) failed: %v", err)
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil {
			t.Fatalf("file.Stat() failed: %v", err)
		}

		if stat.IsDir() {
			t.Error("expected file, got directory")
		}
		if stat.Name() != "traefik.yaml" {
			t.Errorf("expected name 'traefik.yaml', got %q", stat.Name())
		}
	})

	t.Run("error on missing file", func(t *testing.T) {
		_, err := Open("does-not-exist.yaml")
		if err == nil {
			t.Fatal("expected error for missing file, got nil")
		}
	})
}

func TestExists(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{
			name:     "existing file",
			filename: "traefik.yaml",
			want:     true,
		},
		{
			name:     "non-existent file",
			filename: "does-not-exist.yaml",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Exists(tt.filename); got != tt.want {
				t.Errorf("Exists(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestList(t *testing.T) {
	files, err := List()
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}

	if len(files) == 0 {
		t.Error("List() returned no files")
	}

	// Verify known files are present
	expectedFiles := map[string]bool{
		"traefik.yaml":        false,
		"default_config.yaml": false,
	}

	for _, file := range files {
		if _, ok := expectedFiles[file]; ok {
			expectedFiles[file] = true
		}
	}

	for filename, found := range expectedFiles {
		if !found {
			t.Errorf("expected file %q not found in List() output", filename)
		}
	}
}

func TestMustReadFile(t *testing.T) {
	// Test successful read
	t.Run("success", func(t *testing.T) {
		data := MustReadFile("traefik.yaml")
		if data == nil {
			t.Error("MustReadFile returned nil data")
		}
	})

	// Test panic on non-existent file
	t.Run("panic on missing file", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("MustReadFile did not panic for non-existent file")
			}
		}()
		MustReadFile("does-not-exist.yaml")
	})
}

func TestFSInterface(t *testing.T) {
	// Verify FS implements embed.FS and can be used with fs.FS
	var _ fs.FS = FS

	// Test ReadDir
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		t.Fatalf("fs.ReadDir failed: %v", err)
	}

	if len(entries) == 0 {
		t.Error("ReadDir returned no entries")
	}

	for _, entry := range entries {
		if entry.IsDir() {
			t.Errorf("expected only files, got directory: %s", entry.Name())
		}
	}
}
