package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tempest-mcp/internal/config"
)

// TestDefaultConfig verifies that DefaultConfig returns sensible defaults.
func TestDefaultConfig(t *testing.T) {
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() error = %v", err)
	}

	if cfg.MIDI.DeviceName != "Tempest" {
		t.Errorf("DeviceName = %q, want Tempest", cfg.MIDI.DeviceName)
	}
	if cfg.MIDI.Channel != 10 {
		t.Errorf("Channel = %d, want 10", cfg.MIDI.Channel)
	}
	if cfg.MIDI.ClockSource != "internal" {
		t.Errorf("ClockSource = %q, want internal", cfg.MIDI.ClockSource)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("Level = %q, want info", cfg.Log.Level)
	}
	if cfg.Log.MIDITrace {
		t.Error("MIDITrace = true, want false")
	}
	if cfg.SysEx.InterMessageDelayMS != 1000 {
		t.Errorf("InterMessageDelayMS = %d, want 1000", cfg.SysEx.InterMessageDelayMS)
	}
	if !cfg.Library.AutoReindex {
		t.Error("AutoReindex = false, want true")
	}
}

// TestDefaultPath verifies that DefaultPath ends with the expected suffix.
func TestDefaultPath(t *testing.T) {
	path, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	suffix := filepath.Join(".config", "tempest-mcp", "config.yaml")
	if !strings.HasSuffix(path, suffix) {
		t.Errorf("path = %q, want suffix %q", path, suffix)
	}
}

// TestLoad verifies config loading from file, defaults, and validation.
func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		content string // empty = non-existent file
		wantErr bool
		check   func(*testing.T, *config.Config)
	}{
		{
			name:    "non-existent file returns defaults",
			content: "",
			check: func(t *testing.T, cfg *config.Config) {
				if cfg.MIDI.Channel != 10 {
					t.Errorf("Channel = %d, want 10 (default)", cfg.MIDI.Channel)
				}
				if cfg.MIDI.DeviceName != "Tempest" {
					t.Errorf("DeviceName = %q, want Tempest", cfg.MIDI.DeviceName)
				}
			},
		},
		{
			name:    "valid YAML overrides device name",
			content: "midi:\n  device_name: MyDevice\n",
			check: func(t *testing.T, cfg *config.Config) {
				if cfg.MIDI.DeviceName != "MyDevice" {
					t.Errorf("DeviceName = %q, want MyDevice", cfg.MIDI.DeviceName)
				}
			},
		},
		{
			name:    "valid channel 1 is accepted",
			content: "midi:\n  channel: 1\n",
			check: func(t *testing.T, cfg *config.Config) {
				if cfg.MIDI.Channel != 1 {
					t.Errorf("Channel = %d, want 1", cfg.MIDI.Channel)
				}
			},
		},
		{
			name:    "valid channel 16 is accepted",
			content: "midi:\n  channel: 16\n",
			check: func(t *testing.T, cfg *config.Config) {
				if cfg.MIDI.Channel != 16 {
					t.Errorf("Channel = %d, want 16", cfg.MIDI.Channel)
				}
			},
		},
		{
			// Channel 0 is out of range; Load resets to 10.
			name:    "channel 0 is reset to 10",
			content: "midi:\n  channel: 0\n",
			check: func(t *testing.T, cfg *config.Config) {
				if cfg.MIDI.Channel != 10 {
					t.Errorf("Channel = %d, want 10 (auto-corrected)", cfg.MIDI.Channel)
				}
			},
		},
		{
			// Channel 17 exceeds the maximum; Load resets to 10.
			name:    "channel 17 is reset to 10",
			content: "midi:\n  channel: 17\n",
			check: func(t *testing.T, cfg *config.Config) {
				if cfg.MIDI.Channel != 10 {
					t.Errorf("Channel = %d, want 10 (auto-corrected)", cfg.MIDI.Channel)
				}
			},
		},
		{
			name:    "invalid YAML returns error",
			content: "}\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			if tt.content != "" {
				dir := t.TempDir()
				path = filepath.Join(dir, "config.yaml")
				if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
					t.Fatalf("writing temp config: %v", err)
				}
			} else {
				path = filepath.Join(t.TempDir(), "nonexistent.yaml")
			}

			cfg, err := config.Load(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.check != nil && cfg != nil {
				tt.check(t, cfg)
			}
		})
	}
}

// TestSave verifies that Save writes a valid YAML file that Load can read back.
func TestSave(t *testing.T) {
	dir := t.TempDir()
	// Use a nested path to also verify that Save creates parent directories.
	path := filepath.Join(dir, "sub", "config.yaml")

	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig() error = %v", err)
	}
	cfg.MIDI.DeviceName = "SaveTestDevice"
	cfg.MIDI.Channel = 3
	cfg.Log.Level = "debug"

	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() after Save() error = %v", err)
	}
	if loaded.MIDI.DeviceName != "SaveTestDevice" {
		t.Errorf("DeviceName = %q, want SaveTestDevice", loaded.MIDI.DeviceName)
	}
	if loaded.MIDI.Channel != 3 {
		t.Errorf("Channel = %d, want 3", loaded.MIDI.Channel)
	}
	if loaded.Log.Level != "debug" {
		t.Errorf("Level = %q, want debug", loaded.Log.Level)
	}
}
