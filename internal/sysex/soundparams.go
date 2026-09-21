// Code generated from the Tempest SysEx Bit Map gist
// (https://gist.github.com/fadeddata/c39a3b4b10e1e51af58e49ef74aca116),
// translated from its raw-wire-byte.bit frame into this package's confirmed
// collector-first unpacked-payload frame. DO NOT EDIT BY HAND — see
// docs/sysex-tempest-format.md §6 for the source table and translation
// formula, and §7 for the 3 hardware data points that validated it
// (Pitch Env Attack, LP Env Release, AD Mode — all confirmed byte-exact).

package sysex

// SoundParamBit locates one bit of a Sound (0x60) body parameter within the
// unpacked payload (see Unescape7Plus1). Bit 0 is the LSB, bit 7 the MSB.
type SoundParamBit struct {
	Byte int
	Bit  int
}

// SoundParam describes one synthesis parameter's bit layout in a Sound (0x60)
// body. Bits are ordered LSB (param bit 0) to MSB.
type SoundParam struct {
	Name    string
	Section string
	Bits    []SoundParamBit
	// Encoding: "" (raw integer), "offset64" (display = raw-64, semitones),
	// "offset50" (display = raw-50, cents), "bipolar127" (display = raw-127),
	// "modSource" (look up in ModSourceNames), "modDest" (ModDestNames).
	Encoding string
	// Note carries the source bit map's free-text range/meaning — not
	// mechanically translated, included for human reference.
	Note string
}

// SoundParams is every synthesis parameter's confirmed bit location in a
// Sound (0x60) body, translated from the community bit map. Bit *locations*
// come from the gist's own hardware single-bit-diffing (see its Sources doc)
// passed through this repo's independently-confirmed unpack scheme; both
// halves have hardware confirmation, but the combination has only been
// spot-checked on 3 parameters (see docs/sysex-tempest-format.md §7) —
// validate more before relying on this for a write path.
var SoundParams = []SoundParam{
	{
		Name: "Osc 1 Shape", Section: "Oscillator 1 Section (fully mapped)",
		Bits:     []SoundParamBit{{1, 6}, {1, 7}, {2, 0}, {2, 1}, {2, 2}, {2, 3}, {2, 4}},
		Encoding: "", Note: "7 bits, range 0-103: Off=0, Saw=1, Tri=2, Saw-Tri=3, Pulse 0-99%=4-103",
	},
	{
		Name: "Osc 1/2 Mix", Section: "Oscillator 1 Section (fully mapped)",
		Bits:     []SoundParamBit{{17, 2}, {17, 3}, {17, 4}, {17, 5}, {17, 6}, {17, 7}, {18, 0}},
		Encoding: "", Note: "7 bits, range 0-127: 0=100/0, 64=50/50, 127=0/100",
	},
	{
		Name: "Osc 1 Frequency", Section: "Oscillator 1 Section (fully mapped)",
		Bits:     []SoundParamBit{{0, 0}, {0, 1}, {0, 2}, {0, 3}, {0, 4}, {0, 5}, {0, 6}},
		Encoding: "", Note: "7 bits, range 0-127, C0-C10 semitones",
	},
	{
		Name: "Osc 1 Fine Freq", Section: "Oscillator 1 Section (fully mapped)",
		Bits:     []SoundParamBit{{0, 7}, {1, 0}, {1, 1}, {1, 2}, {1, 3}, {1, 4}, {1, 5}},
		Encoding: "offset50", Note: "7 bits, range 0-100, displayed as -50 to +50 cents",
	},
	{
		Name: "Osc 1 Glide", Section: "Oscillator 1 Section (fully mapped)",
		Bits:     []SoundParamBit{{2, 5}, {2, 6}, {2, 7}, {3, 0}, {3, 1}, {3, 2}, {3, 3}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Sync 2>1", Section: "Oscillator 1 Section (fully mapped)",
		Bits:     []SoundParamBit{{16, 0}},
		Encoding: "", Note: "1 bit, Off/On",
	},
	{
		Name: "Sub Osc", Section: "Oscillator 1 Section (fully mapped)",
		Bits:     []SoundParamBit{{18, 1}, {18, 2}, {18, 3}, {18, 4}, {18, 5}, {18, 6}, {18, 7}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Osc Slop", Section: "Oscillator 1 Section (fully mapped)",
		Bits:     []SoundParamBit{{16, 3}, {16, 4}, {16, 5}},
		Encoding: "", Note: "3 bits, range 0-5",
	},
	{
		Name: "Glide Mode", Section: "Oscillator 1 Section (fully mapped)",
		Bits:     []SoundParamBit{{16, 2}},
		Encoding: "", Note: "1 bit, FixRate=0, FixTime=1",
	},
	{
		Name: "Key Follow", Section: "Oscillator 1 Section (fully mapped)",
		Bits:     []SoundParamBit{{3, 4}},
		Encoding: "", Note: "1 bit, Off/On",
	},
	{
		Name: "Wave Reset", Section: "Oscillator 1 Section (fully mapped)",
		Bits:     []SoundParamBit{{3, 5}},
		Encoding: "", Note: "1 bit, Off/On",
	},
	{
		Name: "Key Assign", Section: "Oscillator 1 Section (fully mapped)",
		Bits:     []SoundParamBit{{93, 7}},
		Encoding: "", Note: "1 bit, Last Retrig=0, Last Note=1",
	},
	{
		Name: "Osc 2 Shape", Section: "Oscillator 2 Section (fully mapped)",
		Bits:     []SoundParamBit{{5, 4}, {5, 5}, {5, 6}, {5, 7}, {6, 0}, {6, 1}, {6, 2}},
		Encoding: "", Note: "7 bits, range 0-103",
	},
	{
		Name: "Osc 2 Frequency", Section: "Oscillator 2 Section (fully mapped)",
		Bits:     []SoundParamBit{{3, 6}, {3, 7}, {4, 0}, {4, 1}, {4, 2}, {4, 3}, {4, 4}},
		Encoding: "", Note: "7 bits, range 0-127, C0-C10",
	},
	{
		Name: "Osc 2 Fine Freq", Section: "Oscillator 2 Section (fully mapped)",
		Bits:     []SoundParamBit{{4, 5}, {4, 6}, {4, 7}, {5, 0}, {5, 1}, {5, 2}, {5, 3}},
		Encoding: "offset50", Note: "7 bits, range 0-100, displayed -50 to +50",
	},
	{
		Name: "Osc 2 Glide", Section: "Oscillator 2 Section (fully mapped)",
		Bits:     []SoundParamBit{{6, 3}, {6, 4}, {6, 5}, {6, 6}, {6, 7}, {7, 0}, {7, 1}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Osc 2 Key Follow", Section: "Oscillator 2 Section (fully mapped)",
		Bits:     []SoundParamBit{{7, 2}},
		Encoding: "", Note: "1 bit, Off/On",
	},
	{
		Name: "Osc 2 Wave Reset", Section: "Oscillator 2 Section (fully mapped)",
		Bits:     []SoundParamBit{{7, 3}},
		Encoding: "", Note: "1 bit, Off/On",
	},
	{
		Name: "LPF Freq", Section: "Filter Section (fully mapped)",
		Bits:     []SoundParamBit{{22, 4}, {22, 5}, {22, 6}, {22, 7}, {23, 0}, {23, 1}, {23, 2}, {23, 3}},
		Encoding: "", Note: "8 bits, range 0-164",
	},
	{
		Name: "Resonance", Section: "Filter Section (fully mapped)",
		Bits:     []SoundParamBit{{23, 4}, {23, 5}, {23, 6}, {23, 7}, {24, 0}, {24, 1}, {24, 2}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Audio Mod", Section: "Filter Section (fully mapped)",
		Bits:     []SoundParamBit{{27, 1}, {27, 2}, {27, 3}, {27, 4}, {27, 5}, {27, 6}, {27, 7}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "4 Pole", Section: "Filter Section (fully mapped)",
		Bits:     []SoundParamBit{{28, 0}},
		Encoding: "", Note: "1 bit, Off/On",
	},
	{
		Name: "HP Freq", Section: "Filter Section (fully mapped)",
		Bits:     []SoundParamBit{{28, 7}, {29, 0}, {29, 1}, {29, 2}, {29, 3}, {29, 4}, {29, 5}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "LP Key>Freq", Section: "Filter Section (fully mapped)",
		Bits:     []SoundParamBit{{24, 3}, {24, 4}, {24, 5}, {24, 6}, {24, 7}, {25, 0}, {25, 1}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "HP Key>Freq", Section: "Filter Section (fully mapped)",
		Bits:     []SoundParamBit{{29, 6}, {29, 7}, {30, 0}, {30, 1}, {30, 2}, {30, 3}, {30, 4}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Osc 3 Sample", Section: "Oscillator 3 Section (partial)",
		Bits:     []SoundParamBit{{9, 2}, {9, 3}, {9, 4}, {9, 5}, {9, 6}, {9, 7}, {10, 0}, {10, 2}, {10, 3}},
		Encoding: "", Note: "9 bits, range 0-464",
	},
	{
		Name: "Osc 3 Level", Section: "Oscillator 3 Section (partial)",
		Bits:     []SoundParamBit{{19, 0}, {19, 1}, {19, 2}, {19, 3}, {19, 4}, {19, 5}, {19, 6}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Osc 3 Frequency", Section: "Oscillator 3 Section (partial)",
		Bits:     []SoundParamBit{{7, 4}, {7, 5}, {7, 6}, {7, 7}, {8, 0}, {8, 1}, {8, 2}},
		Encoding: "offset64", Note: "7 bits, range 0-127, displayed as semitones: value - 64",
	},
	{
		Name: "Osc 3 Fine Freq", Section: "Oscillator 3 Section (partial)",
		Bits:     []SoundParamBit{{8, 3}, {8, 4}, {8, 5}, {8, 6}, {8, 7}, {9, 0}, {9, 1}},
		Encoding: "offset50", Note: "7 bits, range 0-100, displayed as cents: value - 50",
	},
	{
		Name: "Osc 3 Reverse", Section: "Oscillator 3 Section (partial)",
		Bits:     []SoundParamBit{{93, 5}},
		Encoding: "", Note: "1 bit, Off/On",
	},
	{
		Name: "Osc 3 Pre/Post Filter", Section: "Oscillator 3 Section (partial)",
		Bits:     []SoundParamBit{{20, 6}, {20, 7}, {21, 0}, {21, 1}, {21, 2}, {21, 3}, {21, 4}},
		Encoding: "", Note: "7 bits, range 0-127: 0=all filtered, 64=equal, 127=bypass",
	},
	{
		Name: "Osc 3 Key Follow", Section: "Oscillator 3 Section (partial)",
		Bits:     []SoundParamBit{{11, 5}},
		Encoding: "", Note: "1 bit, Off/On",
	},
	{
		Name: "Osc 4 Sample", Section: "Oscillator 4 Section (fully mapped)",
		Bits:     []SoundParamBit{{13, 4}, {13, 5}, {13, 6}, {13, 7}, {14, 0}, {14, 1}, {14, 2}, {14, 4}, {14, 5}},
		Encoding: "", Note: "9 bits, range 0-464",
	},
	{
		Name: "Osc 4 Level", Section: "Oscillator 4 Section (fully mapped)",
		Bits:     []SoundParamBit{{19, 7}, {20, 0}, {20, 1}, {20, 2}, {20, 3}, {20, 4}, {20, 5}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Osc 4 Frequency", Section: "Oscillator 4 Section (fully mapped)",
		Bits:     []SoundParamBit{{11, 6}, {11, 7}, {12, 0}, {12, 1}, {12, 2}, {12, 3}, {12, 4}},
		Encoding: "offset64", Note: "7 bits, internal 0-127, displayed as semitones: value - 64",
	},
	{
		Name: "Osc 4 Fine Freq", Section: "Oscillator 4 Section (fully mapped)",
		Bits:     []SoundParamBit{{12, 5}, {12, 6}, {12, 7}, {13, 0}, {13, 1}, {13, 2}, {13, 3}},
		Encoding: "offset50", Note: "7 bits, range 0-100, displayed as cents: value - 50",
	},
	{
		Name: "Osc 4 Reverse", Section: "Oscillator 4 Section (fully mapped)",
		Bits:     []SoundParamBit{{93, 6}},
		Encoding: "", Note: "1 bit, Off/On",
	},
	{
		Name: "Osc 4 Key Follow", Section: "Oscillator 4 Section (fully mapped)",
		Bits:     []SoundParamBit{{15, 7}},
		Encoding: "", Note: "1 bit, Off/On",
	},
	{
		Name: "AD Mode", Section: "Pitch Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{40, 3}},
		Encoding: "", Note: "1 bit, Off=1/ADSR, On=0/AD — inverted",
	},
	{
		Name: "Pitch Env Attack", Section: "Pitch Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{44, 0}, {44, 1}, {44, 2}, {44, 3}, {44, 4}, {44, 5}, {44, 6}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Pitch Env Decay", Section: "Pitch Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{44, 7}, {45, 0}, {45, 1}, {45, 2}, {45, 3}, {45, 4}, {45, 5}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Pitch Env Amount", Section: "Pitch Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{41, 2}, {41, 3}, {41, 4}, {41, 5}, {41, 6}, {41, 7}, {42, 0}, {42, 1}},
		Encoding: "bipolar127", Note: "8 bits, range 0-254, displayed as value - 127",
	},
	{
		Name: "Pitch Env Sustain", Section: "Pitch Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{45, 6}, {45, 7}, {46, 0}, {46, 1}, {46, 2}, {46, 3}, {46, 4}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Pitch Env Release", Section: "Pitch Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{46, 5}, {46, 6}, {46, 7}, {47, 0}, {47, 1}, {47, 2}, {47, 3}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Pitch Env Velocity Amount", Section: "Pitch Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{42, 2}, {42, 3}, {42, 4}, {42, 5}, {42, 6}, {42, 7}, {43, 0}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Pitch Env Delay", Section: "Pitch Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{43, 1}, {43, 2}, {43, 3}, {43, 4}, {43, 5}, {43, 6}, {43, 7}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Pitch Env Peak", Section: "Pitch Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{89, 2}, {89, 3}, {89, 4}, {89, 5}, {89, 6}, {89, 7}, {90, 0}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Pitch Env Destination", Section: "Pitch Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{40, 4}, {40, 5}, {40, 6}, {40, 7}, {41, 0}, {41, 1}},
		Encoding: "modDest", Note: "6 bits, range 0-58",
	},
	{
		Name: "LP Env Attack", Section: "Lowpass Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{48, 3}, {48, 4}, {48, 5}, {48, 6}, {48, 7}, {49, 0}, {49, 1}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "LP Env Decay", Section: "Lowpass Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{49, 2}, {49, 3}, {49, 4}, {49, 5}, {49, 6}, {49, 7}, {50, 0}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "LP Env Amount", Section: "Lowpass Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{25, 2}, {25, 3}, {25, 4}, {25, 5}, {25, 6}, {25, 7}, {26, 0}, {26, 1}},
		Encoding: "bipolar127", Note: "8 bits, range 0-254, displayed as value - 127",
	},
	{
		Name: "LP Env Peak", Section: "Lowpass Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{90, 1}, {90, 2}, {90, 3}, {90, 4}, {90, 5}, {90, 6}, {90, 7}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "LP Env Sustain", Section: "Lowpass Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{50, 1}, {50, 2}, {50, 3}, {50, 4}, {50, 5}, {50, 6}, {50, 7}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "LP Env Release", Section: "Lowpass Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{51, 0}, {51, 1}, {51, 2}, {51, 3}, {51, 4}, {51, 5}, {51, 6}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "LP Env Velocity Amount", Section: "Lowpass Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{26, 2}, {26, 3}, {26, 4}, {26, 5}, {26, 6}, {26, 7}, {27, 0}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "LP Env Delay", Section: "Lowpass Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{47, 4}, {47, 5}, {47, 6}, {47, 7}, {48, 0}, {48, 1}, {48, 2}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Amp Env Attack", Section: "Amp (VCA) Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{52, 6}, {52, 7}, {53, 0}, {53, 1}, {53, 2}, {53, 3}, {53, 4}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Amp Env Decay", Section: "Amp (VCA) Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{53, 5}, {53, 6}, {53, 7}, {54, 0}, {54, 1}, {54, 2}, {54, 3}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Amp Env Amount", Section: "Amp (VCA) Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{31, 4}, {31, 5}, {31, 6}, {31, 7}, {32, 0}, {32, 1}, {32, 2}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Amp Env Peak", Section: "Amp (VCA) Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{91, 0}, {91, 1}, {91, 2}, {91, 3}, {91, 4}, {91, 5}, {91, 6}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Amp Env Sustain", Section: "Amp (VCA) Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{54, 4}, {54, 5}, {54, 6}, {54, 7}, {55, 0}, {55, 1}, {55, 2}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Amp Env Release", Section: "Amp (VCA) Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{55, 3}, {55, 4}, {55, 5}, {55, 6}, {55, 7}, {56, 0}, {56, 1}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Amp Env Velocity Amount", Section: "Amp (VCA) Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{32, 3}, {32, 4}, {32, 5}, {32, 6}, {32, 7}, {33, 0}, {33, 1}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Amp Env Delay", Section: "Amp (VCA) Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{51, 7}, {52, 0}, {52, 1}, {52, 2}, {52, 3}, {52, 4}, {52, 5}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Aux 1 Env Attack", Section: "Aux 1 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{59, 6}, {59, 7}, {60, 0}, {60, 1}, {60, 2}, {60, 3}, {60, 4}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Aux 1 Env Decay", Section: "Aux 1 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{60, 5}, {60, 6}, {60, 7}, {61, 0}, {61, 1}, {61, 2}, {61, 3}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Aux 1 Env Amount", Section: "Aux 1 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{57, 0}, {57, 1}, {57, 2}, {57, 3}, {57, 4}, {57, 5}, {57, 6}, {57, 7}},
		Encoding: "bipolar127", Note: "8 bits, range 0-254, displayed as value - 127",
	},
	{
		Name: "Aux 1 Env Peak", Section: "Aux 1 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{91, 7}, {92, 0}, {92, 1}, {92, 2}, {92, 3}, {92, 4}, {92, 5}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Aux 1 Env Sustain", Section: "Aux 1 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{61, 4}, {61, 5}, {61, 6}, {61, 7}, {62, 0}, {62, 1}, {62, 2}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Aux 1 Env Release", Section: "Aux 1 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{62, 3}, {62, 4}, {62, 5}, {62, 6}, {62, 7}, {63, 0}, {63, 1}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Aux 1 Env Velocity Amount", Section: "Aux 1 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{58, 0}, {58, 1}, {58, 2}, {58, 3}, {58, 4}, {58, 5}, {58, 6}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Aux 1 Env Delay", Section: "Aux 1 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{58, 7}, {59, 0}, {59, 1}, {59, 2}, {59, 3}, {59, 4}, {59, 5}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Aux 1 Env Destination", Section: "Aux 1 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{56, 2}, {56, 3}, {56, 4}, {56, 5}, {56, 6}, {56, 7}},
		Encoding: "modDest", Note: "6 bits, range 0-58",
	},
	{
		Name: "Aux 2 Env Attack", Section: "Aux 2 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{66, 6}, {66, 7}, {67, 0}, {67, 1}, {67, 2}, {67, 3}, {67, 4}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Aux 2 Env Decay", Section: "Aux 2 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{67, 5}, {67, 6}, {67, 7}, {68, 0}, {68, 1}, {68, 2}, {68, 3}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Aux 2 Env Amount", Section: "Aux 2 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{64, 0}, {64, 1}, {64, 2}, {64, 3}, {64, 4}, {64, 5}, {64, 6}, {64, 7}},
		Encoding: "bipolar127", Note: "8 bits, range 0-254, displayed as value - 127",
	},
	{
		Name: "Aux 2 Env Peak", Section: "Aux 2 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{92, 6}, {92, 7}, {93, 0}, {93, 1}, {93, 2}, {93, 3}, {93, 4}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Aux 2 Env Sustain", Section: "Aux 2 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{68, 4}, {68, 5}, {68, 6}, {68, 7}, {69, 0}, {69, 1}, {69, 2}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Aux 2 Env Release", Section: "Aux 2 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{69, 3}, {69, 4}, {69, 5}, {69, 6}, {69, 7}, {70, 0}, {70, 1}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Aux 2 Env Velocity Amount", Section: "Aux 2 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{65, 0}, {65, 1}, {65, 2}, {65, 3}, {65, 4}, {65, 5}, {65, 6}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Aux 2 Env Delay", Section: "Aux 2 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{65, 7}, {66, 0}, {66, 1}, {66, 2}, {66, 3}, {66, 4}, {66, 5}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Aux 2 Env Destination", Section: "Aux 2 Envelope Section (fully mapped)",
		Bits:     []SoundParamBit{{63, 2}, {63, 3}, {63, 4}, {63, 5}, {63, 6}, {63, 7}},
		Encoding: "modDest", Note: "6 bits, range 0-58",
	},
	{
		Name: "LFO 1 Shape", Section: "LFO 1 Section (partial)",
		Bits:     []SoundParamBit{{35, 1}, {35, 2}, {35, 3}},
		Encoding: "", Note: "3 bits, range 0-4: Tri=0, RevSaw=1, Saw=2, Square=3, Random=4",
	},
	{
		Name: "LFO 1 Rate", Section: "LFO 1 Section (partial)",
		Bits:     []SoundParamBit{{34, 1}, {34, 2}, {34, 3}, {34, 4}, {34, 5}, {34, 6}, {34, 7}, {35, 0}},
		Encoding: "", Note: "8 bits, range 0-162",
	},
	{
		Name: "LFO 1 Sync", Section: "LFO 1 Section (partial)",
		Bits:     []SoundParamBit{{37, 1}},
		Encoding: "", Note: "1 bit, Off/On",
	},
	{
		Name: "LFO 1 Amount", Section: "LFO 1 Section (partial)",
		Bits:     []SoundParamBit{{35, 4}, {35, 5}, {35, 6}, {35, 7}, {36, 0}, {36, 1}, {36, 2}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "LFO 1 Destination", Section: "LFO 1 Section (partial)",
		Bits:     []SoundParamBit{{36, 3}, {36, 4}, {36, 5}, {36, 6}, {36, 7}, {37, 0}},
		Encoding: "modDest", Note: "6 bits, range 0-58",
	},
	{
		Name: "LFO 1 Restart", Section: "LFO 1 Section (partial)",
		Bits:     []SoundParamBit{{95, 0}, {95, 1}},
		Encoding: "", Note: "2 bits, range 0-3: Off=0, Note=1, Beat=2, Play=3",
	},
	{
		Name: "LFO 2 Shape", Section: "LFO 2 Section (fully mapped)",
		Bits:     []SoundParamBit{{38, 2}, {38, 3}, {38, 4}},
		Encoding: "", Note: "3 bits, range 0-4: Tri=0, RevSaw=1, Saw=2, Square=3, Random=4",
	},
	{
		Name: "LFO 2 Rate", Section: "LFO 2 Section (fully mapped)",
		Bits:     []SoundParamBit{{37, 2}, {37, 3}, {37, 4}, {37, 5}, {37, 6}, {37, 7}, {38, 0}, {38, 1}},
		Encoding: "", Note: "8 bits, range 0-162",
	},
	{
		Name: "LFO 2 Sync", Section: "LFO 2 Section (fully mapped)",
		Bits:     []SoundParamBit{{40, 2}},
		Encoding: "", Note: "1 bit, Off/On",
	},
	{
		Name: "LFO 2 Amount", Section: "LFO 2 Section (fully mapped)",
		Bits:     []SoundParamBit{{38, 5}, {38, 6}, {38, 7}, {39, 0}, {39, 1}, {39, 2}, {39, 3}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "LFO 2 Destination", Section: "LFO 2 Section (fully mapped)",
		Bits:     []SoundParamBit{{39, 4}, {39, 5}, {39, 6}, {39, 7}, {40, 0}, {40, 1}},
		Encoding: "modDest", Note: "6 bits, range 0-58",
	},
	{
		Name: "LFO 2 Restart", Section: "LFO 2 Section (fully mapped)",
		Bits:     []SoundParamBit{{95, 2}, {95, 3}},
		Encoding: "", Note: "2 bits, range 0-3: Off=0, Note=1, Beat=2, Play=3",
	},
	{
		Name: "VCA Volume", Section: "VCA / Feedback Section (fully mapped)",
		Bits:     []SoundParamBit{{33, 2}, {33, 3}, {33, 4}, {33, 5}, {33, 6}, {33, 7}, {34, 0}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "VCA Feedback", Section: "VCA / Feedback Section (fully mapped)",
		Bits:     []SoundParamBit{{21, 5}, {21, 6}, {21, 7}, {22, 0}, {22, 1}, {22, 2}, {22, 3}},
		Encoding: "", Note: "7 bits, range 0-127",
	},
	{
		Name: "Pitchbend Range", Section: "Misc Section",
		Bits:     []SoundParamBit{{16, 6}, {16, 7}, {17, 0}, {17, 1}},
		Encoding: "", Note: "4 bits, range 0-12 semitones",
	},
	{
		Name: "Mod 1 Source", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{70, 2}, {70, 3}, {70, 4}, {70, 5}, {70, 6}},
		Encoding: "modSource", Note: "",
	},
	{
		Name: "Mod 1 Amount", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{70, 7}, {71, 0}, {71, 1}, {71, 2}, {71, 3}, {71, 4}, {71, 5}, {71, 6}},
		Encoding: "bipolar127", Note: "",
	},
	{
		Name: "Mod 1 Destination", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{71, 7}, {72, 0}, {72, 1}, {72, 2}, {72, 3}, {72, 4}},
		Encoding: "modDest", Note: "",
	},
	{
		Name: "Mod 2 Source", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{72, 5}, {72, 6}, {72, 7}, {73, 0}, {73, 1}},
		Encoding: "modSource", Note: "",
	},
	{
		Name: "Mod 2 Amount", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{73, 2}, {73, 3}, {73, 4}, {73, 5}, {73, 6}, {73, 7}, {74, 0}, {74, 1}},
		Encoding: "bipolar127", Note: "",
	},
	{
		Name: "Mod 2 Destination", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{74, 2}, {74, 3}, {74, 4}, {74, 5}, {74, 6}, {74, 7}},
		Encoding: "modDest", Note: "",
	},
	{
		Name: "Mod 3 Source", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{75, 0}, {75, 1}, {75, 2}, {75, 3}, {75, 4}},
		Encoding: "modSource", Note: "",
	},
	{
		Name: "Mod 3 Amount", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{75, 5}, {75, 6}, {75, 7}, {76, 0}, {76, 1}, {76, 2}, {76, 3}, {76, 4}},
		Encoding: "bipolar127", Note: "",
	},
	{
		Name: "Mod 3 Destination", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{76, 5}, {76, 6}, {76, 7}, {77, 0}, {77, 1}, {77, 2}},
		Encoding: "modDest", Note: "",
	},
	{
		Name: "Mod 4 Source", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{77, 3}, {77, 4}, {77, 5}, {77, 6}, {77, 7}},
		Encoding: "modSource", Note: "",
	},
	{
		Name: "Mod 4 Amount", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{78, 0}, {78, 1}, {78, 2}, {78, 3}, {78, 4}, {78, 5}, {78, 6}, {78, 7}},
		Encoding: "bipolar127", Note: "",
	},
	{
		Name: "Mod 4 Destination", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{79, 0}, {79, 1}, {79, 2}, {79, 3}, {79, 4}, {79, 5}},
		Encoding: "modDest", Note: "",
	},
	{
		Name: "Mod 5 Source", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{79, 6}, {79, 7}, {80, 0}, {80, 1}, {80, 2}},
		Encoding: "modSource", Note: "",
	},
	{
		Name: "Mod 5 Amount", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{80, 3}, {80, 4}, {80, 5}, {80, 6}, {80, 7}, {81, 0}, {81, 1}, {81, 2}},
		Encoding: "bipolar127", Note: "",
	},
	{
		Name: "Mod 5 Destination", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{81, 3}, {81, 4}, {81, 5}, {81, 6}, {81, 7}, {82, 0}},
		Encoding: "modDest", Note: "",
	},
	{
		Name: "Mod 6 Source", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{82, 1}, {82, 2}, {82, 3}, {82, 4}, {82, 5}},
		Encoding: "modSource", Note: "",
	},
	{
		Name: "Mod 6 Amount", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{82, 6}, {82, 7}, {83, 0}, {83, 1}, {83, 2}, {83, 3}, {83, 4}, {83, 5}},
		Encoding: "bipolar127", Note: "",
	},
	{
		Name: "Mod 6 Destination", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{83, 6}, {83, 7}, {84, 0}, {84, 1}, {84, 2}, {84, 3}},
		Encoding: "modDest", Note: "",
	},
	{
		Name: "Mod 7 Source", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{84, 4}, {84, 5}, {84, 6}, {84, 7}, {85, 0}},
		Encoding: "modSource", Note: "",
	},
	{
		Name: "Mod 7 Amount", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{85, 1}, {85, 2}, {85, 3}, {85, 4}, {85, 5}, {85, 6}, {85, 7}, {86, 0}},
		Encoding: "bipolar127", Note: "",
	},
	{
		Name: "Mod 7 Destination", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{86, 1}, {86, 2}, {86, 3}, {86, 4}, {86, 5}, {86, 6}},
		Encoding: "modDest", Note: "",
	},
	{
		Name: "Mod 8 Source", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{86, 7}, {87, 0}, {87, 1}, {87, 2}, {87, 3}},
		Encoding: "modSource", Note: "",
	},
	{
		Name: "Mod 8 Amount", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{87, 4}, {87, 5}, {87, 6}, {87, 7}, {88, 0}, {88, 1}, {88, 2}, {88, 3}},
		Encoding: "bipolar127", Note: "",
	},
	{
		Name: "Mod 8 Destination", Section: "Mod Matrix",
		Bits:     []SoundParamBit{{88, 4}, {88, 5}, {88, 6}, {88, 7}, {89, 0}, {89, 1}},
		Encoding: "modDest", Note: "",
	},
}

// ModSourceNames maps a raw 5-bit Mod Source value (0-22) to its display name.
var ModSourceNames = map[int]string{
	0:  "Off",
	1:  "Pitch Envelope",
	2:  "Filter Envelope",
	3:  "Amp Envelope",
	4:  "Aux 1 Envelope",
	5:  "Aux 2 Envelope",
	6:  "LFO 1",
	7:  "LFO 2",
	8:  "Velocity",
	9:  "Note Number",
	10: "Noise",
	11: "Random",
	12: "Pad Pressure",
	13: "Slider Position 1",
	14: "Slider Position 2",
	15: "Slider Pressure 1",
	16: "Slider Pressure 2",
	17: "Foot Pedal 1",
	18: "Foot Pedal 2",
	19: "MIDI Pitch Bend",
	20: "MIDI Mod Wheel",
	21: "MIDI Breath",
	22: "MIDI Expression",
}

// ModDestNames maps a raw 6-bit Mod/Envelope/LFO Destination value (0-57) to
// its display name. Shared by the mod matrix, envelope destinations, and LFO
// destinations.
var ModDestNames = map[int]string{
	0:  "Off",
	1:  "Osc 1 Frequency",
	2:  "Osc 2 Frequency",
	3:  "Osc 3 Frequency",
	4:  "Osc 4 Frequency",
	5:  "Osc All Frequency",
	6:  "Osc 1/2 Mix",
	7:  "Osc 3 Level",
	8:  "Osc 4 Level",
	9:  "Osc 1 Pulsewidth",
	10: "Osc 2 Pulsewidth",
	11: "Osc 1/2 Pulsewidth",
	12: "Sub Osc Volume",
	13: "Feedback Volume",
	14: "Lowpass Filter",
	15: "Resonance",
	16: "Filter FM",
	17: "Highpass Filter",
	18: "VCA Level",
	19: "Pan",
	20: "LFO 1 Frequency",
	21: "LFO 2 Frequency",
	22: "LFO All Frequency",
	23: "LFO 1 Amount",
	24: "LFO 2 Amount",
	25: "LFO All Amount",
	26: "Pitch Env Amount",
	27: "Filter Env Amount",
	28: "Amp Env Amount",
	29: "Aux 1 Env Amount",
	30: "Aux 2 Env Amount",
	31: "All Env Amount",
	32: "Pitch Env Attack",
	33: "Filter Env Attack",
	34: "Amp Env Attack",
	35: "Aux 1 Env Attack",
	36: "Aux 2 Env Attack",
	37: "All Env Attack",
	38: "Pitch Env Decay",
	39: "Filter Env Decay",
	40: "Amp Env Decay",
	41: "Aux 1 Env Decay",
	42: "Aux 2 Env Decay",
	43: "All Env Decay",
	44: "Pitch Env Release",
	45: "Filter Env Release",
	46: "Amp Env Release",
	47: "Aux 1 Env Release",
	48: "Aux 2 Env Release",
	49: "All Env Release",
	50: "Mod 1 Amount",
	51: "Mod 2 Amount",
	52: "Mod 3 Amount",
	53: "Mod 4 Amount",
	54: "Mod 5 Amount",
	55: "Mod 6 Amount",
	56: "Mod 7 Amount",
	57: "Mod 8 Amount",
}

// ReadSoundParam extracts a parameter's raw integer value (LSB-first across
// p.Bits) from an unpacked Sound (0x60) parameter block.
func ReadSoundParam(params []byte, p SoundParam) int {
	val := 0
	for i, b := range p.Bits {
		if b.Byte < 0 || b.Byte >= len(params) {
			continue
		}
		bit := (params[b.Byte] >> uint(b.Bit)) & 1
		val |= int(bit) << uint(i)
	}
	return val
}

// DisplaySoundParam converts a raw extracted value to its display form per
// p.Encoding. ok is false for "modSource"/"modDest" values with no matching
// name (falls back to the plain integer).
func DisplaySoundParam(p SoundParam, raw int) (display string, numeric float64, ok bool) {
	switch p.Encoding {
	case "offset64":
		return "", float64(raw - 64), true
	case "offset50":
		return "", float64(raw - 50), true
	case "bipolar127":
		return "", float64(raw - 127), true
	case "modSource":
		if name, found := ModSourceNames[raw]; found {
			return name, float64(raw), true
		}
	case "modDest":
		if name, found := ModDestNames[raw]; found {
			return name, float64(raw), true
		}
	}
	return "", float64(raw), false
}

// DecodeSoundParams reads every known parameter from an unpacked Sound (0x60)
// parameter block, returning name -> raw integer value.
func DecodeSoundParams(params []byte) map[string]int {
	out := make(map[string]int, len(SoundParams))
	for _, p := range SoundParams {
		out[p.Name] = ReadSoundParam(params, p)
	}
	return out
}
