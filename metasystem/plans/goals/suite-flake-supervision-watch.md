# suite-flake-supervision-watch

- State: approved
- Intent: The supervision fixtures' run-watch leg dies with Terminated:15 inside nested validates under load, failing unrelated legs for the wrong reason (seen 2026-08-25 00:46 in the counselor round-3 battery; rerun on identical bytes green)
- Origin: main
- Next step: Appetite: 2h triage. Evidence preserved at artifacts/agents/suite-failures/20260824T234637Z-adopt-66071 (orphan.out shows supervision-fixtures.sh reporting the S4-16 'run watch' child terminated before the pruned-skill assertion). Triage: is the watch's SIGTERM a cleanup race under nested-validate load (the fixtures-leak-and-compound family) or a real liveness defect; fix the race or bound the fixture; a flake that fails OTHER legs' assertions for the wrong reason is the worst kind of red. Related: steward-owned-execution consumes the broader suite-custody question.
- OpenedAt: 2026-08-25T01:59:55Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=525b22e8938860281c912fe1d9f7f4a85fcd7d0829a90f1fe69c6e711c0b97d2

History:
- 2026-08-25T01:59:55Z S03C4H64CXYJE8HS2ZM9Y5129M-m1-bf243850 open actor=m1+coordinator targets=suite-flake-supervision-watch
- 2026-09-01T20:27:30Z F08P7KEEDGPQYTDAFNBJHG1A45-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=suite-flake-supervision-watch
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=suite-flake-supervision-watch reason=sweep
Integrity: sha256=4e9ef56a154142e6c87a83c6800d26f0b983e2d00c467f5c85ebe84662b87bbf
