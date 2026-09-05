Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-05

# Goal

Round 2 of the closing code review of build step 3a of goal
fleet-channel-gateway: the register carry. Your round-1 finding F-4
(the budget matrix row was never exercised through
ChannelInboundRequest; the rowName selection in Mutate, the recorded
phase with a non-null approvalUlid and null receipt, and
ValidateCommit's acceptance of that shape were uncertified) was
accepted and sent to the implementer as round 3 (brief at
metasystem/artifacts/agents/fcg-build3a/rounds/3/prompt.md, return in
rounds/3/return.json). A second root (fcg-build3a-cc2,
metasystem/artifacts/agents/fcg-build3a-cc2/rounds/1/return.json)
reviewed the round-3 tree. Your F-1, F-2 and F-3 were accepted as
true design findings: the engine demoted them from this chain's
register (their artifact, the design document, is outside the
reviewed subject set) and the seat carries them to the design chain's
register for the daytime design round; they need no second report.
Your four notes are recorded in the seat's dispositions.

The conformance artefacts of the fixed tree are
metasystem/artifacts/agents/fcg-build3a/rounds/3/diff.patch and
rounds/3/review.json (reviewedTree
516c8e5c21f1489202aa8f18efbb442a36f3f208). Review that diff, never
the delegate's summary. Since your round 1 only channel_inbox_test.go
changed: one new test, TestChannelInboundPublishBudgetAnswerRows.

# Review brief

For the carried finding, return it with `material: false` only if the
fix is complete and proven by a test that fails without it; otherwise
return it material with what is still wrong. The contract the fix was
held to:

- F-4: a git-backed test publishes through ChannelInboundRequest a
  verified threaded reply to an open budget-above-norm question with
  a Budget tuple; its matching-text branch asserts on the committed
  tree phase `recorded`, answer.opid = the commit's opid, the exact
  approval ULID 01M1P6MEC0DTSKCJCZ05C283YB, receipt null, the goal
  row's reason verbatim with no appended token and the channel step,
  and ValidateCommit's acceptance; its differing-text branch asserts
  phase `approved`, a null approval ULID and the "box not raised"
  receipt byte for byte; the row selection is proven by both branches
  so that removing or hard-coding the rowName choice fails a test.

Name the assertion that fails if the rowName selection in Mutate is
removed. No new adversarial layer is asked for: the second root
covered round 3; report anything you nonetheless see.

Materiality criterion, verbatim: would the change ship a defect,
violate its brief, or damage what certifies it? Count only material
findings in the verdict.

Evidence (seat, round-3 tree): `gofmt -l` clean; `go vet
./internal/goal/... ./internal/channel/...` ok; `go test
./internal/goal -run
'ChannelInbound|ReadChannelTree|MatchChannelInbound|ChannelAnswerDisposition'
-count=1` ok (13.8 s); TestChannelInboundPublishBudgetAnswerRows runs
both branches (3.9 s). Full goal package on the round-3 tree, run alone: `go test ./internal/goal -count=1` ok (955.8 s; 352 tests plus the new one). Report what your sandbox
could not run.

# Constraints

Wall-clock budget: 15 minutes. Return per the code-critic schema with
reviewedTree 516c8e5c21f1489202aa8f18efbb442a36f3f208.

# Gap Rule

stop and report a gap; never fill it silently.
