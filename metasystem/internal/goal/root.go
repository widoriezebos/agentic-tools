package goal

// The root record of the multi-machine ledger (plans/goals/backlog.md).
// It carries what no single goal file can: the ledger's identity (a
// ULID minted once at migration or adoption, never rewritten), the
// format version, the sync mode, the migration binding, the Goal-free
// declaration, and the root-record History (declare-free, prune, and
// displacement acknowledgments write here).

import (
	"fmt"
	"strings"
)

// RootRecord is the parsed plans/goals/backlog.md.
type RootRecord struct {
	Identity       string // ULID, minted once, the ledger identity
	FormatVersion  string
	SyncMode       string // remote | local, written once
	MigrationEpoch string // absent on fresh adoptions
	ManifestDigest string // absent on bare migrations and adoptions
	MigrationMode  string // bare | manifest | adoption
	Free           *FreeRecord
	Legacy         []string // root-level LegacyNotes from migration
	Revision       uint64
	History        []HistoryLine
}

// FreeRecord is the Goal-free declaration in the new ledger.
type FreeRecord struct {
	Declared string
	Origin   string
	Digest   string // freshness digest per the design's R7-11 domain
}

// Sync modes, closed.
const (
	SyncRemote = "remote"
	SyncLocal  = "local"
)

// ParseRoot parses the root record with the same strictness as goal
// files: every problem is returned and a problematic root refuses.
func ParseRoot(data []byte) (*RootRecord, []Problem) {
	var problems []Problem
	addProblem := func(format string, args ...any) {
		problems = append(problems, Problem(fmt.Sprintf(format, args...)))
	}

	body, integrity, ok := splitIntegrity(data)
	if !ok {
		addProblem("missing Integrity line")
	} else if got := IntegrityDigest(body); got != integrity {
		addProblem("Integrity mismatch: recorded %s, computed %s", integrity, got)
	}

	r := &RootRecord{}
	section := ""
	sawHeading := false
	seen := map[string]bool{}
	for i, raw := range strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n") {
		line := strings.TrimRight(raw, " \t")
		switch {
		case line == "# Backlog":
			sawHeading = true
		case line == "History:":
			section = "history"
		case line == "LegacyNotes:":
			section = "legacy"
		case section == "history" && strings.HasPrefix(line, "- "):
			h, err := ParseHistoryLine(line)
			if err != nil {
				addProblem("History line %d: %v", i+1, err)
				continue
			}
			r.History = append(r.History, h)
		case section == "legacy" && line != "":
			r.Legacy = append(r.Legacy, line)
		case strings.HasPrefix(line, "- "):
			parseRootField(r, strings.TrimPrefix(line, "- "), seen, addProblem)
		case strings.TrimSpace(line) == "":
		default:
			if section == "" {
				addProblem("unparseable line %d: %q", i+1, line)
			}
		}
	}

	if !sawHeading {
		addProblem("missing # Backlog heading")
	}
	if r.Identity == "" {
		addProblem("missing Identity — the ledger identity is minted once and never rewritten")
	} else if !ulidShaped(r.Identity) {
		addProblem("Identity %q is not ULID-shaped (26 Crockford base32 characters)", r.Identity)
	}
	if r.SyncMode != SyncRemote && r.SyncMode != SyncLocal {
		addProblem("SyncMode %q is not remote|local", r.SyncMode)
	}
	if r.FormatVersion != "1" {
		// A version this reader does not know is a tree it must not
		// trust — refusal is the forward-compatibility story.
		addProblem("FormatVersion %q is not 1", r.FormatVersion)
	}
	if r.Revision == 0 {
		addProblem("missing or zero Revision")
	}
	if r.MigrationEpoch != "" && !validStamp(r.MigrationEpoch) {
		addProblem("MigrationEpoch %q is not an RFC3339 timestamp", r.MigrationEpoch)
	}
	if r.ManifestDigest != "" && !hexDigest(r.ManifestDigest) {
		addProblem("ManifestDigest %q is not a sha256 hex digest", r.ManifestDigest)
	}
	if r.MigrationMode != "" && r.MigrationMode != "manifest" && r.MigrationMode != "bare" {
		addProblem("MigrationMode %q is not manifest|bare", r.MigrationMode)
	}
	if r.Free != nil && !validStamp(r.Free.Declared) {
		addProblem("Goal-free declared=%q is not an RFC3339 timestamp", r.Free.Declared)
	}
	return r, problems
}

// ulidShaped admits exactly the 26-character Crockford base32 form
// the identity mint produces.
func ulidShaped(s string) bool {
	if len(s) != 26 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", r) {
			return false
		}
	}
	return true
}

// hexDigest admits a 64-character lowercase sha256 hex literal.
func hexDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func parseRootField(r *RootRecord, field string, seen map[string]bool, addProblem func(string, ...any)) {
	key, value, found := strings.Cut(field, ":")
	if !found {
		addProblem("field without colon: %q", field)
		return
	}
	if seen[key] {
		addProblem("duplicate field %q — the last write would silently win", key)
		return
	}
	seen[key] = true
	value = strings.TrimSpace(value)
	switch key {
	case "Identity":
		r.Identity = value
	case "FormatVersion":
		r.FormatVersion = value
	case "SyncMode":
		r.SyncMode = value
	case "MigrationEpoch":
		r.MigrationEpoch = value
	case "ManifestDigest":
		r.ManifestDigest = value
	case "MigrationMode":
		r.MigrationMode = value
	case "Revision":
		var n uint64
		if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
			addProblem("Revision %q is not an unsigned integer", value)
			return
		}
		r.Revision = n
	case "Goal-free":
		rec, err := parseKVRecord(value, []string{"declared", "origin", "digest"}, nil)
		if err != nil {
			addProblem("Goal-free: %v", err)
			return
		}
		r.Free = &FreeRecord{Declared: rec["declared"], Origin: rec["origin"], Digest: rec["digest"]}
	default:
		addProblem("unknown field %q", key)
	}
}

// RenderRoot writes the canonical root-record bytes, Integrity included.
func RenderRoot(r *RootRecord) []byte {
	var b strings.Builder
	b.WriteString("# Backlog\n\n")
	fmt.Fprintf(&b, "- Identity: %s\n", r.Identity)
	fmt.Fprintf(&b, "- FormatVersion: %s\n", r.FormatVersion)
	fmt.Fprintf(&b, "- SyncMode: %s\n", r.SyncMode)
	if r.MigrationEpoch != "" {
		fmt.Fprintf(&b, "- MigrationEpoch: %s\n", r.MigrationEpoch)
	}
	if r.ManifestDigest != "" {
		fmt.Fprintf(&b, "- ManifestDigest: %s\n", r.ManifestDigest)
	}
	if r.MigrationMode != "" {
		fmt.Fprintf(&b, "- MigrationMode: %s\n", r.MigrationMode)
	}
	fmt.Fprintf(&b, "- Revision: %d\n", r.Revision)
	if r.Free != nil {
		fmt.Fprintf(&b, "- Goal-free: declared=%s origin=%s digest=%s\n", r.Free.Declared, r.Free.Origin, r.Free.Digest)
	}
	if len(r.Legacy) > 0 {
		b.WriteString("\nLegacyNotes:\n")
		for _, l := range r.Legacy {
			b.WriteString(l + "\n")
		}
	}
	b.WriteString("\nHistory:\n")
	for _, h := range r.History {
		b.WriteString(RenderHistoryLine(h) + "\n")
	}
	body := []byte(b.String())
	return append(body, []byte(fmt.Sprintf("Integrity: sha256=%s\n", IntegrityDigest(body)))...)
}
