package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"tempest-mcp/internal/midi"
	"tempest-mcp/internal/sysex"
)

// Sequencer-region offsets discovered this session (unconfirmed beyond the
// single-note case — see docs/sysex-tempest-format.md §7). Only meaningful
// when exactly one note is active.
const (
	stepPosOffset  = 0x0437 // step index * 3
	velocityOffset = 0x043A // noisy, tap-driven

	baseRawLen   = 5925 // raw message length for a 0-note beat
	bytesPerNote = 8    // raw bytes added per active note
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: capture-tmp <out.syx> <expected_note_count> [timeout_sec] [baseline.syx]")
		fmt.Println("  expected_note_count: -1 to skip the check")
		os.Exit(1)
	}
	outPath := os.Args[1]
	expectedNotes, _ := strconv.Atoi(os.Args[2])
	timeout := 30
	if len(os.Args) > 3 {
		timeout, _ = strconv.Atoi(os.Args[3])
	}
	var baselinePath string
	if len(os.Args) > 4 {
		baselinePath = os.Args[4]
	}

	all, err := runCapture(timeout)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := os.WriteFile(outPath, all.data, 0o600); err != nil {
		fmt.Println("write error:", err)
		os.Exit(1)
	}
	fmt.Printf("Saved %d message(s), %d bytes total, to %s\n\n", all.n, len(all.data), outPath)

	verify(all.data, expectedNotes, baselinePath)
}

type captureResult struct {
	data []byte
	n    int
}

// runCapture connects to the Tempest, waits for SysEx, and collects every
// message that arrives until 3s of silence follows the first one.
func runCapture(timeoutSec int) (captureResult, error) {
	d := midi.New(midi.DeviceConfig{DeviceName: "Tempest", Channel: 10})
	if err := d.Connect(); err != nil {
		return captureResult{}, fmt.Errorf("connect error: %w", err)
	}
	defer d.Disconnect()

	ch, cancel := d.Subscribe()
	defer cancel()

	fmt.Printf("Waiting up to %ds for SysEx (trigger the export on the Tempest now)...\n", timeoutSec)

	var all []byte
	var n int
	deadline := time.After(time.Duration(timeoutSec) * time.Second)
	var quiet <-chan time.Time

	for {
		select {
		case raw := <-ch:
			n++
			t := sysex.Identify(raw)
			fmt.Printf("  [%d] %d bytes, type=%v\n", n, len(raw), t)
			all = append(all, raw...)
			quiet = time.After(3 * time.Second)
		case <-quiet:
			return captureResult{all, n}, nil
		case <-deadline:
			if n == 0 {
				return captureResult{}, fmt.Errorf("timeout — no SysEx received")
			}
			return captureResult{all, n}, nil
		}
	}
}

func verify(raw []byte, expectedNotes int, baselinePath string) {
	t := sysex.Identify(raw)
	if t != sysex.TypeBeatDump {
		fmt.Printf("⚠ not a Beat/Kit dump (type=%v) — skipping verification\n", t)
		return
	}
	unescaped := sysex.Unescape(raw)
	name, _ := sysex.ExtractName(unescaped, t)
	bpm := sysex.KitBPM(unescaped)

	fmt.Println("=== Decode ===")
	fmt.Printf("name=%q bpm=%.1f raw_len=%d unpacked_len=%d\n", name, bpm, len(raw), len(unescaped))

	notes := printNoteCount(len(raw), expectedNotes)
	printStepInfo(unescaped, notes)

	fmt.Println()
	fmt.Println("=== Pad table (first 4 entries) ===")
	for pad := range 4 {
		off := sysex.KitPadTableOffset + pad*sysex.KitPadEntryLen
		entry := unescaped[off : off+sysex.KitPadEntryLen]
		fmt.Printf("  A%d: % 02x\n", pad+1, entry)
	}

	if baselinePath != "" {
		printPadTableDiff(unescaped, baselinePath)
	}
}

// printNoteCount computes and prints the active note count from the raw
// message length, flagging a mismatch against expectedNotes (-1 to skip).
func printNoteCount(rawLen, expectedNotes int) int {
	delta := rawLen - baseRawLen
	notes := -1
	ok := delta >= 0 && delta%bytesPerNote == 0
	if ok {
		notes = delta / bytesPerNote
	}
	status := "✓"
	switch {
	case !ok:
		status = "✗ UNEXPECTED SIZE — not baseRawLen + 8*N"
	case expectedNotes >= 0 && notes != expectedNotes:
		status = fmt.Sprintf("✗ MISMATCH — expected %d note(s)", expectedNotes)
	}
	fmt.Printf("note count: %d %s\n", notes, status)
	return notes
}

// printStepInfo decodes and prints the step-position/velocity bytes, only
// meaningful when exactly one note is active (see package doc comment).
func printStepInfo(unescaped []byte, notes int) {
	switch {
	case notes == 1:
		pos := unescaped[stepPosOffset]
		vel := unescaped[velocityOffset]
		step := pos/3%16 + 1
		bar := pos/3/16 + 1
		fmt.Printf("step position byte (0x%04X): %d  -> bar %d, step %d (if pos/3=%d)\n", stepPosOffset, pos, bar, step, pos/3)
		fmt.Printf("velocity byte (0x%04X): %d (noisy/tap-driven, informational only)\n", velocityOffset, vel)
		if bar != 1 {
			fmt.Println("⚠ bar != 1 — the step-edit screen was probably scrolled to a later bar when this note was added")
		}
	case notes != 0:
		fmt.Println("⚠ note count != 1 — step-position/velocity decode only understood for the single-note case, skipping")
	}
}

// printPadTableDiff compares every pad entry against a baseline capture,
// reporting which (if any) differ.
func printPadTableDiff(unescaped []byte, baselinePath string) {
	bdata, err := os.ReadFile(baselinePath)
	if err != nil {
		fmt.Printf("\n⚠ could not read baseline %s: %v\n", baselinePath, err)
		return
	}
	bunescaped := sysex.Unescape(bdata)
	fmt.Println()
	fmt.Printf("=== Pad table diff vs baseline (%s) ===\n", baselinePath)
	anyDiff := false
	for pad := range sysex.KitPadEntryCount {
		off := sysex.KitPadTableOffset + pad*sysex.KitPadEntryLen
		if off+sysex.KitPadEntryLen > len(unescaped) || off+sysex.KitPadEntryLen > len(bunescaped) {
			continue
		}
		a := unescaped[off : off+sysex.KitPadEntryLen]
		b := bunescaped[off : off+sysex.KitPadEntryLen]
		diffs := 0
		for i := range a {
			if a[i] != b[i] {
				diffs++
			}
		}
		if diffs > 0 {
			bank, slot := "A", pad+1
			if pad >= 16 {
				bank, slot = "B", pad-15
			}
			fmt.Printf("  %s%d: %d byte(s) differ from baseline\n", bank, slot, diffs)
			anyDiff = true
		}
	}
	if !anyDiff {
		fmt.Println("  (no differences — pad table matches baseline exactly)")
	}
}
