Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal fleet-slack-channel)
Date: 2026-09-03

# Goal

The one design review of the fleet conversation channel, tier 3 under
R-54-m1 (design, one review, one fold, one closing review, build, one code
review). Read metasystem/plans/fleet-slack-channel-design.md (revision 1)
against its adopted law, the decided sections of
metasystem/plans/alert-channel-design.md (§2, §2a, §2b, §3, §3a, §4, §10),
and against the code it names: the package metasystem/internal/humanauthority
and the files metasystem/internal/goal/norm.go (RecordedNormApproval and
the strict token) and metasystem/internal/goal/verbs.go (how history
operations and actors are written); the packages metasystem/internal/steward
(the tick) and metasystem/internal/report and metasystem/internal/usage
(report sources).

# The standard for a finding (R-60-m1, binding)

A finding is material only if it changes what gets built AND names the
artifact (the section, type, verb, key, or test) it changes. Everything
else is a note. Under R-60-m1 the review is a risk budget, not a
completeness pass: at the budget the agreed parts build and any disputed
point becomes a named test obligation, never a raise for another review.

# Attack surface, in priority order

1. The reply path (§5): can any inbound message other than Wido's,
   authenticated by the configured user id AND a valid unconsumed TOTP
   code, reach the ledger as `actor=human:wido`? Name the step. Is the
   durable order (record, history op, thread close) safe against a crash
   between steps, and is a re-poll after the crash idempotent (no second
   history op for the same inbound ref)?
2. The recording (§5): does the `answer` history operation with outcome
   `AUTHENTICATED_CHANNEL_WORD` fit the ledger's existing history-line
   grammar and RecordedNormApproval's token scan without changing the
   grammar? If a change is needed, name it.
3. The provider contract (§2): are five operations enough for Slack AND
   for a Telegram/WhatsApp adapter later without changing the interface?
   Name the missing operation if not. Does `Replies` with a cursor and
   paging match Slack's conversations.replies semantics (oldest is
   inclusive; the root message is returned first)?
4. The report (§3): is every named source durable and present on main
   today (goal ledger, landing trailers, job records, internal/usage)? Name
   any source the design assumes that does not exist.
5. The tick hook (§7): the poll and the cadence run outside the
   arbitration lock under a 15-second bound; can they stall or block the
   tick's existing duties? Name the path.
6. Configuration (§6): any key that collides with an existing key read by
   the package metasystem/internal/config or a secret that could reach a
   committed file or an error string.

# Return

Code-critic schema, findings first, each with section and artifact named
and a one-line proposed change. Then the notes. Then one line: "agreed
parts build as written" or the list of disputed points, each phrased as
a test obligation by name.

# Constraints

Wall-clock budget: 40 minutes. Your sandbox is read-only; verify by
reading. R-31: no benchmarks. Do not redesign; the alert-channel sections
are decided and out of scope.

# Gap Rule

stop and report a gap; never fill it silently.
