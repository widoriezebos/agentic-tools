# commit-goal-binding

- State: queued
- Intent: The backlog law (R-38-m2) is unenforced at the commit boundary: nothing in the landing gates or commit machinery links a commit to a backlog item - the ledger and git history are two unconnected records, so anything can land without a goal and only conduct notices (Wido 2026-09-01: 'anything that gets committed better have a backlog item associated'). Applies to the metasystem and to any app it builds - one law, one gate.
- Origin: main
- Next step: Appetite: two 4h slices, full ladder per R-38-m2, queued immediately behind the current fix wave. Slice 1 (refusal, Go): every commit carries a Goal-Item: <id> trailer; the commit gate (the engine check commit.sh already calls) verifies against the ledger that the goal exists, is claimed by this machine and actor lineage, and is not concluded - typed refusal naming what is missing; goal-verb commits and fixture-authorized roots are the enumerated exemptions (the goal machinery's own commits ARE ledger records). Slice 2 (audit, steward): a tick health role sweeps every new canonical-branch commit since its cursor for a valid goal binding at landing time - a violation is an escalation episode naming commit and author machine, so hand commits and any future door-bypass get caught after the fact too. Tests: unbound commit refused; bound commit passes; concluded-goal binding refused; the steward flags a synthetic violation; the exemption list is closed and tested.
- OpenedAt: 2026-09-01T07:28:33Z
- Revision: 1

History:
- 2026-09-01T07:28:33Z V8J32XY2RA4GFPJN8NTWMVK2NW-m2-bc1be9cb open actor=m2+mac-coordinator targets=commit-goal-binding
Integrity: sha256=329f9697742ef9bebc553f39713fb13e189dc393c6a3f500c2c9e4c82c16de33
