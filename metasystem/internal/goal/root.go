package goal

// The root record of the multi-machine ledger (plans/goals/backlog.md).
// It carries what no single goal file can: the ledger's identity (a
// ULID minted once at migration or adoption, never rewritten), the
// format version, the sync mode, the migration binding, the Goal-free
// declaration, and the root-record History (declare-free, prune, and
// displacement acknowledgments write here).

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// RootRecord is the parsed plans/goals/backlog.md.
type RootRecord struct {
	Identity        string // ULID, minted once, the ledger identity
	FormatVersion   string
	SyncMode        string // remote | local, written once
	MigrationEpoch  string // absent on fresh adoptions
	ManifestDigest  string // absent on bare migrations and adoptions
	MigrationMode   string // bare | manifest | adoption
	Free            *FreeRecord
	ApprovalGate    *ApprovalGateRecord
	TierLaw         string // operation id of the final classification edit
	FleetEnrollment *FleetEnrollmentRecord
	Decomposed      []DecomposedEntry
	Legacy          []string // root-level LegacyNotes from migration
	Revision        uint64
	History         []HistoryLine
}

// ApprovalGateRecord permanently marks when the execution-approval invariant
// armed for this ledger.
type ApprovalGateRecord struct {
	Since string
	Opid  string
}

// FleetEnrollmentRecord is the first enrolled human terminal observed by the
// synced fleet. It never changes once written.
type FleetEnrollmentRecord struct {
	At         string
	Machine    string
	Generation uint64
	Opid       string
}

// DecomposedEntry permanently retires one parent identifier after split.
type DecomposedEntry struct {
	Id     string
	Opid   string
	At     string
	OldArc string
}

// FreeRecord is the Goal-free declaration in the new ledger.
type FreeRecord struct {
	Declared string
	Origin   string
	Digest   string // freshness digest over the declared plans world
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
		case line == "Decomposed:":
			section = "decomposed"
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
			r.Legacy = append(r.Legacy, strings.TrimPrefix(line, "  "))
		case section == "decomposed" && strings.HasPrefix(line, "- "):
			entry, entryErr := parseDecomposedEntry(strings.TrimPrefix(line, "- "))
			if entryErr != nil {
				addProblem("Decomposed line %d: %v", i+1, entryErr)
				continue
			}
			r.Decomposed = append(r.Decomposed, entry)
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
	if r.ApprovalGate != nil && (!validStamp(r.ApprovalGate.Since) || r.ApprovalGate.Opid == "") {
		addProblem("ApprovalGate is incomplete")
	}
	if r.TierLaw != "" && !validOpidShape(r.TierLaw) {
		addProblem("TierLaw since=%q is not an operation id", r.TierLaw)
	}
	if r.FleetEnrollment != nil && (!validStamp(r.FleetEnrollment.At) || r.FleetEnrollment.Machine == "" ||
		r.FleetEnrollment.Generation == 0 || r.FleetEnrollment.Opid == "") {
		addProblem("FleetEnrollment is incomplete")
	}
	seenDecomposed := map[string]bool{}
	for _, entry := range r.Decomposed {
		if seenDecomposed[entry.Id] {
			addProblem("Decomposed contains duplicate parent %s", entry.Id)
		}
		seenDecomposed[entry.Id] = true
	}
	return r, problems
}

func parseDecomposedEntry(value string) (DecomposedEntry, error) {
	fields := strings.Fields(value)
	if len(fields) != 4 || !validId(fields[0]) {
		return DecomposedEntry{}, fmt.Errorf("expected <goal-id> opid=<opid> at=<RFC3339> oldArc=<goal-id|->")
	}
	opid, opidFound := strings.CutPrefix(fields[1], "opid=")
	at, atFound := strings.CutPrefix(fields[2], "at=")
	oldArc, oldArcFound := strings.CutPrefix(fields[3], "oldArc=")
	if !opidFound || !atFound || !oldArcFound || !validOpidShape(opid) || !validStamp(at) ||
		(oldArc != "-" && !validId(oldArc)) {
		return DecomposedEntry{}, fmt.Errorf("expected <goal-id> opid=<opid> at=<RFC3339> oldArc=<goal-id|->")
	}
	if oldArc == "-" {
		oldArc = ""
	}
	return DecomposedEntry{Id: fields[0], Opid: opid, At: at, OldArc: oldArc}, nil
}

func rootDecomposed(root *RootRecord, id string) (DecomposedEntry, bool) {
	if root == nil {
		return DecomposedEntry{}, false
	}
	for _, entry := range root.Decomposed {
		if entry.Id == id {
			return entry, true
		}
	}
	return DecomposedEntry{}, false
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
		n, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			addProblem("Revision %q is not an unsigned integer", value)
			return
		}
		r.Revision = n
	case "Goal-free":
		rec, err := parseKVRecord(value, []string{"declared", "origin", "digest"}, nil, "")
		if err != nil {
			addProblem("Goal-free: %v", err)
			return
		}
		r.Free = &FreeRecord{Declared: rec["declared"], Origin: rec["origin"], Digest: rec["digest"]}
	case "ApprovalGate":
		rec, err := parseKVRecord(value, []string{"since", "opid"}, nil, "")
		if err != nil {
			addProblem("ApprovalGate: %v", err)
			return
		}
		r.ApprovalGate = &ApprovalGateRecord{Since: rec["since"], Opid: rec["opid"]}
	case "TierLaw":
		rec, err := parseKVRecord(value, []string{"since"}, nil, "")
		if err != nil {
			addProblem("TierLaw: %v", err)
			return
		}
		r.TierLaw = rec["since"]
	case "FleetEnrollment":
		rec, err := parseKVRecord(value, []string{"at", "machine", "generation", "opid"}, nil, "")
		if err != nil {
			addProblem("FleetEnrollment: %v", err)
			return
		}
		generation, generationErr := strconv.ParseUint(rec["generation"], 10, 64)
		if generationErr != nil || generation == 0 {
			addProblem("FleetEnrollment has invalid generation")
			return
		}
		r.FleetEnrollment = &FleetEnrollmentRecord{At: rec["at"], Machine: rec["machine"], Generation: generation, Opid: rec["opid"]}
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
	if r.ApprovalGate != nil {
		fmt.Fprintf(&b, "- ApprovalGate: since=%s opid=%s\n", r.ApprovalGate.Since, r.ApprovalGate.Opid)
	}
	if r.TierLaw != "" {
		fmt.Fprintf(&b, "- TierLaw: since=%s\n", r.TierLaw)
	}
	if r.FleetEnrollment != nil {
		fmt.Fprintf(&b, "- FleetEnrollment: at=%s machine=%s generation=%d opid=%s\n",
			r.FleetEnrollment.At, r.FleetEnrollment.Machine, r.FleetEnrollment.Generation, r.FleetEnrollment.Opid)
	}
	if len(r.Legacy) > 0 {
		b.WriteString("\nLegacyNotes:\n")
		for _, l := range r.Legacy {
			// Indented for the same reason as the goal files': the
			// carried prose stays opaque to the structural parser.
			b.WriteString("  " + l + "\n")
		}
	}
	if len(r.Decomposed) > 0 {
		entries := append([]DecomposedEntry(nil), r.Decomposed...)
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].At == entries[j].At {
				return entries[i].Id < entries[j].Id
			}
			return entries[i].At < entries[j].At
		})
		b.WriteString("\nDecomposed:\n")
		for _, entry := range entries {
			oldArc := entry.OldArc
			if oldArc == "" {
				oldArc = "-"
			}
			fmt.Fprintf(&b, "- %s opid=%s at=%s oldArc=%s\n", entry.Id, entry.Opid, entry.At, oldArc)
		}
	}
	b.WriteString("\nHistory:\n")
	for _, h := range r.History {
		b.WriteString(RenderHistoryLine(h) + "\n")
	}
	body := []byte(b.String())
	return append(body, []byte(fmt.Sprintf("Integrity: sha256=%s\n", IntegrityDigest(body)))...)
}
