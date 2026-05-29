package midi_test

import (
	"strings"
	"testing"

	"tempest-mcp/internal/midi"
)

// TestCCName verifies human-readable names for CC numbers.
func TestCCName(t *testing.T) {
	tests := []struct {
		name string
		cc   uint8
		want string
	}{
		{name: "distortion CC 12", cc: 12, want: "Distortion"},
		{name: "compression CC 13", cc: 13, want: "Compression"},
		{name: "reset beat FX CC 19", cc: 19, want: "Reset Beat FX"},
		{name: "all osc freq CC 20", cc: 20, want: "All Osc Freq"},
		{name: "VCA feedback CC 21", cc: 21, want: "VCA Feedback"},
		{name: "LP cutoff CC 22", cc: 22, want: "LP Cutoff"},
		{name: "LP resonance CC 23", cc: 23, want: "LP Resonance"},
		{name: "LP audio mod CC 24", cc: 24, want: "LP Audio Mod"},
		{name: "HP cutoff CC 25", cc: 25, want: "HP Cutoff"},
		{name: "env attack CC 26", cc: 26, want: "Env Attack"},
		{name: "env decay CC 27", cc: 27, want: "Env Decay"},
		{name: "unknown CC returns CC-prefixed name", cc: 0, want: "CC0"},
		{name: "unknown CC 100", cc: 100, want: "CC100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := midi.CCName(tt.cc)
			if got != tt.want {
				t.Errorf("CCName(%d) = %q, want %q", tt.cc, got, tt.want)
			}
		})
	}
}

// TestSendCC_validation verifies that SendCC rejects CC numbers not in the
// Tempest Beat FX set without needing a live MIDI connection.
func TestSendCC_validation(t *testing.T) {
	d := midi.New(midi.DeviceConfig{Channel: 1})

	tests := []struct {
		name string
		cc   uint8
	}{
		{name: "CC 0 is not a Tempest CC", cc: 0},
		{name: "CC 11 is below valid range", cc: 11},
		{name: "CC 28 is above valid range", cc: 28},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := d.SendCC(tt.cc, 64)
			if err == nil {
				t.Fatalf("SendCC(%d) expected error, got nil", tt.cc)
			}
			if !strings.Contains(err.Error(), "not a Tempest Beat FX CC") {
				t.Errorf("error = %q, want to contain 'not a Tempest Beat FX CC'",
					err.Error())
			}
		})
	}
}

// TestSetBeatFX_unknownParam verifies that SetBeatFX rejects unknown names.
func TestSetBeatFX_unknownParam(t *testing.T) {
	d := midi.New(midi.DeviceConfig{Channel: 1})

	tests := []struct {
		name  string
		param string
	}{
		{name: "empty param name", param: ""},
		{name: "unknown param reverb", param: "reverb"},
		{name: "wrong case unknown param", param: "REVERB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := d.SetBeatFX(tt.param, 64)
			if err == nil {
				t.Fatalf("SetBeatFX(%q) expected error, got nil", tt.param)
			}
			if !strings.Contains(err.Error(), "unknown Beat FX param") {
				t.Errorf("error = %q, want to contain 'unknown Beat FX param'",
					err.Error())
			}
		})
	}
}

// TestListBeatFXParams verifies that all expected parameter names are present.
func TestListBeatFXParams(t *testing.T) {
	names := midi.ListBeatFXParams()
	if len(names) == 0 {
		t.Fatal("ListBeatFXParams() returned empty list")
	}

	// Verify a representative set of expected parameter names.
	required := []string{
		"distortion", "compression", "feedback",
		"lp-cutoff", "lp-resonance", "hp-cutoff",
		"env-attack", "env-decay",
	}
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		set[strings.ToLower(n)] = struct{}{}
	}
	for _, want := range required {
		if _, ok := set[want]; !ok {
			t.Errorf("ListBeatFXParams() missing %q", want)
		}
	}
}
