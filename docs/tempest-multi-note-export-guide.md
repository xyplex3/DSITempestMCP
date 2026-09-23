# DSI Tempest Multi-Note Export Guide

## Current Understanding of Multi-Note Export Behavior

Recent analysis reveals important technical capabilities and ongoing research about multi-note exports:

### What We Know Technically

- The SysEx file format **CAN** technically contain multiple valid note records
- Some capture files show 2+ complete, well-formed 80-bit note records
- The file structure supports this capability when data is present

### What We Don't Know Yet

- **Reliable techniques**: No proven method consistently produces multi-note exports
- **Root cause**: Exact conditions determining success vs failure are unclear
- **Controlled verification**: Need deliberately tested cases with clean baselines

### Current Research Status

Active investigation is underway to understand:

1. When multi-note data appears in exports vs when it's missing
2. Whether specific techniques can reliably produce multi-note exports
3. What distinguishes successful from unsuccessful export attempts

### Current Best Practices

Until research is complete:

- Single-note exports are reliable and well-tested
- Multi-note export behavior varies unpredictably
- Users should not assume any specific technique will work consistently

## Current Working Approaches

### 1. Individual Note Export Method (Verified Reliable)

**Process:**

1. Create your full multi-note beat in the Tempest
2. Export each note individually by:
   - Muting all other notes temporarily
   - Exporting each note as a separate beat
   - Combining the notes client-side in your DAW/software

**Pros:**

- Most reliable method with proven track record
- Gives you complete control over each note's parameters
- Works consistently across all Tempest models and firmware versions

**Cons:**

- Requires manual coordination
- More time-consuming than single multi-note export

## Recommended Workflow

### For Users Needing Multi-Note Patterns

1. **Plan your beat structure** in advance to minimize note muting
2. **Export each note separately** following this sequence:
   - Mute all notes except the first one
   - Export Beat in RAM over MIDI
   - Restore the first note, mute all others
   - Repeat for each note in your pattern
3. **Import into your DAW** and align the notes manually
4. **Document your note assignments** to recreate the pattern later

### Example Multi-Note Export Process

```
Beat: Kick pattern with snare fills
- Kick: A1 steps 1,5,9,13
- Snare: A2 steps 3,7,11,15

Export Process:
1. Mute snare track, export kick-only beat
2. Mute kick track, unmute snare track, export snare-only beat
3. Import both into DAW on separate tracks
4. Align timing manually in DAW
```

## Alternative Workaround: Project Export Method

### Using Project Exports Instead of Beat Exports

Recent analysis reveals that **Export Project** may provide access to complete multi-note data:

1. **Project Dump (0x61)**: Single large message (~95KB) containing complete project
2. **Project Stream (0x5C/0x5E)**: 17-message sequence with individual beats

### Process

1. Create your full multi-note beat in the Tempest
2. Use "Export Project in RAM over MIDI" instead of "Export Beat in RAM over MIDI"
3. Receive 17 messages: 1 header (0x5E) + 16 individual beats (0x5C)
4. Each 0x5C message may contain complete note information for that beat

### Advantages

- May provide complete multi-note data that Beat exports lack
- Single operation vs. multiple individual note exports
- Potentially preserves all timing and parameter information

### Testing Needed

Further analysis required to determine if 0x5C messages actually contain complete multi-note data or still suffer from the same limitation.

## Future Possibilities

### Firmware Analysis Path

The reverse engineering team is investigating:

- Whether the 0x61 Project format contains complete multi-note data
- If newer firmware versions address this limitation
- The exact code that generates the "Failed to read sequence data" error

### Potential Improvements

Future software could:

- Automatically detect and warn about multi-note export limitations
- Provide guided workflows for the workaround methods
- Parse 0x61 Project messages and 0x5C/0x5E streams if they contain complete data
- Offer note alignment assistance tools

## Troubleshooting Tips

### If You Still Get "Failed to Read Sequence Data" Errors

1. **Verify single-note export works first** - Test with a beat containing only one note
2. **Check your export procedure** - Ensure you're using "Export Beat in RAM over MIDI" correctly
3. **Try the save-to-flash workaround** - Different export paths may behave differently
4. **Contact DSI support** - This is a known issue that may be addressed in future updates

### Checking Export Success

You can verify successful exports by checking file sizes:

- **Baseline (0 notes)**: ~5925 bytes
- **One note**: ~5933 bytes (5925 + 8)
- **Multiple notes**: May show expected size or reduced size due to export behavior

### Current Research Opportunities

We're actively investigating:

- Controlled tests of same-step vs different-step multi-note placement
- Verification of export behavior with clean baselines and verified note counts
- Alternative export methods that might preserve complete multi-note data

## Current Research Status: Active Investigation

The Tempest community is actively investigating multi-note export behavior:

### What We're Learning

- Some files DO contain multiple valid note records technically
- The file format supports this capability when data is present
- Exact conditions for reliable multi-note export remain under study

### Current Best Practice

Until research is complete, the most reliable approach is individual note export:

1. **Export each note separately** - Proven reliable method
2. **Combine in your DAW** - Full control over timing and parameters
3. **Use diagnostic tools** - Verify what data is actually present

### Join the Research

We're conducting controlled tests to better understand:

- When multi-note data appears in exports
- Whether specific techniques can be reliably used
- What distinguishes successful from unsuccessful attempts

Stay tuned for updates as research progresses!

For now, individual note export remains the gold standard for reliability.
