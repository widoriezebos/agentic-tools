# Design brief: goal-worktree-dispatch-uses-the-primary

## Revision

Revision: revision 2 of `plans/designs/goal-worktree-dispatch-uses-the-primary.md`

Reason: the design critic (Codex on Astra, examination design-critic-27f014429cd290f06ce7fc74, round 1) returned three material findings; the seat accepted all three. The goal record also gained one unit at 2026-10-04 09:50 in Wido's word, and the sandbox faces now belong to a goal of their own. Fold all of it; keep everything else on the page as it is.

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check.

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

The complete prior revision is the page itself, at the path below; read it first and edit it in place.

### The unit the goal record added (verbatim)

"Added 2026-10-04 09:50 CEST by m1e in Wido's word (his item 4, 'fresh sessions lose context'): one more unit, the successor session starts from the goal's record, not from the coordinator's notes. Meaning: the steward's seat-start prompt hands the successor, from the goal's record alone, the goal's next step, the unit list with each unit's state (built, read, handed in, landed, returned with the return text), the open refusals with their job ids, and the last three messages between the seat and the coordinator; a coordinator's hand-written briefing is no longer an input, and a handoff continuation is admitted at the same brief path as the predecessor (face 6)."

What the code does today, read at main c806e15cf:

- `seatBrief` (excerpt) writes the whole seat-start prompt from the goal id and whether the seat already holds it; `SeatSelection` (excerpt) has no other field. The prompt is written to `artifacts/agents/steward/seats/seat-NONCE.brief.md` (`internal/steward/seat_start.go:508-509`) and sent unchanged on stdin to the runtime.
- Readers that already hold each fact: the next step is `goal.GoalFacts.NextStep` (`internal/goal/turnverdict.go:137-146`), which already reaches the seat ladder's `SeatWorldFrom` (`internal/steward/seat_ladder.go:73-81`); the work items and their stages are what `metasystem status G` prints (`runIntentStatusGoal`, `cmd/metasystem/intent_selection.go:123-202`, over `branch.InspectStatus`, `internal/goal/branch/status.go:33-70`), which lives in `cmd/metasystem` and so is out of the steward's reach; handed in, landed and returned with the return text are `plain.Latest(install, goal)` (`internal/landing/plain/queue.go:203`); agent messages are `board.Threads` (`internal/board/mailbox.go:982`), today shown to a running seat only by a hook. No reader returns "open refusals with job ids": launch refusals (`launch.Store.Refusals()`, `internal/launch/record.go:267`) carry code, kind, goal and tag but no job id; the nearest is the goal's open job records under `artifacts/agents/jobs/`.
- A handoff continuation is a different launch: `StageHandoffIntent` (`internal/steward/stage.go:139-190`) starts the `steward-continuation` role with a brief that points at the handoff's `state.json` (`HandoffState`, `internal/steward/handoff_state.go:174-190`, which already holds the next step, open jobs, last landings and the departing seat's lessons note). A fresh seat start carries no handoff binding.
- A coordinator's briefing has no input in the seat prompt; it reaches a seat today as a `plans/handoff-*.md` file the departing seat wrote, or as agent mail delivered by the hook while the seat runs.

The page should add this as one unit with its witness and mutation: what the prompt holds, which reader supplies each fact (and where a reader the steward cannot import is moved or wrapped), what "open refusals with their job ids" means concretely, a bound on the prompt's size, and what the prompt says when a reader fails (the fact is named as unavailable with the command that shows it; the start never fails for it). The last clause, the continuation admitted at the predecessor's brief path, is the page's section 5; say so rather than designing it twice.

### The sandbox goal

The sandbox faces now belong to goal `codex-jobs-run-unsandboxed-on-a-trusted-host` (tier 2, pinned to seat m1e, on the ledger since 2026-10-04 09:55): "On a host the person declares trusted, every Codex job the machinery launches (build, revise, read) runs with --sandbox danger-full-access", one unit, a setting `launch.codex.sandbox` read by the adapter mapping and the unit launcher. Replace the page's reference to item AG of `machinery-blocks-of-2026-10-04` with this goal, in one line.

Critique findings being answered:

1. RULING-goal-worktree-dispatch-uses-the-primary (material, high). Claim: "The proposed skew check contradicts the goal's recorded rule: 'the check compares the engine against main-side commits only (merge-base of the branch with main), never against the goal's own units'. Checking the primary checkout's current tip instead introduces an additional refusal that the design itself acknowledges." Disposition: accepted. Amendment: section 3 lists `STAMP..merge-base(HEAD, main)` in the dispatch root's repository, so the goal's own units are never listed; it states which main ref is read (the local main branch, or the configured landing endpoint's ref; choose and say why) and what happens when there is no merge-base or the stamp is unknown; the paragraph that calls the check stricter than the record goes. Unit 3's witness and mutation follow the new rule.
2. BRIEF-CORRECTION-EXISTING-REVIEW (material, high). Claim: "The corrected-brief contract does not define what happens when this unit already has a review. The existing join path silently ignores new brief bytes; removing the join instead refuses them." Evidence: `cmd/metasystem/intent_unit_review.go:286-290` passes `--join` with the build brief; `internal/goal/branch/read.go:560-563` drops a supplied brief when a critic exists, and lines 577-579 otherwise refuse a changed brief (excerpts). Disposition: accepted. Amendment: section 4 states the outcome. `work review G --work W --brief FILE` never joins. When the unit's read has no critic yet, or its start was refused (the case of face 5, a dispatch refused for its brief, which leaves a retryable record with no critic), FILE starts the read, as the existing restart rule already allows. When a critic already examines or examined the unit with another brief, the request is refused, naming that examination (the existing `ReadBriefChangedCode`). An old examination never covers a correction. The unit-4 witness gains both rows.
3. MOVED-EFFECTS-MISSING-INVENTORY (material, medium). Claim: "The design moves responsibilities without the required Moved effects table. It centralizes engine selection, deletes the roster-carriage owner, transfers brief admission responsibility, and moves continuation refusal ahead of handoff consumption." The critic named the current owners `cmd/metasystem/delegate.go:314`, `internal/delegation/owners.go:65`, `internal/adapter/supervisor/deps.go:102`, `cmd/metasystem/goal_branch.go:678`, `internal/delegation/admission.go:611` and `internal/steward/revive.go:196-238`. Disposition: accepted. Amendment: add a section headed exactly `## Moved effects` with a table `| Effect | From | To | Code |`, one row per moved effect (each engine default, the roster carriage, each settings reader that moves, the frozen brief's admission, the continuation check before consumption, and the new unit's prompt facts if a reader moves), the Code column naming every file the effect reaches with its `metasystem/` prefix, as the accepted page `plans/designs/seat-path-lands-without-help.md` does. A new symbol is named with the existing file that will hold it.

Cited code excerpts:

1. `internal/steward/seat_start.go:442-457`

   ```text
   // seatBrief is what the seat main reads on stdin.
   func seatBrief(selection SeatSelection) string {
   	why := "it is approved and ready"
   	if selection.Held {
   		why = "this seat already holds it; the main before you ended"
   	}
   	return fmt.Sprintf(`# Seat session

   You are this seat's session: `+"`metasystem goal claim %s`"+` takes or continues this work.
   Work it and land it; stop when nothing is claimable.

   The steward started you for goal %s: %s.

   Nobody sits at this terminal: your process ends when your turn ends, and every background job you started ends with it. Never end a turn to wait for a job, a critique, a test run or a reply; wait inside the turn with `+"`metasystem work wait`"+` (bounded by --timeout) and carry on. End your turn only when the goal is handed in to land, it is blocked on a person's answer you asked with `+"`metasystem question ask`"+`, or nothing is claimable.
   `, selection.Goal, selection.Goal, why)
   }
   ```

2. `internal/steward/seat_ladder.go:54-61`

   ```text
   // SeatSelection is the goal a seat start names and what its record keeps.
   type SeatSelection struct {
   	Goal         string   `json:"goal"`
   	Held         bool     `json:"held"`
   	ApprovalOpid string   `json:"approvalOpid"`
   	Ready        []string `json:"ready"`
   	SeatHeld     []string `json:"seatHeld"`
   }
   ```

3. `cmd/metasystem/intent_unit_review.go:286-290`

   ```text
   	args := []string{"--root", install, "--goal", goalID, "--unit", subject.Commit}
   	if review.BuildBrief != "" {
   		// The work's own brief starts its read; a read the commit form
   		// already started for this build is joined.
   		args = append(args, "--brief", review.BuildBrief, "--join")
   ```

4. `internal/goal/branch/read.go:556-580`

   ```text
   	supplied, inputSHA256, err := branchReadInput(request.BriefPath)
   	if err != nil {
   		return result, err
   	}
   	if request.Join && record.RootJob != "" {
   		// A read is of one build: the request joins the one already started
   		// and its brief starts nothing.
   		supplied, inputSHA256 = nil, ""
   	}
   	if record.DispatchPending && record.RootJob == "" {
   		// An earlier dispatch never reported back. The job records say
   		// whether it reserved a critic.
   		if record, err = settleBranchReadDispatch(request, common, recordPath, record); err != nil {
   			return result, err
   		}
   	}
   	// A request binds only what was dispatched: a critic root. A start the
   	// dispatch refused (retryable, no root) started nothing, so a request
   	// naming a brief, runtime or model starts over with them; a request
   	// naming none repeats the saved start.
   	restart := record.RootJob == "" && record.DispatchRetryable && (inputSHA256 != "" || request.Runtime != "" || request.Model != "")
   	if !restart && record.RootJob != "" && (inputSHA256 != "" && inputSHA256 != record.BriefInputSHA256 ||
   		request.Runtime != "" && request.Runtime != record.Runtime || request.Model != "" && request.Model != record.Model) {
   		return result, operationRefusal(ReadBriefChangedCode, "this build's review already started with another brief, runtime or model\nrun: metasystem work review %s", request.GoalID)
   	}
   ```

5. `internal/delegation/infra.go:45-82`

   ```text
   // engineSkewPreflight refuses a dispatch whose engine is older than the
   // checkout when engine or agent sources changed since the engine's commit.
   // The stamp is the running engine's build stamp unless a focused fixture
   // supplies one.
   func (s *session) engineSkewPreflight(stamp string) error {
   	if stamp == "" {
   		stamp = supervise.BuildStamp
   	}
   	if stamp == "" || stamp == "dev" {
   		return nil
   	}
   	out, _, err := s.l.ports.Git.Run(s.ctx, s.repoScope, "log", "--format=commit %H", "--name-only", "--ancestry-path", stamp+"..HEAD")
   	if err != nil {
   		return nil
   	}
   	text := string(out)
   	checkoutCommit := text
   	if index := strings.IndexByte(text, '\n'); index >= 0 {
   		checkoutCommit = text[:index]
   	}
   	checkoutCommit = strings.TrimPrefix(checkoutCommit, "commit ")
   	prefix := ""
   	if s.root != s.repoScope {
   		prefix = strings.TrimPrefix(s.root, s.repoScope+"/") + "/"
   	}
   	relevant := false
   	for _, line := range strings.Split(text, "\n") {
   		for _, tree := range steward.EngineSkewPathspecs() {
   			if strings.HasPrefix(line, prefix+tree+"/") {
   				relevant = true
   			}
   		}
   	}
   	if checkoutCommit != "" && relevant {
   		return s.die(1, fmt.Sprintf("dispatch refused: the engine (%s) is older than this checkout (%s), and engine scripts changed\nrebuild with go run ./cmd/devgate build, then arm the steward again", stamp, checkoutCommit))
   	}
   	return nil
   }
   ```

Example page:

`/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/seat-path-lands-without-help.md`, its `## Moved effects` section only, for the table's shape.

## Recurring findings

none recorded

## Tool-call budget

Maximum delegate tool calls: 60

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: 2000 words

Cut a draft that exceeds this ceiling. If cutting would make the page
incomplete, stop and propose a split instead. Keep the prose of sections 1 to 5 as short as it is; the table and the new unit are the growth.

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
