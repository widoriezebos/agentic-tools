Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-05

# Goal

The closing code review of build step 2 of goal fleet-channel-gateway
(tier 3, box 3d/24/1200m/1/3), implementer chain fcg-build2
(gpt-5.6-sol; three rounds — the step, the phase-loader wiring, the
fake's unknown-token regression), against its briefs as delivered
(metasystem/artifacts/agents/fcg-build2/rounds/1/prompt.md,
metasystem/artifacts/agents/fcg-build2/rounds/2/prompt.md,
metasystem/artifacts/agents/fcg-build2/rounds/3/prompt.md) and design
points FCG-RECEIVE-03 (metasystem/plans/fleet-channel-gateway-design.md
lines 382-463), FCG-POLL-04's HTTP and 409 paragraphs (464-531),
FCG-POST-08's deadline paragraph (660-672), FCG-SECRET-15 (886-933),
FCG-EVIDENCE-12's first paragraph on the fake (934-965) and
FCG-BUILD-13 step (2) (1089-1120). The computed diff and reviewedTree
are the conformance artefacts of the terminal round
(metasystem/artifacts/agents/fcg-build2/rounds/3/diff.patch and
metasystem/artifacts/agents/fcg-build2/rounds/3/review.json;
reviewedTree a2e1346a9683f299b4d9628ee584b512e9cf0d03); review that
diff, never the delegate's own summaries. Thirteen files, all under
internal/channel (channel.go, channel_test.go, fake/fake.go,
fake/fake_test.go, mask_test.go new, phase/phase.go,
phase/phase_test.go, slack/slack.go, slack/slack_test.go,
telegram/telegram.go, telegram/telegram_test.go, totp.go) plus
internal/config/validate.go (two keys in the positive-integer list).
The delegate's recorded decisions are in the three rounds' return.json
under `whatWasDone` (the implementer schema has no decisions key); a
decision that changes law (a new refusal, a new authority, a schema
the design does not give) is a finding, a mechanical choice recorded
there is not.

# Review brief

Two ordered layers per the code-critique skill. LAYER 1, conformance:
the step is contract-and-adapters only, with NO behaviour change for
today's callers — confirm poll.go still passes `old.Cursor` to
Receive, ignores `Ack` and never calls Confirm; every existing caller
of SplitTOTP still calls it and StripCode has no caller; no verb or
flag was added and the only config change is the two keys at
validate.go's positive-integer list; nothing under internal/goal, the
steward, recovery or internal/channel/phase beyond the two loader
reads (`channel.http-timeout-sec` on the slack and telegram loaders
with default 30 and refusal of non-positive or non-numeric values;
`channel.fake.listener` on the telegram fake face only). Inbound
carries `Ack Cursor` and `UpdateID int64`; Provider gains
`Confirm(ctx, dest, c) error`; `Busy ErrorKind = "busy"` with ErrBusy
shaped like the other constructors. Telegram: updates() sends the
offset only for a non-empty cursor, limit 100, `timeout` from a new
parameter that every existing caller passes as 0; each Inbound's Ack
is update_id+1 as a decimal Cursor and UpdateID the update_id; the
other-chat and own-message filters still drop items; Confirm sends
offset c / timeout 0 / limit 1 and discards, empty c is a nil no-op;
409 "terminated by other getUpdates request" → ErrBusy with that text,
"webhook is active" and unrecognised 409 bodies → ErrReceiveFailed;
the per-request deadline comes from DestinationConfig.HTTPTimeout (0
→ 30 s; long poll T+15 s) applied in request() so a caller-supplied
client with its own timeout still works. Slack: Ack = ts, UpdateID 0,
Confirm nil no-op, per-root cursor unchanged. Fake: bare
`fake-telegram-token` → listener "", `fake-telegram-token-<l>` →
listener l, any other token → listener "" (round 3, restoring main's
acceptance), malformed paths 404; one confirmed offset and one stream
across tokens with the max(current, c) rule; `timeout` T blocks up to
T seconds polling the scripted file at 100 ms; journal rows carry
listener and a monotonic sequence and the offset-carrying getUpdates
is journaled once with method `confirm` and the raw method in `raw`;
control.json (conflict/deliverOnlyTo/pauseBefore) re-read per
request, absent = no controls, malformed → every request 500 with the
parse error. StripCode's four cases and MaskCodes' three from the
brief; the tests the round-1 brief lists under "Tests" are present
and discriminating (for each, name the assertion that fails if the
behaviour is removed). The seat has run go-gate --fast, `go test
-race ./internal/channel/... ./cmd/...` and
scripts/agents/channel-fixtures.sh on the terminal tree, all green.

LAYER 2, adversarial, on the changed code: the fake's confirmed
offset when a listener sends an offset LOWER than the confirmed one
(does max() hide a replay, and is that the design's rule); a
`timeout` T block that spans a control.json rewrite (which controls
apply — the ones at arrival or at release); `pauseBefore` whose
`until` file appears and is removed again before the 100 ms tick;
`conflict` remaining reaching zero mid-batch; `deliverOnlyTo` for an
update id that also sits below another listener's offset; the
telegram 409 classification when the body is not JSON or the
description has different casing; Confirm on a Cursor that is not a
decimal; HTTPTimeout applied to the long poll when the caller's
context already has a shorter deadline (which wins, and does the
error class stay ReceiveFailed); a `sequence` that resets when the
fake restarts (does the journal say so); MaskCodes on a code at the
very end with no trailing punctuation, on a field that is six digits
plus a non-ASCII digit, and on two codes for adjacent time steps
(VerifyTOTP's skew — does masking mask what binding would accept);
StripCode on "123456." (code alone with punctuation) and on a
two-line message whose code is on the first line; the loader's
refusal of `channel.http-timeout-sec=0` versus the validator's
positive-integer rule (same boundary, same wording); any test that
depends on wall-clock time or real sleeps beyond the fake's own
100 ms tick; and whether any assertion binds message text a later
step would have to preserve without a behaviour change.

Materiality criterion, verbatim: would the change ship a defect,
violate its brief, or damage what certifies it? Count only material
findings in the verdict. A finding that indicts the design rather
than the code is reported as such (design point named) and counts.

Run what the sandbox allows: `go build ./...`, `go vet
./internal/channel/... ./cmd/...`, `go test ./internal/channel/...
-count=1`. Report what could not run (the fixture script owns
processes; the cmd package is slow — both were run by the seat).

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
reviewedTree a2e1346a9683f299b4d9628ee584b512e9cf0d03.

# Gap Rule

stop and report a gap; never fill it silently.
