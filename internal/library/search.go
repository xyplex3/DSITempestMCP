package library

import (
	"sort"
	"strings"
)

// SearchResult wraps a Sound with a relevance score.
type SearchResult struct {
	Sound *Sound // matched sound from the index
	Score int    // relevance score; higher = more relevant
}

// Search performs a case-insensitive fuzzy search over sound names and tags.
// Returns results sorted by relevance descending.
func Search(idx *Index, query string) []SearchResult {
	if idx == nil || len(idx.Sounds) == 0 {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		// Return all sounds with equal score
		results := make([]SearchResult, len(idx.Sounds))
		for i, s := range idx.Sounds {
			results[i] = SearchResult{Sound: s, Score: 1}
		}
		return results
	}

	var results []SearchResult
	for _, s := range idx.Sounds {
		score := scoreSound(s, q)
		if score > 0 {
			results = append(results, SearchResult{Sound: s, Score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	return results
}

// scoreSound returns a relevance score for sound s against lowercase query q.
func scoreSound(s *Sound, q string) int {
	name := strings.ToLower(s.Name)
	folder := strings.ToLower(s.Folder)

	score := 0

	// Exact name match
	switch {
	case name == q:
		score += 100
	case strings.HasPrefix(name, q):
		score += 60
	case strings.Contains(name, q):
		score += 40
	}

	// Folder / tag match
	if strings.Contains(folder, q) {
		score += 20
	}
	for _, tag := range s.Tags {
		if strings.EqualFold(tag, q) {
			score += 30
		} else if strings.Contains(strings.ToLower(tag), q) {
			score += 10
		}
	}

	// Message type match (e.g. query "flash" or "ram")
	if strings.Contains(strings.ToLower(s.MsgType), q) {
		score += 5
	}

	return score
}

// FilterByTag returns all sounds that have the given tag (case-insensitive).
func FilterByTag(idx *Index, tag string) []*Sound {
	tag = strings.ToLower(tag)
	var out []*Sound
	for _, s := range idx.Sounds {
		for _, t := range s.Tags {
			if strings.ToLower(t) == tag {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// FilterByType returns sounds matching a message type string (e.g. "FLASH").
func FilterByType(idx *Index, msgType string) []*Sound {
	msgType = strings.ToLower(msgType)
	var out []*Sound
	for _, s := range idx.Sounds {
		if strings.ToLower(s.MsgType) == msgType {
			out = append(out, s)
		}
	}
	return out
}
