package sysex_test

import (
	"reflect"
	"testing"

	"tempest-mcp/internal/sysex"
)

// TestUnescape7Plus1 verifies the collector-first decoding scheme: byte 0 of
// each group of 8 is a collector whose bit k is the high bit of data byte k.
func TestUnescape7Plus1(t *testing.T) {
	tests := []struct {
		name    string
		encoded []byte
		want    []byte
	}{
		{
			name:    "empty input",
			encoded: []byte{},
			want:    []byte{},
		},
		{
			// Collector 0x00: no high bits set, data passes through unchanged.
			name:    "one full group, collector all zero",
			encoded: []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			want:    []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
		},
		{
			// Collector bit 0 set -> data byte 0 gets its high bit restored.
			// Collector bit 6 set -> data byte 6 gets its high bit restored.
			name:    "collector restores high bits on first and last data byte",
			encoded: []byte{0x41, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			want:    []byte{0x81, 0x02, 0x03, 0x04, 0x05, 0x06, 0x87},
		},
		{
			name: "two full groups",
			encoded: []byte{
				0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
				0x00, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
			},
			want: []byte{
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
				0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
			},
		},
		{
			// Fewer than 8 bytes: first byte is still treated as the collector,
			// remaining bytes as partial data.
			name:    "partial group fewer than 8 bytes",
			encoded: []byte{0x00, 0x0B, 0x0C},
			want:    []byte{0x0B, 0x0C},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sysex.Unescape7Plus1(tt.encoded)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Unescape7Plus1() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestEscape7Plus1 verifies the collector-first encoding scheme.
func TestEscape7Plus1(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []byte
	}{
		{
			name: "empty input",
			data: []byte{},
			want: []byte{},
		},
		{
			name: "exactly 7 bytes, no high bits: collector is zero",
			data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			want: []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
		},
		{
			// High bits on byte 0 and byte 6 set collector bits 0 and 6.
			name: "high bits set collector bits",
			data: []byte{0x81, 0x02, 0x03, 0x04, 0x05, 0x06, 0x87},
			want: []byte{0x41, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
		},
		{
			// 8 bytes: one full group, then a 1-byte group zero-padded to 7.
			name: "8 bytes: two groups, second padded",
			data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			want: []byte{
				0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
				0x00, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sysex.Escape7Plus1(tt.data)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Escape7Plus1() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestEscape7Plus1RoundTrip verifies Unescape7Plus1(Escape7Plus1(data)) == data
// for inputs that are multiples of 7 bytes (no padding ambiguity).
func TestEscape7Plus1RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "7 bytes low", data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}},
		{name: "7 bytes high", data: []byte{0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87}},
		{name: "14 bytes mixed", data: []byte{
			0x90, 0x20, 0x30, 0xC0, 0x50, 0x60, 0x70,
			0x11, 0xA1, 0x31, 0x41, 0xD1, 0x61, 0x71,
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sysex.Unescape7Plus1(sysex.Escape7Plus1(tt.data))
			if !reflect.DeepEqual(got, tt.data) {
				t.Errorf("round-trip = %v, want %v", got, tt.data)
			}
		})
	}
}

// TestUnescapeStandard/TestEscapeStandard: 0x5C/0x5E use the same
// collector-first scheme as everything else (see encoding.go doc comment),
// so these exercise identical behaviour to the Unescape7Plus1/Escape7Plus1
// tests above through the distinct exported names.

func TestUnescapeStandardRoundTrip(t *testing.T) {
	// 14 bytes: a multiple of 7, so no zero-padding ambiguity on the final group.
	data := []byte{0x81, 0x02, 0x83, 0x04, 0x05, 0x86, 0x07, 0x11, 0x92, 0x13, 0x14, 0x95, 0x16, 0x17}
	got := sysex.UnescapeStandard(sysex.EscapeStandard(data))
	if !reflect.DeepEqual(got, data) {
		t.Errorf("round-trip = %v, want %v", got, data)
	}
}

func TestUnescapeStandardMatchesUnescape7Plus1(t *testing.T) {
	encoded := []byte{0x41, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	got := sysex.UnescapeStandard(encoded)
	want := sysex.Unescape7Plus1(encoded)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UnescapeStandard() = %v, want (same as Unescape7Plus1) %v", got, want)
	}
}
