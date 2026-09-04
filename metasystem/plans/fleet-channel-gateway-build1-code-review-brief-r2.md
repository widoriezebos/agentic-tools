Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-04

# Goal

Round 2 of the closing code review of build step 1 of goal
fleet-channel-gateway. Your round-1 findings F-1 to F-5 were accepted
and sent back to the implementer as one fix round (job fcg-build1-r2,
brief at metasystem/artifacts/agents/fcg-build1/rounds/2/prompt.md,
return with its recorded decisions in
metasystem/artifacts/agents/fcg-build1/rounds/2/return.json). F-6 to
F-10 were noted: F-6 and F-9 go to the design and the step-3 brief,
F-8's lineage-own case was added in this round, F-7 and F-10 are
recorded decisions. The conformance artefacts of the fixed tree are
metasystem/artifacts/agents/fcg-build1/rounds/2/diff.patch and
metasystem/artifacts/agents/fcg-build1/rounds/2/review.json
(reviewedTree 330d8371c667fd6cc378b0dd61dcb1f546da3a0b); review that
diff, never the delegate's summary. The diff is the whole step (six
files, base-to-tree); only channel.go and channel_test.go changed
since round 1.

# Review brief

For each carried finding, return it with `material: false` only if the
fix is complete and proven by a test that fails without it; otherwise
return it material with what is still wrong. The contracts the fixes
were held to:

- F-1: the To predicates of every posting-writing row (ask,
  approve-intent, receipt-intent, rejection/list/silence intent,
  take-over) test the posting's kind (and phase/state where the row
  fixes them) with any `by`; the writer argument alone discriminates
  own trailer from LostToCompetitor{Winner: writer}; the matrix test's
  TO tuples carry a posting by the writer they pass.
- F-2: every intent row's From refuses state closed except rejection
  intent with RejectionReason "late"; closed FROM asserted rejected
  for list and silence intent.
- F-3: ChannelTuple.PostingStale; Tuple() leaves it false; TupleAt(now,
  staleAfter) sets it when the posting's `at` parses canonically and
  now − at > staleAfter; take-over From requires it; fresh foreign
  posting rejected, stale applies, Tuple() never applies take-over.
- F-4: absence probed with `ls-tree --name-only tip -- path` (empty
  stdout on exit 0 = absent); no stderr substring anywhere in
  channel.go; a bogus tip surfaces the git error, not the absent
  branch.
- F-5: channelTime refuses any input that does not round-trip
  byte-equal through the second-precision UTC layout; fractional,
  +00:00, lowercase z refused; canonical accepted.

Then the adversarial layer on the changed code only: does the
by-agnostic To of the ask row now match a tip where a DIFFERENT
question's posting stands (is the tuple per question, so no); does
take-over's To (any non-null posting per the delegate's recorded
decision) admit a tuple that another row owns, and does that matter
given the writer rule; TupleAt with a posting whose `at` is exactly
staleAfter old (strict greater-than — is that the design's "older
than"); the ls-tree probe on a path that is a directory prefix of a
real entry (`ls-tree` without `-r` on `a/b` when `a/b/c` exists lists
the tree entry — is a non-empty listing then wrongly read as
present, and does `show` on a tree object then fail loudly or
silently); the round-trip check against a time with a leading `+`
year or a sub-second that formats away; any test that now depends on
wall-clock time.

Materiality criterion, verbatim: would the change ship a defect,
violate its brief, or damage what certifies it? Count only material
findings in the verdict.

Run what the sandbox allows (`go build ./...`, `go vet
./internal/goal/...`, `go test ./internal/goal -run
'Channel|Marshal|ValidateCommit' -count=1`); report what could not
run. The seat has run go-gate --fast on the fixed worktree (green) and
is running the goal and landing packages with -race.

# Constraints

Wall-clock budget: 25 minutes. Return per the code-critic schema with
reviewedTree 330d8371c667fd6cc378b0dd61dcb1f546da3a0b.

# Gap Rule

stop and report a gap; never fill it silently.
