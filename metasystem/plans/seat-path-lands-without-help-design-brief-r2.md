# Design brief: seat-path-lands-without-help

## Revision

Revision: revision 2 of the page `/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/seat-path-lands-without-help.md`

Reason: the goal record gained a unit after the first brief was written. Revision 23 of goal `seat-path-lands-without-help`, edited under Wido's name on 2026-10-02, adds "Unit 1b": a hand-in to the lane suspends the goal's elapsed clock and a return resumes it. Revision 1 of the page lists the elapsed clock under Deferred and designs nothing for it, so the page now disagrees with its goal. No critique has run yet. This revision adds Unit 1b and changes nothing else.

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check.

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

### The prior revision

Revision 1 is the page on disk at `/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/seat-path-lands-without-help.md` (2,495 words, draft, not yet committed). Read it whole, once, before writing. It is not copied into this brief.

Sections 1 to 5 of revision 1 stand as they are: do not rewrite, reorder or reword them. The changes of this revision are confined to:

- a new section for Unit 1b, placed after section 2 or as part of section 6, whichever reads better;
- section 6 (units): add the unit, its witness and its mutation, and say where it comes in the landing order;
- section 7 (out of scope) and section 8 (deferred): the elapsed clock leaves them;
- the line "Revision 1 (first draft)" at the top becomes revision 2, with one sentence saying what changed.

### What the goal record now says (revision 23, verbatim)

"Unit 1b (coordinator m1e, 2026-10-02): a hand-in to the lane suspends the goal's elapsed clock exactly as goal land-ready does, and a return resumes it; a goal waiting in the lane is never budget-stopped for the wait (observed: review-briefs-name-a-threat-model breach-stopped while queued 10-02 01:42Z; fleet-card-can-land-now needed a set-budget). Test: handed in and waiting past the elapsed limit is not breach-stopped; a returned goal's clock runs again. Independent of (1) and (2): build it as its own small unit."

The threat model is unchanged: our own agents and operators make mistakes and crash; nobody attacks. Fixes are by subtraction: reuse an existing act or remove a rule; a new mechanism needs a reason no existing one covers.

### How the elapsed clock works today (read at f85f27efe)

- Elapsed is now, minus the start of the budget episode, minus `Claimed.IdleSeconds` (`internal/dispatch/budget.go:355-364`). The only thing that adds to `IdleSeconds` today is the gap between an own-pair release or park and the same pair's next claim (`resumeEpisode`, `internal/goal/verbs.go:531-552`).
- The stop is fired by the steward tick at the breach limit (the box plus 50 percent by default): `dispatch.FindBreachStops`, `internal/dispatch/stop.go:387-456`.
- `goal.LandReady` (`internal/goal/verbs.go:2206-2262`) writes a `Landing` record on the claim. It does not stop the clock: elapsed keeps counting. What changes is that a claim with that record (`IsLandingClaim`, `internal/goal/file.go:100-106`) is never stopped or refused admission for the elapsed dimension (`internal/dispatch/admission.go:495-530`), and it leaves the machine's one-claim quota, so the seat may claim its next goal.
- `LandReady` is the claim holder's own act (it refuses `--by`, and refuses another pair's claim). It writes the board card `board.StageLandReady`.
- One landing slot per machine: `LandReady` refuses when another goal claimed on the same machine already has a `Landing` record (lines 2249-2255), and tree validation refuses a ledger with two (`internal/goal/validate.go:428-456`).
- Nothing lifts the `Landing` record except the end of the claim: a fresh bind, a release, `done` (`internal/goal/verbs.go:465`, `609`; `internal/goal/abandon.go:332`). There is no act that says "no longer waiting to land".
- `work land G --queue-only` is the public form of `LandReady` (`cmd/metasystem/intent_planning.go:1245-1285`).

### How the lane hand-in and the return work today

- `handIn` (`cmd/metasystem/landing_plain.go:69-88`) appends one line to the lane's `queue.jsonl` and touches no ledger record. It runs in the seat's session, as the claim holder.
- `plain.Return` (`internal/landing/plain/queue.go:268-290`) appends a "returned" line with its time (`ReturnedAt`). It is run by the lane's landing agent through `metasystem landing return`, in the lane checkout. The lane agent is not the goal's claim holder.
- The seat learns of a return when it repeats `work land G`: `laneQueueState` (`cmd/metasystem/landing_plain.go:36-64`) answers "was returned" with the reason. It learns of a landing the same way (main contains the sha) and is then told to run `goal done`.

### What the page must decide for Unit 1b

1. **The suspending act.** Which existing act the hand-in performs so that the wait is not budget-stopped (`goal.LandReady` is the candidate named by the goal record), where in `handIn` it runs relative to the queue line, and what the seat reads when that act fails after the line was written, or the line fails after the act. What a repeat of `work land G` does when the goal is already marked (for example after `--queue-only`).
2. **The one landing slot per machine.** A seat whose goal waits in the lane has left the one-claim quota, so it can claim, build and hand in a second goal while the first still waits or has landed but is not yet concluded. Today the second `LandReady` is refused and a ledger with two is invalid. Apply the threat model: say what the one-slot rule defended against, whether that still exists once the hand route is deleted (unit U4), and what the second hand-in does. If the rule goes, name the two places it is removed and the tests that pin it today. If it stays, say what the second goal's wait costs and why that is acceptable.
3. **The return.** "A return resumes it" needs two things today's code does not have. First, the `Landing` record must be lifted while the claim stays: say who does it (the lane agent's `landing return` runs as another seat; the claim holder's next `work land G` reads the return) and with what act. Second, the arithmetic: because elapsed is computed from the episode start, lifting the record alone makes the whole wait count at once, and a goal that waited past its limit is breach-stopped the moment it is returned. Say where the waited span goes so that the clock "runs again" from where it stood (`IdleSeconds` exists and is already subtracted), and from which two times the span is computed (the hand-in time and `ReturnedAt` are both in the queue). Say what happens between the lane's return and the seat's noticing it.
4. **Landed.** Main contains the sha and the goal is not yet concluded: the claim still carries the record until `goal done`. Say whether that needs anything, in one or two lines.
5. **The no-lane route of section 2.** One line: whether `work land G` on a seat with no lane marks the goal the same way while its detached proof runs, or leaves the clock as it is. Pick the smaller one and give the reason.
6. **The unit.** At most 300 changed lines; split it if the slot rule or the return makes it larger, and say which part lands first. Its witness tests by name, with ledger and git stubbed through the existing seams, covering the goal record's test: handed in and waiting past the elapsed limit is not breach-stopped; a returned goal's clock runs again (and is stopped only after its own remaining time). The mutation that turns each red. It lands by itself through this computer's lane and waits on none of Q1 to Q3.

Do not design: the 2026-10-01 frictions, the lane's agent or keeper, receipts, who concludes a landed goal.

Critique findings being answered:

1. none (no critique has run; the goal record changed)

Cited code excerpts:

1. `internal/goal/verbs.go:2206-2215`

   ```text
   // LandReady marks the claim holder's built and verified work as waiting to
   // land. The goal stays claimed, so its receipts still bind to the claim,
   // but the claim leaves the machine's one-claim quota and its elapsed fence
   // while it waits; the landing lifts the record with the claim.
   func LandReady(r VerbRequest, id string) (PublishResult, error) {
   	if r.Actor.Human != "" {
   		return PublishResult{}, fmt.Errorf("land-ready is the claim holder's own act; it takes no --by")
   	}
   	return publishedCard(Publish(r.Endpoint, landReadyRequest(r, id)))(func() { writeOwnCard(r, id, board.StageLandReady) })
   }
   ```

2. `internal/goal/verbs.go:2243-2257`

   ```text
   			if f.StopFence != nil {
   				return nil, fmt.Errorf("goal %s is breach-stopped by %s; only goal resume, a human act, clears the fence", id, f.StopFence.StopID)
   			}
   			if f.Landing != nil {
   				return nil, AlreadyHolds{Reason: "goal " + id + " is already queued to land (since " + f.Landing.At + ")"}
   			}
   			for _, other := range t.Live {
   				// A fenced landing claim still holds the slot: the resume
   				// restores it, and two slots would then refuse the resume.
   				if other.Id != id && other.State == StateClaimed && other.Claimed != nil && other.Landing != nil && other.Claimed.Machine == r.Actor.Machine {
   					return nil, fmt.Errorf("goal %s already waits to land on machine %s; one landing slot per machine: land it before entering %s", other.Id, r.Actor.Machine, id)
   				}
   			}
   			touch(f, r, "land-ready", []string{id})
   			f.Landing = &LandingRecord{At: r.stamp(), Opid: r.opid()}
   ```

3. `internal/dispatch/admission.go:495-505`

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
   ```

4. `internal/dispatch/budget.go:355-364`

   ```text
   		// The elapsed clock excludes the time nobody held the goal (an own-pair
   		// release or park, then the same pair's claim). A discharge inside the
   		// current hold starts a window with no idle gap to subtract.
   		idle := time.Duration(file.Claimed.IdleSeconds) * time.Second
   		if claimedAt, parseErr := time.Parse(time.RFC3339, file.Claimed.At); parseErr == nil && !budgetStartedAt.Before(claimedAt) {
   			idle = 0
   		}
   		projection.Elapsed = now.Sub(budgetStartedAt) - idle
   		if projection.Elapsed < 0 {
   			projection.Elapsed = 0
   ```

5. `internal/landing/plain/queue.go:268-290`

   ```text
   func Return(install, goal, reason string, now time.Time) (entry Entry, changed bool, err error) {
   	err = withLock(install, func() error {
   		latest, ok, err := Latest(install, goal)
   		switch {
   		case err != nil:
   			return err
   		case !ok:
   			return fmt.Errorf("%w: %s was never handed in", ErrNotWaiting, goal)
   		case latest.State == StateReturned:
   			entry = latest
   			return nil
   		case latest.State != StateWaiting:
   			return fmt.Errorf("%w: %s already %s", ErrNotWaiting, goal, latest.State)
   		}
   		at := now.UTC().Format(time.RFC3339)
   		if err := appendLine(queuePath(install), Line{Goal: goal, SHA: latest.SHA, At: at, Outcome: StateReturned, Reason: reason}); err != nil {
   			return err
   		}
   		latest.State, latest.Reason, latest.ReturnedAt = StateReturned, reason, at
   		entry, changed = latest, true
   		return nil
   	})
   	return entry, changed, err
   ```

6. `cmd/metasystem/landing_plain.go:57-63`

   ```text
   	case plain.StateReturned:
   		return &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
   			Summary: fmt.Sprintf("goal %s at %s was returned: %s", goalID, plain.Short(entry.SHA), entry.Reason),
   			next:    inv.sameCommand(), nextReason: "after the fix is pushed to " + entry.Branch + ", hands it in again"}
   	}
   	return &intentResult{Targets: targets, Outcome: intentUnchanged, Data: data,
   		Summary: fmt.Sprintf("goal %s at %s is waiting in the landing lane; its landing agent proves and pushes it", goalID, plain.Short(entry.SHA))}
   ```

Example page:

`/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/critique-findings-need-proof.md` (structure and level of detail). Revision 1 of this page sets the tone: short, in Wido's plain English.

## Recurring findings

none recorded

## Tool-call budget

Maximum delegate tool calls: 40

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: 2950 words

Cut a draft that exceeds this ceiling. If cutting would make the page
incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique
only through the context pack above. Never resume a delegate from an earlier
revision.

## Page artifact and return shape

Write the page to: /Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/seat-path-lands-without-help.md

Keep the page's head exactly as revision 1 has it (title, Kind, Id `01M3Z6EXJH4CCXWDNZA9DWB8X2`, Status draft, Goals).

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
