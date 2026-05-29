// Package server internal tests cover the unexported helper functions that
// cannot be reached through the exported API.
//
// Subtests share no global state — t.Parallel() is safe here.
package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// makeReq builds a CallToolRequest whose arguments are populated from args.
func makeReq(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

// TestClampUint7 verifies MIDI byte clamping to [0, 127].
func TestClampUint7(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		v    int
		want uint8
	}{
		{"zero", 0, 0},
		{"midpoint", 64, 64},
		{"max MIDI value", 127, 127},
		{"one above max", 128, 127},
		{"well above max", 255, 127},
		{"large positive", 10_000, 127},
		{"negative one", -1, 0},
		{"large negative", -10_000, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := clampUint7(tt.v)
			if got != tt.want {
				t.Errorf("clampUint7(%d) = %d, want %d", tt.v, got, tt.want)
			}
		})
	}
}

// TestClampDuration verifies duration clamping with default and ceiling values.
func TestClampDuration(t *testing.T) {
	t.Parallel()
	const def, max = 50, 30_000

	tests := []struct {
		name string
		v    int
		want int
	}{
		{"zero uses default", 0, def},
		{"negative uses default", -1, def},
		{"large negative uses default", -999, def},
		{"one passes through", 1, 1},
		{"typical duration", 100, 100},
		{"at ceiling", max, max},
		{"one above ceiling clamped", max + 1, max},
		{"well above ceiling clamped", 999_999, max},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := clampDuration(tt.v, def, max)
			if got != tt.want {
				t.Errorf("clampDuration(%d, %d, %d) = %d, want %d",
					tt.v, def, max, got, tt.want)
			}
		})
	}
}

// TestSanitizePath verifies path cleaning, tilde expansion, and rejection of
// relative paths.
func TestSanitizePath(t *testing.T) {
	t.Parallel()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() error = %v", err)
	}

	tests := []struct {
		name    string
		input   string
		want    string // empty means check wantErr
		wantErr string // substring expected in error
	}{
		{
			name:    "empty path returns error",
			input:   "",
			wantErr: "path is required",
		},
		{
			name:  "absolute path returned as cleaned",
			input: "/tmp/test.syx",
			want:  "/tmp/test.syx",
		},
		{
			name:  "tilde slash expands to home sub-path",
			input: "~/Tempest/kick.syx",
			want:  filepath.Join(home, "Tempest/kick.syx"),
		},
		{
			name:  "bare tilde expands to home",
			input: "~",
			want:  home,
		},
		{
			name:    "relative path rejected",
			input:   "relative/path.syx",
			wantErr: "must be absolute",
		},
		{
			name:    "dot-dot relative traversal rejected",
			input:   "../../etc/passwd",
			wantErr: "must be absolute",
		},
		{
			// An absolute path with embedded .. is cleaned but remains absolute.
			name:  "absolute path with embedded dot-dot cleaned",
			input: "/tmp/../tmp/test.syx",
			want:  "/tmp/test.syx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := sanitizePath(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("sanitizePath(%q) error = nil, want to contain %q",
						tt.input, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("sanitizePath(%q) unexpected error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestShortID verifies that shortID truncates to at most 8 characters.
func TestShortID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		id   string
		want string
	}{
		{"empty string", "", ""},
		{"three chars", "abc", "abc"},
		{"exactly 8 chars", "abcd1234", "abcd1234"},
		{"nine chars truncated", "abcd12345", "abcd1234"},
		{"long hex fingerprint", "deadbeefcafebabe0000", "deadbeef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shortID(tt.id)
			if got != tt.want {
				t.Errorf("shortID(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

// TestStrArg verifies extraction and whitespace trimming of string arguments.
func TestStrArg(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args map[string]any
		key  string
		want string
	}{
		{
			name: "present string returned trimmed",
			args: map[string]any{"k": "  hello  "},
			key:  "k",
			want: "hello",
		},
		{
			name: "present empty string returned as empty",
			args: map[string]any{"k": ""},
			key:  "k",
			want: "",
		},
		{
			name: "missing key returns empty string",
			args: map[string]any{"other": "value"},
			key:  "k",
			want: "",
		},
		{
			name: "non-string value returns empty string",
			args: map[string]any{"k": 42.0},
			key:  "k",
			want: "",
		},
		{
			name: "nil args map returns empty string",
			args: nil,
			key:  "k",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := strArg(makeReq(tt.args), tt.key)
			if got != tt.want {
				t.Errorf("strArg(key=%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

// TestIntArg verifies integer argument extraction with default fallback.
func TestIntArg(t *testing.T) {
	t.Parallel()
	const def = 99

	tests := []struct {
		name string
		args map[string]any
		key  string
		want int
	}{
		{
			// JSON numbers decode as float64; intArg handles both types.
			name: "float64 value converted to int",
			args: map[string]any{"k": float64(42)},
			key:  "k",
			want: 42,
		},
		{
			name: "int value passed through",
			args: map[string]any{"k": int(7)},
			key:  "k",
			want: 7,
		},
		{
			name: "missing key returns default",
			args: map[string]any{"other": float64(1)},
			key:  "k",
			want: def,
		},
		{
			name: "string value returns default",
			args: map[string]any{"k": "bad"},
			key:  "k",
			want: def,
		},
		{
			name: "nil args returns default",
			args: nil,
			key:  "k",
			want: def,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := intArg(makeReq(tt.args), tt.key, def)
			if got != tt.want {
				t.Errorf("intArg(key=%q) = %d, want %d", tt.key, got, tt.want)
			}
		})
	}
}

// TestFloatArg verifies float64 argument extraction with default fallback.
func TestFloatArg(t *testing.T) {
	t.Parallel()
	const def = 120.0

	tests := []struct {
		name string
		args map[string]any
		key  string
		want float64
	}{
		{
			name: "float64 value returned as-is",
			args: map[string]any{"k": 98.6},
			key:  "k",
			want: 98.6,
		},
		{
			name: "missing key returns default",
			args: map[string]any{"other": 1.0},
			key:  "k",
			want: def,
		},
		{
			name: "non-float value returns default",
			args: map[string]any{"k": "bad"},
			key:  "k",
			want: def,
		},
		{
			name: "nil args returns default",
			args: nil,
			key:  "k",
			want: def,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := floatArg(makeReq(tt.args), tt.key, def)
			if got != tt.want {
				t.Errorf("floatArg(key=%q) = %g, want %g", tt.key, got, tt.want)
			}
		})
	}
}
