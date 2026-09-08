# uncapped-delegate-fanout

- State: approved
- Priority: 3
- Sequence: 45
- Intent: A Bash-capable delegate can launch a fresh paid Claude session (including the CLI's background mode) that inherits no native budget flag and may escape the dispatching job's process group — an unowned spend and custody path found by the Sol critique of the spend-cap retirement design (records/misc/spend-cap-critique-r1.md finding SCR-R1-ENVELOPE-004, affirmed as an honest open record in r2). Candidate owners named by the design: the permission preset (deny nested claude/codex launches), census/kill-guard reach over escaped children, or an adapter-level nested-launch refusal. The exact behavior of claude --bg from inside a paid delegate is UNPROVED (executing it crosses a spend boundary; the critic twice declined, correctly)
- Origin: main
- Next step: Design round when claimed: pick the owner, prove the nested-launch behavior in a sandboxed/zero-spend way if possible, and wire the refusal; budget is Wido's word at claim (R-2 small-item lane, derived from a critiqued finding)
- OpenedAt: 2026-09-01T17:34:51Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=7f1f67369d232d9dd917917c6ebd8875b5dc390b5f715099dec4a8c54de6e4a4

History:
- 2026-09-01T17:34:51Z R3VGTAHK92HYJ34RJJZCFT5VEP-m0b-6638932d open actor=m0b+main-1788250419-3170380-8a1fb3 targets=uncapped-delegate-fanout
- 2026-09-01T20:27:44Z KRA33XB7PVSZ4T8FSB71JX4C78-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=uncapped-delegate-fanout
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=uncapped-delegate-fanout reason=sweep
- 2026-09-08T16:01:45Z 5H4FHJ2P2BS23DM42KHB657RKM-m1-7cd0bd60 set-priority actor=human:Wido targets=uncapped-delegate-fanout reason=priority-order subject=uncapped-delegate-fanout from=unranked to=3:45 requested-sequence=45
Integrity: sha256=01638b9dd05972f97575bdfc246675f4870319226263d734eca77dcee4279c60
