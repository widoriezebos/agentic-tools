Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-04

# Goal

Follow-up round of build step 1 of goal fleet-channel-gateway: the
closing code review (job fcg-build1-cc, return at
metasystem/artifacts/agents/fcg-build1-cc/rounds/1/return.json) found
five material defects in your round-1 tree, all accepted by the seat
(the dispositions table lands with the step). Fix the five in place, add the one proof case under F-8, change nothing
else. Your round-1 brief (metasystem/artifacts/agents/fcg-build1/rounds/1/prompt.md)
still governs everything it says; where this brief and it differ, this
brief wins for the five points below.

# Workspace

Your existing worktree, uncommitted as you left it. Do not stage or
commit; the seat lands the chain.

- May write: metasystem/internal/goal/channel.go
- May write: metasystem/internal/goal/channel_test.go

No other file changes in this round.

# The five fixes

## F-1 — a lost intent race must classify as a loss

Every row whose TO writes a posting (ask, approve-intent,
receipt-intent, rejection intent, list intent, silence intent,
take-over) has a To predicate that requires `posting.by == me`, so a
tip where a competitor landed that posting never matches To and
ClassifyChannelTransition falls to the generic `channel-transition`
rejection instead of LostToCompetitor{Winner: posting.by}
(FCG-INBOX-02, design lines 345-348: a tuple equal to TO whose set
field names another writer is a loss). Change: those To predicates
test the posting's kind (and, where the row fixes it, the phase and
state) with ANY `by`; the `writer` argument alone discriminates own
trailer from loss, as the round-1 brief's contract for
ClassifyChannelTransition already says. The matrix test's TO tuples
carry a posting by the writer they pass (e.g. by "winner", writer
"winner") and assert LostToCompetitor{Winner: "winner"} for every
posting row; the From cases and the AlreadyApplied case stay.

## F-2 — list and silence intents never apply to a closed question

`channelIntentTransition`'s From refuses state closed only for the
rejection-intent row. Design line 364: closed is legal only for a
rejection with reason `late`; list and silence posts hang on open
questions (365, 730). Change: every intent row's From refuses closed,
except rejection intent when RejectionReason == "late". Tests: the
list-intent and silence-intent From cases use an open FROM tuple; a
closed FROM is asserted rejected for both; the rejection-intent
closed/late case stays.

## F-3 — take-over needs the staleness the design names

The take-over From accepts any foreign posting; design row 366 admits
only a posting older than posting-stale-sec, and the round-1 From
signature carried no clock. Change: `ChannelTuple` gains
`PostingStale bool`; `(*ChannelQuestion).Tuple()` leaves it false and
a new `(*ChannelQuestion).TupleAt(now time.Time, staleAfter
time.Duration) ChannelTuple` sets it when the posting is non-nil, its
`at` parses under channelTime's layout and `now.Sub(at) > staleAfter`
(a parse failure leaves it false). The take-over From requires
`t.Posting != nil && t.Posting.By != me && t.PostingStale`. Print the
tuple unchanged (the stale bit is not part of the printed form).
Tests: a fresh foreign posting via TupleAt is rejected with the
channel-transition error; a stale one applies; the same question
through `Tuple()` never applies take-over.

## F-4 — the absent branch must not read git's prose

`channelPathMissing` matches English stderr and gitIn passes the
caller's locale through, so a localised git turns every absent path
into a git error. Change: probe with
`gitIn(root, "ls-tree", "--name-only", tip, "--", path)`; exit 0 with
empty stdout is the absent branch; a non-empty listing reads the blob
with `show` as today; any git failure from either call is returned as
is. Delete channelPathMissing and its stderr substrings. Test: the
absent branch and the two present branches through the probe (the
existing three-branch test should keep passing; add an assertion that
a bogus tip — a non-existent commit — surfaces the git error rather
than the absent branch).

## F-5 — second precision means exactly the canonical form

`channelTime` accepts `2026-09-04T12:34:56.123Z` because time.Parse
admits a fractional second the layout does not name. Change:
channelTime refuses when `parsed.UTC().Format(layout) != input` (this
also refuses `+00:00`, a lowercase `z` and any other non-canonical
spelling). Tests: fractional second refused, `+00:00` refused,
lowercase `z` refused, the canonical form accepted, the existing
`+02:00` case stays.

## F-8 (one proof case only)

Add the refusal case for the verified-record clause of
`channel-answer-state` on a question of lineage `own` whose answer is
null (or whose inboxId differs) while a verified inbox record names it
— the row is already built; the case was missing.

# Verification

`go build ./...`, `gofmt -l` clean, `go vet ./internal/goal/...`,
`go test ./internal/goal -run 'Channel|Marshal|ValidateCommit' -count=1`
(the seat runs the whole package with `-timeout 30m` separately), `go
run honnef.co/go/tools/cmd/staticcheck@2025.1 ./internal/goal/...`.
Record every mechanical choice you make in `whatWasDone` (the
implementer schema has no decisions key). Every path in your return is
relative to the repository root, so it starts with `metasystem/`.

# Constraints

Wall-clock budget: 30 minutes. No new file, verb, flag or config key;
nothing under internal/channel, the steward or recovery changes; the
struct json tags and the refusal codes are unchanged.

# Gap Rule

Stop and report a gap only for a law-changing contract the five points
above do not settle; a mechanical choice is made from what the tree
does nearest the seam, recorded in `whatWasDone`, and built.
