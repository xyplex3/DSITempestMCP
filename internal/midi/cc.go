package midi

import (
	"fmt"
	"strings"

	gomidi "gitlab.com/gomidi/midi/v2"
)

// beatFXMap maps friendly parameter names to MIDI CC numbers.
// All are beat-wide (affect all voices). Available from OS 1.3.1.6+.
var beatFXMap = map[string]uint8{
	"distortion":        12,
	"compression":       13,
	"reset-beat-fx":     19,
	"all-osc-freq":      20,
	"all-osc-pitch":     20,
	"feedback":          21,
	"lp-cutoff":         22,
	"lp-filter":         22,
	"lowpass-cutoff":    22,
	"lp-resonance":      23,
	"lowpass-resonance": 23,
	"lp-audio-mod":      24,
	"hp-cutoff":         25,
	"hp-filter":         25,
	"highpass-cutoff":   25,
	"env-attack":        26,
	"all-attack":        26,
	"env-decay":         27,
	"all-decay":         27,
}

// validCCs is the set of CC numbers the Tempest responds to.
var validCCs = map[uint8]string{
	12: "Distortion",
	13: "Compression",
	19: "Reset Beat FX",
	20: "All Osc Freq",
	21: "VCA Feedback",
	22: "LP Cutoff",
	23: "LP Resonance",
	24: "LP Audio Mod",
	25: "HP Cutoff",
	26: "Env Attack",
	27: "Env Decay",
}

// SendCC sends a MIDI CC on the configured channel.
// cc must be one of the 11 Tempest-supported CC numbers.
func (d *Device) SendCC(cc, value uint8) error {
	if _, ok := validCCs[cc]; !ok {
		return fmt.Errorf("CC %d is not a Tempest Beat FX CC (valid: 12, 13, 19–27)", cc)
	}
	if value > 127 {
		value = 127
	}
	ch := d.channelIdx()
	return d.Send(gomidi.ControlChange(ch, cc, value))
}

// SetBeatFX sends a named Beat FX parameter.
func (d *Device) SetBeatFX(param string, value uint8) error {
	cc, ok := beatFXMap[strings.ToLower(strings.TrimSpace(param))]
	if !ok {
		return fmt.Errorf("unknown Beat FX param %q — valid names: %s", param, beatFXNames())
	}
	return d.SendCC(cc, value)
}

// CCName returns the human-readable name for a CC number.
func CCName(cc uint8) string {
	if name, ok := validCCs[cc]; ok {
		return name
	}
	return fmt.Sprintf("CC%d", cc)
}

// ListBeatFXParams returns all named Beat FX parameter names.
func ListBeatFXParams() []string {
	names := make([]string, 0, len(beatFXMap))
	for k := range beatFXMap {
		names = append(names, k)
	}
	return names
}

func beatFXNames() string {
	names := ListBeatFXParams()
	return strings.Join(names, ", ")
}
