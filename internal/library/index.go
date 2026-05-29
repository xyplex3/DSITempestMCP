// Package library manages the local Tempest sound library at /Users/xyplex2/Tempest.
// It scans .syx files, extracts sound names and metadata, and maintains a JSON index.
package library

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tempest-mcp/internal/sysex"
)

// BankAssignment records which hardware bank slot a sound has been loaded into.
type BankAssignment struct {
	Bank     string    `json:"bank"`      // "A" or "B"
	Slot     int       `json:"slot"`      // 1–16
	LoadedAt time.Time `json:"loaded_at"` // time the sound was last sent to this slot
}

// Sound represents one entry in the library index.
type Sound struct {
	ID        string          `json:"id"`                  // MD5 fingerprint of the parameter block
	Name      string          `json:"name"`                // extracted from SysEx name field
	Path      string          `json:"path"`                // absolute path to .syx file
	SizeBytes int64           `json:"size_bytes"`          // file size in bytes
	Folder    string          `json:"folder"`              // parent directory name
	Tags      []string        `json:"tags"`                // derived from folder hierarchy
	MsgType   string          `json:"msg_type"`            // "FLASH" | "RAM" | "Project" | "Alternate" | "Unknown"
	IndexedAt time.Time       `json:"indexed_at"`          // time this entry was added to the index
	BankSlot  *BankAssignment `json:"bank_slot,omitempty"` // most recent hardware slot assignment
}

// Index holds all scanned sounds.
type Index struct {
	Sounds    []*Sound  `json:"sounds"`     // all sounds found during the last scan
	ScannedAt time.Time `json:"scanned_at"` // time the scan completed
	RootPath  string    `json:"root_path"`  // root directory that was scanned
}

// Scan walks rootPath, parses every .syx file, and returns a populated Index.
func Scan(rootPath string) (*Index, error) {
	idx := &Index{
		ScannedAt: time.Now(),
		RootPath:  rootPath,
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if info.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".syx") {
			return nil
		}

		sounds, parseErr := parseSyxFile(path, rootPath, info.Size())
		if parseErr != nil {
			// log but don't abort
			fmt.Fprintf(os.Stderr, "[library] skipping %s: %v\n", path, parseErr)
			return nil
		}
		idx.Sounds = append(idx.Sounds, sounds...)
		return nil
	})
	return idx, err
}

// parseSyxFile reads a .syx file and returns one or more Sound entries.
// A single file may contain multiple SysEx messages separated by F7.
func parseSyxFile(path, rootPath string, fileSize int64) ([]*Sound, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	messages := sysex.SplitMessages(data)
	if len(messages) == 0 {
		return nil, fmt.Errorf("no valid SysEx messages")
	}

	folder := filepath.Base(filepath.Dir(path))
	tags := deriveTags(path, rootPath)

	var sounds []*Sound
	for _, msg := range messages {
		t := sysex.Identify(msg)
		if t == sysex.TypeUnknown {
			continue
		}

		unescaped := sysex.Unescape(msg)
		name, _ := sysex.ExtractName(unescaped, t)
		if name == "" {
			// RAM sound or nameless — use filename
			base := filepath.Base(path)
			name = strings.TrimSuffix(base, filepath.Ext(base))
		}
		// Strip factory /S/ prefix for display
		displayName := strings.TrimPrefix(name, "/S/")
		displayName = strings.TrimPrefix(displayName, "/P/")

		fp, _ := sysex.Fingerprint(msg)
		if fp == "" {
			fp = fmt.Sprintf("file-%s-%d", filepath.Base(path), fileSize)
		}

		sounds = append(sounds, &Sound{
			ID:        fp,
			Name:      displayName,
			Path:      path,
			SizeBytes: fileSize,
			Folder:    folder,
			Tags:      tags,
			MsgType:   msgTypeName(t),
			IndexedAt: time.Now(),
		})
	}
	return sounds, nil
}

// deriveTags builds a tag slice from the path relative to rootPath.
// E.g. rootPath/Kicks/Analog/kick808.syx → ["Kicks", "Analog"]
func deriveTags(path, rootPath string) []string {
	rel, err := filepath.Rel(rootPath, filepath.Dir(path))
	if err != nil {
		return nil
	}
	parts := strings.Split(rel, string(filepath.Separator))
	var tags []string
	for _, p := range parts {
		if p != "" && p != "." {
			tags = append(tags, p)
		}
	}
	return tags
}

func msgTypeName(t sysex.MessageType) string {
	switch t {
	case sysex.TypeFLASHSound:
		return "FLASH"
	case sysex.TypeRAMSound:
		return "RAM"
	case sysex.TypeProjectDump:
		return "Project"
	case sysex.TypeAlternateSound:
		return "Alternate"
	case sysex.TypeAlternateBank:
		return "AltBank"
	}
	return "Unknown"
}

// SaveIndex writes the index to path as JSON.
func SaveIndex(idx *Index, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadIndex reads a previously saved JSON index.
func LoadIndex(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var idx Index
	return &idx, json.Unmarshal(data, &idx)
}

// SoundByID finds a sound by fingerprint ID.
func (idx *Index) SoundByID(id string) *Sound {
	for _, s := range idx.Sounds {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// ReadSyxMessages reads the raw SysEx messages from a sound's file path.
func ReadSyxMessages(path string) ([][]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	msgs := sysex.SplitMessages(data)
	if len(msgs) == 0 {
		return nil, fmt.Errorf("no SysEx messages in %s", path)
	}
	return msgs, nil
}
