# Tempest Firmware Analysis - Research Notes

This document collects what's been learned about the Tempest's own internal
firmware - the code that actually runs on its processors - as distinct from
[docs/sysex-tempest-format.md](sysex-tempest-format.md), which covers the
SysEx wire protocol used to talk to the Tempest over MIDI. The two are
related: some of this investigation's motivation is that the firmware
contains readable strings ("Failed to read sequence data" and similar)
directly relevant to the still-open Export Beat multi-note question in the
SysEx doc (§9.4-9.6) - if the firmware's sequence-read/write code can be
found and read, it could explain that bug directly, rather than continuing
to guess at it from the outside via more hardware captures.

**Status as of 2026-09-22: architecture and chip identities are confirmed.
The byte-precise memory layout needed to actually read the interesting code
is not resolved, and multiple independent attempts to solve it did not
succeed.** This is a working state, not a finished investigation - read
through to the end before assuming anything or spending more time
re-attempting what's already been tried.

---

## 1. The firmware itself

The Tempest's operating system is distributed as a MIDI SysEx bulk dump, the
same transport mechanism `tempest-mcp` already uses for sound/beat/project
data. Download from
[sequential.com/support/download/tempest-operating-system](https://sequential.com/support/download/tempest-operating-system/)
(`Tempest_OS_1.5.0.2.zip` as of this writing) - a zip containing **four
separate `.syx` files, one per processor**:

| File | Processor | SysEx type byte |
|---|---|---|
| `Tempest_Main_1.5.0.2.syx` | Main | `0x71` |
| `Tempest_Voice_1.5.syx` | Voice | `0x72` |
| `Tempest_Panel_1.3.syx` | Panel | `0x73` |
| `Tempest_Sam_1.1.syx` | SAM | `0x74` |

This confirms the OS readme's own claim: "Tempest has four different
operating systems, one for each of the different types of processors that
control its functions." These four type bytes are a distinct message family
from the `0x60`/`0x61`/`0x63`/`0x5C`/`0x5E`/`0x5F` Sound/Beat/Project family
documented in the SysEx format doc.

Each file unescapes cleanly with `tempest-mcp`'s existing
`sysex.Unescape7Plus1` (the same collector-first scheme already confirmed for
Sound/Beat/Project data), using a plain 4-byte header (`F0 01 28 <type>`) -
no special-casing needed beyond what the repo's `sysex` package already
knows, other than that `sysex.Identify` doesn't currently recognize these
four type bytes (they decode to `TypeUnknown`), so the standard
`beat-mapper unescape`/`diff` CLI commands don't work on them directly (they
skip unrecognized types). Manually stripping the 4-byte header and calling
`sysex.Unescape7Plus1` on the rest works fine.

## 2. Confirmed: two different processor architectures, not four

**Main and Panel are MIPS32, little-endian - a Microchip PIC32 (MIPS M4K
core).** This is instruction-level confirmed, not a guess:

- Both files' early bytes decode as clean, unambiguous MIPS32 instructions
  when disassembled with the `capstone` Python library
  (`CS_ARCH_MIPS + CS_MODE_MIPS32 + CS_MODE_LITTLE_ENDIAN`) - real function
  prologues (`addiu $sp,$sp,-N` / register `sw` saves / `lui`), not
  coincidental bit patterns.
- Panel's firmware additionally contains genuine `mfc0`/`mtc0` COP0-register
  instructions - these only appear in exception/interrupt-handling code or
  rare low-level configuration, so this is real evidence of a real MIPS
  exception-handling environment, not noise.
- This independently corroborates (via primary-source binary analysis, not
  just the secondhand claim) an unverified Gearspace forum post describing
  the Tempest's main processor as "a PIC32 running at 80MHz." Note: that
  specific claim traces back to a post that reads as being about the
  **Prophet 12** (a different, later Sequential/DSI product using SHARC DSPs
  for audio and a PIC32 for the main OS) more than the Tempest specifically -
  plausible that DSI reused the same control-MCU platform across the product
  line, but that's inference, not a confirmed direct source for the Tempest.
  The instruction-level confirmation above stands on its own regardless.

**Voice and SAM are not MIPS, and not any commonly-documented architecture.**
No MIPS prologue or COP0 signature appears anywhere in either file. Real
Tempest hardware photos (see §4) confirm what they actually are:

- **SAM** is a **Dream S.A.S. SAM3716** - a real, if obscure, off-the-shelf
  part ("SAM" in the firmware filename is literally this chip's product
  name, not project-specific). Its datasheet describes 16 on-chip
  proprietary "P24" DSP cores (2k×24 RAM + 1.2k×24 ROM each, 24-bit data /
  96-bit coefficient resolution), built for audio matrix-mixing/effects
  applications. This architecture is undocumented publicly beyond
  block-diagram level - no public instruction set reference, no accessible
  disassembler or toolchain exists for it (even hobbyist forums report being
  unable to find a full datasheet). **Not realistically reversible with
  available tools.**
- **Voice** is a chip Sequential/DSI had custom-labeled **"TEMPEST DSP 1.0"**
  on the board silkscreen instead of showing its real manufacturer part
  number (a common OEM practice). Its real identity is unknown. Given its
  position on the board right next to the SAM chip in the audio section, and
  that Voice's firmware shows the same "not MIPS, not identifiable" pattern
  as SAM, it's reasonably assumed to be similarly out of reach - not
  independently confirmed.

**Practical conclusion: the Main/Panel MIPS path is the only one of the four
processors with a realistic path to further reverse engineering.** Ghidra,
`capstone`, and general MIPS/PIC32 knowledge all apply there. Nothing
comparable exists for the other two.

## 3. Confirmed: Main and Panel are almost certainly the same PIC32 part

Comparing the *unescaped* headers of Main and Panel directly (not the
escaped wire bytes, which don't compare 1:1 across files due to the
collector-first scheme's per-group structure):

```
Main:  05 f3 40 0f 00 00 00 00 ff ff 00 10 00 00 00 00
Panel: 9a 01 40 0f 00 00 00 00 ff ff 00 10 00 00 00 00
```

**14 of 16 bytes are byte-for-byte identical**, with only the first 2 bytes
differing. This is consistent with those first 2 bytes being a per-file
checksum (varies with content, as a checksum should) and the rest being
fixed structural data shared by both files - which in turn is consistent
with Main and Panel targeting the exact same PIC32 part number, not just the
same architecture family. Practical implication: if Panel's physical chip
marking can ever be read (its board may be smaller/less crowded/more
photographable than the main analog board that hosts the audio chips), that
would answer the "what exact PIC32 variant" question for Main too.

Tested whether the first 2 bytes are a simple checksum over the rest of the
payload (byte sum, 16-bit sum, XOR, the classic Roland 7-bit checksum
convention) - none matched. Doesn't rule out a checksum (there are many
possible algorithms - CRC-16/32, different included byte ranges, etc.),
just means the common simple ones aren't it.

The `ff ff 00 10` bytes (positions 6-9) initially looked like a plausible
flash-page-size field (`0x1000` = 4096, a common PIC32 flash erase-page
size, with `0xffff` as an adjacent erased-flash sentinel) - but the same
8-byte pattern (`ff ff 00 10 00 00 00 00`) appears to repeat at the next
8-byte boundary in Panel's header too, which looks more like plain
`0xFFFF`-padded reserved/unused space than a deliberately meaningful field.
Treat that specific guess as walked back, not confirmed.

## 4. Physical board identification

Real Tempest mainboard photos (found via image search, sourced from a public
Facebook post "Analog drum synthesizer Tempest by Dave Smith Instruments" in
the "I Take Pictures of Electronic Parts" group) show the board silkscreened
**`DSI-800R Rev 1.2`, `(C) 2010 Dave Smith Instruments`**.

![DSI-800R main board, wide view](images/dsi-800r_main-board_wide.png)

- A chip clearly marked **`dream SAM3716`** - the SAM processor (§2).
- A chip custom-labeled **`TEMPEST DSP 1.0`** - almost certainly the Voice
  processor (§2), real identity hidden by the re-badge.

![SAM3716 and TEMPEST DSP 1.0 chips, with board revision text](images/dsi-800r_sam-and-voice-dsp.png)

- Six repeated identical small-IC clusters, consistent with the manual's
  "six powerful analog synthesis voices."

**A higher-resolution version of the same album was later found and pulled
directly from the Facebook post (1740x2048, vs. the ~520x320 first-pass
copy above) - this changed the picture meaningfully:**

![Panel board (top) and main board (bottom), full resolution](images/dsi-800r_panel-and-main-boards_hires.jpg)

This shows there are (at least) **two separate boards** in the unit: a top
board that is a pure button/LED matrix (rows of pad footprints, a ribbon
connector, "Do Not Remove Screws" silkscreen, no IC larger than a shift
register/driver visible anywhere on it) sitting above the main `DSI-800R`
board seen in the original two photos. **This board is most likely the
physical pad/keypad panel specifically, not necessarily "the Panel processor
board" in the SysEx-firmware sense** - if Panel's PIC32 lives on a third,
still-unphotographed board, or off-frame on one of these two, that's still
unresolved. Worth remembering next time "Panel" comes up: the firmware name
and the physical board name may not refer to the same board.

The main board's control-MCU cluster is now visible clearly enough to be
useful:

![MCU cluster: U45, a dream-branded chip, and an ISSI memory chip](images/dsi-800r_mcu-cluster_zoom.jpg)

- **`U45`** (leftmost chip in the cluster) is the strongest remaining
  candidate for Main/Panel's PIC32 by position (directly beside the
  ribbon-cable connector to the panel/button board) and package size.
- Next to it, a **second `dream`-logo chip** - most likely the same
  `SAM3716` already identified elsewhere on this board (same board,
  different framing), not a second distinct part; not fully confirmed
  either way from this angle.
- An **`ISSI`-branded memory chip** (Integrated Silicon Solution Inc, a
  real SRAM/Flash vendor) sits in the same cluster - consistent with a
  classic "MCU + external SRAM" arrangement, circumstantial support for
  `U45` being the actual controller rather than an unrelated chip.

**`U45`'s marking is physically hidden, not just hard to photograph:**

![U45 close-up: a paper label is glued directly over the chip's factory marking](images/dsi-800r_u45_label_zoom.jpg)

Zooming into `U45` at full resolution shows the white square isn't a flash
reflection (the working assumption from the earlier, lower-res photos) - it's
a **paper label glued directly onto the chip package**, with faint rows of
printed text on it that are below this photo's legibility floor. This is the
same playbook already seen on the Voice chip (re-badged "TEMPEST DSP 1.0" on
the silkscreen) applied differently: here, instead of relabeling in
silkscreen, DSI covered the chip's own factory-printed marking directly.
**Practical implication: no clearer photo of this specific unit's `U45` will
ever reveal the part number** - the marking is physically obscured, not
merely blurry. A different unit from a different manufacturing batch
(possibly without the label applied), the label peeled back, or the chip's
topside markings visible from a raking/angled light photo are the only ways
this specific angle could still resolve. Reasonable circumstantial evidence
this deliberate concealment happened *at all* is itself worth noting: DSI
apparently wanted to obscure their MCU sourcing for both Voice (silkscreen
re-badge) and, if `U45` really is Main/Panel's PIC32, this chip too (opaque
label) - suggesting board-level obfuscation was a deliberate choice, not
incidental.

**Searching for the exact PIC32 part number came up empty, and this looks
like a real dead end for the search-based approach specifically:**

- `fccid.io` and the FCC's own legacy equipment-authorization search are
  both impractical to query this way (bot walls on the former; the latter's
  legacy form needs a grantee code as a starting point, which isn't known).
  **Checked the actual manual text directly (not just searched for it): the
  Tempest's FCC section is only the generic Part 15 Class B "verification"
  statement, no ID code or grantee code anywhere.** That tier of compliance
  applies to wired-only devices with no RF transmitter, which typically
  never receive an individual FCC ID at all - this is likely why the FCC
  equipment-authorization search came up empty, not a search-quality
  problem. Treat the FCC-ID angle as structurally closed, not just
  under-searched.
- No schematic, service document, or FCC filing is indexed anywhere under
  the exact board revision `"DSI-800R"`.
- Direct `"PIC32"` + Tempest photo/text searches turn up essentially
  nothing relevant.
- Checked marketplace/parts sources for board-level photos beyond the one
  Facebook album: Syntaur has no parts listed for the Tempest at all;
  Reverb's marketplace search for board/parts listings returns only knobs,
  rack ears, and unrelated EPROM listings; a YouTube search for
  teardown/repair content returned only demo and tutorial videos. eBay was
  not checked past its bot-detection challenge page (not attempted to
  bypass). None of these turned up a second photographed unit.
- The 2010 board date does at least confirm the chip must be a **PIC32MX**
  family part, not PIC32MZ (which wasn't released until ~2013) - narrows
  which vector-spacing convention applies, but doesn't give the specific
  variant.

**Two possible physical debug-access points spotted in the hi-res photo,
neither confirmed:**

- A **6-pin single-row header labeled "P4"**, positioned directly beside
  `U45` and its neighboring `dream`-logo chip - i.e. right where you'd route
  short traces for an in-circuit debug connection to the MCU cluster. Six
  pins matches Microchip's standard ICSP header count exactly (`MCLR`,
  `VDD`, `VSS`, `PGD`, `PGC`, plus one more, often `AVDD` or NC). No
  pin-label silkscreen legible at this photo's resolution to confirm the
  signal names.
- An **RJ11/RJ12-style modular jack** (6 gold contacts) on the panel/button
  board, mounted right where the inter-board ribbon cables also connect.
  Genuinely ambiguous from a photo alone - could be a compact internal
  wiring-harness connector (common in consumer gear, e.g. to a separate
  display sub-board), or could be a service/debug port using a phone-jack
  shell instead of a pin header (PIC32's 2-wire ICSP plus power/ground fits
  comfortably in 6 conductors, so this isn't implausible either).

**If physical access to a unit is available, this is a categorically
stronger path than anything else in this section**: continuity-testing
either connector against the known PIC32 ICSP pinout with a multimeter, or
simply trying a PICkit/ICSP programmer against "P4" directly, would read the
chip's device ID straight from silicon - settling the exact part number
outright - and could enable a full flash dump with real, hardware-reported
addresses, bypassing the entire base-address problem in §5 rather than
solving it computationally.

### 4.1 DSI reused this control platform across other products - a real, independent lead

The Tempest is not the only DSI/Sequential product built around a PIC32
control board, and some of the others are far more actively discussed and
modded by the community than the Tempest is - meaning their exact chip may
already be identified somewhere the Tempest's isn't.

- **A Gearspace repair thread confirms the OB-6 and Prophet-6 "share the
  same architecture"** at the main-board level (the poster replaced a
  failed OB-6 mainboard - which also carries the USB port - under a $25
  DSI service exchange). The thread's own attached photos are dead links
  (2018-era third-party image hosting, since removed) - a checked, real
  dead end for *that specific thread's photos*, not for the underlying fact
  it confirms. **Checked archive.today directly for a cached copy with the
  images intact, both URL forms (the old `/board/...html` redirect and the
  canonical `/threads/...` URL) - no snapshot of either exists.** Genuinely
  closed, not just unfetched.
- **A ModWiggler teardown thread of a Dave Smith Mopho keyboard** (an
  earlier, simpler DSI product, ~2009) states in plain hobbyist language:
  *"The mainboard, voice + controlling microcontroller (PIC32), is quite
  small. A dsPIC seems to be used for the DCOs."* This independently
  corroborates PIC32 as DSI's standard control MCU choice well outside just
  the Tempest, from a source with no connection to this investigation. The
  thread has an attached close-up main-board photo (`IMG_3504.jpg`,
  captioned "close-up of the main board") that could plausibly show a
  legible part number - **gated behind free ModWiggler forum registration,
  not publicly viewable, so not fetched.** If accessible another way (a
  ModWiggler account), this is a concrete, specific image already known to
  exist, not a speculative search. **Also checked archive.today for a cached
  copy - no snapshot of this specific thread exists, and more tellingly,
  several other ModWiggler snapshots archive.today *does* have are
  themselves titled "MOD WIGGLER - Login."** The registration wall applies
  to archiving crawlers too, so even a cached snapshot of this thread would
  most likely preserve the login page, not the photo - this angle is closed
  on both fronts, not just the direct-fetch one.
- **Sequential hosts official, photographed main-board removal guides** for
  several *current* products (Prophet-5/10, Prophet X/XL) at predictable
  URLs (`sequential.com/<product>-main-board-removal/`) - confirmed no
  equivalent page exists for the Tempest (`sequential.com/tempest-main-board-removal/`
  404s, and a site search for "tempest main board" returns nothing
  relevant), consistent with the Tempest being long discontinued and
  outside their current support-doc priorities. Worth checking the *current*
  product guides' own photos for a legible PIC32 if the goal shifts from
  "identify Tempest's exact chip" to "identify DSI's standard chip across
  the whole platform lineage" - not done this session, since the Prophet-5/10
  reissue postdates the Tempest by roughly a decade and may use a different
  chip generation even if architecturally similar in spirit.

**Practical implication:** if asking in the Tempest-specific Gearspace
thread (§7) doesn't get a response, the OB-6/Prophet-6/Mopho communities are
larger, more active, and more likely to have already done this identification

- worth asking there too, framed as "what control MCU does the OB-6/Prophet-6
use" rather than Tempest-specifically, given the shared-architecture claim.

### 4.2 Downloaded and directly compared OB-6 and Mopho firmware - real progress, still not a full solve

Followed up on §4.1 by actually pulling the two sibling products' official
firmware and running them through the same pipeline as Tempest's, rather
than relying on secondhand forum claims. Both downloaded from official
sources: `Mopho_Main_1.4.syx` (`sequential.com`, 2010 release) and
`OB6_Main_1.8.0.syx` (`oberheim.com`, 2024 release - the current OS as of
this session, giving a much newer toolchain build than Tempest's 2017
`1.5.0.2`).

**Wire format is shared platform-wide.** Both files use `F0 01 <device
byte> <type byte>` headers structurally identical to Tempest's (`01 28`) -
Mopho uses `01 25`, OB-6 uses `01 2e`. Stripping the same 4-byte header and
running the same collector-first 7+1 unescape (`sysex.Unescape7Plus1`)
against OB-6's payload works cleanly. This is useful independent
confirmation that this repo's wire-format implementation is a real,
general DSI/Sequential convention, not something reverse-engineered to fit
Tempest specifically.

**OB-6 decodes as clean MIPS32/PIC32 immediately, and gives a stronger
address clue than anything found for Tempest.** Byte 0 of OB-6's unpacked
payload is an unambiguous reset stub:

```
lui $k0, 0x9d0c
addiu $k0, $k0, 0x4680
jr $k0
nop
```

Unlike Tempest's `jal`-based stub (§5.4, which only encodes 28 of the
target's 32 bits, forcing a guess at the top nibble), this `lui`+`addiu`
pair spells out the **full 32-bit target explicitly: `0x9D0C4680`**. This is
a real, independent confirmation that `0x9D` (KSEG0 cached program flash) is
the correct segment convention for this whole DSI/Sequential platform -
previously only an assumption for Tempest, now directly demonstrated on a
sibling product's own firmware.

Tried to use this to actually pin down a base address: computed the naive
file offset (`0xC4680` under a `0x9D000000` base) and disassembled there -
it doesn't land on clean, recognizable code. Swept ±8KB around that offset
looking for a real function prologue (`addiu $sp,$sp,-N`) - no clear winner,
same inconclusive shape as every base-address attempt in §5. **This sub-problem
remains open for OB-6 too, not just Tempest** - the full-address confirmation
narrows *which segment*, not *where in the file*.

**Also generalizes a Tempest-specific finding into a platform-wide one:**
searched OB-6's firmware - built in 2024, a vastly newer toolchain than
Tempest's 2017 build - for the stock XC32 NMI-check preamble (§5.4, §5.5).
Still absent, same as Mopho and Tempest. **"DSI writes custom startup
assembly instead of linking the stock library crt0.o" is not a Tempest
quirk or a toolchain-era artifact - it holds across at least three products
spanning 2010 to 2024.** This closes the door a little further on ever
finding a stock-crt0 reference to diff against, for any product in this
family, not just this one.

**Mopho did not decode cleanly at all.** Tried header lengths 3 through 7
bytes (in case this older, simpler 2010 product uses a different header
convention than Tempest/OB-6) - none produced a recognizable reset-stub
pattern the way Tempest and OB-6 both did immediately. Most likely
explanation: Mopho, the oldest and simplest product in this lineup, uses a
genuinely different wire or escape format - not investigated further this
session given time already spent; a real open question if picked up again,
not a dead end that's been ruled out.

## 5. The base address problem

This is the actual blocker. To do anything useful with Ghidra (cross-reference
strings back to the code that uses them, read the Export Beat logic, etc.),
the tool needs to know the correct virtual load address the firmware is
compiled/linked against - loading it at the wrong address makes every
absolute address reference in the code resolve to nonsense.

**What's confirmed:** Microchip's standard PIC32 convention places the
exception vector base (`EBase`) at `0x9D000000` (KSEG0, cached program
flash), with the general exception vector at `EBase + 0x180` per the
`procdefs.ld` linker script convention (confirmed via Microchip's own
developer documentation and a real example linker script, though for a
different, smaller-flash PIC32MX variant than whatever Sequential actually
used - the *convention* transfers, the *exact vector table size* may not).
Panel's firmware shows COP0 (`mfc0`/`mtc0`) activity clustered in a pattern
plausibly consistent with this (early activity around file offset 0x1c-0x38
looking like CRT0 startup register configuration, a denser cluster around
0x1a4-0x1d8 plausibly matching the `EBase+0x180` general exception vector if
file offset 0 ≈ `EBase`) - but this is "consistent with," not "proof of," a
byte-precise base address.

**Four independent attempts to solve the exact base address algebraically
were tried, and none succeeded:**

1. **Global string correlation.** For every `lui`+(`addiu`|`ori`)
   same-register instruction pair in Main's firmware (217 found - each
   yields a fixed 32-bit "computed address" from the instruction encoding
   alone, independent of any base guess), correlated against the file
   offsets of every string Ghidra's own analysis had detected (265 of them).
   Voted for `candidate_base = computed_address - string_offset` across all
   ~57,500 combinations and histogrammed the result. No spike - top
   candidate got 16 votes with a smooth, gradual falloff (16, 16, 15, 15,
   14...) indistinguishable from coincidence.
2. **Narrowed string correlation.** Same vote data, re-filtered to only a
   tight ±0x4000-byte window around the trusted `0x9D000000` guess (~5300
   candidates instead of the full 32-bit space). Still no spike - top
   candidate got 13 votes against an equally smooth gradient.
3. **Global call-target correlation.** A different, in-principle cleaner
   anchor category: `J`/`JAL` call-instruction targets (whose low 28 bits
   are fully determined by the instruction encoding, independent of base,
   assuming KSEG0's top nibble `0x9`) correlated against function-prologue
   locations (`addiu $sp,$sp,-N` patterns) rather than arbitrary string
   data, since calls should overwhelmingly target real function entry
   points. Found 5689 candidate call targets (most likely mostly spurious -
   `J`/`JAL`'s opcode is only 2 of 64 possible 6-bit values, so roughly 3%
   of *any* 32-bit word matches by chance alone) against only 29 real-looking
   prologues across the whole 525KB file. Diffuse, unconvincing top result
   (4 votes).
4. **Narrowed call-target correlation.** Same technique restricted to just
   the first 16KB (the region with the strongest independent evidence of
   being real code, near the vector table). Found only **1** prologue in
   that entire range - at file offset `0x10`, the same location originally
   guessed as "the entry point" - too few anchors for any statistical signal
   at all.

5. **Pointer-table correlation.** A follow-up session noticed that the
   twelve `"Failed to ..."` strings (§5.1) sit in one tight, contiguous
   block - the signature of a `const char* error_messages[]` table indexed
   by an error code, not twelve separately-referenced literals. Searched for
   that pointer table itself (a run of 4-byte pointers with the same
   relative spacing as the twelve string offsets) two ways: an exact-order
   search (0 hits - the table's real order likely doesn't match the
   strings' file layout) and a flexible any-order pairwise-delta search
   (found large "clusters," but they turned out to be false positives from
   unrelated fixed-stride data elsewhere in the file - e.g. a repeating
   132-byte delta matching this project's own known Sound parameter block
   size, not the error table). With twelve strings there are 132 possible
   pairwise deltas, enough combinations to coincidentally match all sorts of
   unrelated regularly-spaced data in a 525KB file.
6. **SysEx type-byte dispatch search.** Reasoned that code dispatching on an
   incoming SysEx message's type byte would need to compare against several
   of the known type values (`0x5C`/`0x5E`/`0x5F`/`0x60`/`0x61`/`0x63`)
   close together, and searched for them as small immediate constants in the
   code - a search that's base-independent by construction, unlike 1-4
   above. Found a promising-looking cluster referencing `0x5f`, `0x61`, and
   `0x63` together in the trusted early-code region - but disassembling it
   directly showed this was a **false positive**: those bytes are ASCII
   characters (`0x61`='a', `0x63`='c') being stored one-at-a-time to build
   the literal string `"Basic"` (almost certainly a default factory
   sound/beat name), not SysEx-type comparisons. **SysEx type bytes
   (`0x5C`-`0x63`) overlap the printable-ASCII range (`\`, `]`, `^`, `_`,
   `` ` ``, `a`, `b`, `c`) almost exactly, so a bare constant-value search
   can't distinguish "loading a SysEx type for comparison" from "building an
   ASCII string" at the level of a single instruction - worth remembering
   before trusting a similar-looking hit again.** Refined the search to
   require the constant be actually **compared** (`beq`/`bne`) against a
   byte **loaded from memory** (`lbu`/`lb`) nearby, a pattern that should
   only match real dispatch code - zero hits anywhere in the file. Either
   the real dispatch uses a different pattern (a computed jump table indexed
   by the type byte is plausible for a 6+-way switch), the search window was
   too narrow, or it genuinely wasn't found this way.
7. **Jump-table verification - the most rigorous attempt, and the most
   conclusively negative.** Once the toolchain was confirmed as XC32/GCC
   (§5.3), reasoned that a multi-way `switch` (exactly what SysEx dispatch,
   or any similar command dispatch, would compile to) typically becomes a
   **jump table** in GCC-generated code, not a sequence of comparisons -
   explaining why item 6 found nothing. Searched for the specific compiled
   idiom (`sll $reg,$idx,2` immediately followed by an indirect `jr`) - a
   highly selective pattern, only 10 candidates in the whole 525KB file.
   Disassembling the most promising one (file offset `0x3d0`, very close to
   the trusted early-code region) found a genuine, textbook GCC jump table:
   a range check against 40 cases, a `lui`+`addiu` computing a table base
   address, an indexed load, and an indirect jump - unambiguous real
   dispatch code, the clearest structure found all night. **Verified
   whether the table's ~40 entries resolve to valid in-file addresses - a
   much stronger test than anything before, since it requires many entries
   to check out simultaneously, not just one:** 0/40 under the standing
   `0x9D000000` assumption; a coarse full-range sweep (`0x80000000` to
   `0xC0000000`, every 4KB) found no base reaching even 5/40; a fine
   4-byte-resolution sweep across ±128KB around the trusted base found only
   6/40 at best, with no sharp peak. **Reframed to be exhaustive**: instead
   of sweeping candidate base *addresses* (unbounded), swept every possible
   file *position* the table could occupy - bounded by the file's own size
   (~131,000 word-aligned positions), covering literally every base for
   which the table could sit anywhere inside this file. Vectorized with
   `numpy`. **Zero positions in the entire file reach even 10/40 valid
   entries.** This is exhaustive, not "didn't search wide enough" - there is
   no wider window left to try for this specific table under this specific
   validity model. Most likely explanation: this jump table's entries
   aren't simple absolute pointers the way assumed (XC32/GCC can compile
   PIC-style jump tables with PC-relative deltas instead, in some
   configurations - untested) - or this particular 40-case dispatch has
   nothing to do with SysEx types at all.

8. **Direct comparison against XC32's actual crt0.S startup source.**
   Downloaded Microchip's real XC32 v6.00 macOS distribution
   (`xc32-v6.00-full-install-osx.tar.xz`, ~1.4GB) and extracted
   `pic32m-libs/libpic32/startup/crt0.S` - the literal, hand-written assembly
   every PIC32 application links against for its reset/startup sequence,
   rather than inferring it by guessing. First confirmed something genuinely
   new and solid: **byte 0 of both Main's and Panel's raw firmware decodes
   as `jal <target>; nop`** - the exact idiomatic shape of crt0's `_reset:
   jal _startup; nop` stub, with the second word being a true all-zero `nop`
   (not just any nop-decoding value) in both independent files. This is real
   structural evidence both firmware dumps begin exactly at the CPU's entry
   point. But the deeper comparison came back negative: the file's NMI-check
   preamble immediately after (`mfc0 k0,CP0_STATUS` / `ext k0,k0,19,1`,
   unconditional, unguarded by any `#ifdef`) does not appear **anywhere** in
   either binary - checked as a whole-file search for the instruction
   pattern, not just at one guessed offset. Cross-checked this preamble
   against two more independent source snapshots spanning over a decade (a
   June-2013-tagged Microchip mirror on GitHub, closest to the Tempest's own
   era, and a 2014 chipKIT variant) - identical in all three, and Microchip's
   own v2.02 (December 2011) migration notes describe what did change in
   that release (`.dinit` data-initialization tables), not this preamble.
   There's no version history suggesting this code block ever looked
   different. **Conclusion: Sequential/DSI wrote custom startup assembly
   instead of linking the stock library `crt0.o`** - see §5.4 for the full
   reasoning and what was tried to salvage a base-address candidate from it
   anyway (also negative, but for a more specific reason).

**What this consistent pattern of failure suggests:** not that any single
attempt was almost right, but that blind statistical correlation over a raw
firmware blob doesn't have enough structure to solve this without a real
linker map, debug symbols, or confirmed code/data segment boundaries. Further
variations on the same class of technique are unlikely to succeed where
eight already haven't, several exhaustively - this is now a considered
conclusion, not something to re-attempt without new input.

### 5.1 The error-message string table

A broader, cleaner `strings` pass over Main's firmware (filtering to
readable-only runs) turned up more relevant strings than the original five:
`"' loaded from MIDI"`, `"' loaded into Beat "`, `"Receiving MIDI Data... "`,
`"MIDI Buffer Overflow"`, `"MIDI Status Byte"`, `"Send Main OS"`/`"Send Panel
OS"`/`"Send Voice OS"`, `"Beat saved to file: "`/`"Sound saved to file:
"`/`"Project saved to file: "`, and more `"Failed to read/write
beat/project/sequence sounds/params"` variants beyond the original five.

Computing file offsets for all twelve `"Failed to ..."` strings shows they
sit in one tight, contiguous ~360-byte block (`0x6c2ae`-`0x6c43f`) - clearly
one shared error-message table indexed by an error code, not twelve
independently-referenced literals. **This directly explains why the original
per-string reference search (item 1 above) found nothing**: code that does
the equivalent of `puts(table[error_code])` only references the table's
*base* once, not each string individually - searching for twelve separate
direct references was never going to succeed regardless of the base address
guess. Worth keeping in mind for any future string-based correlation
attempt: check whether target strings cluster together (implying a shared
table, one reference) before assuming each one has its own reference site.

**What would actually unblock this**, roughly in order of how much new
information each would provide:

1. Sequential's *actual* PIC32 variant and its real (not generic-example)
   linker script/vector table layout - via product documentation, a service
   manual, or the Tempest hacking community having already solved this.
2. A legible photo of the Main or Panel chip's part marking (§4 - not found
   yet, but the search wasn't exhaustive; Panel's board specifically hasn't
   been photographed/found).
3. Comparing this firmware's header against an **older OS version's**
   header (the same technique that found the Main/Panel structural overlap
   in §3, applied across versions instead of across processors - a real
   address/config field should stay constant across versions where a
   version number or checksum would change). Attempted, but blocked by
   circumstance rather than ruled out: the Internet Archive was returning a
   real "temporarily offline" outage during this session (not a bot block -
   an actual service outage), and Sequential's live download page only
   hosts the current `1.5.0.2` release with no older versions linked. Worth
   retrying later, or looking for a community-hosted mirror of an older
   `Tempest_Main_*.syx` file. **Re-checked in a later follow-up session, via
   direct `curl` against the raw CDX API, not just the browser - still
   returning the same outage page. This is a genuine multi-hour-plus outage,
   not a one-off blip; worth checking again another day rather than
   retrying repeatedly within the same session.**

### 5.2 UI menu/screen text isn't stored as plain strings in any of the four firmware files

A natural-seeming idea: search the manual's own menu and screen names (`"16
Sounds"`, `"16 Beats"`, `"Quantize"`, `"Swing"`, `"Beat Events"`, `"Export
Beat"`, etc.) against each firmware's string table, in case any of them
sit near the code that would be worth reading. Tried this properly - pulling
phrases *from the manual* and searching for them in the binaries, not just
reasoning about whatever `strings` happened to already turn up - across all
four firmware files. **None of the major UI screen/menu names appear as
plain ASCII text anywhere, in any of the four files.** Only a handful of
Main's already-known backend strings matched (`"Init Beat"`, `"Init
Sound"`, `"Format Flash"`, `"Soft Key"` - all file-operation strings, not
UI labels). Panel, Voice, and SAM's entire `strings` output is 100% noise -
not one real English word in any of them, at any length threshold tried.

This is a real, informative negative result, not a failed search: **the
Tempest's on-screen menu text is not stored as plain readable strings in any
of these four OS update files.**

A follow-up session tested two specific encoding hypotheses for where that
text actually lives, both cleanly ruled out:

- **Hex-encoded text/identifiers.** Searched Main's firmware for long runs
  of pure hex-digit ASCII bytes (`0-9A-Fa-f`), which would indicate
  hex-encoded strings or build identifiers. Zero runs of 8+ consecutive
  hex-digit bytes found anywhere in the file.
- **UTF-16 encoding.** Microchip's **Graphics Composer** tool (part of
  MPLAB Harmony / the Microchip Graphics Library - a real, documented
  "String and Font interface" plus "Asset Manager," built for exactly this
  kind of embedded-LCD UI) commonly stores localizable UI strings as UTF-16
  rather than 8-bit ASCII, which would explain why a plain `strings` scan
  (looking for 8-bit character runs) found nothing even if the text were
  otherwise literal. Wrote a manual UTF-16LE/BE scanner and ran it against
  both Main and Panel in both byte orders - **zero hits, every combination.**

**Current best explanation, not confirmed:** the UI text is most likely
stored via Graphics Composer's own **proprietary compiled-asset format**
(documented to exist - it compiles imported strings and fonts into a binary
asset format, not plain string literals) rather than any of the byte-level
text encodings tested here. Decoding that specific format would need its
own dedicated research effort and isn't obviously worth it for this
project's actual goal - the Export Beat / sequence-data code already lives
in one of Main's plain-text-readable regions, so this doesn't block that
work. Worth remembering as an open question if UI text ever becomes
directly relevant.

### 5.3 Toolchain era, confirmed

Researched what PIC32 development actually looked like at the time the
Tempest was built, since assumptions about the toolchain (linker
conventions, standard library behavior) underpin several of the guesses
above. **Confirmed via Microchip's own documentation:** in 2010 (the
Tempest mainboard's silkscreen date, §4), PIC32 development used **C**,
compiled with Microchip's **MPLAB C32** compiler, inside the original
**MPLAB IDE v8.x** (native Windows, predating the NetBeans-based MPLAB X).
Microchip's unified **XC32** compiler and **MPLAB X IDE** became standard
around 2011-2012 - there was no XC32 yet in 2010, and no official C++
support.

This matters for dating the specific binary analyzed here: `Tempest_Main
OS 1.5.0.2` was uploaded by Sequential in **2017**, well after Microchip
moved on from C32. It was almost certainly built with the newer XC32
toolchain, not the original 2010-era C32 - which validates (rather than
merely assumes) the XC32/MPLAB-X-era conventions this investigation has
been relying on elsewhere (e.g. the `procdefs.ld`-style linker script
layout in §5).

### 5.4 Stock crt0 comparison - ruled out, with a specific reason why

Following up on item 8 in §5's list: since the stock XC32 NMI-check preamble
is absent from both binaries entirely, the next question was whether a
self-consistent *candidate* base address could still be extracted from just
the confirmed `jal _startup`-shaped stub at offset 0, without needing the
rest of crt0 to match.

The `jal` instruction's target only fixes 28 of the target address's 32 bits
(the top 4 come from the surrounding segment, not the instruction). Working
backwards from the two known real PIC32 KSEG0/KSEG1 program-flash base
values (`0x9D000000` cached / `0xBD000000` uncached - same physical memory,
same low 28 bits either way) as the assumed value of file offset 0:

- Main's `jal` target resolves to file offset `0x3CC14` (246,292 - within
  Main's `0x80141`-byte size).
- Panel's `jal` target resolves to file offset `0x668` (1,640 - within
  Panel's `0x10029`-byte size).
- Both in-range, and the ~0x3C5AC-byte gap between them is internally
  consistent (falls straight out of the same base assumption applied to both
  files independently).

This looked promising enough to check directly - but disassembling both
predicted offsets shows byte patterns that don't decode as clean, sensible
MIPS code (no recognizable prologue, no `mfc0`/`ext` signature, nothing that
reads as real instructions). **Being "in file bounds" turned out to be a
weak filter, not real confirmation** - satisfied by construction for most of
a 512KB candidate space. Given the crt0 comparison already independently
established this is custom startup code, not the stock library, there's no
more mileage left in chasing the *stock* `_startup` target specifically -
whatever function the `jal` actually calls is Sequential's own code, with no
public reference to compare it against.

### 5.5 Vendor/library signature search - a different question than base

address, also negative

Not another base-address attempt - a separate check for whether any
identifiable third-party chip or USB stack leaves a recognizable signature
(a name string, a register constant, a descriptor structure) anywhere in the
firmware, which could help independently of solving addressing.

**Chip-driver strings.** Searched all four firmware string dumps for the
brand name of every chip legible in the board photos (§4) - `ISSI`, `dream`/
`SAM37`, `PIC32`/`Microchip`/`XC32` - plus a broad sweep of other likely
memory/codec vendor names (Winbond, Micron, Atmel, Cirrus, Wolfson, etc.) in
case of a surprise match. **Zero hits, anywhere, for any of them.** This is
the expected result, not a surprising one: driver code for a memory-mapped
external SRAM chip is normally just direct register/bus access with no
descriptive strings, unlike a USB stack or RTOS that logs debug messages -
and the `dream`/`ISSI` chips are separate silicon with their own firmware
images, so there'd be no reason for their names to appear inside Main's or
Panel's string table regardless.

**USB code.** Main's firmware does contain one genuine, DSI-authored string
confirming USB-related application code exists: `"USB not connected"`. But:

- No Microchip USB-stack internal strings anywhere (`endpoint`, `descriptor`,
  `MCHPFSUSB`, class-specific terms) - consistent with either debug strings
  being compiled out of a release build, or a minimal custom USB layer
  rather than the full Microchip Application Library stack.
- Searched the raw bytes of all four files directly for an actual **USB
  device descriptor structure** - its first several fields are tightly
  constrained (`bLength=0x12`, `bDescriptorType=0x01`, a valid `bcdUSB`
  value, a valid `bMaxPacketSize0`), so a real match would be a strong,
  base-address-independent anchor, and would hand over the real
  `idVendor`/`idProduct` directly. Checked both strictly and with the
  constraints loosened - **zero real matches in any file** (120 raw `12 01`
  two-byte coincidences exist in Main alone, expected given how common those
  byte values are in MIPS instruction encoding, but none are followed by a
  plausible USB version field).
- Checked the public `usb.ids` database and general web search for a
  documented DSI/Sequential/Tempest USB vendor ID - no match. DSI most
  likely never registered their own VID (a real cost many small
  manufacturers skip), which also explains why there was nothing to
  pattern-match against even if a descriptor had been found.
- **Most likely explanation**: the USB descriptor table is probably built
  programmatically in code (byte-by-byte) rather than stored as a literal
  byte sequence, or it lives in a factory-provisioned/bootloader region not
  included in these field-updatable `.syx` OS images at all - similar to why
  the full reset/exception vector table isn't fully present either (§5.4).

### 5.6 The real chip family, found via Microchip's own datasheet - base address confirmed, but two independent jump-table tests against it still fail (2026-10-17)

Previous attempts (§5.1-5.5) all tested `0x9D000000` as a *borrowed* base
address - taken from a generic PIC32MX210 linker-script example with no
connection to this specific product. This session found the real chip
family and confirmed the number for real, but that still wasn't enough to
unlock disassembly - documented here in full since it changes *why* the
base address is trusted without changing *whether* decompilation works yet.

**The real chip family, identified for the first time.** Downloaded
Microchip's actual **PIC32MX5XX/6XX/7XX Family Datasheet** (DS60001156)
and the **MIPS32 M4K Processor Core Software User's Manual** (MD00249) -
real documents, not examples. Two independent facts, both confirmed:

1. The MIPS32 architecture itself fixes the Reset/Soft Reset/NMI exception
   vector at `0xBFC00000` unconditionally (not configurable), and `EBase`
   (CP0 register 15, select 1) resets to `0x80000000` with bits 31:30
   hardwired to `2#10` - explaining *why* software reprograms it into the
   `0x9D`-`0xBF` range, not just that it does.
2. **The datasheet's own memory-map figure (Figure 4-5, which specifically
   covers the 512KB-flash 575/675/775 variants) gives real, chip-specific
   numbers for the first time**: Program Flash = virtual
   `0x9D000000`-`0x9D07FFFF` (KSEG0) / physical `0x1D000000`-`0x1D07FFFF`,
   exactly 512KB. Boot Flash = virtual `0x9FC00000`/`0xBFC00000` / physical
   `0x1FC00000`, ~12KB, ending in 16 bytes of Device Configuration
   registers (`...2FF0`-`...2FFF`).

**Flash-size math narrows the real chip to exactly 8 part numbers.** PIC32MX
(M4K core) tops out at 512KB of flash - anything bigger needs PIC32MZ
(microAptiv core), ruled out by the board's 2010 silkscreen date predating
that core by years. The family datasheet lists exactly 8 parts at 512KB,
split by extra peripherals: **575F512H/L** (USB+CAN), **675F512H/L** and
**695F512H/L** (USB+Ethernet), **775F512H/L** and **795F512H/L**
(USB+Ethernet+CAN). The Tempest has neither an Ethernet nor a CAN port, so
575 is the best-supported single guess if DSI picked the cheapest part
meeting their actual I/O needs - but a shared-SKU product line could still
mean any of the 8, which doesn't matter for the memory map, identical
across all of them.

**A genuinely new quantitative cross-check, and it fits almost exactly.**
512KB Program Flash = 524,288 bytes exactly. Main's own confirmed decoded
size is 524,609 bytes (`0x80141`) - only **321 bytes over**. Too small to
also include the ~12KB Boot Flash region, consistent with (not proving) the
standing assumption that the `.syx` OS-update image is Program-Flash-only.

**Correction, same session, follow-up delegated task: the 321-byte overage
is not a meaningful trailer - it's erased-flash padding, and real content
actually ends *before* the 512KB mark, not after it.** Checked the actual
bytes: the last byte of Main (file offset `0x80140`) is `0x3A`; the last 512
bytes measure ~0.04 bits of entropy (essentially constant). The file's last
long `0xFF` run spans decimal offset 523,796-524,604 (809 bytes) - starting
**492 bytes before** the exact-512KB mark (524,288) and continuing 316
bytes past it, straddling the boundary, followed by only three stray
non-`0xFF` bytes before the literal end. Decode density in that final
region is 0/203 = 0%. Bigger picture: the last ~27KB of the file (from
~offset 497,532 on) is dominated by repeated ~1024-byte `0xFF` blocks (33
runs ≥100 bytes found total) - classic unprogrammed/erased-flash padding,
not content. **The compiled image doesn't fill the full 512KB at all; real
content tapers off around offset ~523,796, well before either the 512KB
mark or the file's own literal end.** Practical implication for base-address
work: don't trust anything in the last ~27KB of Main as meaningful, and
stop treating "the file is 321 bytes over 512KB" as evidence of a
header/trailer structure - it isn't one.

**Re-downloaded the real firmware fresh this session**
(`sequential.com/wp-content/uploads/2017/08/Tempest_OS_1.5.0.2.zip` - the
real working link found via the support page; a guessed URL from memory of
a prior session's path 404'd) and re-derived the unescaped payload from
scratch with an independent Python reimplementation of `Unescape7Plus1`,
landing on the identical 524,609 bytes - confirms the method is
reproducible from zero, not dependent on any state left over from a prior
session (none of which persists on disk between sessions anyway).

**Re-ran the already-known 40-entry jump table (file offset `0x3d0`,
computed table address `0x9D06B860`, found via `lui $t2,0x9d07` /
`sll $t1,$a3,2` / `addiu $v1,$t2,-0x47a0`) against the now-confirmed base:
still 0/40 valid** at `0x9D000000` exactly - reproduces the prior session's
negative result, now against an evidence-based base rather than a guess.
Widened to a ±128KB window (motivated by a real forum example of a
different PIC32 project reserving `0x11000` bytes for its own bootloader
before the real vector table) - best found was 12/40 at a shift cluster
around -38,300 bytes, but the actual 40 values there show a regular,
smoothly-varying pattern characteristic of structured data (most likely
embedded factory sound-parameter presets) being sampled by a sliding
window, not real function pointers - the same false-positive shape §5.5's
coincidence-rate reasoning would predict, not a real signal.

**Found a second, independent jump table with a corrected search script**
(the first attempt's script had a real bug - it required `lui`+`addiu` to
sit adjacent, but the true idiom interleaves `sll` between them; fixed to
track the destination register instead of position). The second table, at
file offset `0x603c0` (computed table address `0x9D07862C`), is a 25-case
dispatch reached via `addiu $v1,$v0,-0x5c` before the range check -
dispatching on `(something - 0x5C)`, and `0x5C` is this project's own
confirmed `TypeAltSound` SysEx byte, with the 25-case range (`0x5C`-`0x74`)
spanning every known Tempest SysEx type byte (`0x5C`-`0x63`). A genuinely
suggestive numeric coincidence - **but it also tested 0/25 valid** at the
confirmed base, and unlike the first table's near-miss, these 25 values
don't even superficially resemble addresses (mostly small values with many
zero bytes, e.g. `0x00820E12`). Treat the `0x5C` match as circumstantial,
not confirmed - 92 is also just a common small integer that could appear in
an unrelated enum.

**Panel showed a different, still-unconfirmed signal.** Panel has only 6
total `lui`+`addiu`/`ori` same-register pairs in its entire 65,577-byte
file (very sparse - most of Panel is data, not code: a per-word capstone
decode scan finds only 21.2% of the file decodes as valid MIPS32 at all).
Two of the 6 compute *physical* addresses `0x1D004740` (file offset
`0x3688`) and `0x1D004641` (file offset `0x374c`) - the datasheet-confirmed
physical Program Flash prefix, found this time in actual firmware code
rather than a datasheet. The `0x374c` site sits in a region decoding at
75-82% density versus Panel's ~21% file-wide baseline - a real,
address-independent signal that this is credibly genuine code, not
coincidence - and is immediately followed by `lbu $a2,($v0)` (a byte read
from that computed address) then a `jal` call, consistent with (not proof
of) flash read/verify code. This doesn't resolve Main's base-address
problem (different chip, different firmware), but it's a second real data
point - now found in running code, not just read from a datasheet -
supporting that the family's documented physical addressing scheme is
genuinely in use here.

**Follow-up (same session, delegated to a background agent): the NVM-unlock
search was redone exhaustively across all of Panel, and the `0x3688` site
was density-checked and traced further - clean negative on the unlock,
upgraded confidence on the site itself.** Searched Panel's entire
65,577-byte file for the standard PIC32 NVM unlock (`0xAA996655` then
`0x556699AA` into NVMKEY) two independent ways: every `lui $reg,0xaa99`/
`lui $reg,0x5566` at any disassembled position, and a raw byte-pattern scan
for the literal constants as data in both byte orders. **Zero hits, either
way, anywhere in the file** - not just near `0x374c`. This doesn't disprove
`0x374c` is flash-related code, but rules out this specific Panel image
performing a self-write under the standard unlock protocol itself; most
likely the real NVM-write/bootloader code lives outside this
field-updatable `.syx` image, consistent with the missing vector table and
missing USB descriptor pattern already established elsewhere in this doc
(§5.4, §5.5). Separately, `0x3688` (computed `0x1D004740`) density-checked
at 75-81% across three window sizes versus the same ~21.2% file-wide
baseline - equally credible as real code as `0x374c`, and in fact the same
~0x3600-0x3900 contiguous block: a full linear disassembly of that range
reads as one coherent function computing both `0x1D004740` and
`0x1D004641`, and also appeared to do `lui $t3,0x9d00` at file offset
`0x37d0`, a `0x9D00xxxx` virtual KSEG0 load nearby.

**Correction (2026-10-17, follow-up trace): the `0x37d0` "virtual address"
reference does not hold up, and the two physical-address sites are not
part of a conventional function at all.** A full trace found: (1) `$t7`
(the register holding `0x1D004740`) is confirmed dead - redefined to an
unrelated value just 2 instructions later, never actually reused; (2) the
`0x37d0` `lui $t3,0x9d00` is never consumed by any later `addiu`/`ori`
within 60 instructions, unlike the two physical-address pairs which pair
cleanly within a few instructions - it fails the exact test that validated
the other two; (3) a few words past `0x37d0`, decode hits `swc1`/`lwc1`
(FPU) and `jalx`/`bz.h` (MSA vector) instructions - impossible on the
confirmed FPU-less M4K core, concrete evidence the stream has broken down
into non-code noise right around there; (4) roughly half of all decoded
branches in this `0x3660`-`0x3900` region target file offsets outside
Panel's own 65,577-byte file entirely - a base-address-independent test
that doesn't require knowing the chip at all. **Most likely the `0x37d0`
site is a coincidental decode of non-code bytes, not a third real
reference alongside the two physical-address computations.**

A whole-file census makes this more than a local anomaly: Panel has only
**7 standard MIPS function prologues** (`addiu $sp,$sp,-N`) in its entire
65,577 bytes, all clustered in the first 1,020 bytes, and **zero**
instances of `jr $ra` (ordinary function return) anywhere in the whole
file. No conventional compiler-generated function with a frame and a
return exists anywhere past roughly file offset `0x1a00` - the two
physical-address computations are real code, but not part of a boundable
"function" in the usual sense. A file-wide search for any `jal` targeting
this fragment also found no caller. One numerically striking side-signal
(14/130 `jal`-shaped decodes landing in-bounds under a shared-low-28-bits
base, ~440x above chance) was checked and traced to the same
chip-impossible-instruction neighborhoods as above - treated as the same
kind of false positive already identified elsewhere in this doc, not a
new signal.

**Also done this session: an exhaustive search for a `$gp`-relative or
PC-relative jump table in Main, specifically to extend the two
already-tested absolute-pointer tables - clean, fairly conclusive
negative.** Per-word-decoded all of Main and found every `jr` through a
register fed by a preceding `lw` within a 20-instruction lookback - a
necessary condition for *any* table-indexed jump regardless of addressing
style, since the final step is always `lw entryreg,0(addrreg); jr entryreg`.
Only **4 such candidates exist in the entire file**: the two already-known,
already-tested tables (`0x3d0`/`0x603c0`, both already 0/40 and 0/25
against the confirmed base), and two new ones (`jr $t3` at `0x2ce58`, `jr
$k1` at `0x35060`) that turned out to match *neither* requested idiom -
`$gp` is confirmed used as an addressing base 157 times elsewhere in Main,
but never once feeds a register that reaches a `jr`; neither new site has
any `addu`/`add` between its defining `lw` and its `jr` either, so there's
no index-scaling step - they're plain indirect calls through a struct
field, not jump tables. Given the fixed-width per-word method, this is
exhaustive for a 20-instruction lookback, not merely "didn't find one yet."

**Where this leaves things.** Two independently-found, real (not guessed)
jump tables in Main now both fail to validate against the one base address
that has actual chip documentation behind it. That's stronger negative
evidence than any single-table result alone: it's not "this one table
happens to be unrelated to SysEx," it's "the simple file-offset-0-equals-
virtual-`0x9D000000` model doesn't hold for anything tried yet." Combined
with Panel's partial, physical-address-prefix-only confirmation, the
standing recommendation is unchanged and now more clearly the right call:
**the ICSP hardware test (§7 item 5) is the one path that doesn't depend on
guessing a model this many independent checks keep failing to validate.**

## 6. Tooling reference (for picking this back up)

None of this is committed to the repo (it's general-purpose reverse
engineering tooling, not project-specific code) - reinstall/redownload as
needed:

- **Firmware download:** `Tempest_OS_1.5.0.2.zip` from
  [sequential.com/support/download/tempest-operating-system](https://sequential.com/support/download/tempest-operating-system/).
  Unpack the four `.syx` files, strip the 4-byte SysEx header, and call
  `sysex.Unescape7Plus1` on the rest to get the raw unpacked firmware bytes
  for analysis.
- **Ghidra** (`brew install ghidra`, pulls in `openjdk@21` automatically).
  Needs `JAVA_HOME` pointed at the Homebrew OpenJDK explicitly - the bare
  `java` on macOS is usually just a stub. Headless analyzer lives at
  `<ghidra prefix>/libexec/support/analyzeHeadless`.
  - Import a raw binary with: processor `MIPS:LE:32:default`, loader
    `BinaryLoader`, `-loader-baseAddr 0x9D000000` (or whatever base is being
    tested).
- **PyGhidra** (`pip3 install pyghidra`, needs `GHIDRA_INSTALL_DIR` set) is
  the practical way to script Ghidra. Plain `.py` scripts passed to
  `analyzeHeadless -postScript` do **not** work in Ghidra 12.x ("Ghidra was
  not started with PyGhidra. Python is not available") - instead call
  `pyghidra.start()` then use the real Ghidra Java API
  (`ghidra.base.project.GhidraProject.openProject(...)`,
  `ghidra.program.flatapi.FlatProgramAPI`, etc.) from an ordinary `python3`
  script.
- **`capstone`** (`pip3 install capstone`) is the quickest way to
  spot-check a specific byte range without spinning up a full Ghidra
  project - `capstone.Cs(capstone.CS_ARCH_MIPS, capstone.CS_MODE_MIPS32 +
  capstone.CS_MODE_LITTLE_ENDIAN)`.

## 7. Suggested next steps

In priority order, given everything above:

1. ~~Get XC32's actual runtime/startup source and compare it directly
   against Main's and Panel's early code.~~ **Done, this session - see §5
   item 8 and §5.4.** Result: negative, but conclusively and specifically
   so - Sequential/DSI wrote custom startup assembly, not the stock
   library `crt0.o`, so there's no reference startup code left to compare
   against. This closes out "get the real source and diff it" as a
   category, not just this one attempt.
2. ~~Check whether the Internet Archive is back up, and if so, search for
   an older `Tempest_Main_*.syx` release to diff against `1.5.0.2`'s
   header~~ **Done, this session - see §14.** Literal `*`-wildcard CDX
   queries against `sequential.com` are still unreliable (confirmed flaky
   a fourth time), but querying with `matchType=domain` plus a regex
   `filter` against DSI's **original pre-rebrand domain**,
   `davesmithinstruments.com`, worked immediately with no retries and
   returned 5 additional real firmware releases spanning 2011-2017. This
   closes out the "need an older release" blocker as a category - if
   anyone needs more old releases later (other products, later gaps), use
   this query pattern, not literal wildcards against `sequential.com`.
3. **Look specifically for a legible photo of Panel's board** - smaller and
   likely less crowded than the analog voice board already found, and per
   §3, whatever chip it uses answers the question for Main too. Two more
   targeted image-search attempts in this session found nothing - this
   angle looks exhausted for query variations specifically, not just
   under-tried; a different source (community, service manual) is more
   likely to help than another search.
4. **Ask directly in the Tempest hacking community** (the long-running
   Gearspace thread, or similar forums) whether anyone has already
   identified the exact PIC32 part or has schematics/service documentation.
   A post was drafted this session (reviewed for LLM-tell vocabulary,
   clean) and submitted by the user - **now pending a response.**
5. **Test the "P4" header and/or the RJ-style jack found on the board
   photos (§4) against the known PIC32 ICSP pinout, if physical access to a
   unit is possible.** Not yet attempted - found via photo inspection only
   this session, never physically tested. **This is now the single
   strongest lead in this whole document**: unlike every computational
   attempt in §5, a real ICSP connection reads the chip's device ID
   directly from silicon (settling the exact part number outright, no
   inference needed) and can potentially dump flash with real,
   hardware-reported addresses - bypassing the base-address problem
   entirely rather than solving it. Start with a multimeter continuity
   check against `MCLR`/`VDD`/`VSS`/`PGD`/`PGC` before risking a programmer
   connection.
6. **Ask in the OB-6/Prophet-6/Mopho communities too, not just
   Tempest-specific ones** (§4.1) - a Gearspace thread confirms OB-6 and
   Prophet-6 share the same main-board architecture, and a ModWiggler
   thread independently confirms PIC32 on the Mopho keyboard, with a
   close-up main-board photo attached that's gated behind free forum
   registration (not fetched this session). These communities are larger
   and more active than the Tempest-specific one; the same question framed
   around the more popular product may get an answer faster.
7. **Run the cheap programmatic-dump-request tests (§8)** - the standard
   MIDI universal request shapes and the DSI-style `0x2D` family, plus bare
   type bytes sent as requests. Never explicitly tested; 15 minutes with
   the hardware connected, no risk. Any response kills the "cannot be
   queried programmatically" claim and upgrades the whole MCP tooling; no
   response confirms the firmware-hook path is the only road.
8. Only after one of the above provides real new information, revisit
   cross-referencing the firmware's `"Failed to read sequence data"` and
   related strings back to their calling code - that was always the actual
   goal, not base-address-hunting for its own sake. Eight independent
   correlation/search/comparison techniques have now been tried without
   success (§5), several exhaustively or conclusively; further variations
   on the same approaches are unlikely to succeed where eight already
   haven't.

---

## 8. Cheap pre-firmware tests: programmatic dump requests (2026-09-23)

Motivation: the user asked whether the firmware could be modified to add a
programmatic dump request (send a SysEx command → the Tempest responds with
a dump). The full assessment: yes in principle but the hard road - the OS
update mechanism (§1's `0x71`-`0x74` family) proves the hardware accepts
new firmware over SysEx, the export feature already exists in the firmware
(the front panel's Export-over-MIDI triggers it), so a patch would be a
*hook* (make the SysEx dispatcher call the existing export routine on a new
message type) rather than a new feature - but it is gated on the base
address (§5, 8 failed attempts) and carries real brick risk. Before any of
that, two cheap tests can settle whether a request command already exists.
The repo's standing claim ("the Tempest cannot be queried
programmatically" - README) has never been explicitly tested with these.

**Safety note:** sending well-formed but unrecognized SysEx is harmless -
the Tempest ignores messages it doesn't handle. The one community report of
a Tempest-side error ("status byte received in data", 2013) was about
*malformed* framing, so keep every test message complete and well-formed
(`F0 … F7`). **Updated after testing (see Results below):** "harmless"
still holds - nothing broke - but a bare Tempest type-byte message (Test 3)
is not silently ignored the way an unrecognized manufacturer ID is. It
puts the Tempest into an on-screen "Receiving..." wait state that requires
a manual Cancel/Exit press to back out of. Low risk (a RAM-buffer receive,
not a flash write) but not zero-friction - be ready to cancel on the
Tempest before sending that specific probe family.

### Test 1 - standard MIDI universal dump request

Send the standard universal non-realtime request shapes, one at a time,
with the Tempest connected via USB:

```
F0 7E <device> 06 7F F7        # SDS-style "dump all" request shape
F0 7E <device> 06 00 F7        # SDS-style request, sample 0
```

`<device>` = the Tempest's device ID (try `0x7F` = all devices, and the
value the Tempest's own MIDI settings show). Procedure per message:

1. Start a capture first: `capture-tmp out.syx 2 30` (or call
   `tempest_save_received_dump` from the MCP client), *then* send the
   request - the response, if any, comes back within milliseconds.
2. Watch the Tempest's display too: some requests trigger a display change
   even when no dump follows, which is itself a signal the message was
   recognized.
3. Record: message sent, any response bytes, any display change.

### Test 2 - DSI-style request commands

Later DSI/Sequential instruments (Prophet '08/Rev2, Prophet-6, OB-6) support
programmatic dumps via a DSI request protocol on manufacturer ID `0x2D` -
the Tempest predates that protocol's documentation, but a hidden command may
exist. Candidate shapes to try (exact sub-commands vary by instrument; these
are the family to sweep):

```
F0 2D <device> 06 F7           # DSI "request edit buffer" shape
F0 2D <device> 0E <bank> F7    # DSI "request bank" shape
F0 2D <device> 0F <bank> <program> F7    # DSI "request single program" shape
```

Same procedure as Test 1. Also worth sweeping: the known Tempest data types
themselves sent as *requests* (e.g. `F0 01 28 60 F7` with no payload) - if
the firmware's dispatcher treats a bare type byte as "dump this", that is
the cheapest possible win.

### What the results mean

- **Any response**: the "cannot be queried programmatically" claim is wrong
  - wire the request into the MCP tooling (a `tempest_request_dump` tool
  preceding `tempest_wait_for_dump`) and the manual Save/Load step
  disappears from every workflow.
- **No response to anything**: the firmware-hook path is the only road, and
  it starts with the ICSP test (§7 item 5) - which is doubly motivated: it
  settles the chip identity outright AND is step 1 of any firmware
  modification (base address → locate the SysEx dispatcher → hook the
  existing export routine → recompute the bootloader checksum → test with
  re-flash-stock recovery).

### Results (tested 2026-09-25, real hardware)

All 8 candidate messages sent to a connected Tempest (Main OS 1.5.0.2,
USB SysEx cable enabled), using a throwaway `cmd/sysex-probe` tool built
for this test. Connection verified genuinely live first - via
`--midi-trace` on a pad-trigger sanity check the Tempest actually
responded to (after finding and fixing a Remote Pad IN Channel mismatch:
the hardware was set to channel 1, not the 10 assumed from the README's
example config).

- **Test 1 (standard MIDI universal dump request, `F0 7E ...`)**: no
  response, no display change, both shapes.
- **Test 2 (DSI-style request commands on manufacturer ID `0x2D`)**: no
  response, no display change, all three shapes.
- **Test 3 (bare Tempest type-bytes, `F0 01 28 60/61/5F F7`)**: no SysEx
  came back on the wire, but this is *not* the same as "ignored." The
  `0x60` probe put the Tempest into an on-screen **"Receiving New Sound in
  RAM"** state, then **"Receiving MIDI Data..."** - both confirmed
  firmware strings (§5.1) - and it stayed there until manually cancelled
  with the Tempest's own Cancel/Exit key. It was waiting to *receive* a
  Sound dump, not preparing to *send* one.

**Conclusion: no hidden dump-request feature found.** Tests 1 and 2 -
the actual "is there an undocumented request command" hypothesis - came
back genuinely silent, not just unanswered-but-recognized. Test 3's
result, read carefully, is also a negative for the hypothesis: it
confirms the SysEx dispatcher pattern-matches on
`F0 01 28 <known-type-byte>` and immediately commits to *write* mode for
that type (start receiving), with no request/dump-response mode at all
for the Tempest's own message family. This is consistent with, not
contrary to, the README's standing claim - the Tempest's SysEx dispatcher
appears to be receive-only for its own data types, and the "cannot be
queried programmatically" limitation is real, not just untested.

**Practical follow-up this surfaced:** sending a bare/truncated type-byte
message is a reliable way to reproduce the "Receiving..." wait state on
demand, without waiting for a real transfer - useful if the multi-note
research (§9 of `docs/sysex-tempest-format.md`) ever needs to test how the
Tempest's receive path behaves when a transfer is interrupted or
malformed, though that hasn't been tried yet.

**Next step, per the plan above:** the ICSP hardware test (§7 item 5) is
now the only path forward for both the base-address problem and any
future firmware-hook work - no code-level shortcut was found.

---

## 9. Programmatic dump request: firmware-hook feasibility (2026-09-23)

The user asked directly whether the firmware could be modified to support a
programmatic dump request - a SysEx command the Tempest responds to with a
dump, instead of the current front-panel-only Save/Load trigger. Assessment
from this session:

**Feasible in principle, but it's the hard road, not a shortcut.** The
Tempest already accepts new firmware over SysEx - that's the existing
0x71-0x74 per-processor OS update mechanism - and the export routine itself
already exists in Main's firmware (the front panel's Export-over-MIDI menu
calls it). A working patch would be a *hook*: make the SysEx dispatcher call
the existing export routine on a new message type, not implement new
functionality from scratch.

**What it would actually take, in order:**

1. Solve the base-address problem (§5) - the actual blocker, unchanged by
   this question. Eight independent attempts have failed; §7 item 5's ICSP
   test is still the strongest lead, and is now doubly motivated since it's
   also step one of any firmware modification.
2. Locate the SysEx receive/dispatch code in Main (the code path that
   already handles 0x71-0x74 and the sound/beat/project load messages).
3. Locate the export routine itself and its calling convention.
4. Write the MIPS32 hook (a new message type, placed in unused/padding
   space).
5. Recompute the bootloader's checksum - almost certainly a simple checksum
   given the Tempest's 2009 vintage rather than a cryptographic signature,
   but this is unverified. If it turns out to be a signature, this path
   ends here.
6. Test on real hardware, with real brick risk. Recovery path: re-flash
   stock OS over SysEx (the bootloader itself should survive a bad Main
   image) or ICSP.

Realistically weeks of disassembly work, and gated entirely on step 1.

**Try the cheap, zero-risk tests in §8 first** - a standard MIDI universal
dump request and DSI-style request commands, neither of which requires any
of the above. If neither gets a response, this firmware-hook path is the
real road forward, and it starts with the ICSP test above, which is worth
doing regardless since it also settles the exact chip identity.

**Status:** this promotes the firmware-analysis thread from background
research to active interest, per the user's own question. Also worth
asking Leviathant (TempestEdit's author, who has mapped more of the
Tempest's SysEx behavior than anyone outside DSI) whether he knows of an
existing request command - the Gearspace post asking Chris Pym is still
pending a reply (§7 item 4).

---

## 10. Front-panel control data: searched for and not found in Panel's firmware (2026-10-17)

A different angle from §5's base-address thread: does Panel's firmware
contain embedded lookup tables (pad-to-LED mappings, button scan tables,
display bitmap/font data) that correspond to the physical front panel
described in the official manual? Motivated by Panel's large "non-code"
fraction (§5.6: only ~21% of the file decodes as valid MIPS32).

**Manual facts, confirmed from the PDF text itself** (`pdftotext -layout`
on all three manual versions, grepped for panel-control language, not
inferred): **16 physical pads**, arranged "in a 2x8 configuration" (not a
4x4 grid as this doc previously assumed in places), pressure- and
velocity-sensitive, multiplexed across modes (16 Sounds, 16 Beats, 16 Time
Steps, etc.). **Two banks (A/B) of 16 pad-sounds each** - the manual's own
text states "thirty-two Sounds (sixteen in pad bank A and sixteen in pad
bank B)" per Beat, not a guess. **4 Soft Knobs, 1 Soft Key.** **2 Real Time
FX touch strips.** **"Ninety panel controls"** total (the manual's own
marketing-copy phrase). **Graphic OLED display, but the manual itself never
states a pixel resolution anywhere in any of the three versions checked**
(explicit grep for "pixel"/"resolution"/"128x"/"256x"/"OLED" - no figure in
the manual text). LEDs are only ever described as "lit"/"dimly lit"/"unlit",
never a color vocabulary anywhere in the manual. For external context only
(not from the manual, not primary-sourced for this investigation):
Sequential's own marketing copy, mirrored on multiple retailer pages,
describes a "256 x 64 dot OLED display."

**The premise behind this search turned out to be wrong, and that's the
real finding.** Re-downloaded Panel's firmware fresh and confirmed the same
65,577-byte decode as every prior session. But: **69.3% of the entire file
is the literal byte `0xFF`** - erased-flash padding, not meaningful data.
Splitting the file at the sharp density cliff already noted in §5.6
(dense/code-like `0x0`-`0x4800`, ~18.4KB; collapsed density from `0x4800`
to EOF): the tail region (72% of the file) is **95.6% `0xFF`**, while the
front code region is only 2.0% `0xFF`. The "79% of the file is data"
framing that motivated this search was itself misleading - almost all of
that "data" is blank padding, not embedded tables.

**Exhaustive checks, all negative:**

- Longest contiguous non-`0xFF` run anywhere in the padding tail: only 302
  bytes - nowhere near enough for even a 1-bit-per-pixel 256x64 bitmap
  (needs 2048 bytes), let alone a font set at that resolution.
- No identity sequences (`0..15`, `0..31`), bitmask tables
  (`{1,2,4,...,128}`, either direction), or constant-offset monotonic
  16-byte runs (every offset 0-255 tried) anywhere in the file.
- Byte-level autocorrelation at every manual-derived control count (2, 4,
  16, 32, 64, 90, 128, 180, 256), restricted to the code-only region to
  exclude the trivial padding autocorrelation: no standout peak at any lag,
  smooth decay indistinguishable from ordinary code.
- Immediate-constant loads (`addiu`/`ori`/`li`/`slti`/`sltiu`/`andi`) of the
  exact manual-derived counts (2, 4, 16, 32, 90) in the code region: 0-9
  hits each against 464 total such instructions - `90` got zero hits at
  all - noise-level for integers this structurally common, not a signal.

**Conclusion: a clean, thorough negative, not an under-search.** No data
region in Panel's firmware shows size, structure, or periodicity matching
any physical-panel count from the manual. If pad-scan/LED-driver logic
exists in this file at all, it's most likely *code* within the dense 18KB
region, not a static table - and reading that code meaningfully still
needs the base-address problem (§5) solved first. Equally possible: this
kind of data simply isn't included in the field-updatable `.syx` OS image
at all, the same explanation already used elsewhere in this doc (§5.5) for
the missing USB descriptor and the missing full vector table.

**Not fully characterized, low priority**: a handful of small (<302-byte)
non-`0xFF` islands scattered through the padding tail - not bitmap/font
shaped, not ruled out as containing something minor like a calibration
constant, not pursued further given their size.

---

## 11. Searched for leaked build artifacts from the original toolchain (2026-10-17)

A different kind of lead than anything tried in §5: rather than computing
the base address from the firmware itself, could an original MPLAB/XC32
build artifact - an `.elf` file, a `.map` linker file, an MPLAB X project
(`.X`/`nbproject/`), or an MPLAB 8.x project (`.mcp`/`.mcw`) - have ever
leaked publicly, for the Tempest or a sibling DSI/Sequential product on the
same platform (OB-6, Prophet-6, Mopho, per §4.1)? Any one of these would
hand over the exact load base address directly via its section headers -
worth a dedicated search even as a long shot.

**Clean negative across 20 targeted queries spanning five angles**: direct
leaks/leftovers (general web and GitHub/GitLab-scoped search for
`.elf`/`.map`/MPLAB-project terms combined with Tempest/DSI-800R), the
sibling-product angle (OB-6/Prophet-6/Mopho firmware or project files),
DSI/Sequential patent filings (Google Patents, Justia - the only hits were
two 2022-2025 Sequential Synths Limited *design* patents, post-rebrand,
ornamental-only, naming Dave Smith as one of four inventors but containing
no schematics or chip part numbers - nothing from the relevant 2010-2011
era at all), a standalone firmware-update utility separate from the MIDI
SysEx path (none found - the MIDI-only path this project already assumes
is consistent with everything found), and service manuals/schematics with
a legible chip designation (none found for the Tempest itself).

**One real, tangential find worth keeping**: a full-resolution (2707x1800)
photo of the shared OB-6/Prophet-6 main board pair exists publicly - imgur
album `MBGwP`, linked from a 2019 Matrixsynth post
(`matrixsynth.com/2019/11/inside-sequential-prophet-6-and-ob-6.html`) -
corroborating §4.1's "shared main board" claim with an actual image rather
than only a secondhand forum quote. Board silkscreen reads `PCB: DS-1001
4R P2`, `ASSY: DS-9060`. Each board has two large (~100-144 pin by eye) QFP
packages side by side, the obvious MCU/FPGA candidates - but their part
markings are not legible at this photo's resolution even after an 8x
upscale plus autocontrast/sharpen pass. This is a *calibrated* negative,
not an under-effort one: a nearby small-signal component's silkscreen
reference designator (`U9`) is crisply legible at the same crop
resolution, so the photo is adequate for high-contrast white silkscreen
ink but genuinely insufficient for the low-contrast marking on a dark chip
top - not proof the chips are deliberately blacktopped, just evidence this
specific photo can't resolve them either way. Worth rechecking if a
higher-resolution version of the same album ever surfaces.

**Where this leaves things**: no software or documentation shortcut exists
for the base-address problem - this was always a long shot, and it's now a
confirmed one, not an abandoned one. The ICSP hardware test (§7 item 5)
remains the one recommendation this search adds no alternative to.

---

## 12. The Graphics Composer asset format, found and tested - clean negative, plus a real reason to doubt the whole theory (2026-10-17)

Follow-up to §5.2's open question (UI text isn't plain ASCII, hex, or
UTF-16 anywhere in the firmware - "most likely explanation, not confirmed"
was a proprietary Graphics Composer asset blob). This session found an
actual documented format and tested it properly, rather than leaving it at
"documented to exist."

**A real binary layout exists, straight from Microchip's own source.**
Cloned Microchip's public `Microchip-MPLAB-Harmony/gfx` GitHub repo and
found `stringtable.py`, the actual asset-generator script, which contains
an explicit byte-layout comment for the string-table blob it emits: a
2-byte index count, 1-byte index size, 1-byte language count, an index
array of (font ID + offset) entries, and length-prefixed string data at
each offset. Real, not guessed.

**Applied it as a structural search, validated against a noise-baseline
control - clean negative.** Brute-forced every byte offset in both Main
and Panel as a candidate table header (generous ranges: 2-400 indices,
1-3-byte index size, 1-4 languages), scored how many index entries resolve
to plausible strings. At a 50%-resolve threshold: 0 candidates, either
file. Relaxed to "any fraction, >=4 entries": Panel's best was 0%; Main's
best was 2.7% (7/257, all `"\t"` repeats, not real words) - and a control
scan of same-size `os.urandom` data found **0 candidates even at the
relaxed threshold**, confirming Main's weak 2.7% is itself indistinguishable
from the kind of incidental printable-byte runs ordinary compiled code
produces, not a real signal. None of the known manual anchor phrases
(`"16 Sounds"`, `"Quantize"`, etc.) appear in any candidate.

**A genuinely important caveat, worth treating as a correction to the
standing theory, not just a footnote**: the specific format tested
("Legato," Harmony 3-era) launched around 2017-2018 - right at or after
Tempest's 2017 firmware build date, meaning it's plausibly *too new* to be
what actually built this firmware. Further research found that the
*older*, era-appropriate tooling (GOL/GRC, used with MLA/early-Harmony,
circa 2008-2012 - the right generation for a 2010-designed product) works
completely differently: **it compiles assets into ordinary `.c`/`.h`
source files, linked normally into `.rodata`, not an opaque binary blob at
all.** This is a real argument against the "proprietary asset blob"
explanation that's been the leading theory since §5.2, not just an
unresolved detail - if GOL/GRC is what Tempest actually used, UI text
would be ordinary string constants in `.rodata`, indistinguishable from
any other string without solving the base-address problem to read them in
context, not hiding behind an undecoded format at all. Not pursued further
this session (the GOL/GRC source/spec itself wasn't located), but worth
treating as the more promising lead than continuing to chase the Legato
hypothesis, if this thread continues.

**Display driver IC: SSD1322 is a strong circumstantial candidate, but
nothing confirms it.** Re-confirmed (independently) that none of this
repo's 5 board photos show the OLED display or a nearby driver chip - the
photographed "panel" board is specifically the button/pad matrix, which
may not even be the same physical board as the display per §4's existing
caveat. Researched candidate controllers: **SSD1322** (Solomon Systech)
fits on three independent grounds - commercial 256x64-pixel OLED modules
built specifically around it exist and are well-documented (Newhaven
NHD-3.12-25664/NHD-2.8-25664 families), its datasheet's relevant revision
(v1.2, Jul 2010) lines up with the board's 2010 silkscreen date, and its
interface options (SPI/parallel, no I2C) are standard embedded-MCU
choices. **But it's circumstantial only**: SSD1322 doesn't appear as a
named built-in driver in any version of Microchip's Graphics Library
documentation checked (not disqualifying - a custom driver against the
library's generic display abstraction is normal - but it means "the
library supports it" isn't corroborating evidence either). A firmware-side
check (searching Panel for literal SSD1322 init-command byte sequences,
including the specific `CA 3F` "64MUX" value a genuine 256x64 panel would
use) found **zero hits anywhere**, including in the dense 18.4KB code
region where a literal init table would most plausibly live - consistent
with (not proof of) the same "not present in this field-updatable image"
explanation already used elsewhere in this doc.

**Where this leaves things**: a real format was found and properly
eliminated (with a noise-baseline control, the standard this doc holds
itself to) rather than left as a vague "probably a binary blob." The more
important output is the correction to the underlying theory - GOL/GRC-era
plain `.rodata` strings is now a better-supported hypothesis than an
undecoded proprietary blob, and is worth checking first if this thread
continues, rather than continuing to chase the Legato format. Display
driver identification remains unconfirmed either way; a physical photo of
the actual display sub-assembly (not yet captured in this repo) is the one
thing that would settle it directly, same conclusion as everywhere else in
this document.

---

## 13. The bitmap asset format, confirmed from Microchip's actual source code - sharpens §10's negative rather than overturning it (2026-10-17)

A follow-up to §10 and §12: given §12 raised real doubt about which
Graphics toolchain generation Tempest actually uses (Legato/Harmony-3 is
plausibly too new; the older MLA/GRC generation fits the confirmed
2012-2017 XC32 era better - §5.3), this session went and found the *older*
generation's format directly, to give any future bitmap search real
byte-level targets instead of generic heuristics.

**Found ground truth, not documentation prose.** Microchip's MLA Graphics
Library source is public on GitHub (`MicrochipTech/mla_gfx`, Apache-2.0) -
cloned it and read the actual runtime decoder C code, plus a real Graphics
Resource Converter (GRC v4.00.39, 2013) output example from a separate
public repo (`luciodj/GTSU-2.0`) to see exactly what the compiler actually
emits for a real image. (The newer "Microchip Graphics Suite" doc pages
were Cloudflare-blocked and couldn't be read either way - this is
specifically the MLA/GRC format, the era-appropriate one per §12, not
confirmed against Tempest's binary directly.)

**Confirmed format**: each image compiles to two separate pieces, linked
only by C symbol name - no manifest or table of contents, matching §10's
finding that nothing manifest-shaped exists in Panel. A fixed **28-byte
`GFX_RESOURCE_HDR` descriptor** (type/location/width/height/colorDepth/
paletteID fields, exact offsets verified from the library's own headers)
pointing to a **separate raw pixel array** in its own `grc_images` linker
section. Pixel packing confirmed per bit depth directly from the drawing
routines: 1bpp (4-byte palette + LSB-first-packed rows), 4bpp (32-byte
palette + 2px/byte, low-nibble-first), 8bpp (512-byte palette + 1
byte/pixel), 16bpp (no palette, straight RGB565). RLE compression (4/8bpp
only) confirmed as a simple 2-byte-header (code,value) packet stream.
**One hardware-specific ruling: DEFLATE/IPU compression requires a
hardware IPU peripheral that PIC32MX5/6/7 (the confirmed chip family)
doesn't have** - only RLE or uncompressed is mechanically possible on this
silicon, narrowing the search space for real.

**A direct, concrete cross-check against §10's existing result - this
sharpens rather than contradicts it.** Using the confirmed packing
formulas, a full 256x64 screen bitmap is 2,052 bytes at the smallest
possible encoding (1bpp) up to 32,768 bytes (16bpp) - every one of these
is categorically ruled out, since §10 already found the longest
non-`0xFF` run anywhere in Panel's padding tail is only 302 bytes (at
least 6.8x too small for even the smallest full-screen encoding). **Only
small icon-class assets are even size-compatible**: a 32x32 1bpp icon
(132 bytes), a 16x16 1bpp icon (36 bytes), a 256x8 1bpp status strip (260
bytes), or similar. This means: if any bitmap asset exists in Panel's
firmware at all, it can only be a small icon - full screens and any
meaningful font glyph table are excluded by size alone, independent of
any pattern-matching.

**Concrete, actionable next step this generates**: §10 already flagged a
handful of small (<302-byte) non-`0xFF` islands in Panel's padding tail as
"not fully characterized, low priority." This finding changes that
assessment - those islands are now the *only* places a bitmap icon could
possibly be, and there are specific candidate byte counts to check them
against (36, 132, 160, 260 bytes for 1bpp/4bpp icons and strips at the
confirmed packing formulas) rather than generic heuristics. Worth a
focused follow-up check against those exact islands and exact sizes,
distinct from the broad heuristic sweep §10 already ran.

**Where this leaves things**: a real, source-verified format now exists to
search against if this thread continues on Main (not yet tried there) or
on Panel's remaining uncharacterized islands specifically. It does not
change the base-address problem (§5) at all - this is a separate,
narrower question about UI asset storage, not CPU addressing.

---

## 14. Five more firmware releases found - the jump-table negative now replicated across 6 independent builds spanning 2011-2017 (2026-10-17)

The long-blocked "find an older firmware release to diff against" item
(§5 item 3, §7 item 2) is finally unblocked. **The working Wayback Machine
technique, for future reference**: literal `*`-wildcard CDX queries
against `sequential.com` are still unreliable (confirmed flaky again this
session). The fix is to query with `matchType=domain` plus a regex
`filter` against DSI's **original pre-rebrand domain**,
`davesmithinstruments.com` (not `sequential.com`, which didn't exist until
~2015) - that single query returned 2086 rows with no retries needed.

**Five additional real releases obtained and verified**, each extracted
and primary-dated from its own readme/file timestamps - a real release
timeline, not guessed: **1.1 (Dec 2011) -> 1.2 (May 2012) -> 1.3.0 (May
2013) -> 1.3.0.6 (Aug 2013) -> 1.4 (Jan 2015) -> 1.5.0.2 (Aug 2017)**.

**Panel has been byte-identical since May 2013, confirmed by SHA-256, not
inferred.** `Tempest_Panel_1.3.syx`'s hash is identical across the 1.3,
1.3.0.6-bundle, 1.4, and 1.5.0.2 releases - Panel shipped unchanged inside
every Main release for 4+ years. **Sam has been byte-identical across the
product's entire released lifetime** (same hash, 1.1 through 1.5.0.2) -
never updated once. Voice changed a few times; Main is distinct in every
release, as expected for the actively-developed component.

**The 14 shared header bytes are now confirmed platform-wide and
version-invariant, not a per-build field.** Comparing all 6 Main releases
and all 3 distinct Panel releases: bytes 2-15 of the decoded header
(`40 0f 00 00 00 00 ff ff 00 10 00 00 00 00`) are byte-for-byte identical
in *every* release of *both* processors across 6 years. This extends last
session's single-version Main-vs-Panel match into real structural
evidence, not coincidence.

**Header bytes 0-1, resolved with a mechanism instead of another failed
checksum guess.** Decoding bytes 0-7 as MIPS32 across all 6 Main versions:
every single one is `jal <target>; nop` (valid JAL opcode bits, second
word a true all-zero `nop`, in all six). **Bytes 0-1 are literally the low
16 bits of a real JAL instruction's target address - not a checksum or
version tag at all.** This explains, rather than just adds to, the earlier
checksum-algorithm dead end (§5.5-adjacent): there was never a checksum
there to find.

**The big result: the same jump-table dispatch function exists in every
release, always fails the same validity test.**

| Version | file offset | computed table addr (base `0x9D000000`) | cases | valid |
|---|---|---|---|---|
| 1.1.3   | `0x2e0` | `0x9D05E3B4` | 45 | **0/45** |
| 1.2.0.8 | `0x330` | `0x9D065AC8` | 45 | **0/45** |
| 1.3.0.6 | `0x334` | `0x9D068810` | 40 | **0/40** |
| 1.3.0   | `0x334` | `0x9D06A378` | 45 | **0/45** |
| 1.4     | `0x32c` | `0x9D0695F0` | 40 | **0/40** |
| 1.5.0.2 | `0x3dc` | `0x9D06B860` | 40 | **0/40** (already documented, §5.6) |

Found via the same register-fingerprint idiom scanner already used for
1.5.0.2 (`sll $t1,$a3,2` ... `lw $a0,($a3)` / `jr $a0`), identified by its
distinctive register allocation in all 6 binaries. The table's file offset
naturally shifts release to release as the code around it changes, and
the computed address tracks that shift consistently - i.e. whatever the
true base is, file-offset-0 has mapped to the *same* virtual base
consistently within each build's own internal logic, this just doesn't
confirm what that base actually is. **Every version, independently, fails
identically: 0 valid entries.** This is meaningfully stronger than before:
it rules out "maybe 1.5.0.2 specifically has some anomaly" - the same
test, against 6 independently-built binaries spanning 6 years, fails the
same way every time. Much harder to explain as a fluke of one build, much
easier to explain as "the flat file-offset-0-equals-`0x9D000000` model is
the wrong model" - the standing §5.6 conclusion, now resting on 6 data
points instead of 1.

**A separate structural finding in Voice's header** (different chip,
different header shape entirely - `f0 15 04 00 00 00 ff ff ff...`, not
Main/Panel's `40 0f...`): bytes 9-20 decode as four sequential 24-bit
little-endian values, consecutive pairs always exactly 16 or 14 bytes
apart, all four values well within Voice's own 65,577-byte decoded size -
reads like a real 4-entry internal offset table, shifting slightly release
to release the same way Main's jump table does. Not independently
actionable (Voice's chip remains undocumented custom silicon), but a
clean, structured, unexplained feature worth keeping on record.

**Decoded firmware size is a fixed architectural constant, not something
that grew with features.** Every Main version across 6 years decodes to
exactly 524,609 bytes; Panel/Voice to exactly 65,577; Sam to 32,789 -
no exceptions across 14 total decoded files.

**Where this leaves things**: the base address itself is not newly
resolved - nothing here validates or invalidates `0x9D000000` differently
than before. What changed is confidence in the negative result itself:
replicated independently 6 times rather than resting on one build. The
ICSP hardware test (§7 item 5) is unchanged as the standing
recommendation, more strongly so now that the computational angle has
been tested as thoroughly as 6 independent binaries allow.
