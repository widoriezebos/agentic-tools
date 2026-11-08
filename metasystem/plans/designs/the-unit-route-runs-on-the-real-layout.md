# The unit route runs on the real layout

- Kind: design
- Id: the-unit-route-runs-on-the-real-layout
- Status: accepted
- Goals: the-unit-route-runs-on-the-real-layout
- Critique: round 1 (Astra, designs/astra-unit-route-gaps.log; 6 material) folded in revision 2 of designs/unit-route-gaps-2026-11-08.md; round 2 (designs/astra-unit-route-gaps-r2.log; 1 material, D5b only) folded in revision 3; converged (decision 234)

Accepted 2026-11-08 by a1 in Wido's name (Wido 20:0x: "analyze the causes, then design fixes and get these critiqued by Astra").

Source analysis: designs/unit-route-gaps-2026-11-08.md (causes with file:line cites at main e8a9cefe05, two Astra rounds). Findings 568, 569, 570, 571.

## Problem

Four defects of the unit route surfaced today, each by a live unit that stalled and needed a person: the unit check ran from the caller's folder (568); a stopped unit kept its name and its kept change hid the next build (569); a committed read's material finding could not be collected, so the round ended "unknown read inputs" (570, six rounds this month); the amend gate ran at the candidate's top (571, hot-fixed at e8a9cefe05). Common root: the rare routes (an explicit `--check` from the checkout top, stop and rebuild, a committed examination, an amend) have no end-to-end test on the production layout, where the installation is `metasystem/` under the repository top; the quick beds put both in one folder, and the schema contract between the examination producer and the stop's collector is tested per side, never across.

## Decisions

**D1 (568) Two folders, named apart.** The plan keeps two directories. The runner directory, `worktree/<InstallationRel>` of the plan's worktree, is where the engine's own checks run: the `unit-check` step and the base comparison (`base-M/<InstallationRel>`); it is recomputed from those two facts at every round and sets `plan.Check.Directory` (the one the child commands use, cmd/metasystem/intent_unit_check.go:35 and :132); it is never read from `Proof[0].Dir` or the caller's cwd. The manual directory is the caller's folder and applies only to a `--check` command the caller typed. Today both are one field (cmd/metasystem/intent_work.go:1137-1152 selects the installation only when `--check` is absent). Acceptance, flat and nested layouts: explicit `--check` from the checkout top; the default check from the top; each followed by a revise; in every row the frozen `Check.Directory` and the executed `base-M` path end in the installation, and the manual command's folder is the caller's only in the explicit rows. Mutation: deriving the runner directory from `Proof[0].Dir` fails the explicit-from-top rows.

**D2 (569) A cancelled name rebuilds; only the stopped unit's own change is adopted.** `work build` of a name whose run is cancelled and holds no live mutation moves the named entry and `.inputs/<key>` aside into the cancelled run's directory (`released-named.json`), reserves a fresh run, and says so ("unit U3 of run R was stopped; this build starts run R2"). A live run's name stays refused (UNIT_NAMED_INPUT_CHANGED); a reserved-but-unrecorded run keeps resuming its own id (internal/launch/unit_named.go:294); a missing run record stays UNIT_NAMED_RUN_MISSING. Adoption is narrow: before the fresh run's round 1 the engine compares the worktree's diff against the goal tip with the cancelled run's last recorded `worktree.diff`. Equal: the round's `build-before` is the goal tip, so the stopped unit's retained change counts as this round's change, and the output says "run R's change (N lines) is this round's starting result". Not equal: the truthful before-snapshot stands and the output names the paths that differ from the stopped unit's change, so an idle builder never claims them; the DONE allowance of internal/launch/unit_build_outcome.go:46 is unchanged. Acceptance: stop, rebuild with a new brief: a fresh run; identical kept change counted and committed; kept change plus unrelated dirt: not counted, the dirt named; a live name refused; a reserved-unrecorded name resumes. Mutation: adopting on an unequal diff fails the dirt row.

**D3 (570) One schema for the code critic; an unreadable read says why.** (a) Producer: the examination hands `code-critic` return schema version 6 (`returnSchemaVersion(role)` in internal/adapter/supervisor/lifecycle.go), the version live reads already use and `VersionSixRoles` already names; design-critic and warden keep 4. (b) Packet: the code critic's packet line names version 6 (internal/protocol/role-packets.json, roles/code-critic/artifactMember). (c) Collector: a present-but-malformed return is a classified stop on both read paths: class "the read cannot be read: <collect error>", handoff "stopped unreadable-read", evidence the return path; `collectRoundRead` (internal/launch/unit_stop.go:130) records the collect error instead of discarding it, and `CollectExamination` does the same; `work review` offers the retry only for transport causes (no return, a compaction), never for a present malformed return. Proof across the contract: one test materializes the code critic's schema, shapes a return from its required members with one material finding, and runs `readsubject.Collect` on it: material count 1. (a), (b) and that test are the hot-fix of 2026-11-08 evening (Astra round 2: scope holds); (c) is unit U1. Acceptance: an examination journey whose critic returns one material finding ends in "new material findings" with a `work revise` offer; a version-4 material return ends in the unreadable-read stop naming the schema, with no retry offer; a live round read with a malformed return ends the same way. Mutation: discarding the collect error leaves "unknown read inputs".

**D4 (571 and the layout) The journeys run nested.** The connection bed gains a layout parameter (the move of `readPublicationAdapterBed`: the installation under `<repo>/metasystem`, the Git top at `<repo>`, the resolver answering the top). Four journeys run on both layouts through one table: the amend publication (TestIntentBuiltUnitToLanding's amend half), stop and rebuild (D2), the examination with a material finding (D3), and the check folder (D1). The bed's gate stub (cmd/metasystem/intent_unit_review_test.go:266) today ignores its directory and always passes, so the amend row records the directory the gate was called with and asserts its base name is the installation. The other sixteen journeys stay flat. Acceptance: the four pass nested on the fixed tree; reverting the 571 hot-fix fails the amend row nested and passes it flat.

## Units

| Unit | Production lines | Total lines | Last | Content |
|---|---|---|---|---|
| U1 | 70 | 190 | no | D3 (c): the classified unreadable-read stop on both collectors; the retry only for transport causes; the examination journey rows |
| U2 | 90 | 230 | no | D1: runner and manual directories; the acceptance table on both layouts |
| U3 | 130 | 280 | no | D2: release of a cancelled name, narrow adoption; seam: the release first, the adoption second |
| U4 | 60 | 240 | yes | D4: the layout parameter and the four nested rows; the gate stub records its directory |

Estimates run double; a unit whose first round measures over 180 production lines splits at its seam. Every test runs on injected clocks (R-104-m1e); no test runs the Go toolchain outside the slot; no background loops; no `rm` on a variable path.

## Order

U1 first (priority one's remaining committed reads go through it), then U2, U3, U4. Pinned m1e behind the-lane-finishes-what-it-starts.
