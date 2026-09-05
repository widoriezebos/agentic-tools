Working Mode: code-critique
Orchestrator Identity: m1 (lineage main-1788594343-3833-fb64b9, dispatch delegate under goal channel-ask-fits-one-message, tier 2 MECHANICAL: build plus one code review, no design round)
Date: 2026-09-05

# What you are reviewing

The implementation chain rooted at implementer-1fb40275a06e386262ce7d0b,
built against plans/channel-ask-fits-one-message-build1-brief.md. Review the
chain's produced change, not the main working tree.

# What the change is for

An ask posted to the fleet channel was a wall of text: renderQuestion in
metasystem/internal/channel/question.go printed every fact, every option's
consequence and the recommendation verbatim, the facts being goal-record
prose thousands of characters long, so the Telegram adapter split one ask
into several 4000-rune messages and the reply token landed at the bottom of
the last one.

# The invariants the build had to hold, in priority order

1. The reply instruction and its verbatim token are built first, never
   trimmed, and stay whole and last so no chunk boundary can separate the
   instruction from the token.
2. Every option label survives, because dropping an option removes a choice
   the human is being asked to make. Consequences are trimmed instead, and
   together take at most half of what remains after head and tail.
3. Facts take what is left, up to a small fixed count, and a drop is stated
   with the goal that holds them. Cuts end in an ellipsis rather than
   stopping mid-sentence as if that were all the seat wrote.
4. The line that admits the trim is reserved BEFORE any fact is spent.
5. A short ask is unchanged in substance and claims no trim it did not make.
6. question.go does not import the telegram package; the bound is a channel
   constant, not a provider detail.

# Attack these specifically

- Arithmetic: can any input make the rendered message exceed the bound?
  Long option labels, an empty options slice, one option with a
  thousand-rune label, zero facts, a single fact longer than the whole
  budget, a nil Budget, an empty Recommendation, and multi-byte runes
  throughout — the counting must be runes, not bytes, everywhere.
- Division by zero or negative limits when the budget is already spent by
  head and tail alone.
- Whether a trim can silently drop the token, an option label, or the
  goal id in the drop notice.
- Whether the tests actually fail without the production change, or pass
  vacuously. Say so if a test would pass against the old renderer.
- Whether anything outside the declared boundary changed: the Telegram
  adapter's chunking, the status report, inbox records, or timestamps
  (timestamps are a separate goal and must not appear here).

# Return

Material findings only, each with file:line evidence and a concrete input
that produces the wrong output. Confirm or refuse each invariant above by
number. Report whether `go test ./internal/channel/...` is green on the
chain's tree.

# The question this round exists to settle

The build reports it keeps the ask "within 4,000 runes", which is the
Telegram adapter's own chunkLimit (metasystem/internal/channel/telegram/telegram.go).
That prevents the split. Ask whether it achieves the goal: Wido's words were
that the messages are a wall of text, and a single 4,000-rune message on a
phone is still a wall. The goal record's DONE says "one bounded message a
human can act on", not merely "one message".

Judge and say plainly: is the chosen bound the right one, and is it a
channel-level constant or the provider's limit restated in the channel
package? If you think it should be materially smaller, say what the ask
actually needs to carry — head, the options, the recommendation, the token —
and what number follows from that, with the arithmetic.
