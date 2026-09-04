Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-05

# Goal

The closing code review, second root, of build step 2 of goal
fleet-channel-gateway (tier 3, box 3d/24/1200m/1/3), implementer
chain fcg-build2 (gpt-5.6-sol; four rounds). The first root
(fcg-build2-cc, metasystem/artifacts/agents/fcg-build2-cc/rounds/1/return.json)
returned one material finding, F-1 (the fake's deliverOnlyTo control
was evaluated from a stale in-memory copy on every tick of a blocked
long poll), and seven non-material notes; the seat accepted F-1 and
F-2 (an offset below the confirmed offset replayed confirmed rows;
the design's rule is "offset c forgets everything below c", design
line 949-950) and sent both back as round 4
(metasystem/artifacts/agents/fcg-build2/rounds/4/prompt.md,
return in rounds/4/return.json). Dispositions of the first root, in
short: F-1 and F-2 accepted (round 4); F-3 (journal sequence restarts
with the fake) and F-6 (three clock-dependent tests) accept-risk; F-4
(two sentences for one boundary) and F-8 (bound message texts) noted;
F-5 (absent-key detection by error text in the fake loader) carried to
the cut-over step; F-7 (unmatched.jsonl persists raw text) accepted and
retired by the cut-over's migrate. The full table lands with the step.

You are a fresh root on the fixed tree: review the terminal round's
computed diff (metasystem/artifacts/agents/fcg-build2/rounds/4/diff.patch,
reviewedTree in rounds/4/review.json — 2504bd49ce52986bd47868a190367346f11049a0) against the
four briefs as delivered (rounds/1-4/prompt.md), the first root's
brief as delivered (metasystem/artifacts/agents/fcg-build2-cc/rounds/1/prompt.md,
whose two layers still apply in full) and the design points it names.
Review the diff, never the delegate's summaries.

# Review brief

LAYER 1, conformance, in this order: (a) round 4 — the controls are
reloaded under the lock on each tick before the delivery filter reads
DeliverOnlyTo; pauseBefore and conflict remain arrival-time; a
malformed control file mid-poll ends that request with the 500 and
the parse error; the offset filter uses max(confirmedOffset, c) and
confirmedOffset only rises; the two new tests are discriminating
(name the assertion that fails if the reload or the max() is
removed); the modified TestTelegramListenersShareStreamAndConfirmedOffset
still proves what it proved (one stream, one offset across tokens)
and its new offset-1 leg proves F-2; no other existing test changed;
the two-file boundary held. (b) The whole step against the first
root's layer 1 — it is one diff now; confirm nothing regressed.

LAYER 2, adversarial, on round 4's code: a reload that fails on one
tick after succeeding at arrival (which error wins, is the 500 body
the parse error); the lock held across the reload and the update
selection (is the file read while holding s.mu, and can a pause
release or a scripted-file append deadlock against it); a
deliverOnlyTo that RESTRICTS the update to the blocked listener
itself mid-poll (does it wake within the tick); the max() rule when
the caller's offset is above the confirmed offset AND the response
would be empty (does confirmedOffset advance before or after the
filter, and does Telegram's "the one it may return stays unconfirmed"
still hold); a listener that sends an offset below its own last
confirm after a restart; plus the first root's layer-2 list on
anything round 4 touched.

Materiality criterion, verbatim: would the change ship a defect,
violate its brief, or damage what certifies it? Count only material
findings in the verdict. A finding that indicts the design rather
than the code is reported as such (design point named) and counts.

The seat has run, on the round-4 tree: `go build ./...`, gofmt, `go
vet`, `go test -race ./internal/channel/... ./cmd/...`,
scripts/agents/channel-fixtures.sh and scripts/agents/go-gate.sh
--fast — report the results you read in this brief's Evidence section
below as the seat's, and what your sandbox could not run.

Evidence (seat): `go test -race ./internal/channel/...` all five packages ok (channel 124 s, fake 11 s, phase 6 s, slack 4 s, telegram 9 s); `go test -race ./cmd/...` ok (133 s); scripts/agents/channel-fixtures.sh "channel fixtures: PASSED"; scripts/agents/go-gate.sh --fast "fast mode passed (gofmt, vet, staticcheck, build)"; conformance review of round 4 wrote rounds/4/diff.patch at reviewedTree 2504bd49.

# Constraints

Wall-clock budget: 25 minutes. Return per the code-critic schema with
reviewedTree 2504bd49ce52986bd47868a190367346f11049a0.

# Gap Rule

stop and report a gap; never fill it silently.
