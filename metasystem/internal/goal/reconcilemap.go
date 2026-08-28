package goal

// Reconcile, stage two: the hand-edit grammar is
// EXECUTABLE. Generated fields — Revision, History, Integrity,
// Claimed, OpenedAt — are IGNORED as input and synthesized at
// publication; a hand-supplied generated value that DIFFERS from
// the base refuses by file and field, never silently rewritten. A
// multi-field delta on one goal maps to the SMALLEST verb set in
// pinned precedence — the state verb first (open/park/unpark/done/
// reopen), then ONE edit for every remaining field change. The
// editable surface is CLOSED; anything else is unmappable and
// refuses by file and field. Identical park deltas across ALL of an
// arc's live members map to one cascade park; a partial-arc park
// refuses. All mapped verbs carry the human actor — a hand edit IS
// a human act.

import (
	"fmt"
	"path"
	"strings"
)

// MappedVerb is one lawful row delta the hand edit decomposed
// into, carrying the BASE-side values its replay compares against
// : a concurrent edit that moved a field past the base is a
// conflict, never an overwrite.
type MappedVerb struct {
	Verb      string // open | park | unpark | done | reopen | edit
	Id        string
	Because   string // park
	Conclude  string // done
	Fields    EditFields
	Base      EditFields // the base's values for every changed field
	BaseState string     // the base's state, for state rows
	Arc       string     // set-arc: the destination arc
	Origin    string     // open rows only: creation provenance, never an edit
	BaseArc   string     // the base's arc, compared at replay
	ArcIds    []string   // cascade park: every live member
	// ArcBaseStates carries each cascade member's own before-state
	//: the cascade is one row, the conflicts are per member.
	ArcBaseStates map[string]string
}

// baseStateFor answers the before-state the replay compares for one
// member: the cascade map when present, the row's own otherwise.
func (v MappedVerb) baseStateFor(id string) string {
	if v.ArcBaseStates != nil {
		return v.ArcBaseStates[id]
	}
	if id == v.Id {
		return v.BaseState
	}
	return ""
}

// MapDeltas decomposes the snapshot-vs-base delta into lawful verb
// rows, refusing the unmappable by file and field.
func MapDeltas(repoRoot, baseCommit string, snap *Snapshot) ([]MappedVerb, error) {
	baseTree, err := loadTree(repoRoot, baseCommit)
	if err != nil {
		return nil, err
	}
	deltas, err := DiffAgainstBase(repoRoot, baseCommit, snap)
	if err != nil {
		return nil, err
	}

	var mapped []MappedVerb
	parks := map[string][]string{} // because -> member ids, for cascade recognition
	parkArcs := map[string]string{}
	parkStates := map[string]string{}

	for _, d := range deltas {
		id := strings.TrimSuffix(path.Base(d.Path), ".md")
		isArchive := strings.HasPrefix(d.Path, goalsPrefix+"done/")
		if d.Path == goalsPrefix+"backlog.md" {
			return nil, fmt.Errorf("%s: the root record has no hand-edit grammar; declare-free and prune are verbs", d.Path)
		}
		switch d.Kind {
		case "added":
			if isArchive {
				return nil, fmt.Errorf("%s: hand-creating archive entries is unmappable; done is a verb", d.Path)
			}
			edited, problems := ParseFile(snap.Files[d.Path])
			// A hand-created file needs only the human fields; parse
			// problems about generated fields are forgiven HERE
			// because synthesis owns them — but the file must at
			// least parse structurally.
			if edited == nil {
				return nil, fmt.Errorf("%s: the hand-created file does not parse: %v", d.Path, problems)
			}
			if edited.Id != id {
				return nil, fmt.Errorf("%s: file name and Id disagree (%s)", d.Path, edited.Id)
			}
			if edited.State != "" && edited.State != StateQueued {
				return nil, fmt.Errorf("%s: a hand-created goal opens queued; %s is unmappable", d.Path, edited.State)
			}
			if edited.Pinned != "" {
				return nil, fmt.Errorf("%s: a hand-created goal carries no pin; open it, then pin with set-pin", d.Path)
			}
			if edited.Budget != nil {
				return nil, fmt.Errorf("%s: a hand-created goal carries no budget; open it, then use goal set-budget", d.Path)
			}
			mapped = append(mapped, MappedVerb{Verb: "open", Id: id, Origin: edited.Origin, Fields: EditFields{
				Intent: &edited.Intent, NextStep: &edited.NextStep,
				Blocked: &edited.Blocked, Labels: &edited.Labels,
			}})
		case "removed":
			return nil, fmt.Errorf("%s: hand-deleting goal files is unmappable; done and prune are verbs", d.Path)
		case "changed":
			if isArchive {
				return nil, fmt.Errorf("%s: the archive has no hand-edit grammar; reopen is a verb", d.Path)
			}
			baseFile := baseTree.Live[id]
			if baseFile == nil {
				return nil, fmt.Errorf("%s: changed against a base that does not carry it", d.Path)
			}
			edited, problems := ParseFile(handLenient(snap.Files[d.Path]))
			if edited == nil {
				return nil, fmt.Errorf("%s: the edited file does not parse", d.Path)
			}
			// Integrity diagnostics are the hand edit's OWN signature
			// — the human changed bytes under a machine digest, and
			// publication re-renders both. Every OTHER diagnostic
			// (an unknown field, a duplicate key, a malformed value)
			// refuses: silently publishing a cleaned-up version of
			// what the human wrote rewrites their edit.
			for _, problem := range problems {
				if strings.HasPrefix(string(problem), "Integrity mismatch") || string(problem) == "missing Integrity line" {
					continue
				}
				// handLenient's own placeholders provoke shape
				// diagnostics by construction; publication
				// synthesizes the real human and stamp.
				if strings.Contains(string(problem), "pending-human") || strings.Contains(string(problem), "pending-stamp") {
					continue
				}
				return nil, fmt.Errorf("%s: the hand edit carries a diagnostic the surface refuses: %s", d.Path, problem)
			}
			rows, err := mapOneChange(d.Path, baseFile, edited)
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				if row.Verb == "park" {
					parks[row.Because] = append(parks[row.Because], row.Id)
					parkArcs[row.Id] = baseFile.Arc
					parkStates[row.Id] = row.BaseState
					continue
				}
				mapped = append(mapped, row)
			}
		}
	}

	// Cascade recognition: identical park deltas across ALL of an
	// arc's live members map to ONE cascade park; a partial arc
	// refuses — cascades are all-or-none.
	var parkRows []MappedVerb
	for because, ids := range parks {
		byArc := map[string][]string{}
		for _, id := range ids {
			byArc[parkArcs[id]] = append(byArc[parkArcs[id]], id)
		}
		for arc, members := range byArc {
			if arc == "" {
				for _, id := range members {
					parkRows = append(parkRows, MappedVerb{Verb: "park", Id: id, Because: because, BaseState: parkStates[id]})
				}
				continue
			}
			var allLive []string
			for _, liveId := range sortedGoalIds(baseTree.Live) {
				if baseTree.Live[liveId].Arc == arc {
					allLive = append(allLive, liveId)
				}
			}
			if len(members) != len(allLive) {
				return nil, fmt.Errorf("a partial-arc hand-park refuses: arc %s has %d live members, the edit parks %d — cascades are all-or-none", arc, len(allLive), len(members))
			}
			memberStates := map[string]string{}
			for _, id := range allLive {
				memberStates[id] = parkStates[id]
			}
			parkRows = append(parkRows, MappedVerb{Verb: "park", Id: members[0], Because: because, ArcIds: allLive, ArcBaseStates: memberStates})
		}
	}
	// State verbs run FIRST: cascade recognition deferred the park
	// rows past the ordinary rows, so a park+edit on one goal would
	// otherwise execute — and journal — the edit against the unparked
	// state, including a displacement the park itself owns.
	return append(parkRows, mapped...), nil
}

// mapOneChange decomposes one changed file into its smallest verb
// set: the state verb first, then one edit for the field remainder.
func mapOneChange(p string, base, edited *GoalFile) ([]MappedVerb, error) {
	// Generated fields: a hand-supplied value that DIFFERS from the
	// base refuses by file and field. (Unchanged copies are the
	// ordinary shape — the file was materialized with them.)
	if edited.Revision != base.Revision {
		return nil, fmt.Errorf("%s: Revision is a generated field; the engine synthesizes it", p)
	}
	if renderHistory(edited.History) != renderHistory(base.History) {
		return nil, fmt.Errorf("%s: History is a generated field; the engine synthesizes it", p)
	}
	if edited.OpenedAt != base.OpenedAt {
		return nil, fmt.Errorf("%s: OpenedAt is a generated field; the engine synthesizes it", p)
	}
	// A hand park of a CLAIMED goal lawfully clears the Claimed line
	// — that is the park's own effect, synthesized either way at
	// replay. Every other Claimed alteration stays refused.
	claimClearedByPark := base.Claimed != nil && edited.Claimed == nil &&
		base.State == StateClaimed && edited.State == StateParked
	if !claimClearedByPark && ((edited.Claimed == nil) != (base.Claimed == nil) ||
		(edited.Claimed != nil && *edited.Claimed != *base.Claimed)) {
		return nil, fmt.Errorf("%s: Claimed is a generated field; claim and release are verbs", p)
	}

	var rows []MappedVerb
	// The state verb, in pinned precedence.
	if edited.State != base.State {
		switch {
		case (base.State == StateQueued || base.State == StateClaimed) && edited.State == StateParked:
			// A hand park is lawful from queued AND from claimed — the
			// pause lever's own predicate, all rows actor H;
			// the replay records displacement for a foreign claim.
			if edited.Parked == nil || edited.Parked.Because == "" {
				return nil, fmt.Errorf("%s: a hand-park needs its Parked because", p)
			}
			rows = append(rows, MappedVerb{Verb: "park", Id: base.Id, Because: edited.Parked.Because, BaseState: base.State})
		case base.State == StateParked && edited.State == StateQueued:
			rows = append(rows, MappedVerb{Verb: "unpark", Id: base.Id, BaseState: base.State})
		case edited.State == StateDone:
			if edited.Conclude == "" {
				return nil, fmt.Errorf("%s: a hand-done needs its Concluded", p)
			}
			rows = append(rows, MappedVerb{Verb: "done", Id: base.Id, Conclude: edited.Conclude, BaseState: base.State})
		default:
			return nil, fmt.Errorf("%s: the state change %s to %s has no hand-edit grammar", p, base.State, edited.State)
		}
	} else if parkReasonChanged(base, edited) {
		return nil, fmt.Errorf("%s: Parked is written by the park verb; a changed reason has no hand-edit grammar", p)
	}

	if edited.Pinned != base.Pinned {
		return nil, fmt.Errorf("%s: Pinned is written by the set-pin verb; a hand-edited pin has no reconcile grammar", p)
	}
	if (edited.Budget == nil) != (base.Budget == nil) ||
		(edited.Budget != nil && *edited.Budget != *base.Budget) {
		return nil, fmt.Errorf("%s: Budget is written by the set-budget verb; a hand-edited budget has no reconcile grammar", p)
	}

	// One edit for the field remainder, over the CLOSED surface.
	fields := EditFields{}
	baseFields := EditFields{}
	editNeeded := false
	if edited.Intent != base.Intent {
		fields.Intent = &edited.Intent
		baseFields.Intent = &base.Intent
		editNeeded = true
	}
	if edited.NextStep != base.NextStep {
		fields.NextStep = &edited.NextStep
		baseFields.NextStep = &base.NextStep
		editNeeded = true
	}
	if edited.Origin != base.Origin {
		// Origin is OUTSIDE the closed edit surface: the
		// provenance of a goal is not a hand-editable fact.
		return nil, fmt.Errorf("%s: Origin is not on the closed edit surface", p)
	}
	if strings.Join(edited.Blocked, ",") != strings.Join(base.Blocked, ",") {
		blocked := append([]string(nil), edited.Blocked...)
		baseBlocked := append([]string(nil), base.Blocked...)
		fields.Blocked = &blocked
		baseFields.Blocked = &baseBlocked
		editNeeded = true
	}
	if strings.Join(edited.Labels, ",") != strings.Join(base.Labels, ",") {
		labels := append([]string(nil), edited.Labels...)
		baseLabels := append([]string(nil), base.Labels...)
		fields.Labels = &labels
		baseFields.Labels = &baseLabels
		editNeeded = true
	}
	// Arc IS on the closed surface: a membership change maps
	// to its verbs — set-arc for a join or move, detach for a clear.
	if edited.Arc != base.Arc {
		if edited.Arc == "" {
			rows = append(rows, MappedVerb{Verb: "detach", Id: base.Id, BaseState: base.State, BaseArc: base.Arc})
		} else {
			rows = append(rows, MappedVerb{Verb: "set-arc", Id: base.Id, BaseState: base.State,
				Arc: edited.Arc, BaseArc: base.Arc})
		}
	}
	if edited.Conclude != base.Conclude && edited.State == base.State {
		return nil, fmt.Errorf("%s: Concluded belongs to done; editing it in place has no grammar", p)
	}
	if editNeeded {
		rows = append(rows, MappedVerb{Verb: "edit", Id: base.Id, Fields: fields, Base: baseFields, BaseState: base.State})
	}
	if len(rows) == 0 {
		// Bytes differ but no mapped field does: whitespace or an
		// unmodeled line — unmappable by name.
		return nil, fmt.Errorf("%s: the bytes changed but no editable field did; the surface is closed", p)
	}
	return rows, nil
}

// handLenient makes a hand-written park expressible: a Parked line
// whose by= or at= is empty gets placeholders BEFORE the strict
// parse — publication synthesizes the real values (the reconciling
// human and the transaction stamp), so the human types only the
// because. Nothing else is rewritten.
func handLenient(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "- Parked: ") {
			continue
		}
		// TOKEN boundaries, exactly as the strict parser reads the
		// line: a "by= " or "at=" buried inside the because's free
		// text must neither trigger a rebuild of a well-formed line
		// nor donate a fabricated value to one.
		tail := strings.TrimPrefix(line, "- Parked: ")
		by, at := "", ""
		byFound, atFound, displacedFound, becauseFound, foreignToken := false, false, false, false, false
		j := 0
		for j < len(tail) {
			for j < len(tail) && (tail[j] == ' ' || tail[j] == '\t') {
				j++
			}
			if j >= len(tail) {
				break
			}
			start := j
			for j < len(tail) && tail[j] != ' ' && tail[j] != '\t' {
				j++
			}
			token := tail[start:j]
			if strings.HasPrefix(token, "because=") {
				becauseFound = true
				break // free text; nothing past here is a key
			}
			switch {
			case strings.HasPrefix(token, "by="):
				if byFound {
					foreignToken = true // a duplicate is the strict parser's to refuse
				}
				by, byFound = strings.TrimPrefix(token, "by="), true
			case strings.HasPrefix(token, "at="):
				if atFound {
					foreignToken = true
				}
				at, atFound = strings.TrimPrefix(token, "at="), true
			case strings.HasPrefix(token, "displaced="):
				if displacedFound {
					foreignToken = true
				}
				displacedFound = true
			default:
				foreignToken = true
			}
		}
		if foreignToken || !becauseFound || (byFound && by != "" && atFound && at != "") {
			// No rebuild: a fully specified line stands as written, a
			// line with no because refuses as itself, and a line
			// carrying a token the grammar does not know must reach
			// the strict parser INTACT — rebuilding would silently
			// discard what the human typed.
			continue
		}
		// Rebuild the line whole from its because (and displaced)
		// tail: the human typed only the reason; publication owns
		// the rest.
		because, displaced := splitParkTail(tail)
		rebuilt := "- Parked: by=pending-human at=pending-stamp"
		if displaced != "" {
			rebuilt += " displaced=" + displaced
		}
		rebuilt += " because=" + because
		lines[i] = rebuilt
	}
	return []byte(strings.Join(lines, "\n"))
}

func parkReasonChanged(base, edited *GoalFile) bool {
	if base.Parked == nil || edited.Parked == nil {
		return false
	}
	return base.Parked.Because != edited.Parked.Because
}

// renderHistory serializes a history for content comparison — a
// same-length alteration must not slip past a length check.
func renderHistory(lines []HistoryLine) string {
	var b strings.Builder
	for _, h := range lines {
		b.WriteString(RenderHistoryLine(h))
		b.WriteString("\n")
	}
	return b.String()
}
