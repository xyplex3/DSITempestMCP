package midi_test

import (
	"context"
	"strings"
	"testing"

	"tempest-mcp/internal/midi"
)

// TestNoteForPad verifies MIDI note lookup for pad names.
func TestNoteForPad(t *testing.T) {
	tests := []struct {
		name    string
		padName string
		want    uint8
		wantErr bool
		errFrag string
	}{
		{name: "kick by name", padName: "kick", want: 36},
		{name: "kick alias bass-drum", padName: "bass-drum", want: 36},
		{name: "kick pad ID a12", padName: "a12", want: 36},
		{name: "snare", padName: "snare", want: 38},
		{name: "snare-1", padName: "snare-1", want: 38},
		{name: "snare-2", padName: "snare-2", want: 40},
		{name: "closed hat", padName: "closed-hat", want: 42},
		{name: "open hat", padName: "open-hat", want: 46},
		{name: "high tom", padName: "high-tom", want: 48},
		{name: "mid tom", padName: "mid-tom", want: 47},
		{name: "low tom", padName: "low-tom", want: 43},
		{name: "crash", padName: "crash", want: 49},
		{name: "ride", padName: "ride", want: 51},
		{name: "clap", padName: "clap", want: 39},
		{name: "case-insensitive KICK", padName: "KICK", want: 36},
		{name: "whitespace trimmed", padName: "  kick  ", want: 36},
		{
			name:    "unknown pad returns error",
			padName: "cowbell",
			wantErr: true,
			errFrag: "unknown pad",
		},
		{
			name:    "empty string returns error",
			padName: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := midi.NoteForPad(tt.padName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NoteForPad(%q) error = %v, wantErr %v", tt.padName, err, tt.wantErr)
			}
			if err != nil {
				if tt.errFrag != "" && !strings.Contains(err.Error(), tt.errFrag) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.errFrag)
				}
				return
			}
			if got != tt.want {
				t.Errorf("NoteForPad(%q) = %d, want %d", tt.padName, got, tt.want)
			}
		})
	}
}

// TestListPadNames verifies that all expected pad names are present.
func TestListPadNames(t *testing.T) {
	names := midi.ListPadNames()
	if len(names) == 0 {
		t.Fatal("ListPadNames() returned empty list")
	}

	// Spot-check a representative set of expected names.
	required := []string{"kick", "snare", "closed-hat", "open-hat",
		"high-tom", "clap", "crash"}
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		set[n] = struct{}{}
	}
	for _, want := range required {
		if _, ok := set[want]; !ok {
			t.Errorf("ListPadNames() missing %q", want)
		}
	}
}

// TestTriggerPad verifies error propagation without a live MIDI connection:
// an unknown pad name fails before ever touching the device, while a known
// pad name on a disconnected device fails at the send step instead.
func TestTriggerPad(t *testing.T) {
	tests := []struct {
		name    string
		pad     string
		wantErr string
	}{
		{name: "unknown pad rejected before connecting", pad: "cowbell", wantErr: "unknown pad"},
		{name: "known pad fails at send on disconnected device", pad: "kick", wantErr: "not connected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := midi.New(midi.DeviceConfig{Channel: 10})
			err := d.TriggerPad(context.Background(), tt.pad, 100, 10)
			if err == nil {
				t.Fatalf("TriggerPad(%q) expected error, got nil", tt.pad)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// TestTriggerNote verifies that TriggerNote on a disconnected device fails
// with a "not connected" error rather than blocking or panicking.
func TestTriggerNote(t *testing.T) {
	d := midi.New(midi.DeviceConfig{Channel: 10})
	err := d.TriggerNote(context.Background(), 36, 100, 10)
	if err == nil {
		t.Fatal("TriggerNote() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "not connected")
	}
}

// TestPlaySequence verifies event sorting and error propagation on a
// disconnected device. Every case uses beat 1 for the earliest event so the
// resulting zero wait keeps the test fast and deterministic — see
// PlaySequence's beatOffsetMS computation.
func TestPlaySequence(t *testing.T) {
	t.Run("empty events returns nil without connecting", func(t *testing.T) {
		d := midi.New(midi.DeviceConfig{Channel: 10})
		if err := d.PlaySequence(context.Background(), nil, 120); err != nil {
			t.Errorf("PlaySequence(nil) = %v, want nil", err)
		}
	})

	t.Run("sorts events by beat before playing", func(t *testing.T) {
		d := midi.New(midi.DeviceConfig{Channel: 10})
		events := []midi.SequenceEvent{
			{Pad: "kick", Beat: 3},
			{Pad: "snare", Beat: 1},
			{Pad: "clap", Beat: 2},
		}
		err := d.PlaySequence(context.Background(), events, 120)
		if err == nil {
			t.Fatal("PlaySequence() expected error from disconnected device, got nil")
		}
		// The lowest-beat event (snare, beat 1) must be attempted first
		// regardless of its position in the input slice.
		if !strings.Contains(err.Error(), "event at beat 1") {
			t.Errorf("error = %q, want to contain %q (sorting not applied)",
				err.Error(), "event at beat 1")
		}
	})

	t.Run("wraps the underlying trigger error with the beat number", func(t *testing.T) {
		d := midi.New(midi.DeviceConfig{Channel: 10})
		events := []midi.SequenceEvent{{Note: 36, Beat: 1}}
		err := d.PlaySequence(context.Background(), events, 120)
		if err == nil {
			t.Fatal("PlaySequence() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "event at beat 1") ||
			!strings.Contains(err.Error(), "not connected") {
			t.Errorf("error = %q, want to contain both %q and %q",
				err.Error(), "event at beat 1", "not connected")
		}
	})
}
