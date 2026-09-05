Working Mode: implement
Orchestrator Identity: m1 (lineage main-1788594343-3833-fb64b9, follow-up round on chain implementer-1fb40275a06e386262ce7d0b, goal channel-ask-fits-one-message)
Date: 2026-09-05

# What this round is

The fold of code-critic-aa235e0ddd1ff4832da65826's review of your build.
One material finding and two accepted minor ones. Keep everything else
exactly as it is: the trimming machinery, the reserve, the priority order
and both tests are right.

# F-1, material: the bound is the provider's limit restated

questionMessageRuneLimit is 4000, the same number as chunkLimit in
metasystem/internal/channel/telegram/telegram.go. That prevents the split
but not the wall: a 4,000-rune ask is roughly a hundred lines on a phone,
which is the thing Wido asked to stop.

Set it to 1600. The critic's arithmetic for what an ask actually needs at
one sentence per item: head 143, tail 160, notice 112, three options 441,
recommendation 167, four facts 572 — 1,595 total, so 1,600, which is about
twelve to fourteen phone lines and matches the twelve-line cap the status
report has carried since it was written
(metasystem/internal/channel/report.go).

The constant is a channel-level bound on what a human is asked to read. It
must not be derived from, or equal to, any provider's chunk limit. Say so
where it is declared, so the next reader does not "fix" it back.

# F-2, accepted: a trim notice that dropped nothing says so

metasystem/internal/channel/question.go:277 renders the notice on every
trimmed path, so an ask trimmed only in its options prints "dropped 0
facts". It is not false, but it is awkward, and at 1600 it becomes common.

Say what was actually trimmed. When no fact was dropped the notice must not
claim a fact count; when facts were dropped it keeps naming how many and
which goal holds them.

# F-3, accepted: restore the layout

The proposed box line moved into the head, directly after the goal line;
before this change it was rendered after the options and before the
recommendation. Substance was preserved but the order was not, and nobody
asked for the move. Put it back where it was.

# Not to change

F-4: when option labels alone exceed the bound the message exceeds it. That
follows from the priority the brief set — labels are never trimmed, because
dropping one removes a choice — and it is correct as built. Leave it.

F-5: the short-ask test passes against the old renderer. It is the
regression guard, which is what it was asked to be. Leave it.

# Tests

The long-ask test must keep asserting against the constant rather than a
literal, so lowering the bound tightens the test automatically. Add the case
F-2 names: an ask pushed over the bound by long options with few short
facts, asserting the notice does not claim a dropped fact count. Keep both
existing tests.

Report `go test ./internal/channel/...` green. Boundary is unchanged: only
metasystem/internal/channel/question.go and
metasystem/internal/channel/channel_test.go.
