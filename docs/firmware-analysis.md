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
Facebook post "Analog drum synthesizer Tempest by Dave Smith Instruments")
show the board silkscreened **`DSI-800R Rev 1.2`, `(C) 2010 Dave Smith
Instruments`**, hosting:

- A chip clearly marked **`dream SAM3716`** - the SAM processor (§2).
- A chip custom-labeled **`TEMPEST DSP 1.0`** - almost certainly the Voice
  processor (§2), real identity hidden by the re-badge.
- Six repeated identical small-IC clusters, consistent with the manual's
  "six powerful analog synthesis voices."
- No chip in these specific photos was legibly marked as a PIC32 variant.
  Extensive zooming into every visible IC on this particular board didn't
  turn up the Main/Panel controller's exact part number - it's most likely
  on a different, unphotographed board (a separate control/panel board), or
  simply wasn't visible at this photo's resolution/angle.

**Searching for the exact PIC32 part number came up empty, and this looks
like a real dead end for the search-based approach specifically:**

- `fccid.io` and the FCC's own legacy equipment-authorization search are
  both impractical to query this way (bot walls on the former; the latter's
  legacy form needs a grantee code as a starting point, which isn't known).
- No schematic, service document, or FCC filing is indexed anywhere under
  the exact board revision `"DSI-800R"`.
- Direct `"PIC32"` + Tempest photo/text searches turn up essentially
  nothing relevant.
- The 2010 board date does at least confirm the chip must be a **PIC32MX**
  family part, not PIC32MZ (which wasn't released until ~2013) - narrows
  which vector-spacing convention applies, but doesn't give the specific
  variant.

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

**What this consistent pattern of failure suggests:** not that any single
attempt was almost right, but that blind statistical correlation over a raw
firmware blob doesn't have enough structure to solve this without a real
linker map, debug symbols, or confirmed code/data segment boundaries. Further
variations on the same class of technique are unlikely to succeed where four
already haven't - this is now a considered conclusion, not something to
re-attempt without new input.

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
   `Tempest_Main_*.syx` file.

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

1. **Check whether the Internet Archive is back up**, and if so, search for
   an older `Tempest_Main_*.syx` release to diff against `1.5.0.2`'s header
   (§5, item 3) - the cheapest remaining idea that hasn't actually been
   executed yet, just blocked by an external outage.
2. **Look specifically for a legible photo of Panel's board** - smaller and
   likely less crowded than the analog voice board already found, and per
   §3, whatever chip it uses answers the question for Main too.
3. **Ask directly in the Tempest hacking community** (the long-running
   Gearspace thread, or similar forums) whether anyone has already
   identified the exact PIC32 part or has schematics/service documentation.
4. Only after one of the above provides real new information, revisit
   cross-referencing the firmware's `"Failed to read sequence data"` and
   related strings back to their calling code - that was always the actual
   goal, not base-address-hunting for its own sake.
