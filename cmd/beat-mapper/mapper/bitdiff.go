package mapper

import (
	"math"
	"sort"
)

// Bit indexing convention used throughout this file: bit i of a payload is
// byte i/8, bit (i%8) of that byte counting from the LSB (bit 0) — the same
// "bit 0 = LSB" convention internal/sysex/soundparams.go uses for Sound
// parameters, so a bit offset found here can be cross-referenced against that
// scheme without a mental conversion.

// DefaultMaxShiftBits is used by BitDiff when maxShift <= 0 is passed. It
// covers a handful of Tempest 7+1 collector groups (7 unpacked bytes = 56
// bits each) in either direction, enough to find a single inserted or
// deleted sequencer note record without an unreasonably wide search.
const DefaultMaxShiftBits = 512

// bitLen returns the number of bits in data.
func bitLen(data []byte) int { return len(data) * 8 }

// bitAt returns bit i of data (0 = LSB of byte i/8), or 0 if i is out of range.
func bitAt(data []byte, i int) byte {
	if i < 0 {
		return 0
	}
	byteIdx := i / 8
	if byteIdx >= len(data) {
		return 0
	}
	return (data[byteIdx] >> uint(i%8)) & 1
}

// extractBits returns bits [start, start+n) of data as a slice of 0/1 values,
// one byte per bit, using the same convention as bitAt.
func extractBits(data []byte, start, n int) []byte {
	out := make([]byte, n)
	for k := range n {
		out[k] = bitAt(data, start+k)
	}
	return out
}

// PackBits reassembles a slice of 0/1 bit values (as produced by
// BitDiffResult.InsertedBits/DeletedBits) back into bytes, LSB-first, for
// compact hex display. The final byte is zero-padded on its high bits if
// len(bits) is not a multiple of 8.
func PackBits(bits []byte) []byte {
	out := make([]byte, (len(bits)+7)/8)
	for i, b := range bits {
		if b != 0 {
			out[i/8] |= 1 << uint(i%8)
		}
	}
	return out
}

// commonPrefixBits returns the number of leading bits identical between a and b.
func commonPrefixBits(a, b []byte) int {
	n := min(bitLen(a), bitLen(b))
	i := 0
	for ; i < n; i++ {
		if bitAt(a, i) != bitAt(b, i) {
			break
		}
	}
	return i
}

// ShiftScore records how well changed's suffix matches baseline's suffix when
// changed is read starting Shift bits later (or earlier, if negative) than
// baseline, both starting from BitDiffResult.PrefixBits.
type ShiftScore struct {
	Shift      int // bits; positive means changed has extra content inserted relative to baseline
	Mismatches int
	Compared   int // number of bit positions actually compared at this shift
}

// BitDiffResult is the outcome of a bit-level alignment search between two
// unescaped payloads that may differ by an insertion or deletion of a run of
// bits partway through — e.g. one bit-packed sequencer note record added to
// the Beat/Kit sequencer region — rather than a same-length, byte-aligned
// change. See docs/sysex-tempest-format.md §7.5 for why byte-level diffing
// can't find this kind of change: shifting content by a non-byte-aligned
// number of bits makes every downstream byte differ even though the
// underlying content past the insertion is unchanged.
type BitDiffResult struct {
	BaselineBits   int // total bits in the baseline payload
	ChangedBits    int // total bits in the changed payload
	PrefixBits     int // bits identical from position 0; PrefixBits/8 is the byte-aligned length of that run
	BestShift      int // best-fit bit shift of `changed` relative to `baseline`
	BestMismatches int
	BestCompared   int
	// InsertedBits holds the extra bits present in `changed` but not
	// `baseline`, as 0/1 values, when BestShift > 0.
	InsertedBits []byte
	// DeletedBits holds the extra bits present in `baseline` but not
	// `changed`, as 0/1 values, when BestShift < 0.
	DeletedBits []byte
	// Scan lists every shift tried, in ascending Shift order, so a human can
	// eyeball the mismatch curve for a clean (near-zero) boundary rather than
	// trusting BestShift blindly — see docs/sysex-tempest-format.md §7.5's
	// "look for a run of changed bits with clean boundaries."
	Scan []ShiftScore
}

// Clean reports whether the best-fit shift realigned the suffix exactly (zero
// mismatches) over a non-trivial span, the strong signal described in
// docs/sysex-tempest-format.md §7.5. A noisy or absent match (BestCompared
// too small, or nonzero BestMismatches) means the region likely isn't a
// simple insertion/deletion and needs a different approach. The one
// exception is a trivial full match, where the common prefix already
// consumes one payload entirely (nothing left that needed a shift search).
func (r BitDiffResult) Clean() bool {
	if r.PrefixBits >= r.BaselineBits && r.PrefixBits >= r.ChangedBits {
		return true
	}
	return r.BestMismatches == 0 && r.BestCompared >= 64
}

// noiseFloorRate returns the median mismatch rate (mismatches/compared)
// across every shift tried, as a baseline for what the mismatch rate looks
// like at a wrong shift. Returns -1 if Scan is empty.
func (r BitDiffResult) noiseFloorRate() float64 {
	if len(r.Scan) == 0 {
		return -1
	}
	rates := make([]float64, len(r.Scan))
	for i, s := range r.Scan {
		rates[i] = float64(s.Mismatches) / float64(s.Compared)
	}
	sort.Float64s(rates)
	return rates[len(rates)/2]
}

// Sharpness returns how many times lower the best shift's mismatch rate is
// than the typical (median) mismatch rate across every shift tried — e.g. 30
// means the best shift aligns 30x more cleanly than a wrong shift typically
// does. It returns +Inf for an exact (zero-mismatch) match, and 0 if there's
// nothing to compare against.
func (r BitDiffResult) Sharpness() float64 {
	if r.BestCompared == 0 {
		return 0
	}
	floor := r.noiseFloorRate()
	if floor <= 0 {
		return 0
	}
	bestRate := float64(r.BestMismatches) / float64(r.BestCompared)
	if bestRate == 0 {
		return math.Inf(1)
	}
	return floor / bestRate
}

// LikelyBoundary reports whether the best-fit shift is strong evidence of a
// genuine insertion/deletion even if not perfectly Clean(). Real hardware
// captures rarely produce an exact zero-mismatch alignment — a field
// elsewhere in the payload can legitimately differ too, independent of the
// insertion itself (e.g. a note-count byte) — but a shift whose mismatch
// rate is sharply lower than neighbouring shifts' is still meaningful signal
// rather than noise. sharpnessThreshold of 5 (5x better than a typical wrong
// shift) is a reasonable default if unsure.
func (r BitDiffResult) LikelyBoundary(sharpnessThreshold float64) bool {
	if r.Clean() {
		return true
	}
	return r.Sharpness() >= sharpnessThreshold
}

// BitDiff searches for the bit shift, within ±maxShift bits (maxShift <= 0
// uses DefaultMaxShiftBits), of `changed` relative to `baseline` that best
// realigns their content after the common bit-aligned prefix. A shift with
// zero mismatches over a large compared span is strong evidence of a clean
// insertion/deletion boundary — e.g. the exact bit width of one sequencer
// note record — the way the Sound (0x60) parameter bit map in §6 was
// originally built by isolating single changed bits.
func BitDiff(baseline, changed []byte, maxShift int) BitDiffResult {
	if maxShift <= 0 {
		maxShift = DefaultMaxShiftBits
	}
	baseBits := bitLen(baseline)
	chgBits := bitLen(changed)
	prefix := commonPrefixBits(baseline, changed)
	result := BitDiffResult{BaselineBits: baseBits, ChangedBits: chgBits, PrefixBits: prefix}

	if prefix >= baseBits && prefix >= chgBits {
		return result // payloads identical: nothing left to search for
	}
	if prefix >= baseBits {
		// baseline is fully consumed by the prefix: whatever remains of
		// changed is trailing content appended past baseline's end.
		result.BestShift = chgBits - prefix
		result.InsertedBits = extractBits(changed, prefix, result.BestShift)
		return result
	}
	if prefix >= chgBits {
		// changed is fully consumed by the prefix: whatever remains of
		// baseline is trailing content missing from changed's end.
		result.BestShift = -(baseBits - prefix)
		result.DeletedBits = extractBits(baseline, prefix, baseBits-prefix)
		return result
	}

	baseRemain := baseBits - prefix
	minEligibleCompared := (baseRemain + 1) / 2 // require comparing at least half the remaining baseline suffix

	var scan []ShiftScore
	bestIdx := -1
	for shift := -maxShift; shift <= maxShift; shift++ {
		mismatches, compared := countMismatches(baseline, changed, prefix, shift)
		if compared == 0 {
			continue
		}
		score := ShiftScore{Shift: shift, Mismatches: mismatches, Compared: compared}
		scan = append(scan, score)
		if compared < minEligibleCompared {
			continue // too little overlap left at this shift to trust the score
		}
		if bestIdx == -1 || betterShift(scan[bestIdx], score) {
			bestIdx = len(scan) - 1
		}
	}
	result.Scan = scan

	if bestIdx >= 0 {
		best := scan[bestIdx]
		result.BestShift = best.Shift
		result.BestMismatches = best.Mismatches
		result.BestCompared = best.Compared
		switch {
		case best.Shift > 0:
			result.InsertedBits = extractBits(changed, prefix, best.Shift)
		case best.Shift < 0:
			result.DeletedBits = extractBits(baseline, prefix, -best.Shift)
		}
	}
	return result
}

// betterShift reports whether candidate is a better alignment than current:
// lower mismatch rate first, then more bits compared, then smaller |shift|.
func betterShift(current, candidate ShiftScore) bool {
	cRate := float64(current.Mismatches) / float64(current.Compared)
	nRate := float64(candidate.Mismatches) / float64(candidate.Compared)
	if nRate != cRate {
		return nRate < cRate
	}
	if candidate.Compared != current.Compared {
		return candidate.Compared > current.Compared
	}
	return abs(candidate.Shift) < abs(current.Shift)
}

// countMismatches compares baseline against changed starting from prefix,
// advancing whichever side is "ahead" by shift bits: for shift >= 0, changed
// reads from prefix+shift while baseline reads from prefix (models `changed`
// having shift extra bits inserted right after the prefix); for shift < 0,
// baseline instead reads from prefix+(-shift) while changed reads from
// prefix (models `changed` missing -shift bits that baseline has there).
// Comparing both sides from the same, unshifted `prefix` regardless of sign
// would silently re-read already-matched prefix bits for shift < 0 instead
// of skipping baseline's extra content, so the two sides must be advanced
// independently rather than by a single shared offset.
func countMismatches(baseline, changed []byte, prefix, shift int) (mismatches, compared int) {
	baseAdvance, chgAdvance := 0, 0
	if shift >= 0 {
		chgAdvance = shift
	} else {
		baseAdvance = -shift
	}
	baseStart := prefix + baseAdvance
	chgStart := prefix + chgAdvance

	n := min(bitLen(baseline)-baseStart, bitLen(changed)-chgStart)
	if n <= 0 {
		return 0, 0
	}
	for k := range n {
		if bitAt(baseline, baseStart+k) != bitAt(changed, chgStart+k) {
			mismatches++
		}
	}
	return mismatches, n
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
