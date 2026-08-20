package goal

// The verb surface (BGS-9/14), first slice: open, claim, release,
// done — the single-file rows of the transition table. Every verb
// is one transaction: its mutation callback re-reads the fetched
// tip and re-decides on the current world (a rebuilt tip classifies
// idempotent success, loss, or refusal by name), and its write set
// touches NO path outside the table's row. Common effects, applied
// once here: every touched file's Revision increments by exactly
// one and its History gains exactly the opid's line with the verb
// and actor. Arc cascades land with the arcs layer (BGS-15).

import (
	"fmt"
	"strings"
	"time"
)

// Actor is the executing identity: the machine+lineage pair always;
// Human names the directing human when one did (authority vs
// execution — the opid attributes execution, the History actor
// attributes authority).
type Actor struct {
	Machine string
	Lineage string
	Human   string // empty for agent-directed verbs
}

func (a Actor) historyActor() string {
	if a.Human != "" {
		return "human:" + a.Human
	}
	return a.Machine + "+" + a.Lineage
}

// VerbRequest carries what every verb needs.
type VerbRequest struct {
	Endpoint Endpoint
	Actor    Actor
	Ulid     string // caller-minted; the opid derives from it
	Now      time.Time
}

func (r VerbRequest) opid() string {
	return Opid(r.Ulid, r.Actor.Machine, r.Actor.Lineage)
}

func (r VerbRequest) stamp() string {
	return r.Now.UTC().Format(time.RFC3339)
}

// loadTree parses one tip's whole ledger subtree; a tree that does
// not parse refuses the verb by name (the verb never writes onto a
// world it cannot read).
func loadTree(root, tip string) (*TreeGoals, error) {
	files, err := ReadCommitGoals(root, tip)
	if err != nil {
		return nil, err
	}
	t, problems := ParseTreeFiles(files)
	if len(problems) > 0 {
		lines := make([]string, len(problems))
		for i, p := range problems {
			lines[i] = string(p)
		}
		return nil, fmt.Errorf("the ledger tree at %s does not parse:\n%s", short(tip), strings.Join(lines, "\n"))
	}
	return t, nil
}

// touch applies the common effects to one goal file: Revision +1,
// History + the opid's line.
func touch(f *GoalFile, r VerbRequest, verb string, targets []string) {
	f.Revision++
	f.History = append(f.History, HistoryLine{
		At: r.stamp(), Opid: r.opid(), Verb: verb,
		Actor: r.Actor.historyActor(), Targets: targets, Keep: -1,
	})
}

// opidLanded reports whether this operation's opid is already in
// the file's History — the idempotent-success half of the
// postcondition, per touched file.
func opidLanded(f *GoalFile, r VerbRequest) bool {
	for _, h := range f.History {
		if h.Opid == r.opid() {
			return true
		}
	}
	return false
}

// livePath and donePath name a goal's two possible homes.
func livePath(id string) string { return goalsPrefix + id + ".md" }
func donePath(id string) string { return goalsPrefix + "done/" + id + ".md" }

// Open adds a queued goal. Goal-free clears in the same commit when
// it was declared.
func Open(r VerbRequest, id, intent, origin, nextStep string) (PublishResult, error) {
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "open", Targets: []string{id}, Args: map[string]string{
			"intent": intent, "origin": origin, "next": nextStep,
		}},
		Message: "goal open " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if f, exists := t.Live[id]; exists {
				if opidLanded(f, r) {
					return nil, AlreadyApplied{}
				}
				return nil, LostToCompetitor{Winner: lastOpid(f)}
			}
			if _, archived := t.Done[id]; archived {
				return nil, fmt.Errorf("goal %s is in the archive; reopen is the explicit exception", id)
			}
			f := &GoalFile{
				Id: id, State: StateQueued, Intent: intent, Origin: origin,
				NextStep: nextStep, OpenedAt: r.stamp(), Revision: 0,
			}
			touch(f, r, "open", []string{id})
			changes := []Change{{Path: livePath(id), Content: RenderFile(f)}}
			// Opening clears a declared Goal-free in the same commit.
			if t.Root != nil && t.Root.Free != nil {
				t.Root.Free = nil
				t.Root.Revision++
				t.Root.History = append(t.Root.History, HistoryLine{
					At: r.stamp(), Opid: r.opid(), Verb: "open",
					Actor: r.Actor.historyActor(), Targets: []string{id}, Keep: -1,
				})
				changes = append(changes, Change{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)})
			}
			return changes, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
}

// Claim takes ownership of a queued goal for the actor's machine.
func Claim(r VerbRequest, id string) (PublishResult, error) {
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "claim", Targets: []string{id}},
		Message: "goal claim " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live; nothing to claim", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.State == StateClaimed {
				if f.Claimed != nil && f.Claimed.Machine == r.Actor.Machine {
					return nil, AlreadyApplied{}
				}
				return nil, LostToCompetitor{Winner: lastOpid(f)}
			}
			if f.State != StateQueued {
				return nil, fmt.Errorf("goal %s is %s; only a queued goal claims (park and done have their own verbs)", id, f.State)
			}
			for _, dep := range f.Blocked {
				if depState(t, dep) != StateDone {
					return nil, fmt.Errorf("goal %s is blocked by %s, which is not done", id, dep)
				}
			}
			f.State = StateClaimed
			f.Claimed = &ClaimRecord{Machine: r.Actor.Machine, Lineage: r.Actor.Lineage, At: r.stamp()}
			touch(f, r, "claim", []string{id})
			return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
}

// Release returns the actor's claimed goal to the queue.
func Release(r VerbRequest, id string) (PublishResult, error) {
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "release", Targets: []string{id}},
		Message: "goal release " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live; nothing to release", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.State != StateClaimed || f.Claimed == nil {
				return nil, fmt.Errorf("goal %s is %s, not claimed", id, f.State)
			}
			if f.Claimed.Machine != r.Actor.Machine && r.Actor.Human == "" {
				return nil, fmt.Errorf("goal %s is claimed by %s; a foreign release is a human act (steal has its own verb)", id, f.Claimed.Machine)
			}
			f.State = StateQueued
			f.Claimed = nil
			touch(f, r, "release", []string{id})
			return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
}

// Done concludes one goal and moves it to the archive — the one
// member only; sibling arc members stay untouched.
func Done(r VerbRequest, id, conclusion string) (PublishResult, error) {
	if strings.TrimSpace(conclusion) == "" {
		return PublishResult{}, fmt.Errorf("done needs its conclusion — the archive is the record")
	}
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "done", Targets: []string{id}, Args: map[string]string{
			"conclusion": conclusion,
		}},
		Message: "goal done " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if f, archived := t.Done[id]; archived {
				if opidLanded(f, r) {
					return nil, AlreadyApplied{}
				}
				return nil, LostToCompetitor{Winner: lastOpid(f)}
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live; nothing to conclude", id)
			}
			// Queued concludes directly (D113 ergonomics); a foreign
			// claim concludes only under a human.
			if f.State == StateClaimed && f.Claimed != nil &&
				f.Claimed.Machine != r.Actor.Machine && r.Actor.Human == "" {
				return nil, fmt.Errorf("goal %s is claimed by %s; concluding another's work is a human act", id, f.Claimed.Machine)
			}
			if f.State == StateParked && r.Actor.Human == "" {
				return nil, fmt.Errorf("goal %s is parked; concluding it is a human act", id)
			}
			for _, dep := range f.Blocked {
				if depState(t, dep) != StateDone {
					return nil, fmt.Errorf("goal %s is blocked by %s, which is not done", id, dep)
				}
			}
			f.State = StateDone
			f.Conclude = conclusion
			f.Claimed = nil
			f.Parked = nil
			touch(f, r, "done", []string{id})
			return []Change{
				{Path: livePath(id), Delete: true},
				{Path: donePath(id), Content: RenderFile(f)},
			}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
}

func depState(t *TreeGoals, id string) string {
	if f, ok := t.Live[id]; ok {
		return f.State
	}
	if _, ok := t.Done[id]; ok {
		return StateDone
	}
	return ""
}

// lastOpid names the most recent operation on a file — the winner a
// losing competitor reports.
func lastOpid(f *GoalFile) string {
	if len(f.History) == 0 {
		return "unknown"
	}
	return f.History[len(f.History)-1].Opid
}

// Park pauses a goal with its reason. Parking another machine's
// claim is a human act, and the displaced claimant is recorded —
// displacement is a stop signal the serving machine hears (BGS-8's
// notification legs land with the projection). Arc cascades land
// with the arcs layer.
func Park(r VerbRequest, id, because string) (PublishResult, error) {
	if strings.TrimSpace(because) == "" {
		return PublishResult{}, fmt.Errorf("park needs its reason — a pause without a why is a stall in disguise")
	}
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "park", Targets: []string{id}, Args: map[string]string{
			"because": because,
		}},
		Message: "goal park " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live; nothing to park", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.State == StateParked {
				return nil, LostToCompetitor{Winner: lastOpid(f)}
			}
			if f.State != StateQueued && f.State != StateClaimed {
				return nil, fmt.Errorf("goal %s is %s; only queued or claimed goals park", id, f.State)
			}
			displaced := ""
			if f.State == StateClaimed && f.Claimed != nil {
				if f.Claimed.Machine != r.Actor.Machine && r.Actor.Human == "" {
					return nil, fmt.Errorf("goal %s is claimed by %s; parking another's claim is a human act", id, f.Claimed.Machine)
				}
				if f.Claimed.Machine != r.Actor.Machine {
					displaced = f.Claimed.Machine + "+" + f.Claimed.Lineage + "@" + f.Claimed.At
				}
			}
			f.State = StateParked
			f.Parked = &ParkRecord{
				By: r.Actor.historyActor(), At: r.stamp(),
				Because: because, Displaced: displaced,
			}
			f.Claimed = nil
			f.Revision++
			f.History = append(f.History, HistoryLine{
				At: r.stamp(), Opid: r.opid(), Verb: "park",
				Actor: r.Actor.historyActor(), Targets: []string{id},
				Displaced: displaced, Keep: -1, Reason: because,
			})
			return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
}

// Unpark returns a parked goal to the queue. The park's records
// stay in the history; Goal-free clears when it was declared.
func Unpark(r VerbRequest, id string) (PublishResult, error) {
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "unpark", Targets: []string{id}},
		Message: "goal unpark " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live; nothing to unpark", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.State != StateParked {
				return nil, fmt.Errorf("goal %s is %s, not parked", id, f.State)
			}
			f.State = StateQueued
			f.Parked = nil
			touch(f, r, "unpark", []string{id})
			changes := []Change{{Path: livePath(id), Content: RenderFile(f)}}
			if t.Root != nil && t.Root.Free != nil {
				t.Root.Free = nil
				t.Root.Revision++
				t.Root.History = append(t.Root.History, HistoryLine{
					At: r.stamp(), Opid: r.opid(), Verb: "unpark",
					Actor: r.Actor.historyActor(), Targets: []string{id}, Keep: -1,
				})
				changes = append(changes, Change{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)})
			}
			return changes, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
}

// Reopen is done's explicit exception: the archived file moves back
// to the live set as queued. Goal-free clears when it was declared.
func Reopen(r VerbRequest, id string) (PublishResult, error) {
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "reopen", Targets: []string{id}},
		Message: "goal reopen " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if f, live := t.Live[id]; live {
				if opidLanded(f, r) {
					return nil, AlreadyApplied{}
				}
				return nil, LostToCompetitor{Winner: lastOpid(f)}
			}
			f, archived := t.Done[id]
			if !archived {
				return nil, fmt.Errorf("goal %s is not in the archive; reopen moves archived goals back", id)
			}
			// Reopening under claimed dependents is the transition
			// closure's refusal: a claimed goal's blockers must stay
			// done (BGS-9).
			for _, liveId := range sortedGoalIds(t.Live) {
				dependent := t.Live[liveId]
				if dependent.State != StateClaimed {
					continue
				}
				for _, dep := range dependent.Blocked {
					if dep == id {
						return nil, fmt.Errorf("goal %s cannot reopen: %s is claimed and depends on it staying done", id, liveId)
					}
				}
			}
			f.State = StateQueued
			f.Conclude = ""
			touch(f, r, "reopen", []string{id})
			changes := []Change{
				{Path: donePath(id), Delete: true},
				{Path: livePath(id), Content: RenderFile(f)},
			}
			if t.Root != nil && t.Root.Free != nil {
				t.Root.Free = nil
				t.Root.Revision++
				t.Root.History = append(t.Root.History, HistoryLine{
					At: r.stamp(), Opid: r.opid(), Verb: "reopen",
					Actor: r.Actor.historyActor(), Targets: []string{id}, Keep: -1,
				})
				changes = append(changes, Change{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)})
			}
			return changes, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
}

// EditFields is the edit verb's delta set: nil-able fields change
// only when set. Prose caps are REMOVED by design (D113/D114) — a
// multi-kilobyte intent is lawful.
type EditFields struct {
	Intent   *string
	NextStep *string
	Origin   *string
	Blocked  *[]string
}

// Edit applies field deltas to one live goal.
func Edit(r VerbRequest, id string, fields EditFields) (PublishResult, error) {
	deltas := []FieldDelta{}
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "edit", Targets: []string{id}, Deltas: deltas},
		Message: "goal edit " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f, exists := t.Live[id]
			if !exists {
				return nil, fmt.Errorf("goal %s is not live; the archive edits through reopen", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if fields.Intent != nil {
				f.Intent = *fields.Intent
			}
			if fields.NextStep != nil {
				f.NextStep = *fields.NextStep
			}
			if fields.Origin != nil {
				f.Origin = *fields.Origin
			}
			if fields.Blocked != nil {
				f.Blocked = append([]string(nil), (*fields.Blocked)...)
			}
			touch(f, r, "edit", []string{id})
			return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
}

// DeclareFree declares the absence of intent: no queued or claimed
// goals may exist, parked coexistence is lawful, renewal is
// idempotent.
func DeclareFree(r VerbRequest, origin, digest string) (PublishResult, error) {
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "declare-free", Args: map[string]string{
			"origin": origin, "digest": digest,
		}},
		Message: "goal declare-free",
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if t.Root == nil {
				return nil, fmt.Errorf("no root record; the ledger is not adopted")
			}
			for _, id := range sortedGoalIds(t.Live) {
				switch t.Live[id].State {
				case StateQueued, StateClaimed:
					return nil, fmt.Errorf("goal %s is %s; Goal-free declares over parked and done only", id, t.Live[id].State)
				}
			}
			if t.Root.Free != nil && t.Root.Free.Digest == digest {
				return nil, AlreadyApplied{}
			}
			t.Root.Free = &FreeRecord{Declared: r.stamp(), Origin: origin, Digest: digest}
			t.Root.Revision++
			t.Root.History = append(t.Root.History, HistoryLine{
				At: r.stamp(), Opid: r.opid(), Verb: "declare-free",
				Actor: r.Actor.historyActor(), Keep: -1,
			})
			return []Change{{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	})
}
