# os-dependency-reduction

- State: queued
- Intent: Remove or reduce OS/tool dependencies to the declared minimum: no perl, no python3, and a pinned inventory that refuses new external binaries undeclared (Wido 2026-08-24: remove or at least reduce as much as possible)
- Origin: human
- Next step: Appetite: 4h across three slices. Slice one (15min): drop the two perl one-liners in adopt-fixtures.sh for sed/engine equivalents. Slice two (3h): port the benchmark kit's python3 — extractor.py and schema helpers — into the Go engine as benchmark verbs (the engine already validates JSON schemas for returns); the kit shell stays orchestration-thin and python3 leaves the dependency list. Slice three (45min): a dependency ratchet in the audit — the external-binary inventory (git, go, runtime CLIs) is declared data, and a new command -v/exec of an undeclared binary refuses at the gate, so dependencies never regrow silently. Out of scope by ruling: git and the runtime CLIs stay.
- OpenedAt: 2026-08-24T15:41:03Z
- Revision: 1

History:
- 2026-08-24T15:41:03Z BEZ13078T604YC32GYRHD6SKQ2-m2-bc1be9cb open actor=human:wido targets=os-dependency-reduction
Integrity: sha256=5102473b7d49803c3f1721fd502b57999f112aad13f13dfbf3ea83e1fb8bab63
