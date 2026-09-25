# DSI Tempest Multi-Note Export Guide

## Status

Up to two simultaneous notes are confirmed to export correctly against
real hardware. Two independent, controlled captures - including a reversed
track/step assignment - both exported and decoded byte-exact. The
procedure that works: fresh `Initialize Beat`, explicit Beat-1 selection
in **16 Beats mode** before editing (skipping this reproduced a different,
worse failure than a lost note), add both notes, confirm both visible,
then **Save/Load → Export Beat over MIDI**, confirming the **Source Beat**
screen before continuing. Full write-up, both capture results, and the
saved fixture files: `docs/sysex-tempest-format.md` §9.13.

Three or more simultaneous notes in one beat are untested.

The `0x61` Project format is fully decoded (§9.10) - `tempest_analyze_project`
and `tempest_decode_project_beats` can read a project's 16 beats
mechanically - but it stores each beat's notes in the same per-beat record
format as a standalone Beat export, so the same two-note confirmation and
three-or-more caveat both apply there too.

## Exporting more than two notes

Until the three-or-more case is resolved, individual-note export remains
the reliable fallback beyond two notes:

1. Export each extra note as its own single-note Beat (mute other tracks,
   **Save/Load → Export Beat over MIDI**).
2. Verify each capture's note count from file size before trusting it -
   `cmd/capture-tmp` does this automatically (`5925 + 8×N` raw bytes for
   N notes), or use `tempest_export_wizard`.
3. Combine the individual captures downstream (DAW, or your own tooling) if
   you need them together.

## If you want to help resolve the three-or-more-note question

See `docs/sysex-tempest-format.md`, "Suggested next steps" item 4, for
what's still untested: whether the confirmed two-note layout still holds
with three or more simultaneous notes, at a higher track index, or across
the A/B bank boundary. `tempest_analyze_beat` (the CLI in
`cmd/tempest-analyze-beat`) can help characterize a capture's note records
once you have one.
