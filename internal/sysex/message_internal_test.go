// Package sysex internal tests cover unexported helpers that cannot be reached
// through the exported API.
package sysex

import "testing"

// TestSoundQuality verifies the soundQuality scorer against known inputs.
// The function awards points for signature similarity, byte variety, non-zero
// count, and valid MIDI range (0–127). Exact scores depend on param content.
func TestSoundQuality(t *testing.T) {
	// Build a block that begins with the reference signature.
	sigBlock := make([]byte, ParamBlockSizeFLASH)
	copy(sigBlock, referenceSignature)

	tests := []struct {
		name    string
		params  []byte
		wantMin int
		wantMax int
	}{
		{
			name:    "nil params returns 0",
			params:  nil,
			wantMin: 0,
			wantMax: 0,
		},
		{
			name:    "empty params returns 0",
			params:  []byte{},
			wantMin: 0,
			wantMax: 0,
		},
		{
			// All zeros: only indices where sig[i]==0x00 match (4 of 16).
			// Variety = 1 unique byte (0x00) → 0 variety pts.
			// NonZero = 0 → 0 pts. Valid range = 132/132 → 15 pts.
			// Sig score = 4*40/16 = 10. Total ≈ 25.
			name:    "all zeros scores low",
			params:  make([]byte, ParamBlockSizeFLASH),
			wantMin: 20,
			wantMax: 30,
		},
		{
			// Perfect signature match + reasonable variety + all valid.
			// Sig = 40 pts, variety ≈ 6 pts, nonZero ≈ 1 pt,
			// valid range = 15 pts → ≈ 62.
			name:    "reference signature scores moderately high",
			params:  sigBlock,
			wantMin: 55,
			wantMax: 70,
		},
		{
			// Single-byte param — boundary condition.
			name:    "single byte param",
			params:  []byte{referenceSignature[0]},
			wantMin: 0,
			wantMax: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := soundQuality(tt.params)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("soundQuality() = %d, want [%d, %d]",
					got, tt.wantMin, tt.wantMax)
			}
		})
	}
}
