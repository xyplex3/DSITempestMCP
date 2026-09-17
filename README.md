# Tempest MCP Server

Control your DSI/Sequential Tempest analog drum machine with Claude through
the Model Context Protocol. Once installed, Claude can trigger pads, run the
sequencer, search and load your sound library, design new sounds, and send or
receive SysEx dumps — all from a conversation.

## Features

- **Pad triggering** — trigger named pads (`kick`, `snare`, `closed-hat`, …)
  or raw MIDI note numbers with velocity and duration
- **Sequence playback** — play a timed JSON event list at any BPM with
  NoteOn/Off scheduling
- **Beat FX CC control** — set distortion, compression, LP/HP filter, envelope
  parameters, and more by name or CC number
- **Sound library** — scan `~/Tempest`, full-text search, and load sounds to
  specific bank/slot assignments tracked in a JSON index
- **Sound design** — morph two library sounds by byte-level parameter
  interpolation; create blank sounds from the Tempest reference signature
- **SysEx I/O** — wait for incoming dumps, save them to disk, send `.syx`
  files back to the hardware, extract individual sounds from project dumps
- **Beat research** — `beat-mapper` CLI computes step and track strides from
  hardware captures to unlock future beat-writing tools

---

## Requirements

| Dependency | Version | Install |
|---|---|---|
| Go | 1.23+ | `brew install go` |
| librtmidi | any | `brew install rtmidi` |
| Tempest firmware | OS 1.3.1.6+ | (for Beat FX CCs) |

librtmidi is the only system-level dependency. The Go toolchain handles
everything else.

---

## Quick Start

### 1. Build

```bash
cd ~/Tools/MCP\ for\ DSI\ Tempest/tempest-mcp
make build
```

This produces a `tempest-mcp` binary in the current directory.

### 2. Confirm the Tempest MIDI port

Connect the Tempest via USB and power it on:

```bash
make list-ports
```

Expected output:

```
MIDI Output ports:
   Tempest (Port 1)
MIDI Input ports:
   Tempest (Port 1)
```

If the port name differs from `"Tempest"`, note the exact string for step 4.

### 3. Install

```bash
make install
```

This copies `tempest-mcp` to `~/.local/bin/`. Ensure that directory is in
your `PATH`:

```bash
# Add to ~/.zshrc or ~/.bash_profile if not already present
export PATH="$HOME/.local/bin:$PATH"
```

### 4. Register with Claude Desktop

Open (or create) the Claude Desktop config file:

```
~/Library/Application Support/Claude/claude_desktop_config.json
```

Add the `tempest` entry:

```json
{
  "mcpServers": {
    "tempest": {
      "command": "/Users/YOUR_USERNAME/.local/bin/tempest-mcp",
      "args": ["--device", "Tempest"]
    }
  }
}
```

Replace `YOUR_USERNAME` with your macOS username. Substitute the `--device`
value if your port name differs. Print a ready-to-paste snippet with:

```bash
make claude-config
```

### 5. Restart Claude Desktop

Quit and reopen Claude Desktop. The Tempest tools appear in Claude's tool
list. Test with:

> *"Call tempest_ping"*

Claude responds with the connected device name and library sound count.

---

## Configuration

A YAML config file is created automatically at
`~/.config/tempest-mcp/config.yaml`. Bootstrap it with:

```bash
make config
```

### Config reference

```yaml
midi:
  device_name: "Tempest"   # substring matched against MIDI port names
  channel: 10              # MIDI channel (GM drums = 10)
  clock_source: internal   # internal | external

library:
  path: ~/Tempest          # your local .syx sound library directory
  index_path: ~/.config/tempest-mcp/library.json
  auto_reindex: true       # rescan on startup if library changed

sysex:
  inter_message_delay_ms: 1000   # required pause between SysEx messages
  capture_dir: ~/.config/tempest-mcp/captures

log:
  level: info              # debug | info | warn | error
  midi_trace: false        # log every raw MIDI byte (very verbose)
```

---

## Tempest Setup

Before using the MCP server, configure the Tempest MIDI settings.

**SysEx cable (required for USB transfer):**

1. Press **System** → navigate to **MIDI System Exclusive**
2. Set **MIDI: Sysex IN-OUT Cable** → **USB**

**MIDI channel (required for pad triggering):**

3. Press **System** → navigate to **MIDI Remote Pad Play**
4. Set **Remote Pad IN Channel** → **10** (or match your `channel` config)
5. Set **Remote Pad OUT Channel** → **10**

> **Note:** The Tempest cannot be queried programmatically. All SysEx dumps
> must be triggered manually from the front panel using **Save/Load**.

---

## Command Line Options

```
tempest-mcp [flags]

Flags:
  --config string     Config file path (default: ~/.config/tempest-mcp/config.yaml)
  --device string     MIDI device name substring (overrides config)
  --channel int       MIDI channel 1–16 (overrides config)
  --debug             Enable debug logging
  --midi-trace        Log every raw MIDI byte (very verbose)
  --list-ports        Print available MIDI ports and exit
```

---

## MCP Tools

### Transport

| Tool | Description |
|---|---|
| `tempest_start` | Send MIDI Start — begin sequencer playback |
| `tempest_stop` | Send MIDI Stop and halt the internal clock |
| `tempest_continue` | Send MIDI Continue — resume from current position |
| `tempest_set_tempo` | Start the internal clock at a given BPM (24 PPQN) |

### Pad Triggering

| Tool | Description |
|---|---|
| `tempest_trigger_pad` | Trigger a named pad: `kick`, `snare`, `closed-hat`, `open-hat`, `clap`, `ride`, `crash`, `a1`–`a16`, etc. |
| `tempest_trigger_note` | Trigger a raw MIDI note number |
| `tempest_play_sequence` | Play a timed sequence of hits from a JSON event list |

**Example:**

> *"Play a four-on-the-floor kick pattern with snare on 2 and 4 at 120 BPM"*

Claude calls `tempest_set_tempo` then `tempest_play_sequence` with the
appropriate events.

### Beat FX (CC Control)

Affects all voices simultaneously. Requires Tempest OS 1.3.1.6+.

| Tool | Description |
|---|---|
| `tempest_set_cc` | Send a raw Beat FX CC (12, 13, or 19–27) |
| `tempest_set_beat_fx` | Send a named parameter: `distortion`, `compression`, `lp-cutoff`, `lp-resonance`, `env-attack`, `env-decay`, `all-osc-freq`, `feedback`, `hp-cutoff`, etc. |

### Sound Library

Your local `.syx` files at `~/Tempest` are indexed automatically on startup.

| Tool | Description |
|---|---|
| `tempest_list_sounds` | List indexed sounds with optional filter |
| `tempest_search_sounds` | Fuzzy search by name, folder, or tag |
| `tempest_describe_sound` | Full metadata for a sound by ID |
| `tempest_load_sound` | Send a sound to the Tempest; optionally target a specific bank (A/B) and slot (1–16) |
| `tempest_show_bank_map` | Display which sounds are loaded into each hardware bank slot |
| `tempest_index_library` | Rescan `~/Tempest` and rebuild the index |

**Example:**

> *"Find an analog 808 kick from my library and load it to Bank A Slot 3"*

Claude calls `tempest_search_sounds` then `tempest_load_sound` with
`bank="A"` and `slot=3`. The assignment is saved to the index and shown in
`tempest_show_bank_map`.

### Sound Design

| Tool | Description |
|---|---|
| `tempest_morph_sound` | Blend two library sounds by parameter-byte interpolation; `mix=0.0` → 100% A, `mix=1.0` → 100% B, `mix=0.5` → equal blend. Saves a `.syx` file and optionally sends to the Tempest |
| `tempest_create_sound` | Create a blank sound initialised to Tempest reference-signature defaults; saves a `.syx` file and optionally sends to the Tempest |

**Example:**

> *"Morph my 808 kick with the Basic Kick at 30% blend and call it 'Hybrid'"*

Claude calls `tempest_morph_sound` with `mix=0.3` and saves the result to
your library.

### SysEx Dumps

> **Important:** The Tempest cannot respond to programmatic dump requests.
> Trigger dumps manually from the **Save/Load** button on the hardware, then
> call the MCP tool within the timeout window.

**To dump a sound currently in the edit buffer (RAM):**

1. On the Tempest: press **Save/Load** → choose **Export Sound in RAM over
   MIDI** → press **Next** → set destination to **USB** → press **Export Now**
2. Ask Claude: *"Save the dump I just sent to ~/Tempest/MyKick.syx"*

**To dump a project:**

1. In **16 Beats** mode: press **Save/Load** → choose **Export Project in
   RAM over MIDI** → press **Next** → destination **USB** → **Export Now**
2. Ask Claude: *"Extract all the sounds from that project dump"*

| Tool | Description |
|---|---|
| `tempest_wait_for_dump` | Wait for an incoming SysEx message and return a summary |
| `tempest_save_received_dump` | Wait for a dump and save it to a `.syx` file |
| `tempest_send_syx_file` | Send a `.syx` file to the Tempest (with 1 s inter-message pause) |
| `tempest_extract_sounds_from_project` | Wait for a project dump and extract individual sounds to `.syx` files |

### Utilities

| Tool | Description |
|---|---|
| `tempest_ping` | Check connection status and library sound count |
| `tempest_list_ports` | Show all MIDI ports available on this computer |
| `tempest_list_pad_names` | Show all recognised pad names |
| `tempest_set_channel` | Change the MIDI channel for this session |

---

## beat-mapper — SysEx Research CLI

### The problem

Two planned MCP tools are blocked because the DSI Tempest's internal byte
layout has never been publicly documented:

**Beat pattern writing** (`tempest_decode_project_beats`, `tempest_write_beat`,
`tempest_clear_beat`) requires knowing exactly which bytes inside the project
dump (0x61) represent each step's gate flag and velocity, and where each
track's data begins. Without `BeatDataOffset`, `TrackStride`, and
`StepStride`, there is no way to read or write a beat without corrupting the
entire project.

**Named sound parameter editing** (`tempest_read_sound_params`,
`tempest_set_sound_param`) requires a table mapping each synthesis parameter
— LP cutoff, envelope decay, oscillator pitch, etc. — to its byte offset in
the 132-byte parameter block. Neither KnobKraft Orm nor any published Tempest
document contains this table.

Both features require the same research method: **capture two dumps that
differ by exactly one known hardware change, unescape the payload, diff the
bytes, and record the offset.**

### The method

The Tempest's project dump is encoded using the Tempest 7+1 SysEx scheme (7
data bytes + 1 mystery byte, repeated). Before any comparison can be done,
the wire bytes must be unescaped to recover the raw data payload. Once
unescaped, a one-change diff produces at most a handful of changed bytes —
the position of those bytes *is* the offset table.

`beat-mapper` automates this workflow. It has no MIDI dependency; it works
entirely on `.syx` files captured to disk with `tempest_save_received_dump`.

> **OS 1.4 format note:** The Tempest's internal project/beat file format
> changed with OS 1.4 and is not backward-compatible. All beat-mapper captures
> must be done on a Tempest running **OS 1.4 or later** to produce bytes that
> match the format the code targets. Verify your OS version at
> **System → System Actions → Show System Information** before starting any
> capture session.
>
> **Beat dump alternative:** The Tempest supports exporting a single beat via
> **Save/Load → Export Beat in RAM over MIDI → Next → USB → Export Now**. This
> produces a smaller SysEx message than a full project dump (~1/16 the size),
> which makes diffs faster to read. The message type byte for this command
> was not previously known; a third-party editor's source (see
> [docs/sysex-tempest-format.md](docs/sysex-tempest-format.md#1-message-types))
> strongly suggests it is **`0x5F`** (the type byte that editor's own
> "export as .syx" feature writes for a single beat — the other candidate,
> `0x62`, looks like that editor's own save-file wrapper format rather than
> the hardware wire format) — **still unconfirmed against real hardware.**
> Determine it for certain by capturing one beat dump and
> checking `raw[3]` (the byte after `F0 01 28`). Once confirmed, add it as
> `TypeBeatDump` in `internal/sysex/message.go` and extend `beat-mapper
> unescape` to handle it — also see that doc's §5 for a candidate
> `BeatDataOffset` (1012) to seed the search.

### Build

```bash
go build -o beat-mapper ./cmd/beat-mapper
```

### Commands

#### `unescape` — extract raw payload

```bash
beat-mapper unescape baseline.syx [--out baseline.raw]
```

Reads a `.syx` project dump, finds the first 0x61 message, unescapes it with
the Tempest 7+1 codec, and writes the raw binary. Use this as a manual
inspection helper — open the output in a hex editor (`xxd baseline.raw | less`)
to browse the full payload visually.

#### `diff` — compare two dumps

```bash
beat-mapper diff baseline.syx kick_a1_s1.syx [--label "kick A1 step1"]
```

Unescapes both files and prints every position where they differ:

```
Diff: baseline.syx vs kick_a1_s1.syx
  offset 0x01A3  baseline=0x00  changed=0x64  delta=+100
  offset 0x01A4  baseline=0x00  changed=0x01  delta=+1
2 byte(s) differ out of 4096
```

`--label` tags the diff in output for annotation later. If exactly two bytes
change, the smaller offset is likely the velocity and the larger is the gate
flag — confirm with the velocity-variation capture.

#### `annotate` — labelled hex dump

```bash
beat-mapper annotate capture.syx --map offsets.json
```

Prints an annotated hex dump of the unescaped payload. `offsets.json` maps
hex-offset strings to human-readable labels, built up incrementally from diff
sessions:

```json
{"0x01A3": "A1 step1 velocity", "0x01A4": "A1 step1 gate"}
```

#### `session` — batch diff + stride inference

```bash
beat-mapper session ./captures/
```

Processes all `.syx` files in the directory against `baseline.syx`. Names
must follow the `<instrument>_<bank><track>_s<step>.syx` convention (e.g.
`kick_a1_s1.syx`). Infers step stride, track stride, and beat data offset,
then prints a Go `const` block ready to paste into
`internal/pattern/offsets.go`:

```go
// Auto-generated by beat-mapper session — verify before use
const (
    BeatDataOffset   = 0x0050
    TrackStride      = 0x????
    StepStride       = 0x????
    StepGateByte     = 0
    StepVelocityByte = 1
)
```

### Steps to unlock beat pattern writing

Complete these steps in order. Do not skip ahead — each step depends on the
previous.

#### Step 1 — Capture project dumps from hardware

Set up a blank beat on the Tempest (all steps silent, all tracks clear). Use
`tempest_save_received_dump` to save each capture.

| File | What to program before dumping |
|---|---|
| `baseline.syx` | Empty beat — all steps silent, all tracks clear |
| `kick_a1_s1.syx` | Kick on track A1, step 1 only, velocity 100 |
| `kick_a1_s2.syx` | Kick on track A1, step 2 only (step stride) |
| `kick_a2_s1.syx` | Kick on track A2, step 1 only (track stride) |
| `kick_b1_s1.syx` | Kick on bank B track 1, step 1 (validates bank B region) |
| `kick_a1_s1_v64.syx` | Kick on A1 step 1, velocity 64 (confirms velocity byte) |
| `kick_4otf.syx` | Kick on A1 steps 1, 5, 9, 13 (four-on-the-floor validation) |

Files 1–4 are the minimum to compute both strides. Files 5–7 validate and
should confirm the model before any code is written.

#### Step 2 — Run the session command

```bash
mkdir ~/Tempest/captures/beat-research
# move the 7 captures into that directory, then:
beat-mapper session ~/Tempest/captures/beat-research/
```

Verify that `BeatDataOffset`, `StepStride`, and `TrackStride` all have real
values (not `0x????`). Cross-check by running `beat-mapper diff` manually on
the step-stride and track-stride pairs.

#### Step 3 — Create `internal/pattern/offsets.go`

Paste the `session` output into a new file:

```go
package pattern

// Beat layout constants — derived from beat-mapper session on hardware captures.
const (
    BeatDataOffset   = 0x????  // fill from beat-mapper session output
    TrackStride      = 0x????
    StepStride       = 0x????
    StepGateByte     = 0
    StepVelocityByte = 1
)
```

Do not proceed to Step 4 until this file contains real values verified against
hardware.

#### Step 4 — Implement and round-trip test `DecodeBeat` / `EncodeBeat`

Once `offsets.go` is filled, implement:

```go
func DecodeBeat(projectPayload []byte, slot int) (*Beat, error)
func EncodeBeat(projectPayload []byte, beat *Beat) ([]byte, error)
```

Add `TestDecodeBeat_roundtrip`: decode a captured fixture → re-encode →
compare bytes → must be byte-for-byte identical. **The round-trip test must
pass before any write tool is built.**

#### Step 5 — Implement `SpliceBeat` and `tempest_decode_project_beats`

```go
// SpliceBeat replaces beat.Slot in a raw project dump, re-encodes with 7+1,
// and returns the modified dump ready to send.
func SpliceBeat(rawProjectDump []byte, beat *Beat) ([]byte, error)
```

Add `tempest_decode_project_beats` (read-only) first. Validate on hardware
before adding any write tools.

#### Step 6 — Add `tempest_write_beat` and `tempest_clear_beat`

Only after `tempest_decode_project_beats` has been validated on real hardware.

> **Warning:** Sending a modified project dump overwrites all 16 beats and all
> sounds on the Tempest simultaneously. Always save a backup dump before
> calling `tempest_write_beat`. The Tempest cannot confirm receipt.

### Steps to unlock named sound parameter editing

The same diff technique applies to sound parameter bytes. The `beat-mapper
diff` command works on FLASH (0x63) or RAM (0x60) dumps as well as project
dumps.

#### Capture protocol (15–20 single-param changes)

1. Load a factory sound to the Tempest edit buffer.
2. Dump via **Save/Load → Export Sound in RAM over MIDI → USB**. Save as
   `sound_baseline.syx`.
3. Change exactly one parameter (e.g. LP Cutoff from 64 to 74).
4. Dump again. Save as `sound_lp_cutoff_74.syx`.
5. Run:

   ```bash
   beat-mapper diff sound_baseline.syx sound_lp_cutoff_74.syx --label "lp_cutoff +10"
   ```

6. The differing byte offset is `lp_cutoff`. Record it in `offsets.json`.
7. Repeat for each parameter.

---

## Testing

```bash
go test ./...
```

All packages have test coverage. The test suite runs without hardware — no
MIDI connection is required.

---

## Troubleshooting

**"no MIDI output matching Tempest"**
Run `./tempest-mcp --list-ports` (or `make list-ports`) and note the exact
port name. Update `--device` or `device_name` in your config to match.

**"Tempest not connected"**
Ensure the Tempest is powered on and USB is connected before starting Claude
Desktop. The server auto-reconnects on each tool call.

**SysEx dump times out**
The default wait is 30 seconds. Press **Save/Load** on the Tempest and choose
the appropriate export option *before* or *immediately after* calling the MCP
tool.

**Beat FX CCs have no effect**
Verify the Tempest is running OS 1.3.1.6 or later. CC support was added in
that release. To check: press **System → System Actions → Show System
Information**.

**Library shows 0 sounds**
Confirm your `.syx` files are in `~/Tempest` (or the path in your config).
Call `tempest_index_library` to force a rescan.

**Binary not found after `make install`**
Ensure `~/.local/bin` is in your `PATH`. Add
`export PATH="$HOME/.local/bin:$PATH"` to your `~/.zshrc` and open a new
terminal.

---

## Project Structure

```
tempest-mcp/
├── cmd/
│   ├── tempest-mcp/main.go          MCP server entry point, CLI flags
│   └── beat-mapper/                 Standalone research CLI (no MIDI dependency)
│       ├── main.go                  Four subcommands: unescape, diff, annotate, session
│       └── mapper/                  Library: diff, stride inference, hex annotation
├── internal/
│   ├── config/config.go             YAML config load/save with defaults
│   ├── midi/
│   │   ├── device.go                USB MIDI port management, SysEx fan-out
│   │   ├── notes.go                 Pad name map, NoteOn/Off, sequence playback
│   │   ├── transport.go             Start/Stop/Continue, 24 PPQN clock goroutine
│   │   └── cc.go                    Beat FX CC sends and name table
│   ├── sysex/
│   │   ├── encoding.go              7+1 and standard DSI 7-of-8 codecs
│   │   └── message.go               Message type detection, parsing, fingerprinting
│   ├── library/
│   │   ├── index.go                 .syx file scanner, JSON index, bank slot tracking
│   │   └── search.go                Fuzzy search
│   ├── pattern/
│   │   └── pattern.go               Beat/Step/Track data structures, step-grid notation
│   ├── sound/
│   │   ├── sound.go                 Morph() — parameter-byte interpolation
│   │   └── defaults.go              DefaultBlankParams() reference signature
│   └── server/server.go             MCP tool registration and all handlers
├── go.mod
└── Makefile
```

---

## SysEx Format Notes

The Tempest uses two encoding schemes depending on message type:

| Message type | Code | Encoding |
|---|---|---|
| RAM sound (edit buffer) | 0x60 | Tempest 7+1 (mystery byte) |
| Project dump | 0x61 | Tempest 7+1 (mystery byte) |
| FLASH sound | 0x63 | Tempest 7+1 (mystery byte) |
| Bank sound (bulk dump) | 0x5C | Standard DSI 7-of-8 MSB |
| Bank header | 0x5E | Standard DSI 7-of-8 MSB |

The **Tempest 7+1 scheme** groups data as 7 bytes + 1 unknown "mystery" byte
(written as 0x00 on re-encode; the Tempest accepts this). This is unique to
the Tempest — all other DSI/Sequential instruments use the standard 7-of-8
MSB scheme.

Sound names are null-terminated ASCII at the start of the unescaped payload.
Factory sounds use `/S/Category/Name` prefixes (e.g. `/S/Kicks/Basic`).

Format details were reverse-engineered from KnobKraft Orm (Christof Ruch,
2022).

> **Update:** a second, independent reverse-engineering source —
> [TempestEdit](https://www.bitrotten.com/tempest/editor/) (an unofficial
> browser-based Tempest editor) and a companion
> [SysEx bit map](https://gist.github.com/fadeddata/c39a3b4b10e1e51af58e49ef74aca116) —
> describes the container format differently in several important ways:
> a leading "collector" byte carrying MSBs (not a discardable mystery byte),
> two additional message types (`0x5F` for a standalone Beat export and
> `0x62` for a Beat "file" variant), a bit-packed (not null-terminated) name
> field in RAM sound bodies, and concrete offsets for a Beat/Kit container
> (BPM, swing, name, 32-entry pad table, sequencer region). None of this has
> been verified against this repo's own hardware captures yet. Full details,
> the complete parameter bit map, and specific discrepancies with the
> encoding in `internal/sysex/` are in
> **[docs/sysex-tempest-format.md](docs/sysex-tempest-format.md)** — read it
> before touching `internal/sysex/encoding.go` or resuming the beat-mapper
> research below.

---

## License

MIT
