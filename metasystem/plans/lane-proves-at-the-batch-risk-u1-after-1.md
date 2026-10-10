# Unit U1 correction 1: an impact environment error stays retryable

Working Mode: Implement. Correction of unit U1 of lane-proves-at-the-batch-risk (brief `plans/lane-proves-at-the-batch-risk-u1-impact-scope.md`, design D1a/D1b) after its Opus read. The worktree holds U1 uncommitted; change only what this brief names.

## F-1 (material): a failed impact proof blocks the batch's tree for good

`internal/landing/plain/prove.go` ~1169-1183 returns `Red` with `Cause{Kind:"environment"}` for an impact plan or fingerprint error but never sets `Repeat = "allowed"`; the existing convention (`observeRed`, `replay.go:38-40`) sets `Repeat = "allowed"` for every first environment red. `checkBound` (`prove.go:350-351`) then refuses the next prove of that tree at any depth ("this code failed its check and gets no other") until a person acts. Reproduced: a transient `test impact --plan` failure or an empty fingerprint on the first run; the second `landing prove --impact --wait` and a plain `landing prove --wait` are both refused.

Fix: the impact environment red follows the convention: `Repeat = "allowed"` on the first environment red of a tree (reuse `observeRed` or its rule rather than a second copy), so the next prove runs; a second environment red on the same tree holds as the convention says. Test: the read's reproduction (`scratchpad/repro/.../zz_repro_test.go` is a sketch: the runner prints no fingerprint on the first call and prints it on later calls): first prove red with the environment cause and `Repeat` allowed; second `prove --impact --wait` runs the runner and is green; mutation: drop the Repeat -> the second prove is refused, the test fails.

## Also (not material, trivial): the static red names its group in the summary

`Failed[0].Unit` names `fast-static-build` but the summary reason says only "the proving command exited 1": make the reason name the group when the static group is the red (one line).

## Checks

`go test -count=1 -timeout 30m ./internal/landing/plain`, the cmd tests by name (`TestLandingProve|TestLandingImpact|TestLandingStatus|TestReadStatus`), `go run ./cmd/devgate static` (the design page header is fixed; the goal is now in origin/main's ledger: run `git fetch origin` first and, if the static check still says the goal is not in the ledger, report that and do not rebase). Never edit testing.json; never open metasystem.conf.local; do not commit. Report: diff --stat of the correction, each exit, the before/after of the reproduction.
