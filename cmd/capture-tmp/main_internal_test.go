// White-box tests for capture-tmp's unexported note-count computation.
// package main has no exported caller path, so this lives alongside the
// source rather than in a _test package. Only the returned value is
// asserted — printNoteCount also prints a status line to stdout, which is
// not captured here (this suite never swaps os.Stdout).
package main

import "testing"

// TestPrintNoteCount verifies the byte-count-derived note count formula
// (baseRawLen + bytesPerNote*N) independent of the expectedNotes argument,
// which only affects the printed status line, not the returned value.
func TestPrintNoteCount(t *testing.T) {
	tests := []struct {
		name          string
		rawLen        int
		expectedNotes int
		want          int
	}{
		{name: "baseline, zero notes", rawLen: baseRawLen, expectedNotes: -1, want: 0},
		{name: "one note", rawLen: baseRawLen + bytesPerNote, expectedNotes: 1, want: 1},
		{name: "two notes", rawLen: baseRawLen + 2*bytesPerNote, expectedNotes: -1, want: 2},
		{name: "mismatched expectation does not change the count", rawLen: baseRawLen + bytesPerNote, expectedNotes: 5, want: 1},
		{name: "smaller than baseline is unclear", rawLen: baseRawLen - 10, expectedNotes: -1, want: -1},
		{name: "not aligned to an 8-byte boundary is unclear", rawLen: baseRawLen + 3, expectedNotes: -1, want: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := printNoteCount(tt.rawLen, tt.expectedNotes)
			if got != tt.want {
				t.Errorf("printNoteCount(%d, %d) = %d, want %d",
					tt.rawLen, tt.expectedNotes, got, tt.want)
			}
		})
	}
}
