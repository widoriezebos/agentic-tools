# A run holds its worktree

- Kind: design
- Id: a-run-holds-its-worktree
- Status: accepted
- Goals: a-run-holds-its-worktree
- Critique: round 1 (Astra, designs/astra-unit-route-gaps.log; findings 2 and 3) folded in revision 2 of designs/unit-route-gaps-2026-11-08.md; round 2 (designs/astra-unit-route-gaps-r2.log; the lane transition) folded in revision 3; converged (decision 234)

Accepted 2026-11-08 by a1 in Wido's name (Wido 20:0x: "analyze the causes, then design fixes and get these critiqued by Astra").

Source analysis: designs/unit-route-gaps-2026-11-08.md, section "Changes between steps have no observer".

## Problem

The engine's tree reservation gives a run custody against other engine processes (internal/launch/tree_reservation.go:310), and it snapshots the worktree (internal/launch/unit_run.go:1401-1431), but it compares only within a step ("the worktree bytes changed across this step") and before and after a build. A change between steps or between rounds is attributed to the builder or unseen: on 2026-11-08 a1 stashed ui's U3b worktree at 18:56:29 while attempt 2 was building and restored it at 18:57:01; the engine would have counted the gap as the builder's. Nothing a person reads at the seat says "run R holds this worktree".

## Decisions

**D5a Changes between steps are observed truthfully.** Each step records the worktree's own state at its end, after the engine's own restoration of excluded files (internal/launch/unit_run.go:598): HEAD, the index digest and the tracked tree digest (the `repositorySnapshot` without its `Refs`). The next step, and the next round, compares at start. Shared refs (branches, tags, notes, the stash) stay observations (unit_run.go:1275): they are reported in the stop's evidence ("refs/stash gained an entry at T"), never compared, so the lane's branch moves and a person's unrelated tag never stop a run. A difference stops the round with class "the worktree changed outside the run between step N and step N+1: <paths>", handoff "stopped environment", evidence the diff and the observed snapshot id. A fresh run's round 1 is exempt (the-unit-route-runs-on-the-real-layout D2 governs its baseline). Acceptance: a tracked file written between two steps stops with the class and the path; a stash between steps stops and names `refs/stash`; a branch moved by another process does not stop; the engine's own restoration does not stop; no change, no stop. Mutation: comparing refs fails the moved-branch row; skipping the comparison passes the written file to the builder.

**D5b Recovery acknowledges the observed state.** The stop's prepared correction brief carries the observed snapshot id and says what the builder must do with the outside change (restore it from the named stash or keep it). The seat's `work revise ... --after N --brief FILE --reason TEXT --by NAME` of that round is the acknowledgement: the engine records the observed snapshot as the new baseline (`step-N-after.json` = observed) before the correction round starts, so the next step does not rediscover it. A lane-owned fix run is different: the lane admits only its recorded fix round, and only while the fix record's `Job` is empty (cmd/metasystem/intent_work.go:557-561; nothing clears it on an environment stop), so no re-dispatch can start while the stopped run stands. The transition is explicit: the environment stop of a lane-owned run settles it in one step: the run is cancelled (its name and worktree released), the fix record returns to `resolving` with `Job` empty and a new field `Baseline` holding the observed snapshot id, and the lane's next dispatch of that fix is a fresh run whose round 1 is admitted (Job empty, one round) and whose starting result is bound by the-unit-route-runs-on-the-real-layout D2 against the stopped run's last `worktree.diff`; the lane's allowance of one fix round per attempt is preserved because the settled run no longer counts as the attempt's round. Acceptance: a seat run recovers through revise and its next step sees no difference; a lane-owned fix with a populated `fix.Job` whose run meets an outside change ends with `Job` empty, `Baseline` set, and the replacement run admitted and started; a second outside change on the replacement stops again. Mutation: not clearing `Job` leaves "the lane admits only its recorded fix round"; not recording the baseline makes the correction round stop again with the same class.

**D6 A seat sees the holder.** `work status G --work U` prints one line: "worktree <path>: held by run R since T (round N, step S; last observed state <id>)"; when no run holds it: "worktree <path>: free". No new verb. The a1 rule stands beside it: before any worktree act on another seat's unit, read `work status`.

## Units

| Unit | Production lines | Total lines | Last | Content |
|---|---|---|---|---|
| U1 | 120 | 260 | no | D5a: the step-end snapshot after restoration, the comparison, the stop class and evidence |
| U2 | 120 | 270 | no | D5b: the seat's revise baseline, then the lane's settle transition; seam between the two |
| U3 | 40 | 80 | yes | D6: the holder line in work status |

Estimates run double; a unit whose first round measures over 180 production lines splits at its seam. Injected clocks (R-104-m1e); no toolchain outside the slot; no background loops; no `rm` on a variable path.

## Order

After the-unit-route-runs-on-the-real-layout, on m1e.
