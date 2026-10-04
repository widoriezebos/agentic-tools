# Design brief: goal-worktree-dispatch-uses-the-primary

## Revision

Revision: revision 3 of `plans/designs/goal-worktree-dispatch-uses-the-primary.md`

Reason: round 2 of the design critique (Codex on Astra, design-critic-27f014429cd290f06ce7fc74) returned four material findings, all in section 6, the seat-start prompt; the seat accepted all four. Sections 1 to 5, 8 and 9 drew no finding: keep them as they are. Change section 6, its Moved effects rows, unit 6 and the not-checked list only.

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check.

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

The complete prior revision is the page itself, at the path below; read it first and edit it in place. The goal record's words for the unit: "the steward's seat-start prompt hands the successor, from the goal's record alone, the goal's next step, the unit list with each unit's state (built, read, handed in, landed, returned with the return text), the open refusals with their job ids, and the last three messages between the seat and the coordinator".

Critique findings being answered:

1. RULING-goal-worktree-dispatch-uses-the-primary (material, high). Claim: "The new message filter contradicts the goal's requirement to provide 'the last three messages between the seat and the coordinator.' Selecting only messages addressed to the goal or belonging to its threads drops an ordinary direct conversation between those participants." Evidence: `internal/board/mailbox.go:139` defines seat and goal addresses as alternatives; `cmd/metasystem/intent_agent.go:370` handles direct seat messages. Disposition: accepted. Amendment: the prompt takes the newest three messages to or from this seat, seat-addressed or goal-addressed, with their thread replies, from the mailbox the inbox reads; the witness has a direct seat message.
2. SEAT-HANDIN-LANDED-STATE (material, medium). Claim: "Latest returns the recorded waiting state even after the commit reaches main, so a successor is told completed work is still handed in." Evidence: `internal/landing/plain/queue.go:203` calls Entries without deriving landing; `Landed` performs that derivation (excerpt); `cmd/metasystem/landing_plain.go` does it in `laneQueueState`. Disposition: accepted. Amendment: the hand-in fact is `plain.Latest` passed through `plain.Landed` against the main the start reads, as `laneQueueState` does; the witness moves one line from waiting to landed.
3. SEAT-REFUSALS-BEFORE-RESERVATION (material, medium). Claim: "The proposed refusal sources omit a review rejected before job reservation, including a corrected brief that cites a missing path. Such a refusal has neither the failed job record the design scans nor an entry in the separate launch-refusal store." Evidence: brief admission runs at `internal/delegation/dispatch_phase.go:383`, before reservation; `internal/goal/branch/read.go:78` records pending and retryable state but no refusal explanation (excerpt). Disposition: accepted. Amendment: the smallest durable source: when a review dispatch is refused before reservation, the goal branch read record keeps the refusal's first line beside its retryable state, and the prompt lists that unit as refused with that line and the command that repeats the review. No new store.
4. SEAT-LAUNCH-REFUSAL-CLOSURE (material, medium). Claim: "The design lists historical launch refusals as open without defining when they close. After an operator fixes a refused launch and successfully retries it, the old refusal remains in the selected history." Evidence: `internal/launch/record.go:242` appends refusal history, and `Refusals` (excerpt) returns every row. Disposition: accepted. Amendment: a launch refusal is open until a later launch record of the same goal and kind exists; the witness has a refused-then-launched pair that lists nothing, and the not-checked line about closure goes.

Cited code excerpts:

1. `internal/landing/plain/queue.go:203-210`

   ```text
   func Latest(install, goal string) (Entry, bool, error) {
   	entries, err := Entries(install)
   	if err != nil {
   		return Entry{}, false, err
   	}
   	for index := len(entries) - 1; index >= 0; index-- {
   		if entries[index].Goal == goal {
   			return entries[index], true, nil
   ```

2. `internal/goal/branch/read.go:70-90`

   ```text

   type ReadGateRequest struct {
   	Repo, GoalID, UnitCommit string
   	Gate                     func(string) (string, error)
   	NewID                    func(string) (string, error)
   	Repository               BranchReadRepository
   }

   type branchReadRecord struct {
   	SchemaVersion     int    `json:"schemaVersion"`
   	Goal              string `json:"goal"`
   	UnitCommit        string `json:"unitCommit"`
   	Tree              string `json:"tree"`
   	GateRunID         string `json:"gateRunId,omitempty"`
   	RootJob           string `json:"rootJob,omitempty"`
   	Brief             string `json:"brief,omitempty"`
   	BriefInputSHA256  string `json:"briefInputSha256,omitempty"`
   	Runtime           string `json:"runtime,omitempty"`
   	Model             string `json:"model,omitempty"`
   	DispatchPending   bool   `json:"dispatchPending,omitempty"`
   	DispatchRetryable bool   `json:"dispatchRetryable,omitempty"`
   ```

3. `internal/launch/record.go:264-280`

   ```text
   	_, err = atomicfile.WriteText(path, string(content)+string(row)+"\n", filepath.Dir(root))
   	return err
   }
   func (s Store) Refusals() ([]Refusal, error) {
   	root, err := s.root()
   	if err != nil {
   		return nil, err
   	}
   	data, err := os.ReadFile(filepath.Join(root, "refusals.jsonl"))
   	if os.IsNotExist(err) {
   		return nil, nil
   	}
   	if err != nil {
   		return nil, err
   	}
   	var result []Refusal
   	for number, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
   ```

4. `internal/landing/plain/queue.go:314-333`

   ```text

   // Landed derives each waiting entry's landing: one whose sha main
   // contains is landed. contains reads it; an error leaves the entry
   // waiting and is returned with the rest still derived.
   func Landed(entries []Entry, contains func(sha string) (bool, error)) ([]Entry, error) {
   	out := make([]Entry, 0, len(entries))
   	var problems []error
   	for _, entry := range entries {
   		if entry.State == StateWaiting {
   			inside, err := contains(entry.SHA)
   			if err != nil {
   				problems = append(problems, fmt.Errorf("%s: %w", entry.Goal, err))
   			} else if inside {
   				entry.State = StateLanded
   			}
   		}
   		out = append(out, entry)
   	}
   	return out, errors.Join(problems...)
   }
   ```

Example page:

The page itself: keep its voice, its section order and its unit format.

## Recurring findings

none recorded

## Tool-call budget

Maximum delegate tool calls: 40

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: 2200 words

Cut a draft that exceeds this ceiling. If cutting would make the page
incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique
only through the context pack above. Never resume a delegate from an earlier
revision.

## Page artifact and return shape

Write the page to: /Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/goal-worktree-dispatch-uses-the-primary.md

Keep its head (title, Kind, Id, Status: draft, Goals) exactly as it is. Do not edit any other file, run test suites, build engines, claim or release goals, or commit.

Return only these two lines, N being the page's word count:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/goal-worktree-dispatch-uses-the-primary.md
DESIGN: ready (N words)
```

or:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/goal-worktree-dispatch-uses-the-primary.md
DESIGN: blocked (the reason in one line)
```

When blocked, stop and return `BLOCKED:` with the refusal's first line, what you tried and the decision needed; your seat asks.
