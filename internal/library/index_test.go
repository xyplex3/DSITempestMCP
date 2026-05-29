package library_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"tempest-mcp/internal/library"
	"tempest-mcp/internal/sysex"
)

// TestSaveLoadIndex verifies that SaveIndex and LoadIndex round-trip correctly.
func TestSaveLoadIndex(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	idx := &library.Index{
		RootPath:  "/test/root",
		ScannedAt: now,
		Sounds: []*library.Sound{
			{
				ID:        "abc123",
				Name:      "Test Sound",
				Path:      "/test/root/sound.syx",
				SizeBytes: 256,
				Folder:    "Kicks",
				Tags:      []string{"Kicks"},
				MsgType:   "FLASH",
				IndexedAt: now,
			},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "library.json")

	if err := library.SaveIndex(idx, path); err != nil {
		t.Fatalf("SaveIndex() error = %v", err)
	}

	loaded, err := library.LoadIndex(path)
	if err != nil {
		t.Fatalf("LoadIndex() error = %v", err)
	}

	if loaded.RootPath != idx.RootPath {
		t.Errorf("RootPath = %q, want %q", loaded.RootPath, idx.RootPath)
	}
	if len(loaded.Sounds) != 1 {
		t.Fatalf("Sounds len = %d, want 1", len(loaded.Sounds))
	}
	s := loaded.Sounds[0]
	if s.ID != "abc123" {
		t.Errorf("ID = %q, want abc123", s.ID)
	}
	if s.Name != "Test Sound" {
		t.Errorf("Name = %q, want 'Test Sound'", s.Name)
	}
	if s.MsgType != "FLASH" {
		t.Errorf("MsgType = %q, want FLASH", s.MsgType)
	}
	if len(s.Tags) != 1 || s.Tags[0] != "Kicks" {
		t.Errorf("Tags = %v, want [Kicks]", s.Tags)
	}
}

// TestSaveIndexCreatesParentDir verifies that SaveIndex creates missing dirs.
func TestSaveIndexCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nested", "library.json")
	idx := &library.Index{RootPath: "/test"}

	if err := library.SaveIndex(idx, path); err != nil {
		t.Fatalf("SaveIndex() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

// TestLoadIndexMissing verifies LoadIndex returns an error for a missing file.
func TestLoadIndexMissing(t *testing.T) {
	_, err := library.LoadIndex("/nonexistent/path/library.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// TestSoundByID verifies that SoundByID finds a sound by fingerprint ID.
func TestSoundByID(t *testing.T) {
	idx := &library.Index{
		Sounds: []*library.Sound{
			{ID: "id1", Name: "Alpha"},
			{ID: "id2", Name: "Beta"},
			{ID: "id3", Name: "Gamma"},
		},
	}

	tests := []struct {
		name     string
		id       string
		wantName string
		wantNil  bool
	}{
		{name: "found first", id: "id1", wantName: "Alpha"},
		{name: "found middle", id: "id2", wantName: "Beta"},
		{name: "found last", id: "id3", wantName: "Gamma"},
		{name: "not found returns nil", id: "missing", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := idx.SoundByID(tt.id)
			if tt.wantNil {
				if got != nil {
					t.Errorf("SoundByID(%q) = %v, want nil", tt.id, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("SoundByID(%q) = nil, want %q", tt.id, tt.wantName)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
		})
	}
}

// mustWriteFile writes data to path, creating parent directories as needed.
func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating dirs: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}
}

// TestScan verifies that Scan walks a directory, parses .syx files, and
// returns a populated Index.
func TestScan(t *testing.T) {
	t.Run("empty directory returns empty index", func(t *testing.T) {
		dir := t.TempDir()
		idx, err := library.Scan(dir)
		if err != nil {
			t.Fatalf("Scan() error = %v", err)
		}
		if len(idx.Sounds) != 0 {
			t.Errorf("Sounds len = %d, want 0", len(idx.Sounds))
		}
		if idx.RootPath != dir {
			t.Errorf("RootPath = %q, want %q", idx.RootPath, dir)
		}
	})

	t.Run("single FLASH sound file is indexed", func(t *testing.T) {
		dir := t.TempDir()
		msg := sysex.BuildFLASHDump("TestKick", make([]byte, sysex.ParamBlockSizeFLASH), 0)
		path := filepath.Join(dir, "kick.syx")
		mustWriteFile(t, path, msg)
		idx, err := library.Scan(dir)
		if err != nil {
			t.Fatalf("Scan() error = %v", err)
		}
		if len(idx.Sounds) != 1 {
			t.Fatalf("Sounds len = %d, want 1", len(idx.Sounds))
		}
		s := idx.Sounds[0]
		if s.Name != "TestKick" {
			t.Errorf("Name = %q, want TestKick", s.Name)
		}
		if s.MsgType != "FLASH" {
			t.Errorf("MsgType = %q, want FLASH", s.MsgType)
		}
	})

	t.Run("non-syx files are skipped", func(t *testing.T) {
		dir := t.TempDir()
		mustWriteFile(t, filepath.Join(dir, "notes.txt"), []byte("hello"))
		idx, err := library.Scan(dir)
		if err != nil {
			t.Fatalf("Scan() error = %v", err)
		}
		if len(idx.Sounds) != 0 {
			t.Errorf("Sounds len = %d, want 0", len(idx.Sounds))
		}
	})

	t.Run("subdirectory name becomes folder tag", func(t *testing.T) {
		dir := t.TempDir()
		msg := sysex.BuildFLASHDump("SubKick", make([]byte, sysex.ParamBlockSizeFLASH), 0)
		mustWriteFile(t, filepath.Join(dir, "Kicks", "kick.syx"), msg)
		idx, err := library.Scan(dir)
		if err != nil {
			t.Fatalf("Scan() error = %v", err)
		}
		if len(idx.Sounds) != 1 {
			t.Fatalf("Sounds len = %d, want 1", len(idx.Sounds))
		}
		s := idx.Sounds[0]
		if s.Folder != "Kicks" {
			t.Errorf("Folder = %q, want Kicks", s.Folder)
		}
		if !slices.Contains(s.Tags, "Kicks") {
			t.Errorf("Tags = %v, want to contain 'Kicks'", s.Tags)
		}
	})
}

// TestReadSyxMessages verifies reading raw SysEx messages from a file.
func TestReadSyxMessages(t *testing.T) {
	t.Run("valid file returns messages", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.syx")
		// Two minimal SysEx messages back to back.
		data := []byte{
			0xF0, 0x01, 0x02, 0xF7,
			0xF0, 0x03, 0x04, 0xF7,
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("writing temp file: %v", err)
		}
		msgs, err := library.ReadSyxMessages(path)
		if err != nil {
			t.Fatalf("ReadSyxMessages() error = %v", err)
		}
		if len(msgs) != 2 {
			t.Errorf("got %d messages, want 2", len(msgs))
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		_, err := library.ReadSyxMessages("/nonexistent/path/test.syx")
		if err == nil {
			t.Fatal("expected error for missing file, got nil")
		}
		if !strings.Contains(err.Error(), "reading") {
			t.Errorf("error = %q, want to contain 'reading'", err.Error())
		}
	})

	t.Run("file with no SysEx returns error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.syx")
		if err := os.WriteFile(path, []byte{0x01, 0x02, 0x03}, 0o644); err != nil {
			t.Fatalf("writing temp file: %v", err)
		}
		_, err := library.ReadSyxMessages(path)
		if err == nil {
			t.Fatal("expected error for no SysEx messages, got nil")
		}
		if !strings.Contains(err.Error(), "no SysEx") {
			t.Errorf("error = %q, want to contain 'no SysEx'", err.Error())
		}
	})
}
