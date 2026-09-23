# DSI Tempest Multi-Note Export Guide

## Status

The SysEx container format can hold multiple note records - three real
captures show two different tracks as clean, correctly-formed consecutive
10-byte records. But no controlled test has yet isolated what actually
causes some multi-note exports to lose a note: the obvious "same/adjacent
step" theory doesn't hold up, since one of the working captures has the
same step-delta as a failing one. None of the existing multi-note captures
were taken under today's verified-clean methodology, so they may not even
be controlled tests. Full research history: `docs/sysex-tempest-format.md`
§9.4-§9.7.

The `0x61` Project format is now fully decoded (§9.10) - `tempest_analyze_project`
and `tempest_decode_project_beats` can read a project's 16 beats
mechanically - but it stores each beat's notes in the same per-beat record
format as a standalone Beat export, so it doesn't sidestep the multi-note
question above.

## Until the multi-note case is resolved

Individual-note export is the only method with a proven track record:

1. Export each note as its own single-note Beat (mute all other tracks,
   **Save/Load → Export Beat in RAM over MIDI**).
2. Verify each capture's note count from file size before trusting it -
   `cmd/capture-tmp` does this automatically (`5925 + 8×N` raw bytes for
   N notes), or use `tempest_export_wizard`.
3. Combine the individual captures downstream (DAW, or your own tooling) if
   you need them together.

## If you want to help resolve the multi-note question

See `docs/sysex-tempest-format.md`, "Suggested next steps" item 1, for the
specific controlled test that's still needed: two notes on the *same* step,
and separately the *same* track at two different steps, captured under a
fresh `Initialize Beat` and verified with `capture-tmp` before trusting the
result. `tempest_analyze_beat` (the CLI in `cmd/tempest-analyze-beat`) can
help characterize a capture's note records once you have one.
