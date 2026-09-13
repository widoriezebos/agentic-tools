Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal coordinator-wakes-on-events-not-polls)
Date: 2026-09-13

# Goal

Fold the first rostered read into the landed design page
metasystem/plans/coordinator-wakes-on-events-not-polls-design.md (commit
1f6cb0e27; your worktree is at that tip and the page is a tracked file
there). The read (job wait-design-read-3, findings CWE-01 to CWE-09) is not
in your worktree; the decisions below are complete and you need not read
it. Edit the page in place. The seat has decided every design
question the read raised; your job is to write those decisions into the
page in plain English, keep every section, and keep the page at most 260
lines.

# Workspace

Your job worktree, branch agent/<job>. Modify exactly one file, the design
page above. Touch nothing else. Do not commit.

# Inputs: the seat's decisions, one per finding

CWE-01, the adapter route. The wait verb is one blocking command run by
the seat; nothing is spawned to hold it. At registration the waiter asks
the adapter's wait-delivery operation one question and records the answer
in the row: `blocking` (the runtime can keep one foreground command open
until the deadline, which is every runtime today: Claude Code, Codex and
Devin all run a shell command to completion) or `resume` (the runtime
cannot; then the waiter still blocks in its own process, and when the seat
is gone at the event the waiter persists the result and asks the existing
steward continuation launch, the one the idle-with-backlog handoff uses,
to start the seat's next turn with `metasystem wait --resume WAIT-ID` as
its first command). The operation's contract: input waitId, nonce,
deadline, session reference; output one word, `blocking` or `resume`, exit
0; exit 2 means the runtime declines both, and registration then refuses
with a named reason (the seat keeps today's watch commands). No third
route. Acknowledgement of a resume delivery is the successor session's
announcement record naming the waitId, written by the engine when that
turn starts; nothing else counts. A native wake (a Claude background-task
notification, a Codex or Devin session event) is only a hint into the
pipe; it never chooses the route.

CWE-02, the stop-decision table. Write it as a table in section 2 with
these rows. A valid pending wait is one that passes every eligibility
check the page already lists (this session and lineage, current lease
epoch, live waiter, unexpired, fresh observation, target still pending).
(1) A valid job, run, attempt or landing wait joined to the seat's claimed
goal is work in flight: it satisfies the unwatched-work join for its
target, it suppresses open work whose signature equals the signature saved
at registration, and it exempts the idle-with-backlog refusal exactly as a
live delegate job does today. (2) A valid human-act wait suppresses only
open work with the saved signature and the waiting-on-human condition of
its goal; it never exempts the idle-with-backlog refusal. (3) A wait that
fails any eligibility check changes nothing: today's decisions apply. (4)
Open work whose signature differs from the saved one is never suppressed;
the wait returns 6 on its next observation so the seat takes the turn, and
the stop gate then decides as today. (5) Fences, human stop authority,
warnings and degraded-input behaviour are unchanged in every row. The
open-work signature is the digest the gate already computes over the open
items; say that a wait covers exactly the items present at registration
and nothing that appears later.

CWE-03, who resumes. The seat's session start (the `start` event of the
supervision hook, which the engine serves through `session start`, and
the plain `report turn-verdict` path for runtimes without hooks)
enumerates this checkout's waiter rows whose owner lineage the new session
succeeds under the lease records, and prints one WAITING line per pending
row with the exact `metasystem wait --resume WAIT-ID` command. The model
issues that command as its first act; nothing launches a model turn by
itself except the `resume` route of CWE-01. Resume takes the waiter lock,
writes `resumedBy` (session, process birth) into the row, and a second
resume of the same row returns 64 while the first is live. Restart
recovery therefore needs no daemon and no journal: the rows are the
queue.

CWE-04, pending hours. Pending hours are the union of the intervals during
which a wait row is pending, from its registration to the moment the
waiter clears pending (event, deadline or failure). A renewal is a new
registration; the gap between a deadline return and the renewal is NOT
pending, and the renewal itself is counted as one model wake-up. Rewrite
the measurement paragraph so the arithmetic follows; the fixture asserts
that a gap is excluded and a renewal is counted.

CWE-05, the record. Version 2 adds `schemaVersion: 2`; a row without the
field is legacy and is read only for legacy watch liveness. The file name
stays kind, target identifier and owner digest. Resume by waitId reads a
pointer file `artifacts/agents/waiters/by-id/<waitId>` that holds the
row's file name, written under the waiter lock at registration and
removed with the row; a missing pointer is a named error (the row may
still be found by its key). waitId is 128 random bits; a live row already
present for the same key returns 64, never a second row.

CWE-06, the members. Replace section 6's slice labels with three member
goals, each with its own DONE and its own fixtures from section 5:
- wait-verb-returns-on-recorded-events: DONE when `metasystem wait
  --job`, `--run`, `--attempt` and `--goal` (landing and human-act) each
  return on the recorded event or the bounded deadline with the typed
  exits, the version-2 row exists with its pointer, and a restart
  re-registers from the rows alone through `--resume` (clauses 1 and 4).
  Fixtures: the three clause-1 rows, the recovery replay, the failure
  bounds.
- publication-owners-hint-the-waiter: DONE when the job transition owner,
  the attempt terminal commit and the confirmed ledger publication each
  hint the waiter after their durable write, a dropped or false hint
  changes no result, and `job watch`, `run watch` and `delegate --wait`
  keep their public exit mappings through wrappers over the verb (clause
  2's engine half and CWE-07). Fixtures: the accelerator tests of
  fixture row 4 that concern hints, the compatibility mapping test.
- stop-gate-honours-a-registered-wait: DONE when the stop decision table
  holds on the fake adapter and on Claude, the adapter wait-delivery
  operation records `blocking` or `resume` for every roster runtime, and a
  resume delivery is acknowledged by the successor's announcement (clauses
  2's adapter half, 3 and 4's delivery half). Fixtures: the stop rows, the
  delivery contract test, the restart bed leg with real process death.
Clause 5 (the measurements on two whole-goal sessions) stays the parent's
own clause, taken after the third member lands, with the real-runtime legs
named in section 5. Say explicitly that no member's DONE needs a later
member.

CWE-07, compatibility. `delegate --wait` keeps its exit mapping (completed
0, failed 3, timeout 4, vanished 5, unknown status 6, malformed 7,
cancelled 8) through a wrapper in metasystem/scripts/agents/dispatch.sh
that calls the new verb and remaps; `job watch` and `run watch` keep
theirs the same way. The new exits belong to `metasystem wait` alone.

CWE-08, channel wait. Add `channel wait` (metasystem/cmd/metasystem/main.go,
the `wait` verb of the channel family, implemented in
metasystem/cmd/metasystem/channel_verbs.go) as the twelfth site in section
1: it waits on a durable human-question record and polls the provider,
unbounded by default. Under this design it is a human-act wait on the
question's goal with verb `answer`: it registers a row, takes the 24-hour
default deadline, and the stop gate sees it as row (2) of the table.

CWE-09, plain words. Rewrite these passages so a reader outside the
repository can follow them, without changing what they decide: the record
description (owner digest, boot-relative deadline, open-work signature:
say what each is and who computes it), the ledger observation (say what
the acceptance gates and the commit trailers are in one sentence each),
the adapter and stop sections (spell out the two routes and the table in
full sentences), and the measurement rules (say what a prompt token and an
inference start are in this repository's terms).

# Constraints

- Keep the page's structure and every existing decision the read did not
  touch. Plain English, short sentences, no bullet padding. At most 260
  lines.
- Every path you cite must exist in the worktree under the metasystem/
  prefix. No globs. Do not write code. Do not run bin/metasystem test run,
  test plan or test verify.
- Wall clock: 30 minutes. Stop and report if the fold is not finished by
  then.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json):
jobId, round, runtime, sessionId, model, evidence, gaps, mode, riskiestPart,
diffBoundary, whatWasDone. evidence carries exactly two `{command, observed,
level}` items, replayable verbatim from the worktree's repository root: (1)
`git -C metasystem status --short` observing the one modified file; (2)
`( cd metasystem/plans && wc -l coordinator-wakes-on-events-not-polls-design.md )`
observing the line count. whatWasDone lists the nine findings and where
each was folded (section and paragraph).

Every path in your return (diffBoundary, files) is relative to the
repository root, so it starts with `metasystem/`.

# Acceptance Criteria

- The page carries the stop-decision table, the two adapter routes with
  their contract, the resume initiator, the pending-hours rule, the
  version-2 record with its pointer, the three members with their own
  DONE and fixtures, the compatibility wrappers, and channel wait as the
  twelfth site.
- Every path it cites exists.

# Gap Rule

stop and report a gap; never fill it silently.
