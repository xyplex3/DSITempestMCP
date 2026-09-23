// White-box tests for beat-mapper's unexported session-inference helpers.
// package main has no exported caller path, so these live alongside the
// source rather than in a _test package.
package main

import "testing"

// TestParseCaptureName verifies bank/track/step extraction from capture
// filenames such as "kick_a1_s1" or "snare_b2_s3".
func TestParseCaptureName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantBank  int
		wantTrack int
		wantStep  int
		wantOK    bool
	}{
		{name: "bank A track and step", input: "kick_a1_s1", wantBank: 0, wantTrack: 1, wantStep: 1, wantOK: true},
		{name: "bank B track and step", input: "snare_b2_s3", wantBank: 1, wantTrack: 2, wantStep: 3, wantOK: true},
		{name: "uppercase normalised", input: "KICK_A1_S1", wantBank: 0, wantTrack: 1, wantStep: 1, wantOK: true},
		{name: "no recognisable pattern", input: "no_pattern_here", wantOK: false},
		{name: "missing step", input: "kick_a1", wantOK: false},
		{name: "missing track", input: "just_s5", wantOK: false},
		{
			name: "second track marker overwrites the first",
			// Matches parseCaptureName's loop, which keeps reassigning
			// bank/track on every a/b token rather than keeping the first.
			input: "a1_b2_s3", wantBank: 1, wantTrack: 2, wantStep: 3, wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bank, track, step, ok := parseCaptureName(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("parseCaptureName(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if bank != tt.wantBank || track != tt.wantTrack || step != tt.wantStep {
				t.Errorf("parseCaptureName(%q) = (bank=%d, track=%d, step=%d), want (bank=%d, track=%d, step=%d)",
					tt.input, bank, track, step, tt.wantBank, tt.wantTrack, tt.wantStep)
			}
		})
	}
}

// TestBuildCaptureIndex verifies that only filenames matching the
// bank/track/step pattern are indexed, keyed by their parsed identity.
func TestBuildCaptureIndex(t *testing.T) {
	captures := []sessionCapture{
		{name: "kick_a1_s1", primaryOffset: 0x10},
		{name: "not_a_pattern", primaryOffset: 0x20},
		{name: "snare_b2_s3", primaryOffset: 0x30},
	}

	byKey := buildCaptureIndex(captures)

	if len(byKey) != 2 {
		t.Fatalf("buildCaptureIndex() len = %d, want 2", len(byKey))
	}
	if c, ok := byKey[captureKey{bank: 0, track: 1, step: 1}]; !ok || c.name != "kick_a1_s1" {
		t.Errorf("missing or wrong entry for kick_a1_s1: %+v", c)
	}
	if c, ok := byKey[captureKey{bank: 1, track: 2, step: 3}]; !ok || c.name != "snare_b2_s3" {
		t.Errorf("missing or wrong entry for snare_b2_s3: %+v", c)
	}
}

// TestComputeStrides covers unambiguous cases only (at most one candidate
// pair per stride), since computeStrides picks its first match via Go map
// iteration order, which is randomized when more than one candidate exists.
func TestComputeStrides(t *testing.T) {
	t.Run("empty index", func(t *testing.T) {
		stepStride, trackStride := computeStrides(map[captureKey]*sessionCapture{})
		if stepStride != -1 || trackStride != -1 {
			t.Errorf("computeStrides(empty) = (%d, %d), want (-1, -1)", stepStride, trackStride)
		}
	})

	t.Run("single step pair, no track pair", func(t *testing.T) {
		byKey := map[captureKey]*sessionCapture{
			{bank: 0, track: 1, step: 1}: {name: "s1", primaryOffset: 0x10},
			{bank: 0, track: 1, step: 2}: {name: "s2", primaryOffset: 0x20},
		}
		stepStride, trackStride := computeStrides(byKey)
		if stepStride != 0x10 {
			t.Errorf("stepStride = 0x%X, want 0x10", stepStride)
		}
		if trackStride != -1 {
			t.Errorf("trackStride = %d, want -1 (no track+1 pair present)", trackStride)
		}
	})

	t.Run("single track pair, no step pair", func(t *testing.T) {
		byKey := map[captureKey]*sessionCapture{
			{bank: 0, track: 1, step: 1}: {name: "t1", primaryOffset: 0x100},
			{bank: 0, track: 2, step: 1}: {name: "t2", primaryOffset: 0x180},
		}
		stepStride, trackStride := computeStrides(byKey)
		if trackStride != 0x80 {
			t.Errorf("trackStride = 0x%X, want 0x80", trackStride)
		}
		if stepStride != -1 {
			t.Errorf("stepStride = %d, want -1 (no step+1 pair present)", stepStride)
		}
	})
}
