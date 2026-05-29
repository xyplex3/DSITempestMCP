package mapper_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tempest-mcp/cmd/beat-mapper/mapper"
	"tempest-mcp/internal/sysex"
)

// TestUnescape_roundtrip verifies unescape(escape(data)) == data using the sysex package.
func TestUnescape_roundtrip(t *testing.T) {
	data := make([]byte, 56) // 8 full 7+1 groups
	for i := range data {
		data[i] = byte(i % 127)
	}
	escaped := sysex.Escape7Plus1(data)
	got := sysex.Unescape7Plus1(escaped)
	if len(got) < len(data) {
		t.Fatalf("round-trip length: got %d want %d", len(got), len(data))
	}
	for i, b := range data {
		if got[i] != b {
			t.Errorf("byte %d: got 0x%02X want 0x%02X", i, got[i], b)
		}
	}
}

// TestDiff_identical verifies that two identical payloads produce zero diff entries.
func TestDiff_identical(t *testing.T) {
	payload := []byte{0x01, 0x02, 0x03, 0x04}
	entries, err := mapper.Diff(payload, payload, "same")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d diff entries, want 0", len(entries))
	}
}

// TestDiff_singleByte verifies that one changed byte is reported at the correct offset.
func TestDiff_singleByte(t *testing.T) {
	baseline := []byte{0x00, 0x00, 0x00, 0x00}
	changed := []byte{0x00, 0x64, 0x00, 0x00}
	entries, err := mapper.Diff(baseline, changed, "test-label")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.Offset != 1 {
		t.Errorf("Offset = %d, want 1", e.Offset)
	}
	if e.Baseline != 0x00 || e.Changed != 0x64 {
		t.Errorf("values: baseline=0x%02X changed=0x%02X, want 0x00/0x64", e.Baseline, e.Changed)
	}
	if e.Delta != 100 {
		t.Errorf("Delta = %d, want 100", e.Delta)
	}
	if e.Label != "test-label" {
		t.Errorf("Label = %q, want %q", e.Label, "test-label")
	}
}

// TestDiff_lengthMismatch verifies that diffing payloads of different length returns an error.
func TestDiff_lengthMismatch(t *testing.T) {
	_, err := mapper.Diff([]byte{0x01, 0x02}, []byte{0x01}, "")
	if err == nil {
		t.Fatal("expected error for length mismatch, got nil")
	}
}

// TestInferStride_twoOffsets verifies that offsets 0x1A3 and 0x1C3 produce stride 0x20.
func TestInferStride_twoOffsets(t *testing.T) {
	offsets := []int{0x1A3, 0x1C3}
	got := mapper.InferStride(offsets)
	if got != 0x20 {
		t.Errorf("InferStride = 0x%02X, want 0x%02X", got, 0x20)
	}
}

// TestGenerateConsts_smoke verifies the const block contains the expected identifiers.
func TestGenerateConsts_smoke(t *testing.T) {
	out := mapper.GenerateConsts(0x50, 0x20, 0x02)
	for _, want := range []string{"BeatDataOffset", "TrackStride", "StepStride"} {
		if !strings.Contains(out, want) {
			t.Errorf("GenerateConsts output missing %q\nGot:\n%s", want, out)
		}
	}
}

// TestAnnotate verifies hex-dump output format and label placement.
func TestAnnotate(t *testing.T) {
	tests := []struct {
		name        string
		payload     []byte
		annotations map[string]string
		wantContain string
		wantEmpty   bool
	}{
		{
			name:      "nil payload returns empty string",
			payload:   nil,
			wantEmpty: true,
		},
		{
			name:        "16-byte payload produces address prefix",
			payload:     make([]byte, 16),
			annotations: nil,
			wantContain: "0x0000:",
		},
		{
			name:        "annotation label appears in output",
			payload:     make([]byte, 16),
			annotations: map[string]string{"0x0002": "cutoff"},
			wantContain: "cutoff",
		},
		{
			name:        "decimal annotation key is parsed",
			payload:     make([]byte, 16),
			annotations: map[string]string{"3": "decay"},
			wantContain: "decay",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapper.Annotate(tt.payload, tt.annotations)
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("Annotate() = %q, want empty string", got)
				}
				return
			}
			if !strings.Contains(got, tt.wantContain) {
				t.Errorf("Annotate() missing %q:\n%s", tt.wantContain, got)
			}
		})
	}
}

// TestParseSyx verifies reading and splitting .syx files into messages.
func TestParseSyx(t *testing.T) {
	t.Run("valid file returns messages", func(t *testing.T) {
		dir := t.TempDir()
		msg := sysex.BuildFLASHDump("Snare", make([]byte, sysex.ParamBlockSizeFLASH), 0)
		path := filepath.Join(dir, "snare.syx")
		if err := os.WriteFile(path, msg, 0o644); err != nil {
			t.Fatalf("writing syx: %v", err)
		}
		msgs, err := mapper.ParseSyx(path)
		if err != nil {
			t.Fatalf("ParseSyx() error = %v", err)
		}
		if len(msgs) != 1 {
			t.Errorf("messages = %d, want 1", len(msgs))
		}
	})

	t.Run("missing file returns error containing 'reading'", func(t *testing.T) {
		_, err := mapper.ParseSyx("/nonexistent/path/test.syx")
		if err == nil {
			t.Fatal("expected error for missing file, got nil")
		}
		if !strings.Contains(err.Error(), "reading") {
			t.Errorf("error = %q, want to contain 'reading'", err.Error())
		}
	})
}

// TestUnescapeProject verifies UnescapeProject recognises Tempest messages.
func TestUnescapeProject(t *testing.T) {
	t.Run("FLASH file returns result with correct type", func(t *testing.T) {
		dir := t.TempDir()
		msg := sysex.BuildFLASHDump("HiHat", make([]byte, sysex.ParamBlockSizeFLASH), 0)
		path := filepath.Join(dir, "hihat.syx")
		if err := os.WriteFile(path, msg, 0o644); err != nil {
			t.Fatalf("writing syx: %v", err)
		}
		r, err := mapper.UnescapeProject(path)
		if err != nil {
			t.Fatalf("UnescapeProject() error = %v", err)
		}
		if r.MsgType != sysex.TypeFLASHSound {
			t.Errorf("MsgType = %v, want TypeFLASHSound", r.MsgType)
		}
		if len(r.Payload) == 0 {
			t.Error("Payload is empty")
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		_, err := mapper.UnescapeProject("/nonexistent/path/test.syx")
		if err == nil {
			t.Fatal("expected error for missing file, got nil")
		}
	})

	t.Run("no recognised SysEx returns error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "noise.syx")
		// Non-Tempest SysEx (universal identity request)
		data := []byte{0xF0, 0x7E, 0x00, 0x06, 0x01, 0xF7}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("writing syx: %v", err)
		}
		_, err := mapper.UnescapeProject(path)
		if err == nil {
			t.Fatal("expected error for unrecognised SysEx, got nil")
		}
		if !strings.Contains(err.Error(), "no recognised") {
			t.Errorf("error = %q, want to contain 'no recognised'", err.Error())
		}
	})
}
