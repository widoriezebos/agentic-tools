Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-04

# Goal

The closing code review of build step 1 of goal fleet-channel-gateway
(tier 3, box 1d/10/1200m/1/3), implementer job fcg-build1
(gpt-5.6-sol), against its brief as delivered
(metasystem/artifacts/agents/fcg-build1/rounds/1/prompt.md; the plan
copy lands with the step) and design
points FCG-INBOX-02 (metasystem/plans/fleet-channel-gateway-design.md
lines 131-381: the three field tables, the refusal table at 320-349,
the tuple predicate and the transition matrix), FCG-SECRET-15's last
paragraph (the validator's `channel-secret` row) and FCG-BUILD-13 step
(1) (lines 1089-1120). The computed diff and reviewedTree are the
conformance artefacts of that job
(metasystem/artifacts/agents/fcg-build1/rounds/1/diff.patch and
metasystem/artifacts/agents/fcg-build1/rounds/1/review.json;
reviewedTree f561c592eddcf0a797c9ea656c13f32cb622c632); review that
diff, never the delegate's own summary. Six files: internal/goal/channel.go
(new), internal/goal/channel_test.go (new), internal/goal/validate.go
(three lines in ValidateCommit), scripts/agents/path-classes.txt (one
row), scripts/agents/pre-commit-guard.sh (one regexp),
scripts/agents/pre-commit-guard-fixtures.sh (one case). The delegate's
recorded decisions are in
metasystem/artifacts/agents/fcg-build1/rounds/1/return.json
under `decisions`; a decision that changes law (a new refusal, a new
authority, a schema the design does not give) is a finding, a
mechanical choice recorded there is not.

# Review brief

Two ordered layers per the code-critique skill. LAYER 1, conformance:
step 1 is library only — confirm no caller of ChannelInboxMutate,
ClassifyChannelTransition or ChannelOpid exists outside channel_test.go,
no verb, flag or config key was added, and nothing under
internal/channel, the steward or recover.go changed; ValidateCommit
appends ValidateChannelTree's problems only after the goal problems
are empty and a commit with no plans/channel/ entries yields no
problems (the absent-directory test proves it, and `validate.go`'s
existing tests stay green); the path-classes row makes plans/channel/
a ledger path exactly as plans/goals/ is (internal/landing/observe.go:586
`ledger-path-not-goal-verb`), and the guard regexp plus its fixture case
refuse a staged plans/channel/ file; every row of the refusal table in
FCG-INBOX-02 has one test case in channel_test.go and each case is
discriminating (name the assertion that fails if the row is removed
from ValidateChannelTree); the three secret cases and the two
closed/null cases the brief lists are present; the structs' json tags
are the table keys in table order, `step` is `*int64`, the slices
refuse `null`, unknown keys are refused, times are RFC 3339 UTC second
precision; ChannelMatrix carries all eighteen named rows and
ClassifyChannelTransition's four outcomes match the brief's contract
(From → apply; TrailerPresent → AlreadyApplied as (false, nil); To under
another writer → LostToCompetitor{Winner}; else the
`channel-transition: <qid> is <tuple>, expected <row>` error with the
tuple printed as the brief says); ChannelInboxMutate's three branches
and the git-error pass-through; ChannelOpid's shape through
validOpidShape; the tests use txn_test.go's bed helpers and add no
second bed. LAYER 2, adversarial: a question whose `goal` names a goal
present at the tip but under a different case or with a trailing
suffix; a record path under plans/channel/ that classifyChannelPath
does not recognise (a stray file, a nested directory, a non-ULID id) —
refused by which code, or silently ignored; an inbox record whose
`question` is a question id that does not exist at the tip; the
verified-record clause against a question whose lineage is `migrated`
(skipped) versus `own` (applied); a six-digit last field followed by
two punctuation marks, a five-digit and a seven-digit field, a
six-digit field with a non-ASCII digit; `channelTime` on a
fractional-second or `+00:00` timestamp; `ChannelInboxMutate` when
`git show` fails for a reason other than a missing path (does the
error pass through unchanged, and can the absent branch be confused
with a corrupt object); `ClassifyChannelTransition` when `writer ==
opid` but the trailer is absent (which outcome, and does the design
name it); the rejection-intent row's `late`-only closed admission;
the `take-over` row checking ownership only — the caller supplies
staleness, is that a gap the brief covers or a decision that widens
the row; and whether any test asserts message text so tightly that a
wording change in a later step breaks it without a behaviour change.

Materiality criterion, verbatim: would the change ship a defect,
violate its brief, or damage what certifies it? Count only material
findings in the verdict. A finding that indicts the design rather
than the code is reported as such (design point named) and counts.

Run what the sandbox allows: `go build ./...`, `go vet
./internal/goal/...`, `go test ./internal/goal -run 'Channel|Marshal|ValidateCommit' -count=1`
(the whole goal package exceeds go test's default timeout; the seat
runs it with `-timeout 30m` separately), and
`scripts/agents/pre-commit-guard-fixtures.sh`. Report what could not
run.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
reviewedTree.

# Gap Rule

stop and report a gap; never fill it silently.
