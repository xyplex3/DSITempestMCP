# Tempest MCP Server

Control your DSI/Sequential Tempest analog drum machine with Claude through
the Model Context Protocol. Once installed, Claude can trigger pads, run the
sequencer, search and load your sound library, design new sounds, and send or
receive SysEx dumps - all from a conversation.

## Features

- **Pad triggering** - trigger named pads (`kick`, `snare`, `closed-hat`, …)
  or raw MIDI note numbers with velocity and duration
- **Sequence playback** - play a timed JSON event list at any BPM with
  NoteOn/Off scheduling
- **Beat FX CC control** - set distortion, compression, LP/HP filter, envelope
  parameters, and more by name or CC number
- **Sound library** - scan `~/Tempest`, full-text search, and load sounds to
  specific bank/slot assignments tracked in a JSON index
- **Sound design** - morph two library sounds by byte-level parameter
  interpolation; create blank sounds from the Tempest reference signature
- **SysEx I/O** - wait for incoming dumps, save them to disk, send `.syx`
  files back to the hardware, extract individual sounds from project dumps
- **Beat research** - `beat-mapper` CLI computes step and track strides from
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

### 4. Register with your MCP client

Every client below uses `/Users/YOUR_USERNAME/.local/bin/tempest-mcp` as the
command; replace `YOUR_USERNAME` with your macOS username, and substitute the
`--device` value if your port name differs. Each `make ...-config` target
prints a ready-to-paste snippet using your actual install path.

**Claude Desktop**

Open (or create) the config file:

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

```bash
make claude-config
```

**Cursor**

Open (or create) `~/.cursor/mcp.json` for every project, or `.cursor/mcp.json`
inside one project for just that project. Same `mcpServers` shape as Claude
Desktop:

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

```bash
make cursor-config
```

**OpenCode**

Open (or create) `~/.config/opencode/opencode.json` for every project, or
`opencode.json` inside one project for just that project. OpenCode's `mcp`
block takes the full command (binary + args) as a single array:

```json
{
  "mcp": {
    "tempest": {
      "type": "local",
      "command": ["/Users/YOUR_USERNAME/.local/bin/tempest-mcp", "--device", "Tempest"],
      "enabled": true
    }
  }
}
```

```bash
make opencode-config
```

### 5. Restart your MCP client

Quit and reopen Claude Desktop / Cursor, or reload OpenCode. The Tempest
tools appear in the client's tool list. Test with:

> *"Call tempest_ping"*

The model responds with the connected device name and library sound count.

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
| `tempest_start` | Send MIDI Start - begin sequencer playback |
| `tempest_stop` | Send MIDI Stop and halt the internal clock |
| `tempest_continue` | Send MIDI Continue - resume from current position |
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
| `tempest_read_sound_params` | Decode a Sound (0x60) dump's named synthesis parameters, from a saved `.syx` or a live dump |

### Utilities

| Tool | Description |
|---|---|
| `tempest_ping` | Check connection status and library sound count |
| `tempest_list_ports` | Show all MIDI ports available on this computer |
| `tempest_list_pad_names` | Show all recognised pad names |
| `tempest_set_channel` | Change the MIDI channel for this session |

---

## beat-mapper - SysEx Research CLI

### The problem

One planned MCP tool is still blocked, though real progress has been made on
the DSI Tempest's undocumented sequencer byte layout:

**Beat pattern writing** (`tempest_decode_project_beats`, `tempest_write_beat`,
`tempest_clear_beat`) requires knowing exactly how a beat's active notes are
stored in a Beat/Kit (0x5F) export. The **single-note case is confirmed**
(`docs/sysex-tempest-format.md` §9): each active note is an 80-bit (10-byte)
record starting at absolute unpacked-payload byte 1077, with a confirmed step
position, a confirmed track-identity byte (`0x80 | 0-based track index`, not
a positional offset the way earlier sessions assumed), and a known-but-noisy
velocity byte. **The container format is now confirmed capable of holding
multiple notes** - three existing captures show two different tracks as
clean, correctly-formed consecutive 10-byte records (see §9.7). But the
obvious follow-up guess ("same/adjacent step is what makes it work") doesn't
hold up: one of those three has the same step-delta as the original failing
test, so step-delta alone isn't the distinguishing factor, and none of the
three were captured under today's verified-clean methodology - they may not
even be controlled tests. Until a real controlled test isolates what
actually causes note loss, still too risky to implement multi-note writing.

**Named sound parameter reading** (`tempest_read_sound_params`) is now
available: the parameter offset table (`internal/sysex/soundparams.go`) was
mechanically translated from a community bit map and spot-checked against 3
hardware captures (Pitch Env Attack, LP Env Release, AD Mode; see
`docs/sysex-tempest-format.md` §7/§8). Most of its 121 parameters have not
been individually re-verified, and there is no write path
(`tempest_set_sound_param`) yet, so treat its output as a strong lead, not a
certainty, until more parameters are hardware-confirmed.

The beat-pattern research requires the same method used to validate the
sound-parameter table: **capture two dumps that differ by exactly one known
hardware change, unescape the payload, diff the bytes, and record the
offset.**

### The method

The Tempest's project dump is encoded using the Tempest 7+1 SysEx scheme (7
data bytes + 1 mystery byte, repeated). Before any comparison can be done,
the wire bytes must be unescaped to recover the raw data payload. Once
unescaped, a one-change diff produces at most a handful of changed bytes -
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
> which makes diffs faster to read. The message type byte for this command is
> **`0x5F`**, added as `TypeBeatDump` in `internal/sysex/message.go` - confirmed
> by decoding real 0x5F `.syx` files already in this user's library: the Kit
> name and BPM fields (`sysex.ExtractName`, `sysex.KitBPM`, `sysex.KitSwing`)
> decode byte-exact against the files' known contents. See
> [docs/sysex-tempest-format.md](docs/sysex-tempest-format.md#1-message-types)
> for details. `KitSequencerOffset` (1012) marks where the pad table ends and
> the note-record region begins; the single-note record format past that
> offset is now confirmed (§9) using `beat-mapper bitdiff` below, not the
> `beat-mapper session` stride-search workflow this section originally
> described - that workflow assumed a fixed per-track/per-step grid, which
> turned out not to match how the hardware actually stores notes.

### Build

```bash
go build -o beat-mapper ./cmd/beat-mapper
```

### Commands

#### `unescape` - extract raw payload

```bash
beat-mapper unescape baseline.syx [--out baseline.raw]
```

Reads a `.syx` project dump, finds the first 0x61 message, unescapes it with
the Tempest 7+1 codec, and writes the raw binary. Use this as a manual
inspection helper - open the output in a hex editor (`xxd baseline.raw | less`)
to browse the full payload visually.

#### `diff` - compare two dumps

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
flag - confirm with the velocity-variation capture.

#### `bitdiff` - find a non-byte-aligned insertion or deletion

```bash
beat-mapper bitdiff baseline.syx kick_a1_s1.syx [--max-shift N] [--show-scan]
```

`diff` can only find changes that land on the same byte offset in both files.
A bit-packed region (like the sequencer past `KitSequencerOffset`) can shift
everything downstream by a few bits instead of a whole byte when one note is
added, which makes `diff` show near-total divergence from that point on even
though nothing conceptually changed. `bitdiff` searches bit-by-bit instead:
it finds the longest identical bit-aligned prefix, then searches a window of
candidate shifts for the one that best realigns the remaining content.

```
Bit diff: baseline.syx vs kick_a1_s1.syx
  baseline: 41440 bits (5180 bytes)
  changed:  41496 bits (5187 bytes)
  common prefix: 8616 bits (1077 bytes + 0 bits)
  best shift: +80 bits (10.00 bytes) - changed has extra content at bit offset 8616
  163/32800 bits mismatch at that shift
  SHARP boundary (not perfectly clean, but 65x below the typical mismatch rate at a wrong shift): likely a real insertion/deletion, with residual mismatches probably from an unrelated field elsewhere in the payload

Inserted content (80 bits), in kick_a1_s1.syx but not baseline.syx:
  binary: 01100000 00000000 00000000 11101110 00000001 11010000 01000000 00000000 00000000 00000000
  packed hex: 06 00 00 77 80 0B 02 00 00 00
```

A real hardware capture almost never produces an exact zero-mismatch match
(some unrelated field, like a note count, can legitimately differ too), so
look for a **sharp** drop in mismatch rate rather than a literal zero -
`--show-scan` prints the mismatch count for every shift tried so you can see
the dip yourself. `--max-shift` widens the search window (bits, each
direction) if the true shift is larger than the default.

#### `annotate` - labelled hex dump

```bash
beat-mapper annotate capture.syx --map offsets.json
```

Prints an annotated hex dump of the unescaped payload. `offsets.json` maps
hex-offset strings to human-readable labels, built up incrementally from diff
sessions:

```json
{"0x01A3": "A1 step1 velocity", "0x01A4": "A1 step1 gate"}
```

#### `session` - batch diff + stride inference

```bash
beat-mapper session ./captures/
```

Processes all `.syx` files in the directory against `baseline.syx`. Names
must follow the `<instrument>_<bank><track>_s<step>.syx` convention (e.g.
`kick_a1_s1.syx`). Infers step stride, track stride, and beat data offset,
then prints a Go `const` block ready to paste into
`internal/pattern/offsets.go`:

```go
// Auto-generated by beat-mapper session - verify before use
const (
    BeatDataOffset   = 0x0050
    TrackStride      = 0x????
    StepStride       = 0x????
    StepGateByte     = 0
    StepVelocityByte = 1
)
```

### Steps to unlock beat pattern writing

This section previously described a stride-search workflow (`beat-mapper
session`, computing a fixed `TrackStride`/`StepStride` grid). That model
turned out to be wrong: track identity isn't a storage position at all, it's
a *value* inside a self-contained per-note record (see
[docs/sysex-tempest-format.md §9](docs/sysex-tempest-format.md#9-confirmed-the-note-record-format-and-track-identity-2026-09-21-session-3)).
The steps below reflect the current, corrected understanding. Complete them
in order - each depends on the previous.

#### Step 1 - Resolve multi-note capture (currently blocking, see §9.4/§9.5)

The single-note case is done (Step 2 below). What's not yet done is capturing
more than one note in the same beat without losing one of them - tried three
times, always losing whichever note was added first. Before writing any
decoder code:

1. Fresh `Initialize Beat`, then one note on track A1 step 1 via **16
   Sounds** → tap A1 → **16 Time Steps** → tap step 1 (confirm on screen).
2. **16 Sounds** → tap A2 → **16 Time Steps** → tap step 2, again confirming
   on screen that A2 is actually selected and the beat number/name hasn't
   changed.
3. In **Save/Load**, double-check the menu says **Export Beat in RAM over
   MIDI**, not Export Project - the two are adjacent and easy to mix up (this
   derailed the last attempt at this exact test).
4. Capture with `cmd/capture-tmp` (`capture-tmp out.syx 2 30
   baseline.syx`), which verifies note count from raw file size
   (`5925 + 8×N` bytes) before you trust the result.

If that still loses a note, try the Tempest's **Beat Events** screen
(`Events` key - row/column soft knobs, explicit Insert/delete) instead of
pad-taps, a different input path that hasn't been tried yet. See docs §9.4
for the full history and §9.5 for a related open question about whether
Export Project reflects live state.

#### Step 2 - Confirmed: single-note record format (done, see §9.1-9.3)

Using [`beat-mapper bitdiff`](#bitdiff---find-a-non-byte-aligned-insertion-or-deletion)
against a confirmed-zero-notes baseline, a single active note is an **80-bit
(10-byte) record** starting at absolute unpacked-payload byte 1077:

```go
package pattern

// Sequencer note-record layout - confirmed for exactly one active note,
// see docs/sysex-tempest-format.md §9. Multi-note layout (§9.4/§9.5) is
// still unresolved - do not assume these offsets repeat per note yet.
const (
    SequencerOffset    = 1012 // KitSequencerOffset: pad table ends, note records begin
    NoteRecordBits      = 80  // one active note's record width
    NoteRecordOffset    = 1077 // absolute byte offset of the record (single-note case)

    // Byte offsets within one note record, relative to NoteRecordOffset:
    RecordStepPosByte   = 2 // step_index * 3 (confirmed §7.4)
    RecordTrackByte     = 4 // 0x80 | 0-based track index (confirmed A1-A3, §9.3)
    RecordVelocityByte  = 5 // noisy/tap-driven (confirmed location, not exact formula)
    // bytes 0, 1, 3, 6-9: constant in every single-note capture so far,
    // meaning unknown - see §9.2.
)
```

#### Step 3 - Implement and round-trip test `DecodeBeat` / `EncodeBeat`

Only once Step 1 is resolved and the multi-note layout is confirmed (not
before - building this on an unverified assumption about how records repeat
risks corrupting real beats). Implement:

```go
func DecodeBeat(kitPayload []byte) (*Beat, error)
func EncodeBeat(kitPayload []byte, beat *Beat) ([]byte, error)
```

Add `TestDecodeBeat_roundtrip`: decode a captured fixture → re-encode →
compare bytes → must be byte-for-byte identical. **The round-trip test must
pass before any write tool is built.**

#### Step 4 - Add `tempest_decode_project_beats` (read-only)

Validate on hardware before adding any write tools.

#### Step 5 - Add `tempest_write_beat` and `tempest_clear_beat`

Only after `tempest_decode_project_beats` has been validated on real
hardware.

> **Warning:** Sending a modified Beat/Kit dump risks overwriting the current
> beat on the Tempest. Always save a backup dump before calling
> `tempest_write_beat`. The Tempest cannot confirm receipt.

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

All packages have test coverage. The test suite runs without hardware - no
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
│   │   ├── sound.go                 Morph() - parameter-byte interpolation
│   │   └── defaults.go              DefaultBlankParams() reference signature
│   └── server/server.go             MCP tool registration and all handlers
├── go.mod
└── Makefile
```

---

## SysEx Format Notes

The Tempest uses one encoding scheme across every recognised message type:

| Message type | Code |
|---|---|
| RAM sound (edit buffer) | 0x60 |
| Project dump | 0x61 |
| FLASH sound | 0x63 |
| Bank sound (bulk dump) | 0x5C |
| Bank header | 0x5E |
| Beat/Kit dump | 0x5F |

Groups of 8 wire bytes are 1 leading **collector** byte followed by 7 data
bytes; bit *k* of the collector is the high bit of data byte *k*. FLASH
(0x63) and bank-sound (0x5C) messages carry one extra header byte before the
payload - for FLASH this is a name/path-length prefix, not a bank/slot (see
below).

Sound names are null-terminated ASCII at the start of the unescaped payload
for FLASH/Project; Beat/Kit names are a fixed-offset, space-padded 20-char
field (`sysex.KitNameOffset`). Factory sounds use `/S/Category/Name` prefixes
(e.g. `/S/Kicks/Basic`).

> **Confirmed 2026-09-19:** this collector-first scheme, `TypeBeatDump =
> 0x5F`, and the FLASH path-length header replace an earlier, unverified pair
> of schemes (a discardable "mystery byte" model, and a wrong-byte-order MSB
> model) that didn't match real hardware. Confirmation came not from a live
> capture session - the Tempest wasn't reachable over USB at the time - but
> from decoding ~500 real hardware-captured `.syx` files already present in
> this user's `~/Tempest` library: FLASH dumps decode to exact
> `/S/Category/Name` paths, and 0x5F dumps decode to exact names and BPM
> values, matching their known contents byte-for-byte.
>
> **One behavioural change this implies:** FLASH's 5th header byte is a
> name-length, not a bank/slot destination - the Tempest does not appear to
> accept a target slot over SysEx at all. `tempest_load_sound`'s `bank`/`slot`
> arguments now only record the intended assignment in the local library
> index (for `tempest_show_bank_map`); select the actual destination slot on
> the Tempest's own Save/Load prompt when the dump arrives.
>
> Original sources: [TempestEdit](https://www.bitrotten.com/tempest/editor/)
> (an unofficial browser-based Tempest editor) and a companion
> [SysEx bit map](https://gist.github.com/fadeddata/c39a3b4b10e1e51af58e49ef74aca116),
> cross-checked against this repo's prior baseline, KnobKraft Orm (Christof
> Ruch, 2022). Still unconfirmed: the RAM (0x60) bit-packed name field, the
> 0x5C/0x5E scheme specifically (assumed uniform with the rest, not
> independently decoded), and everything past `KitSequencerOffset`
> (step/track/gate data - needs a real `beat-mapper session` capture run, not
> just existing files). Full details in
> **[docs/sysex-tempest-format.md](docs/sysex-tempest-format.md)**.

---

## Firmware Analysis

A separate, related research thread: what's known about the Tempest's own
internal firmware (its four processors' architectures, chip identities, and
an unresolved memory-layout question blocking deeper analysis) rather than
the SysEx wire protocol. Not required reading to use this repo - see
**[docs/firmware-analysis.md](docs/firmware-analysis.md)** if useful,
motivated by strings found in the firmware ("Failed to read sequence data")
that are directly relevant to the still-open Export Beat question above.

---

## License

MIT
