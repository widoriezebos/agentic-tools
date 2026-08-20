package goal

// The migration manifest (BGS-4, R5-09): a CLOSED, deterministic
// schema — `### add-goal: <id>` and `### amend-goal: <id>` headings
// with `- key: value` lines. Omitting blockedBy means "no
// amendment"; the explicit clearing form is `blockedBy: -`, the one
// place `-` is lawful. A parked amendment without parked-because
// refuses (R10-M08). Unknown keys refuse — the schema is closed.

import (
	"fmt"
	"strings"
)

// ManifestEntry is one add or amend.
type ManifestEntry struct {
	Kind string // add-goal | amend-goal
	Id   string

	Intent  string
	Origin  string
	Next    string
	HasNext bool

	BlockedBy    []string
	ClearBlocked bool // blockedBy: - (distinct from omission)
	HasBlocked   bool

	ParkedBy      string
	ParkedAt      string
	ParkedBecause string
}

// manifestKeys is the closed key set, lowercased.
var manifestKeys = map[string]bool{
	"intent": true, "origin": true, "next": true, "blockedby": true,
	"parked-by": true, "parked-at": true, "parked-because": true,
}

// ParseManifest reads the amendment entries out of the manifest
// document; prose outside entries is commentary and ignored.
func ParseManifest(data []byte) ([]ManifestEntry, error) {
	var entries []ManifestEntry
	var current *ManifestEntry
	flush := func() error {
		if current == nil {
			return nil
		}
		if current.Kind == "add-goal" && current.Intent == "" {
			return fmt.Errorf("manifest %s %s: an added goal needs its Intent", current.Kind, current.Id)
		}
		if current.ParkedBy != "" || current.ParkedAt != "" {
			if current.ParkedBecause == "" {
				return fmt.Errorf("manifest %s %s: a parked amendment without parked-because refuses (R10-M08)", current.Kind, current.Id)
			}
		}
		entries = append(entries, *current)
		current = nil
		return nil
	}
	for lineNo, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimRight(raw, " \t")
		if strings.HasPrefix(line, "### ") {
			if err := flush(); err != nil {
				return nil, err
			}
			heading := strings.TrimPrefix(line, "### ")
			for _, kind := range []string{"add-goal", "amend-goal"} {
				if strings.HasPrefix(heading, kind+": ") {
					current = &ManifestEntry{Kind: kind, Id: strings.TrimSpace(strings.TrimPrefix(heading, kind+": "))}
				}
			}
			continue
		}
		if current == nil || !strings.HasPrefix(line, "- ") {
			continue
		}
		body := strings.TrimPrefix(line, "- ")
		key, value, found := strings.Cut(body, ":")
		if !found {
			return nil, fmt.Errorf("manifest line %d: no key in %q", lineNo+1, line)
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if !manifestKeys[key] {
			return nil, fmt.Errorf("manifest line %d: unknown key %q — the schema is closed", lineNo+1, key)
		}
		switch key {
		case "intent":
			current.Intent = value
		case "origin":
			current.Origin = value
		case "next":
			current.Next = value
			current.HasNext = true
		case "blockedby":
			current.HasBlocked = true
			if value == "-" {
				current.ClearBlocked = true
			} else {
				for _, dep := range strings.Split(value, ",") {
					if d := strings.TrimSpace(dep); d != "" {
						current.BlockedBy = append(current.BlockedBy, d)
					}
				}
			}
		case "parked-by":
			current.ParkedBy = value
		case "parked-at":
			current.ParkedAt = value
		case "parked-because":
			current.ParkedBecause = value
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return entries, nil
}
