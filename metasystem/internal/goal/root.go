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
	"time"
)

var knownLedgerFormats = map[string]bool{"1": true, "2": true}

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
	// PowerOfAttorney lists the recorded delegations (goal grant): scoped
	// by tier and verb, expiring within seven days, revocable. A seat's
	// approve or set-budget under a live entry is the seat's own act with
	// the entry named on its history line.
	PowerOfAttorney []PowerOfAttorneyEntry
	Legacy          []string // root-level LegacyNotes from migration
	Revision        uint64
	History         []HistoryLine
}

// PowerOfAttorneyEntry is one recorded delegation. Its ID is the grant's
// operation id; Expires is a date, and the entry covers acts until that
// day ends; Revoked closes it earlier.
type PowerOfAttorneyEntry struct {
	ID        string
	By        string // human:<name>
	Tiers     []uint8
	Verbs     []string
	Since     string // RFC3339
	Expires   string // YYYY-MM-DD
	Revoked   string // RFC3339, empty while live
	RevokedBy string // human:<name>, empty while live
}

// AttorneyVerbs are the verbs a delegation may cover in this build.
var AttorneyVerbs = []string{"approve", "set-budget"}

// AttorneyMaxDays bounds an entry's life: expires is at most this many days
// after since (R-95-m1e: no entry lives longer than seven days).
const AttorneyMaxDays = 7

// Covers reports whether the entry names the verb and the tier.
func (e PowerOfAttorneyEntry) Covers(verb string, tier uint8) bool {
	return containsString(e.Verbs, verb) && containsTier(e.Tiers, tier)
}

// WithinBounds reports whether the entry keeps R-95-m1e's bounds: tier 1,
// the attorney verbs, an expiry within seven days of since, a revocation
// no earlier than since. The parser reads any well-formed entry so a landed
// ledger stays readable; an entry outside the bounds is never honoured.
func (e PowerOfAttorneyEntry) WithinBounds() (bool, string) {
	if len(e.Tiers) != 1 || e.Tiers[0] != 1 {
		return false, "tiers=" + renderTiers(e.Tiers) + " is outside tier 1"
	}
	for _, verb := range e.Verbs {
		if !containsString(AttorneyVerbs, verb) {
			return false, "verb " + verb + " is outside " + strings.Join(AttorneyVerbs, ",")
		}
	}
	since, err := time.Parse(time.RFC3339, e.Since)
	if err != nil {
		return false, "since " + e.Since + " is not RFC3339"
	}
	expires, err := time.Parse("2006-01-02", e.Expires)
	if err != nil {
		return false, "expiry " + e.Expires + " is not a date"
	}
	sinceDay := time.Date(since.UTC().Year(), since.UTC().Month(), since.UTC().Day(), 0, 0, 0, 0, time.UTC)
	if expires.Before(sinceDay) || expires.After(sinceDay.AddDate(0, 0, AttorneyMaxDays-1)) {
		return false, "expiry " + e.Expires + " is outside seven days from " + e.Since
	}
	if e.Revoked != "" {
		revoked, err := time.Parse(time.RFC3339, e.Revoked)
		if err != nil || revoked.Before(since) {
			return false, "revocation " + e.Revoked + " precedes the grant"
		}
	}
	return true, ""
}

// LiveAt reports whether the entry is within its bounds, unrevoked and not
// yet expired at now (the expiry day itself still counts).
func (e PowerOfAttorneyEntry) LiveAt(now time.Time) (bool, string) {
	if within, why := e.WithinBounds(); !within {
		return false, "outside its bounds: " + why
	}
	if e.Revoked != "" {
		return false, "revoked at " + e.Revoked + " by " + e.RevokedBy
	}
	expires, err := time.Parse("2006-01-02", e.Expires)
	if err != nil {
		return false, "expiry " + e.Expires + " is not a date"
	}
	day := now.UTC()
	today := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	if today.After(expires) {
		return false, "expired " + e.Expires
	}
	return true, ""
}

func containsTier(tiers []uint8, tier uint8) bool {
	for _, candidate := range tiers {
		if candidate == tier {
			return true
		}
	}
	return false
}

// rootAttorney finds one entry by id.
func rootAttorney(root *RootRecord, id string) (PowerOfAttorneyEntry, bool) {
	if root == nil {
		return PowerOfAttorneyEntry{}, false
	}
	for _, entry := range root.PowerOfAttorney {
		if entry.ID == id {
			return entry, true
		}
	}
	return PowerOfAttorneyEntry{}, false
}

func renderTiers(tiers []uint8) string {
	parts := make([]string, 0, len(tiers))
	for _, tier := range tiers {
		parts = append(parts, strconv.Itoa(int(tier)))
	}
	return strings.Join(parts, ",")
}

// ParseTiers reads a comma-separated tier list (1, 2 or 3), deduplicated
// and sorted.
func ParseTiers(value string) ([]uint8, error) {
	var tiers []uint8
	for _, part := range strings.Split(value, ",") {
		n, err := strconv.ParseUint(strings.TrimSpace(part), 10, 8)
		if err != nil || n < 1 || n > 3 {
			return nil, fmt.Errorf("tiers must list 1, 2 or 3")
		}
		if !containsTier(tiers, uint8(n)) {
			tiers = append(tiers, uint8(n))
		}
	}
	if len(tiers) == 0 {
		return nil, fmt.Errorf("tiers must list 1, 2 or 3")
	}
	sort.Slice(tiers, func(i, j int) bool { return tiers[i] < tiers[j] })
	return tiers, nil
}

func parseAttorneyEntry(value string) (PowerOfAttorneyEntry, error) {
	fields := strings.Fields(value)
	if len(fields) < 6 || !validOpidShape(fields[0]) {
		return PowerOfAttorneyEntry{}, fmt.Errorf("expected <opid> by=human:<name> tiers=<n,..> verbs=<verb,..> since=<RFC3339> expires=<YYYY-MM-DD> [revoked=<RFC3339> revokedBy=human:<name>]")
	}
	rec, err := parseKVRecord(strings.Join(fields[1:], " "), []string{"by", "tiers", "verbs", "since", "expires"}, []string{"revoked", "revokedBy"}, "")
	if err != nil {
		return PowerOfAttorneyEntry{}, err
	}
	tiers, err := ParseTiers(rec["tiers"])
	if err != nil {
		return PowerOfAttorneyEntry{}, err
	}
	entry := PowerOfAttorneyEntry{ID: fields[0], By: rec["by"], Tiers: tiers, Verbs: strings.Split(rec["verbs"], ","),
		Since: rec["since"], Expires: rec["expires"], Revoked: rec["revoked"], RevokedBy: rec["revokedBy"]}
	if !strings.HasPrefix(entry.By, "human:") || !validStamp(entry.Since) {
		return PowerOfAttorneyEntry{}, fmt.Errorf("by= must name a human and since= must be RFC3339")
	}
	if _, err := time.Parse("2006-01-02", entry.Expires); err != nil {
		return PowerOfAttorneyEntry{}, fmt.Errorf("expires= must be a date")
	}
	if (entry.Revoked != "" || entry.RevokedBy != "") && (!validStamp(entry.Revoked) || !strings.HasPrefix(entry.RevokedBy, "human:")) {
		return PowerOfAttorneyEntry{}, fmt.Errorf("revoked= must be RFC3339 with revokedBy=human:<name>")
	}
	for _, verb := range entry.Verbs {
		if strings.TrimSpace(verb) == "" {
			return PowerOfAttorneyEntry{}, fmt.Errorf("verbs= must list verbs")
		}
	}
	return entry, nil
}

func renderAttorneyEntry(e PowerOfAttorneyEntry) string {
	line := fmt.Sprintf("- %s by=%s tiers=%s verbs=%s since=%s expires=%s", e.ID, e.By, renderTiers(e.Tiers), strings.Join(e.Verbs, ","), e.Since, e.Expires)
	if e.Revoked != "" {
		line += " revoked=" + e.Revoked + " revokedBy=" + e.RevokedBy
	}
	return line
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
		case line == "PowerOfAttorney:":
			section = "attorney"
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
		case section == "attorney" && strings.HasPrefix(line, "- "):
			entry, entryErr := parseAttorneyEntry(strings.TrimPrefix(line, "- "))
			if entryErr != nil {
				addProblem("PowerOfAttorney line %d: %v", i+1, entryErr)
				continue
			}
			r.PowerOfAttorney = append(r.PowerOfAttorney, entry)
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
	if !knownLedgerFormats[r.FormatVersion] {
		// A version this reader does not know is a tree it must not
		// trust — refusal is the forward-compatibility story.
		formats := make([]string, 0, len(knownLedgerFormats))
		for format := range knownLedgerFormats {
			formats = append(formats, format)
		}
		sort.Strings(formats)
		addProblem("FormatVersion %q is not %s", r.FormatVersion, strings.Join(formats, " or "))
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
	seenAttorney := map[string]bool{}
	for _, entry := range r.PowerOfAttorney {
		if seenAttorney[entry.ID] {
			addProblem("PowerOfAttorney contains duplicate entry %s", entry.ID)
		}
		seenAttorney[entry.ID] = true
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
	if len(r.PowerOfAttorney) > 0 {
		entries := append([]PowerOfAttorneyEntry(nil), r.PowerOfAttorney...)
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Since == entries[j].Since {
				return entries[i].ID < entries[j].ID
			}
			return entries[i].Since < entries[j].Since
		})
		b.WriteString("\nPowerOfAttorney:\n")
		for _, entry := range entries {
			b.WriteString(renderAttorneyEntry(entry) + "\n")
		}
	}
	b.WriteString("\nHistory:\n")
	for _, h := range r.History {
		b.WriteString(RenderHistoryLine(h) + "\n")
	}
	body := []byte(b.String())
	return append(body, []byte(fmt.Sprintf("Integrity: sha256=%s\n", IntegrityDigest(body)))...)
}
