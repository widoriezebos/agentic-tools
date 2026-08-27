# proof-run-cost-and-liveness

- State: queued
- Intent: A proof run can never again be silent, unbounded, and quadratically priced: pre-landing validation proves the STAGED BATCH once and nested runs reuse it, every suite announces its expected cost up front, emits progress heartbeats, and dies loudly at a silence failsafe instead of stalling dark (Wido 2026-08-27 evening: very serious bug, must never be allowed to happen again — 2h dirty-tree adopt run, 50min operator silence)
- Origin: main
- Next step: Appetite: 1d, design-first (Wido's direct order counts as the draft discussion). ROOT CAUSE (verified): witness-gate.sh arms only on CLEAN gate-input roots, so pre-landing (always dirty) nested validations each re-pay the full race gate (~25min x N nested = ~2h adopt); AND suite runs carry no progress contract — the patience-attempts-not-wallclock law governs actor loops but was never applied to proof runs, so stall and progress are indistinguishable. Slice 1 (design, 2h): the STAGED-BATCH WITNESS — arm against the digest of the staged/uncommitted batch bytes (content digest of gate-input roots as they ARE, not requiring cleanliness), so one outer gate proves the batch and every nested run byte-checks and skips; must not weaken the witness's honesty (digest covers exactly what the gate proved). Slice 2 (3h): the PROGRESS CONTRACT — every validate/adopt/battery run prints an upfront cost banner (witness armed: ~15min / unarmed: ~90min, legs enumerated), writes a progress heartbeat (leg name + timestamp) to a well-known file, and a silence failsafe (no heartbeat movement for a configured window, suite.progress-silence-min) kills the run loudly with evidence preserved — never a dark stall. Slice 3 (1h): the runner surfaces — dispatch watch and background runners read the heartbeat and report progress/ETA; the coordinator's watch output shows leg-by-leg movement. Related: validation-harvest-mode (shared label, m1 coordination), m1's witness mechanism and section selector (build on, do not fork). Labels: shared.
- OpenedAt: 2026-08-27T17:07:53Z
- Revision: 1
- Labels: shared

History:
- 2026-08-27T17:07:53Z 15PY3WX8E8D98882HN5W21B1ND-m2-bc1be9cb open actor=human:wido targets=proof-run-cost-and-liveness
Integrity: sha256=4e924d3cce8247f0bb8d49600b39cb3326b39014a58f37c0f474cb6fbb41c205
