package sysex

import "fmt"

// EncodeProject returns a new Project (0x61) unescaped payload built from
// base (an existing project's unescaped payload, used as a template): the
// project header (see projectHeaderLen — its length depends on base's own
// path, not a fixed size) is copied unchanged, and each of the
// BeatsPerProject kit blocks is either copied unchanged from base or, for
// an index present in edits, rebuilt via EncodeBeat against that block as
// its own base — the same base-preservation approach EncodeBeat itself
// uses, extended one level up. Calling ProjectBeats on base and passing an
// empty edits map back to EncodeProject must reproduce base byte-for-byte;
// this is the round-trip guarantee TestEncodeProject_RoundTrip checks
// before any write tool is built on top of this, mirroring EncodeBeat's own
// documented precondition.
func EncodeProject(base []byte, edits map[int]*Kit) ([]byte, error) {
	headerLen, err := projectHeaderLen(base)
	if err != nil {
		return nil, err
	}
	bounds, err := projectBlockBounds(base)
	if err != nil {
		return nil, err
	}

	out := make([]byte, headerLen)
	copy(out, base[:headerLen])

	for i, b := range bounds {
		block := base[b[0]:b[1]]
		if kit, ok := edits[i]; ok {
			encoded, err := EncodeBeat(block, kit)
			if err != nil {
				return nil, fmt.Errorf("beat %d: %w", i, err)
			}
			// EncodeBeat appends a trailing alignment tail meant for a
			// STANDALONE Beat (0x5F) message's own wire packing (see
			// kitBlockLenInProject's doc comment) — a Project-embedded
			// block carries no such tail, so trim it back off here or
			// every beat after this one misaligns.
			encoded = encoded[:kitBlockLenInProject(len(kit.Notes))]
			out = append(out, encoded...)
		} else {
			out = append(out, block...)
		}
	}
	// Preserve any trailing bytes after the last kit block (e.g. a project
	// footer) unchanged — projectBlockBounds only accounts for the header
	// and the 16 kit blocks, not whatever, if anything, follows them.
	if len(bounds) > 0 {
		out = append(out, base[bounds[len(bounds)-1][1]:]...)
	}
	return out, nil
}

// BuildProjectDump wraps an EncodeProject payload in the standard 5-byte
// Project (0x61) SysEx header (F0 mfg dev 0x61 path-length-byte), copied
// verbatim from baseRaw — a Project write never changes the project's own
// name/path, so the header can be reused rather than reconstructed (see
// headerLen's doc comment on the 0x61 path-length-prefix convention).
// Untested against real hardware as a receive path — unlike BuildBeatDump
// (confirmed §9.16-9.17), whether the Tempest accepts a synthesized Project
// import via plain SysEx at all is still an open question.
func BuildProjectDump(baseRaw []byte, payload []byte) ([]byte, error) {
	if Identify(baseRaw) != TypeProjectDump {
		return nil, fmt.Errorf("baseRaw is not a Project (0x61) dump")
	}
	hl := headerLen(TypeProjectDump)
	if len(baseRaw) < hl {
		return nil, fmt.Errorf("baseRaw is %d bytes, shorter than the %d-byte header", len(baseRaw), hl)
	}
	escaped := escapeProjectPayload(payload)
	msg := make([]byte, 0, hl+len(escaped)+1)
	msg = append(msg, baseRaw[:hl]...)
	msg = append(msg, escaped...)
	msg = append(msg, 0xF7)
	return msg, nil
}

// escapeProjectPayload packs data using the same 7+1 collector scheme as
// Escape7Plus1, but — unlike it — does not zero-pad a genuinely short final
// group out to a full 7 bytes. This matters only for Project (0x61)
// payloads: ProjectHeaderLen (349) is not itself a multiple of 7, so a real
// project's total unescaped length never lands on a 7-byte boundary either,
// and a real hardware capture's wire bytes end in a short final group
// rather than one padded with trailing zeros (confirmed empirically: an
// unescaped real capture unpacks to N bytes; Escape7Plus1 followed by
// Unescape7Plus1 on those same N bytes yields N+(7-N%7) bytes instead —
// extra trailing zero padding not present in the original capture).
// Escape7Plus1 itself is left untouched rather than fixed in place, because
// every other caller's content is already an exact multiple of 7 by
// construction (e.g. EncodeBeat's kitTailPattern padding), so this padding
// question never arises for them — isolating the fix to Project writes
// avoids any risk to those separately-relied-upon paths.
func escapeProjectPayload(data []byte) []byte {
	result := make([]byte, 0, (len(data)/7+1)*8)
	for i := 0; i < len(data); i += 7 {
		end := min(i+7, len(data))
		chunk := data[i:end]
		var collector byte
		for k, b := range chunk {
			collector |= ((b >> 7) & 0x01) << uint(k)
		}
		result = append(result, collector)
		for _, b := range chunk {
			result = append(result, b&0x7F)
		}
	}
	return result
}
