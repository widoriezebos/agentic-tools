Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-05

# Goal

The closing code review, second root, of build step 3a of goal
fleet-channel-gateway (tier 3, box 3d/24/1200m/1/3): the ledger
receive as a library — verify, match, commit — with no caller.
Implementer chain fcg-build3a (gpt-5.6-sol), three rounds: the step
(metasystem/artifacts/agents/fcg-build3a/rounds/1/prompt.md), a
one-file fixture fix (rounds/2/prompt.md: four test ULIDs outside the
Crockford alphabet) and a one-file proof (rounds/3/prompt.md, return
in rounds/3/return.json). The first root (fcg-build3a-cc,
metasystem/artifacts/agents/fcg-build3a-cc/rounds/1/return.json,
reviewedTree 04560087) returned four material findings and four
notes. F-1, F-2 and F-3 indict the design, not the code (the stop
question's trailing full stop binds in the inbox but the resume
authority rule does not drop it; a newline in the human's verbatim
text against the one-line history grammar; a reply threaded to an
orphan post while the question's thread is null has no committable
matrix row); the engine demoted them from this chain's register as
outside the reviewed subject set, and the seat carries them to the
design chain. F-4 (the budget matrix row was never exercised through
ChannelInboundRequest) was accepted and sent back as round 3: one new
git-backed test, TestChannelInboundPublishBudgetAnswerRows, in
channel_inbox_test.go, nothing else.

You are a fresh root on the fixed tree: review the terminal round's
computed diff (metasystem/artifacts/agents/fcg-build3a/rounds/3/diff.patch,
reviewedTree in rounds/3/review.json —
516c8e5c21f1489202aa8f18efbb442a36f3f208; five files, base-to-tree)
against the three briefs as delivered (rounds/1-3/prompt.md) and the
first root's brief as delivered
(metasystem/artifacts/agents/fcg-build3a-cc/rounds/1/prompt.md, whose
threat model and two layers still apply in full). Review the diff,
never the delegate's summaries.

# Review brief

LAYER 1, conformance, in this order: (a) round 3 — the one-file
boundary held (channel_inbox_test.go only; no production line
changed since reviewedTree 04560087); the new test publishes through
ChannelInboundRequest on the git-backed bed, its matching-text branch
asserts phase `recorded`, answer.opid = the commit's opid, the exact
approval ULID 01M1P6MEC0DTSKCJCZ05C283YB, receipt null, the goal
row's reason verbatim and its channel step, and ValidateCommit's
acceptance; its differing-text branch asserts phase `approved`, a null
approval ULID and the "box not raised" receipt byte for byte; the row
probes it installs on ChannelMatrix are restored after each branch
and do not change what the rows decide; name the assertion that
fails if the rowName selection in Mutate is removed or hard-coded.
(b) The whole step against the first root's layer 1 (a)-(h) — it is
one diff now; confirm nothing regressed since reviewedTree 04560087
beyond that one test.

LAYER 2, adversarial, on round 3's test only: does swapping
ChannelMatrix entries from a test leave any other test in the package
observing the probe (the package's tests run in one process; the
restore is deferred per branch — is there a path where a fatal
assertion skips it); is the differing-text branch discriminating
against a Mutate that selects "answer budget" for every
budget-above-norm question (which assertion fails); does the test's
budget tuple and question shape survive ValidateChannelTree at the
tip, or did the bed commit a question the validator would refuse
elsewhere. The first root's layer-2 list was applied to everything
else and stands; report anything you nonetheless see.

Materiality criterion, verbatim: would the change ship a defect,
violate its brief, or damage what certifies it? Count only material
findings in the verdict. A finding that indicts the design rather
than the code is reported as such (design point named) and counts;
F-1, F-2 and F-3 of the first root are already carried and need no
second report.

Evidence (seat, round-3 tree): `gofmt -l` clean; `go vet
./internal/goal/... ./internal/channel/...` ok; `go test
./internal/goal -run
'ChannelInbound|ReadChannelTree|MatchChannelInbound|ChannelAnswerDisposition'
-count=1` ok (13.8 s); TestChannelInboundPublishBudgetAnswerRows
runs both branches (3.9 s). Full goal package on the round-2 tree,
run alone: ok (352 tests, 957.9 s); the round-3 run is in progress
and the seat holds the chain until it is green. `go test -race
./internal/channel/...` all five packages ok on the round-1 tree
(rounds 2 and 3 changed only channel_inbox_test.go). Conformance
review of round 3 wrote rounds/3/diff.patch (5 files) at
reviewedTree 516c8e5c. Run what your sandbox allows; report what
could not run.

# Constraints

Wall-clock budget: 25 minutes. Return per the code-critic schema with
reviewedTree 516c8e5c21f1489202aa8f18efbb442a36f3f208.

# Gap Rule

stop and report a gap; never fill it silently.
