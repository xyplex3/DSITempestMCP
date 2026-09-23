// Command beat-mapper is a standalone CLI for reverse-engineering the DSI
// Tempest project dump byte layout via capture-diff-annotate sessions.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"tempest-mcp/cmd/beat-mapper/mapper"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "beat-mapper",
	Short: "Capture-diff-annotate tool for DSI Tempest SysEx reverse engineering",
}

// ── unescape ──────────────────────────────────────────────────────────────────

var unescapeOut string

func init() {
	cmd := &cobra.Command{
		Use:   "unescape <file.syx>",
		Short: "Unescape a project dump to raw binary",
		Args:  cobra.ExactArgs(1),
		RunE:  runUnescape,
	}
	cmd.Flags().StringVar(&unescapeOut, "out", "", "output file (default: <input>.raw)")
	rootCmd.AddCommand(cmd)
}

func runUnescape(_ *cobra.Command, args []string) error {
	path := args[0]
	r, err := mapper.UnescapeProject(path)
	if err != nil {
		return err
	}
	out := unescapeOut
	if out == "" {
		ext := filepath.Ext(path)
		out = strings.TrimSuffix(path, ext) + ".raw"
	}
	if err := os.WriteFile(out, r.Payload, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", out, err)
	}
	fmt.Printf("Unescaped %d bytes (from %d encoded) → %s\n", len(r.Payload), r.RawSize, out)
	return nil
}

// ── diff ──────────────────────────────────────────────────────────────────────

var diffLabel string

func init() {
	cmd := &cobra.Command{
		Use:   "diff <baseline.syx> <changed.syx>",
		Short: "Compare two project dumps byte-by-byte after unescaping",
		Args:  cobra.ExactArgs(2),
		RunE:  runDiff,
	}
	cmd.Flags().StringVar(&diffLabel, "label", "", "tag this diff in the output")
	rootCmd.AddCommand(cmd)
}

func runDiff(_ *cobra.Command, args []string) error {
	base, err := mapper.UnescapeProject(args[0])
	if err != nil {
		return fmt.Errorf("baseline: %w", err)
	}
	chg, err := mapper.UnescapeProject(args[1])
	if err != nil {
		return fmt.Errorf("changed: %w", err)
	}

	label := diffLabel
	if label == "" {
		label = filepath.Base(args[1])
	}

	entries, err := mapper.Diff(base.Payload, chg.Payload, label)
	if err != nil {
		return err
	}

	fmt.Printf("Diff: %s vs %s\n", filepath.Base(args[0]), filepath.Base(args[1]))
	for _, e := range entries {
		sign := "+"
		if e.Delta < 0 {
			sign = ""
		}
		labelStr := ""
		if e.Label != "" && e.Label != filepath.Base(args[1]) {
			labelStr = fmt.Sprintf("  (label: %s)", e.Label)
		}
		fmt.Printf("  offset 0x%04X  baseline=0x%02X  changed=0x%02X  delta=%s%d%s\n",
			e.Offset, e.Baseline, e.Changed, sign, e.Delta, labelStr)
	}
	fmt.Printf("%d byte(s) differ out of %d\n", len(entries), len(base.Payload))
	return nil
}

// ── bitdiff ───────────────────────────────────────────────────────────────────

var (
	bitDiffMaxShift int
	bitDiffShowScan bool
)

func init() {
	cmd := &cobra.Command{
		Use:   "bitdiff <baseline.syx> <changed.syx>",
		Short: "Compare two dumps bit-by-bit, searching for a non-byte-aligned insertion or deletion",
		Long: "Compare two dumps bit-by-bit rather than byte-by-byte. Unlike diff, this " +
			"can find a change that isn't a whole number of bytes wide or byte-aligned " +
			"(e.g. one bit-packed sequencer note record added partway through the " +
			"payload), which makes every downstream byte differ even though nothing " +
			"past the insertion actually changed. See docs/sysex-tempest-format.md §7.5.",
		Args: cobra.ExactArgs(2),
		RunE: runBitDiff,
	}
	cmd.Flags().IntVar(&bitDiffMaxShift, "max-shift", 0,
		fmt.Sprintf("search window in bits, each direction (default %d)", mapper.DefaultMaxShiftBits))
	cmd.Flags().BoolVar(&bitDiffShowScan, "show-scan", false, "print the mismatch count for every shift tried")
	rootCmd.AddCommand(cmd)
}

func runBitDiff(_ *cobra.Command, args []string) error {
	base, err := mapper.UnescapeProject(args[0])
	if err != nil {
		return fmt.Errorf("baseline: %w", err)
	}
	chg, err := mapper.UnescapeProject(args[1])
	if err != nil {
		return fmt.Errorf("changed: %w", err)
	}

	r := mapper.BitDiff(base.Payload, chg.Payload, bitDiffMaxShift)
	baseName, chgName := filepath.Base(args[0]), filepath.Base(args[1])

	printBitDiffSummary(r, baseName, chgName)
	printBitDiffVerdict(r)
	printBitDiffContent(r, baseName, chgName)
	if bitDiffShowScan {
		printBitDiffScan(r)
	}
	return nil
}

func printBitDiffSummary(r mapper.BitDiffResult, baseName, chgName string) {
	fmt.Printf("Bit diff: %s vs %s\n", baseName, chgName)
	fmt.Printf("  baseline: %d bits (%d bytes)\n", r.BaselineBits, r.BaselineBits/8)
	fmt.Printf("  changed:  %d bits (%d bytes)\n", r.ChangedBits, r.ChangedBits/8)
	fmt.Printf("  common prefix: %d bits (%d bytes + %d bits)\n", r.PrefixBits, r.PrefixBits/8, r.PrefixBits%8)

	switch {
	case r.PrefixBits >= r.BaselineBits && r.PrefixBits >= r.ChangedBits:
		fmt.Println("  payloads are bit-identical")
	case r.BestShift > 0:
		fmt.Printf("  best shift: +%d bits (%.2f bytes) - changed has extra content at bit offset %d\n",
			r.BestShift, float64(r.BestShift)/8, r.PrefixBits)
	case r.BestShift < 0:
		fmt.Printf("  best shift: %d bits (%.2f bytes) - baseline has extra content, missing from changed, at bit offset %d\n",
			r.BestShift, float64(r.BestShift)/8, r.PrefixBits)
	default:
		fmt.Println("  no aligning shift found in the search window - try a larger --max-shift")
	}
	if r.BestCompared > 0 {
		fmt.Printf("  %d/%d bits mismatch at that shift\n", r.BestMismatches, r.BestCompared)
	}
}

// printBitDiffVerdict prints how much to trust BestShift as a real
// insertion/deletion boundary versus noise - see BitDiffResult.Clean and
// BitDiffResult.LikelyBoundary.
func printBitDiffVerdict(r mapper.BitDiffResult) {
	switch {
	case r.Clean():
		fmt.Println("  CLEAN boundary: a strong signal this is a genuine insertion/deletion, not noise")
	case r.LikelyBoundary(5):
		fmt.Printf("  SHARP boundary (not perfectly clean, but %.0fx below the typical mismatch rate "+
			"at a wrong shift): likely a real insertion/deletion, with residual mismatches probably "+
			"from an unrelated field elsewhere in the payload\n", r.Sharpness())
	case r.BestCompared > 0:
		fmt.Println("  NOT clean: residual mismatches remain, with no sharp minimum - this region " +
			"likely isn't a simple insertion/deletion, or --max-shift is too small to find the real one")
	}
}

func printBitDiffContent(r mapper.BitDiffResult, baseName, chgName string) {
	if len(r.InsertedBits) > 0 {
		fmt.Printf("\nInserted content (%d bits), in %s but not %s:\n", len(r.InsertedBits), chgName, baseName)
		fmt.Print(formatBits(r.InsertedBits))
	}
	if len(r.DeletedBits) > 0 {
		fmt.Printf("\nDeleted content (%d bits), in %s but not %s:\n", len(r.DeletedBits), baseName, chgName)
		fmt.Print(formatBits(r.DeletedBits))
	}
}

func printBitDiffScan(r mapper.BitDiffResult) {
	fmt.Println("\nShift scan (shift: mismatches/compared):")
	for _, s := range r.Scan {
		marker := ""
		if s.Shift == r.BestShift {
			marker = "  <- best"
		}
		fmt.Printf("  %+5d: %d/%d%s\n", s.Shift, s.Mismatches, s.Compared, marker)
	}
}

// formatBits renders a slice of 0/1 bit values as a readable binary string
// (grouped in bytes) plus its LSB-first-packed hex reconstruction. The final
// packed byte may include zero-padding bits if the bit count isn't a
// multiple of 8 - see mapper.PackBits.
func formatBits(bits []byte) string {
	var sb strings.Builder
	sb.WriteString("  binary: ")
	for i, b := range bits {
		if i > 0 && i%8 == 0 {
			sb.WriteString(" ")
		}
		if b == 0 {
			sb.WriteString("0")
		} else {
			sb.WriteString("1")
		}
	}
	sb.WriteString("\n  packed hex: ")
	for _, b := range mapper.PackBits(bits) {
		fmt.Fprintf(&sb, "%02X ", b)
	}
	sb.WriteString("\n")
	return sb.String()
}

// ── annotate ──────────────────────────────────────────────────────────────────

var annotateMapFile string

func init() {
	cmd := &cobra.Command{
		Use:   "annotate <file.syx>",
		Short: "Print annotated hex dump of the unescaped payload",
		Args:  cobra.ExactArgs(1),
		RunE:  runAnnotate,
	}
	cmd.Flags().StringVar(&annotateMapFile, "map", "", "JSON file of {\"0xOFFSET\": \"label\"} annotations")
	rootCmd.AddCommand(cmd)
}

func runAnnotate(_ *cobra.Command, args []string) error {
	r, err := mapper.UnescapeProject(args[0])
	if err != nil {
		return err
	}
	annotations := map[string]string{}
	if annotateMapFile != "" {
		data, err := os.ReadFile(annotateMapFile)
		if err != nil {
			return fmt.Errorf("reading map: %w", err)
		}
		if err := json.Unmarshal(data, &annotations); err != nil {
			return fmt.Errorf("parsing map JSON: %w", err)
		}
	}
	fmt.Print(mapper.Annotate(r.Payload, annotations))
	return nil
}

// ── session ───────────────────────────────────────────────────────────────────

type captureKey struct{ bank, track, step int }

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "session <dir>",
		Short: "Process a directory of .syx captures and infer beat layout constants",
		Args:  cobra.ExactArgs(1),
		RunE:  runSession,
	})
}

type sessionCapture struct {
	name          string
	primaryOffset int
	allEntries    []mapper.DiffEntry
}

func runSession(_ *cobra.Command, args []string) error {
	dir := args[0]
	baselinePath := filepath.Join(dir, "baseline.syx")
	baseline, err := mapper.UnescapeProject(baselinePath)
	if err != nil {
		return fmt.Errorf("baseline: %w", err)
	}

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading dir: %w", err)
	}

	var captures []sessionCapture
	for _, de := range dirEntries {
		name := de.Name()
		if de.IsDir() || name == "baseline.syx" || !strings.HasSuffix(name, ".syx") {
			continue
		}
		r, err := mapper.UnescapeProject(filepath.Join(dir, name))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: skipping %s: %v\n", name, err)
			continue
		}
		stem := strings.TrimSuffix(name, ".syx")
		diffs, err := mapper.Diff(baseline.Payload, r.Payload, stem)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: diff %s: %v\n", name, err)
			continue
		}
		if len(diffs) == 0 {
			fmt.Printf("%-30s  identical to baseline\n", name)
			continue
		}
		captures = append(captures, sessionCapture{
			name:          stem,
			primaryOffset: diffs[0].Offset,
			allEntries:    diffs,
		})
		fmt.Printf("%-30s  %d bytes differ, first at 0x%04X\n", name, len(diffs), diffs[0].Offset)
	}

	if len(captures) == 0 {
		fmt.Println("\nNo non-baseline captures found — nothing to infer.")
		return nil
	}

	sort.Slice(captures, func(i, j int) bool {
		return captures[i].primaryOffset < captures[j].primaryOffset
	})

	beatDataOffset := captures[0].primaryOffset
	byKey := buildCaptureIndex(captures)
	stepStride, trackStride := computeStrides(byKey)

	fmt.Printf("\nBeat data offset : 0x%04X\n", beatDataOffset)
	if stepStride > 0 {
		fmt.Printf("Step stride      : 0x%04X (%d bytes)\n", stepStride, stepStride)
	} else {
		fmt.Println("Step stride      : unknown — need step 1 and step 2 captures on the same track")
	}
	if trackStride > 0 {
		fmt.Printf("Track stride     : 0x%04X (%d bytes)\n", trackStride, trackStride)
	} else {
		fmt.Println("Track stride     : unknown — need track 1 and track 2 captures at the same step")
	}
	fmt.Println()
	fmt.Print(mapper.GenerateConsts(beatDataOffset, trackStride, stepStride))
	return nil
}

func buildCaptureIndex(captures []sessionCapture) map[captureKey]*sessionCapture {
	byKey := make(map[captureKey]*sessionCapture)
	for i := range captures {
		if bank, track, step, ok := parseCaptureName(captures[i].name); ok {
			byKey[captureKey{bank, track, step}] = &captures[i]
		}
	}
	return byKey
}

func computeStrides(byKey map[captureKey]*sessionCapture) (stepStride, trackStride int) {
	// Step stride: same bank+track, step N and step N+1.
	stepStride = pickStride(byKey, func(k captureKey) captureKey {
		return captureKey{k.bank, k.track, k.step + 1}
	})
	// Track stride: same bank+step, track N and track N+1.
	trackStride = pickStride(byKey, func(k captureKey) captureKey {
		return captureKey{k.bank, k.track + 1, k.step}
	})
	return
}

// pickStride collects every positive offset delta between a capture and its
// neighbor(k) counterpart, then returns the most frequent delta (ties
// broken by the smallest delta, matching InferStride in mapper/diff.go).
// Picking the *first* qualifying pair instead, via Go's randomized map
// iteration order, would make the result non-deterministic across runs of
// the identical input whenever more than one candidate pair exists.
func pickStride(byKey map[captureKey]*sessionCapture, neighbor func(captureKey) captureKey) int {
	var deltas []int
	for k, c1 := range byKey {
		if c2, ok := byKey[neighbor(k)]; ok {
			if d := c2.primaryOffset - c1.primaryOffset; d > 0 {
				deltas = append(deltas, d)
			}
		}
	}
	if len(deltas) == 0 {
		return -1
	}
	freq := make(map[int]int, len(deltas))
	for _, d := range deltas {
		freq[d]++
	}
	best, bestN := 0, 0
	for d, n := range freq {
		if n > bestN || (n == bestN && d < best) {
			best, bestN = d, n
		}
	}
	return best
}

// parseCaptureName extracts bank (0=a, 1=b), track, and step from filenames such as
// "kick_a1_s1" or "snare_b2_s3". Returns ok=false if the pattern is not found.
func parseCaptureName(name string) (bank, track, step int, ok bool) {
	lower := strings.ToLower(name)
	parts := strings.Split(lower, "_")
	bankSet, trackSet, stepSet := false, false, false
	for _, p := range parts {
		if len(p) >= 2 && (p[0] == 'a' || p[0] == 'b') {
			var n int
			if _, err := fmt.Sscanf(p[1:], "%d", &n); err == nil {
				if p[0] == 'b' {
					bank = 1
				}
				track = n
				bankSet = true
				trackSet = true
			}
		}
		if len(p) >= 2 && p[0] == 's' {
			var n int
			if _, err := fmt.Sscanf(p[1:], "%d", &n); err == nil {
				step = n
				stepSet = true
			}
		}
	}
	return bank, track, step, bankSet && trackSet && stepSet
}
