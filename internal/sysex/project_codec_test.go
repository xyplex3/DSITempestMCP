package sysex_test

import (
	"os"
	"reflect"
	"testing"

	"tempest-mcp/internal/sysex"
)

// loadProjectFixture reads a hardware-captured Project (0x61) .syx file
// (from "Export saved file over MIDI", not the RAM export's 17-message
// stream) and returns its unescaped payload.
func loadProjectFixture(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if msgType := sysex.Identify(raw); msgType != sysex.TypeProjectDump {
		t.Fatalf("%s: got message type %v, want TypeProjectDump", path, msgType)
	}
	return sysex.Unescape(raw)
}

// TestEncodeProject_RoundTrip is the same gate TestEncodeBeat_RoundTrip
// established for single-beat writes, one level up: decode a real hardware
// Project capture, re-encode it with no edits, and the result must be
// byte-for-byte identical to the original before any project-level write
// tool can be trusted.
func TestEncodeProject_RoundTrip(t *testing.T) {
	const path = "testdata/project-research/newbeat2_allempty.syx"
	original := loadProjectFixture(t, path)

	reencoded, err := sysex.EncodeProject(original, nil)
	if err != nil {
		t.Fatalf("EncodeProject() error = %v", err)
	}

	if !reflect.DeepEqual(original, reencoded) {
		if len(original) != len(reencoded) {
			t.Fatalf("EncodeProject() length = %d, want %d", len(reencoded), len(original))
		}
		for i := range original {
			if original[i] != reencoded[i] {
				t.Fatalf("EncodeProject() first mismatch at byte %d: got 0x%02X, want 0x%02X",
					i, reencoded[i], original[i])
			}
		}
	}
}

// TestEncodeProject_EditsOneBeat verifies that EncodeProject can target a
// single beat's notes/name/bpm/swing while leaving the project header and
// every other beat's bytes byte-for-byte untouched.
func TestEncodeProject_EditsOneBeat(t *testing.T) {
	const path = "testdata/project-research/newbeat2_allempty.syx"
	original := loadProjectFixture(t, path)

	beats, err := sysex.ProjectBeats(original)
	if err != nil {
		t.Fatalf("ProjectBeats() error = %v", err)
	}

	const targetIndex = 4 // beat 5
	edited := &sysex.Kit{
		Name:      "Edited Beat",
		ShortName: "Edited",
		BPM:       128.0,
		Swing:     62.5,
		Notes:     []sysex.NoteRecord{{Track: 4, Step: 5, Velocity: 100}},
	}

	encoded, err := sysex.EncodeProject(original, map[int]*sysex.Kit{targetIndex: edited})
	if err != nil {
		t.Fatalf("EncodeProject() error = %v", err)
	}

	redecoded, err := sysex.ProjectBeats(encoded)
	if err != nil {
		t.Fatalf("ProjectBeats() of re-encoded result error = %v", err)
	}
	if len(redecoded) != len(beats) {
		t.Fatalf("ProjectBeats() = %d beats, want %d", len(redecoded), len(beats))
	}

	got := redecoded[targetIndex]
	if got.Name != "Edited Beat" || got.ShortName != "Edited" || got.BPM != 128.0 || got.Swing != 62.5 {
		t.Errorf("beat %d = %+v, want name/short/bpm/swing Edited Beat/Edited/128/62.5", targetIndex, got)
	}
	wantNote := sysex.NoteRecord{Track: 4, Step: 5, Velocity: 100}
	if len(got.Notes) != 1 || got.Notes[0] != wantNote {
		t.Errorf("beat %d notes = %+v, want [%+v]", targetIndex, got.Notes, wantNote)
	}

	for i := range beats {
		if i == targetIndex {
			continue
		}
		if !reflect.DeepEqual(redecoded[i], beats[i]) {
			t.Errorf("beat %d changed unexpectedly: got %+v, want unchanged %+v", i, redecoded[i], beats[i])
		}
	}
}

// TestBuildProjectDump_RoundTrip verifies that wrapping an EncodeProject
// payload with BuildProjectDump and then unescaping it back (as if freshly
// received) reproduces the original unescaped payload exactly, and that
// Identify still recognizes the result as a Project (0x61) dump.
func TestBuildProjectDump_RoundTrip(t *testing.T) {
	const path = "testdata/project-research/newbeat2_allempty.syx"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	unescaped := sysex.Unescape(raw)

	reencoded, err := sysex.EncodeProject(unescaped, nil)
	if err != nil {
		t.Fatalf("EncodeProject() error = %v", err)
	}

	wire, err := sysex.BuildProjectDump(raw, reencoded)
	if err != nil {
		t.Fatalf("BuildProjectDump() error = %v", err)
	}

	if sysex.Identify(wire) != sysex.TypeProjectDump {
		t.Fatalf("Identify(wire) = %v, want TypeProjectDump", sysex.Identify(wire))
	}
	roundTripped := sysex.Unescape(wire)
	if !reflect.DeepEqual(unescaped, roundTripped) {
		t.Fatalf("Unescape(BuildProjectDump(...)) does not reproduce the original unescaped payload")
	}
}
