# Unit U1 correction 1: the scratch clone proves with fixture-only commands, so the green scenario finishes inside the budget

Working Mode: Implement. Correction of unit U1 of lane-rehearsal-before-landing (brief `plans/lane-rehearsal-before-landing-u1-green-rehearsal.md`, design `plans/designs/lane-rehearsal-before-landing.md` with the amendment of 02:10). The worktree holds U1 uncommitted (your own handoff `plans/handoff-lane-rehearsal-before-landing-u1.md` records the state; the previous job was stopped by the steward after two hours of repeated ten-minute runs). Change only what this brief names; leave fixes uncommitted.

## The measured contradiction

Your handoff: bootstrap full proof 264 s, merge gate 289 s and cancelled at 600 s; earlier setup up to 411 s of build. Every proof step of the scratch clone ran this repository's own static check, build and groups. The design never wanted that: the lane under test is the candidate ENGINE; the scratch repository's proofs only need to be real in shape.

## Decisions

- D3 widened: the harness commit pushed to the scratch origin overrides, in the scratch clone only, the proof declarations: `proof.full` and `proof.cheap` run `go vet ./internal/rehearsalgreen/... && go test -count=1 ./internal/rehearsalgreen/...` through the existing reporter protocol (LANDING-FAILED/LANDING-CHECKED lines, LANDING_ONLY honoured) — reuse `proof/full.sh`'s reporter if it can be pointed at the fixture groups cheaply, else a small `proof/rehearsal.sh` in the harness commit that emits the protocol itself. The scratch testing.json's static/build group (`fast-static-build` or whatever the contract names first) runs that same cheap command. Nothing changes in this repository's own proof declarations or testing.json.
- D5 kept: the whole green scenario (setup, bootstrap trunk proof, hand-in, merge gate, batch proof at impact depth, push, landed status) completes in under ten minutes on this machine. The rehearsal record (`records/rehearsals/<tree>.json`) carries the breakdown in milliseconds: setup, bootstrap, hand-in, gate, proof, push, total; `landing rehearse` prints the total and the slowest step.
- Setup: when the candidate checkout already holds a built `bin/metasystem` whose recorded build tree equals the candidate tree, copy it instead of rebuilding (saves up to 411 s); otherwise build once and record the time.
- The source static complaint about the design's goal missing from the ledger is expected until the goal is opened; do not try to resolve it.

## Acceptance

One real run of `metasystem landing rehearse` (or its test) on the candidate = this worktree's tree, printing a pass with the breakdown; one negative run (count canaries in the cheapness measure) failing before push. Report both wall times.

## Checks

`go vet ./...`; `go test -count=1 -timeout 30m ./internal/landing/rehearsal/... ./internal/landing/plain ./internal/layering`; `go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestLandingRehears|TestAudit'`; `go run ./cmd/devgate static` (ignore only the design-ledger goal complaint). Never edit this repository's testing.json; never open metasystem.conf.local; do not commit, push or rebase this checkout (scratch repositories are yours). Report: diff --stat of the correction, each exit, both rehearsal wall times with breakdown.
