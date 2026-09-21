package sysex_test

import (
	"os"
	"testing"

	"tempest-mcp/internal/sysex"
)

// decodeSoundCapture loads a hardware-captured RAM (0x60) .syx file and
// decodes its Sound parameters.
func decodeSoundCapture(t *testing.T, path string) map[string]int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	msgType := sysex.Identify(raw)
	if msgType != sysex.TypeRAMSound {
		t.Fatalf("%s: got message type %v, want TypeRAMSound", path, msgType)
	}
	unescaped := sysex.Unescape(raw)
	params := sysex.ExtractParams(unescaped, msgType)
	return sysex.DecodeSoundParams(params)
}

// TestDecodeSoundParams_HardwareCaptures decodes the three real hardware
// captures in testdata/sound-research/ and asserts the parameter changes
// confirmed live on the Tempest (docs/sysex-tempest-format.md §7 and §8.0):
// setting Pitch Env Attack, then (without reinitializing) also setting LP
// Env Release and toggling AD Mode. This is the 3/3 hardware validation of
// SoundParams' bit locations translated from the community bit map.
func TestDecodeSoundParams_HardwareCaptures(t *testing.T) {
	const dir = "testdata/sound-research/"
	base := decodeSoundCapture(t, dir+"sound_baseline.syx")

	t.Run("pitch attack capture changes only Pitch Env Attack", func(t *testing.T) {
		got := decodeSoundCapture(t, dir+"sound_pitch_attack.syx")
		want := map[string]int{"Pitch Env Attack": 59}
		assertOnlyDiffs(t, base, got, want)
	})

	t.Run("lp env release capture is cumulative over the pitch attack edit", func(t *testing.T) {
		got := decodeSoundCapture(t, dir+"sound_lp_env_release.syx")
		want := map[string]int{
			"Pitch Env Attack": 59, // carried over from the prior, un-reinitialized edit
			"LP Env Release":   44,
			"AD Mode":          1,
		}
		assertOnlyDiffs(t, base, got, want)
	})
}

// assertOnlyDiffs checks that got differs from base at exactly the keys in
// want (with the given values), and nowhere else.
func assertOnlyDiffs(t *testing.T, base, got, want map[string]int) {
	t.Helper()
	for name, wantVal := range want {
		gotVal, ok := got[name]
		if !ok {
			t.Errorf("missing parameter %q in decoded capture", name)
			continue
		}
		if gotVal != wantVal {
			t.Errorf("%s = %d, want %d", name, gotVal, wantVal)
		}
	}
	for name, baseVal := range base {
		if _, changed := want[name]; changed {
			continue
		}
		if gotVal, ok := got[name]; ok && gotVal != baseVal {
			t.Errorf("unexpected diff at %s: base=%d got=%d", name, baseVal, gotVal)
		}
	}
}

// TestDisplaySoundParam covers every SoundParam.Encoding branch.
func TestDisplaySoundParam(t *testing.T) {
	tests := []struct {
		name        string
		encoding    string
		raw         int
		wantDisplay string
		wantNumeric float64
		wantOK      bool
	}{
		{"raw passthrough", "", 42, "", 42, false},
		{"offset64 below center", "offset64", 0, "", -64, true},
		{"offset64 above center", "offset64", 127, "", 63, true},
		{"offset50 cents", "offset50", 100, "", 50, true},
		{"bipolar127 negative", "bipolar127", 0, "", -127, true},
		{"bipolar127 positive", "bipolar127", 254, "", 127, true},
		{"modSource known", "modSource", 6, "LFO 1", 6, true},
		{"modSource unknown falls back", "modSource", 99, "", 99, false},
		{"modDest known", "modDest", 18, "VCA Level", 18, true},
		{"modDest unknown falls back", "modDest", 999, "", 999, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := sysex.SoundParam{Encoding: tt.encoding}
			display, numeric, ok := sysex.DisplaySoundParam(p, tt.raw)
			if display != tt.wantDisplay || numeric != tt.wantNumeric || ok != tt.wantOK {
				t.Errorf("DisplaySoundParam(%q, %d) = (%q, %v, %v), want (%q, %v, %v)",
					tt.encoding, tt.raw, display, numeric, ok, tt.wantDisplay, tt.wantNumeric, tt.wantOK)
			}
		})
	}
}
