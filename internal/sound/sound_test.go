package sound_test

import (
	"testing"

	"tempest-mcp/internal/sound"
)

func TestMorph_halfway(t *testing.T) {
	a := []byte{0, 100}
	b := []byte{100, 0}
	got := sound.Morph(a, b, 0.5)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0] != 50 || got[1] != 50 {
		t.Errorf("Morph(0.5) = %v, want [50 50]", got)
	}
}

func TestMorph_fullA(t *testing.T) {
	a := []byte{10, 20, 30}
	b := []byte{90, 80, 70}
	got := sound.Morph(a, b, 0.0)
	for i, want := range a {
		if got[i] != want {
			t.Errorf("got[%d] = %d, want %d", i, got[i], want)
		}
	}
}

func TestMorph_fullB(t *testing.T) {
	a := []byte{10, 20, 30}
	b := []byte{90, 80, 70}
	got := sound.Morph(a, b, 1.0)
	for i, want := range b {
		if got[i] != want {
			t.Errorf("got[%d] = %d, want %d", i, got[i], want)
		}
	}
}

func TestMorph_clampTo127(t *testing.T) {
	a := []byte{127, 127}
	b := []byte{127, 127}
	got := sound.Morph(a, b, 0.5)
	for i, v := range got {
		if v > 127 {
			t.Errorf("got[%d] = %d, exceeds 127", i, v)
		}
	}
}

func TestMorph_shortestLength(t *testing.T) {
	a := []byte{1, 2, 3}
	b := []byte{4, 5}
	got := sound.Morph(a, b, 0.5)
	if len(got) != 2 {
		t.Errorf("len = %d, want 2 (shorter input)", len(got))
	}
}

func TestMorph_bothNil(t *testing.T) {
	got := sound.Morph(nil, nil, 0.5)
	if got != nil {
		t.Errorf("Morph(nil, nil) = %v, want nil", got)
	}
}

func TestMorph_amountClamped(t *testing.T) {
	a := []byte{0}
	b := []byte{100}
	// amount > 1 should clamp to 1
	got := sound.Morph(a, b, 2.0)
	if got[0] != 100 {
		t.Errorf("Morph(2.0) = %d, want 100 (clamped to 1.0)", got[0])
	}
	// amount < 0 should clamp to 0
	got = sound.Morph(a, b, -1.0)
	if got[0] != 0 {
		t.Errorf("Morph(-1.0) = %d, want 0 (clamped to 0.0)", got[0])
	}
}

func TestDefaultBlankParams_size(t *testing.T) {
	p := sound.DefaultBlankParams()
	if len(p) != sound.ParamBlockSize {
		t.Errorf("len = %d, want %d", len(p), sound.ParamBlockSize)
	}
}

func TestDefaultBlankParams_signature(t *testing.T) {
	p := sound.DefaultBlankParams()
	want := []byte{
		0x24, 0x19, 0x00, 0x10, 0x49, 0x06, 0x00, 0x04,
		0x14, 0x1d, 0x00, 0x20, 0x50, 0x36, 0x23, 0x00,
	}
	for i, b := range want {
		if p[i] != b {
			t.Errorf("signature[%d] = 0x%02X, want 0x%02X", i, p[i], b)
		}
	}
}

func TestDefaultBlankParams_restZero(t *testing.T) {
	p := sound.DefaultBlankParams()
	for i := 16; i < len(p); i++ {
		if p[i] != 0 {
			t.Errorf("p[%d] = 0x%02X, want 0 (blank region)", i, p[i])
		}
	}
}
