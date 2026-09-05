Working Mode: implement
Orchestrator Identity: m1 (lineage main-1788594343-3833-fb64b9, dispatch delegate under goal channel-ask-fits-one-message, approved 2026-09-05 under Wido's relayed word, box 1d/10/240m/1, review rounds 2)
Date: 2026-09-05

# Goal

Bound what an ask posts to the fleet channel so it is one message a human
can act on, and so the reply token can never be pushed out of reach.

Wido's words, 2026-09-05: the Telegram messages are a wall of text, and he
wants that prevented. Tier 2, MECHANICAL: build plus one code review, no
design round.

# The defect, against the code

metasystem/internal/channel/question.go renderQuestion prints every fact,
every option's consequence, and the recommendation verbatim. The facts a
seat passes are goal-record prose — the Next step and basis strings in
metasystem/plans/goals run to thousands of characters. The Telegram adapter
then splits whatever it is handed at a 4000-rune chunkLimit
(metasystem/internal/channel/telegram/telegram.go, splitText), so one ask
becomes several giant messages and the reply instruction with its verbatim
token lands at the bottom of the last one.

The status report already solved its half of this: it caps itself to twelve
lines and trims with oneSentence in metasystem/internal/channel/report.go.
The ask has no such bound.

# What to build

Bound the ask at its renderer, in metasystem/internal/channel/question.go.
The ordering is the design and it binds:

1. The reply instruction and its verbatim token are built FIRST and are
   never trimmed. They stay last in the rendered message and whole, so no
   chunk boundary can separate the instruction from the token.
2. Every option label survives. Dropping an option removes a choice the
   human is being asked to make. Option consequences are trimmed, not their
   labels, and the consequences together take at most half of what is left
   after the head and the tail, so several long options cannot crowd out
   every fact.
3. Facts take what remains, up to a small fixed count. When facts are
   dropped, the message says so and names the goal whose record holds them.
   A trim is visible: a cut line ends in an ellipsis rather than stopping
   mid-sentence as if that were all the seat wrote.
4. The line that admits the trim is itself part of the message, so reserve
   its worst case BEFORE spending anything on facts. Without that reserve
   the bound is announced and then exceeded by exactly the notice that
   admits it.
5. A short ask is unchanged in substance: its facts, option lines,
   recommendation and token all still appear, and it must not claim a trim
   it did not make.

The whole-message bound is a constant in the channel package, not a
provider detail: question.go must not import the telegram package.

# Tests

In metasystem/internal/channel/channel_test.go, both against renderQuestion
directly:

- An ask with eight goal-record-sized facts, three options with
  goal-record-sized consequences, and a goal-record-sized recommendation
  renders under the bound, ends with the exact reply instruction and token,
  contains every option label, says how many facts it dropped and in which
  goal, and contains no untrimmed fact.
- A short ask keeps its head, both facts, its option line, its
  recommendation and its token, and claims no trim.

# Boundary

Touch only metasystem/internal/channel/question.go and
metasystem/internal/channel/channel_test.go. Declare both in diffBoundary
with the metasystem/ prefix. Do not change the Telegram adapter's chunking,
the status report, the inbox records, or any timestamp: local timestamps are
a separate goal (channel-local-timestamps) and must not be folded in here.

Report `go test ./internal/channel/...` green in your return. Any deviation
you find necessary is a GAP to report, never a silent choice.
