# Design brief: seat-path-lands-without-help

## Revision

Revision: revision 4 of the page `/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/seat-path-lands-without-help.md`

Reason: round 2 of the design critique (Codex on Astra, job design-critic-46437605ef0c580fea7461d1-r2, reviewed at 4babb05ec) returned four material findings, all accepted. Wido also answered three of the page's questions on 2026-10-03. This revision answers the findings with the amendments below and records the answers. Nothing else changes.

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check.

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

### The prior revision

Revision 3 is the page on disk at `/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/seat-path-lands-without-help.md` (on main at defe189b1). Read it whole, once, before writing. It is not copied into this brief. Every section stands except where an amendment or answer below changes it. Keep the head exactly (title, Kind, Id `01M3Z6EXJH4CCXWDNZA9DWB8X2`, Status draft, Goals). The line "Revision 3: ..." becomes revision 4, with one sentence saying it answers critique round 2 and records Wido's answers.

The threat model is unchanged: our own agents and operators make mistakes and crash; nobody attacks. Fixes are by subtraction: reuse an existing act or remove a rule; a new mechanism needs a reason no existing one covers.

### Wido's answers (2026-10-03, through the channel, recorded on the goal)

- Q1 (delete the publishing `--message` forms): "yes". D2 becomes unit U5, a pure deletion after U4.
- Q2 (delete `--exception` and `--using-exception`): "not-now and explain in the new goal why this is delayed and why we should do it". Goal `landing-exception-gets-a-successor` was opened for it (blocked by this goal). D3 leaves this page: name that goal as its owner in one line; drop U6.
- Q4 (a seat's conflict): "this is too simplistic. This needs a proper design and review round. The goal is unattended functionality". Goal `conflicts-resolve-unattended` (tier 3) owns conflict resolution for a seat without a lane, for the lane's conflict returns and for moving a goal branch onto a newer main. See amendment 3.
- Q3 (which proof command seats use) is not asked; keep it as the page has it.

### The accepted amendments (the seat's decisions, verbatim)

1. RULING-R-142-m1e (section 2b): "`handIn` writes the queue line first and marks after it. Before the line it checks, read-only from the projection the gate already read, that the mark would be taken (the seat's own claim, no stop fence), so a stopped goal still hands nothing in. A mark that fails after the line is repaired by the repeat: its "waiting" answer marks (or moves the mark). The moved mark follows the same order. Then no gate read ever falls between a hand-in's mark and its line, and a person's word for the tip stays in force for every retry. Say on the page that the word's window is otherwise unchanged (a later land-ready still opens a new review), and drop the matching item from section 9."

2. RULING-seat-path-lands-without-help (section 2b; this replaces revision 3's "the span ends at the seat's act" and "a wait that ends in a landing is never credited"; amendments (b) to (d) of round 1 stand: the moved mark, the return read at any commit, the waiting repeat that marks): "One rule for every wait, whatever its outcome: the clock stops at the mark and runs again at the first of (1) the goal's own next job or proof being active (a job already running at the mark ends the wait at once), (2) the next hand-in, which moves the mark, and (3) the act that lifts the mark (`LandReturn`, or `goal done`). While marked, the budget projection subtracts that open span from elapsed; it already reads the goal's job and proof records with their start times (`internal/dispatch/budget.go`, `ProjectBudget`, 269-370 and the job loop after it). Every act that moves or lifts the mark adds the same span to `IdleSeconds`. A wait that ends in a landing is credited like a return. Work the seat does on the goal without a job (reading, writing a brief) during a wait stays uncharged: state that bound on the page. Because a pure wait no longer grows elapsed, the elapsed exemptions `landingStopReason` and `admissionBreachesFor` can both go (subtraction): a marked goal that works past its limit is refused and stopped like any other. `LandingOverdue` uses the same computation. Order the units so that no unit marks more goals than today before the exemption is gone and the credit is in."

3. SEAT-CONFLICT-RECOVERY (the conflict row, section 4, section 6, section 7): "Remove Q4 and its options. The conflict row stays a refusal: line 1 names the paths and says nothing was proven; line 2 is `metasystem goal show conflicts-resolve-unattended`, the goal that owns the recovery. Section 7 lists conflict recovery as out of scope with that goal as its owner. U1 keeps `merge --abort` and the conflict's paths; the recovery witness is dropped from U3 and from the bed run."

4. SEAT-LANDED-SUBJECT (section 2, loop step 1; U3's witness): "The landed check's subject is this goal's work: the branch tip, or a `--through` commit that the branch reader classifies as a unit of goal G (its unit trailer names G: `branch.KindOf`, `internal/goal/branch/range.go:129`) and that is the tip or an ancestor of it. Any other `--through` falls through to `handLandingSubject`'s refusal. Witness: `--through` naming another goal's commit that main holds is refused, not answered as landed."

### Notes for the rewrite of section 2b

- With amendment 2 the stop exemption and the admission exemption both go, so the bullet "The mark covers a wait, never the goal's next work" becomes the clock rule of amendment 2. Keep the rest of 2b that still holds: every hand-in marks (now after its line), the one landing slot goes, the moved mark, `LandReturn` at any commit, the holder's Stop line, the idle rule at `budget.go` (excerpt 3) becoming "after", landed and no lane.
- The `IsLandingClaim` reads that stay: the one-claim quota (`validate.go`), the current claim (`goalverbs.go:935`), the machine-wide admission skip (`admission.go:146`), `LandingDue` and `LandingClaimLines`. Say in one line that they are unchanged.
- Units: no unit may mark more goals than today (`--queue-only` only) before the credit is in and the exemptions are gone. One workable order: U1b.1 the clock (the open wait subtracted, credit written by `LandReady`'s move and by goal done or release where a mark is lifted, the exemptions removed, the idle rule), U1b.2 every hand-in marks after its line and the slot rule goes, with the Stop's line, U1b.3 `LandReturn` at any commit and the moved mark. Each at most 300 changed lines, each with its witnesses and mutations; split further if needed and say which lands first.

### Size

Maximum page size below: 4,400 words. The answers above remove Q4, D3's detail and U6, which should pay for most of amendment 2.

Critique findings being answered:

1. RULING-R-142-m1e (high): the mark written after the gate hides the person's word, so a retry after a failed queue write demands the word again.
2. RULING-seat-path-lands-without-help (high): a wait that ends in a landing is never credited, so the next unit is refused for time nobody worked.
3. SEAT-CONFLICT-RECOVERY (high): revision 3 left the conflict's recovery open between three choices.
4. SEAT-LANDED-SUBJECT (medium): the early landed check accepts a `--through` commit of another goal that main already holds.

Cited code excerpts:

1. `internal/goal/landgate.go:208-230`

   ```text
   func since(f *GoalFile) int {
   	if index := landingIndex(f); index >= 0 {
   		return index + 1
   	}
   	return 0
   }

   func newestWord(f *GoalFile) *word {
   	if f == nil {
   		return nil
   	}
   	var newest *word
   	for _, h := range f.History[since(f):] {
   		switch h.Verb {
   		case reviewVerb:
   			if line, err := parseReviewReason(h.Reason); err == nil {
   				newest = &word{kind: line.Verdict, tip: line.Tip, by: line.By, opid: h.Opid, record: line.Record}
   			}
   		case LandWithoutSittingVerb:
   			if decided, err := parseWithoutSitting(h.Reason); err == nil {
   				newest = &word{kind: LandWithoutSittingVerb, tip: decided.Tip, by: decided.By, opid: h.Opid, because: decided.Reason}
   			}
   		}
   ```

2. `internal/goal/landgate.go:252-270`

   ```text
   	}
   	said := newestWord(f)
   	missing := func(why string) error {
   		return &GateRefusal{Code: GateWaitsForHuman, Reason: fmt.Sprintf(
   			"goal %s is tier %d, at or above landing.review.human-from-tier=%d, and waits for a person: %s; a person reviews it in a sitting that ends clear to land, or lands it without a sitting: metasystem goal land-without-sitting %s --reason TEXT",
   			f.Id, tier, s.HumanFromTier, why, f.Id)}
   	}
   	switch {
   	case said == nil:
   		return "", missing("its history carries no clear-to-land verdict and no land-without-sitting decision")
   	case said.kind == VerdictSendBack:
   		return "", missing(fmt.Sprintf("its newest verdict is %s's send-back", said.by))
   	case tip == "":
   		return "", missing("this landing names no branch tip the word could be bound to")
   	case said.tip != tip:
   		return "", missing(fmt.Sprintf("%s's word was given at %s and the branch is now at %s; a moved tip needs the word again", said.by, short(said.tip), short(tip)))
   	}
   	return said.under(tier, s), nil
   }
   ```

3. `internal/dispatch/budget.go:352-366`

   ```text
   		if err != nil {
   			return unknownBudget(file.Id, revision, recordPath, err.Error())
   		}
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
   		}
   		projection.ElapsedGracePercent = gracePercent
   ```

4. `internal/dispatch/admission.go:496-530`

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

5. `cmd/metasystem/intent_delivery.go:2068-2083`

   ```text
   	if through == "" {
   		if landable != len(units) {
   			return "", 0, unread(landable)
   		}
   		return state.BranchTip, len(units), nil
   	}
   	for index, unit := range units {
   		if unit.Commit == through {
   			if index >= landable {
   				return "", 0, unread(landable)
   			}
   			return unit.Commit, index + 1, nil
   		}
   	}
   	return "", 0, &intentResult{Targets: targets, Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("--through %s is not a work commit on goal/%s; nothing was landed", through, goalID),
   		next: []string{"metasystem", "status", goalID}, nextReason: "lists the goal's commits; --through takes one in full"}
   ```

6. `internal/goal/branch/range.go:129-135`

   ```text
   func KindOf(repo, commit, goalID string) (KindInfo, error) {
   	return kindOfWithGit(repo, commit, goalID, gitOutput)
   }

   func kindOfWithGit(repo, commit, goalID string, gitRead func(string, ...string) ([]byte, error)) (KindInfo, error) {
   	out, err := gitRead(repo, "show", "-s", "--format=%(trailers:only,unfold=true)", commit)
   	if err != nil {
   ```

Example page:

`/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/critique-findings-need-proof.md` (structure and level of detail). Revisions 1 to 3 of this page set the tone: short, in Wido's plain English.

## Recurring findings

none recorded

## Tool-call budget

Maximum delegate tool calls: 45

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: 4400 words

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
