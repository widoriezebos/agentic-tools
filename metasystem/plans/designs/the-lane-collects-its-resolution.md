# The lane collects its resolution

- Kind: design
- Id: the-lane-collects-its-resolution
- Status: accepted
- Goals: a-green-rounds-read-is-collected

Accepted 2026-11-05 by a1 in Wido's name (decision 202, the seat contracts program at priority 1): the collection half of the joint page the-lane-finishes-its-own-resolution, whose Astra rounds 1 and 2 (7 material and 1 minor each) were folded in full; the split put the keeper-owned wait and the redo of a lost merge on their own page, the-lane-redoes-a-lost-merge, blocked by this one. This page serves contracts 1 and 2 of the seat's actors-and-contracts model (one record read by all; the next act runnable by its reader) and takes D7 in Wido's name under R-142 and R-143; Wido may overrule with one line.

Written 2026-11-05 by opus-designer (Claude Opus 5.5), split by a1 under decision 202 from the deleted joint page `the-lane-finishes-its-own-resolution` revision 2. The capacity wait and the redo of a lost merge are `plans/designs/the-lane-redoes-a-lost-merge.md`, blocked by this page. Decision numbers are kept from the joint page; Astra's rounds 1 and 2 are folded (Critique record).

Code: the m1e main checkout at `f3a5db89f0`; every `file:line` was read there. Findings: `agentic-tools-evidence/lane-test-20261009/findings.md`, entries 523 to 525, 529 and 537.

## 1. Intent

A green resolution job is collected, even with the engine's own records beside it; a real edit still refuses. A refused launch names a remedy for its cause and reader. A person who asks for the merge build gets it, told what it does. No agent rebuilds its engine. This serves contracts 1, 2 and 3 of `agentic-tools-evidence/lane-test-20261009/designs/the-seats-actors-and-their-contracts.md`: one record read by all; the next act is runnable by its reader, or one ask and the end of the turn; waits belong to the engine.

## 2. Problem

- Findings 524 and 525. On 11-04 host admission refused the lane's resolution launch (load 16.55, limit 8) and told a person to run "the original work build command" from an enrolled terminal. Wido did; it was refused: "only the lane's current session builds its merge".
- Finding 529. The resolution job finished green. Collecting it refused "the check left unstaged changes"; the only unstaged file was `metasystem/records/narrator-digest.log`, written by the lane steward's tick. The agent followed the hint, `landing resolve`, which returned the goal.
- Finding 537. The agent rebuilt the lane's engine with `go run ./cmd/devgate build` and ran `session start` on its own.
- Finding 523. The lane steward recorded a person's give-back as "took no effect", although it emptied the batch.

What the code does:

- The collection judges every unstaged path. `commitResolvedMerge` (`metasystem/internal/landing/plain/resolve.go:581`) refuses on any worktree change: `git diff --quiet --` (`:631-633`). Git runs from the checkout, so paths come back checkout-relative (`metasystem/records/...`). `fix.Paths` are the unmerged paths only (`--diff-filter=U`, `resolve.go:88`). The coordination paths are named once, installation-relative: `WorkspaceExclusions()` (`metasystem/internal/landing/registers.go:64`; the registers `:13`, `:25-28`; the ledger paths `:41-47`). The workspace projection adds the installation prefix before matching (`registers.go:101-108`); `plain` has `installationPrefix` (`resolve.go:488-497`). `plain` cannot import `internal/landing`, which depends on it. `ResolveSeams` (`resolve.go:26-35`) is zero-valued in production (`metasystem/cmd/metasystem/intent_landing.go:77`), passed at `metasystem/cmd/metasystem/landing_merge.go:93` and `metasystem/cmd/metasystem/intent_landing_resolve.go:43`.
- The failed collection names the returning verb. The answer starts with `next: sameCommand`, `nextReason: "collects this same job"` (`landing_merge.go:51`); on any error `next` becomes `landing resolve` and the reason stays (`:234-239`). `landing resolve` without `--as-seat` aborts the merge and returns the goal by design (`intent_landing_resolve.go:17-20`; `resolve.go:219-226`). A launch refused for load reaches the same line (`metasystem/internal/landing/plain/launch_record.go:35-41`).
- One remedy for five causes. For an agent, `refuse` (`metasystem/internal/launch/hostcapacity.go:111-113`) attaches `humanauthority.PersonActRemedy("the original work build command")` (`metasystem/internal/humanauthority/authority.go:1149-1154`) to `host.builds=person` (`:114-116`), an invalid `host.load-max` (`:117-119`), unknown capacity (`:120-122`), measured overload (`:123-125`) and the build cap below. A person is admitted at once, past the load gate (`:83-85`). The facts carry `load=` and `limit=` (`:107`). The register says the same (`metasystem/internal/refusal/register.go:74-75`).
- A person cannot build a lane merge (`landing_merge.go:41-43`; `landingFixActor`, `metasystem/cmd/metasystem/landing_fix.go:19-38`). The job id is derived from the attempt (`landing_merge.go:58`); an existing job is read at `:62` outside the lane lock and reused (`:62-68`). `landing resolve` already reads an enrolled person (`landingPersonCheck`, `intent_landing_resolve.go:46-48`). `--reason TEXT` exists on `work build` (`metasystem/cmd/metasystem/intent_work.go:285`).
- The skill has no rule for a refused launch, a refused collection, or rebuilding (`metasystem/skills/landing-agent/SKILL.md:24-25`, `:150-171`, `:184-187`). `landing deploy` is a person's verb (`metasystem/cmd/metasystem/intent_landing_deploy.go:14`).
- The effect check counts only an agent step or a proof start after a person's act (`metasystem/internal/landing/plain/health.go:223-264`); a give-back removes the member (`metasystem/internal/landing/plain/giveback.go:73-80`).

## 3. Decisions

- **D1 The collection ignores only coordination paths outside the merge.** `ResolveSeams` gains `Coordination []string`; the command fills it from `landing.WorkspaceExclusions()` at both call sites. Before matching, `plain` makes each entry checkout-relative with `installationPrefix(install, checkout)`, as `registers.go:101-108` does. In `commitResolvedMerge`, `git diff --quiet --` becomes `git diff --name-only -z --`. The merge's paths are `fix.Paths` plus every path the goal side changes (`git diff --name-only -z <merge-base HEAD MERGE_HEAD> MERGE_HEAD`). An unstaged path is ignored only when it is under a coordination prefix and is not a merge path. Every other unstaged path refuses, named: "the check left unstaged changes in PATHS; nothing was committed". A nil `Coordination` ignores nothing (fails closed, as today). Ignored bytes stay, uncommitted. One function, `coordinationOnly(prefix, policy, mergePaths, paths)`, which the redo page reuses. In the failure path (`landing_merge.go:234-239`) `nextReason` is set with `next`: while the fix is `resolving` and its job completed, `next` stays the same build command, "collects this same job once the cause above is gone"; `landing resolve` is named only for a job that ended without completing, "returns the goal with its paths".
  Rejected: all of `records/`; it hides review evidence (`metasystem/records/reads`). Rejected: matching installation-relative names against checkout paths; the incident's file would still refuse.

- **D2 The remedy follows the admission cause.** `refuse` takes a cause: `person-policy`, `invalid-limit`, `unknown-capacity`, `overload`, `build-cap`. `StartSpec` gains `LaneMerge bool` (`metasystem/internal/launch/launch.go:54-72`), set at the lane's start (`landing_merge.go:81`). For a lane merge: `overload`, "the lane waits until the one-minute load is below host.load-max=LIMIT and retries this same command"; `build-cap`, "the lane waits for a build place and retries this same command"; `unknown-capacity`, "the lane retries when host capacity can be read: CAUSE"; `person-policy`, "host.builds=person: a person runs this build from an enrolled terminal" (true by D7); `invalid-limit`, "a person declares a positive host.load-max". Other specs keep `PersonActRemedy`. The refusal carries `cause=` beside `load=` and `limit=`. The register rows name the lane variants. In `laneMergeWork`, a `LAUNCH_BUILD_CAPACITY` refusal keeps `next` as the same build command.
  Rejected: one load-retry text for every cause. Rejected: exempting the lane from the load gate, a host-wide rule.

- **D4 The agent ends its turn on a refused launch and never rebuilds its engine.** Skill text. Beside the forbidden list (`SKILL.md:24-25`): never run `devgate build`, `go build` or `go run` of the engine, `session start`, or an install step; a stale engine is reported by `landing status` and `landing deploy`, a person's verb, is the only route. Case 4 (`:164-171`, `:184-187`): a refused launch ends the turn, with no loop, no sleep, no reading of the load and no question; a refused collection is not a job that cannot resolve, and the agent follows the refusal's `next`. Until the redo page's capacity wait lands, two such turns are barren and the keeper holds for a person (`metasystem/internal/landing/lane/agent.go:45`, `:487-513`); D7 is that person's route.
  Rejected: an agent loop with a count; waits belong to the engine (contract 3). Rejected: the agent deploying its own engine; it runs under that binary.

- **D6 A give-back that empties the batch is its own effect.** In `buildHealth`, after the act is chosen (`health.go:223-238`) and before the check (`:259-264`): when the act is a person's give-back and the batch has no members, `progressed` is the act's time. A give-back that leaves members still needs an agent step or a proof start.
  Rejected: count any batch write as progress; a reopen or restart writes the batch and still needs the agent.

- **D7 A person's merge build is accepted, after he is told its impact.** Decided by this page in Wido's name under R-142-m1e (`metasystem/memory/rulings.md:201`) and R-143-m1e (`:202`). In `laneMergeWork` (`landing_merge.go:41-43`), a caller who is not the lane's session but passes `landingPersonCheck` is accepted. Before the start, the verb prints the impact in plain words: "This builds the lane's merge of G in the shared lane checkout, now, past the host load limit (load L, limit N). The lane agent collects it; a build by anyone else joins this job. To undo: metasystem landing give-back G." The build then goes through the same owner: under the lane lock, the job status read and the start move into `LaunchResolution`'s locked section (from `landing_merge.go:62-68`), with the attempt-derived id, so a running job is joined, never doubled. The start is a person build (`Actor: "person"`). The launch record gains `Person *ActProvenance`, `Reason` and `Impact`: the person's provenance, `--reason TEXT` (the existing flag) or "no reason given", and the exact impact text shown. Any other caller is refused as today. Wido can overrule.
  Rejected: keeping the refusal; it stops a person's decision. Rejected: a required reason.

## 4. What does not change

- The load gate; `landing resolve` without `--as-seat`; `landing resume`; the barren hold; the marker check (`resolve.go:612-630`); the fresh-resolution guard (`resolve.go:176-186`, changed by the redo page).

## 5. Units

Production lines are additions plus deletions; a replaced line counts twice. Counted by file at `f3a5db89f0`, then doubled in the table (estimates run double). Stop line 250; split at 110.

| Unit | Decisions | Production lines | Tests (estimate) | Split seam | Step value |
|---|---|---|---|---|---|
| U1a the collection | D1 | 100 | 130 | none needed | a green resolution with engine records beside it is collected; a real edit still refuses |
| U1b the causes | D2 | 84 | 140 | none needed | each lane refusal names a remedy that fits its cause |
| U2 the person's merge build | D7 | 84 | 120 | none needed | a person's merge build runs, joins the lane's job and is recorded with its impact |
| U6 the effect check | D6 | 12 | 40 | none needed | an emptied batch is not reported as a person's act without effect |
| U5 the skill text | D4 | 0 | 0 | none needed | the agent ends its turn on a refused launch and never rebuilds its engine |

- U1a, 50: `resolve.go` 36 (the seam field 2; `coordinationOnly` with the prefix conversion 12; the unstaged list 6; the goal-side paths 10; the named refusal, 3 replaced lines 6); `landing_merge.go` 12 (2 replaced lines 4, plus 8); `intent_landing_resolve.go` 2.
- U1b, 42: `hostcapacity.go` 30 (5 call sites replaced 10; the lane remedy table 14; the cause fact 2; the build-cap call 4); `launch.go` 2; `landing_merge.go` 6; `register.go` 4.
- U2, 42: `landing_merge.go` 31 (the person branch, 3 replaced lines 6; the person check 7; `Actor` 2; the impact text printed and kept 10; the status read moved under the lock 6); `launch_record.go` 11 (three fields 3; the locked read and join 4; the write 4).
- U6, 6: `health.go` 6.
- U5: `SKILL.md` lines 24-25, 164-171, 184-187, about 12 lines of text, plus the registration copies `metasystem system setup` refreshes in the same commit (finding 533; `TestShippedRepositoryHostRegistrationsAreConfigurationReady`) and the skill audits.

Order: U6, U1b, U1a, U2, U5. Reverse dependents run (R-18): `CompleteMerge` (`resolve.go:566`), `createAdmitted`'s callers.

## 6. Tests

New or changed behaviour only, through the production composition with injected clocks and probers (R-104-m1e). Each behavioural assertion fails on main, not only at compile time.

- T1 `TestACollectionIgnoresOnlyCoordinationOutsideTheMerge` (U1a, `metasystem/internal/landing/plain/merge_fix_test.go`, real git, the installation nested under `metasystem/`). (a) `metasystem/records/narrator-digest.log` and `metasystem/memory/receipts.log` unstaged: committed, both still unstaged with their bytes. (b) `metasystem/records/reads/x.json`: refused, named. (c) A coordination path the goal side merged cleanly: refused. (d) `metasystem/internal/x.go`: refused. (e) Nil policy with (a)'s files: refused. First failure on main: (a) refuses. Mutation: skip the prefix conversion; (a) refuses and fails.
- T2 `TestARefusedCollectionNamesTheSameCommand` (U1a, `metasystem/cmd/metasystem`, beside `TestLaneMergeGuardRealGitCommit`). A refused collection: `next` is the same build command, the reason does not name `landing resolve`. A job that ended without completing: `landing resolve`, "returns the goal with its paths". First failure on main: `landing resolve` with "collects this same job". Mutation: `landing resolve` for every error; the first case fails.
- T3 `TestALaneRefusalNamesTheRemedyOfItsCause` (U1b, `metasystem/internal/launch`). On a `LaneMerge` spec: overload (load 30, limit 24) says "below host.load-max=24" with `cause=overload`; `host.builds=person` names the person's build, no load words; the build cap saturated says "a build place"; unreadable capacity says "can be read", no limit; `host.load-max=0` says "declares a positive host.load-max". A seat spec keeps `PersonActRemedy`; `laneMergeWork` keeps `next` the same command. First failure on main: every case names the person route. Mutation: map every cause to the overload text; the person-policy case fails.
- T4 `TestAPersonsMergeBuildIsToldItsImpactAndJoinsTheLane` (U2, `metasystem/cmd/metasystem`). A resolving fix, no job, load 30 against 24. An enrolled person builds: the impact text is printed before the injected manager's start is called (the start seam records the output length it saw); one person build starts; the launch record holds the provenance, "no reason given" and that exact impact. The lane session's build: the same job, no second start. Neither caller: refused as today. First failure on main: refused "only the lane's current session builds its merge". Mutation: print the impact after the start; the ordering assertion fails.
- T5 `TestAGiveBackThatEmptiesTheBatchHasTakenEffect` (U6, `metasystem/internal/landing/plain/steward_rows_test.go`). The only member given back, the agent barren, 61 minutes later: no `person-act-no-effect`. A second member left and no step: the row fires. First failure on main: the row fires for the empty batch. Mutation: count any give-back as progress; the second case fails.

U5 has no test of its own; the skill audits and the registration check run.

## 7. Moved effects

| Effect | From | To | Code |
|---|---|---|---|
| judging a lane checkout change as the check's leftover | every unstaged path (`resolve.go:631-633`) | every unstaged path except checkout-relative coordination paths outside the merge (U1a, D1; T1) | `metasystem/internal/landing/plain/resolve.go:631-633` |
| naming the next step after a refused lane merge collection or launch | `landing resolve` (`landing_merge.go:234-239`) | the same build command, `landing resolve` only for a job that did not complete (U1a, U1b; T2, T3) | `metasystem/cmd/metasystem/landing_merge.go:234-239` |
| the remedy of a refused lane launch | the person route for every cause (`hostcapacity.go:111-113`) | the remedy of the cause (U1b, D2; T3) | `metasystem/internal/launch/hostcapacity.go:111-125` |
| building a lane merge on a person's request | refused (`landing_merge.go:41-43`) | accepted after its impact is shown, joined under the lane lock, recorded (U2, D7; T4) | `metasystem/cmd/metasystem/landing_merge.go:41-43` |
| judging a person's give-back as without effect | an agent step or a proof start only (`health.go:240-264`) | also the emptied batch (U6, D6; T5) | `metasystem/internal/landing/plain/health.go:259-264` |

## 8. Triggers

- Briefed when accepted (a1: after these folds, no third round). The redo page follows U1a.
- Re-read if Wido overrules D7 or the host admission rule changes.

## Critique record

- Round 1, 2026-11-05, Astra on the joint page `the-lane-finishes-its-own-resolution`: 7 material, 1 minor, redesign. Here: F1 to D1 (explicit policy, all merge paths); F2 to D7; F7 to D2. F3 to F6 and F8 went to the redo page.
- Round 2, 2026-11-05, Astra on the joint page, log `agentic-tools-evidence/lane-test-20261009/designs/astra-lane-resolution-r2.log`: 7 material, 1 minor, redesign. a1 split the page (decision 202). Here: F6 to D1 and T1 (checkout-relative policy, nested installation); F7 to D7 and T4 (impact shown before the start, kept with the provenance; reason optional, "no reason given"); F8 to the whole page (each decision and ruling once; the Wido section, the rulings list and the repeats are gone). F1 to F5 went to the redo page.
