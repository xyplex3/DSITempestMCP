package midi

import (
	"fmt"
	"time"

	gomidi "gitlab.com/gomidi/midi/v2"
)

// Start sends MIDI Start (0xFA).
func (d *Device) Start() error {
	return d.Send(gomidi.Start())
}

// Stop sends MIDI Stop (0xFC) and halts the internal clock goroutine.
func (d *Device) Stop() error {
	d.mu.Lock()
	d.stopClockLocked()
	d.mu.Unlock()
	return d.Send(gomidi.Stop())
}

// Continue sends MIDI Continue (0xFB).
func (d *Device) Continue() error {
	return d.Send(gomidi.Continue())
}

// SetTempo starts (or restarts) the internal MIDI clock at the given BPM.
// The clock sends 24 Timing Clock pulses per quarter note.
func (d *Device) SetTempo(bpm float64) error {
	if bpm < 20 || bpm > 300 {
		return fmt.Errorf("BPM %g out of range (20–300)", bpm)
	}

	d.mu.Lock()
	d.stopClockLocked()
	stop := make(chan struct{})
	d.clockStop = stop
	d.clockBPM = bpm
	send := d.send
	d.mu.Unlock()

	if send == nil {
		return fmt.Errorf("not connected")
	}

	// One quarter note = 60/bpm seconds → one PPQN tick = 60/(bpm*24) seconds
	interval := time.Duration(float64(time.Minute) / (bpm * 24))

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				_ = send(gomidi.TimingClock())
			}
		}
	}()

	return nil
}

// CurrentBPM returns the BPM the internal clock is currently running at (0 if stopped).
func (d *Device) CurrentBPM() float64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.clockStop == nil {
		return 0
	}
	return d.clockBPM
}
