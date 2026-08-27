# timing-tests-synthetic-clock

- State: queued
- Intent: Timing-logic tests run on an injected synthetic clock and finish in microseconds; only legs that spawn real processes keep the real clock — the race gate's 25min is mostly wall-clock waits on arithmetic (Wido 2026-08-27 evening: why does timing-sensitive testing take so long, should we use a synthetic clock)
- Origin: main
- Next step: Appetite: 2h audit slice first, then conversion slices sized by its findings. Audit: classify every leg in internal/missionrunner (23min of the 25min race gate), internal/steward (105s), internal/mission (43s) as REAL-PROCESS (spawns/needs live pids and kernel time — keeps real clock) or TIMING-LOGIC (patience windows, grace periods, backoff, breakers — converts to the injected-clock idiom gaterun and the goal engine already use). Deliverable: the classified table with per-leg current cost and the projected gate duration after conversion. Conversion slices then follow the normal loop. Fourth leg of the proof-pricing overhaul (staged-batch witness, severity-tiered rigor, small-change lane): this shrinks the unit price of every proof; the witness stops multiplying it. Labels: shared (m1 owns battery process; the gate is shared ground).
- OpenedAt: 2026-08-27T17:12:26Z
- Revision: 1
- Labels: shared

History:
- 2026-08-27T17:12:26Z GRZ4RPVHPK0D6H2SKE8P1X46EV-m2-bc1be9cb open actor=human:wido targets=timing-tests-synthetic-clock
Integrity: sha256=2abc06a1d6048a00307b5cfd7412d043597c256a07e573eea2120ce15d7dcee7
