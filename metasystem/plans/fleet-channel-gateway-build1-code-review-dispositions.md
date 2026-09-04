# fleet-channel-gateway — dispositions, build step 1 code review round 1 (job fcg-build1-cc)

The closing code critic (claude-fable-5-1, reviewedTree f561c592) read
the computed diff of fcg-build1 against the brief as delivered and
FCG-INBOX-02, and returned five material findings. All five are
accepted and go back to the implementer as one follow-up round
(plans/fleet-channel-gateway-build1-fix-brief.md); the five non-material
findings are noted here, two of them as facts the design should state
(F-6, F-9) and carried into the step-3 brief. The critic could not run
the build or the tests (read-only sandbox); the seat ran them on the
worktree before the review (go-gate --fast, `go test ./internal/goal`
in 1787 s, landing package, guard fixtures — all green) and runs them
again after the fix round. One table per file, as `validate
critique-closed` joins it.

| Finding id | Disposition | Reasoning and evidence | Amendment |
|---|---|---|---|
| F-1 | accepted | channel.go 762-763, 805-806, 823-824, 845, 875: every posting-writing row's To requires posting.by == me, so a tip where a competitor landed the posting never matches To and falls to the generic rejection; the design (INBOX-02 lines 345-348) says a tuple equal to TO whose set field names another writer is a loss; the matrix test (channel_test.go 1291-1321) passes only by pairing a posting by me with writer "winner", which the brief's writer definition (posting.by) makes impossible | fix round: the To predicates of ask, approve-intent, receipt-intent, rejection/list/silence intent and take-over test the posting's kind (and for approve/receipt the phase) with any `by`; the writer argument alone discriminates own trailer from loss; the matrix test's TO tuples carry posting by the writer they pass and assert LostToCompetitor{Winner: writer} |
| F-2 | accepted | channel.go 866-874 excludes closed only for rejection intent; design line 364 admits closed only for a rejection with reason late, and list/silence posts hang on open questions (365, 730); channel_test.go 1302 asserts a closed FROM applies for silence intent | fix round: every intent row's From refuses closed except rejection intent with RejectionReason "late"; the silence-intent and list-intent cases use an open FROM and a closed FROM is asserted rejected for both |
| F-3 | accepted | channel.go 844: take-over From is any foreign posting; design row 366 admits only a posting older than posting-stale-sec; the brief's From signature carried no clock, so the row as built would let a caller steal a fresh intent on a rebuilt tip | fix round: ChannelTuple gains `PostingStale bool`; `Tuple()` leaves it false and the new `TupleAt(now time.Time, staleAfter time.Duration)` sets it when the posting's `at` parses and now − at > staleAfter; take-over From additionally requires PostingStale; tests: a fresh foreign posting is rejected, a stale one applies, and `Tuple()` never applies take-over |
| F-4 | accepted | channel.go 935-941, 958-964 match English git stderr and gitIn (genesis.go:92) passes LANG/LC_MESSAGES through; a localised git turns every absent path into a git error and no inbox record could be written on that machine | fix round: probe absence with `gitIn(root, "ls-tree", "--name-only", tip, "--", path)` — empty stdout on exit 0 is the absent branch, a non-empty listing reads the blob with `show`, any git failure is returned as is; channelPathMissing is deleted; a test proves the absent branch does not depend on stderr text (run the probe with LANG set to a non-English locale in the test's git call — or, if the test cannot set the subprocess locale, assert that no stderr substring appears in channel.go's absent branch) |
| F-5 | accepted | channel.go 648-653: time.Parse admits a fractional second after the seconds field even when the layout has none, so 2026-09-04T12:34:56.123Z passes channelTime; the tables require second precision; channel_test.go 1234 covers only +02:00 | fix round: channelTime refuses when the parsed time re-formatted with the layout is not byte-equal to the input; test cases: fractional second refused, +00:00 refused, lowercase z refused, the canonical form accepted |
| F-6 | noted | an inbox record with opid "" reaches LostToCompetitor{""} because TrailerPresent treats the empty line of a trailer-less commit as a match; needs a hand-written record past three fences; the design's inbound table gives opid as a required string | step 3 brief: the inbound writer mints the opid and the validator's `channel-json` row refuses an empty `opid` as a missing required field (a missing-required refusal the table already carries, applied to the empty string) — carried, not built in this round |
| F-7 | noted | channelGoalIDs accepts plans/goals/, plans/goals/done/ and records/goals/ where the brief named plans/goals/<goal>.md; the design says only that the goal file exists on the tip, and the narrow reading would refuse every commit after a goal with a channel question concluded; correct, unrecorded | recorded here as the delegate's decision; no change |
| F-8 | noted | no test asserts the verified-record refusal for lineage own; the approvalUlid tuple rule is refused under channel-answer-state without a table code; the inbox record's destination is never compared to its directory | fix round adds the lineage-own refusal case (it is one row of the table already built); the other two are proof gaps for the step-3 brief, where the writers of those fields land |
| F-9 | noted | TO matches, writer == opid, trailer absent: the generic rejection; the design names no outcome; cannot arise on a consistent tip | design revision 5 (with step 3's plan copy) names the rejection for this state; no code change |
| F-10 | noted | two exact-message assertions bind wording the brief fixes by name; every problem assertion matches on the code prefix; decisions live in whatWasDone prose because the implementer return schema forbids a decisions key | the fix brief and every later brief say "record decisions in whatWasDone"; no change |

## Round 2 (job fcg-build1-cc-r2, reviewedTree 330d8371)

Round 2 returned every carried finding `material: false` and two new
non-material notes; zero material findings closes the loop under
R-60-m1. F-4's proof is structural (no stderr text remains in
channel.go; absence is exit 0 plus empty ls-tree stdout; the three
branches and a bogus tip are tested) because the bed cannot run a
localised git — recorded here as the critic asked. F-11 (take-over's
To names the current posting owner, whatever its kind) and F-12 (a
tree entry at a record path fails loudly at the opid decode) carry
into the step-3 brief as facts its callers must know. The round-2
dispositions are in the register (the fold of round 2 marks F-1 to
F-5 resolved); no second table is kept here so `validate
critique-closed` joins one table per file.
