Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal human-carried-landing-carry)
Date: 2026-09-11

# A fresh chain: finish the carried landing from the round that ran out of time

Chain hcl-build1-20260911 built most of the carried landing in one round
and hit its two-hour cap while its land bed was running; a timed-out
round takes no follow-up, so this chain starts from `main` at ab513438b,
applies that round's work verbatim, and finishes it. Its one round is
what the closing read and the live proof cover. The goal is
human-carried-landing-carry (metasystem/plans/goals/human-carried-landing-carry.md,
Wido's approval of 2026-09-06, tier 3); the design is
metasystem/plans/human-carried-landing-carry-design.md, revision 6 (sha256
45b204f9b5d9a037edb06dc3a054ca053c7dedae4f0cf4edc251acc16faac7a5),
read three times on the Sol lane and folded three times; its decisions
are settled and it is the specification. The earlier round's brief is
metasystem/artifacts/agents/hcl-context/build-brief.md; its decisions,
facts and proof list hold here unchanged.

# Step 0: apply the earlier round, verbatim

`metasystem/artifacts/agents/hcl-context/build1-r1.patch` (sha256
96503eac9c0b9261f06d4e6b4dba6a87d73072b4712c25496c67b2bab2b15a87) is
chain hcl-build1-20260911's staged diff, 52 files, taken with
`git diff --cached --binary` at the repository toplevel. Apply it from the
repository toplevel with `git apply --index`; the coordinator has checked
it applies cleanly on ab513438b. Read it before you change anything: it
holds the engine (governance, goal, humanauthority, channel, landing,
config, counselor, steward, dispatch, refusal), the goal, landing and
channel verbs, commit.sh, land.sh and dispatch.sh in carried mode, the
register flip with `TestHCL03NoPendingAfterSlice2`, testing.json's
ownership, the three goal-cli scenarios, the six carried land legs, the
dispatch bed's commit scenario, and the two docs pages. Keep what it
contains; change what the state below and the page require.

# State of the candidate, verified by the coordinator outside the sandbox

The coordinator applied the round's tree in its worktree and ran, on the
host, what the sandbox could not. Observed:

- `go build ./...` and `go vet ./...`: green. `metasystem test check
  --root .`: READY, 67 groups.
- `go test ./internal/... ./cmd/metasystem/...`: green across internal/... and cmd/metasystem (one
  cmd/metasystem test, the frozen public corpus run, failed only while
  four beds loaded the host and passed alone in 77 seconds).
- `scripts/agents/goal-cli-fixtures.sh`: the fifteen existing scenarios
  pass; the three new ones fail, each at a defect in the engine or the
  verb, not in the scenario:
  1. `carry-word`: a `goal carry` on a format-1 ledger without
     `--raise-format` exits 1 with the rejected JSON on stdout
     (`{"detail":"carry-format-required: …","outcome":"rejected"}`)
     where the page and the scenario want exit 3 and the ask text.
     Cause: `Carrying` (metasystem/internal/goal/verbs.go, the
     `if result.Outcome == OutcomeRejected` block after its `Publish`)
     turns a rejected detail into a `CarryAskError` through
     `carryAskFromRejectedDetail`; `Carry` (the same file, from
     `func Carry`) returns the rejected result without that mapping, so
     `printCarryMutation` in metasystem/cmd/metasystem/goalsync_mutations.go
     never sees an ask. Every ask the page names for `goal carry`
     (`carry-format-required`, `carry-cap-reached`, `carry-debt-unpaid`,
     the supersede refusals, `carry-goal-not-live`) is on this path. Map
     them the way `Carrying` does, and make the scenario's cap and
     supersede steps pass too.
  2. `carried-record` and `carried-discharge`: both fail in
     `prepare_carried_record_fixture` at `goal carrying --root … --id
     ship-widget --ref <word> --tree <project>`: "the ledger tree at … does
     not validate: plans/goals/ship-widget.md: History line 18: carrying
     open reason is missing by". Cause: the `carrying` row's reason is
     rendered as `open workspace=… project=… expires=… by=%s` from
     `args.By` (metasystem/internal/goal/verbs.go, the row build in
     `Carrying`), and the scenario, like land.sh's first reservation form,
     gives no `--by`; the validator (metasystem/internal/goal/file.go, the
     `carrying` case of the history parse) requires the four keys. The
     page's row holds the seat and the word; the human is the word's `by`.
     Decide it as: `goal carrying` takes `by` from the carry word's row
     when the flag is absent and refuses only when neither exists; say so
     in the verb's usage. Then the two scenarios' own assertions (the
     exact obligation line, the budget-exception count, the replay that
     does not advance the ledger, the accepted-risk discharge and its
     append-once register) are unverified until they run: make them pass.
- `scripts/agents/dispatch-fixtures.sh`: nine scenarios pass; the
  `dispatch` scenario fails at the new commit-subject step: its
  `git commit -qm 'commit subject fixture'` in the agent fixture repo
  (metasystem/scripts/agents/dispatch-fixtures.sh, the block under the
  "HCL-09: a landed commit is a first-class critic subject" comment) runs
  without `-c core.hooksPath=/dev/null`, which every other commit in that
  repo passes, so the pre-commit guard refuses it ("agent commit requires
  scripts/agents/commit.sh; the live wrapper ancestry token is missing")
  and everything after it in the scenario, the verifier `review-proof`
  step included, never ran. Add the override; then the commit-subject
  assertions (exact `reviews` binding, frozen reviewedTree, the
  parent-to-commit patch byte for byte, the second chain on the same
  commit) are verified for the first time.
- `scripts/agents/land-fixtures.sh` (cap scale 8): the thirteen existing
  legs pass; the six carried legs pass too: `carried-fresh`,
  `carried-chain-group`, `carried-forward`, `carried-crash`,
  `carried-asks`, `carried-record-failures`; "land fixtures passed (19
  isolated legs)". Keep them green.

# What this round does, in order

1. Apply the patch; `go build ./... && go vet ./...`; `go test
   ./internal/... ./cmd/metasystem/...` green (name any sandbox-bound
   test).
2. `bash scripts/agents/go-build.sh` (default stamp), then
   `metasystem test check --root .` green.
3. `bash scripts/agents/land-fixtures.sh`: every leg green, the thirteen
   existing and the six carried legs (`carried-fresh`,
   `carried-chain-group`, `carried-forward`, `carried-crash`,
   `carried-asks`, `carried-record-failures`). Fix what the legs find in
   the wrapper or the engine; a fixture the page names that you cannot
   make pass is a `deviations` line with the reason, never a weakened
   assertion. Run single legs with `--fixture-bed-child <leg>` while you
   iterate, the whole bed once at the end.
4. `bash scripts/agents/goal-cli-fixtures.sh` and
   `bash scripts/agents/dispatch-fixtures.sh` green with the page's
   scenarios (the goal-cli bed is sandbox-bound for you: run it, report
   what it says, and rely on the coordinator's run recorded above).
5. `bash scripts/agents/go-gate.sh --fast` green.
6. `metasystem test plan --root . --mode auto --purpose delivery --goal
   human-carried-landing-carry` on the staged candidate; the selected
   groups in your return.
7. Re-read the page's "Files this design touches" against your diff and
   say in the return what is absent and why.

# Return

`diffBoundary` and `files` are repository-root paths (`metasystem/...`).
Under `evidence` every command above with its observed result; under
`deviations` every departure from the page with the line. Wall-clock
expectation: the cap (120 minutes); return BEFORE the cap with what is
green, in the order above, and say what is red and why. A return at 100
minutes with two red legs named is worth more than a timeout at 120
with none.
