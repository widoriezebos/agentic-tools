Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal alert-escalation-channel)
Date: 2026-09-01

# Goal

Revision 8 of metasystem/plans/alert-channel-design.md: fold or refute, by id,
all nine material findings of the Sol round-1 critique of revision 7. The
verbatim register is landed at records/misc/alert-channel-critique-r7.md
(readable in your worktree); the critic's full return with quoted evidence is
durable at
artifacts/agents/design-critic-563ae99ff0a1c5e082e659fb/rounds/1/return.json.

# Workspace

The delegate worktree the dispatcher created for this job. Revise exactly one
file: metasystem/plans/alert-channel-design.md (revision 7 is in the
worktree). Read anything; the shipped code the critic cited
(internal/steward/alert_episode.go, internal/dispatch/record.go,
internal/dispatch/stop.go, internal/supervise/reaper.go) is your ground truth
for what exists today.

# The mandate

Address every finding decisively — a fold that changes the design, or a
refutation quoting the text and code that prove the critic wrong. No finding
may be narrowed silently. The critical one first:

- AC7-PRODUCER-ATOMICITY-001 (critical): neither new slice-1 producer has a
  recoverable handoff into the episode store. Design direction to consider
  seriously, not a mandate: derive the alert from the durable SOURCE state
  (the terminal job record, the landed stop record) by an idempotent scan at
  tick time, instead of requiring producers to dual-write — that obeys the
  design's own episode-store source-of-truth law and makes a missed write
  self-healing. If you choose differently, state the crash-window proof.
- AC7-PRODUCER-STATE-001: give the two producers a durable discriminator and
  payload shape in the episode schema (what exists today: one opaque Digest,
  one Message string — say exactly what slice 1 adds or reuses).
- AC7-STOP-OUTCOME-001: the stop-awaiting-resume class must alert only on
  outcomes that prove a fence closed and resume is required; enumerate the
  stop outcomes and their alert verdicts.
- AC7-JOB-WRITER-001: RecordProtocolError terminalizes failures outside
  RecordCAS; wire every terminal-failure writer into the class or state which
  are excluded and why that is safe.
- AC7-MESSAGEREF-PERSISTENCE-001: name the exact persisted AlertAttempt field
  for the provider message reference.
- AC7-SEND-OUTCOME-001: make sections 2 and 11a.3 agree on the
  unconfigured-destination outcome — one contract, stated in both.
- AC7-COMPOSER-BYTES-001: define the composed message's uncut portions
  byte-exactly, not only the tail.
- AC7-DEDUP-ENCODING-001: define how each dedup tuple becomes the episode
  store's required 64-hex SHA-256 digest (field order, separator, encoding).
- AC7-TICK-ERROR-PATH-001: assign the external tick driver's failed-RunTick
  branch an explicit DeliverDueAlerts behavior.

Then: re-run the self-consistency pass over every changed rule and its touched
sections, name the pairs in the status line, and update the self-grade — the
reject condition stays a third implementer gap-stop.

# Constraints

Wall-clock budget: 35 minutes. No design content changes beyond what the nine
resolutions and the consistency pass require. Wido's standing design words are
untouchable: adapter abstraction, Telegram first, the session bridge as second
consumer, Slack threading via conversation identity, and the two slice-1
producer classes themselves.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/alert-channel-design.md; whatWasDone maps each finding id to
fold or refutation in one line each.

# Gap Rule

stop and report a gap; never fill it silently.
