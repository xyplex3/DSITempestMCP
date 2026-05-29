package midi_test

import (
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
