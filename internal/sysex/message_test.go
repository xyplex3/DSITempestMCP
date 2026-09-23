package sysex_test

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"strings"
	"testing"

	"tempest-mcp/internal/sysex"
)

// packRAMName builds a minimal unescaped RAM payload with name packed at
// sysex.SoundNameBitOffset, for round-trip-testing sysex.ExtractName.
func packRAMName(t *testing.T, name string) []byte {
	t.Helper()
	padded := name + strings.Repeat(" ", sysex.SoundNameLen-len(name))
	totalBits := sysex.SoundNameBitOffset + sysex.SoundNameLen*7
	buf := make([]byte, (totalBits+7)/8)
	for c := 0; c < len(padded); c++ {
		val := padded[c]
		for b := range 7 {
			if val&(1<<b) == 0 {
				continue
			}
			globalBit := sysex.SoundNameBitOffset + c*7 + b
			buf[globalBit/8] |= 1 << (globalBit % 8)
		}
	}
	return buf
}

// TestIdentify verifies that Identify classifies SysEx messages correctly.
func TestIdentify(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want sysex.MessageType
	}{
		{
			name: "too short",
			raw:  []byte{0xF0, 0x01, 0x28},
			want: sysex.TypeUnknown,
		},
		{
			name: "wrong start byte",
			raw:  []byte{0x00, 0x01, 0x28, 0x63},
			want: sysex.TypeUnknown,
		},
		{
			name: "wrong manufacturer",
			raw:  []byte{0xF0, 0x02, 0x28, 0x63},
			want: sysex.TypeUnknown,
		},
		{
			name: "wrong device",
			raw:  []byte{0xF0, 0x01, 0x01, 0x63},
			want: sysex.TypeUnknown,
		},
		{
			name: "FLASH sound (0x63)",
			raw:  []byte{0xF0, 0x01, 0x28, 0x63, 0x00},
			want: sysex.TypeFLASHSound,
		},
		{
			name: "RAM sound (0x60)",
			raw:  []byte{0xF0, 0x01, 0x28, 0x60, 0x00},
			want: sysex.TypeRAMSound,
		},
		{
			name: "project dump (0x61)",
			raw:  []byte{0xF0, 0x01, 0x28, 0x61, 0x00},
			want: sysex.TypeProjectDump,
		},
		{
			name: "alternate sound (0x5C)",
			raw:  []byte{0xF0, 0x01, 0x28, 0x5C, 0x00},
			want: sysex.TypeAlternateSound,
		},
		{
			name: "alternate bank header (0x5E)",
			raw:  []byte{0xF0, 0x01, 0x28, 0x5E, 0x00},
			want: sysex.TypeAlternateBank,
		},
		{
			name: "Beat/Kit dump (0x5F)",
			raw:  []byte{0xF0, 0x01, 0x28, 0x5F, 0x00},
			want: sysex.TypeBeatDump,
		},
		{
			name: "beat file export (0x62)",
			raw:  []byte{0xF0, 0x01, 0x28, 0x62, 0x00},
			want: sysex.TypeBeatFileDump,
		},
		{
			name: "unknown type byte",
			raw:  []byte{0xF0, 0x01, 0x28, 0x01, 0x00},
			want: sysex.TypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sysex.Identify(tt.raw)
			if got != tt.want {
				t.Errorf("Identify() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestLocation verifies the Location byte extractor.
func TestLocation(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want uint8
	}{
		{
			name: "too short returns 0",
			raw:  []byte{0xF0, 0x01, 0x28, 0x63},
			want: 0,
		},
		{
			name: "location byte 0",
			raw:  []byte{0xF0, 0x01, 0x28, 0x63, 0x00},
			want: 0,
		},
		{
			name: "location byte 5",
			raw:  []byte{0xF0, 0x01, 0x28, 0x63, 0x05},
			want: 5,
		},
		{
			name: "location byte 31",
			raw:  []byte{0xF0, 0x01, 0x28, 0x63, 0x1F},
			want: 31,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sysex.Location(tt.raw)
			if got != tt.want {
				t.Errorf("Location() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestBankSlot verifies the bank and slot derivation from a location byte.
func TestBankSlot(t *testing.T) {
	tests := []struct {
		name     string
		loc      uint8
		wantBank string
		wantSlot int
	}{
		{name: "first bank A slot", loc: 0, wantBank: "A", wantSlot: 1},
		{name: "last bank A slot", loc: 15, wantBank: "A", wantSlot: 16},
		{name: "first bank B slot", loc: 16, wantBank: "B", wantSlot: 1},
		{name: "last bank B slot", loc: 31, wantBank: "B", wantSlot: 16},
		{name: "mid bank A", loc: 7, wantBank: "A", wantSlot: 8},
		{name: "mid bank B", loc: 24, wantBank: "B", wantSlot: 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bank, slot := sysex.BankSlot(tt.loc)
			if bank != tt.wantBank || slot != tt.wantSlot {
				t.Errorf("BankSlot(%d) = (%q, %d), want (%q, %d)",
					tt.loc, bank, slot, tt.wantBank, tt.wantSlot)
			}
		})
	}
}

// TestPayload verifies that Payload returns the correct slice, using the
// type-dependent header length (4 bytes for most types, 5 for FLASH/0x5C).
func TestPayload(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want []byte
	}{
		{
			name: "too short returns nil",
			raw:  []byte{0xF0, 0x01, 0x28, 0x63, 0x00},
			want: nil,
		},
		{
			// FLASH (0x63): 5-byte header, so payload starts at index 5.
			name: "FLASH minimal message",
			raw:  []byte{0xF0, 0x01, 0x28, 0x63, 0x00, 0xAA, 0xF7},
			want: []byte{0xAA},
		},
		{
			// RAM (0x60): 4-byte header, so payload starts at index 4.
			name: "RAM minimal message",
			raw:  []byte{0xF0, 0x01, 0x28, 0x60, 0xAA, 0xF7},
			want: []byte{0xAA},
		},
		{
			name: "FLASH three payload bytes",
			raw:  []byte{0xF0, 0x01, 0x28, 0x63, 0x00, 0x01, 0x02, 0x03, 0xF7},
			want: []byte{0x01, 0x02, 0x03},
		},
		{
			// Beat file export (0x62): also a 5-byte header, confirmed from
			// TempestEdit's own source (docs/sysex-tempest-format.md §9.11).
			name: "beat file export minimal message",
			raw:  []byte{0xF0, 0x01, 0x28, 0x62, 0x00, 0xAA, 0xF7},
			want: []byte{0xAA},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sysex.Payload(tt.raw)
			if tt.want == nil && got != nil {
				t.Errorf("Payload() = %v, want nil", got)
				return
			}
			if tt.want != nil {
				if len(got) != len(tt.want) {
					t.Errorf("Payload() = %v, want %v", got, tt.want)
					return
				}
				for i := range tt.want {
					if got[i] != tt.want[i] {
						t.Errorf("Payload()[%d] = 0x%02X, want 0x%02X", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

// TestUnescape verifies Unescape reads the type-dependent header length and
// applies the (now-uniform) collector-first decoding scheme.
func TestUnescape(t *testing.T) {
	// FLASH message: 5-byte header, payload = Escape7Plus1([0x41..0x47]).
	flashMsg := append([]byte{0xF0, 0x01, 0x28, 0x63, 0x00}, sysex.Escape7Plus1([]byte{0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47})...)
	flashMsg = append(flashMsg, 0xF7)

	// Alternate message: 5-byte header, payload = EscapeStandard([0x01..0x07]).
	altMsg := append([]byte{0xF0, 0x01, 0x28, 0x5C, 0x00}, sysex.EscapeStandard([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07})...)
	altMsg = append(altMsg, 0xF7)

	t.Run("FLASH decodes correctly", func(t *testing.T) {
		got := sysex.Unescape(flashMsg)
		want := []byte{0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47}
		if len(got) != len(want) {
			t.Fatalf("Unescape(FLASH) = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("byte[%d] = 0x%02X, want 0x%02X", i, got[i], want[i])
			}
		}
	})

	t.Run("Alternate decodes correctly", func(t *testing.T) {
		got := sysex.Unescape(altMsg)
		want := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
		if len(got) != len(want) {
			t.Fatalf("Unescape(Alt) = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("byte[%d] = 0x%02X, want 0x%02X", i, got[i], want[i])
			}
		}
	})
}

// TestExtractName verifies null-terminated name extraction from payloads.
func TestExtractName(t *testing.T) {
	tests := []struct {
		name        string
		unescaped   []byte
		msgType     sysex.MessageType
		wantName    string
		wantNameEnd int
	}{
		{
			name:        "RAM: too short to reach the bit-packed name field",
			unescaped:   []byte{0x01, 0x02, 0x03},
			msgType:     sysex.TypeRAMSound,
			wantName:    "",
			wantNameEnd: 0,
		},
		{
			// Bit-packed at SoundNameBitOffset (880), 7 bits/char, this
			// package's bit-0-is-LSB convention. See
			// docs/sysex-tempest-format.md §4 — confirmed against 46 real
			// RAM captures.
			name:        "RAM: bit-packed name field at offset 880",
			unescaped:   packRAMName(t, "Basic"),
			msgType:     sysex.TypeRAMSound,
			wantName:    "Basic",
			wantNameEnd: 0,
		},
		{
			name:        "FLASH: null-terminated name",
			unescaped:   []byte{'K', 'i', 'c', 'k', 0x00, 0x01, 0x02},
			msgType:     sysex.TypeFLASHSound,
			wantName:    "Kick",
			wantNameEnd: 4,
		},
		{
			name:        "FLASH: no null terminator uses full block",
			unescaped:   []byte{'A', 'B', 'C'},
			msgType:     sysex.TypeFLASHSound,
			wantName:    "ABC",
			wantNameEnd: 3,
		},
		{
			name:        "FLASH: name at start only",
			unescaped:   []byte{0x00, 0x01},
			msgType:     sysex.TypeFLASHSound,
			wantName:    "",
			wantNameEnd: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotEnd := sysex.ExtractName(tt.unescaped, tt.msgType)
			if gotName != tt.wantName {
				t.Errorf("name = %q, want %q", gotName, tt.wantName)
			}
			if gotEnd != tt.wantNameEnd {
				t.Errorf("nameEndIdx = %d, want %d", gotEnd, tt.wantNameEnd)
			}
		})
	}
}

// TestExtractParams verifies parameter block extraction from unescaped payloads.
func TestExtractParams(t *testing.T) {
	tests := []struct {
		name      string
		unescaped []byte
		msgType   sysex.MessageType
		want      []byte
	}{
		{
			name:      "RAM: entire block is params",
			unescaped: []byte{0x01, 0x02, 0x03},
			msgType:   sysex.TypeRAMSound,
			want:      []byte{0x01, 0x02, 0x03},
		},
		{
			// FLASH: "Name" + 0x00 + params
			name:      "FLASH: params after null terminator",
			unescaped: []byte{'N', 'a', 'm', 'e', 0x00, 0x0A, 0x0B},
			msgType:   sysex.TypeFLASHSound,
			want:      []byte{0x0A, 0x0B},
		},
		{
			// FLASH: name ends at last byte, nothing left for params.
			name:      "FLASH: null at end returns nil",
			unescaped: []byte{'N', 'a', 'm', 'e', 0x00},
			msgType:   sysex.TypeFLASHSound,
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sysex.ExtractParams(tt.unescaped, tt.msgType)
			if tt.want == nil {
				if got != nil {
					t.Errorf("ExtractParams() = %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("ExtractParams() = %v, want %v", got, tt.want)
				return
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("params[%d] = 0x%02X, want 0x%02X", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestBuildFLASHDump verifies the SysEx envelope built by BuildFLASHDump
// matches the confirmed real-hardware format: 5-byte header (F0 mfg dev
// 0x63 pathLen), pathLen = len(name)+1 for the terminator.
func TestBuildFLASHDump(t *testing.T) {
	params := []byte{0x24, 0x19, 0x00, 0x10, 0x49}
	raw := sysex.BuildFLASHDump("TestSound", params)

	if raw[0] != 0xF0 {
		t.Errorf("byte[0] = 0x%02X, want 0xF0", raw[0])
	}
	if raw[1] != sysex.ManufacturerID {
		t.Errorf("byte[1] = 0x%02X, want ManufacturerID", raw[1])
	}
	if raw[2] != sysex.DeviceID {
		t.Errorf("byte[2] = 0x%02X, want DeviceID", raw[2])
	}
	if raw[3] != sysex.TypeFLASH {
		t.Errorf("byte[3] = 0x%02X, want TypeFLASH", raw[3])
	}
	wantPathLen := len("TestSound") + 1 // +1 for the null terminator
	if raw[4] != byte(wantPathLen) {
		t.Errorf("pathLen = %d, want %d", raw[4], wantPathLen)
	}
	if raw[len(raw)-1] != 0xF7 {
		t.Errorf("last byte = 0x%02X, want 0xF7", raw[len(raw)-1])
	}
	if sysex.Identify(raw) != sysex.TypeFLASHSound {
		t.Error("Identify(built) != TypeFLASHSound")
	}

	// Round-trip: extract name from unescaped payload.
	unescaped := sysex.Unescape(raw)
	name, _ := sysex.ExtractName(unescaped, sysex.TypeFLASHSound)
	if name != "TestSound" {
		t.Errorf("extracted name = %q, want %q", name, "TestSound")
	}
}

// TestRenameFLASH verifies that RenameFLASH updates the name without
// changing params.
func TestRenameFLASH(t *testing.T) {
	params := []byte{0x01, 0x02, 0x03, 0x04}
	original := sysex.BuildFLASHDump("OldName", params)

	t.Run("renames FLASH sound", func(t *testing.T) {
		renamed, err := sysex.RenameFLASH(original, "NewName")
		if err != nil {
			t.Fatalf("RenameFLASH() error = %v", err)
		}
		unescaped := sysex.Unescape(renamed)
		name, _ := sysex.ExtractName(unescaped, sysex.TypeFLASHSound)
		if name != "NewName" {
			t.Errorf("new name = %q, want %q", name, "NewName")
		}
		// The wire scheme packs in fixed groups of 7; when name+params isn't
		// a multiple of 7 the final group is zero-padded, so gotParams may
		// have trailing zero bytes beyond len(params). Check the real prefix.
		gotParams := sysex.ExtractParams(unescaped, sysex.TypeFLASHSound)
		if len(gotParams) < len(params) {
			t.Fatalf("params length = %d, want at least %d", len(gotParams), len(params))
		}
		for i := range params {
			if gotParams[i] != params[i] {
				t.Errorf("params[%d] = 0x%02X, want 0x%02X", i, gotParams[i], params[i])
			}
		}
	})

	t.Run("rejects non-FLASH message", func(t *testing.T) {
		// Build a RAM message to trigger the error path.
		ram := []byte{0xF0, sysex.ManufacturerID, sysex.DeviceID, 0x60, 0x00, 0x00, 0xF7}
		_, err := sysex.RenameFLASH(ram, "NewName")
		if err == nil {
			t.Fatal("expected error for non-FLASH input, got nil")
		}
		if !strings.Contains(err.Error(), "FLASH") {
			t.Errorf("error = %q, want to contain 'FLASH'", err.Error())
		}
	})
}

// TestFingerprint verifies MD5 fingerprinting of sound messages.
func TestFingerprint(t *testing.T) {
	params := []byte{0x01, 0x02, 0x03, 0x04, 0x05}

	t.Run("FLASH sound returns 32-char hex MD5", func(t *testing.T) {
		raw := sysex.BuildFLASHDump("Sound", params)
		fp, err := sysex.Fingerprint(raw)
		if err != nil {
			t.Fatalf("Fingerprint() error = %v", err)
		}
		if len(fp) != 32 {
			t.Errorf("fingerprint length = %d, want 32", len(fp))
		}
	})

	t.Run("same params different names produce same fingerprint", func(t *testing.T) {
		fp1, err := sysex.Fingerprint(sysex.BuildFLASHDump("NameA", params))
		if err != nil {
			t.Fatalf("Fingerprint(NameA) error = %v", err)
		}
		fp2, err := sysex.Fingerprint(sysex.BuildFLASHDump("NameB", params))
		if err != nil {
			t.Fatalf("Fingerprint(NameB) error = %v", err)
		}
		if fp1 != fp2 {
			t.Errorf("fingerprints differ: %q vs %q", fp1, fp2)
		}
	})

	t.Run("different params produce different fingerprints", func(t *testing.T) {
		fp1, _ := sysex.Fingerprint(sysex.BuildFLASHDump("S", params))
		fp2, _ := sysex.Fingerprint(sysex.BuildFLASHDump("S", []byte{0xFF, 0xFE}))
		if fp1 == fp2 {
			t.Error("expected different fingerprints for different params")
		}
	})

	t.Run("valid RAM sound returns fingerprint", func(t *testing.T) {
		// F0 01 28 60 <payload bytes> F7 — headerLen(RAM)=4, so 2 payload bytes.
		ram := []byte{0xF0, sysex.ManufacturerID, sysex.DeviceID, 0x60,
			0xAA, 0xBB, 0xF7}
		fp, err := sysex.Fingerprint(ram)
		if err != nil {
			t.Fatalf("Fingerprint(RAM) error = %v", err)
		}
		if len(fp) != 32 {
			t.Errorf("fingerprint length = %d, want 32", len(fp))
		}
	})

	t.Run("RAM dump too short returns error", func(t *testing.T) {
		// 5 bytes: valid Tempest header but no payload bytes (headerLen(RAM)=4
		// leaves nothing before F7).
		ram := []byte{0xF0, sysex.ManufacturerID, sysex.DeviceID, 0x60, 0xF7}
		_, err := sysex.Fingerprint(ram)
		if err == nil {
			t.Fatal("expected error for short RAM dump, got nil")
		}
		if !strings.Contains(err.Error(), "too short") {
			t.Errorf("error = %q, want to contain 'too short'", err.Error())
		}
	})

	t.Run("unknown message returns error", func(t *testing.T) {
		_, err := sysex.Fingerprint([]byte{0x00, 0x01, 0x02})
		if err == nil {
			t.Fatal("expected error for unknown message, got nil")
		}
	})

	t.Run("AltBank not supported", func(t *testing.T) {
		altBank := []byte{0xF0, sysex.ManufacturerID, sysex.DeviceID,
			sysex.TypeAltHeader, 0x00, 0xF7}
		_, err := sysex.Fingerprint(altBank)
		if err == nil {
			t.Fatal("expected error for AltBank fingerprint, got nil")
		}
		if !strings.Contains(err.Error(), "not supported") {
			t.Errorf("error = %q, want to contain 'not supported'", err.Error())
		}
	})
}

// TestFingerprint_md5 verifies that Fingerprint uses MD5 (not SHA-256).
// It rebuilds the hash input the same way Fingerprint does — via Unescape +
// ExtractParams — to account for 7+1 encoding round-trip padding.
func TestFingerprint_md5(t *testing.T) {
	params := []byte{0x24, 0x19, 0x00, 0x10, 0x49, 0x06, 0x00, 0x04}
	raw := sysex.BuildFLASHDump("MD5Test", params)

	fp, err := sysex.Fingerprint(raw)
	if err != nil {
		t.Fatalf("Fingerprint() error = %v", err)
	}

	// MD5 produces 16 bytes → 32 hex chars. SHA-256 would give 64.
	if len(fp) != 32 {
		t.Fatalf("fingerprint length = %d, want 32 (MD5, not SHA-256)", len(fp))
	}

	// Re-derive the hash input the same way Fingerprint does, then verify
	// fp == md5(hashInput) — this confirms MD5 is the algorithm in use.
	unescaped := sysex.Unescape(raw)
	hashInput := sysex.ExtractParams(unescaped, sysex.TypeFLASHSound)
	want := fmt.Sprintf("%x", md5.Sum(hashInput))
	if fp != want {
		t.Errorf("Fingerprint() = %q, want MD5 %q", fp, want)
	}
}

// TestExtractSoundsFromProject verifies project-dump sound extraction.
func TestExtractSoundsFromProject(t *testing.T) {
	t.Run("non-project dump returns error", func(t *testing.T) {
		flash := sysex.BuildFLASHDump("S", []byte{0x01, 0x02})
		_, err := sysex.ExtractSoundsFromProject(flash, 70, "")
		if err == nil {
			t.Fatal("expected error for non-project input, got nil")
		}
		if !strings.Contains(err.Error(), "project dump") {
			t.Errorf("error = %q, want to contain 'project dump'", err.Error())
		}
	})

	t.Run("minimal project dump with no matching sounds returns empty", func(t *testing.T) {
		// Shape: F0 mfr dev 0x61 <pathLen=0> <one escaped byte> F7 — the
		// 5-byte header (pathLen at [4]) leaves a single payload byte, which
		// unescapes to nothing (no loop iterations in Unescape7Plus1).
		msg := []byte{0xF0, sysex.ManufacturerID, sysex.DeviceID, 0x61, 0x00, 0x00, 0xF7}
		results, err := sysex.ExtractSoundsFromProject(msg, 70, "")
		if err != nil {
			t.Fatalf("ExtractSoundsFromProject() error = %v", err)
		}
		if len(results) != 0 {
			t.Errorf("got %d results, want 0", len(results))
		}
	})
}

// TestProjectDumpHeaderLen verifies the 0x61 extra header byte: a project
// dump's escaped payload begins after a 5th header byte that is a
// name/path-length prefix — confirmed against a real hardware capture where
// byte[4] = 0x1c = 28 = the 27-char project path plus its null terminator
// (docs/sysex-tempest-format.md §10). Payload and Unescape must skip that
// byte, and ExtractName reads the null-terminated path from the start of the
// unescaped block.
func TestProjectDumpHeaderLen(t *testing.T) {
	path := "/P/Projects 9/Test"
	nameBytes := append([]byte(path), 0x00) // terminator, counted in pathLen like FLASH
	params := []byte{0x01, 0x02, 0x03}
	want := append(append([]byte{}, nameBytes...), params...)
	escaped := sysex.Escape7Plus1(want)

	msg := []byte{0xF0, sysex.ManufacturerID, sysex.DeviceID, 0x61, byte(len(nameBytes))}
	msg = append(msg, escaped...)
	msg = append(msg, 0xF7)

	p := sysex.Payload(msg)
	if !bytes.Equal(p, escaped) {
		t.Fatalf("Payload() = %v, want the escaped body after the 5-byte header", p)
	}

	un := sysex.Unescape(msg)
	// Unescape returns full 7-byte groups, so it may carry trailing zero
	// padding when len(want) is not a multiple of 7 — compare the real
	// content and require the remainder to be zeros.
	if len(un) < len(want) || !bytes.Equal(un[:len(want)], want) {
		t.Fatalf("Unescape() = %v, want %v (plus round-trip padding)", un, want)
	}
	for _, b := range un[len(want):] {
		if b != 0x00 {
			t.Fatalf("Unescape() padding = %v, want all zeros", un[len(want):])
		}
	}

	name, end := sysex.ExtractName(un, sysex.TypeProjectDump)
	// nameEndIdx is the index of the null terminator itself (== len(path)),
	// not len(nameBytes) — matching every other ExtractName case in this
	// file (see "FLASH: null-terminated name" above) and what ExtractParams
	// actually relies on (it reads from nameEnd+1, i.e. right after this
	// byte).
	if name != path || end != len(path) {
		t.Errorf("ExtractName() = %q, %d; want %q, %d", name, end, path, len(path))
	}
}

// TestSplitMessages verifies that SplitMessages splits a raw byte stream
// into individual F0…F7 SysEx messages.
func TestSplitMessages(t *testing.T) {
	single := []byte{0xF0, 0x01, 0x02, 0xF7}
	double := []byte{0xF0, 0x01, 0xF7, 0xF0, 0x02, 0xF7}

	tests := []struct {
		name      string
		data      []byte
		wantCount int
		// wantFirst is checked when non-nil.
		wantFirst []byte
	}{
		{
			name:      "nil input returns nil",
			data:      nil,
			wantCount: 0,
		},
		{
			name:      "no SysEx bytes returns nil",
			data:      []byte{0x01, 0x02, 0x03},
			wantCount: 0,
		},
		{
			name:      "single complete message",
			data:      single,
			wantCount: 1,
			wantFirst: single,
		},
		{
			name:      "two consecutive messages",
			data:      double,
			wantCount: 2,
			wantFirst: []byte{0xF0, 0x01, 0xF7},
		},
		{
			name:      "unclosed message is discarded",
			data:      []byte{0xF0, 0x01, 0x02},
			wantCount: 0,
		},
		{
			name:      "bytes before first F0 are ignored",
			data:      []byte{0x00, 0xFF, 0xF0, 0x01, 0xF7},
			wantCount: 1,
			wantFirst: []byte{0xF0, 0x01, 0xF7},
		},
		{
			// Second F0 before a closing F7 resets the current message.
			name:      "second F0 before F7 restarts current message",
			data:      []byte{0xF0, 0xAA, 0xF0, 0x01, 0x02, 0xF7},
			wantCount: 1,
			wantFirst: []byte{0xF0, 0x01, 0x02, 0xF7},
		},
		{
			// Valid closed message followed by an unclosed one.
			name:      "closed then unclosed message",
			data:      []byte{0xF0, 0x01, 0xF7, 0xF0, 0x02},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sysex.SplitMessages(tt.data)
			if len(got) != tt.wantCount {
				t.Errorf("SplitMessages() = %d messages, want %d",
					len(got), tt.wantCount)
				return
			}
			if tt.wantFirst != nil && len(got) > 0 {
				if !bytes.Equal(got[0], tt.wantFirst) {
					t.Errorf("first message = %v, want %v",
						got[0], tt.wantFirst)
				}
			}
		})
	}
}
