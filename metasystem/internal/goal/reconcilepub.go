package goal

// Reconcile, stage three: the publish loop. The mapped rows apply
// onto the FETCHED tip's tree — never the base, never the snapshot
// — inside ONE transaction commit, each row with its verb's
// complete effects and every touched file gaining the one opid's
// history line under the HUMAN actor (a hand edit IS a human act).
// A row whose before-predicate fails on the fetched tip is a
// CONFLICT, refused naming the goal and the field. After
// confirmation the refresh rematerializes the checkout with the
// post-capture-edit protection stage one built.

import (
	"fmt"
	"strings"
)

// ReconcileResult reports one reconcile session.
type ReconcileResult struct {
	Publish PublishResult
	Rows    []MappedVerb
	Skipped []string // files edited after capture, preserved and named
}

// Reconcile runs the whole session: capture, map, publish, refresh.
// The actor must carry its human — reconcile IS the hand-edit path.
func Reconcile(r VerbRequest) (ReconcileResult, error) {
	if r.Actor.Human == "" {
		return ReconcileResult{}, fmt.Errorf("reconcile republishes hand edits and names its human (--by)")
	}
	base, err := BaseTip(r.Endpoint.Root)
	if err != nil {
		return ReconcileResult{}, err
	}
	snap, err := CaptureSnapshot(r.Endpoint.Root)
	if err != nil {
		return ReconcileResult{}, err
	}
	rows, err := MapDeltas(r.Endpoint.Root, base, snap)
	if err != nil {
		return ReconcileResult{}, err
	}
	if len(rows) == 0 {
		return ReconcileResult{Rows: rows}, nil
	}

	targets := make([]string, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, row.Id)
	}
	res, err := Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "reconcile", Targets: targets, Args: map[string]string{
			"by": r.Actor.Human, "rows": fmt.Sprintf("%d", len(rows)),
		}},
		Message: "goal reconcile (" + r.Actor.Human + ")",
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			var changes []Change
			touched := map[string]bool{}
			for _, row := range rows {
				rowChanges, err := applyRow(t, r, row)
				if err != nil {
					return nil, err
				}
				for _, c := range rowChanges {
					if touched[c.Path] {
						// A later row re-renders the same file; the
						// LAST render wins and the earlier change is
						// replaced below.
						continue
					}
					touched[c.Path] = true
					changes = append(changes, c)
				}
			}
			// Re-render every touched live file once, LAST state wins
			// (a park row and an edit row on one goal compose).
			for i, c := range changes {
				if c.Delete {
					continue
				}
				id := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(c.Path, goalsPrefix+"done/"), goalsPrefix), ".md")
				if f, live := t.Live[id]; live {
					changes[i].Content = RenderFile(f)
				} else if f, done := t.Done[id]; done {
					changes[i].Content = RenderFile(f)
				}
			}
			return changes, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		return ReconcileResult{Publish: res, Rows: rows}, err
	}
	skipped, err := Refresh(r.Endpoint.Root, res.Commit, snap)
	if err != nil {
		return ReconcileResult{Publish: res, Rows: rows}, fmt.Errorf("published, but the refresh died; goal reconcile --refresh-only completes it: %w", err)
	}
	return ReconcileResult{Publish: res, Rows: rows, Skipped: skipped}, nil
}

// applyRow applies one mapped verb's complete effects onto the
// fetched tree, with the row's before-predicate as the conflict
// gate.
func applyRow(t *TreeGoals, r VerbRequest, row MappedVerb) ([]Change, error) {
	conflict := func(field, format string, args ...any) error {
		return fmt.Errorf("reconcile conflict on %s (%s): %s", row.Id, field, fmt.Sprintf(format, args...))
	}
	switch row.Verb {
	case "open":
		if _, exists := t.Live[row.Id]; exists {
			return nil, conflict("state", "the goal already exists on the fetched tip")
		}
		if _, archived := t.Done[row.Id]; archived {
			return nil, conflict("state", "the goal is archived on the fetched tip; reopen is a verb")
		}
		f := &GoalFile{Id: row.Id, State: StateQueued, OpenedAt: r.stamp(), Revision: 0}
		if row.Fields.Intent != nil {
			f.Intent = *row.Fields.Intent
		}
		if row.Fields.NextStep != nil {
			f.NextStep = *row.Fields.NextStep
		}
		if row.Fields.Origin != nil {
			f.Origin = *row.Fields.Origin
		}
		if row.Fields.Blocked != nil {
			f.Blocked = append([]string(nil), (*row.Fields.Blocked)...)
		}
		touch(f, r, "open", []string{row.Id})
		t.Live[row.Id] = f
		return []Change{{Path: livePath(row.Id), Content: RenderFile(f)}}, nil

	case "park":
		ids := row.ArcIds
		if len(ids) == 0 {
			ids = []string{row.Id}
		}
		var changes []Change
		for _, id := range ids {
			f, exists := t.Live[id]
			if !exists {
				return nil, conflict("state", "%s is not live on the fetched tip", id)
			}
			if f.State != StateQueued && f.State != StateClaimed {
				return nil, conflict("state", "%s is %s on the fetched tip", id, f.State)
			}
			if f.State == StateClaimed && f.Claimed != nil && f.Claimed.Machine != r.Actor.Machine {
				// The human parks a foreign claim lawfully; the
				// displaced pair is recorded.
				f.Parked = &ParkRecord{By: r.Actor.historyActor(), At: r.stamp(), Because: row.Because,
					Displaced: f.Claimed.Machine + "+" + f.Claimed.Lineage + "@" + f.Claimed.At}
			} else {
				f.Parked = &ParkRecord{By: r.Actor.historyActor(), At: r.stamp(), Because: row.Because}
			}
			f.State = StateParked
			f.Claimed = nil
			f.Revision++
			f.History = append(f.History, HistoryLine{
				At: r.stamp(), Opid: r.opid(), Verb: "park",
				Actor: r.Actor.historyActor(), Targets: ids,
				Displaced: f.Parked.Displaced, Keep: -1, Reason: row.Because,
			})
			changes = append(changes, Change{Path: livePath(id), Content: RenderFile(f)})
		}
		return changes, nil

	case "unpark":
		f, exists := t.Live[row.Id]
		if !exists {
			return nil, conflict("state", "not live on the fetched tip")
		}
		if f.State != StateParked {
			return nil, conflict("state", "is %s on the fetched tip, not parked", f.State)
		}
		f.State = StateQueued
		f.Parked = nil
		touch(f, r, "unpark", []string{row.Id})
		return []Change{{Path: livePath(row.Id), Content: RenderFile(f)}}, nil

	case "done":
		f, exists := t.Live[row.Id]
		if !exists {
			return nil, conflict("state", "not live on the fetched tip")
		}
		for _, dep := range f.Blocked {
			if depState(t, dep) != StateDone {
				return nil, conflict("blockedBy", "blocker %s is not done on the fetched tip", dep)
			}
		}
		f.State = StateDone
		f.Conclude = row.Conclude
		f.Claimed = nil
		f.Parked = nil
		touch(f, r, "done", []string{row.Id})
		delete(t.Live, row.Id)
		t.Done[row.Id] = f
		return []Change{
			{Path: livePath(row.Id), Delete: true},
			{Path: donePath(row.Id), Content: RenderFile(f)},
		}, nil

	case "edit":
		f, exists := t.Live[row.Id]
		if !exists {
			return nil, conflict("state", "not live on the fetched tip")
		}
		if row.Fields.Intent != nil {
			f.Intent = *row.Fields.Intent
		}
		if row.Fields.NextStep != nil {
			f.NextStep = *row.Fields.NextStep
		}
		if row.Fields.Origin != nil {
			f.Origin = *row.Fields.Origin
		}
		if row.Fields.Blocked != nil {
			f.Blocked = append([]string(nil), (*row.Fields.Blocked)...)
		}
		touch(f, r, "edit", []string{row.Id})
		return []Change{{Path: livePath(row.Id), Content: RenderFile(f)}}, nil
	}
	return nil, fmt.Errorf("reconcile has no application for verb %q", row.Verb)
}
