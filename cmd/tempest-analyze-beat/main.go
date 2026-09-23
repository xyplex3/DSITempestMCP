// Command tempest-analyze-beat provides diagnostic analysis of Tempest
// Beat/Kit exports to help users understand multi-note export limitations.
package main

import (
	"fmt"
	"os"
	"strings"

	"tempest-mcp/internal/sysex"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("tempest-analyze-beat - Diagnostic analysis of Tempest Beat/Kit exports")
		fmt.Println("")
		fmt.Println("Usage:")
		fmt.Println("  tempest-analyze-beat <file.syx>")
		fmt.Println("")
		fmt.Println("This tool analyzes Beat/Kit export files to help diagnose multi-note export issues.")
		fmt.Println("It reports actual note counts, expected sizes, and identifies the firmware limitation.")
		os.Exit(1)
	}

	filePath := os.Args[1]

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Printf("Error: File '%s' not found\n", filePath)
		os.Exit(1)
	}

	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Analyzing: %s\n", filePath)
	fmt.Printf("File size: %d bytes\n", len(data))
	fmt.Println("")

	// Split into messages
	messages := sysex.SplitMessages(data)
	fmt.Printf("Found %d SysEx message(s)\n", len(messages))

	// Analyze each message
	for i, msg := range messages {
		fmt.Printf("\n--- Message %d ---\n", i+1)

		msgType := sysex.Identify(msg)
		fmt.Printf("Type: %s\n", messageTypeToString(msgType))

		switch msgType {
		case sysex.TypeBeatDump:
			analyzeBeatDump(msg)
		case sysex.TypeProjectDump:
			analyzeProjectDump(msg)
		case sysex.TypeAlternateSound:
			analyzeAlternateSound(msg)
		default:
			fmt.Printf("Message type not analyzed by this tool\n")
		}
	}

	printDiagnosticSummary()
}

func analyzeBeatDump(msg []byte) {
	unescaped := sysex.Unescape(msg)

	// Extract basic info
	name, _ := sysex.ExtractName(unescaped, sysex.TypeBeatDump)
	bpm := sysex.KitBPM(unescaped)
	swing := sysex.KitSwing(unescaped)

	fmt.Printf("Beat Name: %q\n", name)
	fmt.Printf("BPM: %.1f\n", bpm)
	fmt.Printf("Swing: %.1f%%\n", swing)

	// Calculate expected note count from message size
	actualSize := len(msg)
	baseSize := 5925
	noteSize := 8
	sizeBasedNotes := (actualSize - baseSize) / noteSize

	// Try to parse actual note records from sequencer data
	records := parseNoteRecords(unescaped)

	fmt.Printf("Message Size: %d bytes\n", actualSize)
	fmt.Printf("Size-Based Estimate: %d note(s)\n", sizeBasedNotes)

	if len(records) > 0 {
		fmt.Printf("Parsed Records: %d note record(s)\n", len(records))
		fmt.Println("Record Details:")
		for i, record := range records {
			fmt.Printf("  %d. Track: %s, Step: %d, Velocity: %d\n",
				i+1, record.Track, record.Step, record.Velocity)
		}

		// Additional analysis for research purposes
		if len(records) > 1 {
			analyzeMultiNotePatterns(records)
		}
	} else {
		fmt.Println("Parsed Records: 0 note records found")
	}

	// Provide research context
	fmt.Println("\nResearch Context:")
	fmt.Printf("- Base size (no notes): %d bytes\n", baseSize)
	fmt.Printf("- Size per note: %d bytes\n", noteSize)
	fmt.Printf("- Expected size formula: %d + (%d × note_count)\n", baseSize, noteSize)
	fmt.Println("- Technical observation: Format supports multiple records when present")
	fmt.Println("- Research status: No proven techniques for reliable multi-note export yet")
}

// analyzeMultiNotePatterns provides detailed analysis of multi-note arrangements
// Useful for controlled testing and research documentation
func analyzeMultiNotePatterns(records []NoteRecord) {
	fmt.Println("\nMulti-Note Analysis:")

	// Track analysis
	tracks := make(map[string]bool)
	for _, record := range records {
		tracks[record.Track] = true
	}
	fmt.Printf("Distinct Tracks: %d (%v)\n", len(tracks), getKeys(tracks))

	// Step analysis
	steps := make(map[int][]string) // step -> tracks at that step
	for _, record := range records {
		steps[record.Step] = append(steps[record.Step], record.Track)
	}
	fmt.Printf("Distinct Steps: %d\n", len(steps))

	// Same-step vs different-step analysis
	switch {
	case len(steps) == 1:
		fmt.Println("📝 Configuration: ALL NOTES ON SAME STEP")
		fmt.Println("   - Good for controlled testing of same-step behavior")
		fmt.Println("   - May have different export characteristics")
	case len(steps) == len(records):
		fmt.Println("📝 Configuration: ALL NOTES ON DIFFERENT STEPS")
		fmt.Println("   - Matches §9.4 failure pattern (A1 step 1 + A2 step 2)")
		fmt.Println("   - Research needed: Does this consistently fail?")
	default:
		fmt.Println("📝 Configuration: MIXED STEP PLACEMENT")
		fmt.Println("   - Some notes share steps, others don't")
		fmt.Println("   - Intermediate case for research")
	}

	// Distance analysis
	minStep, maxStep := getStepRange(steps)
	if maxStep-minStep <= 1 {
		fmt.Println("📏 Step Proximity: ADJACENT or SAME steps")
		fmt.Println("   - Close step positioning may behave differently")
	} else {
		fmt.Printf("📏 Step Proximity: SPREAD (%d to %d)\n", minStep, maxStep)
		fmt.Println("   - Wide step separation may affect export behavior")
	}
}

// getKeys returns the keys from a string->bool map as a slice
func getKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// getStepRange returns min and max step values from steps map
func getStepRange(steps map[int][]string) (int, int) {
	if len(steps) == 0 {
		return 0, 0
	}

	min, max := 999, -1
	for step := range steps {
		if step < min {
			min = step
		}
		if step > max {
			max = step
		}
	}
	return min, max
}

// NoteRecord represents a parsed sequence note record
type NoteRecord struct {
	Track    string
	Step     int
	Velocity int
}

// parseNoteRecords extracts note records from the sequencer region
// Based on confirmed format from documentation and Claude's analysis
func parseNoteRecords(unescaped []byte) []NoteRecord {
	var records []NoteRecord

	// Search for record pattern in the sequencer region
	// Start searching from around byte 1077 where records are expected
	startSearch := 1077
	if startSearch >= len(unescaped) {
		return records
	}

	// Look for the marker pattern: ?? ?? ?? 77 XX XX ...
	// Where 77 is the constant marker and XX XX contains the data
	for i := startSearch; i < len(unescaped)-10; i++ {
		// Check for marker byte 0x77 at offset 3 from current position
		if unescaped[i+3] == 0x77 {
			// Check if this looks like a valid record start
			// Track byte should have high bit set (0x80)
			trackByte := unescaped[i+4]
			if trackByte&0x80 == 0x80 {
				// Extract record data
				stepByte := unescaped[i+2]
				velocityByte := unescaped[i+5]

				// Decode track identity (0x80 | track_index)
				trackIndex := int(trackByte & 0x7F)
				trackName := fmt.Sprintf("A%d", trackIndex+1) // A1, A2, etc.

				// Decode step position (step_index * 3)
				stepIndex := int(stepByte) / 3
				step := stepIndex + 1 // 1-based step numbering

				record := NoteRecord{
					Track:    trackName,
					Step:     step,
					Velocity: int(velocityByte),
				}
				records = append(records, record)

				// Skip ahead by record size to find next record
				i += 9 // 10 bytes per record, -1 because loop will increment
			}
		}
	}

	return records
}

func analyzeProjectDump(msg []byte) {
	fmt.Printf("Project Dump Size: %d bytes\n", len(msg))
	fmt.Println("ℹ️  Project dumps use different format than Beat/Kit exports")
	fmt.Println("   They may contain complete multi-note information")
	fmt.Println("   but require different parsing than 0x5F Beat messages")
	fmt.Println("   This export contains ALL beats from your project (typically 16)")
}

func analyzeAlternateSound(msg []byte) {
	unescaped := sysex.Unescape(msg)

	// Try to extract basic info if possible
	name, _ := sysex.ExtractName(unescaped, sysex.TypeAlternateSound)
	if name != "" {
		fmt.Printf("Beat Name: %q\n", name)
	}

	fmt.Printf("Alternate Sound Message Size: %d bytes\n", len(msg))
	fmt.Println("ℹ️  This is an individual beat from a Project export")
	fmt.Println("   May contain complete note information for this specific beat")
}

func messageTypeToString(t sysex.MessageType) string {
	switch t {
	case sysex.TypeUnknown:
		return "Unknown"
	case sysex.TypeRAMSound:
		return "RAM Sound (0x60)"
	case sysex.TypeProjectDump:
		return "Project Dump (0x61)"
	case sysex.TypeFLASHSound:
		return "FLASH Sound (0x63)"
	case sysex.TypeAlternateSound:
		return "Alternate Sound (0x5C) - Individual Beat"
	case sysex.TypeAlternateBank:
		return "Alternate Bank (0x5E) - Project Header"
	case sysex.TypeBeatDump:
		return "Beat/Kit Dump (0x5F)"
	default:
		return fmt.Sprintf("Type %d", int(t))
	}
}

func printDiagnosticSummary() {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("DIAGNOSTIC SUMMARY")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("This tool analyzes what's actually present in your Beat/Kit export:")
	fmt.Println("- Shows individual note records detected in the file")
	fmt.Println("- Displays track, step, and velocity information for each note")
	fmt.Println("- Calculates expected vs actual file sizes")
	fmt.Println("")
	fmt.Println("TECHNICAL FACT: The SysEx format CAN contain multiple valid note records.")
	fmt.Println("Some capture files show 2+ complete, well-formed 80-bit records.")
	fmt.Println("")
	fmt.Println("RESEARCH STATUS: No proven techniques for reliably producing")
	fmt.Println("multi-note exports yet. Current best practice is individual note export.")
	fmt.Println("")
	fmt.Println("CONTROLLED TESTING: This tool provides raw data for future research.")
	fmt.Println("See docs/tempest-multi-note-export-guide.md for current status.")
}
