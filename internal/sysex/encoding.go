// Package sysex implements the Tempest's SysEx wire encoding, plus
// message-type detection and payload parsing.
//
// One scheme, used by every recognised Tempest message type (0x60 RAM,
// 0x61 Project, 0x63 FLASH, 0x5C alternate sound, 0x5E alternate header,
// 0x5F Beat/Kit): groups of 8 wire bytes containing 1 leading "collector"
// byte followed by 7 data bytes. Bit k of the collector byte is the high
// bit (bit 7) of data byte k in that group.
//
// This replaces an earlier, unverified pair of schemes (a "7 data + 1
// discarded mystery byte" model for 0x60/0x61/0x63, and a "7 data + trailing
// MSB byte" model for 0x5C/0x5E) that turned out not to match real hardware.
//
// Source: reverse-engineered from TempestEdit (bitrotten.com/tempest/editor),
// an unofficial browser-based Tempest editor, by static analysis of its
// unpackPayload/packPayload routines. Confirmed against this project's own
// hardware-captured .syx files: decoding real FLASH (0x63) dumps with this
// scheme recovers exact, byte-perfect "/S/Category/Name" paths, and decoding
// real Beat/Kit (0x5F) dumps recovers exact names and BPM values matching
// their known contents. See docs/sysex-tempest-format.md §3 for details.
package sysex

// unpackCollectorFirst decodes the Tempest wire scheme: each group of 8
// encoded bytes is 1 collector byte followed by 7 data bytes. Bit k (0-6) of
// the collector is OR'd into bit 7 of data byte k.
func unpackCollectorFirst(encoded []byte) []byte {
	result := make([]byte, 0, len(encoded)*7/8)
	i := 0
	for i < len(encoded) {
		collector := encoded[i]
		for k := 0; k < 7 && i+1+k < len(encoded); k++ {
			result = append(result, encoded[i+1+k]|(((collector>>uint(k))&0x01)<<7))
		}
		i += 8
	}
	return result
}

// packCollectorFirst encodes data using the Tempest wire scheme: every 7
// input bytes become 1 collector byte (carrying each byte's high bit) plus
// the 7 bytes with their high bit cleared. A short final chunk is zero-padded
// to 7 bytes.
func packCollectorFirst(data []byte) []byte {
	groups := (len(data) + 6) / 7
	result := make([]byte, 0, groups*8)
	for i := 0; i < len(data); i += 7 {
		end := min(i+7, len(data))
		chunk := data[i:end]

		var collector byte
		for k, b := range chunk {
			collector |= ((b >> 7) & 0x01) << uint(k)
		}
		result = append(result, collector)

		for k := range 7 {
			if k < len(chunk) {
				result = append(result, chunk[k]&0x7F)
			} else {
				result = append(result, 0x00)
			}
		}
	}
	return result
}

// Unescape7Plus1 decodes the Tempest wire scheme (see unpackCollectorFirst),
// used for 0x60 (RAM), 0x61 (Project), 0x63 (FLASH), and 0x5F (Beat/Kit).
func Unescape7Plus1(encoded []byte) []byte {
	return unpackCollectorFirst(encoded)
}

// Escape7Plus1 encodes data using the Tempest wire scheme (see
// packCollectorFirst).
func Escape7Plus1(data []byte) []byte {
	return packCollectorFirst(data)
}

// UnescapeStandard decodes 0x5C (alternate bank sound) and 0x5E (alternate
// bank header) payloads. TempestEdit's source uses a single unpack routine
// for every message type (see package doc) rather than a distinct scheme for
// these two — unlike Unescape7Plus1's callers, this has not been
// independently confirmed against a decoded 0x5C/0x5E hardware capture in
// this repo (only the FLASH/RAM/Beat family has been); revisit if evidence
// to the contrary turns up.
func UnescapeStandard(encoded []byte) []byte {
	return unpackCollectorFirst(encoded)
}

// EscapeStandard encodes data for 0x5C/0x5E messages. See UnescapeStandard.
func EscapeStandard(data []byte) []byte {
	return packCollectorFirst(data)
}
