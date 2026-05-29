package pattern_test

import (
	"testing"

	"tempest-mcp/internal/pattern"
)

func TestStepGridFromString_valid(t *testing.T) {
	steps, err := pattern.StepGridFromString("x...x...", 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 8 {
		t.Fatalf("got %d steps, want 8", len(steps))
	}
	if !steps[0].Gate || steps[0].Velocity != 100 {
		t.Errorf("steps[0] = %+v, want gate=true vel=100", steps[0])
	}
	if steps[1].Gate {
		t.Errorf("steps[1].Gate = true, want false (rest)")
	}
	if !steps[4].Gate {
		t.Errorf("steps[4].Gate = false, want true")
	}
}

func TestStepGridFromString_velocityDigit(t *testing.T) {
	steps, err := pattern.StepGridFromString("5...", 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !steps[0].Gate {
		t.Error("steps[0].Gate = false, want true")
	}
	const want = 5 * 14 // 70
	if steps[0].Velocity != want {
		t.Errorf("steps[0].Velocity = %d, want %d", steps[0].Velocity, want)
	}
}

func TestStepGridFromString_allDigits(t *testing.T) {
	steps, err := pattern.StepGridFromString("123456789", 9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < 9; i++ {
		wantVel := uint8((i + 1) * 14)
		if !steps[i].Gate || steps[i].Velocity != wantVel {
			t.Errorf("steps[%d] = %+v, want gate=true vel=%d", i, steps[i], wantVel)
		}
	}
}

func TestStepGridFromString_unknownChar(t *testing.T) {
	_, err := pattern.StepGridFromString("x.@.", 4)
	if err == nil {
		t.Fatal("expected error for unknown char '@', got nil")
	}
}

func TestStepGridFromString_barSeparatorIgnored(t *testing.T) {
	steps, err := pattern.StepGridFromString("x...|x...", 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 8 {
		t.Fatalf("got %d steps, want 8", len(steps))
	}
	// '|' is skipped; index 4 is the second 'x'
	if !steps[4].Gate {
		t.Errorf("steps[4].Gate = false, want true (after bar separator)")
	}
}

func TestStepGridFromString_truncate(t *testing.T) {
	steps, err := pattern.StepGridFromString("xxxxxxxx", 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 4 {
		t.Errorf("got %d steps, want 4 (truncated to stepCount)", len(steps))
	}
}

func TestStepGridFromString_padToStepCount(t *testing.T) {
	steps, err := pattern.StepGridFromString("x.", 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 8 {
		t.Errorf("got %d steps, want 8 (padded)", len(steps))
	}
	for i := 2; i < 8; i++ {
		if steps[i].Gate {
			t.Errorf("steps[%d].Gate = true, want false (padded rest)", i)
		}
	}
}

func TestStepGridFromString_empty(t *testing.T) {
	steps, err := pattern.StepGridFromString("", 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 4 {
		t.Errorf("got %d steps, want 4 (all padded rests)", len(steps))
	}
	for i, s := range steps {
		if s.Gate {
			t.Errorf("steps[%d].Gate = true, want false", i)
		}
	}
}

func TestStepGridFromString_zeroStepCount(t *testing.T) {
	steps, err := pattern.StepGridFromString("xxxx", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 0 {
		t.Errorf("got %d steps, want 0", len(steps))
	}
}
