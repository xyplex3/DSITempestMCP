package sysex

import (
	"crypto/md5"
	"fmt"
	"sort"
	"strings"
)

// Tempest SysEx constants.
const (
	ManufacturerID = 0x01 // Sequential / DSI
	DeviceID       = 0x28 // Tempest

	TypeRAM       = 0x60 // Edit buffer / RAM sound (volatile)
	TypeProject   = 0x61 // Complete project dump (all 16 beats + 32 sounds)
	TypeFLASH     = 0x63 // Permanent FLASH sound (Bank A/B)
	TypeAltSound  = 0x5C // Alternate bank sound (standard DSI encoding)
	TypeAltHeader = 0x5E // Alternate bank global header
	// TypeBeat is a single Beat/Kit export ("Export Beat in RAM over MIDI",
	// available since OS 1.1). Confirmed 2026-09-19 by decoding this
	// project's own hardware-captured .syx files: real 0x5F messages decode
	// (via the Kit layout below) to exact, byte-perfect names and BPM values
	// matching their known contents. See docs/sysex-tempest-format.md §5.
	TypeBeat = 0x5F

	// Sound parameter block sizes (unescaped).
	ParamBlockSizeFLASH = 132 // approximate; 0x63 format
	ParamBlockSizeAlt   = 320 // per-sound in 0x5C bank; includes sequence data

	// Number of sounds per 0x5C bank message and per bank (A or B).
	SoundsPerBank    = 16
	AltBankHeaderOff = 40 // approximate byte offset where sounds begin in 0x5C

	// Beat/Kit (0x5F) container layout — offsets into the unescaped payload.
	// Confirmed against real hardware-captured .syx files (see
	// docs/sysex-tempest-format.md §5); step/track sequencer offsets past
	// KitSequencerOffset remain unknown and still need a beat-mapper capture
	// session.
	KitBPMOffset       = 4  // 2 bytes, big-endian; BPM = raw/10
	KitSwingOffset     = 6  // 1 byte, raw 0-12 -> 50%-75% swing, linear
	KitNameOffset      = 24 // 20 bytes, space-padded ASCII (not null-terminated)
	KitNameLen         = 20
	KitShortNameOffset = 44 // 8 bytes, space-padded ASCII
	KitShortNameLen    = 8
	KitPadTableOffset  = 52
	KitPadEntryLen     = 30
	KitPadEntryCount   = 32
	KitSequencerOffset = KitPadTableOffset + KitPadEntryLen*KitPadEntryCount // 1012
)

// MessageType classifies a raw SysEx byte slice by its Tempest message type.
// The zero value is [TypeUnknown].
type MessageType int

// MessageType values returned by [Identify].
const (
	TypeUnknown        MessageType = 0             // not a recognised Tempest SysEx message
	TypeRAMSound       MessageType = TypeRAM       // 0x60 volatile edit-buffer sound
	TypeProjectDump    MessageType = TypeProject   // 0x61 complete project dump
	TypeFLASHSound     MessageType = TypeFLASH     // 0x63 permanent bank sound (A/B)
	TypeAlternateSound MessageType = TypeAltSound  // 0x5C alternate bank sound
	TypeAlternateBank  MessageType = TypeAltHeader // 0x5E alternate bank global header

	// TypeBeatDump is a single Beat/Kit export. See the TypeBeat constant
	// above for confirmation details.
	TypeBeatDump MessageType = TypeBeat
)

// SplitMessages splits a raw byte slice into individual SysEx messages,
// each delimited by F0 (start) and F7 (end). Bytes outside F0…F7 pairs
// are ignored. Overlapping or unclosed messages are discarded.
func SplitMessages(data []byte) [][]byte {
	var out [][]byte
	start := -1
	for i, b := range data {
		switch {
		case b == 0xF0:
			start = i
		case b == 0xF7 && start >= 0:
			msg := make([]byte, i-start+1)
			copy(msg, data[start:i+1])
			out = append(out, msg)
			start = -1
		}
	}
	return out
}

// Identify returns the MessageType for a raw SysEx message.
// Returns TypeUnknown if the message is not a recognised Tempest SysEx.
func Identify(raw []byte) MessageType {
	if len(raw) < 4 {
		return TypeUnknown
	}
	if raw[0] != 0xF0 || raw[1] != ManufacturerID || raw[2] != DeviceID {
		return TypeUnknown
	}
	switch raw[3] {
	case TypeRAM:
		return TypeRAMSound
	case TypeProject:
		return TypeProjectDump
	case TypeFLASH:
		return TypeFLASHSound
	case TypeAltSound:
		return TypeAlternateSound
	case TypeAltHeader:
		return TypeAlternateBank
	case TypeBeat:
		return TypeBeatDump
	}
	return TypeUnknown
}

// headerLen returns the number of leading bytes — F0, manufacturer, device,
// type, and (for some types) one extra byte — before the escaped payload
// begins. FLASH (0x63) and alternate-bank sound (0x5C) carry that extra byte;
// for FLASH it is a name/path-length prefix (see BuildFLASHDump), confirmed
// by decoding real hardware captures. Every other recognised type has a
// plain 4-byte header. Source: TempestEdit's headerLen()
// (docs/sysex-tempest-format.md §2).
func headerLen(t MessageType) int {
	switch t {
	case TypeFLASHSound, TypeAlternateSound:
		return 5
	default:
		return 4
	}
}

// Location returns byte [4] of the message.
//
// For 0x5C (alternate bank sound) this is a genuine 0-indexed bank slot —
// confirmed against a real 16-sound bank capture, where 16 consecutive 0x5C
// messages carry byte[4] = 0x00..0x0F in order.
//
// For FLASH (0x63) this byte is NOT a slot: it is a name/path-length prefix
// (see headerLen and BuildFLASHDump), confirmed by decoding real FLASH
// captures' embedded "/S/Category/Name" paths. Callers must not treat it as
// a bank/slot for FLASH messages — see BankSlot.
func Location(raw []byte) uint8 {
	if len(raw) < 5 {
		return 0
	}
	return raw[4]
}

// BankSlot returns the friendly bank name (A/B) and 1-indexed slot for a
// location byte. Only meaningful for 0x5C (alternate bank sound) messages —
// see Location.
func BankSlot(loc uint8) (bank string, slot int) {
	if loc < 16 {
		return "A", int(loc) + 1
	}
	return "B", int(loc-16) + 1
}

// Payload returns the escaped payload bytes, i.e. everything between the
// type-dependent header (see headerLen) and the trailing F7.
func Payload(raw []byte) []byte {
	hl := headerLen(Identify(raw))
	if len(raw) < hl+2 {
		return nil
	}
	return raw[hl : len(raw)-1]
}

// Unescape decodes the payload of a message using the appropriate scheme.
func Unescape(raw []byte) []byte {
	t := Identify(raw)
	payload := Payload(raw)
	switch t {
	case TypeAlternateSound, TypeAlternateBank:
		return UnescapeStandard(payload)
	default:
		return Unescape7Plus1(payload)
	}
}

// ExtractName reads the sound/beat name from an unescaped payload block.
// Returns ("", 0) for RAM sounds (bit-packed name field location unconfirmed
// — see docs/sysex-tempest-format.md §4). For Beat/Kit dumps, reads the
// fixed-offset space-padded name field (see KitNameOffset). Everything else
// (FLASH, Project) reads a null-terminated ASCII run from the start of the
// block.
func ExtractName(unescaped []byte, msgType MessageType) (name string, nameEndIdx int) {
	switch msgType {
	case TypeRAMSound:
		return "", 0
	case TypeBeatDump:
		return extractKitName(unescaped), KitNameOffset + KitNameLen
	}
	for i, b := range unescaped {
		if b == 0 {
			return string(unescaped[:i]), i
		}
	}
	// No null terminator found — treat entire block as name
	return string(unescaped), len(unescaped)
}

// extractKitName reads the fixed-offset, space-padded 20-char name field of
// a Beat/Kit (0x5F) unescaped payload. Confirmed against real hardware
// captures — see the TypeBeat constant doc comment.
func extractKitName(unescaped []byte) string {
	if len(unescaped) < KitNameOffset+KitNameLen {
		return ""
	}
	return strings.TrimRight(string(unescaped[KitNameOffset:KitNameOffset+KitNameLen]), " \x00")
}

// KitBPM decodes the BPM field of an unescaped Beat/Kit (0x5F) payload.
func KitBPM(unescaped []byte) float64 {
	if len(unescaped) < KitBPMOffset+2 {
		return 0
	}
	raw := int(unescaped[KitBPMOffset])*256 + int(unescaped[KitBPMOffset+1])
	return float64(raw) / 10
}

// KitSwing decodes the swing field (50%-75%) of an unescaped Beat/Kit payload.
func KitSwing(unescaped []byte) float64 {
	if len(unescaped) <= KitSwingOffset {
		return 50
	}
	return 50 + float64(unescaped[KitSwingOffset])*(75-50)/12
}

// ExtractParams returns the parameter block from an unescaped payload.
// For FLASH/Project: everything after the null-terminated name.
// For RAM: the entire unescaped block.
func ExtractParams(unescaped []byte, msgType MessageType) []byte {
	if msgType == TypeRAMSound {
		return unescaped
	}
	_, nameEnd := ExtractName(unescaped, msgType)
	if nameEnd+1 >= len(unescaped) {
		return nil
	}
	return unescaped[nameEnd+1:]
}

// Fingerprint returns an MD5 hex string of the sound parameters,
// independent of name or location, for duplicate detection.
func Fingerprint(raw []byte) (string, error) {
	t := Identify(raw)
	if t == TypeUnknown {
		return "", fmt.Errorf("not a recognised Tempest SysEx message")
	}

	var hashInput []byte

	switch t {
	case TypeRAMSound:
		// No confirmed name field — hash the escaped payload directly.
		p := Payload(raw)
		if p == nil {
			return "", fmt.Errorf("RAM dump too short")
		}
		hashInput = p

	case TypeAlternateSound:
		// Hash entire unescaped data
		hashInput = UnescapeStandard(Payload(raw))

	case TypeFLASHSound, TypeProjectDump:
		unescaped := Unescape7Plus1(Payload(raw))
		params := ExtractParams(unescaped, t)
		hashInput = params

	default:
		return "", fmt.Errorf("fingerprinting not supported for message type 0x%02X", raw[3])
	}

	sum := md5.Sum(hashInput)
	return fmt.Sprintf("%x", sum), nil
}

// BuildFLASHDump constructs a complete FLASH sound dump (0x63) from a sound
// name/path and its parameter bytes, matching the format confirmed by
// decoding real hardware captures: a 5-byte header (F0 mfg dev 0x63
// pathLen) followed by the escaped [name+0x00 terminator, params] payload.
// pathLen (byte [4]) is the length of the name including its terminator, not
// a bank/slot — the Tempest does not appear to encode a destination slot in
// this message at all; slot assignment happens via the front-panel Save/Load
// prompt when the dump is received. See docs/sysex-tempest-format.md §2.
func BuildFLASHDump(soundName string, params []byte) []byte {
	nameBytes := []byte(soundName)
	nameBytes = append(nameBytes, 0x00) // terminator, counted in pathLen
	pathLen := len(nameBytes)
	payload := make([]byte, 0, len(nameBytes)+len(params))
	payload = append(payload, nameBytes...)
	payload = append(payload, params...)
	escaped := Escape7Plus1(payload)

	msg := make([]byte, 0, 5+len(escaped)+1)
	msg = append(msg, 0xF0, ManufacturerID, DeviceID, TypeFLASH, byte(pathLen))
	msg = append(msg, escaped...)
	msg = append(msg, 0xF7)
	return msg
}

// RenameFLASH returns a copy of a FLASH sound dump with a new name.
func RenameFLASH(raw []byte, newName string) ([]byte, error) {
	if Identify(raw) != TypeFLASHSound {
		return nil, fmt.Errorf("can only rename FLASH (0x63) sound dumps")
	}
	unescaped := Unescape7Plus1(Payload(raw))
	params := ExtractParams(unescaped, TypeFLASHSound)
	return BuildFLASHDump(newName, params), nil
}

// referenceSignature is the known 16-byte pattern at the start of a
// Tempest sound parameter block (factory/default initialisation values).
var referenceSignature = []byte{
	0x24, 0x19, 0x00, 0x10, 0x49, 0x06, 0x00, 0x04,
	0x14, 0x1d, 0x00, 0x20, 0x50, 0x36, 0x23, 0x00,
}

// ExtractSoundsFromProject scans a 0x61 project dump for embedded sound
// parameter blocks and returns them as individual 0x63 FLASH dumps.
// minQuality (0–100) filters low-confidence matches; default 70.
func ExtractSoundsFromProject(raw []byte, minQuality int, namePrefix string) ([][]byte, error) {
	if Identify(raw) != TypeProjectDump {
		return nil, fmt.Errorf("ExtractSoundsFromProject: not a project dump (0x61)")
	}
	if minQuality <= 0 {
		minQuality = 70
	}
	if namePrefix == "" {
		namePrefix = "Sound"
	}

	unescaped := Unescape7Plus1(Payload(raw))
	sigLen := len(referenceSignature)
	paramLen := ParamBlockSizeFLASH

	type candidate struct {
		offset  int
		params  []byte
		quality int
	}
	var candidates []candidate

	for offset := 0; offset+paramLen <= len(unescaped); offset++ {
		matches := 0
		for i := range sigLen {
			if unescaped[offset+i] == referenceSignature[i] {
				matches++
			}
		}
		if matches < 12 {
			continue
		}
		params := make([]byte, paramLen)
		copy(params, unescaped[offset:offset+paramLen])
		q := soundQuality(params)
		if q >= minQuality {
			candidates = append(candidates, candidate{offset, params, q})
		}
	}

	// Sort by quality descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].quality > candidates[j].quality
	})

	// Deduplicate: keep best within 64-byte window
	const minDist = 64
	unique := candidates[:0]
	lastOffset := -minDist - 1
	for _, c := range candidates {
		if c.offset-lastOffset >= minDist {
			unique = append(unique, c)
			lastOffset = c.offset
		}
	}

	var results [][]byte
	for i, c := range unique {
		name := fmt.Sprintf("%s %d", namePrefix, i+1)
		dump := BuildFLASHDump(name, c.params)
		results = append(results, dump)
	}
	return results, nil
}

// soundQuality scores a candidate parameter block 0–100.
func soundQuality(params []byte) int {
	if len(params) == 0 {
		return 0
	}
	// Signature similarity (40 pts)
	sigMatches := 0
	for i, b := range referenceSignature {
		if i < len(params) && params[i] == b {
			sigMatches++
		}
	}
	score := sigMatches * 40 / len(referenceSignature)

	// Data variety (30 pts)
	unique := make(map[byte]struct{})
	for _, b := range params {
		unique[b] = struct{}{}
	}
	score += min(30, len(unique)/2)

	// Non-zero (15 pts)
	nonZero := 0
	for _, b := range params {
		if b != 0 {
			nonZero++
		}
	}
	score += nonZero * 15 / len(params)

	// Valid range 0–127 (15 pts)
	valid := 0
	for _, b := range params {
		if b <= 0x7F {
			valid++
		}
	}
	score += valid * 15 / len(params)

	if score > 100 {
		return 100
	}
	return score
}
