Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal backlog-ordered-by-priority)
Date: 2026-09-08

# Fold brief: round 3 of chain bolnext-build1, one broken test call

Round 2's code is right. The orchestrator verified it outside the
sandbox on your worktree: the priority and next canaries pass in the
goal, command and channel packages, and the fast gate exits 0. Do not
revisit the four corrections; they are done.

Two things remain, both small, and one of them is a test you added.

## D1 - the new bed case calls a flag the verb does not accept

Round 2 added this to `scripts/agents/goal-cli-fixtures.sh`, in the
labels-and-filtering scenario:

```
"$ms" goal set-pin --root "$clone" --id machine-only --pin other-machine \
  --by Wido --fixture-human-authority >/dev/null
```

`goal set-pin` does not define `--fixture-human-authority`. The bed
therefore prints "flag provided but not defined:
-fixture-human-authority", dumps the set-pin usage and exits 2 before
reaching a single assertion. The orchestrator ran the bed on your tree
and reproduced this twice.

So the machine-scoped empty-message assertion you added has never
actually run. It proves nothing yet.

Fix it, and choose deliberately between the two shapes:

- Give the bed case a way to reach that state without needing human
  authority on a pin. The state you need is simply that the only goal
  matching the filter is one this machine may not claim.
- Or add the fixture-authority flag to `goal set-pin` so it matches the
  other human verbs that already accept it, defined at
  metasystem/cmd/metasystem/goalsync_mutations.go around lines 246 and
  634, with the same fake-runtime-root restriction.

Prefer the first. Do not widen an authority surface to make a test
pass; take the second only if you can say in the return why set-pin
genuinely belongs with those verbs, independent of this test.

Whichever you choose, the assertion must then actually run and must fail
if the message is wrong. Say in the return that you watched it fail
before you made it pass, by feeding it the other sentence.

## D2 - the return names the round

Round 2 was recorded as failed with a protocol error, because its return
carried `jobId` "bolnext-build1", the chain root, where the job record
says "bolnext-build1-r2". Conformance accepted the round anyway and no
code is affected, but do not repeat it: this round's return must carry
its own job id.

## Scope

These two only. No change to the four corrections, no new behaviour, no
authority widened beyond what D1 permits and justifies.

## Verification

- The corrected bed case, run as a focused check if you can reach it, and
  the statement that you watched the assertion fail for the right reason.
- `bash -n scripts/agents/goal-cli-fixtures.sh`
- The three canaries: `go test ./internal/goal -run
  'TestNextPriority|TestPriority'`, `go test ./cmd/metasystem -run
  'TestGoalPriority'`, `go test ./internal/channel -run
  'TestReportPriority'`.
- `go build ./...` and `scripts/agents/go-gate.sh --fast`, once, at the end.

The orchestrator runs the goal-command bed outside your sandbox and that
run is the proof this is fixed. Do not attempt the beds.

Gap rule: stop and report a gap; never fill it silently.
