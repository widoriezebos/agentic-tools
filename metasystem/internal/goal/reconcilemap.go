package goal

// Reconcile, stage two (R9-03): the hand-edit grammar is
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

// MappedVerb is one lawful row delta the hand edit decomposed into.
type MappedVerb struct {
	Verb     string // open | park | unpark | done | reopen | edit
	Id       string
	Because  string // park
	Conclude string // done
	Fields   EditFields
	ArcIds   []string // cascade park: every live member
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
			mapped = append(mapped, MappedVerb{Verb: "open", Id: id, Fields: EditFields{
				Intent: &edited.Intent, NextStep: &edited.NextStep, Origin: &edited.Origin,
				Blocked: &edited.Blocked,
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
			edited, _ := ParseFile(handLenient(snap.Files[d.Path]))
			if edited == nil {
				return nil, fmt.Errorf("%s: the edited file does not parse", d.Path)
			}
			rows, err := mapOneChange(d.Path, baseFile, edited)
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				if row.Verb == "park" {
					parks[row.Because] = append(parks[row.Because], row.Id)
					parkArcs[row.Id] = baseFile.Arc
					continue
				}
				mapped = append(mapped, row)
			}
		}
	}

	// Cascade recognition: identical park deltas across ALL of an
	// arc's live members map to ONE cascade park; a partial arc
	// refuses — cascades are all-or-none.
	for because, ids := range parks {
		byArc := map[string][]string{}
		for _, id := range ids {
			byArc[parkArcs[id]] = append(byArc[parkArcs[id]], id)
		}
		for arc, members := range byArc {
			if arc == "" {
				for _, id := range members {
					mapped = append(mapped, MappedVerb{Verb: "park", Id: id, Because: because})
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
			mapped = append(mapped, MappedVerb{Verb: "park", Id: members[0], Because: because, ArcIds: allLive})
		}
	}
	return mapped, nil
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
	if len(edited.History) != len(base.History) {
		return nil, fmt.Errorf("%s: History is a generated field; the engine synthesizes it", p)
	}
	if edited.OpenedAt != base.OpenedAt {
		return nil, fmt.Errorf("%s: OpenedAt is a generated field; the engine synthesizes it", p)
	}
	if (edited.Claimed == nil) != (base.Claimed == nil) ||
		(edited.Claimed != nil && *edited.Claimed != *base.Claimed) {
		return nil, fmt.Errorf("%s: Claimed is a generated field; claim and release are verbs", p)
	}

	var rows []MappedVerb
	// The state verb, in pinned precedence.
	if edited.State != base.State {
		switch {
		case base.State == StateQueued && edited.State == StateParked:
			if edited.Parked == nil || edited.Parked.Because == "" {
				return nil, fmt.Errorf("%s: a hand-park needs its Parked because", p)
			}
			rows = append(rows, MappedVerb{Verb: "park", Id: base.Id, Because: edited.Parked.Because})
		case base.State == StateParked && edited.State == StateQueued:
			rows = append(rows, MappedVerb{Verb: "unpark", Id: base.Id})
		case edited.State == StateDone:
			if edited.Conclude == "" {
				return nil, fmt.Errorf("%s: a hand-done needs its Concluded", p)
			}
			rows = append(rows, MappedVerb{Verb: "done", Id: base.Id, Conclude: edited.Conclude})
		default:
			return nil, fmt.Errorf("%s: the state change %s to %s has no hand-edit grammar", p, base.State, edited.State)
		}
	} else if parkReasonChanged(base, edited) {
		return nil, fmt.Errorf("%s: Parked is written by the park verb; a changed reason has no hand-edit grammar", p)
	}

	// One edit for the field remainder, over the CLOSED surface.
	fields := EditFields{}
	editNeeded := false
	if edited.Intent != base.Intent {
		fields.Intent = &edited.Intent
		editNeeded = true
	}
	if edited.NextStep != base.NextStep {
		fields.NextStep = &edited.NextStep
		editNeeded = true
	}
	if edited.Origin != base.Origin {
		fields.Origin = &edited.Origin
		editNeeded = true
	}
	if strings.Join(edited.Blocked, ",") != strings.Join(base.Blocked, ",") {
		blocked := append([]string(nil), edited.Blocked...)
		fields.Blocked = &blocked
		editNeeded = true
	}
	if edited.Arc != base.Arc {
		return nil, fmt.Errorf("%s: Arc membership moves through set-arc and detach, not hand edits", p)
	}
	if edited.Conclude != base.Conclude && edited.State == base.State {
		return nil, fmt.Errorf("%s: Concluded belongs to done; editing it in place has no grammar", p)
	}
	if editNeeded {
		rows = append(rows, MappedVerb{Verb: "edit", Id: base.Id, Fields: fields})
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
		emptyBy := strings.Contains(line, "by= ") || strings.Contains(line, "by=at=")
		emptyAt := strings.Contains(line, "at= ") || strings.HasSuffix(strings.TrimSpace(line), "at=")
		if !emptyBy && !emptyAt {
			continue
		}
		// Rebuild the line whole from its because (and displaced)
		// tail: the human typed only the reason; publication owns
		// the rest.
		because := ""
		if idx := strings.Index(line, "because="); idx >= 0 {
			because = line[idx+len("because="):]
		}
		displaced := ""
		if idx := strings.Index(line, "displaced="); idx >= 0 {
			tail := line[idx+len("displaced="):]
			if fields := strings.Fields(tail); len(fields) > 0 {
				displaced = fields[0]
			}
		}
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
