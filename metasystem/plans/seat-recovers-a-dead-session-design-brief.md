# Design brief: seat-recovers-a-dead-session (from the first lane run 2026-10-09, findings 6-10)

Wido's rules: a person is never refused except to prevent damage; every remedy succeeds when followed; idempotent verbs; smallest thing that works.

Evidence: agentic-tools-evidence/lane-test-20261009/findings.md items 6-10.

What happened: a goal built in a session that has since died cannot be moved on by anyone: `work rebase G` refuses "held by another session" yet creates a rebase run that then owns the worktree; that completed run makes every later `work rebase`/`work wait run:...` answer "wait with work wait run:..." (a remedy pointing at itself); `goal claim --take-over` refuses an approved goal ("steal reassigns a standing claim") with no command in its remedy; verbs refuse in the goal worktree because enrollment is per installation; verbs built in the goal worktree refuse ("this engine is the goal worktree's own build").

Units (each at most 250 production lines):
1. A completed run releases its worktree, and `work wait` on a completed run returns at once with the run's outcome (never names itself as the remedy).
2. A refused verb creates no run (the claim check precedes the run record).
3. A person can move a goal whose session is dead to the current session with one recorded act (the existing claim owner; the remedy printed by "held by another session" is that act).
4. Remedies name the checkout they must run in (the seat checkout with the serving engine, enrolled), so a person is never sent to a worktree that cannot run them.

Acceptance: the first-run sequence (findings 6-10) replayed on a fixture goal ends with the goal handed in, each remedy succeeding when followed.
