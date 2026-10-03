# Design brief: seat-path-lands-without-help

## Revision

Revision: revision 3 of the page `/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/seat-path-lands-without-help.md`

Reason: round 1 of the design critique (Codex on Astra, job design-critic-46437605ef0c580fea7461d1, reviewed at eee6cc25a) returned five material findings. The seat accepted all five; this revision answers them with the amendments below, and folds in seven facts the seat re-read while deciding them. Nothing else changes.

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check.

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

### The prior revision

Revision 2 is the page on disk at `/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/seat-path-lands-without-help.md` (2,945 words, draft, on main at eee6cc25a). Read it whole, once, before writing. It is not copied into this brief. Its cites were read at f85f27efe; the excerpts below are at eee6cc25a, and where a line number on the page has moved, use the one below.

Every section stands except where an amendment below changes it. Keep the head exactly (title, Kind, Id `01M3Z6EXJH4CCXWDNZA9DWB8X2`, Status draft, Goals). The line "Revision 2: ..." becomes revision 3, with one sentence saying it answers critique round 1.

The threat model is unchanged: our own agents and operators make mistakes and crash; nobody attacks. Fixes are by subtraction: reuse an existing act or remove a rule; a new mechanism needs a reason no existing one covers.

### The accepted amendments (the seat's decisions, verbatim)

1. RULING-R-13 (Unit 1b, section 2b): "The mark covers a wait, never the goal's next work. (a) While a goal is marked, its own work admission keeps the elapsed limit: `admissionBreachesFor` no longer drops elapsed for the goal's own admission, so later work is refused at the limit with the structured breach; the breach stop (`stopReasonFor`) and the machine-wide check of the other claims still skip a marked claim's elapsed, so a waiting goal is never stopped. Say whether the landing's own proof keeps an exemption and why. (b) Each wait starts at its own hand-in: a hand-in that appends a new queue line while the goal is marked from an earlier one moves the mark to the hand-in's time, and the span before it stays charged, so `LandReturn` credits only the latest wait. Say the act that moves it and what it does to the landing gate's clock and verdicts (`ReadGate` and `VerdictsOf` read from the standing `Landing`). (c) A `work land G` that finds the goal's newest hand-in returned, at any commit, runs `LandReturn` before anything else, then goes on (a new commit is handed in again). (d) Every `work land G` that ends with the goal waiting in the lane leaves it marked, a repeat at the same commit included. State the remaining imprecision on the page: the credit ends at the seat's act, so work the seat did on the same goal between the return and that act is credited; it is bounded by one lane wait."

2. RULING-R-129-ui (section 2, the loop): "With no lane, the landed check comes before `handLandingSubject`, right after the branch read, in the same place the lane route asks `laneQueueState`: when main contains the selected commit (the branch tip, or the `--through` commit) `work land G` says landed and offers `goal done`. The `SaysLanded` witness reads a branch whose tip main already contains, so the unit list is empty, not stubbed."

3. SEAT-CONFLICT-RECOVERY (the conflict row, U3's witness): "The conflict row's second line is the goal's normal correction, the same route a lane return takes: `metasystem work revise G --brief FILE`, the brief naming the conflicting paths. A follow-up rebases the goal's work onto main when main touched its paths (`internal/dispatch/followup_rebase.go`); then `work review G`, then `work land G`. Never a merge of main into the goal branch. The page checks that this route really rebases onto the new main and says so with its citation. The conflict witness adds the recovery: after the branch is rebased onto the moved main (one parent per commit), the next `work land G` merges cleanly."

4. SEAT-DEAD-PROOF-WAIT (the proving row, "How the seat learns the proof ended"): "The seat's loop has one command. In the proving state `work land G` itself waits, bounded (a few minutes, said on the page), until `running.json` is absent or its process is dead (`plain.ReadRunning`), then reads the result once: green pushes, red refuses, dead restarts the proof. Still proving at the bound: line 1 says so, line 2 is this command. No `work wait --path` in the table. Witness: a dead proof's `running.json` makes the next `work land G` start the proof again."

5. MOVED-EFFECTS-SEAT-LANDING (a new section): "Add a 'Moved effects' section with the four-column table the validator reads (Effect, From, To, Code), one row per moved owner, Code paths starting `metasystem/` and present at the tip: the merge onto main, the proof, the push, the landed record (derived from main), the branch sweep and the release set (say the verb that does them once the landing no longer does; the page's section 9 leaves this unchecked, so check it), the board card at landing, and the delivered sentence. The sentence's owner: with no lane the seat keeps the lane's own hand-in line in its records folder (`plain.HandIn`, and `plain.Say` on a repeat), so the sentence given at the first run is the one the pushing run posts."

The section must be headed exactly `## Moved effects` and hold a table whose header row is `| Effect | From | To | Code |`; every path in a Code cell starts with `metasystem/` and exists at the tip (`metasystem design review FILE --check-only` checks this). Name only paths that exist today; a new symbol goes in the To cell with the existing file that will hold it.

### Facts the seat re-read for Unit 1b (at eee6cc25a)

1. A return is read only at the returned commit: `laneQueueState` returns nil when the newest queue entry's commit differs from the one asked (excerpt 2). A seat that pushed its fix before repeating `work land G` never reads "returned" today. Amendment 1(c) answers this.
2. A repeat of `work land G` at the handed-in commit returns from `laneQueueState` before `handIn` (excerpt 3), so a mark placed only inside `handIn` is skipped for a goal handed in before the change. Amendment 1(d) answers this.
3. The exemption is in `stopReasonFor` and `admissionBreachesFor` (excerpt 1), not in `FindBreachStops`, which calls `stopReasonFor` (`internal/dispatch/stop.go:442`). The page's witness `TestTwoGoalsOfOneMachineWaitToLand` still reads `FindBreachStops`.
4. A second elapsed computation, `LandingOverdue` (`internal/goal/turnverdict.go:1757-1775`), prints a marked claim past its box as overdue; it already subtracts `IdleSeconds`. Say in one line whether it needs a change (the seat's reading: no).
5. The landing gate starts its grace clock at `Landing.At` (excerpt 5), and `VerdictsOf` reads verdicts only since the standing `Landing` (excerpt 6): "a later land-ready puts the goal back in Review with no verdict on it". Amendment 1(b) moves the mark; say what that does here.
6. Every hand-in now marks, so a goal waiting in the lane past the gate's grace is a due landing on the holder's Stop (`LandingDue`, `internal/goal/landgate.go:345`): the Stop runs `work land G --json` for it (`cmd/metasystem/holder_step.go:27-44`) and counts any answer that is not refused or failed as landed (excerpt 7), so a waiting goal prints "LANDED G: goal G at SHA is waiting in the landing lane". Say what that line reads instead (the smallest change), and whether the Stop's run reading a return and running `LandReturn` there is wanted (the Stop runs under the holder's own session). Put it in U1b.1 or U1b.2.
7. The dead-proof liveness read exists: `plain.ReadRunning` returns whether a proof is recorded and whether its process runs (excerpt 8). The delivered sentence is posted from this command's `--delivered`, else the one recorded with the landing (excerpt 9).

### Size

The units table keeps its 300-line cap per unit except U4. If an amendment pushes a unit past it, split the unit and say which part lands first. U1b.1 and U1b.2 still wait on no question and land ahead of U1.

Critique findings being answered:

1. RULING-R-13 (high): automatic marking suspends the elapsed budget for later implementation, not just the lane wait; after the first unit lands, work on later units stays exempt.
2. RULING-R-129-ui (high): a repeat after a no-lane landing is refused by `handLandingSubject` ("no committed work to land") before the loop's landed case.
3. SEAT-CONFLICT-RECOVERY (high): merging origin/main into goal/G makes a two-parent commit, which the branch reader refuses.
4. SEAT-DEAD-PROOF-WAIT (high): a dead proof leaves `running.json`, so the printed presence wait never ends and never reaches the `work land G` that would restart it.
5. MOVED-EFFECTS-SEAT-LANDING (medium): the Moved effects inventory is absent, and the delivered sentence given before the background proof is lost at the push.

Cited code excerpts:

1. `internal/dispatch/admission.go:496-530`

   ```text
   // stopReasonFor is the live-stop reason for one claim. A claim waiting to
   // land (goal land-ready) has its elapsed fence suspended: the wait is
   // printed, never stopped; attempts, minutes and active jobs still bind, so
   // a corrupt-over-limit stop still fences it.
   func stopReasonFor(file *goal.GoalFile, projection BudgetProjection) string {
   	if !file.IsLandingClaim() {
   		return liveStopReason(projection)
   	}
   	return landingStopReason(projection)
   }

   // landingStopReason is the stop reason of a claim waiting to land: every
   // breach but the elapsed one.
   func landingStopReason(projection BudgetProjection) string {
   	for _, breach := range projection.Breaches {
   		if breach.Field != "elapsedLimit" {
   			return goal.StopReasonCorruptOverLimit
   		}
   	}
   	return ""
   }

   // admissionBreachesFor drops the elapsed dimension for a claim waiting to
   // land; every other breach stands.
   func admissionBreachesFor(file *goal.GoalFile, breaches []BudgetBreach) []BudgetBreach {
   	if !file.IsLandingClaim() {
   		return breaches
   	}
   	kept := make([]BudgetBreach, 0, len(breaches))
   	for _, breach := range breaches {
   		if breach.Field != "elapsedLimit" {
   			kept = append(kept, breach)
   		}
   	}
   	return kept
   ```

2. `cmd/metasystem/landing_plain.go:31-49`

   ```text

   // laneQueueState answers work land G from the lane's queue when the goal's
   // newest hand-in is at sha (or its branch is gone): waiting, returned with
   // its reason, or landed when main (the seat's endpoint tip) contains it.
   // nil when it has none, or one at another sha, so the hand-in goes on.
   func (inv *intentInvocation) laneQueueState(targets []intentTarget, install, goalID, sha, main string) *intentResult {
   	entry, ok, err := plain.Latest(install, goalID)
   	if err == nil && ok && main != "" {
   		var derived []plain.Entry
   		derived, err = plain.Landed([]plain.Entry{entry}, plain.ContainedIn(inv.layout.InstallationRoot, main))
   		entry = derived[0]
   	}
   	if err != nil {
   		return &intentResult{Targets: targets, Outcome: intentFailed, code: 1,
   			Summary: "the landing lane's queue can't be read, so nothing was handed in",
   			next:    inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
   	}
   	if !ok || (sha != "" && entry.SHA != sha) {
   		return nil
   ```

3. `cmd/metasystem/intent_delivery.go:1849-1874`

   ```text
   			}
   		}
   		if result := inv.laneQueueState(targets, install, goalID, selected, state.EndpointTip); result != nil {
   			return *result
   		}
   	}
   	if state.BranchTip == "" {
   		if landed, ok := latestLanded(base); ok {
   			return intentResult{Targets: targets, Outcome: intentUnchanged, Data: map[string]any{"route": "hand", "landing": landed},
   				Summary: fmt.Sprintf("goal %s landed %s on %s and its branch is swept", goalID, landed.Landing, landed.Endpoint)}
   		}
   		return intentResult{Targets: targets, Outcome: intentRefused, code: 1, Summary: fmt.Sprintf("origin has no goal/%s to land", goalID),
   			next: inv.publicArgv("status", goalID), nextReason: "shows the goal's work"}
   	}
   	subject, _, refusal := handLandingSubject(targets, goalID, through, state)
   	if refusal != nil {
   		return *refusal
   	}
   	if refused := inv.admitLanding(targets, goalID, state.BranchTip); refused != nil {
   		return *refused
   	}
   	if configured {
   		return inv.handIn(targets, laneInstall, goalID, subject, state.EndpointTip)
   	}
   	return inv.landByHand(targets, goalID, through, subject, state, base)
   }
   ```

4. `internal/goal/branch/range.go:262-273`

   ```text
   	unitCommits := map[string]KindInfo{}
   	unitLists := map[string]bool{}
   	seenUnits := map[string]string{}
   	for _, line := range strings.FieldsFunc(strings.TrimSpace(string(out)), func(r rune) bool { return r == '\n' || r == '\r' }) {
   		fields := strings.Fields(line)
   		if len(fields) == 0 {
   			continue
   		}
   		id := fields[0]
   		if len(fields) != 2 {
   			return nil, rangeRefusal(goalID, id, fmt.Sprintf("it has %d parents, and a goal branch commit has one", len(fields)-1))
   		}
   ```

5. `internal/goal/landgate.go:317-333`

   ```text
   	if read.WaitsForHuman || len(read.HeldBy) > 0 || f.Landing == nil {
   		return read
   	}
   	start, err := time.Parse(time.RFC3339, f.Landing.At)
   	if err != nil {
   		return read
   	}
   	for _, h := range f.History[since(f):] {
   		if !strings.HasPrefix(h.Actor, "human:") {
   			continue
   		}
   		if at, err := time.Parse(time.RFC3339, h.At); err == nil && at.After(start) {
   			start = at
   		}
   	}
   	read.ClockFrom = start
   	read.AutoLandsAt = start.Add(s.AutoAfter)
   ```

6. `internal/goal/review.go:525-536`

   ```text

   // VerdictsOf reads one goal's verdicts. A verdict older than the goal's
   // standing Landing answered an earlier landing and says nothing now: a later
   // land-ready puts the goal back in Review with no verdict on it.
   func VerdictsOf(f *GoalFile) Verdicts {
   	read := Verdicts{}
   	if f == nil || f.Landing == nil {
   		return read
   	}
   	since := landingIndex(f)
   	if since < 0 {
   		return read
   ```

7. `cmd/metasystem/holder_step.go:47-60`

   ```text
   func holderStepLine(step goal.HolderStep, result intentResult) string {
   	taken := result.Outcome != intentRefused && result.Outcome != intentFailed
   	data, _ := result.Data.(map[string]any)
   	switch {
   	case step.Revise && taken:
   		return fmt.Sprintf("REVISION STARTED %s, %s: %s", step.Goal, step.Why, result.Summary)
   	case step.Revise:
   		return fmt.Sprintf("REVISION REFUSED %s, %s: %s", step.Goal, step.Why, result.Summary)
   	case taken:
   		if joined, _ := data["joinedNow"].(bool); joined {
   			return fmt.Sprintf("LANDED %s: joined batch %v", step.Goal, data["batchId"])
   		}
   		return fmt.Sprintf("LANDED %s: %s", step.Goal, result.Summary)
   	}
   ```

8. `internal/landing/plain/prove.go:119-135`

   ```text
   // ReadRunning is the proof recorded running and whether its process runs;
   // false when none is recorded.
   func ReadRunning(install string, seams ProveSeams) (Running, bool, bool, error) {
   	data, err := os.ReadFile(runningPath(install))
   	if errors.Is(err, os.ErrNotExist) {
   		return Running{}, false, false, nil
   	}
   	if err != nil {
   		return Running{}, false, false, err
   	}
   	var running Running
   	if err := json.Unmarshal(data, &running); err != nil {
   		// A torn record is a proof that died mid-start.
   		return Running{}, true, false, nil
   	}
   	return running, true, seams.alive(running), nil
   }
   ```

9. `cmd/metasystem/landing_gate.go:187-203`

   ```text
   func (inv *intentInvocation) noteLanded(goalID string, result intentResult) intentResult {
   	if !landed(result) {
   		return result
   	}
   	// A landing on main is the one piece of news the channel carries
   	// (Decision 7), once per sha; a failed post is kept for a retry.
   	if landing := result.Data.(map[string]any)["landing"].(intentLanded); landing.Landing != "" {
   		// The message is the plain sentence of what it delivered: this
   		// command's --delivered, else the one recorded with the landing.
   		text := strings.TrimSpace(inv.input.text("delivered"))
   		if text == "" {
   			text = landing.Delivered
   		}
   		if err := postLanded(inv.layout.InstallationRoot, text, landing.Landing, inv.delivery().now()); err != nil {
   			result.Data.(map[string]any)["landedNotice"] = err.Error()
   		}
   	}
   ```

Example page:

`/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/critique-findings-need-proof.md` (structure and level of detail). Revisions 1 and 2 of this page set the tone: short, in Wido's plain English.

## Recurring findings

none recorded

## Tool-call budget

Maximum delegate tool calls: 45

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: 3800 words

Cut a draft that exceeds this ceiling. If cutting would make the page
incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique
only through the context pack above. Never resume a delegate from an earlier
revision.

## Page artifact and return shape

Write the page to: /Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/seat-path-lands-without-help.md

Do not edit any other file, run test suites, build engines, claim or release goals, or commit.

Return only these two lines, N being the page's word count:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/seat-path-lands-without-help.md
DESIGN: ready (N words)
```

or:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/seat-path-lands-without-help.md
DESIGN: blocked (the reason in one line)
```
