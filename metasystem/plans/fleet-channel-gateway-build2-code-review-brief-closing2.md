Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-05

# Goal

The closing code review, third root, of build step 2 of goal
fleet-channel-gateway (tier 3, box 3d/24/1200m/1/3), implementer
chain fcg-build2 (gpt-5.6-sol; five rounds). The first root
(fcg-build2-cc, metasystem/artifacts/agents/fcg-build2-cc/rounds/1/return.json)
returned one material finding and seven notes; the seat sent F-1 and
F-2 back as round 4. The second root (fcg-build2-cc2,
metasystem/artifacts/agents/fcg-build2-cc2/rounds/1/return.json,
reviewedTree 2504bd49) reviewed the round-4 tree and returned one
material finding, F-1: round 4 changed the post-confirm Receive in
TestTelegramListenersShareStreamAndConfirmedOffset from an empty
cursor to cursor "1", so the design sentence "getUpdates without
offset returns only unconfirmed updates" lost its only test. Its F-2
(the fixtures write control.json in place, not by rename; a reader
can see a torn file) is carried to the cut-over step's fixture brief;
its F-3 is noted. The seat sent F-1 back as round 5
(metasystem/artifacts/agents/fcg-build2/rounds/5/prompt.md, return in
rounds/5/return.json): restore the empty-cursor leg after the confirm
alongside the offset-1 leg, one file, nothing else.

You are a fresh root on the fixed tree: review the terminal round's
computed diff (metasystem/artifacts/agents/fcg-build2/rounds/5/diff.patch,
reviewedTree in rounds/5/review.json —
98f0b886646c3706e7bdd727829161665a2dc9d5) against the five briefs as
delivered (rounds/1-5/prompt.md), the first root's brief as delivered
(metasystem/artifacts/agents/fcg-build2-cc/rounds/1/prompt.md, whose
two layers still apply in full) and the second root's brief as
delivered (metasystem/artifacts/agents/fcg-build2-cc2/rounds/1/prompt.md).
Review the diff, never the delegate's summaries.

# Review brief

LAYER 1, conformance, in this order: (a) round 5 — the one-file
boundary held (fake_test.go only); in
TestTelegramListenersShareStreamAndConfirmedOffset, after the first
listener's Confirm, a Receive by the second listener with the EMPTY
cursor asserts exactly the one unconfirmed update, and the offset-1
leg and both journal assertions round 4 added are unchanged; name the
assertion that fails if the absent-offset branch regressed to
filtering from zero; no other test changed. (b) The whole step
against the first and second roots' layer 1 — it is one diff now;
confirm nothing regressed since reviewedTree 2504bd49 beyond that one
test.

LAYER 2, adversarial, on round 5's leg only: does the empty-cursor
Receive itself advance or create a confirm row (it must not — the
journal assertions count them); does its ordering before the offset-1
leg change what the offset-1 leg proves; is the leg discriminating
against a fake that returns the whole stream for an absent offset.
The two earlier roots' layer-2 lists were applied to everything else
and stand.

Materiality criterion, verbatim: would the change ship a defect,
violate its brief, or damage what certifies it? Count only material
findings in the verdict. A finding that indicts the design rather
than the code is reported as such (design point named) and counts.

Evidence (seat, round-5 tree): `go test ./internal/channel/fake/...
-count=1` ok (9.8 s); `go test -race ./internal/channel/... ./cmd/...`
all six packages ok (channel 105 s, fake 10 s, phase 6 s, slack 4 s,
telegram 8 s, cmd 115 s); conformance review of
round 5 wrote rounds/5/diff.patch (13 files) at reviewedTree 98f0b886.
Report what your sandbox could not run.

# Constraints

Wall-clock budget: 20 minutes. Return per the code-critic schema with
reviewedTree 98f0b886646c3706e7bdd727829161665a2dc9d5.

# Gap Rule

stop and report a gap; never fill it silently.
