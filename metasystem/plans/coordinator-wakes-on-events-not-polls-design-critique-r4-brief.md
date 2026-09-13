Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal coordinator-wakes-on-events-not-polls)
Date: 2026-09-13

# Review brief: the fourth read Wido granted, for CWE-11 alone

FINDING IDS: chain-unique across this goal's critic chains; CWE-01 to
CWE-11 are taken. CWE-11 is returned under its OWN identifier with
`material: false` if resolved, `material: true` if it still stands; a new
defect gets CWE-12 onward. Never report a resolution as prose inside
another finding.

The "Declared Outputs" section the dispatcher appends below names the
outputs manifest and gives that manifest's SHA-256 digest; the design
page carries no declared digest.

Why this review exists: the third rostered read found CWE-11, the
channel-answer handoff. The page required a human-act cursor captured
before asking, but `channel wait` begins after `channel ask`, and the
question record stored no cursor, so an answer published while the wait
established its cursor could fall before it and the wait would run to its
deadline despite a durable answer. The seat folded it into the page:
`channel ask` records in the question record the accepted ledger tip it
read just before publishing the question, and the answer wait takes its
cursor from that record, never from the tip read at registration. Member
one, wait-verb-returns-on-recorded-events, has since built and landed it
(commit 57ba0586, with TestWaitChannelAnswer covering the cursor, the poll
and an answer that lands between ask and wait), and the page's section 6
records what that build fixed. Wido granted a fourth read over the norm
for this finding rather than an accepted risk.

Round budget: 1 focused round. A finding is material only if an
implementer working from this page would build something different or
wrong because of it, and it names the artifact it would change.

Specification under review:
metasystem/plans/coordinator-wakes-on-events-not-polls-design.md. Contract:
metasystem/plans/goals/coordinator-wakes-on-events-not-polls.md. Code the
fold is built in: metasystem/internal/channel/question.go,
metasystem/cmd/metasystem/channel_verbs.go,
metasystem/cmd/metasystem/wait_verb.go.

# Mandate

1. CWE-11: does the page now decide the channel-answer handoff so an
   implementer builds it without guessing, including an answer that lands
   between `channel ask` and `channel wait`, and a legacy question record
   without a cursor? Return it under its identifier.
2. Nothing else. Members two and three are not under review here.

# Expected Return

The design-critic return schema (version 3), findings sorted by
materiality, each with file:line evidence and its rigor row. Every path in
your return is relative to the repository root, so it starts with
`metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
