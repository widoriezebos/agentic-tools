# suite-flake-supervision-watch

- State: queued
- Intent: The supervision fixtures' run-watch leg dies with Terminated:15 inside nested validates under load, failing unrelated legs for the wrong reason (seen 2026-08-25 00:46 in the counselor round-3 battery; rerun on identical bytes green)
- Origin: main
- Next step: Appetite: 2h triage. Evidence preserved at artifacts/agents/suite-failures/20260824T234637Z-adopt-66071 (orphan.out shows supervision-fixtures.sh reporting the S4-16 'run watch' child terminated before the pruned-skill assertion). Triage: is the watch's SIGTERM a cleanup race under nested-validate load (the fixtures-leak-and-compound family) or a real liveness defect; fix the race or bound the fixture; a flake that fails OTHER legs' assertions for the wrong reason is the worst kind of red. Related: steward-owned-execution consumes the broader suite-custody question.
- OpenedAt: 2026-08-25T01:59:55Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-08-25T01:59:55Z S03C4H64CXYJE8HS2ZM9Y5129M-m1-bf243850 open actor=m1+coordinator targets=suite-flake-supervision-watch
- 2026-09-01T20:27:30Z F08P7KEEDGPQYTDAFNBJHG1A45-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=suite-flake-supervision-watch
Integrity: sha256=4bfbd4f6abdac063ce6cbb7c3e1487d1885a79ca12dd9962099e001f3fa02073
