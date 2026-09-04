Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-05

# Goal

Round 2 of the second closing code review of build step 2 of goal
fleet-channel-gateway: the register carry. Your round-1 finding F-1
(round 4 changed the post-confirm Receive in
TestTelegramListenersShareStreamAndConfirmedOffset from the empty
cursor to cursor "1", and the design sentence "getUpdates without
offset returns only unconfirmed updates" lost its only test) was
accepted and sent to the implementer as round 5 (brief at
metasystem/artifacts/agents/fcg-build2/rounds/5/prompt.md, return in
rounds/5/return.json). A third root (fcg-build2-cc3,
metasystem/artifacts/agents/fcg-build2-cc3/rounds/1/return.json)
certified the round-5 tree with zero material findings. Your F-2 and
F-3 were noted; F-2 is carried to the cut-over step's fixture brief
as a rule (every control.json writer writes by rename).

The conformance artefacts of the fixed tree are
metasystem/artifacts/agents/fcg-build2/rounds/5/diff.patch and
rounds/5/review.json (reviewedTree
98f0b886646c3706e7bdd727829161665a2dc9d5). Review that diff, never
the delegate's summary. Since your round 1 only fake_test.go changed.

# Review brief

For the carried finding, return it with `material: false` only if the
fix is complete and proven by a test that fails without it; otherwise
return it material with what is still wrong. The contract the fix was
held to:

- F-1: in TestTelegramListenersShareStreamAndConfirmedOffset, after
  the first listener's Confirm, a Receive by the second listener with
  the EMPTY cursor asserts exactly the one unconfirmed update; the
  offset-1 leg and its journal assertions from round 4 are unchanged;
  fake_test.go is the only file that changed in round 5.

Name the assertion that fails if the absent-offset branch regressed
to filtering from zero. No new adversarial layer is asked for: the
third root covered round 5; report anything you nonetheless see.

Materiality criterion, verbatim: would the change ship a defect,
violate its brief, or damage what certifies it? Count only material
findings in the verdict.

Evidence (seat, round-5 tree): `go test -race ./internal/channel/...
./cmd/...` all six packages ok; `go test ./internal/channel/fake/...
-count=1` ok. Report what your sandbox could not run.

# Constraints

Wall-clock budget: 15 minutes. Return per the code-critic schema with
reviewedTree 98f0b886646c3706e7bdd727829161665a2dc9d5.

# Gap Rule

stop and report a gap; never fill it silently.
