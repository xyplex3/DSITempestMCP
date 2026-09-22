# Tempest SysEx Format - Research Notes

This document collects everything currently known about the DSI/Sequential
Tempest's SysEx protocol, gathered from third-party reverse-engineering
(DSI never published a spec). It supplements the summary in the main
[README](../README.md#sysex-format-notes).

**Status: §1–§3 and part of §5 are now confirmed** - not by a live hardware
capture session (the Tempest wasn't reachable over USB when this was done),
but by decoding ~500 real hardware-captured `.syx` files already present in
this user's `~/Tempest` library. `internal/sysex/encoding.go` and
`message.go` have been updated to match:

- The collector-first unpack scheme (§3) is implemented and verified: real
  FLASH (0x63) dumps decode to exact `/S/Category/Name` paths, and real
  Beat/Kit (0x5F) dumps decode to exact names and BPM values.
- `TypeBeat = 0x5F` (§1) is added to `message.go` and confirmed.
- FLASH's 5th header byte (§2) is confirmed as a path-length prefix, not a
  bank slot; `Location()`/`BankSlot()` and the affected server.go send path
  were updated accordingly (see README's SysEx Format Notes for the
  behavioural implication on `tempest_load_sound`'s bank/slot targeting).
- The Beat/Kit BPM/swing/name fields (§5) are confirmed and exposed via
  `sysex.KitBPM`, `sysex.KitSwing`, and `ExtractName`.

**Update - live hardware session (2026-09-20):** the Tempest was reachable
over USB and a real `beat-mapper`-style capture session was run (see §7).
This independently reconfirmed `TypeBeat = 0x5F` via `raw[3]` on a live
capture (not just decoding pre-existing files), found and fixed a real crash
bug in the SysEx receive path, and made concrete progress inside
`KitSequencerOffset` - the step-position field is now confirmed. Track
stride and the exact note-record layout are still open; see §7 for what was
tried and why it's harder than expected.

**Session paused mid-work (2026-09-21) - read §8 before resuming.** §7.5's
track-identity conclusion did not survive a more careful re-test and should
be treated as unconfirmed; §8 has the corrected understanding, the
operational gotchas that caused it, a new capture-verification tool, and
exactly where to pick back up. There's also uncommitted work
(`internal/sysex/soundparams.go`, `cmd/capture-tmp/`) sitting in the working
tree - see §8.0.

Still **unconfirmed**: the RAM (0x60) bit-packed name field (§4) - brute-force
search across 30 real RAM captures found no bit offset that decodes cleanly;
the 0x5C/0x5E scheme specifically (assumed uniform with everything else per
TempestEdit's source, but not independently decoded from a real 0x5C/0x5E
capture the way FLASH/RAM/Beat were); and most of the sequencer region past
`KitSequencerOffset` - §7 confirms the step-position field but track stride
and the note-record format remain open. Before relying on any of the
still-unconfirmed material for a *write* path (`tempest_write_beat`,
`tempest_set_sound_param`, etc.), confirm it with a `beat-mapper diff`
capture session on real hardware, per the workflow in the README.

---

## Sources

- **TempestEdit** - <https://www.bitrotten.com/tempest/editor/>, an unofficial
  browser-based Tempest editor (Web MIDI, no server-side component). Its
  author states: "every parameter location, encoding, and container format
  this editor understands was reverse-engineered from scratch by capturing
  MIDI dumps, changing one thing at a time on a real unit, and comparing the
  results." Container-level facts below (message types, header layout, the
  escape scheme, Beat/Kit layout) were read directly out of the app's client-side
  `bundle.js`.
- **Tempest SysEx Bit Map** (gist) -
  <https://gist.github.com/fadeddata/c39a3b4b10e1e51af58e49ef74aca116>, a
  verified bit-level map of every synthesis parameter in a Sound (0x60) body,
  produced by the same kind of single-bit-change diffing that `beat-mapper`
  already automates in this repo.
- This repo's prior baseline, per the existing README, was **KnobKraft Orm**
  (Christof Ruch, 2022), which only covers the two encoding *schemes* in
  general DSI/Sequential terms, not Tempest-specific offsets.

---

## 1. Message types

`bundle.js` defines these SysEx type bytes (index 3 of the message, after
`F0 01 28`):

| Constant | Byte | What it is |
|---|---|---|
| `SOUND_EXPORT_TYPE` | `0x60` | Live RAM edit-buffer export - "Sound/Beat" (see note below) |
| `PROJECT_FILE_TYPE` | `0x61` | Complete project dump (16 beats + 32 sounds) |
| `KIT_EXPORT_TYPE` | **`0x5F`** | A single Beat, exported as raw hardware SysEx (32 pads + sound bodies + tempo/swing/name) |
| `BEAT_FILE_EXPORT_TYPE` | `0x62` | A single Beat, in TempestEdit's own "file" wrapper format |
| `FILE_EXPORT_TYPE` | `0x63` | FLASH sound (permanent bank slot) |
| `PROJECT_BEAT_MESSAGE_TYPE` | `0x5C` | Per-beat message inside a project bulk transfer |
| `PROJECT_PLAIN_EXPORT_TYPE` | `0x5E` | Project-level global/header export |

Compared to this repo's current constants in `internal/sysex/message.go`:

- `0x60`, `0x61`, `0x63` match (`TypeRAM`, `TypeProject`, `TypeFLASH`).
- **`0x5F` is a message type this repo does not know about at all.** It is
  very likely the answer to the long-standing "`TypeBeatDump` byte is
  unknown" TODO in `message.go` and the beat-mapper README section - it is
  the type TempestEdit itself uses for `exportBeatAsSyx()`, i.e. the raw
  on-the-wire form of a single Beat. **This still needs hardware
  confirmation**: capture a beat via **Save/Load → Export Beat in RAM over
  MIDI → USB → Export Now** and check `raw[3]`. It is expected to be `0x5F`;
  `0x62` is the second candidate (see below).
- This repo's `0x5C` (`TypeAltSound`, described as "Alternate bank sound")
  and `0x5E` (`TypeAltHeader`, "Alternate bank global header") carry the same
  byte values as TempestEdit's `PROJECT_BEAT_MESSAGE_TYPE` and
  `PROJECT_PLAIN_EXPORT_TYPE`, but the semantic labels differ (a per-beat
  message inside a project transfer, vs. a generic "bank sound"/"bank
  header"). The byte values likely agree by coincidence of context (both
  come from bulk/project transfers); the *meaning* attached to them in this
  repo's comments should be treated as unconfirmed.
- `0x62` (`BEAT_FILE_EXPORT_TYPE`) has no equivalent in this repo. TempestEdit
  treats `0x5F` and `0x62` as interchangeable inputs in several places
  (`type === KIT_EXPORT_TYPE || type === BEAT_FILE_EXPORT_TYPE`), which
  suggests `0x62` may be a save-file variant of the same Beat content rather
  than something the hardware emits directly - but this is inferred, not
  confirmed.

**Note on `0x60` covering both Sound and Beat:** TempestEdit's own
unrecognized-type error message groups `0x60` under the label "Sound/Beat".
This suggests the *live RAM* export (as opposed to the standalone Beat
export `0x5F`) may reuse the same `0x60` type byte for both a Sound and a
Beat currently in the edit buffer, disambiguated by payload length/shape
rather than by the type byte. Needs hardware confirmation.

---

## 2. Header structure

TempestEdit's `headerLen()`:

```js
function headerLen(bytes) {
  return (bytes[3] === FILE_EXPORT_TYPE ||        // 0x63
          bytes[3] === PROJECT_BEAT_MESSAGE_TYPE || // 0x5C
          bytes[3] === BEAT_FILE_EXPORT_TYPE)       // 0x62
    ? 5
    : 4;
}
```

So the header is **`F0 <mfg> <dev> <type>` (4 bytes)** for most types, or
**5 bytes** (one extra byte at index 4) for FLASH (`0x63`), the project-beat
message (`0x5C`), and the beat-file export (`0x62`).

Critically, for FLASH (`0x63`) that 5th byte is **not** a bank/slot location.
`parseTempestMessage()` reads it as `pathLen` - the length, in characters, of
a factory path string (e.g. `/S/Kicks/Basic`) that is the *first* thing
inside the unescaped payload:

```js
const isFile = bytes[3] === FILE_EXPORT_TYPE;
const pathLen = isFile ? bytes[4] : 0;
const raw = unpackPayload(bytes);
const path = isFile ? String.fromCharCode(...raw.slice(0, pathLen)).replace(/\0.*$/, '') : null;
body = raw.slice(pathLen, pathLen + BODY_LEN);   // BODY_LEN = 128
```

This directly contradicts `internal/sysex/message.go`'s `Location()`, which
reads `raw[4]` as a bank slot (0–31) for *every* message type, including
FLASH. For FLASH dumps, `raw[4]` is a **name length**, not a slot - meaning
`tempest_show_bank_map` and `BankSlot()` may be misinterpreting this byte for
FLASH sounds specifically. This needs verification (a FLASH dump captured
from a known bank/slot should be inspected byte-by-byte), since the
Tempest's FLASH export may not encode a destination slot in the dump at all
(slot assignment might happen purely via the front-panel Save/Load flow).

---

## 3. The actual escape/unescape scheme

This is the most significant discrepancy. TempestEdit's unpack/pack functions:

```js
function unpackPayload(bytes) {
  const raw = [];
  const end = bytes.length - 1;               // exclude trailing F7
  for (let i = headerLen(bytes); i < end; i += 8) {
    const collector = bytes[i];                // <-- first byte of each group of 8
    for (let k = 0; k < 7 && i + 1 + k < end; k++) {
      raw.push(bytes[i + 1 + k] | (((collector >> k) & 1) << 7));
    }
  }
  return raw;
}

function packPayload(raw, prefix, typeByte, extraHeaderByte) {
  const out = [prefix[0], prefix[1], prefix[2], typeByte];
  if (typeByte === FILE_EXPORT_TYPE || typeByte === PROJECT_BEAT_MESSAGE_TYPE) {
    out.push(extraHeaderByte);
  }
  for (let g = 0; g * 7 < raw.length; g++) {
    const group = raw.slice(g * 7, g * 7 + 7);
    let collector = 0;
    for (let k = 0; k < group.length; k++) collector |= ((group[k] >> 7) & 1) << k;
    out.push(collector);                        // collector byte FIRST
    for (let k = 0; k < 7; k++) out.push(k < group.length ? group[k] & 0x7F : 0);
  }
  out.push(0xF7);
  return new Uint8Array(out);
}
```

In plain terms: every group of 8 wire bytes is **1 leading "collector" byte,
then 7 data bytes**. The collector's bit *k* is the high bit (bit 7) of data
byte *k* in that group. This is applied uniformly to **every** message type:
there is no separate scheme for "bank" messages vs. everything else, and
there is no 8th "mystery" byte that gets discarded.

This contradicts both schemes currently implemented in
`internal/sysex/encoding.go`:

- **`Unescape7Plus1`/`Escape7Plus1`** (applied to `0x60`/`0x61`/`0x63` in this
  repo) treats the 8th byte of each group as a meaningless "mystery" byte and
  drops it on decode / writes `0x00` on encode. Per TempestEdit, that byte is
  not a mystery byte at all - it is a real MSB-collector carrying the high
  bit of the following 7 bytes. Dropping it discards real data (any
  parameter value ≥ 64, since bit 7 set corresponds to values 128–255 in an
  un-collapsed byte, or more precisely, the reconstructed bit for the
  corresponding data byte); the current code also never reconstructs values
  above 0x7F this way. **If this repo's `Unescape7Plus1` is run against a
  real capture, every 8th byte of meaning is silently lost, and no bit-7 gets
  restored into the 7 that remain.**
- **`UnescapeStandard`/`EscapeStandard`** (applied to `0x5C`/`0x5E`) uses the
  right *idea* (7 data bytes + 1 MSB byte) but the **opposite byte order**:
  this repo puts the 7 data bytes first and the MSB collector last
  (`msbByte := encoded[i+7]`), while TempestEdit's hardware-verified version
  puts the collector **first**, then the 7 data bytes.

**Net effect:** neither encoding function in this repo currently matches the
scheme TempestEdit found by diffing real hardware captures. This would
explain why sound-parameter and beat-pattern decoding has been blocked -
the raw bytes were plausibly being unpacked with an incorrect group layout
all along. Before trusting this, run `beat-mapper unescape` against a real
capture using both the current implementation and a `collector-first`
implementation, and check which one produces a payload whose name field
reads back as valid ASCII (see below) - that is a fast, self-verifying test.

---

## 4. Name field encoding (Sound / RAM body)

For a `0x60` Sound/Beat body, TempestEdit does **not** treat the name as
null-terminated ASCII at the start of the payload (as this repo's
`ExtractName` assumes for FLASH/Project, and explicitly skips for RAM).
Instead, the name is **bit-packed**, not byte-aligned, further into the body:

```js
const NAME_BIT_OFFSET = 880;     // bit offset into the unpacked (7-bit-per-slot) payload
const NAME_MAX_CHARS = 20;
const NAME_BITS_PER_CHAR = 7;
```

Each character occupies exactly 7 bits (standard ASCII range), starting at
bit 880 of the *unpacked* payload (i.e. after `unpackPayload` has already
reassembled full bytes from the collector scheme above) - not at a fixed
byte offset, and not null-terminated. Up to 20 characters, trimmed of
trailing spaces/padding.

For FLASH (`0x63`), by contrast, the name *is* a plain byte-aligned string -
but it is the `path` field described in §2 (`/S/Category/Name`, length-prefixed
by the 5th header byte), not a null-terminated field inside the parameter
block.

This means this repo's current name handling is likely wrong in two
different ways for two different message types:

- `ExtractName` returns `("", 0)` for RAM (`0x60`) - but RAM dumps do appear
  to carry a name, just bit-packed rather than a leading null-terminated
  string.
- `ExtractName`/`ExtractParams` treat FLASH/Project names as a leading
  null-terminated ASCII run - TempestEdit's model instead uses a
  length-prefixed path string only for FLASH, addressed via the 5th header
  byte rather than being embedded literally as bytes to scan for `0x00`.

---

## 5. Beat / Kit container layout

TempestEdit calls a Beat a "Kit" internally. Layout constants for a `0x5F`
Kit body (offsets into the *unpacked* payload, i.e. after the collector
scheme has been undone):

```js
const BEAT_BPM_OFFSET       = 4;    // 2 bytes, big-endian, value = raw / 10 (one decimal place)
const BEAT_SWING_OFFSET     = 6;    // 1 byte, 0-12 raw -> 50%-75% (linear)
const BEAT_NAME_OFFSET      = 24;
const BEAT_NAME_LEN         = 20;
const SHORT_NAME_OFFSET     = BEAT_NAME_OFFSET + BEAT_NAME_LEN;  // 44
const SHORT_NAME_LEN        = 8;
const KIT_PAD_TABLE_OFFSET  = SHORT_NAME_OFFSET + SHORT_NAME_LEN; // 52
const KIT_PAD_ENTRY_LEN     = 30;
const KIT_PAD_ENTRY_COUNT   = 32;
const KIT_PAD_TABLE_LEN     = KIT_PAD_ENTRY_LEN * KIT_PAD_ENTRY_COUNT; // 960
const KIT_SEQUENCER_OFFSET  = KIT_PAD_TABLE_OFFSET + KIT_PAD_TABLE_LEN; // 1012
```

BPM/swing decode:

```js
function readBeatBpm(kit) {
  return (kit.fixedHeader[4] * 256 + kit.fixedHeader[5]) / 10;
}
function readBeatSwing(kit) {
  return 50 + kit.fixedHeader[6] * (75 - 50) / 12;
}
```

This gives this repo's blocked "beat pattern writing" work (see README
`beat-mapper` section) real starting numbers instead of `0x????`:

- **`BeatDataOffset` is very likely `1012`** (`KIT_SEQUENCER_OFFSET`) - the
  step/gate/velocity sequencer data starts immediately after a 32-entry,
  30-byte-per-entry pad table (960 bytes), which itself starts at byte 52,
  after an 8-byte short name and a 20-byte long name, after a 4-byte
  BPM+swing region.
- TempestEdit's own editor **does not decode the sequencer region past
  `KIT_SEQUENCER_OFFSET`** - it edits pad→sound assignment, tempo, swing, and
  name, but treats step/gate/track data as an opaque tail it preserves
  byte-for-byte. `TrackStride` and `StepStride` are still unknown and still
  require this repo's own `beat-mapper session` capture workflow - but now
  the search only needs to start at byte 1012 instead of scanning the whole
  payload from zero.
- `KIT_PAD_ENTRY_LEN = 30` bytes per pad (×32 pads) is itself new
  information: each pad's entry (likely including its embedded sound
  reference/override and per-pad settings) is a fixed 30-byte record, not
  the 128-byte full sound body (`BODY_LEN`) used elsewhere. A pad entry is
  probably a reference/pointer plus a handful of per-pad parameters rather
  than a full embedded sound.

---

## 6. Full parameter bit map (Sound / RAM body, `0x60`)

The gist referenced in Sources provides a complete bit-level map of every
synthesis parameter in the 0x60 Sound body - oscillators, filter,
envelopes (Pitch/Filter/Amp/Aux1/Aux2), LFOs, mod matrix (8 slots), and
misc/global parameters. It was produced by the exact method `beat-mapper`
already automates: set a parameter to 0, capture; set it to 1, 2, 4, 8, 16,
32, 64, 128 in turn, capture each; diff against the zero-capture; each diff
isolates one bit.

Full table (mirrored here for durability - the gist could disappear or
change):

**Byte-range overview** (0-indexed within the full `F0 01 28 60 ... F7`
message, i.e. before running it through the corrected unpack scheme in §3 -
these are the raw wire-byte positions the gist author diffed directly):

```
Byte  Area                      Notes
----  ----                      -----
0-3   SysEx Header              F0 01 28 60 (fixed)
4     Osc 1/2 overflow          Fine, Shape, Glide bits from Osc 1 & 2
5     Osc 1 Frequency           (all 7 bits)
6     Osc 1 Fine/Shape          Fine bits 1-6, Shape bit 0
7     Osc 1 Shape/Glide         Shape bits 2-6, Glide bits 0-1
8     Osc 1 Glide/misc          Glide bits 3-6, KeyFollow, WaveReset, Osc2 Freq bit 0
9     Osc 2 Freq/Fine           Freq bits 2-6, Fine bits 0-1
10    Osc 2 Fine/Shape          Fine bits 3-6, Shape bits 0-2
11    Osc 2 Shape/Glide         Shape bits 4-6, Glide bits 0-3
12    Osc 3/4 overflow          Osc3 Freq.3, Osc3 Sample.5, Osc4 Freq.1, Osc4 Fine.2, Osc4 Sample.3
13    Osc 2/3 mixed             Osc2 Glide.5-6, KeyFollow, WaveReset, Osc3 Freq.0-2
14    Osc 3 Freq/Fine           Freq bits 4-6, Fine bits 0-4
15    Osc 3 Fine/Sample         Fine bits 5-6, Sample bits 0-4
16    Osc 3 Sample              Sample bits 6-8
17    Osc 3/4 misc              Osc3 KeyFollow, Osc4 Freq bit 0
18    Osc 4 Freq/Fine           Freq bits 2-6, Fine bits 0-1
19    Osc 4 Fine/Sample         Fine bits 3-6, Sample bits 0-2
20    Mixed overflow            Osc4 KeyFollow, PBRange, Mix, SubOsc, Osc4 Level, PrePost
21    Osc 4 Sample              Sample bits 4-8 (skips bit 3)
22    Unknown
23    Global Osc                Sync, GlideMode, OscSlop, PBRange bit 0
24    Osc Mix/PBRange           PBRange bits 2-3, Mix bits 0-4
25    Sub Osc/Mix               Mix bit 6, SubOsc bits 0-5
26    Osc 3 Level               (all 7 bits)
27    Osc 4 Level/PrePost       Level bits 1-6, PrePost bit 0
28    Filter overflow           Feedback, LPF Freq, Resonance, Key>Freq, LP Env Amt/Vel, AudioMod
29    Feedback/PrePost          Feedback bits 0-1, PrePost bits 2-6
30    LPF Freq/Feedback         LPF Freq bits 0-2, Feedback bits 3-6
31    LPF Freq/Resonance        LPF Freq bits 4-7, Resonance bits 0-2
32    Resonance/LP Key          Resonance bits 4-6, LP Key>Freq bits 0-3
33    LP Key/LP Env Amt         LP Key bits 5-6, LP Env Amount bits 0-4
34    LP Env Amt/Vel            LP Env Amount bits 6-7, LP Env VelAmt bits 0-4
35    LP Env Vel/AudioMod       LP Env VelAmt bit 6, AudioMod bits 0-5
36    HP/Amp/VCA/LFO overflow   HP Freq.0, HP Key.1, Amp Amt.3, Amp Vel.4, Volume.5, LFO1 Rate.6
37    4 Pole                    (1 bit used)
38    HP Freq/Key               HP Freq bits 1-6, HP Key>Freq bit 0
39    HP Key>Freq               bits 2-6
40    Amp Env Amount            bits 0-2
41    Amp Env Amt/Vel           Amount bits 4-6, VelAmt bits 0-3
42    Amp Vel/VCA Volume        VelAmt bits 5-6, Volume bits 0-4
43    VCA Volume/LFO1 Rate      Volume bit 6, LFO1 Rate bits 0-5
44    LFO overflow              LFO1 Amt.3, LFO1 Dest.4, LFO2 Rate.5, LFO2 Amt.2, LFO2 Dest.3, PitchEnv Dest/Amt
45    LFO 1 Shape/Rate/Amt      Rate bit 7, Shape bits 0-2, Amount bits 0-2
46    LFO 1 Amt/Dest            Amount bits 4-6, Dest bits 0-3
47    LFO 1/2 mixed             LFO1 Dest.5, LFO1 Sync, LFO2 Rate bits 0-4
48    LFO 2 Rate/Shape/Amt      Rate bits 6-7, Shape bits 0-2, Amount bits 0-1
49    LFO 2 Amt/Dest            Amount bits 3-6, Dest bits 0-2
50    LFO 2/Pitch Env           AD Mode, LFO2 Sync, LFO2 Dest bits 4-5, PitchEnv Dest bits 0-2
51    Pitch Env Dest/Amt        Dest bits 4-5, Amount bits 0-4
52    Pitch/LP Env overflow     PitchEnv VelAmt.5, Delay.6, Decay.0, Sustain.1, Release.2, LP Delay.3, LP Attack.4
53    Pitch Env Amt/Vel         Amount bits 6-7, VelAmt bits 0-4
54    Pitch Env Vel/Delay       VelAmt bit 6, Delay bits 0-5
55    Pitch Env Attack          (all 7 bits)
56    Pitch Env Decay/Sustain   Decay bits 1-5, Sustain bit 0
57    Pitch Env Sustain/Release Sustain bits 2-6, Release bits 0-1
58    Pitch/LP Env Release/Delay PitchEnv Release bits 3-6, LP Delay bits 0-2
59    LP Env Attack/Delay       Attack bits 0-3, Delay bits 4-6
60    LP/Amp Env overflow       LP Decay.5, LP Sustain.1, Amp: Delay.2, Attack.3, Decay.4, Sustain.5, Release.6
61    LP Env Attack/Decay       Attack bits 5-6, Decay bits 0-4
62    LP Env Decay/Sustain      Decay bit 6, Sustain bits 0-5
63    LP Env Release            (all 7 bits)
64    Amp Env Delay/Attack      Delay bits 1-6, Attack bit 0
65    Amp Env Attack/Decay      Attack bits 2-6, Decay bits 0-1
66    Amp Env Decay/Sustain     Decay bits 3-6, Sustain bits 0-2
67    Amp Env Sustain/Release   Sustain bits 4-6, Release bits 0-3
68    Aux 1 Env overflow        Dest.5, Amt.7, Delay.0, Attack.1, Decay.2, Sustain.3, Release.4
69    Aux 1 Env Dest/Release    Dest bits 0-4, (Amp Release bits 5-6)
70    Aux 1 Env Amount          (all 7 bits)
71    Aux 1 Env VelAmt          (all 7 bits)
72    Aux 1 Env Delay/Attack    Delay bits 1-6, Attack bit 0
73    Aux 1 Env Attack/Decay    Attack bits 2-6, Decay bits 0-1
74    Aux 1 Env Decay/Sustain   Decay bits 3-6, Sustain bits 0-2
75    Aux 1 Env Sustain/Release Sustain bits 4-6, Release bits 0-3
76    Aux 2 Env overflow        Dest.5, Amt.7, Delay.0, Attack.1, Decay.2, Sustain.3, Release.4
77    Aux 2 Env Dest/Aux1 Rel   Aux1 Release bits 5-6, Dest bits 0-4
78    Aux 2 Env Amount          (all 7 bits)
79    Aux 2 Env VelAmt          (all 7 bits)
80    Aux 2 Env Delay/Attack    Delay bits 1-6, Attack bit 0
81    Aux 2 Env Attack/Decay    Attack bits 2-6, Decay bits 0-1
82    Aux 2 Env Decay/Sustain   Decay bits 3-6, Sustain bits 0-2
83    Aux 2 Env Sustain/Release Sustain bits 4-6, Release bits 0-3
84    Mod 1-3 overflow          One bit per: Mod1 Amt, Dest, Mod2 Src, Amt, Dest, Mod3 Amt, Dest
85    Mod 1 Src/Aux2 Rel        Mod1 Source bits 0-4, Aux2 Release bits 5-6
86    Mod 1 Amount              bits 1-7 (bit 0 in byte 84)
87    Mod 1 Dest/Mod 2 Src      Dest bits 1-5, Src bits 0-1
88    Mod 2 Src/Amt             Src bits 3-4, Amt bits 0-4
89    Mod 2 Amt/Dest            Amt bits 6-7, Dest bits 0-4
90    Mod 3 Source/Amt          Source bits 0-4, Amount bits 0-1
91    Mod 3 Amt/Dest            Amount bits 3-7, Dest bits 0-1
92    Mod 4-6 overflow          One bit per: Mod4 Src, Amt, Mod5 Src, Amt, Dest, Mod6 Amt, Dest
93    Mod 3 Dest/Mod 4 Src      Dest bits 3-5, Source bits 0-3
94    Mod 4 Amount              (all 7 bits, bit 7 in byte 92)
95    Mod 4 Dest/Mod 5 Src      Dest bits 0-5, Src bit 0
96    Mod 5 Src/Amt             Src bits 2-4, Amt bits 0-3
97    Mod 5 Amt/Dest            Amt bits 5-7, Dest bits 0-3
98    Mod 5 Dest/Mod 6 Src      Dest bit 5, Source bits 0-4
99    Mod 6 Amt/Dest            Amount bits 2-7, Dest bit 0
100   Mod 7-8/Env Peaks overflow Mod7 Src.3, Amt.6, Mod8 Src.0, Amt.3, Dest.3, PitchPeak.5, LPPeak.6
101   Mod 6 Dest/Mod 7 Src      Dest bits 2-5, Source bits 0-2
102   Mod 7 Src/Amt             Source bit 4, Amount bits 0-5
103   Mod 7 Amt/Dest            Amount bit 7, Dest bits 0-5
104   Mod 8 Src/Amt             Source bits 1-4, Amount bits 0-2
105   Mod 8 Amt/Dest            Amount bits 4-7, Dest bits 0-2
106   Mod 8 Dest/Pitch Peak     Dest bits 4-5, Pitch Env Peak bits 0-4
107   Pitch/LP Peak             Pitch Env Peak bit 6, LP Env Peak bits 0-5
108   Global/Env Peaks          KeyAssign, Aux1 Peak bit 0, Aux2 Peak bit 1
109   Amp Env Peak              (all 7 bits)
110   Aux 1/2 Peak              Aux1 Peak bits 1-5, Aux2 Peak bit 0
111   Aux 2 Peak/Osc Reverse    Aux2 Peak bits 2-6, Osc3 Reverse, Osc4 Reverse
112   Fixed                     0x18
113   LFO Restart               LFO1 Restart bits 0-1, LFO2 Restart bits 0-1
114-  Fixed/Name region
156   SysEx Footer               F7
```

> Note: this byte numbering (raw wire bytes, "mystery byte" model) is the
> gist author's own frame of reference and predates cross-checking against
> TempestEdit's collector-first unpack scheme in §3. The two sources were
> produced independently (single-bit diffing vs. static analysis of a working
> app) and agree closely on *which* bits belong to which parameter, which is
> good independent corroboration - but the exact byte-to-byte mapping between
> "raw wire byte N" here and "unpacked payload byte N" in §3/§4/§5 has not
> been reconciled yet. Treat byte numbers here as relative/topological
> (which bytes cluster together) rather than absolute until reconciled with
> a real capture run through both unpack implementations side by side.

### Global tables

**Mod Sources** (5 bits, 0–22):

| Value | Source | Value | Source |
|---|---|---|---|
| 0 | Off | 12 | Pad Pressure |
| 1 | Pitch Envelope | 13 | Slider Position 1 |
| 2 | Filter Envelope | 14 | Slider Position 2 |
| 3 | Amp Envelope | 15 | Slider Pressure 1 |
| 4 | Aux 1 Envelope | 16 | Slider Pressure 2 |
| 5 | Aux 2 Envelope | 17 | Foot Pedal 1 |
| 6 | LFO 1 | 18 | Foot Pedal 2 |
| 7 | LFO 2 | 19 | MIDI Pitch Bend |
| 8 | Velocity | 20 | MIDI Mod Wheel |
| 9 | Note Number | 21 | MIDI Breath |
| 10 | Noise | 22 | MIDI Expression |
| 11 | Random | | |

**Mod/Envelope Destinations** (6 bits, 0–58) - used by all 8 mod matrix
slots, Pitch/Aux1/Aux2 envelope destinations, and LFO 1/2 destinations:

| Value | Destination | Value | Destination | Value | Destination |
|---|---|---|---|---|---|
| 0 | Off | 20 | LFO 1 Frequency | 40 | Amp Env Decay |
| 1 | Osc 1 Frequency | 21 | LFO 2 Frequency | 41 | Aux 1 Env Decay |
| 2 | Osc 2 Frequency | 22 | LFO All Frequency | 42 | Aux 2 Env Decay |
| 3 | Osc 3 Frequency | 23 | LFO 1 Amount | 43 | All Env Decay |
| 4 | Osc 4 Frequency | 24 | LFO 2 Amount | 44 | Pitch Env Release |
| 5 | Osc All Frequency | 25 | LFO All Amount | 45 | Filter Env Release |
| 6 | Osc 1/2 Mix | 26 | Pitch Env Amount | 46 | Amp Env Release |
| 7 | Osc 3 Level | 27 | Filter Env Amount | 47 | Aux 1 Env Release |
| 8 | Osc 4 Level | 28 | Amp Env Amount | 48 | Aux 2 Env Release |
| 9 | Osc 1 Pulsewidth | 29 | Aux 1 Env Amount | 49 | All Env Release |
| 10 | Osc 2 Pulsewidth | 30 | Aux 2 Env Amount | 50 | Mod 1 Amount |
| 11 | Osc 1/2 Pulsewidth | 31 | All Env Amount | 51 | Mod 2 Amount |
| 12 | Sub Osc Volume | 32 | Pitch Env Attack | 52 | Mod 3 Amount |
| 13 | Feedback Volume | 33 | Filter Env Attack | 53 | Mod 4 Amount |
| 14 | Lowpass Filter | 34 | Amp Env Attack | 54 | Mod 5 Amount |
| 15 | Resonance | 35 | Aux 1 Env Attack | 55 | Mod 6 Amount |
| 16 | Filter FM | 36 | Aux 2 Env Attack | 56 | Mod 7 Amount |
| 17 | Highpass Filter | 37 | All Env Attack | 57 | Mod 8 Amount |
| 18 | VCA Level | 38 | Pitch Env Decay | | |
| 19 | Pan | 39 | Filter Env Decay | | |

**LFO Sync Rate values** (shared by LFO 1 & 2 Rate, 8 bits, 0–162 - sync
mode constrains the value to these increments of 2):

| Sync Rate | Value | Sync Rate | Value |
|---|---|---|---|
| 32 Qrtr | 70 | 1 Qrtr | 86 |
| 16 Qrtr | 72 | Qrtr Trip | 88 |
| 8 Qrtr | 74 | 8th | 90 |
| 6 Qrtr | 76 | 8th Trip | 92 |
| 4 Qrtr | 78 | 16th | 94 |
| 3 Qrtr | 80 | 16th Trip | 96 |
| 1/2 Note | 82 | 32nd | 98 |
| Qrtr Dot | 84 | 64th | 100 |

### Encoding conventions observed

- Parameters are **not** packed sequentially by index; each parameter's bits
  are scattered across 1–3 bytes.
- "Overflow" bytes collect stray high bits from several nearby parameters -
  e.g. byte 28 (filter section), 84/92/100 (mod matrix), 68/76 (aux envelopes).
- Every SysEx data byte is 7-bit clean (0x00–0x7F); only bits 0–6 are used.
- Some bits are aliased - e.g. Osc 3 Fine Freq bit 4 is written at both byte
  14 bit 7 and byte 12 bit 1; use the byte-14 location for writes.
- `AD Mode` (byte 50 bit 3) is **inverted**: 1 = ADSR, 0 = AD.
- Osc 3/4 Frequency uses offset encoding: `displayed semitones = internal - 64`.
- Osc 3/4 Fine Freq uses offset encoding: `displayed cents = internal - 50`.
- Bipolar amounts (Pitch/LP/Aux1/Aux2 Env Amount, Mod Amounts) are 8-bit,
  stored 0–254, `displayed = internal - 127`.
- Byte 22 and bytes 114–155 (fixed/name region) remain only partially mapped
  by the gist source.

Per-parameter byte.bit tables for every oscillator, envelope, LFO, and mod
slot are in the gist linked in Sources - they are not duplicated bit-by-bit
here beyond the overview above, to keep this file maintainable. Pull the
full tables from the gist when implementing `tempest_read_sound_params` /
`tempest_set_sound_param`, and validate each one against a hardware capture
before shipping a write path, exactly as the existing `beat-mapper` workflow
prescribes.

---

## 7. Live hardware capture session findings (2026-09-20)

With the Tempest reachable over USB, a real capture session was run against
a freshly **Initialize Project**'d Tempest (`Erase + Copy` → `Initialize
Project`), following the beat-mapper workflow. Captures live in
`~/Tempest/captures/beat-research/`. This section records what was learned,
including two dead ends that cost real capture cycles - both worth avoiding
in a future session.

### 7.0 A crash bug, found and fixed

The very first live capture attempt crashed the capture process entirely:
`gomidi.ListenTo()` in `internal/midi/device.go` didn't pass
`SysExBufferSize`, so it used the underlying library's 1024-byte default. A
Beat/Kit (0x5F) dump is ~5.9KB - well past that - and the library panics
(`index out of range`) rather than erroring gracefully. This would have
crashed the real MCP server too, on any `tempest_wait_for_dump` /
`tempest_save_received_dump` / `tempest_extract_sounds_from_project` call
against a Beat or Project dump. Fixed in `internal/config/config.go` /
`internal/midi/device.go` (`SysExConfig.BufferBytes`, default 1MiB) -
commit `fb2e649`.

### 7.1 `0x5F` reconfirmed live

`raw[3] == 0x5F` on a live "Export Beat in RAM over MIDI" capture, matching
the file-library-based confirmation in §1. Length 5925 bytes for a freshly
initialized (empty) beat - matches the two pre-existing `HWC *.syx` samples
used for the original confirmation.

### 7.2 Dead end: 16 Beats vs 16 Sounds - a pad-function trap

**This is the single most important operational fact for anyone running
more captures.** The Tempest's pad-function keys are easy to confuse:

- **16 Beats** - the 16 pads select which of the **16 separate beats**
  (entire different patterns) is active. This is *not* a track/pad selector
  within one beat.
- **16 Sounds** - the 16 pads select which **sound/track pad** (A1–A16, or
  B1–B16) *within the currently selected beat* you're editing. This is what
  you actually want for track-stride testing.
- **16 Time Steps** - the 16 pads represent 16 time steps for the
  *currently selected sound* (from 16 Sounds), toggling a note at "the
  velocity level at which the pad was struck" (Tempest Operation Manual,
  "16 Time Steps"). This is also the source of §7.4's velocity noise.

Early in this session, "track stride" was tested by pressing **16 Beats +
pad 2** (intending "track A2") - which actually switched to an entirely
different *beat* (Beat 2), not a different track in Beat 1. The resulting
capture (`kick_a2_s1.syx`, first attempt) diffed against Beat 1's capture
showed 58 bytes differing across a ~124-byte scattered range - that was
beat-to-beat noise (likely uninitialized/non-deterministic leftover bytes
in each beat's own sequencer region), not meaningful track-offset data. The
capture was silently comparing two different beats the whole time.

**Fix:** use **16 Sounds + pad N** to select track/pad AN, staying on the
same beat throughout. Confirmed no false "success" on the corrected
attempt (see §7.5) - the byte-count math checked out as informative instead.

### 7.3 Dead end: clearing vs. adding a step

When "moving" a note from step 1 to step 2 by pressing the step-2 pad, if
step 1 isn't explicitly cleared first, the Tempest keeps *both* notes (two
active steps) rather than moving the one note - toggling a lit pad in 16
Time Steps mode deletes it, but pressing an *unlit* pad only inserts, it
doesn't implicitly clear elsewhere. The payload size is the tell: a beat
with N active notes is consistently `5925 + 8*N` raw bytes (7 unpacked
bytes per note, one full 8-byte wire-encoding group). A same-length
recapture after explicitly confirms whether a "move" was clean.

### 7.4 Confirmed: step-position field

Comparing three clean single-note captures (`kick_a1_s1.syx`,
`kick_a1_s2.syx`, `kick_a1_s3.syx` - same track, same velocity target, step
1/2/3 respectively, each with the prior step explicitly cleared) via
`beat-mapper diff` gives a clean, reproducible 2-byte diff each time:

| Absolute offset | Sequencer-relative offset | step 1 | step 2 | step 3 |
|---|---|---|---|---|
| `0x0437` | 67 (= `KitSequencerOffset` + 65) | `0x00` | `0x03` | `0x06` |
| `0x043A` | 70 | `0x0B` | varies | varies |

**`0x0437` (sequencer-relative offset 65) is confirmed as the step-position
field, encoded as `step_index × 3`** - reproduced identically across two
independent captures of "step 2" (both gave `0x03` at this offset, despite
the *other* byte differing between the two takes). This is a solid,
byte-exact, reproducible result - the strongest finding of this session
after the encoding-scheme work.

The `×3` multiplier suggests a finer time-resolution tick count rather than
a flat 0–15 step index (3 ticks per 16th-note step, one guess), but the
exact meaning of the multiplier hasn't been tested (e.g. by capturing a
note on an 8th-note or triplet grid to see if the stride changes).

**`0x043A` is not step-position-derived** - two independent captures of
"step 2" gave different values (`0x33`/51 and `0x2B`/43) at this offset.
Per the manual, 16 Time Steps programming captures a note "at the velocity
level at which the pad was struck" - this is almost certainly (part of) the
velocity field, contaminated by real tap-to-tap variance from manual
programming. Isolating its exact formula would need a numeric/fixed-value
entry method rather than live pad taps, if the Tempest has one (not yet
checked against the manual).

### 7.5 Still open: track stride and note-record layout

With the 16-Beats/16-Sounds confusion (§7.2) corrected, a proper A2 test
was attempted, but Beat 1 already had a leftover, never-cleared kick on A1
step 3 from the §7.4 testing - so the resulting capture had *two* notes
(A1 step 3 + the new A2 step 1), not a clean single-track swap. This
actually turned out useful: it let the new note be isolated the same way
§0→1-note transitions were tested, but with a negative result both times:

- Comparing `baseline.syx` (0 notes) → `kick_a1_s1.syx` (1 note): best
  fixed-offset insertion point realigns only 11 of ~4000 remaining bytes.
- Comparing `kick_a1_s3.syx` (1 note) → the contaminated 2-note capture:
  same result, 11/4000, and even the *prefix* before the "best" insertion
  point only matched 82/175 bytes - not clean either.

**Conclusion: the sequencer region past `KitSequencerOffset` (1012) almost
certainly is not a flat, byte-aligned structure.** A single fixed-size
record insertion should produce a clean prefix match + shifted-but-matching
suffix; neither test found that. The far more likely explanation is
**bit-packing** - sub-byte fields, the same technique already confirmed for
the Sound (0x60) parameter body in §6 - where inserting one note shifts
every subsequent bit-packed field by a non-byte-aligned amount, making
naive byte-level diffing show near-total divergence downstream even for a
small, local content change.

What *is* confirmed, useful groundwork for whoever picks this up:

- Header (offsets 0–51: BPM, swing, name, short name) and the full 960-byte
  pad table (offsets 52–1011) are **always byte-identical** across single-note
  variations - confirms §5's `KitPadTableOffset`/`KitSequencerOffset` split
  and rules out pad-table involvement in step/track changes.
- Total payload size is `5925 + 8N` raw bytes for `N` active notes (§7.3) -
  a fast, cheap sanity check for "did this capture actually change what I
  intended" before spending a `beat-mapper diff` call on it.
- The step-position field (§7.4) is confirmed and byte-aligned - so *some*
  fields in this region are plain bytes, at least until whatever offset the
  bit-packing (if that's really what's happening) kicks in.

**What a future session needs to make progress here:**

1. ~~**A bit-level diff tool**, not just byte-level `beat-mapper diff`~~ -
   done: `beat-mapper bitdiff` (`cmd/beat-mapper/mapper/bitdiff.go`). It finds
   the longest identical bit-aligned prefix between two unescaped payloads,
   then searches a configurable window of bit shifts for the one that best
   realigns the remaining content - exactly the "XOR bit-by-bit, look for a
   clean boundary" method this item originally called for, generalized to
   also handle deletions and to report a shift's *sharpness* (how many times
   lower its mismatch rate is than a wrong shift's) rather than requiring a
   literal zero-mismatch match, since real hardware captures essentially
   never produce one (see below). See the README's beat-mapper section for
   usage and full example output.

   **Preliminary result, not yet independently verified:** running it against
   this session's own trusted `baseline.syx` (0 notes) vs `kick_a1_s1.syx` (1
   note, A1 step 1) finds a sharp best-fit shift of **+80 bits (10 bytes)** at
   bit offset 8616 (byte 1077) - notably *not* byte 1012
   (`KitSequencerOffset`) or a clean multiple of the 56-bit
   (7-unpacked-byte) wire-group size one might naively expect. The match
   isn't a literal zero (163/32800 bits still mismatch), but it's ~65x
   cleaner than neighboring shifts, which is a strong signal, not proof. This
   is a single data point from one capture pair - it needs a repeat run
   against another confirmed single-note pair (e.g. `kick_a1_s2.syx`) before
   trusting either the 8616-bit boundary or the 80-bit width as real,
   per this doc's usual bar for confirmation.
2. **A clean, single-variable track-stride test** - start from a beat with
   *zero* notes (freshly initialized or all steps explicitly cleared),
   confirm 5925 bytes before capturing, then add exactly one note on a
   specific track/step. Repeat for a second track, same step. This avoids
   both dead ends in §7.2/§7.3 at once.
3. **A numeric-entry velocity test**, if the Tempest has one - check the
   manual for a step-edit screen that sets velocity by value rather than
   tap strength, to finally isolate `0x043A`'s exact encoding.

---

## 8. Session handoff - in progress (2026-09-21, paused mid-session)

This section is a working handoff, not a confirmed-findings writeup like §7 -
picking this back up should start by reading this section first.

### 8.0 Uncommitted work sitting in the working tree

**Update (2026-09-21, later the same day):** both items below were committed
as `bbf5487`. The sound-parameter half is now finished: a hardware-capture
test (`internal/sysex/soundparams_test.go`, backed by the three captures
copied into `internal/sysex/testdata/sound-research/`) confirms the same 3/3
hits described below, and `tempest_read_sound_params` is now wired up in
`internal/server/server.go` (read-only, decodes a saved `.syx` or waits for
a live dump; there is still no write path). The beat-pattern half
(`cmd/capture-tmp`, §8.1-§8.4) is unchanged; that work needs the physical
hardware and hasn't resumed.

Two things were built and compiling but not committed at the time this
section was originally written:

- **`internal/sysex/soundparams.go`** - the full Sound (0x60) parameter offset
  table, generated from the community gist (fetched via `gh gist view
  c39a3b4b10e1e51af58e49ef74aca116`) and mechanically translated through the
  raw-byte→unpacked-byte formula validated in §7 (3/3 confirmed hits: Pitch
  Env Attack, LP Env Release, AD Mode). 121 parameters, `ReadSoundParam`/
  `DisplaySoundParam`/`DecodeSoundParams` helpers, `ModSourceNames`/
  `ModDestNames` lookup tables. Compiles and passes `go vet`/`gofmt`, but has
  **no test coverage yet** and isn't wired into an MCP tool
  (`tempest_read_sound_params`) yet. Next step: write a test that decodes the
  real captures in `~/Tempest/captures/sound-research/` (`sound_baseline.syx`,
  `sound_pitch_attack.syx`, `sound_lp_env_release.syx`) and asserts the known
  parameter values, then wire up the read-only MCP tool.
- **`cmd/capture-tmp/`** - a throwaway-turned-useful live-MIDI capture tool
  (uses `internal/midi.Device` directly, unlike `beat-mapper` which has no
  MIDI dependency by design - this is why it's a separate tool, not a
  `beat-mapper` subcommand). Usage:
  `capture-tmp <out.syx> <expected_note_count> [timeout_sec] [baseline.syx]`.
  As of this session it auto-verifies every capture immediately: computes
  note count from raw size (`(len-5925)/8`), flags a mismatch against
  `expected_note_count`, decodes the step-position/velocity bytes for the
  single-note case, and diffs the pad table against a baseline file if given.
  Built specifically because manual hardware capture sessions kept producing
  silently-contaminated files (see §8.2) - use it for every future capture,
  don't go back to capturing blind. Not committed yet; decide whether it
  belongs in the repo (it's proven valuable) or stays a local research tool.

### 8.1 The §7.5 "track footprint" finding did not hold up

§7.5 concluded that offsets `0x0440`–`0x04B6` (a ~58-byte-changing, ~124-byte
footprint) encoded track/sound identity, based on two comparisons that both
showed the same footprint change. Tonight, a more carefully controlled
re-test contradicted this:

- Two independently-verified single-note captures (`kick_a1_s1_v2.syx`, A1;
  `kick_a2_s1_clean.syx`, A2 - both confirmed exactly 5933 bytes, i.e. one
  note, via the new capture-tmp verifier) were diffed. Result: **only the
  step-position byte (`0x0437`) and velocity byte (`0x043A`) differed - the
  footprint was byte-identical.**
- The step-position byte's value in `kick_a1_s1_v2.syx` was `0x30` = 48. Under
  the confirmed `step_index × 3` formula that's **step 16, not step 1** -
  despite the file being deliberately set up as "A1, step 1." `pos/3 = 16`
  decodes (per the new tool's bar/step math) to **bar 2**, not bar 1.

Conclusion: the step-edit ("16 Time Steps") screen can be scrolled to a later
bar (`Soft Knob 2: See`, per the manual - "moves between displaying
sequential time blocks within the beat"), and this happened at least once
without an obvious remote cue, silently invalidating a "step 1" label. **The
original §7.5 footprint conclusion is now unconfirmed and probably wrong** -
it's more likely that at least one of the two captures behind it was
contaminated by a stray note, a scrolled bar view, or a Copy-Sound side
effect (see §8.2) rather than genuinely isolating track identity. The
footprint's real meaning is back to unknown.

**What's still solid, unaffected by this:** the step-position field itself
(`0x0437`, `step_index × 3`, sequencer-relative offset 65), confirmed 3
separate times with consistent values before tonight's confusion started.

### 8.2 Operational failure modes hit this session (read before resuming)

These cost real capture cycles tonight. Internalize them before doing more
captures:

1. **Copy Sound's pad-tap meaning depends on the current pad-function mode.**
   "Press Copy, tap source pad, tap dest pad" only copies a *sound* correctly
   in **16 Sounds** mode. If the machine is still in **16 Time Steps** mode
   (left over from adding a note), the same taps get interpreted as step
   numbers, which can silently create/toggle notes as a side effect. Always
   explicitly switch to 16 Sounds before a Copy-Sound sequence.
2. **Stray notes accumulate silently.** Multiple captures this session came
   back with 2+ notes when exactly 1 was intended, with no clear single root
   cause identified - sometimes a genuinely leftover note from earlier
   testing, sometimes reappearing even after an Initialize Beat + careful
   redo. Don't trust a capture's note count from memory - always check the
   raw file size (`5925 + 8×N` bytes) before using it, which capture-tmp now
   does automatically.
3. **The step-edit view can scroll to a different bar** without an obvious
   remote-visible cue, making a "step 1" label wrong (see §8.1). No fix
   identified yet beyond checking the decoded position byte's bar value
   (capture-tmp now warns if `bar != 1`) - worth checking the manual for how
   to reliably reset `Soft Knob 2: See` to the first bar before each capture.
4. **General lesson:** don't build a multi-step edit on top of uncertain
   prior state. Re-Initialize (Beat, or Project if needed) before *every*
   single-variable test rather than incrementally clearing/editing from
   whatever state the machine happens to be in - it's slower per-capture but
   has been reliably faster overall than debugging contaminated captures
   after the fact.

### 8.3 Where to resume

1. Finish validating `cmd/capture-tmp`'s verify output on a real fresh
   Initialize+export cycle (was interrupted mid-test - the last run timed
   out because no export had been triggered yet).
2. Redo the track-stride test from §7.5/§8.1 properly: Initialize Beat, add
   one note on A1 step 1, **check capture-tmp's decoded bar/step is bar 1**
   before trusting the file, then repeat for A2 step 1, then diff. Only trust
   the result if both captures show bar 1 and note count 1.
3. Decide what (if anything) in the footprint region is worth pursuing
   further, or whether it needs the bit-level diff tool from §7.5 after all.
4. Separately: finish the sound-parameter work (§8.0) - this doesn't depend
   on any of the above and is much closer to done.

### 8.4 Capture file inventory (as of this session)

`~/Tempest/captures/beat-research/` has accumulated files from both the
earlier (§7) and tonight's (§8) sessions, some with **misleading names** -
notably `kick_a1_s1_v2.syx` is actually bar 2 step 1, not bar 1 step 1 (see
§8.1). Don't assume a filename is accurate; re-verify with capture-tmp's
decode before reusing any of these. `kick_a1_s1.syx`, `kick_a1_s2.syx`,
`kick_a1_s3.syx` (the original §7.4 step-position captures) are still
trusted - they predate tonight's confusion and their result reproduced
consistently 3 times.

---

## 9. Confirmed: the note record format and track identity (2026-09-21, session 3)

With the Tempest reachable again and `beat-mapper bitdiff` (§7.5) now built,
this session redid the track-stride test the way §7.5/§8.3 called for: a
confirmed-zero-notes beat before every single-variable capture, one note
added per test, `capture-tmp` verifying note count and bar/step before
trusting any file. Three clean captures were taken - `kick_a1_s1_fresh.syx`,
`kick_a2_s1_fresh.syx`, `kick_a3_s1_fresh.syx`, each: Initialize Beat → 16
Sounds → tap the target track pad → 16 Time Steps → tap step 1 → Export Beat
in RAM over MIDI. All three verified as exactly 1 note, bar 1, step 1, pad
table byte-identical to baseline.

### 9.1 The note record is 80 bits (10 bytes), not a byte-aligned 56 bits

`beat-mapper bitdiff` against `baseline_fresh.syx` (0 notes) found the same
result for both `kick_a1_s1_fresh.syx` and `kick_a2_s1_fresh.syx`: a common
bit-aligned prefix of exactly 8616 bits (byte 1077), and a best-fit shift of
**+80 bits (10 bytes)** with only 1/32800 bits mismatching - a 10,573x sharper
match than any neighboring shift, as clean a boundary as this project has
found anywhere. This is *not* the 56-bit (7-unpacked-byte) width one might
naively expect from one Tempest-7+1 wire group; the note record is 3 bytes
wider than that, non-obviously.

### 9.2 Byte-by-byte layout of the 10-byte record

Diffing the three captures pairwise (`kick_a1_s1_fresh.syx`,
`kick_a2_s1_fresh.syx`, `kick_a3_s1_fresh.syx`) against each other and against
baseline gives this layout for absolute unpacked-payload offsets 1077-1086
(all three captures agree on every byte except the two noted as varying):

| Offset | A1 s1 | A2 s1 | A3 s1 | What it is |
|---|---|---|---|---|
| 1077 | `06` | `06` | `06` | constant across all 3 - meaning unknown |
| 1078 | `00` | `00` | `00` | constant across all 3 - meaning unknown |
| 1079 (`0x0437`) | `00` | `00` | `00` | step position, confirmed formula (§7.4) - `0` for step 1 in all 3 |
| 1080 | `77` | `77` | `77` | constant across all 3 - meaning unknown |
| **1081 (`0x0439`)** | **`80`** | **`81`** | **`82`** | **track identity - see below** |
| 1082 (`0x043A`) | `34` (52) | `2D` (45) | `27` (39) | velocity (§7.4, noisy/tap-driven) - matches `capture-tmp`'s independent readout exactly |
| 1083-1086 | `02 00 00 00` | `02 00 00 00` | `02 00 00 00` | constant across all 3 - meaning unknown |

### 9.3 Track identity: `0x80 | track_index` (0-based), byte `0x0439`

Byte 1081 (`0x0439`) is the *only* byte, besides the known-noisy velocity
byte, that differs across three otherwise-identical captures that vary in
nothing but which track the note is on. Its value increases by exactly 1
for each track: A1 = `0x80` (index 0), A2 = `0x81` (index 1), A3 = `0x82`
(index 2). The high bit (`0x80`) is constant across all three - almost
certainly an unrelated flag (possibly "note active"/gate-on) rather than
part of the index - with the low bits carrying a 0-based track index. This
is a clean, linear, 3-point-confirmed result, cross-validated two
independent ways: a direct `beat-mapper diff` between capture pairs, and
`beat-mapper bitdiff`'s bit-level insertion-content extraction against
baseline. Both land on the exact same byte and the exact same values.

**This resolves the §7.5 dead end.** The original "track stride" search
assumed track identity was encoded by *storage position* - a fixed
per-track/per-step region, the way the old (debunked, see §8.1) "58-byte
footprint" theory imagined. It now looks structurally different: the
sequencer likely stores a **sparse list of ~80-bit note records**, one per
active note, each self-contained (step position, track identity, and
velocity all as *values* inside the record) rather than a dense
track-by-step grid addressed by a positional stride. That would also explain
why every byte-level "insertion point" search in §7.5 came back nearly
random past the pad table - there is no fixed stride to find, because track
identity isn't positional here.

**What this doesn't yet establish, and shouldn't be assumed:**

- Only tracks A1-A3 tested. The linear pattern should be checked at least
  once more at a higher index (e.g. A16, or crossing into bank B at B1) to
  rule out a non-linear encoding (bank bit, wraparound, etc.) that only three
  low, consecutive values couldn't reveal.
- Only single-note beats tested. Whether multiple active notes are stored as
  simple consecutive 10-byte records (the simplest version of the "sparse
  list" theory) - and if so, in what order (insertion order? step order?) -
  is untested. A 2-note capture (e.g. A1 step 1 + A2 step 2) is the natural
  next test: it should show the payload grow by exactly 160 bits (2 x 80)
  if the theory holds, and `bitdiff` should find two clean 80-bit insertions.
- The four constant-across-all-3-captures bytes (1077, 1078, 1080, and the
  1083-1086 block) are unexplained. They didn't vary in these tests because
  nothing that might affect them (gate length, note duration, which sound is
  assigned to the pad, etc.) was varied. Worth a capture that changes one of
  those while holding track/step fixed, once a hypothesis for what they might
  encode is worth testing.

---

## Suggested next steps for this repo

Done as of this session (see §7 and `internal/sysex/`):

1. ~~Confirm the collector-first unpack scheme (§3) against a real
   capture~~ - done; implemented in `internal/sysex/encoding.go`.
2. ~~Add `0x5F` to `internal/sysex/message.go`'s type table~~ - done
   (`TypeBeat`/`TypeBeatDump`); `0x62` remains unadded/unconfirmed.
3. ~~Re-derive `Location()`/`BankSlot()` for FLASH (`0x63`)~~ - done;
   `BuildFLASHDump` no longer embeds a location byte, and `tempest_load_sound`
   only tracks bank/slot locally (see README).
4. ~~Build a `internal/sysex/soundparams.go` offset table~~ - done; 121
   parameters, spot-checked against 3 hardware captures, wired up as the
   read-only `tempest_read_sound_params` MCP tool. No write path
   (`tempest_set_sound_param`) yet, and most parameters are still unverified
   individually - see §8.0.
5. ~~Build a bit-level diff tool~~ (§7.5) - done: `beat-mapper bitdiff`.
6. ~~Redo the track-stride capture cleanly~~ (§7.5, step 2) - done, see §9:
   the note record is 80 bits wide at byte 1077, and track identity is a
   linear value (`0x80 | track_index`) at byte `0x0439`, not a positional
   stride. Confirmed across A1-A3; see §9's own caveats before treating it
   as the final word (untested at higher indices, across the bank boundary,
   or with multiple simultaneous notes).

Still open, in priority order:

1. **Validate §9's note-record theory further** - confirm the linear track
   index holds at a higher index or across the A/B bank boundary, and test a
   multi-note capture to check whether records really do concatenate as
   simple consecutive 10-byte blocks (see §9's caveats for the exact tests).
2. **Decode the still-unknown constant bytes in the note record** (offsets
   1077, 1078, 1080, 1083-1086, §9.2) - untested against anything that might
   vary them, like gate length or which sound is assigned to the pad.
3. **Isolate the velocity field** (`0x043A`, §7.4/§9.2) - needs a
   numeric-entry method instead of live pad taps, if one exists.
4. **Independently verify the 0x5C/0x5E scheme** - still just assumed
   uniform with everything else (§3), never decoded from real 0x5C/0x5E
   content the way FLASH/RAM/Beat were.
5. **Investigate the RAM (0x60) name field** - §4's bit-offset-880 theory
   didn't hold up against 30 real captures; still unknown where (or if) RAM
   dumps carry a name.
6. **Capture and decode a real Project (0x61) dump** - zero examples exist
   in this project's entire sample library (`~/Tempest`, `~/Tempest/captures`);
   completely unexplored.
