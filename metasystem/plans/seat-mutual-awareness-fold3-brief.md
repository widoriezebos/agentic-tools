Working Mode: design
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal seat-mutual-awareness)
Date: 2026-09-06

# Goal

Revision 3 of the design for goal seat-mutual-awareness, the last
revision the ladder allows before the build. Revision 2
(metasystem/plans/seat-mutual-awareness-design.md, commit 2e109816)
went to the second design review (job sma-crit2-20260906); its register
is metasystem/records/misc/seat-mutual-awareness-critique-r2.md: ten
material findings, every one accepted. Answer all ten in the design and
extend the dispositions table at the end with a second round, one row
per finding. Your earlier briefs
(metasystem/plans/seat-mutual-awareness-design-brief.md,
metasystem/plans/seat-mutual-awareness-fold2-brief.md) still bind. Wido's
inbound rule and the no-authority-for-seats rule are not up for
redesign.

# Workspace

The delegate worktree the dispatcher created for this job. Write
exactly one file, the existing design under metasystem/plans (revision
3 in place).

# The ten findings and the constraint on each answer

- SMA-C-11 (an engine cannot fence an old engine; the marker is not
  fail-closed). Constraint: stop claiming a code fence against old
  checkouts. State the truth: old engines are kept out by the FLEET
  ORDER (every machine pulls and rebuilds before the enable act, and a
  pull rebuilds the engine by the engine-rearm law landed today), by
  the fact that no old verb writes under the seat directory (only an
  agent landing a new plan record by hand, which the guard already
  makes a deliberate acknowledged act), and by the upgraded validator
  refusing a spoiled tip. Make the enable marker unforgeable by content
  the validator can check: it carries the opid of the human's own
  transaction and the enrolled terminal's identity the way approvals
  record `authority=proven`, and the validator refuses a marker whose
  authority is not proven, so an old landing cannot produce one that
  upgraded writers accept. Say what remains a residual and who accepts
  it (Wido, at approval of the build).
- SMA-C-12 (no executable escape from a spoiled tip). Constraint: name
  a repair verb (human-only, at the terminal) that removes or corrects
  a seat record through the ledger's own transaction with a named
  intent, exempt from the seat fences by that intent, and give its
  recovery case; the design cites how `goal repair --accept-remote`
  and `goal recover` are shaped today.
- SMA-C-13 (the enable verb's slice and its human name). Constraint:
  put `seat enable` and `seat repair` in slice 1 (the fences land with
  their human escape), and take the human's name the way every human
  verb does today (`--by`, prefixed human: by the verb; the terminal
  proof supplies the class, the flag supplies the name).
- SMA-C-14 (recovery not total: enable and retire). Constraint: every
  writer, enable and retire included, has a journal intent that
  rebuilds its record, a request-constructor case, named absent-commit
  outcomes and a recovery fixture; a human act that cannot be replayed
  from its journal entry is rejected with a message naming the terminal
  act to repeat.
- SMA-C-15 (the deadline is not a single durable boundary). Constraint:
  decide who owns expiration when no waiter runs (the target's tick,
  the asker's tick, or both, first commit wins) and make not-mine and
  withdraw after the deadline into named outcomes that lose to
  expiration or are recorded as late; every post-deadline transition
  has exactly one outcome.
- SMA-C-16 (a refresh can undo retirement). Constraint: a retired
  membership record is immutable except by a human un-retire act;
  refresh preserves retiredAt and refuses to change a retired record.
- SMA-C-17 (the lineage still does not reach the runner). Constraint:
  name the transport (an argument to the detached runner, or a file the
  arm writes and the runner reads), what armedLineage means when there
  is no lease at arm time (a named value, not a guess), and cite the
  launcher and the runner writer by file and line.
- SMA-C-18 (validator allows updatedAt later than tickAt). Constraint:
  the refusal row matches the schema (equality, or a stated tolerance
  with its reason).
- SMA-C-19 (the proof matrix misses the riskiest cases). Constraint:
  add the named cases: a forged marker refused; recovery from a spoiled
  tip through the repair verb; enable and retire recovery; retire then
  up; waiter-absent expiration; post-deadline not-mine; an answer in the
  deadline second.
- SMA-C-20 (the box is not honest). Constraint: count every job
  reservation (each build attempt and each review round reserves one
  attempt and 120 job-minutes in the accounting), state attempts and
  job-minutes for the whole build with the two review chains, and
  state the elapsed days; the goal's box is now one day, ten attempts,
  720 job-minutes, three review rounds (one attempt and 120 minutes of
  it are used); name the raise as a complete five-part tuple.

Ground every changed claim in file-and-line evidence; keep the
self-grade current. Plain English throughout.

# Constraints

Wall-clock budget: 45 minutes. Do not edit anything but the design
file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file.

# Gap Rule

stop and report a gap; never fill it silently.
