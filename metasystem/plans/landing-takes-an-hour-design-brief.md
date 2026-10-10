# Design brief: landing-takes-an-hour (plan goal 8, before switch-on)

Working Mode: Design
Goal: landing-takes-an-hour. Tier 3, priority 1, before the machinery is switched on (Wido 2026-10-08 17:25: "we need to change our behavior immediately because there's lots to be gained. So this is a goal that needs to be done before the machinery is switched on"). Coordinated by m1e in Wido's word.

Read first: plans/machinery-open-findings-plan-2026-10-06.md "Learned 2026-10-08: goal 2's landing took 6h44m" (the measured basis and the lesson table L1-L7), the accepted designs plans/designs/lane-lands-finished-goals.md (1a, done), plans/designs/lane-reproves-only-what-a-change-can-affect.md (done: the impact selection), plans/designs/review-chain-stops-and-records.md Decision 6 (the declared check, on main at 4c0a5aadc), plans/designs/fleet-survives-its-providers.md P5 Part B (host.builds and host.load-max, on branch goal/fleet-survives-its-providers until it lands tonight), plans/designs/process-changes-cover-declarations-and-interventions.md U2a (cheap stays cheap: the preview and the deep hold stay there), and the code sites below (read on origin/main; cite the site you build against).

## Intent

A goal lands in about an hour: merge main, one gate, at most one targeted fix round, push. Goal 2 took 6h44m: 156 reds at the first gate, all pre-existing tests of packages its 25 unit commits had changed and never run; a panic hid 34 more; three full gates of 40-54 minutes, one under self-inflicted load; fix-forward as one two-hour job. The lane as designed runs the same loop. This goal changes the loop in the machinery and in the hand practice at the same time.

## What is true today (read, with sites)

1. The gate: `cmd/devgate/gate.go:207` `full` runs vet, then `./internal/...` (`:387` lists it) and `./cmd/...` in series; one `go test` per tree, so a panic in one cmd test kills the package binary and hides every later result; nothing shards cmd/metasystem; nothing holds builders off the host while it runs.
2. The lane's proof: `internal/landing/plain/prove.go` runs the declared proof once per batch; `red.go:48` `ClassifyAttempt` and `replay.go:17` `classifyRed` classify a red attempt as a whole and return one outcome to the seat; lane-reproves' selection (done) narrows the reprove after a fix to what the fix can affect.
3. The unit check: `cmd/metasystem/intent_unit_check.go` `resolveUnitCheck` freezes proof.cheap and proof.audits per round from the committed declarations (goal 2 Decision 6); the shipped default of proof.cheap and the COMMON brief's Check ran only the unit's own new and changed tests until 10-08 17:15, when the brief block was amended by hand (plans/lane-policies-and-helm-u1-settings.md).
4. host.builds / host.load-max (fleet P5b) admit a build against current load; nothing yet lets a proof claim the host.
5. No record says how long a landing took or where the minutes went.

## What the design must deliver (the units; re-cut allowed, intent not)

| Unit | Intent | Lines |
| --- | --- | ---: |
| U1 the gate survives a panic and runs in parallel | devgate gate shards cmd/metasystem by test-name ranges (one `go test -run` per shard) so a panic loses one shard, never the package; internal/... and cmd/... run in parallel; the gate prints one package line per package and shard and counts only `ok` lines as green (a panic or a missing line is red); a hand gate and the lane's proof use the same command. | 200 |
| U2 a proof holds the host | while a gate or the lane's proof runs, host.builds is held at 0 by the proof (P5b's admission refuses with "the proof holds the host until <time>", builders queue, never fail); released when the proof ends or dies; status shows the hold. | 150 |
| U3 reds return per package | red classification groups a red attempt's failures by package and cause and returns one fix unit per package to the seat, buildable in parallel; the reprove after fixes runs only the failed packages plus the importers of each fix (lane-reproves' selection); the full proof runs once more only when every package is `ok`. | 220 |
| U4 the unit check includes what the unit changed | the shipped default of proof.cheap, and the Check the brief composer prints, are the unit's own new and changed tests plus the test package of every package whose production code the unit changed (whole for internal/*), plus cmd/metasystem tests selected by lane-reproves' impact selection from the changed symbols, verbs and fixtures, never the whole cmd package; a red there is the unit's to fix; the declared check (resolveUnitCheck) freezes that selection per round. process-changes-cover U2a keeps the preview and the deep hold. | 200 |
| U5 the landing clock | a landing records merge, gate (per shard), fix rounds and push with their minutes on the goal's record and in landing status; `goal status` shows the last landing's minutes; this is the measure of the goal. | 120 |

Build order: U1, U5, U4, U3, U2 (U2 after fleet-survives-its-providers lands). Each unit at most 250 production lines, at most 5 units; one build, at most two corrections; each unit's check per the amended COMMON block; the full gate once at landing, with U1 in it.

Split after critique round 1 (2026-10-08): the Check text the brief composer prints (U4's clause "the Check the brief composer prints") moves to the follow-up brief the design names under "Split off after round 1" (R-148-m1e); U4 keeps the selection, the verb, the declaration and the frozen base on both callers of the check.

## Not in this goal

The flake classification and repeat (machinery-housekeeping, done); the cheap-check preview, the deep-selection hold and the changed-assertion audit (process-changes-cover U2a); the load-fragile landing test (housekeeping follow-up); design size admission (review-drops).

## Readers of what this goal changes

`cmd/devgate/gate.go` and its tests; `internal/landing/plain/{prove,red,replay,batch}.go`, `skills/landing-agent/SKILL.md`; `cmd/metasystem/intent_unit_check.go`, `internal/launch/unit_check.go`, the brief composer (`cmd/metasystem/intent_work.go` briefScaffold) and `internal/protocol/templates/brief.md`; `internal/config/defaults.go` (proof.cheap default); `internal/launch/hostcapacity.go` and fleet P5b's admission; `internal/goal/` records and `cmd/metasystem/intent_landing.go` status; tests pinning today's gate (`cmd/devgate/gate_test.go`) and the lane (`internal/landing/plain/*_test.go`).

## Acceptance of the goal

- Two consecutive goal landings measured by U5 at 60 minutes or less from "every unit committed" to pushed, every package and shard `ok`.
- A gate with a deliberately panicking test still reports every other package and shard.
- No builder starts under a running proof; the refused builder's remedy names the hold's end.
- A unit that changes `internal/launch` runs `internal/launch`'s tests in its own check and goes red there, not at landing.
- The full cmd/metasystem package is green on the integrated tree.

## Questions the design may put to Wido

Only ones whose answer changes what is built. Name the mechanism and the two options.
