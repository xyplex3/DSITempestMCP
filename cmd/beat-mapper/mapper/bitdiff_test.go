package mapper

import (
	"math/rand"
	"testing"
)

// randomBits returns n pseudo-random 0/1 values from a fixed seed, so tests
// are deterministic but the content isn't accidentally self-similar (which
// would let a wrong shift "accidentally" match too).
func randomBits(seed int64, n int) []byte {
	r := rand.New(rand.NewSource(seed))
	bits := make([]byte, n)
	for i := range bits {
		bits[i] = byte(r.Intn(2))
	}
	return bits
}

// insertBits returns a copy of bits with insert spliced in at position pos.
func insertBits(bits []byte, pos int, insert []byte) []byte {
	out := make([]byte, 0, len(bits)+len(insert))
	out = append(out, bits[:pos]...)
	out = append(out, insert...)
	out = append(out, bits[pos:]...)
	return out
}

func TestBitDiff_identicalPayloads(t *testing.T) {
	payload := PackBits(randomBits(1, 4000))
	result := BitDiff(payload, payload, 0)

	if result.PrefixBits != bitLen(payload) {
		t.Errorf("PrefixBits = %d, want %d (full length)", result.PrefixBits, bitLen(payload))
	}
	if result.BestShift != 0 {
		t.Errorf("BestShift = %d, want 0", result.BestShift)
	}
	if result.BestMismatches != 0 {
		t.Errorf("BestMismatches = %d, want 0", result.BestMismatches)
	}
	if !result.Clean() {
		t.Error("Clean() = false for identical payloads, want true")
	}
}

// TestBitDiff_findsNonByteAlignedInsertion is the core scenario this tool
// exists for (docs/sysex-tempest-format.md §7.5): a run of bits inserted at a
// position that isn't a whole number of bytes, simulating one bit-packed
// sequencer note record being added. A byte-level diff would show near-total
// divergence after the insertion point; BitDiff must recover the exact
// insertion bit-offset, its exact bit width, and its exact content.
func TestBitDiff_findsNonByteAlignedInsertion(t *testing.T) {
	base := randomBits(2, 10000)
	const insertPos = 8096 // KitSequencerOffset * 8, the known fixed-region boundary
	const insertLen = 37   // deliberately not byte-aligned
	insert := randomBits(99, insertLen)

	changedBits := insertBits(base, insertPos, insert)
	baseline := PackBits(base)
	changed := PackBits(changedBits)

	result := BitDiff(baseline, changed, 128)

	if result.PrefixBits < insertPos {
		t.Fatalf("PrefixBits = %d, want >= %d", result.PrefixBits, insertPos)
	}
	if result.BestShift != insertLen {
		t.Fatalf("BestShift = %d, want %d", result.BestShift, insertLen)
	}
	if result.BestMismatches != 0 {
		t.Errorf("BestMismatches = %d, want 0 (clean boundary)", result.BestMismatches)
	}
	if !result.Clean() {
		t.Error("Clean() = false, want true for an exact insertion")
	}

	// The inserted content BitDiff recovers must exactly match what was
	// spliced in, read starting from wherever the true common prefix ended
	// (which may be later than insertPos if base happens to agree with
	// insert's leading bits by chance — vanishingly unlikely with random
	// data, but the test should tolerate it rather than assume exactness).
	gotInserted := result.InsertedBits
	if len(gotInserted) != insertLen {
		t.Fatalf("len(InsertedBits) = %d, want %d", len(gotInserted), insertLen)
	}
	wantInserted := changedBits[result.PrefixBits : result.PrefixBits+insertLen]
	for i := range wantInserted {
		if gotInserted[i] != wantInserted[i] {
			t.Errorf("InsertedBits[%d] = %d, want %d", i, gotInserted[i], wantInserted[i])
		}
	}
}

// TestBitDiff_findsDeletion mirrors the insertion case for a shrink (fewer
// bits in `changed` than `baseline`), reported as DeletedBits with a negative
// BestShift.
func TestBitDiff_findsDeletion(t *testing.T) {
	base := randomBits(3, 5000)
	const deletePos = 1600
	const deleteLen = 19

	changedBits := append(append([]byte{}, base[:deletePos]...), base[deletePos+deleteLen:]...)
	baseline := PackBits(base)
	changed := PackBits(changedBits)

	result := BitDiff(baseline, changed, 128)

	if result.BestShift != -deleteLen {
		t.Fatalf("BestShift = %d, want %d", result.BestShift, -deleteLen)
	}
	if result.BestMismatches != 0 {
		t.Errorf("BestMismatches = %d, want 0 (clean boundary)", result.BestMismatches)
	}
	gotDeleted := result.DeletedBits
	if len(gotDeleted) != deleteLen {
		t.Fatalf("len(DeletedBits) = %d, want %d", len(gotDeleted), deleteLen)
	}
	wantDeleted := base[result.PrefixBits : result.PrefixBits+deleteLen]
	for i := range wantDeleted {
		if gotDeleted[i] != wantDeleted[i] {
			t.Errorf("DeletedBits[%d] = %d, want %d", i, gotDeleted[i], wantDeleted[i])
		}
	}
}

// TestBitDiff_noCleanAlignment covers unrelated payloads: no shift should
// reach zero mismatches, so Clean() must report false rather than picking an
// arbitrary "best of a bad lot" shift as if it meant something.
func TestBitDiff_noCleanAlignment(t *testing.T) {
	baseline := PackBits(randomBits(4, 4000))
	changed := PackBits(randomBits(5, 4000))

	result := BitDiff(baseline, changed, 64)

	if result.Clean() {
		t.Error("Clean() = true for unrelated random payloads, want false")
	}
	if result.LikelyBoundary(5) {
		t.Errorf("LikelyBoundary(5) = true for unrelated random payloads (sharpness %.2f), want false",
			result.Sharpness())
	}
}

// TestBitDiff_sharpButNotClean models the real capture behaviour seen
// against actual Tempest hardware dumps: a clean insertion plus a few
// scattered, unrelated bit flips elsewhere in the payload (e.g. a separate
// note-count-like field also changing when a note is added). The true
// insertion shift should still win by a wide margin even though it's no
// longer a literal zero-mismatch match, so Clean() is false but
// LikelyBoundary reports it as a strong signal via Sharpness().
func TestBitDiff_sharpButNotClean(t *testing.T) {
	base := randomBits(7, 10000)
	const insertPos = 8096
	const insertLen = 80
	insert := randomBits(100, insertLen)
	changedBits := insertBits(base, insertPos, insert)

	// Flip a handful of bits well past the insertion, unrelated to it.
	r := rand.New(rand.NewSource(42))
	for range 5 {
		i := insertPos + insertLen + r.Intn(len(changedBits)-insertPos-insertLen)
		changedBits[i] ^= 1
	}

	baseline := PackBits(base)
	changed := PackBits(changedBits)
	result := BitDiff(baseline, changed, 128)

	if result.BestShift != insertLen {
		t.Fatalf("BestShift = %d, want %d", result.BestShift, insertLen)
	}
	if result.Clean() {
		t.Error("Clean() = true, want false (5 unrelated bits were flipped)")
	}
	if !result.LikelyBoundary(5) {
		t.Errorf("LikelyBoundary(5) = false, want true (sharpness %.2f)", result.Sharpness())
	}
	if result.Sharpness() < 5 {
		t.Errorf("Sharpness() = %.2f, want >= 5", result.Sharpness())
	}
}

func TestPackBits_roundTrip(t *testing.T) {
	bits := randomBits(6, 53) // not a multiple of 8, exercises the padding path
	packed := PackBits(bits)
	for i, want := range bits {
		got := bitAt(packed, i)
		if got != want {
			t.Errorf("bitAt(packed, %d) = %d, want %d", i, got, want)
		}
	}
}

func TestCommonPrefixBits(t *testing.T) {
	a := []byte{0xFF, 0x0F, 0xAA}
	b := []byte{0xFF, 0x0E, 0xAA} // byte 1 differs starting at its LSB (bit 0)
	got := commonPrefixBits(a, b)
	want := 8 // all 8 bits of byte 0 match; byte 1 bit 0 (LSB) differs (0x0F vs 0x0E)
	if got != want {
		t.Errorf("commonPrefixBits = %d, want %d", got, want)
	}
}
