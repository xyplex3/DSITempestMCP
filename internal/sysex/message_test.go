package sysex_test

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"strings"
	"testing"

	"tempest-mcp/internal/sysex"
)

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

// TestPayload verifies that Payload returns the correct slice.
func TestPayload(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want []byte
	}{
		{
			name: "too short returns nil",
			raw:  []byte{0xF0, 0x01, 0x28, 0x63, 0x00, 0x00},
			want: nil,
		},
		{
			// Minimum valid: raw[6:len-1] = raw[6:7] = one byte.
			name: "minimal 8-byte message",
			raw:  []byte{0xF0, 0x01, 0x28, 0x63, 0x00, 0x00, 0xAA, 0xF7},
			want: []byte{0xAA},
		},
		{
			name: "three payload bytes",
			raw:  []byte{0xF0, 0x01, 0x28, 0x63, 0x00, 0x00, 0x01, 0x02, 0x03, 0xF7},
			want: []byte{0x01, 0x02, 0x03},
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

// TestUnescape verifies Unescape dispatches the correct decoding scheme.
func TestUnescape(t *testing.T) {
	// FLASH message (7+1 scheme).
	// Payload is Escape7Plus1([0x41..0x47]).
	flashMsg := []byte{
		0xF0, 0x01, 0x28, 0x63, 0x00, 0x00,
		0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x00, // 7 bytes + mystery
		0xF7,
	}

	// Alternate message (standard 7-of-8 scheme).
	// Payload is EscapeStandard([0x01..0x07]) = 7 low bytes + MSB 0x00.
	altMsg := []byte{
		0xF0, 0x01, 0x28, 0x5C, 0x00, 0x00,
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x00, // 7 bytes + MSB
		0xF7,
	}

	t.Run("FLASH uses 7+1 scheme", func(t *testing.T) {
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

	t.Run("Alternate uses standard 7-of-8 scheme", func(t *testing.T) {
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
			name:        "RAM sound has no name",
			unescaped:   []byte{0x01, 0x02, 0x03},
			msgType:     sysex.TypeRAMSound,
			wantName:    "",
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

// TestBuildFLASHDump verifies the SysEx envelope built by BuildFLASHDump.
func TestBuildFLASHDump(t *testing.T) {
	params := []byte{0x24, 0x19, 0x00, 0x10, 0x49}
	raw := sysex.BuildFLASHDump("TestSound", params, 5)

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
	if raw[4] != 5 {
		t.Errorf("location = %d, want 5", raw[4])
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
// changing params or location.
func TestRenameFLASH(t *testing.T) {
	params := []byte{0x01, 0x02, 0x03, 0x04}
	original := sysex.BuildFLASHDump("OldName", params, 7)

	t.Run("renames FLASH sound", func(t *testing.T) {
		renamed, err := sysex.RenameFLASH(original, "NewName")
		if err != nil {
			t.Fatalf("RenameFLASH() error = %v", err)
		}
		if sysex.Location(renamed) != 7 {
			t.Errorf("location = %d, want 7", sysex.Location(renamed))
		}
		unescaped := sysex.Unescape(renamed)
		name, _ := sysex.ExtractName(unescaped, sysex.TypeFLASHSound)
		if name != "NewName" {
			t.Errorf("new name = %q, want %q", name, "NewName")
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
		raw := sysex.BuildFLASHDump("Sound", params, 0)
		fp, err := sysex.Fingerprint(raw)
		if err != nil {
			t.Fatalf("Fingerprint() error = %v", err)
		}
		if len(fp) != 32 {
			t.Errorf("fingerprint length = %d, want 32", len(fp))
		}
	})

	t.Run("same params different names produce same fingerprint", func(t *testing.T) {
		fp1, err := sysex.Fingerprint(sysex.BuildFLASHDump("NameA", params, 0))
		if err != nil {
			t.Fatalf("Fingerprint(NameA) error = %v", err)
		}
		fp2, err := sysex.Fingerprint(sysex.BuildFLASHDump("NameB", params, 15))
		if err != nil {
			t.Fatalf("Fingerprint(NameB) error = %v", err)
		}
		if fp1 != fp2 {
			t.Errorf("fingerprints differ: %q vs %q", fp1, fp2)
		}
	})

	t.Run("different params produce different fingerprints", func(t *testing.T) {
		fp1, _ := sysex.Fingerprint(sysex.BuildFLASHDump("S", params, 0))
		fp2, _ := sysex.Fingerprint(sysex.BuildFLASHDump("S", []byte{0xFF, 0xFE}, 0))
		if fp1 == fp2 {
			t.Error("expected different fingerprints for different params")
		}
	})

	t.Run("valid RAM sound returns fingerprint", func(t *testing.T) {
		// Minimum RAM: F0 01 28 60 loc 0xAA 0xBB 0xF7 (len=8 >= 7)
		ram := []byte{0xF0, sysex.ManufacturerID, sysex.DeviceID, 0x60, 0x00,
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
		// 6 bytes: valid Tempest header but len < 7 for RAM case.
		ram := []byte{0xF0, sysex.ManufacturerID, sysex.DeviceID, 0x60, 0x00, 0xF7}
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
	raw := sysex.BuildFLASHDump("MD5Test", params, 0)

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
		flash := sysex.BuildFLASHDump("S", []byte{0x01, 0x02}, 0)
		_, err := sysex.ExtractSoundsFromProject(flash, 70, "")
		if err == nil {
			t.Fatal("expected error for non-project input, got nil")
		}
		if !strings.Contains(err.Error(), "project dump") {
			t.Errorf("error = %q, want to contain 'project dump'", err.Error())
		}
	})

	t.Run("minimal project dump with no matching sounds returns empty", func(t *testing.T) {
		// Construct an empty project message: payload = Escape7Plus1([]).
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
