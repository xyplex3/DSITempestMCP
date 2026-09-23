// Package pattern defines the Tempest beat/step data structures and
// step-grid notation.
package pattern

import "fmt"

// Step represents one note event in a sound track.
// All fields beyond Gate and Velocity require confirmed byte offsets from a
// beat-mapper capture session before they can be decoded or encoded.
//
// Fields documented by the Tempest manual (v1.4 OS 1.4, 96 PPQN resolution):
//   - Velocity / Duration are always recorded when a note is captured.
//   - TimeShift range is ±3 PPQN parts (~±5.2 ms at 120 BPM).
//   - NoteFX[0..3] correspond to Note FX channels 1–4 per note event.
//   - Reverse replays the sound with all envelopes inverted.
type Step struct {
	Gate      bool     // true when a note is programmed at this step
	Velocity  uint8    // 1–127; 0 when Gate is false
	LengthMS  int      // note duration in ms; 0 = default gate length
	Tuning    int8     // semitone offset, range TBD from hardware capture
	NoteFX    [4]uint8 // Note FX channels 1–4 values; offset TBD
	TimeShift int8     // ±3 PPQN parts (96 PPQN per quarter note); offset TBD
	Reverse   bool     // play sound in reverse (envelopes and samples); offset TBD
}

// Track is one sound's step sequence within a beat.
// PadName is "A1"–"A16" or "B1"–"B16".
type Track struct {
	PadName   string // hardware pad identifier, e.g. "A1" or "B16"
	StepCount int    // 1–128 (up to 8 bars × 16th notes)
	Steps     []Step // len(Steps) == StepCount after construction
}

// Beat holds a full drum pattern. Tracks always has 32 entries:
// index 0–15 = A1–A16, index 16–31 = B1–B16.
type Beat struct {
	Name   string  // beat name as stored in the project
	Slot   int     // 1–16 within the project
	BPM    float64 // tempo in beats per minute
	Tracks []Track // always 32 entries (A1–A16 then B1–B16)
}

// StepGridFromString parses compact step notation into a Steps slice.
//
// Character meanings:
//
//	'x'     — hit at default velocity 100
//	'1'–'9' — hit at velocity digit×14 (e.g. '5' → 70)
//	'.'     — rest
//	'|'     — bar separator, ignored
//
// The result is truncated to stepCount steps. Grids shorter than stepCount are
// right-padded with rests. An unrecognised character returns an error.
func StepGridFromString(grid string, stepCount int) ([]Step, error) {
	steps := make([]Step, 0, stepCount)
	for _, ch := range grid {
		if len(steps) >= stepCount {
			break
		}
		switch {
		case ch == '|':
			continue
		case ch == '.':
			steps = append(steps, Step{})
		case ch == 'x':
			steps = append(steps, Step{Gate: true, Velocity: 100})
		case ch >= '1' && ch <= '9':
			steps = append(steps, Step{Gate: true, Velocity: uint8(int(ch-'0') * 14)})
		default:
			return nil, fmt.Errorf("unrecognised character %q in step grid", string(ch))
		}
	}
	for len(steps) < stepCount {
		steps = append(steps, Step{})
	}
	return steps, nil
}
