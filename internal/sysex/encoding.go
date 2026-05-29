// Package sysex implements the two SysEx encoding schemes used by the DSI Tempest,
// plus message-type detection and payload parsing.
//
// Two schemes:
//   - Tempest 7+1: used for 0x60 (RAM), 0x61 (Project), 0x63 (FLASH)
//     Groups of 8: 7 data bytes + 1 mystery byte (purpose unknown, written as 0x00).
//   - Standard DSI 7-of-8: used for 0x5C (bank sound), 0x5E (bank header)
//     Groups of 8: 7 data bytes (high bit cleared) + 1 MSB byte encoding the high bits.
//
// Source: reverse-engineered from KnobKraft Orm DSI_Tempest.py (Christof Ruch, 2022).
package sysex

// Unescape7Plus1 decodes the Tempest-specific 7+1 scheme (for 0x60/0x61/0x63).
// Read 7 bytes, skip 1 mystery byte, repeat.
func Unescape7Plus1(encoded []byte) []byte {
	result := make([]byte, 0, len(encoded)*7/8)
	i := 0
	for i < len(encoded) {
		for j := 0; j < 7; j++ {
			if i < len(encoded) {
				result = append(result, encoded[i])
			}
			i++
		}
		i++ // skip mystery 8th byte
	}
	return result
}

// Escape7Plus1 encodes data using the Tempest 7+1 scheme.
// The mystery 8th byte is written as 0x00 (Tempest accepts this).
func Escape7Plus1(data []byte) []byte {
	groups := (len(data) + 6) / 7
	result := make([]byte, 0, groups*8)
	for i := 0; i < len(data); i += 7 {
		end := i + 7
		if end > len(data) {
			end = len(data)
		}
		chunk := data[i:end]
		result = append(result, chunk...)
		// Pad to 7 bytes if last chunk is short
		for j := len(chunk); j < 7; j++ {
			result = append(result, 0x00)
		}
		result = append(result, 0x00) // mystery byte
	}
	return result
}

// UnescapeStandard decodes the standard DSI 7-of-8 MSB scheme (for 0x5C/0x5E).
// Groups of 8: 7 data bytes (high bit cleared) + 1 MSB byte.
// Bit (6-i) of the MSB byte is the high bit of data byte i.
func UnescapeStandard(encoded []byte) []byte {
	result := make([]byte, 0, len(encoded)*7/8)
	i := 0
	for i+7 < len(encoded) {
		msbByte := encoded[i+7]
		for j := 0; j < 7; j++ {
			b := encoded[i+j]
			msb := (msbByte >> uint(6-j)) & 0x01
			result = append(result, b|(msb<<7))
		}
		i += 8
	}
	// Handle any remaining bytes without a full MSB group
	if i < len(encoded) {
		result = append(result, encoded[i:]...)
	}
	return result
}

// EscapeStandard encodes data using the standard DSI 7-of-8 MSB scheme.
func EscapeStandard(data []byte) []byte {
	groups := (len(data) + 6) / 7
	result := make([]byte, 0, groups*8)
	for i := 0; i < len(data); i += 7 {
		end := i + 7
		if end > len(data) {
			end = len(data)
		}
		chunk := data[i:end]

		var msbByte byte
		for j, b := range chunk {
			if b&0x80 != 0 {
				msbByte |= 1 << uint(6-j)
			}
		}

		// Write 7 data bytes with high bit cleared
		for _, b := range chunk {
			result = append(result, b&0x7F)
		}
		// Pad to 7 if short chunk
		for j := len(chunk); j < 7; j++ {
			result = append(result, 0x00)
		}
		result = append(result, msbByte)
	}
	return result
}
