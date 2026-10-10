# Unit U3 of lane-proves-at-the-batch-risk: a gate runs only when cheap; `last_gate` is bound to HEAD; reuse needs the same base

Working Mode: Implement. Design: `plans/designs/lane-proves-at-the-batch-risk.md`, decisions D3 and D4 (and the critique notes F3, F5 they answer). Builds on U1 and U2 (the `impact` scope and the cheapness rule exist). Size: at most 250 production lines.

## What exists

- `internal/landing/plain/gate.go` ~50-70: the per-merge gate compares against the merge's first parent (`decision.Base`, `decision.BaseCommit`).
- `internal/landing/plain/status.go` ~312-316: `last_gate` is the latest gate record, not bound to the current HEAD.
- `skills/landing-agent/SKILL.md` ~60-73 and cases 1, 2, 5: after every merge `landing prove --gate --wait`, read `last_gate`, green -> continue, else hold; then `landing prove`, push when green.

## Build

1. Every `landing prove --gate` first runs the `fast-static-build` group (today's cheap runner excludes it, test_impact.go:86-96) and records that group's result on the gate record; its red is a gate red. Then the cheapness rule (U2's function) applies to the merge's own selection: when not cheap the gate records state `skipped` with the reason (`impact covers 97%; the batch proof follows`) instead of running the selection. Every gate record carries `Requested` (the merge commit at the request) and `Attributed` (the commit the check ran or failed on: the merge, or its first parent when the baseline check failed, gate.go:80 / prove.go:1126).
2. `landing status --json` exposes `last_gate` by `Requested == HEAD`: state `green`, `red` (with `Attributed` and the cause, so a baseline failure on the parent stays visible as this gate's red naming the parent), `skipped`, or `none` (no gate requested for this HEAD; a stale gate from an earlier HEAD reads as `none`, never as the outcome).
3. Reuse (D4): the batch proof at `impact` depth reuses the last gate attempt only when it is green on the same tree, same depth, same environment fingerprint, its `fast-static-build` group recorded green, AND its comparison base equals the batch base; it records `proven by attempt X`; otherwise it runs.
4. Skill: cases 1, 2 and 5 treat `skipped` like `green` for the merge step, `none` as "the gate has not run for this merge", and say "push when the proof at the batch's depth is green"; every phrase on the traveling surface qualified (the audit refuses bare "this gate"/"its job"); `TestSkillLandingAgent*` updated one for one.

## Tests

- Not-cheap merge: gate result `skipped` bound to HEAD with reason, static still ran (mutation: skip static -> a static red is missed, test fails).
- `last_gate` for an absent record and for a stale red from an earlier HEAD both read `none` (mutation: return the historical record); a baseline failure on the parent reads as this gate's `red` with `Attributed` = the parent (mutation: filter by the attributed commit -> `none`, test fails).
- A one-member gate with tests green and the static group red: no reuse, push refused (mutation: drop the static condition).
- Reuse admitted for a one-member batch (gate base = batch base) and refused for a two-member batch whose last gate base is the first member's tip (mutation: drop the base comparison).
- Skill tests for the three states; `go test -count=1 -timeout 30m ./internal/audit` green.

Then `go run ./cmd/devgate static`, `./internal/landing/plain`, cmd tests by name (`TestLandingGate|TestLandingProve|TestLandingStatus|TestReadStatus|TestSkillLandingAgent`), every existing test using a changed seam. Never edit testing.json; never open metasystem.conf.local. Report: diff --stat, each exit, the status JSON for the three states.
