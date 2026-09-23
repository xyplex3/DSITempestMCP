package mapper

import "fmt"

// DiffEntry records one differing byte position between two unescaped payloads.
type DiffEntry struct {
	Offset   int    // byte offset within the unescaped payload
	Baseline byte   // value in the baseline capture
	Changed  byte   // value in the changed capture
	Delta    int    // int(Changed) - int(Baseline), signed
	Label    string // capture label, typically the stem of the changed filename
}

// Diff compares two unescaped payloads byte by byte. label is attached to
// every entry for session log tagging. Returns an error if the slices have
// different lengths.
func Diff(baseline, changed []byte, label string) ([]DiffEntry, error) {
	if len(baseline) != len(changed) {
		return nil, fmt.Errorf("length mismatch: baseline=%d changed=%d", len(baseline), len(changed))
	}
	var entries []DiffEntry
	for i := range baseline {
		if baseline[i] != changed[i] {
			entries = append(entries, DiffEntry{
				Offset:   i,
				Baseline: baseline[i],
				Changed:  changed[i],
				Delta:    int(changed[i]) - int(baseline[i]),
				Label:    label,
			})
		}
	}
	return entries, nil
}

// InferStride returns the most common positive gap between consecutive offsets.
// Returns 0 if fewer than two offsets are provided.
func InferStride(offsets []int) int {
	if len(offsets) < 2 {
		return 0
	}
	freq := make(map[int]int)
	for i := 1; i < len(offsets); i++ {
		d := offsets[i] - offsets[i-1]
		if d > 0 {
			freq[d]++
		}
	}
	best, bestN := 0, 0
	for d, n := range freq {
		if n > bestN || (n == bestN && d < best) {
			best, bestN = d, n
		}
	}
	return best
}
