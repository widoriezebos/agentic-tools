# fleet-channel-gateway — dispositions, build step 2 code review (job fcg-build2-cc)

The closing code critic (claude-fable-5-1, fresh root on the terminal
round fcg-build2-r3, reviewedTree a2e1346a) read the computed diff
against the three briefs as delivered and FCG-RECEIVE-03, POLL-04,
POST-08, SECRET-15, EVIDENCE-12 and BUILD-13 (2), and returned one
material finding (F-1) and seven non-material notes. F-1 and F-2 go
back to the implementer as one follow-up round
(plans/fleet-channel-gateway-build2-fix3-brief.md); the rest are
recorded here. The critic could not run the build or the tests
(read-only sandbox); the seat ran go-gate --fast, `go test -race
./internal/channel/... ./cmd/...` and scripts/agents/channel-fixtures.sh
on the worktree before the review and runs them again after the fix
round. A fresh critic root (fcg-build2-cc2) reviews the fixed tree;
its verdict is noted below the table. One table per file and per critic round, as
`validate critique-closed` joins each round's return against one
table: round 2 of this root is in
plans/fleet-channel-gateway-build2-code-review-dispositions-cc-r2.md,
the second root's rounds in ...-dispositions-cc2.md and
...-dispositions-cc2-r2.md, the third root in ...-dispositions-cc3.md.

| Finding id | Disposition | Reasoning and evidence | Amendment |
|---|---|---|---|
| F-1 | accepted | fake.go: reloadControls runs only from requestControls at arrival; telegramUpdates ticks every 100 ms reading s.controls.DeliverOnlyTo from the stale copy, so a deliverOnlyTo written during a blocked long poll does not exclude the blocked listener; design line 955 says other listeners never see that update and 952 says the file is re-read per request; the delivered test wrote the control before a fresh short poll and never exercised a poll in flight | fix round: the controls are reloaded on each tick before the delivery filter runs (pauseBefore and conflict stay arrival-time); a malformed file mid-poll ends the request with the 500; test: a poll blocked with timeout 3 sees a control written at 250 ms and returns zero updates at its deadline while the named listener's fresh poll returns the update |
| F-2 | accepted | fake.go filters from the body's offset even when it is below the confirmed offset, replaying rows another listener confirmed; the design (949-950) says offset c forgets everything below c, as Telegram does; the round-1 brief's "returns from c" was loose | fix round: filter from max(confirmedOffset, c); test: A confirms 3, B's poll with offset 1 returns only ids >= 3 |
| F-3 | noted | the journal sequence restarts at 1 when the fake restarts while journal.jsonl is appended; every fixture starts its fake in a fresh directory; the design (948) asks for a monotonic sequence per row and says nothing about restarts | none now; the step-3c fixture brief starts each fake in a fresh directory and says so |
| F-4 | noted | loader and validator refuse channel.http-timeout-sec at the same boundary with two sentences ("must be a positive integer of seconds" / "must be a positive integer, got <value>"); same law | no change |
| F-5 | noted | loadFake detects an absent channel.fake.listener by comparing the error text "no value configured for channel.fake.listener" produced in internal/config/resolve.go; a wording change there would turn an absent key into a load refusal; recorded by the delegate as a mechanical choice; the phase package's existing reads use the same Get and the same shape | carried to the step-3c brief as a fact its loader changes must respect (a sentinel or ok-bool from Get is the honest fix and belongs with the loader work that step brings) |
| F-6 | noted | three tests depend on elapsed time beyond the fake's tick: a 1.2 s server sleep with a 900 ms-2 s window, two 250 ms releases; what they prove needs a clock | noted for flake triage; no change |
| F-7 | accepted | Poll appends the whole Inbound to unmatched.jsonl, so new rows carry ack and updateID (the reader matches on Ref only; nothing breaks); pre-existing on main, that file persists the raw text, code included — a durable surface FCG-SECRET-15 does not name | step 3c: FCG-MIGRATE-10 republishes each row as an inbox record with text = StripCode's clean and renames the file; after the cut-over the new Poll never writes unmatched.jsonl; the 3c brief names the file among the durable surfaces the cut-over retires |
| F-8 | noted | new assertions bind the generic 409 text, Go's "invalid character" for a malformed control file, the loader sentence and the context-deadline text; none conflicts with a planned step | recorded so a later behaviour-neutral change is not surprised by a red test; no change |

## Second root (fcg-build2-cc2, reviewedTree 2504bd49, on the round-4 tree)

One material finding and two notes. Its F-1 (material): round 4
changed the post-confirm Receive in
TestTelegramListenersShareStreamAndConfirmedOffset from the empty
cursor to cursor "1" to prove F-2's max() rule, and the design
sentence "getUpdates without offset returns only unconfirmed updates"
(design line 949) lost its only test; the round-4 brief had said the
existing tests stay green unmodified except where F-2 changes what a
lower offset returns, and this leg was not such a place. Accepted:
round 5 (plans/fleet-channel-gateway-build2-fix4-brief.md) restores
the empty-cursor leg after the confirm alongside the offset-1 leg,
one file, nothing else. Its F-2 (not material): reloading the control
file on every tick of a blocked long poll multiplies the chance of
reading a truncate-then-write rewrite while empty, which ends that
poll with a 500 the fixture did not intend; the test helper uses
os.WriteFile and channel-fixtures.sh has no control.json writer yet.
Accept-risk now; carried to the cut-over step's fixture brief as a
rule: every control.json writer writes by rename. Its F-3 (not
material): the new delivery-controls test adds three seconds of real
time and the malformed-control test binds the Go parser's wording,
the same kinds as F-6 and F-8 above; noted, no change.

## Third root (fcg-build2-cc3, reviewedTree 98f0b886, on the round-5 tree)

Zero material findings; the loop closes on this root (R-60-m1). Its
one note, F-1 (not material), corrects the seat's brief rather than
the code: the journal assertions in
TestTelegramListenersShareStreamAndConfirmedOffset check that two
specific confirm rows are present, they do not count rows, so a
spurious confirm from the restored empty-cursor leg would not have
turned the test red. That the leg writes no confirm rests on code
unchanged since the first root certified it (the Telegram adapter
sends the offset key only for a non-empty cursor; the fake journals a
confirm only when the key is present; blob hashes of telegram.go and
fake.go are identical between the round-4 and round-5 patches). Noted;
the closing rationale cites the code, not the test; no change.
Its table is plans/fleet-channel-gateway-build2-code-review-dispositions-cc3.md (one table per file).
