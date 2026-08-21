package goal

// The migration manifest (BGS-4, R5-09): a CLOSED, deterministic
// schema, parsed exactly as the checked-in document declares
// itself. MIGRATION_EPOCH and REVIEWED_SOURCE_SHA256 are REQUIRED
// headers, each exactly once before the first entry. Entries are
// `### add-goal: <id>` / `### amend-goal: <id>` with `- key: value`
// lines; values are single paragraphs, LF-joined by continuation
// lines (a literal `- ` at a continuation start refuses). add-goal
// requires Intent/Origin/Next and admits BlockedBy/Arc; amend-goal
// admits state (queued|parked only), parked-by/parked-at (required
// together whenever state: parked; parked-at may be the literal
// EPOCH), parked-because, next, blockedBy (full replacement; `-`
// clears), and arc. Unknown keys, duplicate keys, duplicate entry
// ids, and a parked state without its because all refuse. OpenedAt
// for add-goals = EPOCH + (1000 + position) minutes, one-based in
// textual order.

import (
	"fmt"
	"strings"
	"time"
)

// Manifest is the parsed document: its two binding headers and the
// entries in textual order.
type Manifest struct {
	Epoch          string // MIGRATION_EPOCH, RFC3339
	ReviewedSHA256 string // REVIEWED_SOURCE_SHA256
	Entries        []ManifestEntry
}

// ManifestEntry is one add or amend.
type ManifestEntry struct {
	Kind     string // add-goal | amend-goal
	Id       string
	Position int // one-based among add-goal entries, textual order

	Intent  string
	Origin  string
	Next    string
	HasNext bool

	State  string // amend only: queued | parked | ""
	Arc    string
	HasArc bool

	BlockedBy    []string
	ClearBlocked bool
	HasBlocked   bool

	ParkedBy      string
	ParkedAt      string
	ParkedBecause string
}

var addKeys = map[string]bool{
	"intent": true, "origin": true, "next": true, "blockedby": true, "arc": true,
}
var amendKeys = map[string]bool{
	"state": true, "parked-by": true, "parked-at": true, "parked-because": true,
	"next": true, "blockedby": true, "arc": true,
}

// ParseManifest reads the whole document; prose outside entries is
// commentary, but the two header fields bind.
func ParseManifest(data []byte) (*Manifest, error) {
	m := &Manifest{}
	var current *ManifestEntry
	var lastKey string
	seenIds := map[string]bool{}
	seenKeys := map[string]bool{}
	addPosition := 0
	epochCount, shaCount := 0, 0
	inEntry := false

	setValue := func(key, value string) error {
		allowed := addKeys
		if current.Kind == "amend-goal" {
			allowed = amendKeys
		}
		if !allowed[key] {
			return fmt.Errorf("manifest %s %s: unknown key %q — the schema is closed", current.Kind, current.Id, key)
		}
		if seenKeys[key] {
			return fmt.Errorf("manifest %s %s: duplicate key %q", current.Kind, current.Id, key)
		}
		seenKeys[key] = true
		switch key {
		case "intent":
			current.Intent = value
		case "origin":
			current.Origin = value
		case "next":
			current.Next = value
			current.HasNext = true
		case "state":
			if value != StateQueued && value != StateParked {
				return fmt.Errorf("manifest %s %s: state %q is not manifest-assignable (queued or parked only)", current.Kind, current.Id, value)
			}
			current.State = value
		case "arc":
			current.Arc = value
			current.HasArc = true
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
		return nil
	}

	flush := func() error {
		if current == nil {
			return nil
		}
		if seenIds[current.Id] {
			return fmt.Errorf("manifest: duplicate entry id %s", current.Id)
		}
		seenIds[current.Id] = true
		if current.Kind == "add-goal" {
			if current.Intent == "" || current.Origin == "" || !current.HasNext {
				return fmt.Errorf("manifest add-goal %s: Intent, Origin, and Next are required", current.Id)
			}
		} else {
			if !current.HasNext && !current.HasBlocked && !current.HasArc &&
				current.State == "" && current.ParkedBecause == "" && current.ParkedBy == "" {
				return fmt.Errorf("manifest amend-goal %s: at least one key is required", current.Id)
			}
		}
		if current.State == StateParked {
			if current.ParkedBy == "" || current.ParkedAt == "" {
				return fmt.Errorf("manifest %s %s: state parked requires parked-by and parked-at together", current.Kind, current.Id)
			}
			if current.ParkedBecause == "" {
				return fmt.Errorf("manifest %s %s: a park always carries its reason (parked-because)", current.Kind, current.Id)
			}
		}
		m.Entries = append(m.Entries, *current)
		current = nil
		return nil
	}

	for lineNo, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimRight(raw, " \t")
		if strings.HasPrefix(line, "MIGRATION_EPOCH:") {
			if len(m.Entries) > 0 || current != nil {
				return nil, fmt.Errorf("manifest line %d: MIGRATION_EPOCH must precede the first entry", lineNo+1)
			}
			m.Epoch = strings.TrimSpace(strings.TrimPrefix(line, "MIGRATION_EPOCH:"))
			epochCount++
			continue
		}
		if strings.HasPrefix(line, "REVIEWED_SOURCE_SHA256:") {
			if len(m.Entries) > 0 || current != nil {
				return nil, fmt.Errorf("manifest line %d: REVIEWED_SOURCE_SHA256 must precede the first entry", lineNo+1)
			}
			m.ReviewedSHA256 = strings.TrimSpace(strings.TrimPrefix(line, "REVIEWED_SOURCE_SHA256:"))
			shaCount++
			continue
		}
		if strings.HasPrefix(line, "### ") {
			if err := flush(); err != nil {
				return nil, err
			}
			seenKeys = map[string]bool{}
			lastKey = ""
			inEntry = false
			heading := strings.TrimPrefix(line, "### ")
			for _, kind := range []string{"add-goal", "amend-goal"} {
				if strings.HasPrefix(heading, kind+": ") {
					current = &ManifestEntry{Kind: kind, Id: strings.TrimSpace(strings.TrimPrefix(heading, kind+": "))}
					inEntry = true
					if kind == "add-goal" {
						addPosition++
						current.Position = addPosition
					}
				}
			}
			if current == nil {
				// An unrecognized entry heading is a tree nobody
				// reviewed, never commentary (R2-16).
				return nil, fmt.Errorf("manifest line %d: unknown entry heading %q — the schema admits add-goal and amend-goal", lineNo+1, line)
			}
			continue
		}
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "# ") {
			if err := flush(); err != nil {
				return nil, err
			}
			inEntry = false
			continue
		}
		if !inEntry || current == nil {
			continue
		}
		if strings.HasPrefix(line, "- ") {
			body := strings.TrimPrefix(line, "- ")
			key, value, found := strings.Cut(body, ":")
			if !found {
				return nil, fmt.Errorf("manifest line %d: no key in %q", lineNo+1, line)
			}
			key = strings.ToLower(strings.TrimSpace(key))
			if err := setValue(key, strings.TrimSpace(value)); err != nil {
				return nil, err
			}
			lastKey = key
			continue
		}
		if line == "" {
			continue
		}
		// A continuation line LF-joins the previous value; a literal
		// `- ` start was caught above as a key line — the schema
		// forbids ambiguity, so a continuation resembling a key is
		// unreachable here by construction.
		if lastKey == "" {
			return nil, fmt.Errorf("manifest line %d: prose inside %s %s before any key", lineNo+1, current.Kind, current.Id)
		}
		if err := appendContinuation(current, lastKey, line); err != nil {
			return nil, err
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if epochCount != 1 || shaCount != 1 {
		return nil, fmt.Errorf("manifest: MIGRATION_EPOCH and REVIEWED_SOURCE_SHA256 are required exactly once (got %d and %d)", epochCount, shaCount)
	}
	if _, err := time.Parse(time.RFC3339, m.Epoch); err != nil {
		return nil, fmt.Errorf("manifest: MIGRATION_EPOCH is not RFC3339: %v", err)
	}
	return m, nil
}

func appendContinuation(entry *ManifestEntry, key, line string) error {
	joined := "\n" + line
	switch key {
	case "intent":
		entry.Intent += joined
	case "next":
		entry.Next += joined
	case "parked-because":
		entry.ParkedBecause += joined
	case "origin":
		entry.Origin += joined
	default:
		return fmt.Errorf("manifest: key %q takes a single-line value", key)
	}
	return nil
}

// AddOpenedAt is the positional formula: EPOCH + (1000 + position)
// minutes, one-based in textual order of add-goal entries.
func (m *Manifest) AddOpenedAt(position int) (string, error) {
	epoch, err := time.Parse(time.RFC3339, m.Epoch)
	if err != nil {
		return "", err
	}
	return epoch.Add(time.Duration(1000+position) * time.Minute).UTC().Format(time.RFC3339), nil
}

// LegacyOpenedAt orders the converted legacy goals: EPOCH + position
// minutes, one-based in the ledger's textual order.
func (m *Manifest) LegacyOpenedAt(position int) (string, error) {
	epoch, err := time.Parse(time.RFC3339, m.Epoch)
	if err != nil {
		return "", err
	}
	return epoch.Add(time.Duration(position) * time.Minute).UTC().Format(time.RFC3339), nil
}
