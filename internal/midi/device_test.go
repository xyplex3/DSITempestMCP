package midi_test

import (
	"strings"
	"testing"

	"tempest-mcp/internal/midi"
)

// TestNew verifies that New creates an unconnected Device with correct config.
func TestNew(t *testing.T) {
	cfg := midi.DeviceConfig{DeviceName: "TestDevice", Channel: 5}
	d := midi.New(cfg)
	if d == nil {
		t.Fatal("New() returned nil")
	}
	if d.Name() != "TestDevice" {
		t.Errorf("Name() = %q, want TestDevice", d.Name())
	}
	if d.Channel() != 5 {
		t.Errorf("Channel() = %d, want 5", d.Channel())
	}
	if d.IsConnected() {
		t.Error("IsConnected() = true, want false for new device")
	}

	// Subscribe should return a usable channel and a working cancel func.
	ch, cancel := d.Subscribe()
	if ch == nil {
		t.Error("Subscribe() returned nil channel")
	}
	cancel() // must not panic; closes the channel
}

// TestSetChannel verifies that SetChannel updates the MIDI channel.
func TestSetChannel(t *testing.T) {
	tests := []struct {
		name string
		from uint8
		to   uint8
	}{
		{name: "change to channel 1", from: 10, to: 1},
		{name: "change to channel 16", from: 1, to: 16},
		{name: "same channel", from: 10, to: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := midi.New(midi.DeviceConfig{Channel: tt.from})
			d.SetChannel(tt.to)
			if got := d.Channel(); got != tt.to {
				t.Errorf("Channel() after SetChannel(%d) = %d, want %d",
					tt.to, got, tt.to)
			}
		})
	}
}

// TestCurrentBPMUnconnected verifies that CurrentBPM returns 0 when the clock
// has not been started.
func TestCurrentBPMUnconnected(t *testing.T) {
	d := midi.New(midi.DeviceConfig{Channel: 10})
	if bpm := d.CurrentBPM(); bpm != 0 {
		t.Errorf("CurrentBPM() = %g, want 0 for stopped clock", bpm)
	}
}

// TestSetTempo_BPMRange verifies that SetTempo rejects out-of-range BPM values
// without requiring a live MIDI connection.
func TestSetTempo_BPMRange(t *testing.T) {
	d := midi.New(midi.DeviceConfig{Channel: 10})

	tests := []struct {
		name string
		bpm  float64
	}{
		{name: "BPM zero", bpm: 0},
		{name: "BPM negative", bpm: -1},
		{name: "BPM below minimum (19)", bpm: 19},
		{name: "BPM above maximum (301)", bpm: 301},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := d.SetTempo(tt.bpm)
			if err == nil {
				t.Fatalf("SetTempo(%g) expected error, got nil", tt.bpm)
			}
			if !strings.Contains(err.Error(), "out of range") {
				t.Errorf("error = %q, want to contain 'out of range'", err.Error())
			}
		})
	}
}

// TestSendRawValidation verifies that SendRaw rejects malformed SysEx without
// requiring a live connection.
func TestSendRawValidation(t *testing.T) {
	d := midi.New(midi.DeviceConfig{Channel: 10})

	tests := []struct {
		name    string
		raw     []byte
		wantErr string
	}{
		{
			name:    "too short",
			raw:     []byte{0xF0},
			wantErr: "invalid SysEx",
		},
		{
			name:    "does not start with F0",
			raw:     []byte{0x00, 0x01, 0xF7},
			wantErr: "invalid SysEx",
		},
		{
			name:    "does not end with F7",
			raw:     []byte{0xF0, 0x01, 0x00},
			wantErr: "invalid SysEx",
		},
		{
			// Valid framing but no connection → "not connected".
			name:    "valid framing on disconnected device",
			raw:     []byte{0xF0, 0x01, 0x02, 0xF7},
			wantErr: "not connected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := d.SendRaw(tt.raw)
			if err == nil {
				t.Fatalf("SendRaw() expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
