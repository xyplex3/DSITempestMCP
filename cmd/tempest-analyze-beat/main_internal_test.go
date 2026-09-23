// White-box tests for tempest-analyze-beat's unexported analysis helpers.
// package main has no exported caller path, so these live alongside the
// source rather than in a _test package.
package main

import (
	"sort"
	"testing"

	"tempest-mcp/internal/sysex"
)

// TestMessageTypeToString verifies the display label for every recognised
// SysEx message type, plus an unrecognised numeric fallback.
func TestMessageTypeToString(t *testing.T) {
	tests := []struct {
		name string
		t    sysex.MessageType
		want string
	}{
		{name: "unknown", t: sysex.TypeUnknown, want: "Unknown"},
		{name: "RAM sound", t: sysex.TypeRAMSound, want: "RAM Sound (0x60)"},
		{name: "project dump", t: sysex.TypeProjectDump, want: "Project Dump (0x61)"},
		{name: "FLASH sound", t: sysex.TypeFLASHSound, want: "FLASH Sound (0x63)"},
		{name: "alternate sound", t: sysex.TypeAlternateSound, want: "Alternate Sound (0x5C) - Individual Beat"},
		{name: "alternate bank", t: sysex.TypeAlternateBank, want: "Alternate Bank (0x5E) - Project Header"},
		{name: "beat dump", t: sysex.TypeBeatDump, want: "Beat/Kit Dump (0x5F)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := messageTypeToString(tt.t); got != tt.want {
				t.Errorf("messageTypeToString(%v) = %q, want %q", tt.t, got, tt.want)
			}
		})
	}
}

// TestGetStepRange verifies min/max extraction from a step-to-tracks map.
func TestGetStepRange(t *testing.T) {
	tests := []struct {
		name    string
		steps   map[int][]string
		wantMin int
		wantMax int
	}{
		{name: "empty map", steps: map[int][]string{}, wantMin: 0, wantMax: 0},
		{name: "single step", steps: map[int][]string{5: {"A1"}}, wantMin: 5, wantMax: 5},
		{
			name:    "multiple steps",
			steps:   map[int][]string{1: {"A1"}, 5: {"A2"}, 3: {"A3"}},
			wantMin: 1, wantMax: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			min, max := getStepRange(tt.steps)
			if min != tt.wantMin || max != tt.wantMax {
				t.Errorf("getStepRange() = (%d, %d), want (%d, %d)", min, max, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestGetKeys verifies that every key of the input map is returned, in any
// order (map iteration order is not guaranteed, so the result is sorted
// before comparison).
func TestGetKeys(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]bool
		want []string
	}{
		{name: "empty map", m: map[string]bool{}, want: nil},
		{name: "single key", m: map[string]bool{"A1": true}, want: []string{"A1"}},
		{
			name: "multiple keys",
			m:    map[string]bool{"A1": true, "A2": true, "A3": true},
			want: []string{"A1", "A2", "A3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getKeys(tt.m)
			sort.Strings(got)
			if len(got) != len(tt.want) {
				t.Fatalf("getKeys() = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("getKeys() = %v, want %v", got, tt.want)
					break
				}
			}
		})
	}
}

// TestParseNoteRecords verifies note-record extraction from the sequencer
// region: a 10-byte record starting at the search origin (byte 1077), with
// the 0x77 marker at relative offset 3 and a high-bit-set track byte at
// relative offset 4.
func TestParseNoteRecords(t *testing.T) {
	// buildPayload places one record at absolute offset 1077 with the
	// given step byte, track byte, and velocity byte.
	buildPayload := func(stepByte, trackByte, velocityByte byte) []byte {
		buf := make([]byte, 1100)
		buf[1077+2] = stepByte
		buf[1077+3] = 0x77
		buf[1077+4] = trackByte
		buf[1077+5] = velocityByte
		return buf
	}

	t.Run("single well-formed record", func(t *testing.T) {
		// stepByte=3 -> stepIndex=1 -> Step=2 (1-based); trackByte=0x80 ->
		// track index 0 -> "A1".
		payload := buildPayload(3, 0x80, 100)
		records := parseNoteRecords(payload)
		if len(records) != 1 {
			t.Fatalf("parseNoteRecords() = %d records, want 1", len(records))
		}
		want := NoteRecord{Track: "A1", Step: 2, Velocity: 100}
		if records[0] != want {
			t.Errorf("parseNoteRecords()[0] = %+v, want %+v", records[0], want)
		}
	})

	t.Run("track byte without high bit is not a record", func(t *testing.T) {
		payload := buildPayload(3, 0x02, 100)
		if records := parseNoteRecords(payload); len(records) != 0 {
			t.Errorf("parseNoteRecords() = %+v, want no records (high bit unset)", records)
		}
	})

	t.Run("payload shorter than the search origin", func(t *testing.T) {
		if records := parseNoteRecords(make([]byte, 100)); records != nil {
			t.Errorf("parseNoteRecords() = %+v, want nil for too-short payload", records)
		}
	})

	t.Run("no marker byte present", func(t *testing.T) {
		if records := parseNoteRecords(make([]byte, 1100)); len(records) != 0 {
			t.Errorf("parseNoteRecords() = %+v, want no records", records)
		}
	})
}
