// Package config loads and exposes the tempest-mcp YAML configuration file.
// Default location: ~/.config/tempest-mcp/config.yaml
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration structure for tempest-mcp.
// Fields are populated from a YAML file; use [DefaultConfig] for initial values
// and [Load] to overlay file settings.
type Config struct {
	MIDI    MIDIConfig    `yaml:"midi"`
	Library LibraryConfig `yaml:"library"`
	SysEx   SysExConfig   `yaml:"sysex"`
	Log     LogConfig     `yaml:"log"`
}

// MIDIConfig holds MIDI connection and channel settings.
type MIDIConfig struct {
	// DeviceName is a substring matched against available MIDI port names.
	DeviceName  string `yaml:"device_name"`
	Channel     uint8  `yaml:"channel"`      // 1–16; stored 1-indexed, sent 0-indexed
	ClockSource string `yaml:"clock_source"` // "internal" | "external"
}

// LibraryConfig holds paths and indexing options for the local sound library.
type LibraryConfig struct {
	Path        string `yaml:"path"`
	IndexPath   string `yaml:"index_path"`
	AutoReindex bool   `yaml:"auto_reindex"`
}

// SysExConfig controls SysEx transfer timing and optional capture behavior.
type SysExConfig struct {
	InterMessageDelayMS int    `yaml:"inter_message_delay_ms"`
	CaptureDir          string `yaml:"capture_dir"`
	// BufferBytes sizes the incoming SysEx receive buffer. The underlying
	// gomidi library defaults to 1024 bytes if left at 0, which is smaller
	// than a Beat/Kit (0x5F, ~5.9KB) or Project (0x61, likely much larger)
	// dump — receiving either would panic the whole process. Default here is
	// generous specifically to avoid that.
	BufferBytes int `yaml:"buffer_bytes"`
}

// LogConfig controls log verbosity and optional per-byte MIDI tracing.
type LogConfig struct {
	Level     string `yaml:"level"`      // debug | info | warn | error
	MIDITrace bool   `yaml:"midi_trace"` // log every raw MIDI byte
}

// DefaultConfig returns a config populated with sensible defaults.
func DefaultConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolving home directory: %w", err)
	}
	return &Config{
		MIDI: MIDIConfig{
			DeviceName:  "Tempest",
			Channel:     10,
			ClockSource: "internal",
		},
		Library: LibraryConfig{
			Path:        filepath.Join(home, "Tempest"),
			IndexPath:   filepath.Join(home, ".config", "tempest-mcp", "library.json"),
			AutoReindex: true,
		},
		SysEx: SysExConfig{
			InterMessageDelayMS: 1000,
			CaptureDir:          filepath.Join(home, ".config", "tempest-mcp", "captures"),
			BufferBytes:         1 << 20, // 1MiB — see SysExConfig.BufferBytes doc comment
		},
		Log: LogConfig{
			Level:     "info",
			MIDITrace: false,
		},
	}, nil
}

// DefaultPath returns the default config file path.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".config", "tempest-mcp", "config.yaml"), nil
}

// Load reads a YAML config file, starting from defaults.
// If the file does not exist, the defaults are returned without error.
func Load(path string) (*Config, error) {
	cfg, err := DefaultConfig()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	// Validate channel range
	if cfg.MIDI.Channel < 1 || cfg.MIDI.Channel > 16 {
		cfg.MIDI.Channel = 10
	}

	// Guard against a stale/zeroed value ever reaching gomidi, which would
	// silently fall back to its own too-small 1024-byte default.
	if cfg.SysEx.BufferBytes <= 0 {
		cfg.SysEx.BufferBytes = 1 << 20
	}

	return cfg, nil
}

// Save writes the config to path, creating parent directories as needed.
func Save(cfg *Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}
