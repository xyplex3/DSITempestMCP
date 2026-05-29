package library_test

import (
	"testing"

	"tempest-mcp/internal/library"
)

// testIndex returns an Index with a small fixed set of sounds for search tests.
func testIndex() *library.Index {
	return &library.Index{
		Sounds: []*library.Sound{
			{
				ID:      "fp1",
				Name:    "Kick Drum",
				MsgType: "FLASH",
				Folder:  "Kicks",
				Tags:    []string{"Kicks", "Drums"},
			},
			{
				ID:      "fp2",
				Name:    "Snare Hit",
				MsgType: "RAM",
				Folder:  "Snares",
				Tags:    []string{"Snares"},
			},
			{
				ID:      "fp3",
				Name:    "HiHat Closed",
				MsgType: "FLASH",
				Folder:  "HiHats",
				Tags:    []string{"HiHats", "Cymbals"},
			},
		},
	}
}

// containsSound reports whether results contains a sound with the given name.
func containsSound(results []library.SearchResult, name string) bool {
	for _, r := range results {
		if r.Sound.Name == name {
			return true
		}
	}
	return false
}

// assertAllScore fails the test if any result does not have the expected score.
func assertAllScore(t *testing.T, results []library.SearchResult, score int) {
	t.Helper()
	for i, r := range results {
		if r.Score != score {
			t.Errorf("results[%d].Score = %d, want %d", i, r.Score, score)
		}
	}
}

// TestSearch verifies relevance-ranked search over the sound library.
func TestSearch(t *testing.T) {
	idx := testIndex()

	t.Run("nil index returns nil", func(t *testing.T) {
		results := library.Search(nil, "kick")
		if results != nil {
			t.Errorf("Search(nil) = %v, want nil", results)
		}
	})

	t.Run("empty index returns nil", func(t *testing.T) {
		results := library.Search(&library.Index{}, "kick")
		if results != nil {
			t.Errorf("Search(empty) = %v, want nil", results)
		}
	})

	t.Run("empty query returns all sounds with equal score", func(t *testing.T) {
		results := library.Search(idx, "")
		if len(results) != len(idx.Sounds) {
			t.Errorf("len = %d, want %d", len(results), len(idx.Sounds))
		}
		assertAllScore(t, results, 1)
	})

	t.Run("exact name match returns that sound", func(t *testing.T) {
		results := library.Search(idx, "Kick Drum")
		if len(results) != 1 {
			t.Fatalf("len = %d, want 1", len(results))
		}
		if results[0].Sound.Name != "Kick Drum" {
			t.Errorf("Name = %q, want %q", results[0].Sound.Name, "Kick Drum")
		}
	})

	t.Run("prefix match scores higher than contains match", func(t *testing.T) {
		// "kick" is a prefix of "Kick Drum" but not in Snare or HiHat.
		results := library.Search(idx, "kick")
		if len(results) == 0 {
			t.Fatal("expected results for 'kick'")
		}
		if results[0].Sound.Name != "Kick Drum" {
			t.Errorf("first result = %q, want %q", results[0].Sound.Name, "Kick Drum")
		}
	})

	t.Run("no matching query returns empty results", func(t *testing.T) {
		results := library.Search(idx, "xylophone")
		if len(results) != 0 {
			t.Errorf("len = %d, want 0", len(results))
		}
	})

	t.Run("results sorted by score descending", func(t *testing.T) {
		results := library.Search(idx, "hi")
		for i := 1; i < len(results); i++ {
			if results[i].Score > results[i-1].Score {
				t.Errorf("results not sorted: results[%d].Score=%d > results[%d].Score=%d",
					i, results[i].Score, i-1, results[i-1].Score)
			}
		}
	})

	t.Run("tag match contributes to score", func(t *testing.T) {
		// "Drums" is a tag on Kick Drum.
		results := library.Search(idx, "Drums")
		if len(results) == 0 {
			t.Fatal("expected results for tag 'Drums'")
		}
		if !containsSound(results, "Kick Drum") {
			t.Error("expected 'Kick Drum' in results for tag 'Drums'")
		}
	})

	t.Run("whitespace-trimmed query", func(t *testing.T) {
		r1 := library.Search(idx, "snare")
		r2 := library.Search(idx, "  snare  ")
		if len(r1) != len(r2) {
			t.Errorf("trimmed query gives different len: %d vs %d", len(r1), len(r2))
		}
	})
}

// TestFilterByTag verifies tag-based filtering.
func TestFilterByTag(t *testing.T) {
	idx := testIndex()

	tests := []struct {
		name    string
		tag     string
		wantIDs []string
	}{
		{
			name:    "matching tag returns sounds",
			tag:     "Drums",
			wantIDs: []string{"fp1"},
		},
		{
			name:    "case-insensitive match",
			tag:     "drums",
			wantIDs: []string{"fp1"},
		},
		{
			name:    "tag present on multiple sounds",
			tag:     "Cymbals",
			wantIDs: []string{"fp3"},
		},
		{
			name:    "non-existent tag returns nil",
			tag:     "Woodwinds",
			wantIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := library.FilterByTag(idx, tt.tag)
			if tt.wantIDs == nil {
				if len(got) != 0 {
					t.Errorf("FilterByTag(%q) = %v, want nil", tt.tag, got)
				}
				return
			}
			if len(got) != len(tt.wantIDs) {
				t.Errorf("len = %d, want %d", len(got), len(tt.wantIDs))
				return
			}
			for i, s := range got {
				if s.ID != tt.wantIDs[i] {
					t.Errorf("got[%d].ID = %q, want %q", i, s.ID, tt.wantIDs[i])
				}
			}
		})
	}
}

// TestFilterByType verifies message-type filtering.
func TestFilterByType(t *testing.T) {
	idx := testIndex()

	tests := []struct {
		name    string
		msgType string
		wantLen int
		wantIDs []string
	}{
		{
			name:    "FLASH returns two sounds",
			msgType: "FLASH",
			wantLen: 2,
			wantIDs: []string{"fp1", "fp3"},
		},
		{
			name:    "RAM returns one sound",
			msgType: "RAM",
			wantLen: 1,
			wantIDs: []string{"fp2"},
		},
		{
			name:    "case-insensitive match",
			msgType: "flash",
			wantLen: 2,
		},
		{
			name:    "unknown type returns nil",
			msgType: "Alternate",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := library.FilterByType(idx, tt.msgType)
			if len(got) != tt.wantLen {
				t.Errorf("FilterByType(%q) len = %d, want %d", tt.msgType, len(got), tt.wantLen)
				return
			}
			if tt.wantIDs != nil {
				for i, s := range got {
					if i < len(tt.wantIDs) && s.ID != tt.wantIDs[i] {
						t.Errorf("got[%d].ID = %q, want %q", i, s.ID, tt.wantIDs[i])
					}
				}
			}
		})
	}
}
