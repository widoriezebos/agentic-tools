Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-05

# Goal

Round 2 of the first closing code review of build step 2 of goal
fleet-channel-gateway: the register carry. Your round-1 finding F-1
(the fake's deliverOnlyTo control was evaluated from a stale in-memory
copy on every tick of a blocked long poll) was accepted, together
with F-2, and sent to the implementer as round 4 (brief at
metasystem/artifacts/agents/fcg-build2/rounds/4/prompt.md, return in
rounds/4/return.json). A fresh root (fcg-build2-cc2,
metasystem/artifacts/agents/fcg-build2-cc2/rounds/1/return.json)
reviewed the round-4 tree and found one further material defect in a
test leg, fixed by round 5 (rounds/5/prompt.md); a third root
(fcg-build2-cc3, metasystem/artifacts/agents/fcg-build2-cc3/rounds/1/return.json)
certified the round-5 tree with zero material findings. Your F-3 to
F-8 were noted; their dispositions land with the step.

The conformance artefacts of the fixed tree are
metasystem/artifacts/agents/fcg-build2/rounds/5/diff.patch and
rounds/5/review.json (reviewedTree
98f0b886646c3706e7bdd727829161665a2dc9d5). Review that diff, never
the delegates' summaries. The diff is the whole step (13 files,
base-to-tree); since your round 1 only fake.go and fake_test.go
changed.

# Review brief

For the carried finding, return it with `material: false` only if the
fix is complete and proven by a test that fails without it; otherwise
return it material with what is still wrong. The contract the fix was
held to:

- F-1: the controls are reloaded under the lock on each tick of a
  blocked long poll, before the delivery filter reads DeliverOnlyTo;
  pauseBefore and conflict stay arrival-time; a malformed control
  file mid-poll ends that request with the 500 and the parse error;
  test: a poll blocked with timeout 3 sees a control written at
  250 ms and returns zero updates at its deadline while the named
  listener's fresh poll returns the update.

Name the assertion that fails if the per-tick reload is removed. No
new adversarial layer is asked for: the second and third roots
covered round 4 and round 5; report anything you nonetheless see.

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
