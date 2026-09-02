# fleet-join-bootstrap

- State: claimed
- Intent: A fresh session cannot boot itself into the fleet unaided (Wido's open question 2026-09-02; three machines hit the same wall: m1b's fresh host clone, and the m0/m0b guest clones before hand-fixing). A fresh clone has no engine (bin/ is gitignored, so bin/metasystem does not exist and nothing tells the newcomer to run scripts/agents/go-build.sh), no roster (metasystem.conf.local is gitignored, so every role.*.model.* key and the evidence root are absent), no ledger (the fetch refspec is +refs/heads/* only, while the ledger lives under refs/metasystem/* on origin - refs/metasystem/machines/<m>/accepted is published there and never fetched), and the projection's staleness banner names a flag that does not exist ('goal list --fetch validates and advances it', internal/goal/project.go:73; the first-fetch message 'no accepted tree; the first fetch or the migration bootstraps it' names no command at all). An existing seat has its own wall: after a pull that changed the engine, metasystem up refuses with ENROLLMENT_DRIFT and its remedy names only the agent-free terminal, not the R-37-m3 re-arm path every seat actually uses. Nothing in AGENTS.md, wow.md, docs/collaboration.md or docs/working-with-agents.md documents joining; docs/backlog-mechanism.md's fleet notes assume the ledger is already there. DONE means: one documented, mechanically checked join path - a newcomer runs a named sequence (build engine, lay down the local roster from a committed template, fetch the ledger refs, arm, up) and every refusal on that path names the real next command; proven by a fixture that joins a fresh clone of the template against a bare remote and reaches a green metasystem up.
- Origin: main
- Next step: Small-item ladder per R-38-m2 with R-44/R-45 budgets. Slice 1 (design, 4h): the join sequence as one owner (an engine verb or a script, decided by the design against the existing-owner-before-new-surface rule), the committed roster template, the ledger refspec (add +refs/metasystem/*:refs/metasystem/* at clone or at first goal verb), and every wrong or missing remedy text listed with file and line. Slice 2 (build + code critique, 4h): implement, fix the two refusal messages, land the fixture. Related: repair-accept-remote-verb (same advertise-a-missing-verb class), supervision-hook-wrong-root (m0's SessionStart hook cannot cd to <root>/metasystem on a checkout whose project dir already is metasystem), m1b's design-critique audit on the join question.
- OpenedAt: 2026-09-02T11:38:06Z
- Revision: 5
- Labels: bootstrap, fleet
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=240 activeJobLimit=1
- Claimed: machine=m1 lineage=main-1788333680-2840-7f79f4 at=2026-09-02T16:41:14Z revision=5
- StopCapability: generation=5 revision=5 machine=m1 claimEpoch=4 fenceEpoch=0

History:
- 2026-09-02T11:38:06Z Z4PYJ5FANGBERA70E3NB535N3D-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=fleet-join-bootstrap
- 2026-09-02T11:38:17Z SRNAYVH07JE7R15CR5H5GYXXQQ-m1-7bb1546e set-budget actor=m1+main-1788333680-2840-7f79f4 targets=fleet-join-bootstrap
- 2026-09-02T15:50:07Z EM7PG7Y65AATKYADBYXCSNJF9R-m1-7bb1546e claim actor=m1+main-1788333680-2840-7f79f4 targets=fleet-join-bootstrap
- 2026-09-02T15:53:59Z N0ZAM7P8E6H6T9T7CV9P321M91-m1-7bb1546e release actor=m1+main-1788333680-2840-7f79f4 targets=fleet-join-bootstrap
- 2026-09-02T16:41:14Z V57QSXTV01RD0HDSQXX9A1XKDN-m1-7bb1546e claim actor=m1+main-1788333680-2840-7f79f4 targets=fleet-join-bootstrap
Integrity: sha256=cd8b5c9a9b80a769cccd8f7f3f8fc86395d76f972ec94052ade3026dde1606df
