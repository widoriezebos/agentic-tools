# adopt-bed-gate-unreachable-under-load

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="severity 2: a fixture bed that cannot run on a working machine leaves shell changes to adoption unproven, as happened today; novelty 1: the witness gate and its fallbacks exist, this scopes or reuses them; exposure 2: every seat that changes adopt.sh or its bed, and every validation that nests the bed; accumulation 2: three red runs in one evening on unrelated bounds, and each landing that skips the bed compounds the unproven surface"
- Tier: 2
- Intent: scripts/adopt-fixtures.sh arms a frozen witness gate before any of its legs, and that gate is the FULL race+coverage Go suite (scripts/agents/witness-gate.sh -> go-gate.sh, fifteen to twenty minutes). On a shared fleet machine at load 4-7 the suite fails on unrelated five- and ten-second wall-clock bounds (channel/fake TestTelegramTokenRoutes twice on 2026-09-06, missionrunner TestTerminateGroup and TestTerminateGroupLeaksNoGroupsUnderCompression, all green in isolation, all in memory/flake-registry.md), and the bed exits before its adoption legs run: three attempts on m1 on 2026-09-06 (goal rearm-remedies-and-adoption-notes) never reached a single leg, so the three new adoption legs had to be executed by hand against a committed snapshot. The bed's own legs test a shell script and the adopted engine; they do not need the whole race suite to pass first. DONE means the adopt bed can run its legs on a busy machine: either the witness gate it arms is scoped to what adoption exercises (the engine build plus the packages the bed's nested validations actually gate), or the bed accepts an inherited witness made once and reused, or the timing-bound tests stop being part of the gate a fixture bed pays; and a run on a loaded m1 reaches and passes the adoption legs.
- Origin: main
- Next step: Design first (tier 3 box): read scripts/agents/witness-gate.sh and the bed's gate arming at adopt-fixtures.sh lines 74-85 (WITNESS_GATE_FALLBACK=none); decide between a scoped witness, an inherited witness, or a gate without wall-clock-bound tests; then brief, build, critique, land; proof is one bed run on m1 at load 4 or more reaching its adoption legs.
- OpenedAt: 2026-09-06T18:57:06Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T18:57:06Z G8V4BC09A1ACC4RR1ZF7JB2CAG-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=adopt-bed-gate-unreachable-under-load
Integrity: sha256=25c6d7d3d793faae9b5396735874f3ce6906b1d8d717ae4853b897a6ca24667b
