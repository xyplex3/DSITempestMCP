package midi

import (
	"context"
	"fmt"
	"strings"
	"time"

	gomidi "gitlab.com/gomidi/midi/v2"
)

// padNoteMap maps friendly pad names to GM drum MIDI note numbers.
// Based on the Tempest factory defaults documented in the MCP reference.
var padNoteMap = map[string]uint8{
	// Kick / Bass drum
	"kick":      36,
	"bass-drum": 36,
	"bd":        36,
	"a12":       36,

	// Snares
	"snare":   38,
	"snare-1": 38,
	"a4":      38,
	"snare-2": 40,
	"a3":      40,

	// Hi-hats
	"closed-hat":   42,
	"closed-hihat": 42,
	"chh":          42,
	"a13":          42,
	"open-hat":     46,
	"open-hihat":   46,
	"ohh":          46,
	"a5":           46,

	// Toms
	"high-tom": 48,
	"a6":       48,
	"mid-tom":  47,
	"a7":       47,
	"low-tom":  43,
	"a8":       43,

	// Cymbals
	"crash":  49,
	"a15":    49,
	"ride":   51,
	"a14":    51,
	"splash": 55,
	"a16":    55,

	// Hand percussion
	"clap":       39,
	"a10":        39,
	"side-stick": 37,
	"rimshot":    37,
	"a11":        37,
	"cabasa":     69,
	"a1":         69,
	"tambourine": 54,
	"a2":         54,
}

// NoteForPad looks up the MIDI note number for a pad name.
// Returns an error if the name is not recognised.
func NoteForPad(padName string) (uint8, error) {
	note, ok := padNoteMap[strings.ToLower(strings.TrimSpace(padName))]
	if !ok {
		return 0, fmt.Errorf("unknown pad %q — use a name like 'kick', 'snare', 'closed-hat', or a pad ID like 'a12'", padName)
	}
	return note, nil
}

// ListPadNames returns all known pad names.
func ListPadNames() []string {
	names := make([]string, 0, len(padNoteMap))
	for k := range padNoteMap {
		names = append(names, k)
	}
	return names
}

// TriggerPad sends NoteOn → sleep → NoteOff for the named pad.
// ctx cancellation causes an immediate NoteOff and ctx.Err() is returned.
func (d *Device) TriggerPad(ctx context.Context, padName string, velocity uint8, durationMS int) error {
	if velocity == 0 {
		velocity = 100
	}
	if durationMS <= 0 {
		durationMS = 50
	}

	note, err := NoteForPad(padName)
	if err != nil {
		return err
	}
	return d.TriggerNote(ctx, note, velocity, durationMS)
}

// TriggerNote sends NoteOn → sleep → NoteOff for a raw MIDI note number.
// ctx cancellation sends NoteOff immediately and returns ctx.Err().
func (d *Device) TriggerNote(ctx context.Context, note, velocity uint8, durationMS int) error {
	ch := d.channelIdx()
	if err := d.Send(gomidi.NoteOn(ch, note, velocity)); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		_ = d.Send(gomidi.NoteOff(ch, note))
		return ctx.Err()
	case <-time.After(time.Duration(durationMS) * time.Millisecond):
	}
	return d.Send(gomidi.NoteOff(ch, note))
}

// SequenceEvent represents a single pad hit in a timed sequence.
// Either Pad or Note must be set; Pad takes precedence when non-empty.
type SequenceEvent struct {
	Pad        string // pad name or empty if Note is set
	Note       uint8  // raw MIDI note (used when Pad is empty)
	Beat       int    // 1-indexed sixteenth-note position within the bar
	Velocity   uint8  // note velocity 1–127; 0 defaults to 100
	DurationMS int    // note-on duration in milliseconds; 0 defaults to 50
}

// PlaySequence triggers a list of timed events at the given BPM.
// Beat positions are 1-indexed sixteenth notes within a 4/4 bar.
// ctx cancellation stops playback mid-sequence.
func (d *Device) PlaySequence(ctx context.Context, events []SequenceEvent, bpm float64) error {
	if bpm <= 0 {
		bpm = 120
	}

	// Duration of one sixteenth note in ms
	sixteenthMS := (60_000.0 / bpm) / 4.0

	// Sort events by beat (simple insertion sort — N is small)
	sorted := make([]SequenceEvent, len(events))
	copy(sorted, events)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].Beat < sorted[j-1].Beat; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	startTime := time.Now()

	for _, ev := range sorted {
		// Calculate time offset for this beat
		beatOffsetMS := float64(ev.Beat-1) * sixteenthMS
		triggerAt := startTime.Add(time.Duration(beatOffsetMS) * time.Millisecond)
		now := time.Now()
		if wait := triggerAt.Sub(now); wait > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}

		dur := ev.DurationMS
		if dur <= 0 {
			dur = 50
		}
		vel := ev.Velocity
		if vel == 0 {
			vel = 100
		}

		var err error
		if ev.Pad != "" {
			err = d.TriggerPad(ctx, ev.Pad, vel, dur)
		} else {
			err = d.TriggerNote(ctx, ev.Note, vel, dur)
		}
		if err != nil {
			return fmt.Errorf("event at beat %d: %w", ev.Beat, err)
		}
	}
	return nil
}
