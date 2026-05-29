package sysex_test

import (
	"reflect"
	"testing"

	"tempest-mcp/internal/sysex"
)

// TestUnescape7Plus1 verifies the Tempest 7+1 decoding scheme.
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
			name:    "one full group: 7 data + 1 mystery byte",
			encoded: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0xFF},
			want:    []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
		},
		{
			name: "two full groups",
			encoded: []byte{
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0xFF,
				0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0xFE,
			},
			want: []byte{
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
				0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
			},
		},
		{
			// Fewer than 8 bytes: inner loop reads what is present.
			name:    "partial group fewer than 8 bytes",
			encoded: []byte{0x0A, 0x0B, 0x0C},
			want:    []byte{0x0A, 0x0B, 0x0C},
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

// TestEscape7Plus1 verifies the Tempest 7+1 encoding scheme.
func TestEscape7Plus1(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []byte
	}{
		{
			name: "empty input",
			data: []byte{},
			// groups = 0, result is empty
			want: []byte{},
		},
		{
			name: "exactly 7 bytes: one group + mystery zero",
			data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			want: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x00},
		},
		{
			// 8 bytes: 1 full group + 1 partial (1 byte + 6 zero pads + mystery).
			name: "8 bytes: two groups second padded",
			data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			want: []byte{
				0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x00,
				0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
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
		{name: "7 bytes", data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}},
		{name: "14 bytes", data: []byte{
			0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70,
			0x11, 0x21, 0x31, 0x41, 0x51, 0x61, 0x71,
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

// TestUnescapeStandard verifies the standard DSI 7-of-8 MSB decoding.
func TestUnescapeStandard(t *testing.T) {
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
			// 8-byte group: 7 data bytes all low, MSB byte = 0.
			name:    "one group all low bits",
			encoded: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x00},
			want:    []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
		},
		{
			// MSB byte 0x7F means all 7 data bytes had their high bit set.
			name:    "one group all high bits restored",
			encoded: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x7F},
			want:    []byte{0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87},
		},
		{
			// Fewer than 8 bytes: appended as-is (remainder path).
			name:    "fewer than 8 bytes appended as remainder",
			encoded: []byte{0x05, 0x06},
			want:    []byte{0x05, 0x06},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sysex.UnescapeStandard(tt.encoded)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnescapeStandard() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestEscapeStandard verifies the standard DSI 7-of-8 MSB encoding.
func TestEscapeStandard(t *testing.T) {
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
			name: "7 bytes all low bits: MSB byte is zero",
			data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			want: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x00},
		},
		{
			// All 7 data bytes have high bit set; MSB byte collects them as
			// 0x7F (bits 6–0 set).
			name: "7 bytes all high bits: MSB byte is 0x7F",
			data: []byte{0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87},
			want: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x7F},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sysex.EscapeStandard(tt.data)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EscapeStandard() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestEscapeStandardRoundTrip verifies UnescapeStandard(EscapeStandard(data)) == data
// for inputs that are multiples of 7 bytes.
func TestEscapeStandardRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "7 bytes all low",
			data: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
		},
		{
			name: "7 bytes all high",
			data: []byte{0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87},
		},
		{
			name: "7 bytes mixed",
			data: []byte{0x01, 0x82, 0x03, 0x84, 0x05, 0x86, 0x07},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sysex.UnescapeStandard(sysex.EscapeStandard(tt.data))
			if !reflect.DeepEqual(got, tt.data) {
				t.Errorf("round-trip = %v, want %v", got, tt.data)
			}
		})
	}
}
