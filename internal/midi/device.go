// Package midi handles all MIDI communication with the DSI Tempest over USB.
package midi

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	gomidi "gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/drivers"
	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv" // registers the rtmidi driver
)

// Device manages a connection to the Tempest's USB MIDI ports.
// All exported methods are safe for concurrent use by multiple goroutines.
// Create with [New]; call [Device.Connect] before sending messages.
type Device struct {
	mu         sync.Mutex
	cfg        DeviceConfig
	out        drivers.Out
	in         drivers.In
	send       func(gomidi.Message) error
	stopListen func()

	// Clock goroutine control
	clockStop chan struct{}
	clockBPM  float64

	// Fan-out SysEx subscribers — use Subscribe() to register a consumer.
	subMu sync.Mutex
	subs  []chan []byte
}

// DeviceConfig holds the parameters needed to open the Tempest device.
type DeviceConfig struct {
	DeviceName string // substring matched against available MIDI port names
	Channel    uint8  // 1-indexed (stored); converted to 0-indexed when sending
	MIDITrace  bool   // log every raw MIDI byte to stderr when true
}

// New creates an unconnected Device ready to be opened.
func New(cfg DeviceConfig) *Device {
	return &Device{cfg: cfg}
}

// Subscribe returns a channel that receives every incoming SysEx message
// and a cancel function that unsubscribes and closes the channel.
// Each subscriber gets an independent copy of every message.
// The returned channel has a small buffer; slow consumers may miss messages.
func (d *Device) Subscribe() (<-chan []byte, func()) {
	ch := make(chan []byte, 8)
	d.subMu.Lock()
	d.subs = append(d.subs, ch)
	d.subMu.Unlock()

	cancel := func() {
		d.subMu.Lock()
		defer d.subMu.Unlock()
		for i, s := range d.subs {
			if s == ch {
				d.subs = append(d.subs[:i], d.subs[i+1:]...)
				close(ch)
				return
			}
		}
	}
	return ch, cancel
}

// Connect finds and opens the Tempest MIDI ports by name substring match.
func (d *Device) Connect() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	outs, err := drivers.Outs()
	if err != nil {
		return fmt.Errorf("listing MIDI outputs: %w", err)
	}
	ins, err := drivers.Ins()
	if err != nil {
		return fmt.Errorf("listing MIDI inputs: %w", err)
	}

	name := strings.ToLower(d.cfg.DeviceName)

	// Find output port
	var outPort drivers.Out
	for _, o := range outs {
		if strings.Contains(strings.ToLower(o.String()), name) {
			outPort = o
			break
		}
	}
	if outPort == nil {
		return fmt.Errorf("no MIDI output matching %q (available: %s)", d.cfg.DeviceName, portNames(outs))
	}

	// Find input port
	var inPort drivers.In
	for _, i := range ins {
		if strings.Contains(strings.ToLower(i.String()), name) {
			inPort = i
			break
		}
	}
	if inPort == nil {
		return fmt.Errorf("no MIDI input matching %q (available: %s)", d.cfg.DeviceName, portNames(ins))
	}

	// Open output
	if err := outPort.Open(); err != nil {
		return fmt.Errorf("opening output %q: %w", outPort, err)
	}
	send, err := gomidi.SendTo(outPort)
	if err != nil {
		_ = outPort.Close()
		return fmt.Errorf("creating sender for %q: %w", outPort, err)
	}

	// Open input and start listening for SysEx
	if err := inPort.Open(); err != nil {
		_ = outPort.Close()
		return fmt.Errorf("opening input %q: %w", inPort, err)
	}
	stopListen, err := gomidi.ListenTo(inPort, d.handleIncoming, gomidi.UseSysEx())
	if err != nil {
		_ = inPort.Close()
		_ = outPort.Close()
		return fmt.Errorf("starting listener on %q: %w", inPort, err)
	}

	d.out = outPort
	d.in = inPort
	d.send = send
	d.stopListen = stopListen
	return nil
}

// Disconnect closes the MIDI ports and stops any running clock.
func (d *Device) Disconnect() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stopClockLocked()

	if d.stopListen != nil {
		d.stopListen()
		d.stopListen = nil
	}
	if d.in != nil {
		_ = d.in.Close()
		d.in = nil
	}
	if d.out != nil {
		_ = d.out.Close()
		d.out = nil
	}
	d.send = nil
}

// IsConnected reports whether the device ports are open.
func (d *Device) IsConnected() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.send != nil
}

// Name returns the device name string used for port matching.
func (d *Device) Name() string { return d.cfg.DeviceName }

// Channel returns the 1-indexed MIDI channel from config.
func (d *Device) Channel() uint8 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.cfg.Channel
}

// SetChannel updates the MIDI channel (1-indexed) used for note and CC messages.
func (d *Device) SetChannel(ch uint8) {
	d.mu.Lock()
	d.cfg.Channel = ch
	d.mu.Unlock()
}

// channelIdx returns the 0-indexed channel for use in MIDI messages.
func (d *Device) channelIdx() uint8 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.cfg.Channel - 1
}

// Send transmits a MIDI message to the Tempest. Caller must not hold mu.
func (d *Device) Send(msg gomidi.Message) error {
	d.mu.Lock()
	s := d.send
	d.mu.Unlock()

	if s == nil {
		return fmt.Errorf("not connected")
	}
	if d.cfg.MIDITrace {
		fmt.Fprintf(os.Stderr, "[MIDI OUT] %v\n", msg)
	}
	return s(msg)
}

// SendRaw transmits a raw SysEx byte slice (including F0 / F7).
// It strips the F0/F7 before passing to gomidi which re-adds them.
func (d *Device) SendRaw(raw []byte) error {
	if len(raw) < 2 || raw[0] != 0xF0 || raw[len(raw)-1] != 0xF7 {
		return fmt.Errorf("invalid SysEx: must start with F0 and end with F7")
	}
	inner := raw[1 : len(raw)-1]
	return d.Send(gomidi.SysEx(inner))
}

// SendRawWithDelay sends each raw SysEx message with inter-message delay.
// ctx cancellation is honoured between messages.
func (d *Device) SendRawWithDelay(ctx context.Context, messages [][]byte, delayMS int) error {
	for i, msg := range messages {
		if err := d.SendRaw(msg); err != nil {
			return fmt.Errorf("message %d: %w", i, err)
		}
		if i < len(messages)-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(delayMS) * time.Millisecond):
			}
		}
	}
	return nil
}

// ListPorts returns all available MIDI output and input port names.
func ListPorts() (outs []string, ins []string, err error) {
	outPorts, e := drivers.Outs()
	if e != nil {
		return nil, nil, e
	}
	inPorts, e := drivers.Ins()
	if e != nil {
		return nil, nil, e
	}
	for _, o := range outPorts {
		outs = append(outs, o.String())
	}
	for _, i := range inPorts {
		ins = append(ins, i.String())
	}
	return
}

// handleIncoming is called by the gomidi listener for every incoming message.
func (d *Device) handleIncoming(msg gomidi.Message, timestampMS int32) {
	if d.cfg.MIDITrace {
		fmt.Fprintf(os.Stderr, "[MIDI IN ] %v\n", msg)
	}
	var data []byte
	if msg.GetSysEx(&data) {
		// Re-add F0/F7 to match raw Tempest format
		full := make([]byte, len(data)+2)
		full[0] = 0xF0
		copy(full[1:], data)
		full[len(full)-1] = 0xF7

		d.subMu.Lock()
		for _, ch := range d.subs {
			select {
			case ch <- full:
			default:
				fmt.Fprintf(os.Stderr, "[MIDI] SysEx subscriber buffer full; dropping message\n")
			}
		}
		d.subMu.Unlock()
	}
}

// stopClockLocked stops the clock goroutine; caller must hold mu.
func (d *Device) stopClockLocked() {
	if d.clockStop != nil {
		close(d.clockStop)
		d.clockStop = nil
	}
}

// portNames returns a comma-separated list of port names for error messages.
func portNames[T interface{ String() string }](ports []T) string {
	names := make([]string, len(ports))
	for i, p := range ports {
		names[i] = p.String()
	}
	return strings.Join(names, ", ")
}
