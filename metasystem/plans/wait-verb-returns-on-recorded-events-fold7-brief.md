Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal wait-verb-returns-on-recorded-events)
Date: 2026-09-13

# Goal

Round 8 of the build of member 1: fix the two material findings of an
independent Opus read of round 7. The goal, the design page
metasystem/plans/coordinator-wakes-on-events-not-polls-design.md and the
return schema are those of round 1's brief; the workspace is round 2's
widened one, and this round needs metasystem/internal/run/waiter.go,
metasystem/internal/goal/attention.go and their tests. Build on the
worktree as round 7 left it. Keep round 7's replay floor (the row's last
checked tip, which for a published goal match is the matched commit) and
the seat's renewal-floor behaviour with its subtests. Do not commit.

# The contract for this round

Section 6 of the page states the limit this member accepts:
registration and resume project every accepted change since their cursor
floor in one observation, and "hours-old cursors on a busy ledger return
65". So a replay or resume whose ledger walk cannot finish inside its
budget must answer 65 (storage or transport failure), an interrupted one
130, and only a source that really is missing, replaced or invalid answers
4. This round makes the code and the tests keep that contract. It does not
change the cost model of the walk.

# The findings, as the reader wrote them

R7-01 (high, reproduced). A replay now starts at the act commit, so the
real human-act observer reads every accepted change after the act: about
147 ms per change on this repository, with under five seconds of Git
budget in the ten-second replay context (`waitGit` refuses a call when
less than `boundedCaptureGrace` remains), so about 30 changes. When time
runs out mid-walk, the error from `projectWaitAt` comes back as an
invalid-source observation with a nil error (the untyped returns in
`ObserveLedgerForWait` around the intervening-state reads), so the
replay's new 65/130 branch never runs and the invalid-source check
answers 4, "the saved result no longer matches its source evidence":
the WVB-55 symptom. Ctrl-C during the walk also answers 4 instead of 130.
The same holds for a saved 6. Landing waits return their error and get
65. Reproduced on a real clone and origin: 40 changes after the act with
300 ms of injected latency per `ls-tree` answered exit 4 after 5.0 s; a
cancel after 1.5 s answered 4.

R7-02 (medium). The round 7 tests pass whether or not the fix works. The
counting test charges 60 reads only when the floor equals the original
cursor, so it cannot see the cost of changes after the act; the
evidence-failure test's fake returns Temporary with DeadlineExceeded,
which the real human-act observer does not do; and no test replays a
saved goal result through `ObserveLedgerForWait`.

# What the fix must satisfy

- The goal observer reports its own deadline, an exhausted Git budget,
  or a cancelled context during the walk as a failure the waiter maps to
  65, or to 130 on cancellation, on every path that uses it: a running
  wait, registration, renewal and replay. It reports invalid-source only
  for a ledger fact: a rewound or rewritten accepted branch, a
  non-consecutive history, a goal missing at the floor, a History row that
  does not join its commit's trailer.
- Tests through the real observer (a test in metasystem/internal/goal
  or in metasystem/cmd/metasystem using `ObserveLedgerForWait` on a real
  clone and origin, as the existing ledger wait tests do): save a real
  exit 0 through the waiter and replay it through `ResumeWait` after a
  few later accepted changes (answers 0); the same with the walk made to
  run out of budget (answers 65, not 4); a cancel during that walk
  (answers 130); a rewound or rewritten act commit (answers 4). Replace
  or correct the two round 7 tests the reader named so each can fail.
- Keep every existing wait test green.

The reader also noted, as not material, that replaying a goal that was
lawfully pruned after its act now answers 4 where the original-cursor
floor answered 0. Do not change that deliberately; say in whatWasDone
what a pruned goal's replay answers after your change.

# Constraints

As in round 1's brief. Wall clock: 60 minutes; a partial round returns
with its tests green for what exists and names what is left.

# Expected Return

As in round 1's brief: evidence (1) `git -C metasystem status --short`,
(2) `( cd metasystem && go test ./internal/run/ -run 'TestWait' -count=1 )`,
`( cd metasystem && go test ./internal/goal/ -run 'Wait' -count=1 )` and
`( cd metasystem && go test ./cmd/metasystem/ -run 'Wait|Channel' -count=1 )`
with their pass lines, (3) `( cd metasystem && scripts/agents/go-gate.sh --fast )`
with its last line. whatWasDone names each observer return you retyped
and each new test with the case it proves.

# Gap Rule

stop and report a gap; never fill it silently.
