package sysex_test

import (
	"os"
	"strings"
	"testing"

	"tempest-mcp/internal/sysex"
)

// buildKitBlock constructs a synthetic kit block's unescaped bytes for
// testing KitNoteRecords/ProjectBeats: sysex.KitBlockBaseSize bytes plus
// sysex.KitBlockBytesPerNote per entry in notes, with name, short name,
// BPM, swing, and note records placed at their confirmed offsets (see
// docs/sysex-tempest-format.md §5/§9.1-9.3/§9.10). Deliberately does NOT
// use sysex.KitBlockLen's tail-included size: that tail exists only to
// keep a standalone Beat (0x5F) message's own wire packing aligned, and
// a Project's own embedded blocks carry no such tail (confirmed against
// two real Project captures) — this matches the real no-tail layout, so
// a block built here concatenates correctly into a synthetic Project
// payload via buildProjectPayload without an EncodeProject-style trim.
func buildKitBlock(name, shortName string, bpmRaw uint16, swingRaw byte, notes []sysex.NoteRecord) []byte {
	size := sysex.KitBlockBaseSize + sysex.KitBlockBytesPerNote*len(notes)
	kit := make([]byte, size)
	kit[sysex.KitBPMOffset] = byte(bpmRaw >> 8)
	kit[sysex.KitBPMOffset+1] = byte(bpmRaw)
	kit[sysex.KitSwingOffset] = swingRaw
	copy(kit[sysex.KitNameOffset:], padRight(name, sysex.KitNameLen))
	copy(kit[sysex.KitShortNameOffset:], padRight(shortName, sysex.KitShortNameLen))
	for i, n := range notes {
		off := sysex.KitNoteRecordOffset + i*sysex.KitNoteRecordLen
		kit[off+2] = byte((n.Step - 1) * 3)
		kit[off+3] = 0x77
		kit[off+4] = byte(0x80 | n.Track)
		kit[off+5] = byte(n.Velocity)
	}
	return kit
}

// padRight returns s as a byte slice of exactly n bytes, space-padded.
func padRight(s string, n int) []byte {
	b := make([]byte, n)
	copy(b, s)
	for i := len(s); i < n; i++ {
		b[i] = ' '
	}
	return b
}

// decodeBeatCapture loads a hardware-captured Beat/Kit (0x5F) .syx file
// and returns its decoded note records.
func decodeBeatCapture(t *testing.T, path string) []sysex.NoteRecord {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	msgType := sysex.Identify(raw)
	if msgType != sysex.TypeBeatDump {
		t.Fatalf("%s: got message type %v, want TypeBeatDump", path, msgType)
	}
	unescaped := sysex.Unescape(raw)
	return sysex.KitNoteRecords(unescaped)
}

// TestKitNoteRecords_HardwareCaptures decodes two real hardware captures of
// a controlled two-note Export Beat (docs/sysex-tempest-format.md §9.13):
// A1/step 1 + A2/step 2, and the reversed A2/step 1 + A1/step 2. Both
// captures verified via Beat 1/16-Beats-mode selection before editing, the
// Save/Load "Export Beat over MIDI" -> Source Beat confirmation flow, and
// independently re-run to rule out a one-off. This is the first confirmed
// (not extrapolated) validation of the multi-note record layout.
func TestKitNoteRecords_HardwareCaptures(t *testing.T) {
	const dir = "testdata/beat-research/"

	t.Run("A1 step1, A2 step2", func(t *testing.T) {
		got := decodeBeatCapture(t, dir+"twonote_a1s1_a2s2.syx")
		want := []sysex.NoteRecord{
			{Track: 0, Step: 1, Velocity: 34},
			{Track: 1, Step: 2, Velocity: 45},
		}
		if len(got) != len(want) {
			t.Fatalf("KitNoteRecords() = %+v, want %+v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("record %d = %+v, want %+v", i, got[i], want[i])
			}
		}
	})

	t.Run("reversed: A2 step1, A1 step2", func(t *testing.T) {
		got := decodeBeatCapture(t, dir+"twonote_a2s1_a1s2.syx")
		want := []sysex.NoteRecord{
			{Track: 1, Step: 1, Velocity: 36},
			{Track: 0, Step: 2, Velocity: 49},
		}
		if len(got) != len(want) {
			t.Fatalf("KitNoteRecords() = %+v, want %+v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("record %d = %+v, want %+v", i, got[i], want[i])
			}
		}
	})
}

// TestKitNoteRecords verifies note-record decoding: zero, one, and multiple
// consecutive records, and that decoding stops at the first byte that
// doesn't match the confirmed marker/track-high-bit shape rather than
// scanning past it.
func TestKitNoteRecords(t *testing.T) {
	t.Run("no notes", func(t *testing.T) {
		kit := buildKitBlock("Basic", "Basic", 1100, 0, nil)
		if got := sysex.KitNoteRecords(kit); len(got) != 0 {
			t.Errorf("KitNoteRecords() = %+v, want no records", got)
		}
	})

	t.Run("one note", func(t *testing.T) {
		want := []sysex.NoteRecord{{Track: 2, Step: 1, Velocity: 28}}
		kit := buildKitBlock("Basic", "Basic", 1100, 0, want)
		got := sysex.KitNoteRecords(kit)
		if len(got) != 1 || got[0] != want[0] {
			t.Errorf("KitNoteRecords() = %+v, want %+v", got, want)
		}
	})

	t.Run("two consecutive notes", func(t *testing.T) {
		want := []sysex.NoteRecord{
			{Track: 2, Step: 1, Velocity: 28},
			{Track: 5, Step: 3, Velocity: 100},
		}
		kit := buildKitBlock("Basic", "Basic", 1100, 0, want)
		got := sysex.KitNoteRecords(kit)
		if len(got) != len(want) {
			t.Fatalf("KitNoteRecords() = %+v, want %+v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("record %d = %+v, want %+v", i, got[i], want[i])
			}
		}
	})

	t.Run("stops at first non-matching position", func(t *testing.T) {
		// One real record followed by bytes that don't match the marker
		// shape (as if reading into an unrelated adjacent block).
		kit := buildKitBlock("Basic", "Basic", 1100, 0,
			[]sysex.NoteRecord{{Track: 2, Step: 1, Velocity: 28}})
		kit = append(kit, make([]byte, 40)...) // trailing non-record bytes
		got := sysex.KitNoteRecords(kit)
		if len(got) != 1 {
			t.Errorf("KitNoteRecords() = %+v, want exactly 1 record", got)
		}
	})

	t.Run("track byte without high bit is not a record", func(t *testing.T) {
		kit := buildKitBlock("Basic", "Basic", 1100, 0, nil)
		off := sysex.KitNoteRecordOffset
		kit[off+3] = 0x77 // marker present
		kit[off+4] = 0x02 // high bit unset
		if got := sysex.KitNoteRecords(kit); len(got) != 0 {
			t.Errorf("KitNoteRecords() = %+v, want no records (high bit unset)", got)
		}
	})
}

// buildProjectPayload concatenates a stand-in project header (ProjectBeats
// only reads past it) with the given kit blocks, matching the confirmed
// back-to-back layout (§9.10). The real header length depends on the
// project's own null-terminated path (see projectHeaderLen — ProjectBeats
// reads it from the payload, not a fixed constant), so this builds a
// 27-char placeholder path to land on exactly ProjectHeaderLen (349)
// total, matching every byte offset this test file's assertions assume.
func buildProjectPayload(blocks ...[]byte) []byte {
	payload := make([]byte, sysex.ProjectHeaderLen)
	for i := range 27 {
		payload[i] = 'x' // placeholder path; byte 27 stays 0 as the null terminator
	}
	for _, b := range blocks {
		payload = append(payload, b...)
	}
	return payload
}

// TestProjectBeats_MixedNoteCounts verifies decoding of all 16 beats from a
// synthetic Project payload where the first two beats carry one note each
// and the rest are empty.
func TestProjectBeats_MixedNoteCounts(t *testing.T) {
	blocks := make([][]byte, sysex.BeatsPerProject)
	blocks[0] = buildKitBlock("Initialize", "Basic", 1100, 5,
		[]sysex.NoteRecord{{Track: 2, Step: 1, Velocity: 28}})
	blocks[1] = buildKitBlock("Initialize", "Basic", 1100, 5,
		[]sysex.NoteRecord{{Track: 2, Step: 2, Velocity: 79}})
	for i := 2; i < sysex.BeatsPerProject; i++ {
		blocks[i] = buildKitBlock("Initialize", "Basic", 1100, 5, nil)
	}
	payload := buildProjectPayload(blocks...)

	beats, err := sysex.ProjectBeats(payload)
	if err != nil {
		t.Fatalf("ProjectBeats() error = %v", err)
	}
	if len(beats) != sysex.BeatsPerProject {
		t.Fatalf("ProjectBeats() = %d beats, want %d", len(beats), sysex.BeatsPerProject)
	}

	if beats[0].Name != "Initialize" || beats[0].ShortName != "Basic" {
		t.Errorf("beat 0 name/short = %q/%q, want %q/%q",
			beats[0].Name, beats[0].ShortName, "Initialize", "Basic")
	}
	if beats[0].BPM != 110.0 {
		t.Errorf("beat 0 BPM = %g, want 110.0", beats[0].BPM)
	}
	wantNote := sysex.NoteRecord{Track: 2, Step: 1, Velocity: 28}
	if len(beats[0].Notes) != 1 || beats[0].Notes[0] != wantNote {
		t.Errorf("beat 0 notes = %+v, want one record %+v", beats[0].Notes, wantNote)
	}
	if len(beats[1].Notes) != 1 || beats[1].Notes[0].Step != 2 {
		t.Errorf("beat 1 notes = %+v, want one record at step 2", beats[1].Notes)
	}
	assertEmptyBeats(t, beats[2:])
}

// assertEmptyBeats checks that each beat has no notes and its Index matches
// its position, offset by the number of beats skipped before it.
func assertEmptyBeats(t *testing.T, beats []sysex.ProjectBeat) {
	t.Helper()
	for i, beat := range beats {
		wantIndex := i + (sysex.BeatsPerProject - len(beats))
		if beat.Index != wantIndex {
			t.Errorf("beats[%d].Index = %d, want %d", i, beat.Index, wantIndex)
		}
		if len(beat.Notes) != 0 {
			t.Errorf("beat %d notes = %+v, want none", wantIndex, beat.Notes)
		}
	}
}

// TestProjectBeats_TooShortHeader verifies that a payload shorter than the
// project header is rejected with an error. Uses the same 27-char
// placeholder path as buildProjectPayload so the computed header length
// is still exactly ProjectHeaderLen (349), one byte more than this
// payload provides.
func TestProjectBeats_TooShortHeader(t *testing.T) {
	payload := make([]byte, sysex.ProjectHeaderLen-1)
	for i := range 27 {
		payload[i] = 'x'
	}
	_, err := sysex.ProjectBeats(payload)
	if err == nil {
		t.Fatal("ProjectBeats() expected error for too-short payload, got nil")
	}
}

// TestProjectBeats_TruncatedMidBlock verifies that a payload containing
// fewer than BeatsPerProject blocks returns the beats successfully decoded
// so far alongside an error, rather than panicking or silently truncating.
func TestProjectBeats_TruncatedMidBlock(t *testing.T) {
	blocks := [][]byte{
		buildKitBlock("Initialize", "Basic", 1100, 5, nil),
		buildKitBlock("Initialize", "Basic", 1100, 5, nil),
	}
	payload := buildProjectPayload(blocks...) // only 2 of 16 beats present

	beats, err := sysex.ProjectBeats(payload)
	if err == nil {
		t.Fatal("ProjectBeats() expected an error for a truncated payload, got nil")
	}
	if len(beats) != 2 {
		t.Errorf("ProjectBeats() = %d beats, want the 2 successfully decoded before truncation", len(beats))
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("error = %q, want it to mention truncation", err.Error())
	}
}

// TestProjectBeats_HardwareCapture decodes a real hardware-captured Project
// (0x61) dump whose path is shorter than the original calibration sample's
// (see projectHeaderLen's doc comment) — a project header built on a fixed
// ProjectHeaderLen would misdecode beat 0 at a 3-byte offset and then drift
// further out of alignment on every later beat via KitBlockLen's
// standalone-message tail, which doesn't belong between a Project's own
// back-to-back embedded blocks (see kitBlockLenInProject). Every one of the
// 16 beats in this capture shares the same name/short-name/BPM/swing, so a
// misaligned decode would show it: garbled text, wildly implausible BPM
// values, or decode errors, instead of the same clean values 16 times over.
func TestProjectBeats_HardwareCapture(t *testing.T) {
	raw, err := os.ReadFile("testdata/project-research/newbeat2_allempty.syx")
	if err != nil {
		t.Fatalf("reading capture: %v", err)
	}
	if got := sysex.Identify(raw); got != sysex.TypeProjectDump {
		t.Fatalf("got message type %v, want TypeProjectDump", got)
	}
	unescaped := sysex.Unescape(raw)

	beats, err := sysex.ProjectBeats(unescaped)
	if err != nil {
		t.Fatalf("ProjectBeats() error = %v", err)
	}
	if len(beats) != sysex.BeatsPerProject {
		t.Fatalf("ProjectBeats() = %d beats, want %d", len(beats), sysex.BeatsPerProject)
	}

	for _, b := range beats {
		if b.Name != "Basic Beat" || b.ShortName != "Beat1" {
			t.Errorf("beat %d name/short = %q/%q, want %q/%q",
				b.Index, b.Name, b.ShortName, "Basic Beat", "Beat1")
		}
		if b.BPM != 120 {
			t.Errorf("beat %d BPM = %g, want 120", b.Index, b.BPM)
		}
		if b.Swing != 50 {
			t.Errorf("beat %d swing = %g, want 50", b.Index, b.Swing)
		}
	}

	// Three beats in this capture carry real notes; every other beat has
	// none. A misaligned decode would either miss these (reading into the
	// wrong block) or fabricate note-shaped noise in blocks that have
	// none — checking both directions here, not just that these three are
	// non-empty.
	wantNoteCounts := map[int]int{9: 7, 10: 26, 13: 17}
	for _, b := range beats {
		want := wantNoteCounts[b.Index]
		if len(b.Notes) != want {
			t.Errorf("beat %d notes = %d, want %d", b.Index, len(b.Notes), want)
		}
	}
}
