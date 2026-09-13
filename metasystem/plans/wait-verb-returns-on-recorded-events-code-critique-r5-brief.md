Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal wait-verb-returns-on-recorded-events)
Date: 2026-09-13

# Review brief: the closing read of the landed member, over round 8

FINDING IDS: chain-unique across this goal's critic chains; WVB-40 to
WVB-55 are taken. WVB-55 is returned under its OWN identifier with
`material: false` if resolved, `material: true` if it still stands; a
new defect gets WVB-56 onward. Never report a resolution as prose inside
another finding.

Why this review exists: the member landed at commit 57ba0586 from the
build chain wait-member1-build-1, whose terminal round is round 8. The
previous rostered critic chain reviewed round 6 and found WVB-55:
replaying a saved result re-read the ledger from the wait's original
cursor, so `metasystem wait --resume WAIT-ID` answered exit 4 ("the
saved result no longer matches its source evidence") after a human act
many accepted changes after the cursor. Round 7 made replay check the
saved evidence from the row's last checked tip, which for a published
goal match is the matched commit (`replaySavedWait` in
metasystem/internal/run/waiter.go). Round 8 made the goal ledger observer
(`ObserveLedgerForWait` in metasystem/internal/goal/attention.go) report
its own deadline, exhausted Git budget or cancellation during the walk as
a temporary error, so every wait path answers 65 or 130 there and only a
ledger fact answers 4, and added TestWaitGoalSavedResultReplayThroughAcceptedLedger
in metasystem/internal/goal/attention_test.go, which replays a real saved
exit 0 through the real observer on a two-clone ledger. This chain is the
DESIGN-BEARING read of the terminal round that lets the build chain close.

Round budget: 1 focused round. A finding is material only if an
implementer must change the code or tests, and it names the artifact it
would change.

Contract: the design page
metasystem/plans/coordinator-wakes-on-events-not-polls-design.md
(section 2, and section 6's stated limit: registration and resume project
every accepted change since their cursor floor, and hours-old cursors on
a busy ledger return 65).

# Mandate

1. WVB-55: is it resolved as the contract states it, on the landed code?
   Return it under its identifier.
2. The reach of rounds 7 and 8: does any wait path (replay, registration,
   renewal, a running wait) still answer 4 for a deadline, an exhausted
   Git budget or a cancellation inside the ledger walk; does any real
   ledger fact (a rewound or rewritten accepted branch, a non-consecutive
   history, a goal missing at the floor, a trailer that does not join)
   now answer 65 or loop; can a saved result replay after its source
   evidence is gone?
3. Nothing else.

# Expected Return

The code-critic return schema, findings sorted by materiality, each with
file:line evidence and its rigor row. Every path in your return is
relative to the repository root, so it starts with `metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
