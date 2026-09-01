# commit-goal-binding

- State: queued
- Intent: The backlog law (R-38-m2) is unenforced at the commit boundary: nothing in the landing gates or commit machinery links a commit to a backlog item - the ledger and git history are two unconnected records, so anything can land without a goal and only conduct notices (Wido 2026-09-01: 'anything that gets committed better have a backlog item associated'). Applies to the metasystem and to any app it builds - one law, one gate.
- Origin: main
- Next step: Appetite: two 4h slices, full ladder per R-38-m2, queued immediately behind the current fix wave. Slice 1 (refusal, Go): every commit carries a Goal-Item: <id> trailer; the commit gate (the engine check commit.sh already calls) verifies against the ledger that the goal exists, is claimed by this machine and actor lineage, and is not concluded - typed refusal naming what is missing; goal-verb commits and fixture-authorized roots are the enumerated exemptions (the goal machinery's own commits ARE ledger records). Slice 2 (audit, steward): a tick health role sweeps every new canonical-branch commit since its cursor for a valid goal binding at landing time - a violation is an escalation episode naming commit and author machine, so hand commits and any future door-bypass get caught after the fact too. Tests: unbound commit refused; bound commit passes; concluded-goal binding refused; the steward flags a synthetic violation; the exemption list is closed and tested. WAIVER CLAUSE (Wido's word R-39-m2, design this in slice 1): the ONLY lawful unbound commit is one carrying an explicit human approval - designed as a recorded waiver in the strict-approval/temporary-word family: the human's verbatim word, bound to the exact commit digest it approves, single-use, loudly announced at commit time, and enumerable from the durable record so the terminal re-ratification pass can review every waiver ever used. The gate verifies the waiver deterministically; an unbound commit without one refuses naming R-38/R-39. Seat conduct is binding NOW ahead of the machinery: no seat-authored code change without a backlog item or his explicit word.
- OpenedAt: 2026-09-01T07:28:33Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-09-01T07:28:33Z V8J32XY2RA4GFPJN8NTWMVK2NW-m2-bc1be9cb open actor=m2+mac-coordinator targets=commit-goal-binding
- 2026-09-01T07:30:10Z 88ER02ZEY29YD8H2GGPD106VTA-m2-bc1be9cb edit actor=m2+mac-coordinator targets=commit-goal-binding
- 2026-09-01T20:28:13Z EJGKGKK6TS78JHYJ3ZWQD5BBYB-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=commit-goal-binding
Integrity: sha256=3ea6ec5829fb9139dfa8aa61dd350afb20811baa4c0e242e11d19c8f56cb329f
