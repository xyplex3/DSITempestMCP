// Package mapper implements the capture-diff-annotate workflow for
// reverse-engineering the DSI Tempest project dump byte layout.
package mapper

import (
	"fmt"
	"os"

	"tempest-mcp/internal/sysex"
)

// Result holds the output of unescaping one SysEx file.
type Result struct {
	Payload []byte            // unescaped payload bytes
	MsgType sysex.MessageType // classified message type
	RawSize int               // length of the escaped (wire) payload
}

// UnescapeProject reads path, finds the first 0x61 project dump (or the first
// recognised Tempest message if no project dump exists), and returns the
// unescaped payload.
func UnescapeProject(path string) (*Result, error) {
	msgs, err := ParseSyx(path)
	if err != nil {
		return nil, err
	}

	var fallback *Result
	for _, msg := range msgs {
		t := sysex.Identify(msg)
		if t == sysex.TypeUnknown {
			continue
		}
		r := &Result{
			Payload: sysex.Unescape(msg),
			MsgType: t,
			RawSize: len(sysex.Payload(msg)),
		}
		if t == sysex.TypeProjectDump {
			return r, nil
		}
		if fallback == nil {
			fallback = r
		}
	}
	if fallback != nil {
		return fallback, nil
	}
	return nil, fmt.Errorf("no recognised Tempest SysEx message in %s", path)
}

// ParseSyx reads a .syx file and splits it into individual SysEx messages
// (F0…F7).
func ParseSyx(path string) ([][]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return sysex.SplitMessages(data), nil
}
