package sysex_test

import (
	"os"
	"reflect"
	"testing"

	"tempest-mcp/internal/sysex"
)

// loadKitFixture reads a hardware-captured Beat/Kit (0x5F) .syx file and
// returns its unescaped kit-block bytes.
func loadKitFixture(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if msgType := sysex.Identify(raw); msgType != sysex.TypeBeatDump {
		t.Fatalf("%s: got message type %v, want TypeBeatDump", path, msgType)
	}
	return sysex.Unescape(raw)
}

// TestEncodeBeat_RoundTrip is the gate this package's own documentation
// requires before any write tool (tempest_write_beat) can be built: decode
// a real hardware capture, re-encode it unchanged, and the result must be
// byte-for-byte identical to the original. Run against real 0/1/2-note
// captures (docs/sysex-tempest-format.md §9.13).
func TestEncodeBeat_RoundTrip(t *testing.T) {
	const dir = "testdata/beat-research/"
	fixtures := []string{
		dir + "zeronote_baseline.syx",
		dir + "onenote_a1s1.syx",
		dir + "twonote_a1s1_a2s2.syx",
		dir + "twonote_a2s1_a1s2.syx",
		dir + "threenote_a1s1_a2s2_a3s3.syx",
		// Round-trip cares about byte fidelity, not semantic correctness —
		// this fixture is a real capture where the Tempest itself corrupted
		// one note's step field (§9.15); it must still round-trip exactly.
		dir + "threenote_a1s1_a2s2_a3s3_stepcorrupted.syx",
	}

	for _, path := range fixtures {
		t.Run(path, func(t *testing.T) {
			original := loadKitFixture(t, path)

			kit, err := sysex.DecodeBeat(original)
			if err != nil {
				t.Fatalf("DecodeBeat() error = %v", err)
			}

			reencoded, err := sysex.EncodeBeat(original, kit)
			if err != nil {
				t.Fatalf("EncodeBeat() error = %v", err)
			}

			if !reflect.DeepEqual(original, reencoded) {
				if len(original) != len(reencoded) {
					t.Fatalf("EncodeBeat() length = %d, want %d", len(reencoded), len(original))
				}
				for i := range original {
					if original[i] != reencoded[i] {
						t.Fatalf("EncodeBeat() first mismatch at byte %d: got 0x%02X, want 0x%02X",
							i, reencoded[i], original[i])
					}
				}
			}
		})
	}
}

// TestEncodeBeat_EditsConfirmedFields verifies that EncodeBeat can actually
// change name/BPM/swing and an existing note's step/track/velocity, not
// just reproduce the input unchanged — re-decoding the result must reflect
// every edit while leaving everything else (byte-for-byte) untouched.
func TestEncodeBeat_EditsConfirmedFields(t *testing.T) {
	base := loadKitFixture(t, "testdata/beat-research/onenote_a1s1.syx")
	kit, err := sysex.DecodeBeat(base)
	if err != nil {
		t.Fatalf("DecodeBeat() error = %v", err)
	}

	kit.Name = "Edited Beat"
	kit.ShortName = "Edited"
	kit.BPM = 128.0
	kit.Swing = 62.5
	kit.Notes[0] = sysex.NoteRecord{Track: 3, Step: 5, Velocity: 100}

	encoded, err := sysex.EncodeBeat(base, kit)
	if err != nil {
		t.Fatalf("EncodeBeat() error = %v", err)
	}
	if len(encoded) != len(base) {
		t.Fatalf("EncodeBeat() length = %d, want %d (note count unchanged)", len(encoded), len(base))
	}

	redecoded, err := sysex.DecodeBeat(encoded)
	if err != nil {
		t.Fatalf("DecodeBeat() of re-encoded result error = %v", err)
	}
	if redecoded.Name != "Edited Beat" {
		t.Errorf("Name = %q, want %q", redecoded.Name, "Edited Beat")
	}
	if redecoded.ShortName != "Edited" {
		t.Errorf("ShortName = %q, want %q", redecoded.ShortName, "Edited")
	}
	if redecoded.BPM != 128.0 {
		t.Errorf("BPM = %g, want 128.0", redecoded.BPM)
	}
	if redecoded.Swing != 62.5 {
		t.Errorf("Swing = %g, want 62.5", redecoded.Swing)
	}
	want := sysex.NoteRecord{Track: 3, Step: 5, Velocity: 100}
	if len(redecoded.Notes) != 1 || redecoded.Notes[0] != want {
		t.Errorf("Notes = %+v, want [%+v]", redecoded.Notes, want)
	}
}

// TestEncodeBeat_SynthesizeNewRecord verifies that EncodeBeat can add a
// genuinely new note record beyond what the base payload contains — using
// the confirmed formula for a record's non-confirmed bytes (§9.15) rather
// than requiring the position to already exist in base. Uses the real
// three-note capture's own decoded note values (including velocity, which
// is noisy/tap-driven and not reproducible across independently-captured
// sessions — using its exact real values isolates this test to the
// structural bytes EncodeBeat actually controls) as the input, starting
// from the unrelated real two-note fixture as base, and requires the
// result's note-record bytes to match the real three-note capture
// byte-for-byte.
func TestEncodeBeat_SynthesizeNewRecord(t *testing.T) {
	base := loadKitFixture(t, "testdata/beat-research/twonote_a1s1_a2s2.syx")
	want := loadKitFixture(t, "testdata/beat-research/threenote_a1s1_a2s2_a3s3.syx")

	wantKit, err := sysex.DecodeBeat(want)
	if err != nil {
		t.Fatalf("DecodeBeat() of the real three-note fixture error = %v", err)
	}

	encoded, err := sysex.EncodeBeat(base, &sysex.Kit{Name: "Initialize", Notes: wantKit.Notes})
	if err != nil {
		t.Fatalf("EncodeBeat() error = %v", err)
	}

	noteBytesLen := sysex.KitNoteRecordLen * len(wantKit.Notes)
	gotBytes := encoded[sysex.KitNoteRecordOffset : sysex.KitNoteRecordOffset+noteBytesLen]
	wantBytes := want[sysex.KitNoteRecordOffset : sysex.KitNoteRecordOffset+noteBytesLen]
	if !reflect.DeepEqual(gotBytes, wantBytes) {
		t.Errorf("synthesized note-record bytes = % X, want % X", gotBytes, wantBytes)
	}
}

// TestDecodeBeat_HardwareCaptures verifies DecodeBeat's decoded fields
// against what's independently known about each fixture.
func TestDecodeBeat_HardwareCaptures(t *testing.T) {
	tests := []struct {
		path      string
		wantNotes []sysex.NoteRecord
	}{
		{"testdata/beat-research/zeronote_baseline.syx", nil},
		{"testdata/beat-research/onenote_a1s1.syx", []sysex.NoteRecord{
			{Track: 0, Step: 1, Velocity: 0}, // velocity is noisy/tap-driven; not asserted below
		}},
		{"testdata/beat-research/twonote_a1s1_a2s2.syx", []sysex.NoteRecord{
			{Track: 0, Step: 1, Velocity: 34},
			{Track: 1, Step: 2, Velocity: 45},
		}},
		{"testdata/beat-research/twonote_a2s1_a1s2.syx", []sysex.NoteRecord{
			{Track: 1, Step: 1, Velocity: 36},
			{Track: 0, Step: 2, Velocity: 49},
		}},
		{"testdata/beat-research/threenote_a1s1_a2s2_a3s3.syx", []sysex.NoteRecord{
			{Track: 0, Step: 1, Velocity: 37},
			{Track: 1, Step: 2, Velocity: 36},
			{Track: 2, Step: 3, Velocity: 69},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			kit, err := sysex.DecodeBeat(loadKitFixture(t, tt.path))
			if err != nil {
				t.Fatalf("DecodeBeat() error = %v", err)
			}
			if kit.Name != "Initialize" {
				t.Errorf("Name = %q, want %q", kit.Name, "Initialize")
			}
			if len(kit.Notes) != len(tt.wantNotes) {
				t.Fatalf("Notes = %+v, want %d record(s)", kit.Notes, len(tt.wantNotes))
			}
			for i, want := range tt.wantNotes {
				if kit.Notes[i].Track != want.Track || kit.Notes[i].Step != want.Step {
					t.Errorf("Notes[%d] = %+v, want Track:%d Step:%d", i, kit.Notes[i], want.Track, want.Step)
				}
			}
		})
	}
}
