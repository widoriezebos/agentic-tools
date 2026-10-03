# Design brief: seat-path-lands-without-help

## Revision

Revision: first draft

Reason: goal `seat-path-lands-without-help` (tier 2, approved, claimed by seat m1g) has no design record. The page settles how a seat on a computer without a landing lane lands through the same mechanism the lane uses, and what of the old seat self-land plumbing is then deleted.

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check.

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

### What the goal asks (the goal record, verbatim parts)

Intent: "a seat agent takes a finished goal from claim to landed (alone or through the lane) without a person or another session clearing gates. Today's real hand-ins (2026-10-01) each stopped on one: --message hand-in refuses 'delivery candidate differs from working-tree inputs'; receipt line, test run with a named budgeted goal and a goal record all demanded before a hand-in; two sessions in one checkout can't both review or land (lease held by the other); a goal's 1h tier-1 box breach-stops it while it waits on fixes."

Next step, with Wido's two rulings of 2026-10-01:

1. "apply the threat model to every seat-path layer (receipt worktrees, test-policy engine checks, budget-episode gates, custody): what does it defend against; outside 'nobody attacks' -> remove, don't fix."
2. "TARGET: one landing mechanism. A seat without a lane lands exactly as the lane agent does for a batch of one: merge its branch onto latest main, landing prove (same landing.prove.command, detached), landing push (same rail) - on its own landing tree. The lane adds only batching across seats and sharing the expensive suite run. Then delete the old seat self-land plumbing (landpath receipts, test-policy planning in landing, budget-episode gates for landing)."

Narrowed on 2026-10-02 (the goal record, revision 20, edited under Wido's name): "the 10-01 hand-in frictions are largely resolved; on 2026-10-02 the ui seat's hand-in went through the lane end to end (fleet-card-can-land-now, 91579aa80) after only a budget fix. Do NOT re-reproduce them. Remaining work: (1) a seat on a host WITHOUT a lane lands exactly like the lane's batch of one (merge onto main, landing prove with landing.prove.command, landing push); (2) delete the old self-land plumbing (landpath receipts, test-policy planning in landing, landing budget gates), keeping only what the lane and (1) use. Smallest thing that works; ask the human if blocked."

The page designs exactly (1) and (2).

The threat model, as the accepted plain-lane design states it (`/Users/wido/LocalStorage/agentic-tools-evidence/lane-simple-20261001/plain-lane.md`): "Our own agents and operators make mistakes and crash. Nobody attacks. Any reused machinery whose job is to defend against an adversary or a rogue process is bypassed, not kept or fixed."

Fixes are by subtraction: remove the gate or reuse an existing act; a new mechanism needs a reason no existing one covers.

### Background only: the 2026-10-01 frictions (do not design for them)

Read at main 785e68bf5. The old batch lane's plumbing is deleted (54a6daac3) and the plain lane replaced it (8b0c25cd8, 9367b11f9). Two of the frictions live only on the route this page replaces, which is why they are named here:

- "delivery candidate differs from relevant working-tree inputs" (`internal/testrun/prepare.go:821-834`) exists because the seat's landing proof runs in the seat's live working tree, so the tree it proves must equal the files on disk. The lane proves in a fresh worktree at the commit and has no such check.
- The receipt line, `test run --goal` and the budgeted goal are demanded by the staged landing through `internal/landing/landpath`.

One finding is noted and stays out of this page unless Wido says otherwise: a hand-in to the lane does not stop the goal's elapsed clock. `records/goals/review-briefs-name-a-threat-model.md` (tier 1, one-hour box) was breach-stopped at 2026-10-02T01:42:57Z while it waited in the queue, and `fleet-card-can-land-now` needed a person's `set-budget` seconds before its hand-in. The engine's existing act for "built, waiting to land" is `goal.LandReady` (`internal/goal/verbs.go:2206-2262`), which suspends the elapsed fence (`internal/dispatch/admission.go:495-530`); `handIn` (`cmd/metasystem/landing_plain.go:69-88`) does not call it. The page lists this under Deferred in one line and designs nothing for it. It matters to (1) in one way only: the no-lane route also waits on a detached proof, so say whether the elapsed clock runs during that proof and leave it as it is today unless the route cannot work otherwise.

### The two routes of `work land G` today

`landGoalRoute`, `cmd/metasystem/intent_delivery.go:1732-1805`, in order: `resumeSweep` and `finishReleaseSets` (leftovers of the hand route), `laneCheck`, `branchState` (reads the goal branch at origin), with a lane `laneQueueState` (waiting, returned with its reason, or landed when main contains the sha), `handLandingSubject` (every unit has a clean read, or reads are waived), `admitLanding` (`goal.Gate`: a standing review hold, or a tier at or above `landing.review.human-from-tier`, `cmd/metasystem/landing_gate.go:106-116`). Then the routes part:

- **Lane registered:** `handIn` appends `{goal, branch, sha, seat, at, delivered}` to the lane's `queue.jsonl`. Nothing else. The lane's agent merges, runs `landing prove`, runs `landing push`.
- **No lane:** `landByHand`, `cmd/metasystem/intent_delivery.go:1853-1935`: `landCandidate`, `prepareReceipt` (the landing test receipt: testing-policy plan, budget episode, the parity check of friction 1), `landPrep`, `admitLanding` again, `recordReleaseSet`, `landPush`, sweep of the merged branch, `landed.json`, board card. This is the old seat self-land plumbing.

Other forms of the verb (`runIntentLand`, `cmd/metasystem/intent_delivery.go:1512-1585`): `--message` with `--staged` or `--path` (a hand-made change through `landpath`), `work land j2:J` (a job's chain), `--exception` / `--using-exception` (a person's carried landing), `--queue-only`, `--through COMMIT`. With a lane, `--message` and `j2:J` are refused (`laneRegistered`, lines 1608-1616).

### The plain lane's mechanism, which takes its folders as parameters

- `plain.Start(install, checkout, seams)` and `plain.Run(install, checkout, command, attempt, output, seams)` (`internal/landing/plain/prove.go:154-257`): prove `checkout`'s HEAD with the shell command in a fresh detached worktree at that commit; one line per result in `<install>/artifacts/agents/landing/results.jsonl`.
- `plain.Push(install, checkout, now)` (`internal/landing/plain/push.go:50-91`): pushes HEAD to main only when the results say green for exactly HEAD's tree and origin's main is an ancestor of HEAD, with a lease on that main.
- `plain.HandIn`, `Latest`, `Landed`, `Return` (`internal/landing/plain/queue.go:181-316`): the queue; "landed" is derived from main containing the sha.
- The verbs `landing prove`, `landing push`, `landing status`, `landing return` admit only a registered lane (`admitLane`, `cmd/metasystem/intent_landing_prove.go:52-65`) and read `landing.prove.command` from the lane installation's `metasystem.conf` (line 113). The key is set today only in the lane checkout's uncommitted local configuration. Seat m1g has no value for it.

Nothing in `plain` depends on the lane: a seat's installation and a landing tree of its own satisfy the same parameters.

### Sizes, for the deletion list

`internal/landing/landpath` 3,266 code / 4,596 test lines; `internal/landing` (receipt, receipt line, testing, carried, attested, drift, fast-forward, observe, park, registers) 6,407 / 10,176; `internal/goal/branch` land, push, sweep 1,848 code lines; `cmd/metasystem/landing_path.go` 632, `intent_land_staged.go` 229, `intent_land_release.go` 179. `landpath` is imported by `cmd/metasystem/landing_stop.go`, `intent_exception.go`, `precommit_entry.go`, `landing_path.go`, `intent_land_staged.go`, `landing_gate.go`, `helm_admits.go`, `intent_delivery.go`, and by `internal/refusal/register.go` and `internal/testpolicy/protection.go`. The author checks which of these the pre-commit boundary still needs before listing anything for deletion.

### What the page must decide

1. **One landing mechanism.** With no lane registered, what `work land G` does after today's reads and gate, as the lane agent's loop for a batch of one on the seat's own landing tree: where that tree and its records live, who merges, how the detached proof is started and how the seat learns it ended (`metasystem work wait` exists; a seat's tool call never waits more than 240 seconds), what a repeat of `work land G` does in each state (proving, green, red, landed, main moved), and the two lines the seat reads on a conflict and on a red. Whether `landing prove`, `landing push` and `landing status` also work on a seat with no lane, or `work land G` is the whole surface: pick the smaller one that still is the same mechanism. Where `landing.prove.command` comes from on a seat, and the plain refusal when it is unset.
2. **The other forms.** For each of `--message`, `j2:J`, `--exception` / `--using-exception`, `--queue-only`, `--through`: kept on the one mechanism, kept as is with the reason, or deleted. Deleting a form a person uses is a decision for Wido: list it under open questions with a recommendation instead of deciding it, and order the units so nothing waits on that answer that need not.
3. **The deletion.** The exact list of packages, files, settings and refusal codes only the old self-land route reaches once item 1 is built, in landing order, with what each defended against (apply the threat model: outside "our own agents make mistakes, nobody attacks", remove). Keep what the lane and item 1 use, and what the pre-commit boundary still needs. Deleting tests of kept code is not allowed; coverage floors in `testing-coverage-floors.json` may not be lowered.
4. **Units.** At most 300 changed lines each except pure deletions, in landing order, each with its witness test and the mutation that turns it red. Say which is first. Behaviour tests stub git (per-test instances, existing seams); one narrowly named test with real git is allowed where the git interaction is the claim. Each unit lands by itself through the lane on this computer. The last witness is one landing on a seat with no lane, described so it can be run without unregistering this computer's lane (for example in a bed).

Out of scope, name them so: the 2026-10-01 friction items and the elapsed-clock finding above; the lane's own agent, keeper and launch (goal `old-lane-plumbing-is-deleted`); receipts written by the landing (goal `land-verb-writes-the-receipt`); who concludes a landed goal.

Critique findings being answered:

1. none (first draft)

Cited code excerpts:

1. `cmd/metasystem/landing_plain.go:69-88`

   ```text
   func (inv *intentInvocation) handIn(targets []intentTarget, install, goalID, sha, main string) intentResult {
   	now := inv.delivery().now()
   	line := plain.Line{Goal: goalID, Branch: "goal/" + goalID, SHA: sha, Seat: batchowner.LandingLaneRegistrant(inv.layout.InstallationRoot), At: now.UTC().Format(time.RFC3339),
   		Delivered: strings.TrimSpace(inv.input.text("delivered"))}
   	_, added, err := plain.HandIn(install, line)
   	if err != nil {
   		return intentResult{Targets: targets, Outcome: intentFailed, code: 1,
   			Summary: "the landing lane's queue can't be written, so nothing was handed in",
   			next:    inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
   	}
   	if !added {
   		if result := inv.laneQueueState(targets, install, goalID, sha, main); result != nil {
   			return *result
   		}
   	}
   	entry, _, _ := plain.Latest(install, goalID)
   	return intentResult{Targets: targets, Outcome: intentConfirmed, Data: map[string]any{"route": "lane", "queue": entry},
   		Summary: fmt.Sprintf("goal %s at %s handed to the lane; its landing agent proves and pushes it", goalID, plain.Short(sha)),
   		next:    inv.sameCommand(), nextReason: "shows whether it waits, landed or was returned"}
   }
   ```

2. `cmd/metasystem/intent_delivery.go:1794-1804`

   ```text
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
   ```

3. `internal/landing/plain/push.go:72-90`

   ```text
   	if refusal := provenGreen(install, tree); refusal != nil {
   		return outcome, refusal
   	}
   	forward, err := IsAncestor(checkout, old, head)
   	if err != nil {
   		return outcome, err
   	}
   	if !forward {
   		return outcome, &Refusal{Code: CodeNotFastForward,
   			Reason: "HEAD does not contain origin's main " + Short(old) + ", so pushing it would rewrite main; nothing was pushed",
   			Next:   "merge origin/main into the lane checkout, prove it and push again"}
   	}
   	if _, err := Git(checkout, "push", "--quiet", "--force-with-lease=refs/heads/main:"+old, "origin", head+":refs/heads/main"); err != nil {
   		return outcome, fmt.Errorf("push %s to main: %w", Short(head), err)
   	}
   	outcome.Changed = true
   	return outcome, withLock(install, func() error {
   		return appendLine(pushesPath(install), Pushed{Old: old, Commit: head, Tree: tree, At: now.UTC().Format(time.RFC3339)})
   	})
   ```

Example page:

`/Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/critique-findings-need-proof.md` (structure and level of detail), and the plain-lane page cited above for tone: short, in Wido's plain English, what exists first, then what changes, then what is deleted, tests, deferred.

## Recurring findings

none recorded

## Tool-call budget

Maximum delegate tool calls: 60

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: 2500 words

Cut a draft that exceeds this ceiling. If cutting would make the page
incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique
only through the context pack above. Never resume a delegate from an earlier
revision.

## Page artifact and return shape

Write the page to: /Users/wido/LocalStorage/GitHub/agentic-tools-m1g/metasystem/plans/designs/seat-path-lands-without-help.md

The page opens with its head, exactly this shape, the blank line after the title included:

```text
# The page's own title

- Kind: design
- Id: a new 26-character ULID that you generate
- Status: draft
- Goals: seat-path-lands-without-help
```

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
