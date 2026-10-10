# Unit U3 correction 1: a dead gate never reads green; a static red at the merge is classified like a test red

Working Mode: Implement. Correction of unit U3 (brief `plans/lane-proves-at-the-batch-risk-u3-gate-and-reuse.md`, design D3/D4) after its Opus read. The worktree holds U3 uncommitted on top of U2 (02720e1c9); change only what this brief names.

## F-1 (material): a gate that dies after its baseline check passes shows as green

`gate.go` ~42 builds the baseline (parent) record with `Requested: running.Commit` (the new merge, HEAD); ~84-85 change only Commit, Tree and Attributed to the parent; ~133-134 append that green to gates.jsonl. When the gate process then dies, `checkState` (`prove.go` ~323-344) appends a dead-gate red with NO Requested, so `landing status` (`status.go` ~320-325, newest record with Requested == HEAD) shows the baseline's green as HEAD's `last_gate: green`; the same wrong green shows while the gate still runs its own tests. Fix: the baseline record's Requested is the parent commit (never HEAD); the dead-gate red (and any gate red written by checkState) carries Requested = the merge it was requested for; `last_gate` therefore reads `none` while the gate runs its own tests? NO: while the gate runs, `last_gate` reads `running` (add that state) or `none`, never the baseline's green. Test: baseline green recorded, gate process dies -> `last_gate` is `red` (dead gate) for HEAD, not green (mutation: keep Requested = HEAD on the baseline -> green, test fails); while running -> not green.

## F-2 (material): a static red at the merge is never attributed, so the lane asks a person

`gate_impact.go` ~30-38 returns `Cause{Kind:"unclassified"}` with no replay and no `repeat: allowed` for a static red at HEAD; the skill starts a fix round only for an `own` cause and re-runs only on `repeat: allowed`, so a merge that breaks vet or static now stops the lane for a person, where before U3 the same break failed as a test and replay classified it `own` for an unattended fix round. Fix: a static red at the merge goes through the same attribution as a test red: run the static group on the parent (the baseline tree, as the replay does for tests); parent green -> cause `own` (the batch's: the fix round runs); parent red -> cause `main`; the environment/repeat rule as for tests. The impact-depth batch proof classifies static reds through `classifyRed` already: the gate must agree with it (one path). Test: merge with a vet error, parent clean -> gate red cause own (mutation: unclassified -> test fails); parent also red -> cause main.

## F-3 (not material, one line): a failed plan read is a retryable environment red

`gate_impact.go` ~61-64 records an environment red without `repeat: allowed`; use `allowEnvironmentRepeat` as the impact proof does (`prove.go` ~1176-1182).

## Checks

`go test -count=1 -timeout 30m ./internal/landing/plain ./internal/audit`; cmd tests by name (`TestLandingGate|TestLandingProve|TestLandingStatus|TestReadStatus|TestSkillLandingAgent|TestLandingPush|TestLandingDepth|TestLandingImpact|TestLandingReplay|TestLandingCause`); every existing test using a seam you changed (grep, by name); `go run ./cmd/devgate static`. Never edit testing.json; never open metasystem.conf.local; do not commit. Report: diff --stat of the correction, each exit, the three scenarios before/after.
