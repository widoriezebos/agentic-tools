Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal fleet-slack-channel)
Date: 2026-09-03

# Goal

The closing design review of slice 2 of the fleet conversation channel,
tier 3 under R-54-m1: the review after the one fold. Read
metasystem/plans/fleet-slack-channel-slice2-design.md revision 2, whose
§9 lists how each of round 1's eight findings (FSC2-R1-001 to 008, your
own round fsc2-design-crit1 at
metasystem/artifacts/agents/fsc2-design-crit1/rounds/1/return.json) was
folded, against the base design
metasystem/plans/fleet-slack-channel-design.md (revision 4, §2 fixed) and
the landed code: metasystem/internal/channel/poll.go,
metasystem/internal/channel/question.go,
metasystem/internal/channel/slack/slack.go,
metasystem/internal/channel/fake/fake.go,
metasystem/internal/channel/phase/phase.go,
metasystem/cmd/metasystem/channel_verbs.go.

# The standard for a finding (R-60-m1, binding)

A finding is material only if it changes what gets built AND names the
artifact. This is the closing review: its output is build law. A fold
that does not close its finding is a finding; a fold that opens a new
hole is a finding; everything else is a note. At the budget the agreed
parts build and any disputed point becomes a named test obligation
(already in §7 or added by name), never a raise for another review.

# Questions, in priority order

1. FSC2-R1-001: with the Telegram Ref built from the update's own fields
   (message_id, reply_to_message.message_id) and Inbound.ThreadID the
   resolved root, does every dedupe in poll.go (matchedRefs on
   Answer.Ref, alreadyRejected on Rejection.Ref, unmatchedAlready on
   the unmatched row's Ref) now hold across the crash-after-answer
   replay? Does the consumed row (ThreadID from Inbound.ThreadID) still
   scope the TOTP step to the envelope as base §5 requires?
2. FSC2-R1-002: record, post, rewrite. Is the at-most-once receipt and
   the declared orphan outcome consistent with base §5's "each invalid
   inbound ref answered once", and does `alreadyRejected` skip the ref
   on replay before any post? Name the step if not.
3. FSC2-R1-003/004/005: the loader's `withHuman`, the caller list, the
   nil-provider rule, and the resolver returning the face. Any caller
   or refusal still missing?
4. FSC2-R1-006/007/008: the fake's face discriminator and emitted
   fields against what §3's Receive reads; `Adapter.Peek` on the shared
   request path; the hard split.
5. Anything in revision 2 that changes base §2's signature or the
   Slack adapter's wire bytes.

# Return

Code-critic schema, findings first, each with section and artifact named
and a one-line proposed change. Then the notes. Then one line: "agreed
parts build as written" or the list of disputed points, each phrased as
a test obligation by name.

# Constraints

Wall-clock budget: 30 minutes. Your sandbox is read-only; verify by
reading. R-31: no benchmarks. Do not redesign: base §2 is fixed and
WhatsApp is out of scope.

# Gap Rule

stop and report a gap; never fill it silently.
