package sysex

import "fmt"

// kitFillerLen is the size, in bytes, of the region between the end of a
// kit block's note records and its trailing tail padding. It is constant
// regardless of note count — KitBlockBaseSize (a 0-note block's total
// size) minus KitNoteRecordOffset (where note records begin) — so its
// absolute position shifts by KitBlockBytesPerNote for each active note,
// but its length never does. Nothing about its contents is confirmed
// (docs/sysex-tempest-format.md §9.2's "still-unknown constant bytes" and
// the pad-table region both fall inside earlier parts of the block, not
// this filler); EncodeBeat copies it verbatim from the base payload rather
// than reconstructing it.
const kitFillerLen = KitBlockBaseSize - KitNoteRecordOffset

// kitTailPattern is the fixed padding pattern appended after a kit
// block's content to keep the SysEx 7+1 wire packing aligned, confirmed
// against TempestEdit's own source (docs/sysex-tempest-format.md §9.11)
// and real-capture tail bytes for 0/1/2-note blocks.
var kitTailPattern = []byte{2, 0, 64, 0, 16, 0}

// kitTailLen returns the number of tail padding bytes a kit block of the
// given content length needs, per TempestEdit's [content][tail] model:
// sized to (7 - content%7) % 7 so the total stays a multiple of 7 bytes.
func kitTailLen(contentLen int) int {
	return (7 - contentLen%7) % 7
}

// Kit holds a Beat/Kit block's confirmed fields: name, tempo, and note
// records. Everything else in a kit block's bytes (the pad table, and
// still-unconfirmed header fields between offset 7 and KitNameOffset) is
// not represented here — EncodeBeat preserves those bytes unchanged from
// its base payload rather than reconstructing them from this struct.
type Kit struct {
	Name      string
	ShortName string
	BPM       float64
	Swing     float64
	// Notes holds the decoded note records. See KitNoteRecords' doc
	// comment: confirmed for up to three simultaneous notes (§9.15); more
	// remain untested. Note also that exporting three simultaneous notes
	// from the Tempest itself is not perfectly reliable (§9.15) — one of
	// two identical controlled attempts corrupted a note's step field —
	// independent of whether this package's own encoding is correct.
	Notes []NoteRecord
}

// DecodeBeat decodes a Beat/Kit block's confirmed fields (kit is the
// block's own unescaped bytes, starting at its offset 0 — the same
// convention as KitNoteRecords) into a Kit. It does not modify kit.
func DecodeBeat(kit []byte) (*Kit, error) {
	if len(kit) < KitNoteRecordOffset {
		return nil, fmt.Errorf("kit payload too short: %d bytes, want at least %d", len(kit), KitNoteRecordOffset)
	}
	return &Kit{
		Name:      extractKitName(kit),
		ShortName: extractKitShortName(kit),
		BPM:       KitBPM(kit),
		Swing:     KitSwing(kit),
		Notes:     KitNoteRecords(kit),
	}, nil
}

// noteRecordFirstByte returns relative offset 0's value for the first
// (index 0) record in a kit holding noteCount active notes. Confirmed
// against real hardware across 1, 2 (two independent captures), and 3
// notes (docs/sysex-tempest-format.md §9.15): 6, 11, and 16
// respectively — an exact fit for 1 + 5*noteCount. Every position after
// the first reads 0 regardless of note count, confirmed across the same
// captures (see noteRecordFirstByte's callers). Not confirmed beyond 3
// simultaneous notes; treat as a strong extrapolation past that point,
// not a certainty.
func noteRecordFirstByte(position, noteCount int) byte {
	if position != 0 {
		return 0
	}
	return byte(1 + 5*noteCount)
}

// EncodeBeat returns a new kit block built from base (an existing kit
// block's unescaped bytes, used as a template) with the given Kit's name,
// short name, BPM, swing, and note records written at their confirmed
// offsets. Note records are synthesized fully from kit — including their
// non-confirmed bytes, using noteRecordFirstByte for relative offset 0
// and the confirmed-constant values for offsets 1 and 6-9 (§9.15) — so
// kit.Notes does not need to match what was in base. Every other
// byte — the pad table, the still-unconfirmed header fields between
// offset 7 and KitNameOffset, and the filler region between the note
// records and the tail — is copied unchanged from base rather than
// reconstructed, since this package does not know what those bytes mean.
// The trailing tail padding is recomputed to match kit's note count.
//
// Calling DecodeBeat on base and passing the unmodified result straight
// back to EncodeBeat must reproduce base byte-for-byte — this is the
// round-trip guarantee TestEncodeBeat_RoundTrip checks against real
// hardware captures, and it must hold before any write tool
// (tempest_write_beat) is built on top of this. A capture session this
// package's tests are built from also found that 3-simultaneous-note
// exports are themselves not perfectly reliable on real hardware — one
// of two identical attempts corrupted a note's step field — so treat any
// write built on this as needing a read-back verification step, not a
// fire-and-forget operation.
func EncodeBeat(base []byte, kit *Kit) ([]byte, error) {
	if len(kit.Name) > KitNameLen {
		return nil, fmt.Errorf("name %q is %d bytes, want at most %d", kit.Name, len(kit.Name), KitNameLen)
	}
	if len(kit.ShortName) > KitShortNameLen {
		return nil, fmt.Errorf("short name %q is %d bytes, want at most %d", kit.ShortName, len(kit.ShortName), KitShortNameLen)
	}

	baseNotes := KitNoteRecords(base)
	fillerStart := KitNoteRecordOffset + KitNoteRecordLen*len(baseNotes)
	fillerEnd := fillerStart + kitFillerLen
	if fillerEnd > len(base) {
		return nil, fmt.Errorf("base payload too short: %d bytes, want at least %d (for %d existing note(s))",
			len(base), fillerEnd, len(baseNotes))
	}

	out := make([]byte, KitNoteRecordOffset)
	copy(out, base[:KitNoteRecordOffset])

	bpmRaw := uint16(kit.BPM * 10)
	out[KitBPMOffset] = byte(bpmRaw >> 8)
	out[KitBPMOffset+1] = byte(bpmRaw)
	out[KitSwingOffset] = swingToRaw(kit.Swing)
	copy(out[KitNameOffset:KitNameOffset+KitNameLen], padRightBeat(kit.Name, KitNameLen))
	copy(out[KitShortNameOffset:KitShortNameOffset+KitShortNameLen], padRightBeat(kit.ShortName, KitShortNameLen))

	for i, n := range kit.Notes {
		var rec [KitNoteRecordLen]byte
		rec[0] = noteRecordFirstByte(i, len(kit.Notes))
		rec[1] = 0x00
		rec[2] = byte((n.Step - 1) * 3)
		rec[3] = 0x77
		rec[4] = byte(0x80 | n.Track)
		rec[5] = byte(n.Velocity)
		copy(rec[6:10], []byte{0x02, 0x00, 0x00, 0x00})
		out = append(out, rec[:]...)
	}

	out = append(out, base[fillerStart:fillerEnd]...)

	contentLen := KitBlockBaseSize + KitBlockBytesPerNote*len(kit.Notes)
	tail := kitTailPattern[:kitTailLen(contentLen)]
	out = append(out, tail...)

	return out, nil
}

// swingToRaw inverts KitSwing's raw-to-percent formula (50 +
// raw*(75-50)/12), rounding to the nearest representable raw value and
// clamping to the confirmed 0-12 range.
func swingToRaw(percent float64) byte {
	raw := int((percent-50)*12/(75-50) + 0.5)
	raw = max(raw, 0)
	raw = min(raw, 12)
	return byte(raw)
}

// padRightBeat returns s as a byte slice of exactly n bytes, space-padded
// — the same convention KitNameOffset/KitShortNameOffset fields use.
func padRightBeat(s string, n int) []byte {
	b := make([]byte, n)
	copy(b, s)
	for i := len(s); i < n; i++ {
		b[i] = ' '
	}
	return b
}

// BuildBeatDump wraps an EncodeBeat kit payload in the standard 4-byte
// Beat/Kit (0x5F) SysEx header (F0 mfg dev 0x5F — TypeBeatDump takes no
// extra path-length byte, per headerLen) and escapes it for the wire.
// Untested against real hardware as a receive path — sysex.KitNoteRecords
// and EncodeBeat are confirmed for the export/decode direction; whether
// the Tempest accepts a payload built this way and where it routes it
// (which beat slot) is the open question tempest_write_beat needs to
// answer before it can be trusted.
func BuildBeatDump(kitPayload []byte) []byte {
	escaped := Escape7Plus1(kitPayload)
	msg := make([]byte, 0, 4+len(escaped)+1)
	msg = append(msg, 0xF0, ManufacturerID, DeviceID, TypeBeat)
	msg = append(msg, escaped...)
	msg = append(msg, 0xF7)
	return msg
}
