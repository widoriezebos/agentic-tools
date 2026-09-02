# fleet-join-bootstrap

- State: queued
- Intent: A fresh session cannot boot itself into the fleet unaided (Wido's open question 2026-09-02; three machines hit the same wall: m1b's fresh host clone, and the m0/m0b guest clones before hand-fixing). A fresh clone has no engine (bin/ is gitignored and nothing tells the newcomer to run scripts/agents/go-build.sh), no roster (metasystem.conf.local is gitignored, so every role.*.model.* key, the machine nickname and the evidence root are absent), no accepted ledger tree until its first goal fetch, and no enrollment; the projection's staleness banner names a flag that does not exist ('goal list --fetch validates and advances it', internal/goal/project.go line 73) and the first-fetch message ('no accepted tree; the first fetch or the migration bootstraps it', line 39) names no command. CORRECTED by the design (plans/fleet-join-bootstrap-design.md, 2026-09-02): the earlier claim that a +refs/metasystem/* fetch refspec is needed was wrong; the canonical ledger is the main branch fetched per operation and the accepted pointer is created locally by the first goal fetch (internal/goal/txn.go). The refs/metasystem/machines/m0/* refs visible on origin are leftovers of the 2026-08-31 reconciliation, not part of the join path. An existing seat has its own wall: after a pull that changed the engine, metasystem up refuses with ENROLLMENT_DRIFT and its remedy names only the agent-free terminal, not the R-37-m3 re-arm path every seat actually uses. Nothing in AGENTS.md, wow.md, docs/collaboration.md or docs/working-with-agents.md documents joining. DONE means: one documented, mechanically checked join path (the design's scripts/agents/join-fleet.sh composing the existing Go owners), a committed roster template, every refusal on the path naming the real next command, and a fixture that joins a fresh clone of the template against a bare remote and reaches a green metasystem up or the honest documented stop where a human word is required.
- Origin: main
- Next step: Small-item ladder per R-38-m2 with R-44/R-45 budgets. Slice 1 (design, 4h): the join sequence as one owner (an engine verb or a script, decided by the design against the existing-owner-before-new-surface rule), the committed roster template, the ledger refspec (add +refs/metasystem/*:refs/metasystem/* at clone or at first goal verb), and every wrong or missing remedy text listed with file and line. Slice 2 (build + code critique, 4h): implement, fix the two refusal messages, land the fixture. Related: repair-accept-remote-verb (same advertise-a-missing-verb class), supervision-hook-wrong-root (m0's SessionStart hook cannot cd to <root>/metasystem on a checkout whose project dir already is metasystem), m1b's design-critique audit on the join question.
- OpenedAt: 2026-09-02T11:38:06Z
- Revision: 8
- Labels: bootstrap, fleet
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1
- Sliced: machine=m1 lineage=main-1788333680-2840-7f79f4 revision=5 at=2026-09-02T16:41:37Z

History:
- 2026-09-02T11:38:06Z Z4PYJ5FANGBERA70E3NB535N3D-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=fleet-join-bootstrap
- 2026-09-02T11:38:17Z SRNAYVH07JE7R15CR5H5GYXXQQ-m1-7bb1546e set-budget actor=m1+main-1788333680-2840-7f79f4 targets=fleet-join-bootstrap
- 2026-09-02T15:50:07Z EM7PG7Y65AATKYADBYXCSNJF9R-m1-7bb1546e claim actor=m1+main-1788333680-2840-7f79f4 targets=fleet-join-bootstrap
- 2026-09-02T15:53:59Z N0ZAM7P8E6H6T9T7CV9P321M91-m1-7bb1546e release actor=m1+main-1788333680-2840-7f79f4 targets=fleet-join-bootstrap
- 2026-09-02T16:41:14Z V57QSXTV01RD0HDSQXX9A1XKDN-m1-7bb1546e claim actor=m1+main-1788333680-2840-7f79f4 targets=fleet-join-bootstrap
- 2026-09-02T16:41:37Z J4N80W2DXJAXX8TG1BAPPGXK66-m1-7bb1546e slice-start actor=m1+main-1788333680-2840-7f79f4 targets=fleet-join-bootstrap
- 2026-09-02T16:58:58Z 363VWZ9R5QAAXTJW72MHJS54F0-m1-7bb1546e edit actor=m1+main-1788333680-2840-7f79f4 targets=fleet-join-bootstrap
- 2026-09-02T17:00:16Z 4REAHW81PSTESM0FP3ST6YW4YS-m1-7bb1546e release actor=m1+main-1788333680-2840-7f79f4 targets=fleet-join-bootstrap
Integrity: sha256=f0d543a9e5463b155ec6d2dc070ede02e5c9d307d425c6202f645bd82f2eb3b9
