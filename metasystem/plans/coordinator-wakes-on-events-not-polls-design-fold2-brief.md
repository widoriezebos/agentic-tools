Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal coordinator-wakes-on-events-not-polls)
Date: 2026-09-13

# Goal

Fold the second rostered read into the landed design page
metasystem/plans/coordinator-wakes-on-events-not-polls-design.md (commit
2009c525a; your worktree is at that tip and the page is a tracked file
there). The read kept CWE-01, CWE-02, CWE-03, CWE-06 and CWE-07 open and
added CWE-10. The seat has decided each below; write the decisions into
the page in plain English, keep every section, at most 260 lines. Where a
decision removes something the page says today, remove it; do not leave
both versions.

# Workspace

Your job worktree, branch agent/<job>. Modify exactly one file, the design
page above. Touch nothing else. Do not commit.

# Inputs: the seat's decisions

CWE-01, the resume route is dropped. There is one route: the seat runs one
foreground `metasystem wait` command and the runtime keeps it open until
its result or deadline. Every roster runtime today (Claude Code, Codex,
Devin, the fake adapter) runs a shell command to completion, so every
adapter answers `blocking`. The adapter's wait-delivery operation stays as
the conformance seam: it takes waitId, nonce, deadline and the session
reference and answers `blocking` with exit 0, or exit 2 to decline; a
decline refuses the registration with a named reason, grants no exemption
and leaves today's watch commands and today's stop decisions in force. A
runtime that cannot hold a foreground command is a follow-on goal, named
in section 6 as managed-seats-wait-through-a-holder, opened when such a
runtime joins the roster; this page does not design it. Remove every
sentence about the resume route, the steward continuation launch as a
wait launcher, delivery requests, delivery acknowledgement by a successor
announcement, and the retained-result-for-a-replaced-seat rule. Recovery
after a restart stays records-based through the rows and `--resume`
(CWE-03 below); it needs no launcher.

CWE-02, the stop table. Row 1: a valid job, run, attempt or landing wait
joined to the seat's claimed goal is work in flight, with exactly the
effect a live delegate job has today on the unwatched-work join, on open
work with the saved signature and on the idle-with-backlog refusal and its
counter; the page adds no counter rule of its own. Delete the sentence
"Freeze refusal counters and signatures only while suppression applies."
Row 2: a valid human-act wait suppresses only open work with the saved
signature and its goal's waiting-on-human condition; it never exempts the
idle-with-backlog refusal, and the waiter returns 6 (actionable work) as
soon as another goal becomes claimable, so the seat takes the turn to
claim it and no forced turn happens while the wait is pending. Restore
that return-6 rule in the row and in the waiter's observation loop. Rows
3, 4 and 5 stay.

CWE-03, restart on runtimes without a session-start hook. Say it plainly:
the records-based recovery DONE clause 4 requires is the rows plus
`metasystem wait --resume WAIT-ID`, which any seat can run at any time
after a restart; the ordering "first act of the new session" is provided
where the runtime has a session-start hook (Claude Code, through the
supervision hook's `start` event and the engine's `session start` verb)
and is an accelerator, not the guarantee. On a runtime without such a
hook the seat's first `goal next` or `report turn-verdict` prints the same
WAITING lines with the resume commands. Name no component that does not
exist. Replace the current recovery paragraph's "Without hooks, plain
report turn-verdict does this scan" with this.

CWE-06, the members. Move the adapter wait-delivery operation (the
`blocking` answer in metasystem/scripts/agents/adapters/runtime-common.sh
and the three runtime adapters plus the fake adapter) into member ONE,
wait-verb-returns-on-recorded-events, so the installed `metasystem wait`
works after member one lands with no temporary fallback. Member two keeps
the publication hints and the compatibility wrappers. Member three,
stop-gate-honours-a-registered-wait, keeps the stop table and the
conformance test of the delivery contract. Rewrite the three DONE rows
and the file lists in section 6 accordingly; say again that no member's
DONE needs a later member, and show it: member one's DONE names the
adapter answer; member two's wrappers call a verb that already works;
member three changes only the stop gate and the contract document.

CWE-07, compatibility. The existing `delegate --wait` mapping is:
completed 0, failed 3, timeout 4, and 5 for a missing record, a malformed
status read and an unknown status alike; cancelled 8. The page must
preserve exactly that: the wrapper maps the new verb's exit 4 (missing,
replaced or invalid source record) to 5, and `job watch` and `run watch`
keep their own current mappings the same way. Correct the numbers in
section 2 and in the compatibility fixture row; the fixture asserts the
mapping as it is today, read from
metasystem/scripts/agents/dispatch.sh, not a new one.

CWE-10, channel wait names its question. The human-act selector for an
answer carries `--question ID`; the saved selector and the ledger
predicate match only an answer act on that goal that names that question;
an answer to another question on the same goal is not the event and the
wait continues. Say this in section 1's twelfth row and in section 2's
human-act paragraph, and add the two-questions case to the
TestWaitChannelAnswer setup in section 5.

# Constraints

- Plain English, short sentences, no bullet padding. At most 260 lines.
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
observing the line count. whatWasDone lists the six findings and where
each was folded, and confirms the resume route is gone from every section.

Every path in your return (diffBoundary, files) is relative to the
repository root, so it starts with `metasystem/`.

# Acceptance Criteria

- The page has one route (blocking), a stop table without a counter rule
  of its own and with the return-6 rule for human-act waits, the
  restart paragraph as decided, the adapter operation inside member one,
  the existing delegate --wait mapping, and channel wait keyed by
  question.
- Every path it cites exists.

# Gap Rule

stop and report a gap; never fill it silently.
