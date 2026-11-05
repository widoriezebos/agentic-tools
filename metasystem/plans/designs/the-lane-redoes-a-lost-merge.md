# The lane redoes a lost merge

- Kind: design
- Id: the-lane-redoes-a-lost-merge
- Status: accepted
- Goals: a-green-rounds-read-is-collected

Accepted 2026-11-05 by a1 in Wido's name (decision 202, the seat contracts program at priority 1): the redo half of the joint page the-lane-finishes-its-own-resolution; after the split, Astra's round 3 (4 material, 1 minor; verdict build after the folds) was folded in full. This page serves contract 3 of the seat's actors-and-contracts model (waits belong to the engine: the keeper owns the capacity wait and the redo of a lost merge, both counted); the decisions of section 4 (two redos per goal and tip; six refusals, then the person's merge build; admission clears the wait; the 11-05 cause a hypothesis) are a1's in Wido's name; Wido may overrule with one line. It builds after the-lane-collects-its-resolution.

Written 2026-11-05 by opus-designer (Claude Opus 5.5), split by a1 under decision 202 from the deleted joint page `the-lane-finishes-its-own-resolution`. Blocked by `plans/designs/the-lane-collects-its-resolution.md`, whose `coordinationOnly` (D1) and person's merge build (D7) it reuses. Decision numbers are the joint page's; Astra's rounds 1 and 2 are folded; a third round follows.

Code: main at `f3a5db89f0`, every `file:line` read there. Lane records, read only: `agentic-tools-landing/metasystem/artifacts/agents/landing/events.jsonl`, `fixes/20261105T114843.298826000Z.json`. Findings: `agentic-tools-evidence/lane-test-20261009/findings.md`, entries 524, 526 and 536.

## 1. Intent

The engine waits out a busy host, with a count, and the lane resumes when admission permits. The lane redoes a lost merge once its old writers are gone and nothing else would be lost. This serves contracts 1, 2 and 3 of `agentic-tools-evidence/lane-test-20261009/designs/the-seats-actors-and-their-contracts.md`, mainly 3: the keeper owns the capacity wait and the redo, with a count.

## 2. Problem

- Findings 524 and 526. On 11-04 the lane's resolution launch was refused for load; the agent waited in its own unbounded loop on a copied 7.5, while the lane declared 24 (decision 199, local configuration, not read).
- Finding 536. At 11:48:43Z on 11-05 `landing resolve --as-seat` recorded a new attempt resolving (`events.jsonl`, by "resolve"). At 11:49:17Z an event by "landing resolve" rebuilt the batch view. At 11:51:03Z the keeper recorded "the pending merge is gone; its resolution was abandoned". Status named `landing resume`, a person's act. What removed the merge is not recorded: hypothesis.

What the code does:

- A refused launch leaves no counted wait: `LaunchResolution` replaces `fix.Launch` and records only `Error` (`metasystem/internal/landing/plain/launch_record.go:9-43`); so does `laneMergeWork` when no job exists (`metasystem/cmd/metasystem/landing_merge.go:69-70`). `createAdmitted` reserves as it decides (`metasystem/internal/launch/hostcapacity.go:61-140`).
- A lane question holds the keeper (`metasystem/cmd/metasystem/landing_agent.go:217-230`, holds `:377-405`). Two barren launches hold it for a person until the fingerprint changes (`metasystem/internal/landing/lane/agent.go:45`, `:487-513`); `KeeperFingerprint` has no fix records (`metasystem/internal/landing/plain/wake.go:285-305`; reasons `:22-33`).
- Abandonment is written only by a writer (`metasystem/internal/landing/plain/fix.go:299-339`); the steward observes read-only (`metasystem/internal/landing/plain/steward.go:82`) and acts with `ReadOnly` false (`:367`). `ReadFix` excludes abandoned records (`fix.go:65`, `:108-110`); status shows the latest (`:130-132`); `Resolve` then changes nothing (`metasystem/internal/landing/plain/resolve.go:100-108`).
- `ResolutionWritersSettled` (`metasystem/internal/landing/plain/resume.go:274-282`) needs `ResumeKeeper`, set only by `landing resume` and `landing deploy` (`metasystem/cmd/metasystem/intent_landing_resume.go:30`, `metasystem/cmd/metasystem/intent_landing_deploy.go:30`).
- A fresh resolution refuses any difference from `AUTO_MERGE`, staged or not, and any untracked file (`resolve.go:176-186`); with no unresolved paths it returns before an attempt (`:126-129`); an attempt opens at `:198-206`.
- `abandoned-resolution` is report only (`metasystem/internal/landing/plain/health.go:718-720`, `steward.go:279`); status says `landing resume` (`metasystem/internal/landing/state/describe.go:70`), which aborts a present merge of the same tip (`resume.go:104-122`).

## 3. Decisions

- **D3 A capacity refusal is a keeper-owned wait with a count.** `LaunchRecord` gains `Capacity *CapacityWait {Cause, Limit, Refusals, Since}`. For the same attempt, `laneMergeWork` (`landing_merge.go:70`) and `LaunchResolution` (`launch_record.go:30`) both carry the wait into the new `fix.Launch`; `LaunchResolution` increments `Refusals` once per refusal with a retryable cause (`overload`, `build-cap`, `unknown-capacity`) and clears the wait only on admission. `StartSpec` gains `DryRun`, which returns from `createAdmitted` before the reservation (`hostcapacity.go:61-140`); `launch.Manager.Admits(spec)` calls it. The lane build binds `BuildPolicy` to the selected installation (`metasystem/cmd/metasystem/intent_work.go:485-489`); the keeper's default manager reads the serving engine's (`metasystem/cmd/metasystem/launch_verbs.go:48`). One helper, `laneAdmission(install) launch.Manager`, binds the lane's policy for both, and the probe's spec carries the resolution's goal, kind `build` and `LaneMerge`, since admission counts distinct goals (`hostcapacity.go:126`). A keeper hold (beside `landing_agent.go:377-405`) reads the active fix: a wait and `Admits` false holds, "waiting for host load below LIMIT" (or the cause's words); `Admits` true adds the wake reason `capacity` (`wake.go:22-33`), and the agent repeats the build command. `KeeperFingerprint` includes `Refusals`, so a race refusal is not barren. At 6 refusals the hold stays, and the health row `capacity-loop` and status print the complete person-build command of the other page's D7: `metasystem work build G --work lane-merge-1 --brief <lane>/artifacts/agents/landing/fixes/ATTEMPT.build.md --check 'metasystem test impact'`, with the actual goal and the retained brief (`landing_merge.go:76`), which passes the verb's argument check (`landing_merge.go:53`); the engine does not append its check line twice. No question is asked; no person stop is touched.
  Rejected: an agent loop, then a question that outlives the load. Rejected: a counter in `LaunchResolution` alone; `laneMergeWork` erases it.

- **D5 The lane redoes a lost merge once its old writers are gone.** One owner, `plain.RedoLostMerge(install, checkout, now, seams)` in `fix.go`, under the lane lock.
  - Read and persist. It reads the active, else the latest abandoned, merge attempt (`readFixRecords`, `fix.go:65-132`) and runs `refreshMergeLocked`, so a vanished merge is written abandoned before anything is decided.
  - Eligible only when all hold: no pending merge (`MERGE_HEAD` absent); the old writers settled (`ResolutionWritersSettled`); no staged change (`git diff --cached --name-only` empty); no unstaged or untracked change outside the coordination policy (`coordinationOnly` of the other page, with no merge paths); the goal's hand-in still waits at the fix's tip in the batch; fewer than 2 redos used for this goal and tip. Another commit's pending merge keeps its owner; a present merge of the same tip stays `landing resume`'s.
  - The act closes the fix ("redone by the lane: " plus the old reason) through `landingstate.Transition`, as `fix.go:117-119` does; appends `{goal, tip, at}` to `redo.jsonl`, an append-only usage ledger counted per goal and tip; and adds a pending continuation to `continuations.json`, keyed by goal and tip.
  - Consumption. A pending continuation is consumed when `Resolve --as-seat` opens a new attempt for that goal and tip (`resolve.go:198-206`), or when `SettleRedo` finds that exact tip merged without conflict (the lane checkout's `HEAD` contains it and no merge is pending) or already contained in `origin/main`. The steward calls `SettleRedo` first in each acting step.
  - Fresh admission. `Resolve`'s guard (`resolve.go:176-186`) passes worktree and untracked paths through `coordinationOnly` with the merge's paths, the conflicted paths (`resolve.go:88`) plus every goal-side changed path (the other page's D1), and refuses what remains; a coordination file in the merge stays protected, since the non-seat path aborts the merge (`resolve.go:245`); staged changes refuse; ignored bytes stay. The redo itself, with no merge, passes none.
  - Through the keeper. `KeeperFingerprint` includes the pending continuations, so the barren hold clears by its own rule; `WakeReasons` adds `redo-merge` while one is pending. The production steward composition (`landing_agent.go:458-470`) supplies `ResumeKeeper`, the resolution store, and the coordination policy: `ProveSeams` (`metasystem/internal/landing/plain/prove.go:177`) gains `Coordination []string`, set to `landing.WorkspaceExclusions()`; `RedoLostMerge` converts it with D1's checkout-relative rule, and a nil policy ignores nothing. For T7, `landingAgentStep` (`landing_agent.go:453`) becomes a wrapper of `landingAgentStepWith(repo, stepOwners{agent, keeper, now})`, so the test runs the same composition with an injected clock. The steward acts on `abandoned-resolution` (`steward.go:279`): `SettleRedo`, `RedoLostMerge`, then `effects.Wake` (`steward.go:399-404`). Not eligible: the row stays report only, naming why. Status and health say "the lane redoes the merge" when eligible, `landing resume` otherwise.
  Rejected: close and wake only; the first sight is read-only and nothing counts. Rejected: one `{goal, tip, count}` record; another goal replaces it. Rejected: consuming only at a new attempt; a clean replay stays pending. Rejected: a person's `landing resume` each time (R-135-m1e).

## 4. Decisions taken by a1 in Wido's name

Two redos per goal and tip. Six refusals in a row; after them the recovery is the person's merge build (D3). A successful admission clears the wait. Finding 536's cause stays a hypothesis; the redo reads the merge state, not a cause.

## 5. Units

Production lines are additions plus deletions; a replaced line counts twice. Counted by file at `f3a5db89f0` after the other page's units, then doubled in the table. Stop line 250; split at 110.

| Unit | Decisions | Production lines | Tests (estimate) | Split seam | Step value |
|---|---|---|---|---|---|
| U3a the capacity record | D3 | 72 | 100 | none needed | a refused launch leaves one counted wait |
| U3b1 the capacity probe | D3 | 60 | 90 | none needed | the keeper probes the lane's own admission rule |
| U3b2 the capacity wake | D3 | 60 | 90 | none needed | the keeper resumes when admitted; after six, the person build |
| U4a1 the redo owner | D5 | 96 | 130 | none needed | a lost merge is persisted, then closed only when safe |
| U4a2 the redo records | D5 | 56 | 80 | none needed | redo usage per goal and tip survives other goals |
| U4b1 admission and consumption | D5 | 64 | 100 | none needed | a fresh attempt opens beside engine records; a clean replay ends the continuation |
| U4b2 the redo through the keeper | D5 | 108 | 150 | none needed | the steward redoes and wakes; the keeper restarts from a barren hold |

- U3a, 36: `launch_record.go` 16 (`CapacityWait` 6; carry, increment, clear 10); `landing_merge.go` 8 (carry the wait at `:70`); `metasystem/internal/launch/hostcapacity.go` 4 (`DryRun` return); `metasystem/internal/launch/launch.go` 8 (the field 1; `Admits` 7).
- U3b1, 30: `landing_agent.go` 30 (the capacity hold 18; `laneAdmission` 6 and its two uses 4; the probe's spec 2).
- U3b2, 30: `wake.go` 10 (the reason 6; `Refusals` in the fingerprint 4); `health.go` 8 and `describe.go` 4 (`capacity-loop`); the complete command 6; `landing_merge.go` 2 (no second append to a retained brief).
- U4a1, 48: `fix.go` 48 (the reader of active or latest abandoned 8; persist 4; the merge test 6; settlement 4; the index guard 4; `coordinationOnly` on worktree and untracked 8; the batch and tip test 6; the close through `Transition` 8).
- U4a2, 28: `fix.go` 28 (`redo.jsonl` append and count 12; `continuations.json` read, add, consume 12; the two writes in the act 4).
- U4b1, 32: `resolve.go` 18 (the fresh guard through `coordinationOnly`, 3 replaced lines 6, plus 2; the merge paths from the conflicted list and the goal-side helper 4; consumption at the new attempt 6); `fix.go` 14 (`SettleRedo`: the contained and clean-merge tests and the consume).
- U4b2, 54: `steward.go` 18 (the act list, 1 replaced line 2; the case with `SettleRedo`, the redo and the wake 16); `landing_agent.go` 18 (`ResumeKeeper`, the store and the policy 6; `landingAgentStepWith` and `stepOwners` 12); `prove.go` 2 (the field); `wake.go` 10 (continuations in the fingerprint 4; `redo-merge` 6); `describe.go` and `health.go` 6.

Order: U3a, U3b1, U3b2, U4a1, U4a2, U4b1, U4b2. Reverse dependents run (R-18): `createAdmitted`, `KeeperFingerprint`, `Resolve`, the steward's acts.

## 6. Tests

New or changed behaviour only, through the production composition, `landingAgentStepWith` included, with injected clocks and probers (R-104-m1e). Each behavioural assertion fails on main.

- T5 `TestACapacityWaitIsCountedAndResumedByTheKeeper` (U3a, U3b1, U3b2, `metasystem/cmd/metasystem`), two fixtures. (a) Recovery: two refused production build commands (`Refusals` 1, 2); a keeper step at load above: held, "waiting for host load below 24", no start; below: one start with reason `capacity`; the admitted launch clears the wait. Here the serving engine declares limit 8 and the lane 24, at load 16: the probe admits, as the launch does. (b) Exhaustion: six refused commands (`Refusals` 1 to 6, once each); the hold stays, no start, no question, barren count 0, a person stop untouched; status prints the complete command; the test runs that exact command as an enrolled person through D7 and the wait clears. First failure on main: no wait is recorded. Mutation: carry the wait only in `LaunchResolution`; the count stays 1 and (b) fails at the second command.
- T6 `TestARedoNeedsALostMergeSettledWritersAndAQuietCheckout` (U4a1, U4a2, `metasystem/internal/landing/plain`, real git). (a) From the incident's shape, a fix persisted abandoned with no `MERGE_HEAD`: closed, usage 1, one continuation. (b) From a persisted resolving fix whose merge was removed, the job still running: written abandoned, not closed; a later call after the job ends: closed. (c) A staged source edit: no act; fix, `git diff --cached` and worktree bytes unchanged. (d) An unstaged `metasystem/internal/x.go`: no act, bytes kept. (e) An unstaged narrator digest: acts, bytes kept. (f) Another commit's `MERGE_HEAD`: untouched. (g) Redo A, B, A; a third A is refused while B has one left. (h) A repeat writes nothing (R-129-ui). First failure on main: the function does not exist; then (b) leaves the fix resolving. Mutation: drop the index guard; (c) acts and fails.
- T7 `TestTheKeeperRedoesALostMergeFromABarrenHold` (U4b1, U4b2, `metasystem/cmd/metasystem`, lane bed, real git, the real keeper, the production steward composition). Barren count 2, a fix persisted abandoned, an unstaged narrator digest, the old job still running. Tick 1: no redo. The job ends; tick 2: the redo, the wake; the keeper's hold clears by the fingerprint; the agent (injected) merges the tip again; `landing resolve --as-seat` opens a new attempt beside the digest and consumes the continuation; the build collects to reviewing; the digest's bytes are unchanged; the policy arrives through `landingAgentStepWith`, not the test. An unstaged edit to a coordination file the goal side changes refuses the fresh attempt and its bytes stay. Variants, a clean replay and a tip already on `origin/main`: the next tick's `SettleRedo` consumes the continuation. With a person stop recorded: nothing starts and the stop stays. First failure on main: the row is report only and the keeper stays held. Mutation: leave the digest out of the fresh guard's filter; the new attempt refuses and the test fails.
- T8 `TestLandingStewardRemainingReportOnlyRows` (changed, U4b2, `metasystem/internal/landing/plain/steward_test.go:466`). The `abandoned-resolution` case leaves; the other three stay as regression cases. No mutation claimed: T6 and T7 fail first.

## 7. Moved effects

| Effect | From | To | Code |
|---|---|---|---|
| waiting out a capacity refusal of the lane | the agent's own loop or a person | the keeper's counted capacity hold and wake, on the lane's admission rule (U3a, U3b1, U3b2, D3; T5) | `metasystem/internal/landing/plain/launch_record.go:20-43`, `metasystem/cmd/metasystem/landing_agent.go:377-405` |
| admitting a fresh lane resolution beside other checkout changes | refused for any difference (`resolve.go:176-186`) | refused except coordination paths outside the merge, staged changes still refused (U4b1, D5; T7) | `metasystem/internal/landing/plain/resolve.go:176-186` |
| redoing a resolution whose pending merge is gone | a person's `landing resume` (`describe.go:70`) | the lane's `RedoLostMerge` and its counted continuation through the keeper (U4a1 to U4b2, D5; T6, T7) | `metasystem/internal/landing/plain/steward.go:279`, `metasystem/internal/landing/state/describe.go:70` |

## 8. Triggers

- Briefed when the other page's U1a lands and this page is accepted. Precondition, stated not asked: the lane runs an engine containing the landed units, by a person's `landing deploy` (`intent_landing_deploy.go:14`).
- Re-read when finding 536's cause is found.

## Critique record

- Round 1, 2026-11-05, Astra on the joint page: 7 material, 1 minor, redesign. Here: F3, F4, F5 to D5; F6 to D3; F8 to T8.
- Round 2, 2026-11-05, Astra, log `agentic-tools-evidence/lane-test-20261009/designs/astra-lane-resolution-r2.log`: 7 material, 1 minor, redesign; a1 split the page (decision 202). Here: F1 (reader, `ResumeKeeper`), F2 (fresh admission), F3 (index guard), F5 (usage ledger, consumption) to D5; F4 (both owners carry the wait) to D3; tests T5 to T7; units recounted.
- Round 3, 2026-11-05, Astra, log `agentic-tools-evidence/lane-test-20261009/designs/astra-lane-redo-r3.log`: 4 material, 1 minor; verdict build after the folds. F1 to D5's fresh admission (merge paths protected) and T7; F2 to `ProveSeams.Coordination` and T7; F3 to `laneAdmission`, the probe's goal and T5(a) at limits 8 and 24; F4 to the complete command and T5(b) through D7; M1 to T5's two fixtures; `landingAgentStepWith` named in U4b2. U3b split; U4b1 and U4b2 recounted.
