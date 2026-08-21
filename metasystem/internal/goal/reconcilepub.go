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
	"errors"
	"fmt"
	"os"
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
	MaintainBase(r.Endpoint.Root)
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
	// The pending record — snapshot included — lands durably BEFORE
	// the publish leaves this process (F10): a crash at ANY point
	// after publication finds the snapshot on disk and
	// --refresh-only completes exactly what the live session would
	// have done. The commit field is stamped after confirmation.
	if err := WriteBase(r.Endpoint.Root, BaseRecord{
		Commit: base, WrittenAt: nowISO8601(), RefreshDue: true,
		Publishing: true, Opid: r.opid(), Snapshot: snap.Files,
	}); err != nil {
		return ReconcileResult{}, err
	}
	res, err := Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  reconcileIntent(r.Actor.Human, targets, rows),
		Message: "goal reconcile (" + r.Actor.Human + ")",
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			var changes []Change
			touched := map[string]bool{}
			session := newReplaySession()
			for _, row := range rows {
				rowChanges, err := applyRow(t, r, row, session)
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
			// A reconcile is a publication like any other: standing
			// displacement addressed to this pair acks here (R9-06).
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
	if err != nil || res.Outcome != OutcomeConfirmed {
		// Only a DEFINITIVE non-landing clears the pending record: a
		// rejected, lost, or abandoned publish provably wrote nothing.
		// An unknown outcome (a pushed-but-unconfirmed crash window,
		// a failed confirming refetch) KEEPS the record and its
		// snapshot — --refresh-only resolves it by the opid's trailer
		// (R2-1). The JOURNAL decides definitiveness, not the returned
		// outcome: pre-push failures mark the
		// entry abandoned but return an empty result, and leaving the
		// pending record then blocks ordinary reconcile spuriously.
		definitive := false
		switch res.Outcome {
		case OutcomeRejected, OutcomeLost, OutcomeAbandoned:
			definitive = true
		default:
			if entry, readErr := ReadEntry(r.Endpoint.Root, r.opid()); readErr == nil {
				if entry.Phase == PhaseTerminal && entry.Outcome != OutcomeConfirmed && entry.Outcome != OutcomeConfirmedLate {
					definitive = true
				}
			} else if errors.Is(readErr, os.ErrNotExist) {
				// No entry at all is as definitive as an abandoned
				// one: entries are created before anything pushes, so
				// a publish refused pre-journal (an older pushed
				// entry blocking, a duplicate opid) provably wrote
				// nothing — leaving the pending record would block
				// ordinary reconcile behind a ghost.
				definitive = true
			}
		}
		if definitive {
			// Only THIS session's record clears: another session's
			// pending record (ours failed before writing, theirs is
			// mid-publish) carries the snapshot ITS recovery needs.
			if rec, exists, _ := ReadBase(r.Endpoint.Root); exists && rec.RefreshDue && rec.Opid == r.opid() {
				_ = WriteBase(r.Endpoint.Root, BaseRecord{Commit: rec.Commit, WrittenAt: rec.WrittenAt})
			}
		}
		return ReconcileResult{Publish: res, Rows: rows}, err
	}
	skipped, err := Refresh(r.Endpoint.Root, res.Commit, snap)
	if err != nil {
		return ReconcileResult{Publish: res, Rows: rows}, fmt.Errorf("published, but the refresh died; goal reconcile --refresh-only completes it: %w", err)
	}
	return ReconcileResult{Publish: res, Rows: rows, Skipped: skipped}, nil
}

// replaySession remembers what THIS reconcile's earlier rows already
// did to the tree: a later row for the same goal composes with those
// moves instead of re-litigating a base the session itself outdated.
type replaySession struct {
	archived map[string]bool
	moved    map[string]bool
}

func newReplaySession() *replaySession {
	return &replaySession{archived: map[string]bool{}, moved: map[string]bool{}}
}

// applyRow applies one mapped verb's complete effects onto the
// fetched tree, with the row's before-predicate as the conflict
// gate.
func applyRow(t *TreeGoals, r VerbRequest, row MappedVerb, session *replaySession) ([]Change, error) {
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
		f.Origin = row.Origin
		if row.Fields.Blocked != nil {
			f.Blocked = append([]string(nil), (*row.Fields.Blocked)...)
		}
		touch(f, r, "open", []string{row.Id})
		t.Live[row.Id] = f
		changes := []Change{{Path: livePath(row.Id), Content: RenderFile(f)}}
		changes = append(changes, clearFreeIfDeclared(t, r, row.Id)...)
		return changes, nil

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
			// The state before-value binds (R2-10): a hand park made
			// against queued must not swallow a claim that landed
			// meanwhile — the competitor's work is preserved and the
			// conflict named.
			if expected := row.baseStateFor(id); expected != "" && f.State != expected {
				return nil, conflict("state", "%s is %s on the fetched tip, the hand edit was made against %s", id, f.State, expected)
			}
			if f.State != StateQueued && f.State != StateClaimed {
				return nil, conflict("state", "%s is %s on the fetched tip", id, f.State)
			}
			if f.State == StateClaimed && f.Claimed != nil && !ownPair(f.Claimed, r.Actor) {
				// The human parks a foreign claim lawfully; the
				// displaced PAIR is recorded (R2-12: the full
				// ownership key, never the machine alone).
				f.Parked = &ParkRecord{By: r.Actor.historyActor(), At: r.stamp(), Because: row.Because,
					Displaced: pairMarker(f.Claimed)}
			} else {
				f.Parked = &ParkRecord{By: r.Actor.historyActor(), At: r.stamp(), Because: row.Because}
			}
			f.State = StateParked
			f.Claimed = nil
			session.moved[id] = true
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
		session.moved[row.Id] = true
		touch(f, r, "unpark", []string{row.Id})
		changes := []Change{{Path: livePath(row.Id), Content: RenderFile(f)}}
		changes = append(changes, clearFreeIfDeclared(t, r, row.Id)...)
		return changes, nil

	case "done":
		f, exists := t.Live[row.Id]
		if !exists {
			return nil, conflict("state", "not live on the fetched tip")
		}
		if row.BaseState != "" && f.State != row.BaseState {
			return nil, conflict("state", "is %s on the fetched tip, the hand edit was made against %s", f.State, row.BaseState)
		}
		for _, dep := range f.Blocked {
			if depState(t, dep) != StateDone {
				return nil, conflict("blockedBy", "blocker %s is not done on the fetched tip", dep)
			}
		}
		displaced := ""
		if f.State == StateClaimed && f.Claimed != nil && !ownPair(f.Claimed, r.Actor) {
			displaced = pairMarker(f.Claimed)
		}
		f.State = StateDone
		f.Conclude = row.Conclude
		f.Claimed = nil
		f.Parked = nil
		touchDisplaced(f, r, "done", []string{row.Id}, displaced)
		delete(t.Live, row.Id)
		t.Done[row.Id] = f
		session.archived[row.Id] = true
		session.moved[row.Id] = true
		return []Change{
			{Path: livePath(row.Id), Delete: true},
			{Path: donePath(row.Id), Content: RenderFile(f)},
		}, nil

	case "edit":
		// done+edit compose (F11): the edited goal may already have
		// moved to the archive by an earlier row in THIS session —
		// only that archive is lawful to keep editing. An archive
		// already on the fetched tip means another transaction
		// completed the goal while the hand edit was captured
		// against a live state: writing into it would rewrite a
		// finished record, so it conflicts like any other raced row.
		f, exists := t.Live[row.Id]
		archived := false
		if !exists {
			if df, inDone := t.Done[row.Id]; inDone {
				if !session.archived[row.Id] {
					return nil, conflict("state", "archived on the fetched tip; the hand edit was made against a live goal")
				}
				f, exists, archived = df, true, true
			}
		}
		if !exists {
			return nil, conflict("state", "not live on the fetched tip")
		}
		// The state before-value binds for edits too: a hand edit
		// captured against queued must not publish over a
		// concurrently landed claim as if displacement were consent.
		// The compose case is exempt — this session's own state row
		// already moved the goal, and ITS BaseState bound there.
		// An earlier row in THIS session lawfully moved the state
		// (unpark, detach, an arc join) and ITS BaseState bound at
		// that row — the edit that follows composes with the move
		// rather than rejecting it as a phantom race.
		if !archived && !session.moved[row.Id] && row.BaseState != "" && f.State != row.BaseState {
			return nil, conflict("state", "is %s on the fetched tip, the hand edit was made against %s", f.State, row.BaseState)
		}
		// The BASE comparison per field (F9): a concurrent edit that
		// moved a field past the base conflicts by name — the hand
		// edit never overwrites what it never saw.
		if row.Base.Intent != nil && f.Intent != *row.Base.Intent {
			return nil, conflict("intent", "the fetched tip carries %q, the hand edit was made against %q", f.Intent, *row.Base.Intent)
		}
		if row.Base.NextStep != nil && f.NextStep != *row.Base.NextStep {
			return nil, conflict("next", "the fetched tip carries %q, the hand edit was made against %q", f.NextStep, *row.Base.NextStep)
		}
		if row.Base.Blocked != nil && strings.Join(f.Blocked, ",") != strings.Join(*row.Base.Blocked, ",") {
			return nil, conflict("blockedBy", "the fetched tip's edges moved past the hand edit's base")
		}
		if row.Fields.Intent != nil {
			f.Intent = *row.Fields.Intent
		}
		if row.Fields.NextStep != nil {
			f.NextStep = *row.Fields.NextStep
		}
		if row.Fields.Blocked != nil {
			f.Blocked = append([]string(nil), (*row.Fields.Blocked)...)
		}
		editDisplaced := ""
		if f.State == StateClaimed && f.Claimed != nil && !ownPair(f.Claimed, r.Actor) {
			// A hand edit of another pair's claim is lawful under the
			// human, and displacement-bearing (R2-12/R9-05).
			editDisplaced = pairMarker(f.Claimed)
		}
		touchDisplaced(f, r, "edit", []string{row.Id}, editDisplaced)
		path := livePath(row.Id)
		if archived {
			path = donePath(row.Id)
		}
		return []Change{{Path: path, Content: RenderFile(f)}}, nil

	case "detach":
		f, exists := t.Live[row.Id]
		if !exists {
			return nil, conflict("state", "not live on the fetched tip")
		}
		if f.Arc != row.BaseArc {
			return nil, conflict("arc", "the fetched tip carries arc %q, the hand edit was made against %q", f.Arc, row.BaseArc)
		}
		if row.BaseState != "" && f.State != row.BaseState {
			return nil, conflict("state", "is %s on the fetched tip, the hand edit was made against %s", f.State, row.BaseState)
		}
		detachDisplaced := ""
		if f.State == StateClaimed && f.Claimed != nil {
			if !ownPair(f.Claimed, r.Actor) {
				// The released foreign pair hears it (R2-12/R9-05).
				detachDisplaced = pairMarker(f.Claimed)
			}
			f.State = StateQueued
			f.Claimed = nil
		}
		f.Arc = ""
		session.moved[row.Id] = true
		touchDisplaced(f, r, "detach", []string{row.Id}, detachDisplaced)
		return []Change{{Path: livePath(row.Id), Content: RenderFile(f)}}, nil

	case "set-arc":
		f, exists := t.Live[row.Id]
		if !exists {
			return nil, conflict("state", "not live on the fetched tip")
		}
		if f.Arc != row.BaseArc {
			return nil, conflict("arc", "the fetched tip carries arc %q, the hand edit was made against %q", f.Arc, row.BaseArc)
		}
		if row.BaseState != "" && f.State != row.BaseState {
			return nil, conflict("state", "is %s on the fetched tip, the hand edit was made against %s", f.State, row.BaseState)
		}
		// The hand move composes under the SAME membership matrix as
		// the verb, all rows actor H (R2-13: the replay was stricter
		// than the table and refused lawful human joins).
		setArcDisplaced := ""
		handSourceWasClaimed := false
		switch f.State {
		case StateQueued:
		case StateClaimed:
			handSourceWasClaimed = true
			if row.BaseArc == "" {
				return nil, conflict("state", "a claimed standalone goal joins no arc; release first")
			}
			if f.Claimed != nil && !ownPair(f.Claimed, r.Actor) {
				setArcDisplaced = pairMarker(f.Claimed)
			}
			// Released as it detaches — the claim never splits.
			f.State = StateQueued
			f.Claimed = nil
		case StateParked:
			// A parked member moves under the human; the destination
			// decides its landing below.
		default:
			return nil, conflict("state", "arc membership moves live goals; %s is %s", row.Id, f.State)
		}
		var standing *GoalFile
		for _, liveId := range sortedGoalIds(t.Live) {
			m := t.Live[liveId]
			if m.Arc == row.Arc && m.Id != row.Id {
				standing = m
				break
			}
		}
		switch {
		case standing == nil || standing.State == StateQueued:
			if f.State == StateParked {
				return nil, conflict("state", "a parked member joins a queued arc after unpark")
			}
		case standing.State == StateClaimed && standing.Claimed != nil:
			if handSourceWasClaimed {
				// Two displaced pairs in one hand move refuses exactly
				// as the verb refuses it (R2-14): release first.
				return nil, conflict("state", "the member moves from one claimed arc into another; two claimants cannot trade a member in one move — release it first")
			}
			if f.State == StateParked {
				return nil, conflict("state", "a claimed arc admits queued members with done blockers only")
			}
			for _, dep := range f.Blocked {
				if depState(t, dep) != StateDone {
					return nil, conflict("blockedBy", "blocker %s is not done; it cannot join the claimed arc unclaimed-late", dep)
				}
			}
			if !ownPair(standing.Claimed, r.Actor) {
				// A human injection into another pair's claim is
				// displacement-bearing: the runner hears its scope
				// changed (R9-05).
				setArcDisplaced = pairMarker(standing.Claimed)
			}
			f.State = StateClaimed
			f.Claimed = &ClaimRecord{Machine: standing.Claimed.Machine, Lineage: standing.Claimed.Lineage, At: r.stamp()}
		case standing.State == StateParked && standing.Parked != nil:
			// A queued or parked incoming member parks with the arc's
			// record — a human act, which a reconcile always is.
			f.State = StateParked
			f.Parked = &ParkRecord{By: standing.Parked.By, At: standing.Parked.At, Because: standing.Parked.Because}
		}
		f.Arc = row.Arc
		session.moved[row.Id] = true
		touchDisplaced(f, r, "set-arc", []string{row.Id}, setArcDisplaced)
		return []Change{{Path: livePath(row.Id), Content: RenderFile(f)}}, nil
	}
	return nil, fmt.Errorf("reconcile has no application for verb %q", row.Verb)
}

// reconcileIntent serializes every mapped row into the journal's
// durable intent (F7): verb, target, reason, conclusion, cascade
// set, and field deltas — the whole session, rebuildable.
func reconcileIntent(human string, targets []string, rows []MappedVerb) Intent {
	in := Intent{Verb: "reconcile", Targets: targets, Args: map[string]string{
		"by": human, "rows": fmt.Sprintf("%d", len(rows)),
	}}
	for i, row := range rows {
		prefix := fmt.Sprintf("row%d.", i)
		in.Args[prefix+"verb"] = row.Verb
		in.Args[prefix+"id"] = row.Id
		if row.Because != "" {
			in.Args[prefix+"because"] = row.Because
		}
		if row.Conclude != "" {
			in.Args[prefix+"conclude"] = row.Conclude
		}
		if len(row.ArcIds) > 0 {
			in.Args[prefix+"arc"] = strings.Join(row.ArcIds, ",")
		}
		if row.Fields.Intent != nil {
			in.Deltas = append(in.Deltas, FieldDelta{Target: row.Id, Field: "intent", New: *row.Fields.Intent})
		}
		if row.Fields.NextStep != nil {
			in.Deltas = append(in.Deltas, FieldDelta{Target: row.Id, Field: "next", New: *row.Fields.NextStep})
		}
		if row.Origin != "" {
			in.Args[fmt.Sprintf("row%d.origin", i)] = row.Origin
		}
		if row.Fields.Blocked != nil {
			in.Deltas = append(in.Deltas, FieldDelta{Target: row.Id, Field: "blockedBy", New: strings.Join(*row.Fields.Blocked, ",")})
		}
	}
	return in
}

// clearFreeIfDeclared applies the verbs' Goal-free clearing rule to
// a mapped row (F11): an open or unpark under a declared Goal-free
// clears it in the same commit, exactly as the verb does.
func clearFreeIfDeclared(t *TreeGoals, r VerbRequest, target string) []Change {
	if t.Root == nil || t.Root.Free == nil {
		return nil
	}
	t.Root.Free = nil
	t.Root.Revision++
	t.Root.History = append(t.Root.History, HistoryLine{
		At: r.stamp(), Opid: r.opid(), Verb: "reconcile",
		Actor: r.Actor.historyActor(), Targets: []string{target}, Keep: -1,
	})
	return []Change{{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)}}
}
