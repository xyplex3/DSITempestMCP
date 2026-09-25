// Package server wires all MCP tools to their MIDI/library handlers.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"tempest-mcp/internal/config"
	"tempest-mcp/internal/library"
	"tempest-mcp/internal/midi"
	"tempest-mcp/internal/sound"
	"tempest-mcp/internal/sysex"
)

// Server bundles the MCP server, MIDI device, library index and config.
type Server struct {
	cfg    *config.Config
	device *midi.Device
	lib    *library.Index
	libMu  sync.RWMutex
	log    *zap.SugaredLogger
	mcp    *mcpserver.MCPServer
}

// New creates a Server and registers all tools.
func New(cfg *config.Config, log *zap.SugaredLogger) *Server {
	s := &Server{
		cfg: cfg,
		log: log,
		device: midi.New(midi.DeviceConfig{
			DeviceName:      cfg.MIDI.DeviceName,
			Channel:         cfg.MIDI.Channel,
			MIDITrace:       cfg.Log.MIDITrace,
			SysExBufferSize: uint32(cfg.SysEx.BufferBytes),
		}),
	}

	s.mcp = mcpserver.NewMCPServer(
		"Tempest MCP Server",
		"1.0.0",
	)

	s.registerTransportTools()
	s.registerTriggerTools()
	s.registerCCTools()
	s.registerLibraryTools()
	s.registerSysExTools()
	s.registerSoundTools()
	s.registerUtilityTools()

	return s
}

// ServeStdio starts the MCP server on stdin/stdout.
func (s *Server) ServeStdio() error {
	s.log.Info("tempest-mcp starting on stdio")
	if err := s.connectDevice(); err != nil {
		s.log.Warnf("Initial device connect failed (will retry per tool call): %v", err)
	}
	if err := s.loadLibrary(); err != nil {
		s.log.Warnf("Library index load failed: %v", err)
	}
	return mcpserver.ServeStdio(s.mcp)
}

// ── Private helpers ───────────────────────────────────────────────────────────

func (s *Server) connectDevice() error {
	if s.device.IsConnected() {
		return nil
	}
	return s.device.Connect()
}

func (s *Server) requireDevice() error {
	if s.device.IsConnected() {
		return nil
	}
	if err := s.device.Connect(); err != nil {
		return fmt.Errorf("tempest not connected: %w — check USB and that the Tempest is powered on", err)
	}
	return nil
}

func (s *Server) loadLibrary() error {
	idx, err := library.LoadIndex(s.cfg.Library.IndexPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if idx != nil && len(idx.Sounds) > 0 {
		s.setLib(idx)
		s.log.Infof("Loaded library index: %d sounds", len(idx.Sounds))
		return nil
	}
	// No saved index — scan now
	return s.rescanLibrary()
}

func (s *Server) rescanLibrary() error {
	s.log.Infof("Scanning library at %s", s.cfg.Library.Path)
	idx, err := library.Scan(s.cfg.Library.Path)
	if err != nil {
		return fmt.Errorf("scanning library: %w", err)
	}
	s.setLib(idx)
	s.log.Infof("Indexed %d sounds", len(idx.Sounds))
	if err := library.SaveIndex(idx, s.cfg.Library.IndexPath); err != nil {
		s.log.Warnf("Failed to save library index: %v", err)
	}
	return nil
}

// getLib returns the current library index under a read lock.
func (s *Server) getLib() *library.Index {
	s.libMu.RLock()
	defer s.libMu.RUnlock()
	return s.lib
}

// setLib replaces the library index under a write lock.
func (s *Server) setLib(idx *library.Index) {
	s.libMu.Lock()
	s.lib = idx
	s.libMu.Unlock()
}

func ok(text string) *mcp.CallToolResult {
	return mcp.NewToolResultText(text)
}

func fail(err error) (*mcp.CallToolResult, error) {
	return nil, err
}

// shortID returns up to the first 8 characters of an ID for display.
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func strArg(req mcp.CallToolRequest, key string) string {
	args := req.GetArguments()
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func intArg(req mcp.CallToolRequest, key string, def int) int {
	args := req.GetArguments()
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return def
}

func floatArg(req mcp.CallToolRequest, key string, def float64) float64 {
	args := req.GetArguments()
	if v, ok := args[key]; ok {
		if n, ok := v.(float64); ok {
			return n
		}
	}
	return def
}

func boolArg(req mcp.CallToolRequest, key string, def bool) bool {
	args := req.GetArguments()
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

// clampUint7 clamps an int to the valid MIDI data byte range [0, 127].
func clampUint7(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 127 {
		return 127
	}
	return uint8(v)
}

// clampDuration clamps a duration in milliseconds to [1, maxVal], using defaultVal when v <= 0.
func clampDuration(v, defaultVal, maxVal int) int {
	if v <= 0 {
		return defaultVal
	}
	if v > maxVal {
		return maxVal
	}
	return v
}

// sanitizePath cleans a caller-supplied path, expands ~, and requires it to be absolute.
func sanitizePath(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("path is required")
	}
	if strings.HasPrefix(input, "~/") || input == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		input = filepath.Join(home, input[1:])
	}
	cleaned := filepath.Clean(input)
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("path must be absolute, got %q", input)
	}
	return cleaned, nil
}

// ── Transport Tools ───────────────────────────────────────────────────────────

func (s *Server) registerTransportTools() {
	s.mcp.AddTool(mcp.NewTool("tempest_start",
		mcp.WithDescription("Send MIDI Start to begin Tempest sequencer playback."),
	), func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := s.requireDevice(); err != nil {
			return fail(err)
		}
		if err := s.device.Start(); err != nil {
			return fail(err)
		}
		return ok("Sequencer started"), nil
	})

	s.mcp.AddTool(mcp.NewTool("tempest_stop",
		mcp.WithDescription("Send MIDI Stop and halt the internal clock."),
	), func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := s.requireDevice(); err != nil {
			return fail(err)
		}
		if err := s.device.Stop(); err != nil {
			return fail(err)
		}
		return ok("Sequencer stopped"), nil
	})

	s.mcp.AddTool(mcp.NewTool("tempest_continue",
		mcp.WithDescription("Send MIDI Continue to resume from the current position."),
	), func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := s.requireDevice(); err != nil {
			return fail(err)
		}
		if err := s.device.Continue(); err != nil {
			return fail(err)
		}
		return ok("Sequencer resumed"), nil
	})

	s.mcp.AddTool(mcp.NewTool("tempest_set_tempo",
		mcp.WithDescription("Start (or restart) the internal MIDI clock at the given BPM. Sends 24 PPQN clock pulses."),
		mcp.WithNumber("bpm", mcp.Required(), mcp.Description("Tempo in BPM (20–300)")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := s.requireDevice(); err != nil {
			return fail(err)
		}
		bpm := floatArg(req, "bpm", 120)
		if err := s.device.SetTempo(bpm); err != nil {
			return fail(err)
		}
		return ok(fmt.Sprintf("Clock running at %.1f BPM", bpm)), nil
	})
}

// ── Trigger Tools ─────────────────────────────────────────────────────────────

func (s *Server) registerTriggerTools() {
	s.mcp.AddTool(mcp.NewTool("tempest_trigger_pad",
		mcp.WithDescription("Trigger a named Tempest pad with a note-on/off. "+
			"Use names like: kick, snare, snare-2, closed-hat, open-hat, clap, ride, crash, high-tom, mid-tom, low-tom, side-stick, a1–a16."),
		mcp.WithString("pad", mcp.Required(), mcp.Description("Pad name e.g. 'kick', 'snare', 'closed-hat', 'a12'")),
		mcp.WithNumber("velocity", mcp.Description("Velocity 1–127 (default 100)")),
		mcp.WithNumber("duration_ms", mcp.Description("Note duration in milliseconds (default 50)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := s.requireDevice(); err != nil {
			return fail(err)
		}
		pad := strArg(req, "pad")
		vel := clampUint7(intArg(req, "velocity", 100))
		dur := clampDuration(intArg(req, "duration_ms", 50), 50, 30_000)
		if err := s.device.TriggerPad(ctx, pad, vel, dur); err != nil {
			return fail(err)
		}
		return ok(fmt.Sprintf("Triggered %s (vel=%d, dur=%dms)", pad, vel, dur)), nil
	})

	s.mcp.AddTool(mcp.NewTool("tempest_trigger_note",
		mcp.WithDescription("Trigger a raw MIDI note number on the Tempest drum channel."),
		mcp.WithNumber("note", mcp.Required(), mcp.Description("MIDI note number 0–127")),
		mcp.WithNumber("velocity", mcp.Description("Velocity 1–127 (default 100)")),
		mcp.WithNumber("duration_ms", mcp.Description("Note duration in ms (default 50)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := s.requireDevice(); err != nil {
			return fail(err)
		}
		note := clampUint7(intArg(req, "note", 36))
		vel := clampUint7(intArg(req, "velocity", 100))
		dur := clampDuration(intArg(req, "duration_ms", 50), 50, 30_000)
		if err := s.device.TriggerNote(ctx, note, vel, dur); err != nil {
			return fail(err)
		}
		return ok(fmt.Sprintf("Triggered note %d (vel=%d, dur=%dms)", note, vel, dur)), nil
	})

	s.mcp.AddTool(mcp.NewTool("tempest_play_sequence",
		mcp.WithDescription("Play a sequence of pad hits timed to a BPM. "+
			"Each event specifies a pad name, beat position (1=first sixteenth), and velocity. "+
			"Example events JSON: [{\"pad\":\"kick\",\"beat\":1,\"velocity\":110},{\"pad\":\"snare\",\"beat\":5,\"velocity\":100}]"),
		mcp.WithString("events", mcp.Required(), mcp.Description("JSON array of {pad, beat, velocity, duration_ms}")),
		mcp.WithNumber("bpm", mcp.Description("Tempo in BPM (default 120)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := s.requireDevice(); err != nil {
			return fail(err)
		}
		eventsJSON := strArg(req, "events")
		bpm := floatArg(req, "bpm", 120)

		var raw []struct {
			Pad        string `json:"pad"`
			Note       int    `json:"note"`
			Beat       int    `json:"beat"`
			Velocity   int    `json:"velocity"`
			DurationMS int    `json:"duration_ms"`
		}
		if err := json.Unmarshal([]byte(eventsJSON), &raw); err != nil {
			return fail(fmt.Errorf("invalid events JSON: %w", err))
		}

		events := make([]midi.SequenceEvent, len(raw))
		for i, r := range raw {
			events[i] = midi.SequenceEvent{
				Pad:        r.Pad,
				Note:       clampUint7(r.Note),
				Beat:       r.Beat,
				Velocity:   clampUint7(r.Velocity),
				DurationMS: clampDuration(r.DurationMS, 50, 30_000),
			}
		}
		if err := s.device.PlaySequence(ctx, events, bpm); err != nil {
			return fail(err)
		}
		return ok(fmt.Sprintf("Played %d-event sequence at %.1f BPM", len(events), bpm)), nil
	})
}

// ── CC / Beat FX Tools ────────────────────────────────────────────────────────

func (s *Server) registerCCTools() {
	s.mcp.AddTool(mcp.NewTool("tempest_set_cc",
		mcp.WithDescription("Send a MIDI CC to the Tempest. Only the 11 Beat FX CCs are supported: 12 (distortion), 13 (compression), 19 (reset-beat-fx), 20 (all-osc-freq), 21 (feedback), 22 (lp-cutoff), 23 (lp-resonance), 24 (lp-audio-mod), 25 (hp-cutoff), 26 (env-attack), 27 (env-decay)."),
		mcp.WithNumber("cc", mcp.Required(), mcp.Description("CC number (12, 13, or 19–27)")),
		mcp.WithNumber("value", mcp.Required(), mcp.Description("Value 0–127")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := s.requireDevice(); err != nil {
			return fail(err)
		}
		cc := clampUint7(intArg(req, "cc", 0))
		val := clampUint7(intArg(req, "value", 0))
		if err := s.device.SendCC(cc, val); err != nil {
			return fail(err)
		}
		return ok(fmt.Sprintf("Sent CC %d (%s) = %d", cc, midi.CCName(cc), val)), nil
	})

	s.mcp.AddTool(mcp.NewTool("tempest_set_beat_fx",
		mcp.WithDescription("Set a named Beat FX parameter. "+
			"Valid names: distortion, compression, reset-beat-fx, all-osc-freq, feedback, lp-cutoff, lp-resonance, lp-audio-mod, hp-cutoff, env-attack, env-decay."),
		mcp.WithString("param", mcp.Required(), mcp.Description("Beat FX parameter name")),
		mcp.WithNumber("value", mcp.Required(), mcp.Description("Value 0–127")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := s.requireDevice(); err != nil {
			return fail(err)
		}
		param := strArg(req, "param")
		val := clampUint7(intArg(req, "value", 0))
		if err := s.device.SetBeatFX(param, val); err != nil {
			return fail(err)
		}
		return ok(fmt.Sprintf("Set %s = %d", param, val)), nil
	})
}

// ── Library Tools ─────────────────────────────────────────────────────────────

func (s *Server) registerLibraryTools() {
	s.mcp.AddTool(mcp.NewTool("tempest_list_sounds",
		mcp.WithDescription("List sounds from the local Tempest library. Optional filter matches name or tags."),
		mcp.WithString("filter", mcp.Description("Optional substring filter (name or tag)")),
		mcp.WithNumber("limit", mcp.Description("Maximum results to return (default 50)")),
	), s.handleListSounds)

	s.mcp.AddTool(mcp.NewTool("tempest_search_sounds",
		mcp.WithDescription("Fuzzy search the local Tempest library by name, folder, or tag."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search terms e.g. 'analog kick' or 'snare'")),
		mcp.WithNumber("limit", mcp.Description("Max results (default 20)")),
	), s.handleSearchSounds)

	s.mcp.AddTool(mcp.NewTool("tempest_describe_sound",
		mcp.WithDescription("Return full metadata for a sound from the library by ID."),
		mcp.WithString("sound_id", mcp.Required(), mcp.Description("Sound ID (fingerprint prefix, at least 8 chars)")),
	), s.handleDescribeSound)

	s.mcp.AddTool(mcp.NewTool("tempest_load_sound",
		mcp.WithDescription("Send a sound from the local library to the Tempest over SysEx. "+
			"The user must have the Tempest powered on and SysEx IN set to USB. "+
			"A 1-second pause is inserted between messages as required by the Tempest. "+
			"Optionally record the intended bank (A or B) and slot (1–16) in the local "+
			"index — the Tempest does not accept a destination slot over SysEx, so select "+
			"the slot on the hardware's own Save/Load prompt when the dump arrives."),
		mcp.WithString("sound_id", mcp.Required(), mcp.Description("Sound ID or prefix from tempest_list_sounds / tempest_search_sounds")),
		mcp.WithString("bank", mcp.Description("Target bank: A or B (optional)")),
		mcp.WithNumber("slot", mcp.Description("Target slot 1–16 within the bank (optional)")),
	), s.handleLoadSound)

	s.mcp.AddTool(mcp.NewTool("tempest_index_library",
		mcp.WithDescription("Rescan the local sound library and rebuild the index. Run this after adding new .syx files."),
	), s.handleIndexLibrary)

	s.mcp.AddTool(mcp.NewTool("tempest_show_bank_map",
		mcp.WithDescription("Show which sounds are recorded as loaded into each hardware bank slot (Bank A and Bank B, slots 1–16). "+
			"This reflects the library index; use tempest_load_sound with bank/slot to update it."),
	), s.handleShowBankMap)
}

func (s *Server) handleListSounds(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	lib := s.getLib()
	if lib == nil {
		return fail(fmt.Errorf("library not loaded — call tempest_index_library first"))
	}
	filter := strArg(req, "filter")
	limit := intArg(req, "limit", 50)

	results := library.Search(lib, filter)
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	lines := make([]string, 0, len(results)+1)
	lines = append(lines, fmt.Sprintf("%d sounds found:", len(results)))
	for _, r := range results {
		tags := strings.Join(r.Sound.Tags, "/")
		lines = append(lines, fmt.Sprintf("  [%s] %s  (id: %s, tags: %s)", r.Sound.MsgType, r.Sound.Name, shortID(r.Sound.ID), tags))
	}
	return ok(strings.Join(lines, "\n")), nil
}

func (s *Server) handleSearchSounds(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	lib := s.getLib()
	if lib == nil {
		return fail(fmt.Errorf("library not loaded — call tempest_index_library first"))
	}
	q := strArg(req, "query")
	limit := intArg(req, "limit", 20)

	results := library.Search(lib, q)
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	if len(results) == 0 {
		return ok(fmt.Sprintf("No sounds found matching %q", q)), nil
	}

	lines := make([]string, 0, len(results)+1)
	lines = append(lines, fmt.Sprintf("%d results for %q:", len(results), q))
	for _, r := range results {
		lines = append(lines, fmt.Sprintf("  [score:%d] %s  (id: %s)", r.Score, r.Sound.Name, shortID(r.Sound.ID)))
	}
	return ok(strings.Join(lines, "\n")), nil
}

func (s *Server) handleDescribeSound(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	lib := s.getLib()
	if lib == nil {
		return fail(fmt.Errorf("library not loaded"))
	}
	id := strArg(req, "sound_id")
	snd := findSound(lib, id)
	if snd == nil {
		return fail(fmt.Errorf("sound %q not found in library", id))
	}
	data, err := json.MarshalIndent(snd, "", "  ")
	if err != nil {
		return fail(fmt.Errorf("serialising sound: %w", err))
	}
	return ok(string(data)), nil
}

func (s *Server) handleLoadSound(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.requireDevice(); err != nil {
		return fail(err)
	}
	lib := s.getLib()
	if lib == nil {
		return fail(fmt.Errorf("library not loaded"))
	}

	id := strArg(req, "sound_id")
	snd := findSound(lib, id)
	if snd == nil {
		return fail(fmt.Errorf("sound %q not found", id))
	}

	bank := strings.ToUpper(strArg(req, "bank"))
	slot := intArg(req, "slot", 0)

	if err := validateBankSlot(bank, slot); err != nil {
		return fail(err)
	}

	msgs, err := library.ReadSyxMessages(snd.Path)
	if err != nil {
		return fail(err)
	}

	if err := s.device.SendRawWithDelay(ctx, msgs, s.cfg.SysEx.InterMessageDelayMS); err != nil {
		return fail(fmt.Errorf("sending %s: %w", snd.Name, err))
	}

	result := fmt.Sprintf("Loaded %q to Tempest (%d SysEx messages sent)", snd.Name, len(msgs))

	// Record the bank assignment in the library index. NOTE: a FLASH SysEx
	// dump does not carry a destination bank/slot byte (see
	// docs/sysex-tempest-format.md §2) — the Tempest assigns the incoming
	// sound to a slot via its own front-panel Save/Load prompt. This only
	// tracks the intended assignment locally for tempest_show_bank_map.
	if bank != "" && slot > 0 {
		s.libMu.Lock()
		snd.BankSlot = &library.BankAssignment{
			Bank:     bank,
			Slot:     slot,
			LoadedAt: time.Now(),
		}
		s.libMu.Unlock()
		if err := library.SaveIndex(s.getLib(), s.cfg.Library.IndexPath); err != nil {
			s.log.Warnf("Failed to save library index: %v", err)
		}
		result += fmt.Sprintf(" → Bank %s Slot %d (select this slot on the Tempest's Save/Load prompt when it appears)", bank, slot)
	}

	return ok(result), nil
}

// validateBankSlot returns an error if bank or slot are set but invalid.
func validateBankSlot(bank string, slot int) error {
	if bank == "" && slot == 0 {
		return nil
	}
	if bank != "A" && bank != "B" {
		return fmt.Errorf("bank must be A or B, got %q", bank)
	}
	if slot < 1 || slot > 16 {
		return fmt.Errorf("slot must be 1–16, got %d", slot)
	}
	return nil
}

func (s *Server) handleIndexLibrary(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.rescanLibrary(); err != nil {
		return fail(err)
	}
	lib := s.getLib()
	return ok(fmt.Sprintf("Library indexed: %d sounds at %s", len(lib.Sounds), s.cfg.Library.Path)), nil
}

func (s *Server) handleShowBankMap(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.libMu.RLock()
	lib := s.lib
	if lib == nil {
		s.libMu.RUnlock()
		return fail(fmt.Errorf("library not loaded — call tempest_index_library first"))
	}

	// Build slot maps for Bank A and Bank B.
	// Hold the read lock through the loop so BankSlot reads don't race
	// with concurrent tempest_load_sound writes.
	type entry struct {
		name     string
		loadedAt time.Time
	}
	bankA := make(map[int]entry)
	bankB := make(map[int]entry)
	for _, snd := range lib.Sounds {
		if snd.BankSlot == nil {
			continue
		}
		e := entry{name: snd.Name, loadedAt: snd.BankSlot.LoadedAt}
		switch snd.BankSlot.Bank {
		case "A":
			bankA[snd.BankSlot.Slot] = e
		case "B":
			bankB[snd.BankSlot.Slot] = e
		}
	}
	s.libMu.RUnlock()

	var sb strings.Builder
	for _, bankName := range []string{"A", "B"} {
		var m map[int]entry
		if bankName == "A" {
			m = bankA
		} else {
			m = bankB
		}
		fmt.Fprintf(&sb, "Bank %s:\n", bankName)
		for slot := 1; slot <= 16; slot++ {
			if e, ok := m[slot]; ok {
				fmt.Fprintf(&sb, "  Slot %2d: %-16s (loaded %s)\n",
					slot, e.name, e.loadedAt.Format("2006-01-02"))
			} else {
				fmt.Fprintf(&sb, "  Slot %2d: (untracked)\n", slot)
			}
		}
	}
	return ok(sb.String()), nil
}

// ── SysEx Tools ───────────────────────────────────────────────────────────────

func (s *Server) registerSysExTools() {
	s.mcp.AddTool(mcp.NewTool("tempest_wait_for_dump",
		mcp.WithDescription("Wait for an incoming SysEx dump from the Tempest and return a summary. "+
			"IMPORTANT: The Tempest cannot be queried programmatically. You must manually trigger the dump "+
			"from the hardware first: SETUP → MIDI → Send Program / Send Edit Buffer / Send Project. "+
			"Then call this tool within the timeout window."),
		mcp.WithNumber("timeout_sec", mcp.Description("How many seconds to wait (default 30)")),
	), s.handleWaitForDump)

	s.mcp.AddTool(mcp.NewTool("tempest_save_received_dump",
		mcp.WithDescription("Wait for an incoming SysEx dump and save it to a .syx file."),
		mcp.WithString("path", mcp.Required(), mcp.Description("Destination file path (e.g. ~/Tempest/MyKick.syx)")),
		mcp.WithNumber("timeout_sec", mcp.Description("Seconds to wait (default 30)")),
	), s.handleSaveReceivedDump)

	s.mcp.AddTool(mcp.NewTool("tempest_send_syx_file",
		mcp.WithDescription("Send a .syx file to the Tempest over USB MIDI. "+
			"Inserts a 1-second pause between messages as required for bulk import."),
		mcp.WithString("path", mcp.Required(), mcp.Description("Path to .syx file")),
	), s.handleSendSyxFile)

	s.mcp.AddTool(mcp.NewTool("tempest_extract_sounds_from_project",
		mcp.WithDescription("Wait for an incoming project dump (0x61) and extract individual sounds from it. "+
			"Trigger a project dump from SETUP → MIDI → Send Project on the Tempest first."),
		mcp.WithString("output_dir", mcp.Required(), mcp.Description("Directory to save extracted .syx files")),
		mcp.WithNumber("timeout_sec", mcp.Description("Seconds to wait (default 60)")),
		mcp.WithNumber("min_quality", mcp.Description("Minimum extraction quality 0–100 (default 70)")),
	), s.handleExtractSoundsFromProject)

	s.mcp.AddTool(mcp.NewTool("tempest_read_sound_params",
		mcp.WithDescription("Decode a Sound (0x60 RAM/edit-buffer) dump into its on-device name and named "+
			"synthesis parameters — oscillators, filter, envelopes, LFOs, mod matrix. Provide path to "+
			"decode a previously saved .syx file, or omit it to wait for a live dump: on the Tempest "+
			"press Save/Load → Export Sound over MIDI → Next → USB → Export Now. "+
			"The name is bit-packed and confirmed against 46 real hardware captures (see "+
			"docs/sysex-tempest-format.md §9.9) — it reflects whatever the sound was last saved/renamed "+
			"as, not necessarily anything related to the current edits. "+
			"Parameter bit locations are translated from a community-sourced bit map and spot-checked "+
			"against 3 hardware captures (see §7/§8) — most individual parameters have not been "+
			"independently re-verified."),
		mcp.WithString("path", mcp.Description("Path to a previously saved RAM (0x60) .syx file. Omit to wait for a live dump instead.")),
		mcp.WithNumber("timeout_sec", mcp.Description("Seconds to wait for a live dump if path is omitted (default 30)")),
	), s.handleReadSoundParams)

	s.mcp.AddTool(mcp.NewTool("tempest_export_wizard",
		mcp.WithDescription("Walk through exporting a Beat or Project from the Tempest correctly, catching "+
			"the two mistakes that have repeatedly derailed real capture sessions during this project's own "+
			"research (see docs/sysex-tempest-format.md §9.2/§9.6): confusing Export Beat with Export "+
			"Project (they're adjacent Save/Load menu items), and skipping the required beat-selection step "+
			"before exporting a Beat. Relay these steps to the user before calling this tool, then call it "+
			"to wait for the dump and validate what actually arrived:\n\n"+
			"For intent=beat:\n"+
			"  1. In 16 Beats mode, tap the pad for the beat you want to export (do not skip this — the "+
			"manual lists it as step one of the procedure, and it's easy to miss).\n"+
			"  2. Recommended: press the Events key to check the Beat Events screen and confirm on-screen "+
			"which notes are actually present, rather than trusting memory or a different screen.\n"+
			"  3. Press Save/Load.\n"+
			"  4. Confirm the screen reads \"Export Beat over MIDI\" — NOT \"Export Project\". These "+
			"are adjacent menu items and this mixup has happened repeatedly in real testing.\n"+
			"  5. Press Next. If a \"Source Beat\" selection screen appears, confirm it shows the same "+
			"beat number/name you selected in step 1 before continuing.\n"+
			"  6. Set destination to USB, press Export Now.\n\n"+
			"For intent=project:\n"+
			"  1. Press Save/Load.\n"+
			"  2. Confirm the screen reads \"Export Project over MIDI\" — NOT \"Export Beat\".\n"+
			"  3. Press Next, set destination to USB, press Export Now.\n"+
			"  Note: this tool only validates the first incoming SysEx message. A live Project RAM export "+
			"sends 17 separate messages (one 0x5E header + sixteen 0x5C per-beat messages) — this tool will "+
			"only confirm the header arrived, not decode the full project. For a single self-contained "+
			"0x61 dump, use \"Export saved file over MIDI\" from a flash-saved Project instead.\n\n"+
			"After a Beat dump: reports the byte-count-implied note count (each note adds exactly 8 bytes "+
			"to the raw dump; see docs/sysex-tempest-format.md §7.3) as a fact for you to check against "+
			"what you intended. Up to two simultaneous notes are confirmed to export correctly when the "+
			"procedure above is followed exactly (see §9.13); three or more remain untested."),
		mcp.WithString("intent", mcp.Required(), mcp.Description("What you're exporting: \"beat\" or \"project\"")),
		mcp.WithNumber("timeout_sec", mcp.Description("Seconds to wait for the dump (default 30)")),
	), s.handleExportWizard)

	s.mcp.AddTool(mcp.NewTool("tempest_decode_project_beats",
		mcp.WithDescription("Decode all 16 beats from a Project (0x61) dump: name, short name, BPM, "+
			"swing, and any detected note records, per beat. Provide path to decode a previously saved "+
			"\"Export saved file over MIDI\" .syx file (from a flash-saved Project), or omit it to wait "+
			"for a live dump — trigger it from Save/Load → \"Export Project over MIDI\" → Next → "+
			"USB → Export Now (a live RAM export sends 17 separate messages; this tool only decodes a "+
			"single self-contained 0x61 dump, so prefer the saved-file export path). Layout confirmed "+
			"against one real hardware sample — see docs/sysex-tempest-format.md §9.10. "+
			"IMPORTANT caveats: (1) the note-record format is confirmed against real hardware for up to "+
			"two simultaneous notes (§9.13); three or more remain untested. (2) Per §9.5/§9.10, a "+
			"Project export may reflect stale/saved state "+
			"rather than the Tempest's live edit buffer — don't assume it matches what's currently on "+
			"screen without checking. (3) If beat contents look garbled from some point onward, the most "+
			"likely cause is an earlier beat's note count being misdecoded, which misaligns every beat "+
			"after it (blocks are packed back-to-back with no fixed spacing) — this tool reports that as "+
			"an error rather than returning garbage silently."),
		mcp.WithString("path", mcp.Description("Path to a previously saved Project (0x61) .syx file. Omit to wait for a live dump instead.")),
		mcp.WithNumber("timeout_sec", mcp.Description("Seconds to wait for a live dump if path is omitted (default 30)")),
	), s.handleDecodeProjectBeats)

	s.mcp.AddTool(mcp.NewTool("tempest_analyze_project",
		mcp.WithDescription("Diagnostic summary of a Project (0x61) dump: byte size, how many of the "+
			"16 beats are still at the default \"Initialize\" state vs. have content, total note "+
			"records across the whole project, and the same research caveats as "+
			"tempest_decode_project_beats (stale-vs-live-state uncertainty, note records confirmed up "+
			"to three simultaneous notes, unconfirmed project-header fields beyond name/bpm/swing — "+
			"see docs/sysex-tempest-format.md §9.5/§9.10/§9.13/§9.15). This is a project-level overview, not "+
			"a full per-beat dump — use tempest_decode_project_beats for individual beat/note detail. "+
			"Provide path to analyze a previously saved \"Export saved file over MIDI\" .syx file, or "+
			"omit it to wait for a live dump (see tempest_decode_project_beats's description for the "+
			"same live-export caveat about the 17-message RAM export path)."),
		mcp.WithString("path", mcp.Description("Path to a previously saved Project (0x61) .syx file. Omit to wait for a live dump instead.")),
		mcp.WithNumber("timeout_sec", mcp.Description("Seconds to wait for a live dump if path is omitted (default 30)")),
	), s.handleAnalyzeProject)

	s.mcp.AddTool(mcp.NewTool("tempest_write_beat",
		mcp.WithDescription("Send a modified Beat/Kit dump to the Tempest, replacing its note pattern "+
			"and/or name/tempo/swing. Confirmed working against real hardware for one note "+
			"(docs/sysex-tempest-format.md §9.16) and two notes (§9.17): a synthesized beat sent this way "+
			"was accepted and exported back byte-exact both times. Three-note writes are untested — "+
			"EncodeBeat's encoding is confirmed correct at three notes (§9.15), but that confirmation is "+
			"for decoding/re-encoding, not an actual send, and §9.15 separately found three-note *export* "+
			"from the Tempest isn't perfectly reliable, so a three-note write is genuinely unverified "+
			"territory, not just an extrapolation. Requires a base Beat/Kit (0x5F) .syx file (export one "+
			"first with tempest_export_wizard or tempest_save_received_dump) — every byte this tool "+
			"doesn't understand (the pad table, several still-unconfirmed header fields) is preserved "+
			"unchanged from that base rather than guessed at. notes REPLACES the base's note records "+
			"entirely, not merges with them — omit it to keep the base's notes unchanged while only "+
			"editing name/bpm/swing. "+
			"IMPORTANT — slot targeting (§9.17): there is no destination field in this message. The write "+
			"lands on whatever beat is currently selected on the Tempest's own UI (16 Beats mode) at "+
			"receive time — select the destination on the Tempest before calling this tool, the same as "+
			"sound loading. It cannot be targeted from software. "+
			"IMPORTANT — no receipt confirmation: this overwrites the beat currently in the Tempest's live "+
			"edit buffer, and the Tempest gives no confirmation it worked — always export and re-check "+
			"afterward (tempest_decode_project_beats or another export+read cycle) rather than trusting "+
			"the send alone."),
		mcp.WithString("path", mcp.Required(), mcp.Description("Path to a previously saved Beat/Kit (0x5F) .syx file to use as the base")),
		mcp.WithString("name", mcp.Description("New beat name (up to 20 chars). Omit to keep the base's name.")),
		mcp.WithString("short_name", mcp.Description("New short name (up to 8 chars). Omit to keep the base's short name.")),
		mcp.WithNumber("bpm", mcp.Description("New tempo in BPM. Omit to keep the base's tempo.")),
		mcp.WithNumber("swing", mcp.Description("New swing percentage, 50-75. Omit to keep the base's swing.")),
		mcp.WithString("notes", mcp.Description("JSON array replacing the base's notes entirely, e.g. "+
			"[{\"track\":\"A1\",\"step\":1,\"velocity\":100},{\"track\":\"A2\",\"step\":2,\"velocity\":90}]. "+
			"track is \"A1\"-\"A16\" or \"B1\"-\"B16\"; step is 1-based. Omit to keep the base's notes unchanged.")),
	), s.handleWriteBeat)

	s.mcp.AddTool(mcp.NewTool("tempest_clear_beat",
		mcp.WithDescription("Send a Beat/Kit dump with every note removed, keeping the base's name/tempo/"+
			"swing (or a new name if given). A thin wrapper over tempest_write_beat with notes forced "+
			"empty — see its description for the base-file requirement and the same write-then-verify "+
			"caveats."),
		mcp.WithString("path", mcp.Required(), mcp.Description("Path to a previously saved Beat/Kit (0x5F) .syx file to use as the base")),
		mcp.WithString("name", mcp.Description("New beat name (up to 20 chars). Omit to keep the base's name.")),
	), s.handleClearBeat)
}

func (s *Server) handleWaitForDump(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.requireDevice(); err != nil {
		return fail(err)
	}
	timeout := intArg(req, "timeout_sec", 30)
	ch, cancel := s.device.Subscribe()
	defer cancel()
	select {
	case raw := <-ch:
		return s.describeDump(raw)
	case <-time.After(time.Duration(timeout) * time.Second):
		return fail(fmt.Errorf("timeout after %ds — trigger a dump from SETUP → MIDI on the Tempest", timeout))
	}
}

func (s *Server) handleSaveReceivedDump(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.requireDevice(); err != nil {
		return fail(err)
	}
	rawPath := strArg(req, "path")
	path, err := sanitizePath(rawPath)
	if err != nil {
		return fail(fmt.Errorf("invalid path: %w", err))
	}
	timeout := intArg(req, "timeout_sec", 30)
	ch, cancel := s.device.Subscribe()
	defer cancel()
	select {
	case raw := <-ch:
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			return fail(fmt.Errorf("writing %s: %w", path, err))
		}
		desc, err := s.describeDump(raw)
		if err != nil || len(desc.Content) == 0 {
			return ok(fmt.Sprintf("Saved %d bytes to %s", len(raw), path)), nil
		}
		var descText string
		if tc, ok := desc.Content[0].(mcp.TextContent); ok {
			descText = tc.Text
		}
		return ok(fmt.Sprintf("Saved %d bytes to %s\n%s", len(raw), path, descText)), nil
	case <-time.After(time.Duration(timeout) * time.Second):
		return fail(fmt.Errorf("timeout after %ds", timeout))
	}
}

func (s *Server) handleSendSyxFile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.requireDevice(); err != nil {
		return fail(err)
	}
	rawPath := strArg(req, "path")
	path, err := sanitizePath(rawPath)
	if err != nil {
		return fail(fmt.Errorf("invalid path: %w", err))
	}
	msgs, err := library.ReadSyxMessages(path)
	if err != nil {
		return fail(err)
	}
	if err := s.device.SendRawWithDelay(ctx, msgs, s.cfg.SysEx.InterMessageDelayMS); err != nil {
		return fail(err)
	}
	return ok(fmt.Sprintf("Sent %d SysEx messages from %s", len(msgs), path)), nil
}

func (s *Server) handleExtractSoundsFromProject(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.requireDevice(); err != nil {
		return fail(err)
	}
	rawDir := strArg(req, "output_dir")
	outDir, err := sanitizePath(rawDir)
	if err != nil {
		return fail(fmt.Errorf("invalid output_dir: %w", err))
	}
	timeout := intArg(req, "timeout_sec", 60)
	minQ := intArg(req, "min_quality", 70)
	ch, cancel := s.device.Subscribe()
	defer cancel()
	select {
	case raw := <-ch:
		if sysex.Identify(raw) != sysex.TypeProjectDump {
			return fail(fmt.Errorf("received dump is not a project (0x61) — trigger Send Project from SETUP → MIDI"))
		}
		sounds, err := sysex.ExtractSoundsFromProject(raw, minQ, "Sound")
		if err != nil {
			return fail(err)
		}
		if err := os.MkdirAll(outDir, 0o700); err != nil {
			return fail(err)
		}
		var writeErrs int
		for i, dump := range sounds {
			fname := filepath.Join(outDir, fmt.Sprintf("extracted_%03d.syx", i+1))
			if err := os.WriteFile(fname, dump, 0o600); err != nil {
				s.log.Warnf("writing %s: %v", fname, err)
				writeErrs++
			}
		}
		msg := fmt.Sprintf("Extracted %d sounds → %s", len(sounds), outDir)
		if writeErrs > 0 {
			msg += fmt.Sprintf(" (%d writes failed, check logs)", writeErrs)
		}
		if len(sounds) == 0 {
			msg += "\n\nNote: this scans for embedded FLASH-style Sound parameter blocks " +
				"(the same signature BuildFLASHDump writes). Per docs/sysex-tempest-format.md " +
				"§9.10/§9.11, a Project dump's 16 beats are kit blocks with a 30-byte-per-pad " +
				"table, not full 132-byte embedded Sound blocks - a real Project dump can " +
				"legitimately have zero matches here. Use tempest_decode_project_beats to see " +
				"the beat/pad contents this dump actually has."
		}
		return ok(msg), nil
	case <-time.After(time.Duration(timeout) * time.Second):
		return fail(fmt.Errorf("timeout after %ds — trigger Send Project from SETUP → MIDI", timeout))
	}
}

// loadDumpOrWait returns the raw SysEx bytes read from the request's
// "path" argument if set, otherwise waits for a live dump using the
// request's "timeout_sec" argument (default 30s). triggerHint is included
// in the timeout error to tell the caller what to press on the Tempest.
func (s *Server) loadDumpOrWait(req mcp.CallToolRequest, triggerHint string) ([]byte, error) {
	if rawPath := strArg(req, "path"); rawPath != "" {
		path, err := sanitizePath(rawPath)
		if err != nil {
			return nil, fmt.Errorf("invalid path: %w", err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		return raw, nil
	}
	if err := s.requireDevice(); err != nil {
		return nil, err
	}
	timeout := intArg(req, "timeout_sec", 30)
	ch, cancel := s.device.Subscribe()
	defer cancel()
	select {
	case raw := <-ch:
		return raw, nil
	case <-time.After(time.Duration(timeout) * time.Second):
		return nil, fmt.Errorf("timeout after %ds — %s", timeout, triggerHint)
	}
}

func (s *Server) handleReadSoundParams(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	raw, err := s.loadDumpOrWait(req, "trigger Export Sound over MIDI from Save/Load on the Tempest")
	if err != nil {
		return fail(err)
	}

	if sysex.Identify(raw) != sysex.TypeRAMSound {
		return fail(fmt.Errorf("dump is not a Sound (0x60) RAM/edit-buffer dump"))
	}
	unescaped := sysex.Unescape(raw)
	name, _ := sysex.ExtractName(unescaped, sysex.TypeRAMSound)
	params := sysex.ExtractParams(unescaped, sysex.TypeRAMSound)
	decoded := sysex.DecodeSoundParams(params)

	var b strings.Builder
	fmt.Fprintf(&b, "Name: %q\n", name)
	section := ""
	for _, p := range sysex.SoundParams {
		if p.Section != section {
			section = p.Section
			fmt.Fprintf(&b, "\n%s\n", section)
		}
		display, numeric, dispOK := sysex.DisplaySoundParam(p, decoded[p.Name])
		if dispOK && display != "" {
			fmt.Fprintf(&b, "  %-28s %s\n", p.Name, display)
		} else {
			fmt.Fprintf(&b, "  %-28s %g\n", p.Name, numeric)
		}
	}
	return ok(strings.TrimSpace(b.String())), nil
}

func (s *Server) handleDecodeProjectBeats(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	raw, err := s.loadDumpOrWait(req, "trigger \"Export Project over MIDI\" or \"Export saved file over MIDI\" from Save/Load on the Tempest")
	if err != nil {
		return fail(err)
	}

	if sysex.Identify(raw) != sysex.TypeProjectDump {
		return fail(fmt.Errorf("dump is not a Project (0x61) dump"))
	}
	unescaped := sysex.Unescape(raw)
	projectName, _ := sysex.ExtractName(unescaped, sysex.TypeProjectDump)
	beats, decodeErr := sysex.ProjectBeats(unescaped)

	var b strings.Builder
	fmt.Fprintf(&b, "Project %q — %d of %d beats decoded:\n", projectName, len(beats), sysex.BeatsPerProject)
	unconfirmedNoteCountSeen := false
	for _, beat := range beats {
		fmt.Fprintf(&b, "\nBeat %2d: %q (short: %q)  bpm=%.1f  swing=%.1f%%  notes=%d\n",
			beat.Index+1, beat.Name, beat.ShortName, beat.BPM, beat.Swing, len(beat.Notes))
		for _, n := range beat.Notes {
			fmt.Fprintf(&b, "    track=%d step=%d velocity=%d\n", n.Track, n.Step, n.Velocity)
		}
		if len(beat.Notes) > 3 {
			unconfirmedNoteCountSeen = true
		}
	}
	if unconfirmedNoteCountSeen {
		fmt.Fprint(&b, "\nNote: one or more beats above show more than three note records. The record "+
			"format is confirmed against real hardware for up to three simultaneous notes (see "+
			"docs/sysex-tempest-format.md §9.15); four or more remain untested.\n")
	}
	if decodeErr != nil {
		fmt.Fprintf(&b, "\nDecoding stopped early: %v\n", decodeErr)
	}
	return ok(b.String()), nil
}

func (s *Server) handleAnalyzeProject(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	raw, err := s.loadDumpOrWait(req, "trigger \"Export Project over MIDI\" or \"Export saved file over MIDI\" from Save/Load on the Tempest")
	if err != nil {
		return fail(err)
	}

	if sysex.Identify(raw) != sysex.TypeProjectDump {
		return fail(fmt.Errorf("dump is not a Project (0x61) dump — for a Beat/Kit export use tempest_export_wizard or tempest_wait_for_dump instead"))
	}
	unescaped := sysex.Unescape(raw)
	projectName, _ := sysex.ExtractName(unescaped, sysex.TypeProjectDump)
	beats, decodeErr := sysex.ProjectBeats(unescaped)

	totalNotes := 0
	multiNoteBeats := 0
	unconfirmedNoteCountBeats := 0
	initializeBeats := 0
	var customized []int
	for _, beat := range beats {
		totalNotes += len(beat.Notes)
		if len(beat.Notes) > 1 {
			multiNoteBeats++
		}
		if len(beat.Notes) > 3 {
			unconfirmedNoteCountBeats++
		}
		if strings.TrimSpace(beat.Name) == "Initialize" && len(beat.Notes) == 0 {
			initializeBeats++
		} else {
			customized = append(customized, beat.Index+1)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Project %q — %d bytes, %d of %d beats decoded\n",
		projectName, len(raw), len(beats), sysex.BeatsPerProject)
	fmt.Fprintf(&b, "  Total note records across all beats: %d\n", totalNotes)
	fmt.Fprintf(&b, "  Beats still at default \"Initialize\" (no notes, unrenamed): %d\n", initializeBeats)
	if len(customized) > 0 {
		fmt.Fprintf(&b, "  Beats with content (renamed and/or notes present): %v\n", customized)
	}
	if multiNoteBeats > 0 {
		fmt.Fprintf(&b, "  Beats with more than one note record: %d\n", multiNoteBeats)
	}
	if decodeErr != nil {
		fmt.Fprintf(&b, "  Decoding stopped early: %v\n", decodeErr)
	}

	fmt.Fprint(&b, "\nCaveats:\n"+
		"  - Per docs/sysex-tempest-format.md §9.5/§9.10, a Project export may reflect stale/saved "+
		"state rather than the Tempest's live edit buffer — don't assume it matches what's currently "+
		"on screen without checking.\n"+
		"  - Per §9.10, the project-header field map beyond name/bpm/swing is mostly unconfirmed.\n")
	if unconfirmedNoteCountBeats > 0 {
		fmt.Fprint(&b, "  - Beats reported with more than three note records use a record format "+
			"confirmed against real hardware only up to three simultaneous notes (§9.15); four or "+
			"more remain untested.\n")
	}
	fmt.Fprint(&b, "\nFor full per-beat detail (name, bpm, swing, individual note records), use "+
		"tempest_decode_project_beats.\n")

	return ok(b.String()), nil
}

// parseTrackName converts a sequencer track name ("A1"-"A16" or "B1"-"B16")
// to its 0-based track index (0-15 for A, 16-31 for B), matching
// sysex.NoteRecord.Track's convention.
func parseTrackName(name string) (int, error) {
	name = strings.ToUpper(strings.TrimSpace(name))
	if len(name) < 2 {
		return 0, fmt.Errorf("invalid track %q: want \"A1\"-\"A16\" or \"B1\"-\"B16\"", name)
	}
	bank := name[0]
	if bank != 'A' && bank != 'B' {
		return 0, fmt.Errorf("invalid track %q: bank must be A or B", name)
	}
	num, err := strconv.Atoi(name[1:])
	if err != nil || num < 1 || num > 16 {
		return 0, fmt.Errorf("invalid track %q: pad number must be 1-16", name)
	}
	idx := num - 1
	if bank == 'B' {
		idx += 16
	}
	return idx, nil
}

// loadBaseKit reads and decodes a Beat/Kit (0x5F) .syx file for use as an
// EncodeBeat base, returning both the raw unescaped payload (EncodeBeat's
// base argument) and its decoded Kit (the starting point for edits).
func loadBaseKit(rawPath string) ([]byte, *sysex.Kit, error) {
	path, err := sanitizePath(rawPath)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid path: %w", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	if sysex.Identify(raw) != sysex.TypeBeatDump {
		return nil, nil, fmt.Errorf("%s is not a Beat/Kit (0x5F) dump", path)
	}
	base := sysex.Unescape(raw)
	kit, err := sysex.DecodeBeat(base)
	if err != nil {
		return nil, nil, err
	}
	return base, kit, nil
}

// sendKit encodes kit against base and sends the resulting Beat/Kit dump to
// the Tempest, returning a summary string including the standing
// write-then-verify warning (see tempest_write_beat's description).
func (s *Server) sendKit(base []byte, kit *sysex.Kit) (string, error) {
	encoded, err := sysex.EncodeBeat(base, kit)
	if err != nil {
		return "", err
	}
	wire := sysex.BuildBeatDump(encoded)
	if err := s.device.SendRaw(wire); err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"Sent: name=%q short_name=%q bpm=%.1f swing=%.1f notes=%d. "+
			"The Tempest cannot confirm receipt — export the beat again and decode it "+
			"(tempest_decode_project_beats, or capture-tmp) to verify this actually took effect.",
		kit.Name, kit.ShortName, kit.BPM, kit.Swing, len(kit.Notes)), nil
}

func (s *Server) handleWriteBeat(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.requireDevice(); err != nil {
		return fail(err)
	}
	base, kit, err := loadBaseKit(strArg(req, "path"))
	if err != nil {
		return fail(err)
	}

	if name := strArg(req, "name"); name != "" {
		kit.Name = name
	}
	if shortName := strArg(req, "short_name"); shortName != "" {
		kit.ShortName = shortName
	}
	if bpm := floatArg(req, "bpm", 0); bpm > 0 {
		kit.BPM = bpm
	}
	if swing := floatArg(req, "swing", -1); swing >= 0 {
		kit.Swing = swing
	}
	if notesJSON := strArg(req, "notes"); notesJSON != "" {
		var rawNotes []struct {
			Track    string `json:"track"`
			Step     int    `json:"step"`
			Velocity int    `json:"velocity"`
		}
		if err := json.Unmarshal([]byte(notesJSON), &rawNotes); err != nil {
			return fail(fmt.Errorf("invalid notes JSON: %w", err))
		}
		notes := make([]sysex.NoteRecord, len(rawNotes))
		for i, n := range rawNotes {
			track, err := parseTrackName(n.Track)
			if err != nil {
				return fail(err)
			}
			notes[i] = sysex.NoteRecord{Track: track, Step: n.Step, Velocity: int(clampUint7(n.Velocity))}
		}
		kit.Notes = notes
	}

	summary, err := s.sendKit(base, kit)
	if err != nil {
		return fail(err)
	}
	return ok(summary), nil
}

func (s *Server) handleClearBeat(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.requireDevice(); err != nil {
		return fail(err)
	}
	base, kit, err := loadBaseKit(strArg(req, "path"))
	if err != nil {
		return fail(err)
	}
	if name := strArg(req, "name"); name != "" {
		kit.Name = name
	}
	kit.Notes = nil

	summary, err := s.sendKit(base, kit)
	if err != nil {
		return fail(err)
	}
	return ok(summary), nil
}

// exportWizardWantType maps a validated tempest_export_wizard "intent"
// argument to the SysEx message type and human label it should see.
func exportWizardWantType(intent string) (t sysex.MessageType, label string) {
	if intent == "project" {
		return sysex.TypeProjectDump, "Project"
	}
	return sysex.TypeBeatDump, "Beat"
}

// exportWizardMismatchLabel describes an unexpectedly-received message type
// for tempest_export_wizard's intent-mismatch error, calling out the two
// menu mixups this tool exists to catch (see its registration doc).
func exportWizardMismatchLabel(got sysex.MessageType) string {
	switch got {
	case sysex.TypeBeatDump:
		return "a Beat/Kit dump (0x5F) — did you export Beat when you meant to export Project, or select the wrong menu item?"
	case sysex.TypeProjectDump:
		return "a Project dump (0x61) — did you export Project when you meant to export Beat?"
	case sysex.TypeRAMSound, sysex.TypeFLASHSound, sysex.TypeAlternateSound, sysex.TypeAlternateBank:
		return "a Sound dump, not a Beat or Project — check you're in Save/Load, not Sound Edit"
	}
	return "an unrecognised message"
}

// beatNoteCountLine reports the note count implied by a Beat/Kit dump's raw
// size (base 5925 bytes + 8 bytes/note, see docs/sysex-tempest-format.md
// §7.3), or explains why the size doesn't fit that pattern.
func beatNoteCountLine(rawLen int) string {
	const baseSize = 5925
	const bytesPerNote = 8
	extra := rawLen - baseSize
	if extra >= 0 && extra%bytesPerNote == 0 {
		return fmt.Sprintf("\n  Notes implied by size: %d", extra/bytesPerNote)
	}
	return fmt.Sprintf("\n  Notes implied by size: unclear (%d bytes doesn't match the base+8N pattern)", rawLen)
}

func (s *Server) handleExportWizard(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := s.requireDevice(); err != nil {
		return fail(err)
	}
	intent := strings.ToLower(strArg(req, "intent"))
	if intent != "beat" && intent != "project" {
		return fail(fmt.Errorf("intent must be \"beat\" or \"project\", got %q", intent))
	}
	timeout := intArg(req, "timeout_sec", 30)
	ch, cancel := s.device.Subscribe()
	defer cancel()

	var raw []byte
	select {
	case raw = <-ch:
	case <-time.After(time.Duration(timeout) * time.Second):
		return fail(fmt.Errorf("timeout after %ds — trigger the export from Save/Load on the Tempest", timeout))
	}

	t := sysex.Identify(raw)
	wantType, wantLabel := exportWizardWantType(intent)

	if t != wantType {
		return fail(fmt.Errorf("expected %s export but received %s (%d bytes)",
			wantLabel, exportWizardMismatchLabel(t), len(raw)))
	}

	unescaped := sysex.Unescape(raw)
	name, _ := sysex.ExtractName(unescaped, t)

	if t == sysex.TypeBeatDump {
		return ok(fmt.Sprintf(
			"Received Beat/Kit dump (0x5F):\n  Name: %q\n  Size: %d bytes%s\n\n"+
				"This is an observed fact from byte count alone, per docs/sysex-tempest-format.md §7.3. "+
				"Up to three simultaneous notes are confirmed against real hardware to decode/encode "+
				"correctly (§9.13/§9.15), but export reliability at three notes is not perfect — one of "+
				"two identical controlled attempts corrupted a note's step field (§9.15) — so verify "+
				"anything exported with three notes by reading it back, not by trusting a single export.",
			name, len(raw), beatNoteCountLine(len(raw)))), nil
	}

	return ok(fmt.Sprintf(
		"Received Project header dump (0x61):\n  Name: %q\n  Size: %d bytes\n\n"+
			"Note: only this first message was validated. A live Project RAM export over MIDI sends 17 "+
			"messages total (this 0x5E/0x61 header plus 16 per-beat 0x5C messages); this tool does not "+
			"collect or decode the remaining ones. Full project decode is unsolved — see "+
			"docs/sysex-tempest-format.md §9.8.",
		name, len(raw))), nil
}

// ── Utility Tools ─────────────────────────────────────────────────────────────

func (s *Server) registerUtilityTools() {
	s.mcp.AddTool(mcp.NewTool("tempest_ping",
		mcp.WithDescription("Check connection to the Tempest. Returns device name and status."),
	), func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := s.requireDevice(); err != nil {
			return ok(fmt.Sprintf("NOT connected: %v", err)), nil
		}
		return ok(fmt.Sprintf("Connected to %q on MIDI channel %d. Library: %d sounds loaded.",
			s.device.Name(), s.device.Channel(), len(s.libSounds()))), nil
	})

	s.mcp.AddTool(mcp.NewTool("tempest_list_ports",
		mcp.WithDescription("List all available MIDI output and input port names on this computer."),
	), func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		outs, ins, err := midi.ListPorts()
		if err != nil {
			return fail(err)
		}
		var sb strings.Builder
		sb.WriteString("MIDI Output ports:\n")
		for _, o := range outs {
			sb.WriteString("  " + o + "\n")
		}
		sb.WriteString("MIDI Input ports:\n")
		for _, i := range ins {
			sb.WriteString("  " + i + "\n")
		}
		return ok(sb.String()), nil
	})

	s.mcp.AddTool(mcp.NewTool("tempest_list_pad_names",
		mcp.WithDescription("List all recognised pad names that can be used with tempest_trigger_pad."),
	), func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		names := midi.ListPadNames()
		return ok("Pad names: " + strings.Join(names, ", ")), nil
	})

	s.mcp.AddTool(mcp.NewTool("tempest_set_channel",
		mcp.WithDescription("Change the MIDI channel used for note and CC messages (1–16, default 10)."),
		mcp.WithNumber("channel", mcp.Required(), mcp.Description("MIDI channel 1–16")),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ch := intArg(req, "channel", 10)
		if ch < 1 || ch > 16 {
			return fail(fmt.Errorf("channel must be 1–16"))
		}
		s.device.SetChannel(uint8(ch))
		return ok(fmt.Sprintf("MIDI channel set to %d", ch)), nil
	})
}

// describeDump returns a human-readable summary of a raw SysEx dump.
func (s *Server) describeDump(raw []byte) (*mcp.CallToolResult, error) {
	t := sysex.Identify(raw)

	unescaped := sysex.Unescape(raw)
	name, _ := sysex.ExtractName(unescaped, t)

	fp, _ := sysex.Fingerprint(raw)

	typeName := map[sysex.MessageType]string{
		sysex.TypeRAMSound:       "RAM sound (0x60)",
		sysex.TypeProjectDump:    "Project dump (0x61)",
		sysex.TypeBeatFileDump:   "Beat file export (0x62)",
		sysex.TypeFLASHSound:     "FLASH sound (0x63)",
		sysex.TypeAlternateSound: "Alternate sound (0x5C)",
		sysex.TypeAlternateBank:  "Alternate bank (0x5E)",
		sysex.TypeBeatDump:       "Beat/Kit dump (0x5F)",
	}[t]
	if typeName == "" {
		typeName = "Unknown"
	}

	// Bank/Slot is only meaningful for 0x5C (alternate bank sound) — see
	// sysex.Location's doc comment. FLASH's byte[4] is a name-length prefix,
	// not a slot, so it's omitted here rather than shown as misleading info.
	var bankLine string
	if t == sysex.TypeAlternateSound {
		bank, slot := sysex.BankSlot(sysex.Location(raw))
		bankLine = fmt.Sprintf("\n  Bank: %s, Slot: %d", bank, slot)
	}

	summary := fmt.Sprintf(
		"Received SysEx dump:\n  Type: %s\n  Name: %q%s\n  Size: %d bytes\n  Fingerprint: %s",
		typeName, name, bankLine, len(raw), fp)

	return ok(summary), nil
}

func (s *Server) libSounds() []*library.Sound {
	lib := s.getLib()
	if lib == nil {
		return nil
	}
	return lib.Sounds
}

// ── Sound Design Tools ────────────────────────────────────────────────────────

func (s *Server) registerSoundTools() {
	s.mcp.AddTool(mcp.NewTool("tempest_morph_sound",
		mcp.WithDescription("Create a new sound by linearly blending the parameter bytes of two library sounds. "+
			"mix=0.0 → 100% sound A; mix=1.0 → 100% sound B; mix=0.5 → equal blend. "+
			"All parameter bytes are clamped to the valid 7-bit range (0–127). "+
			"The result is saved as a FLASH (.syx) file and optionally sent to the Tempest."),
		mcp.WithString("sound_id_a", mcp.Required(), mcp.Description("First sound ID or prefix (from tempest_list_sounds)")),
		mcp.WithString("sound_id_b", mcp.Required(), mcp.Description("Second sound ID or prefix")),
		mcp.WithNumber("mix", mcp.Description("Blend ratio 0.0–1.0 (default 0.5)")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Name for the morphed sound (max 8 chars for hardware display)")),
		mcp.WithString("output_path", mcp.Required(), mcp.Description("Destination .syx file path (e.g. ~/Tempest/Custom/MyMorph.syx)")),
		mcp.WithBoolean("send", mcp.Description("Send the morphed sound to the Tempest after saving (default false)")),
	), s.handleMorphSound)

	s.mcp.AddTool(mcp.NewTool("tempest_create_sound",
		mcp.WithDescription("Create a new blank sound and save it as a .syx file. "+
			"The parameter block is initialised to the Tempest reference signature defaults. "+
			"Use tempest_morph_sound to blend this with an existing sound for more interesting results. "+
			"Per-parameter editing (filter cutoff, envelope, etc.) requires hardware-capture research "+
			"to confirm byte offsets; that is not yet implemented."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Sound name (max 8 chars for hardware display)")),
		mcp.WithString("output_path", mcp.Required(), mcp.Description("Destination .syx file path")),
		mcp.WithBoolean("send", mcp.Description("Send to Tempest after saving (default false)")),
	), s.handleCreateSound)
}

func (s *Server) handleMorphSound(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	lib := s.getLib()
	if lib == nil {
		return fail(fmt.Errorf("library not loaded — call tempest_index_library first"))
	}
	idA := strArg(req, "sound_id_a")
	idB := strArg(req, "sound_id_b")
	mix := floatArg(req, "mix", 0.5)
	name := strArg(req, "name")
	rawPath := strArg(req, "output_path")
	doSend := boolArg(req, "send", false)

	if name == "" {
		return fail(fmt.Errorf("name is required"))
	}
	outPath, err := sanitizePath(rawPath)
	if err != nil {
		return fail(fmt.Errorf("invalid output_path: %w", err))
	}

	sndA := findSound(lib, idA)
	if sndA == nil {
		return fail(fmt.Errorf("sound %q not found in library", idA))
	}
	sndB := findSound(lib, idB)
	if sndB == nil {
		return fail(fmt.Errorf("sound %q not found in library", idB))
	}

	paramsA, err := extractSoundParams(sndA.Path)
	if err != nil {
		return fail(fmt.Errorf("reading %s: %w", sndA.Name, err))
	}
	paramsB, err := extractSoundParams(sndB.Path)
	if err != nil {
		return fail(fmt.Errorf("reading %s: %w", sndB.Name, err))
	}

	morphed := sound.Morph(paramsA, paramsB, mix)
	if len(morphed) == 0 {
		return fail(fmt.Errorf("morph produced empty parameter block — check source sounds"))
	}
	dump := sysex.BuildFLASHDump(name, morphed)

	if err := os.MkdirAll(filepath.Dir(outPath), 0o700); err != nil {
		return fail(err)
	}
	if err := os.WriteFile(outPath, dump, 0o600); err != nil {
		return fail(fmt.Errorf("writing %s: %w", outPath, err))
	}

	result := fmt.Sprintf("Morphed %q + %q (mix=%.2f) → %s (%d bytes)",
		sndA.Name, sndB.Name, mix, outPath, len(dump))

	if doSend {
		if err := s.requireDevice(); err != nil {
			return fail(err)
		}
		if err := s.device.SendRawWithDelay(ctx, [][]byte{dump}, s.cfg.SysEx.InterMessageDelayMS); err != nil {
			return fail(fmt.Errorf("sending to Tempest: %w", err))
		}
		result += "\nSent to Tempest"
	}
	return ok(result), nil
}

func (s *Server) handleCreateSound(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := strArg(req, "name")
	rawPath := strArg(req, "output_path")
	doSend := boolArg(req, "send", false)

	if name == "" {
		return fail(fmt.Errorf("name is required"))
	}
	outPath, err := sanitizePath(rawPath)
	if err != nil {
		return fail(fmt.Errorf("invalid output_path: %w", err))
	}

	params := sound.DefaultBlankParams()
	dump := sysex.BuildFLASHDump(name, params)

	if err := os.MkdirAll(filepath.Dir(outPath), 0o700); err != nil {
		return fail(err)
	}
	if err := os.WriteFile(outPath, dump, 0o600); err != nil {
		return fail(fmt.Errorf("writing %s: %w", outPath, err))
	}

	result := fmt.Sprintf("Created blank sound %q → %s (%d bytes)", name, outPath, len(dump))

	if doSend {
		if err := s.requireDevice(); err != nil {
			return fail(err)
		}
		if err := s.device.SendRawWithDelay(ctx, [][]byte{dump}, s.cfg.SysEx.InterMessageDelayMS); err != nil {
			return fail(fmt.Errorf("sending to Tempest: %w", err))
		}
		result += "\nSent to Tempest"
	}
	return ok(result), nil
}

// findSound looks up a sound by full fingerprint ID or prefix match. Returns
// nil for an empty id rather than matching every sound via an empty-string
// prefix.
func findSound(lib *library.Index, id string) *library.Sound {
	if id == "" {
		return nil
	}
	for _, snd := range lib.Sounds {
		if snd.ID == id || strings.HasPrefix(snd.ID, id) {
			return snd
		}
	}
	return nil
}

// extractSoundParams reads the first FLASH or RAM sound from path and returns its parameter block.
func extractSoundParams(path string) ([]byte, error) {
	msgs, err := library.ReadSyxMessages(path)
	if err != nil {
		return nil, err
	}
	for _, msg := range msgs {
		t := sysex.Identify(msg)
		if t != sysex.TypeFLASHSound && t != sysex.TypeRAMSound {
			continue
		}
		unescaped := sysex.Unescape(msg)
		params := sysex.ExtractParams(unescaped, t)
		if len(params) > 0 {
			return params, nil
		}
	}
	return nil, fmt.Errorf("no FLASH or RAM sound parameters found in %s", path)
}
