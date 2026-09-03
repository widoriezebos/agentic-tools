Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal fleet-slack-channel)
Date: 2026-09-03

# Goal

The one design review of slice 2 of the fleet conversation channel, tier 3
under R-54-m1 (design, one review, one fold, one closing review, build,
one code review). Read metasystem/plans/fleet-slack-channel-slice2-design.md
(revision 1) against the base design
metasystem/plans/fleet-slack-channel-design.md (revision 4; its §2 is the
fixed contract and this slice claims to change no line of it) and against
the landed code it names: metasystem/internal/channel/channel.go (the
contract, the registry it retires), metasystem/internal/channel/poll.go
(what Poll hands to Receive, the cursor record, the rejection receipt,
the per-ref dedupe), metasystem/internal/channel/question.go (Rejection),
metasystem/internal/channel/slack/slack.go (Receive pages by root),
metasystem/internal/channel/fake/fake.go (the one server it extends),
metasystem/internal/channel/phase/phase.go and
metasystem/cmd/metasystem/channel_verbs.go (the two switches it merges),
and metasystem/scripts/agents/channel-fixtures.sh (the fixture it extends).

# The standard for a finding (R-60-m1, binding)

A finding is material only if it changes what gets built AND names the
artifact (the section, type, verb, key, or test) it changes. Everything
else is a note. The review is a risk budget, not a completeness pass: at
the budget the agreed parts build and any disputed point becomes a named
test obligation, never a raise for another review.

# Attack surface, in priority order

1. Exactly-once under Telegram's offset (§3): the design claims the
   persisted cursor is the acknowledgment because the adapter only ever
   sends the persisted offset. Is there any path where Poll persists a
   cursor while an envelope from that page is not durably disposed, or
   where a re-read page yields a second history operation for the same
   reply (poll.go's dedupe by matched refs, recorded rejections and
   unmatched.jsonl)? Name the step. Does the "more than the budget
   arrived" argument hold when the page has exactly 100 updates and none
   is disposable?
2. Reply-to-root resolution (§2, §3): Poll now passes every posted ref in
   the thread with its root in ThreadID and records the receipt's ref on
   the Rejection. Can a reply to a receipt be attributed to the wrong
   question? Can the Slack adapter's wire bytes change (it must dedupe by
   root; base §8's TestReceivePagesAndFiltersByCursor must stay green
   unchanged)? Is a receipt whose post succeeded but whose record crashed
   before writing a loss of anything more than a hint?
3. The one loader (§2): does phase.Load preserve every refusal of the two
   switches it replaces (absent adapter silent; fake not serving typed;
   committed secret reported and ignored; unknown adapter named), and does
   the human user-id key by face keep the existing fixture running with
   channel.human.slack.user-id under adapter=fake? Name any caller of
   loadChannel or the private load that the design forgets (grep the
   command file for every channel verb).
4. The secret in the URL (§3): the Telegram token is a path segment.
   Name any error path (transport error, redirect, JSON decode, 4xx body
   echoing the token, the peek command) where the token could reach an
   error string or stdout unscrubbed, and whether channel.Scrub on
   dest.Secrets covers it or a test must prove it.
5. The fake's Telegram face (§4): can one server with two faces on one
   counter produce an ordering the Slack tests do not expect? Are the
   scripted-reply and journal shapes enough for the fixture's assertions
   in §7? Name any Bot API field the adapter reads that the fake does not
   emit.
6. Chunking (§3, D7): does splitting a long status post change any
   assertion of base §8's report tests or the status-state digest?

# Return

Code-critic schema, findings first, each with section and artifact named
and a one-line proposed change. Then the notes. Then one line: "agreed
parts build as written" or the list of disputed points, each phrased as
a test obligation by name.

# Constraints

Wall-clock budget: 40 minutes. Your sandbox is read-only; verify by
reading. R-31: no benchmarks. Do not redesign: base §2 is fixed and
WhatsApp is out of scope.

# Gap Rule

stop and report a gap; never fill it silently.
