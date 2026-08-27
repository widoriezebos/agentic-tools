# timing-tests-synthetic-clock

- State: queued
- Intent: Timing-logic tests run on an injected synthetic clock and finish in microseconds; only legs that spawn real processes keep the real clock — the race gate's 25min is mostly wall-clock waits on arithmetic (Wido 2026-08-27 evening: why does timing-sensitive testing take so long, should we use a synthetic clock)
- Origin: main
- Next step: Appetite: 2h audit slice first, then conversion slices sized by its findings. CENSUS (2026-08-27, coordinator): internal/missionrunner has NO clock abstraction — 289 tests, ~4.8s average, only 6 explicit sleeps (all <=300ms), 39 test sites spawning REAL subprocesses; cost is real process lifecycles under real grace/timeout knobs plus race overhead, NOT naive sleeps. Audit therefore classifies each leg THREE ways: TIMING-LOGIC (convert to injected clock — microseconds), REAL-PROCESS-TUNABLE (shrink grace/poll knobs via the env-knob idiom the fingerprint harness already uses; share spawned fixtures), REAL-PROCESS-IRREDUCIBLE (keep). Also steward (105s) and mission (43s). Deliverable: classified table with measured per-leg cost and projected gate duration (working hypothesis: 25min -> 5-8min). Fourth leg of the proof-pricing overhaul. Labels: shared.
- OpenedAt: 2026-08-27T17:12:26Z
- Revision: 2
- Labels: shared

History:
- 2026-08-27T17:12:26Z GRZ4RPVHPK0D6H2SKE8P1X46EV-m2-bc1be9cb open actor=human:wido targets=timing-tests-synthetic-clock
- 2026-08-27T17:15:51Z 8TK863Y9F7XH960CTKX092C0AN-m2-bc1be9cb edit actor=m2+mac-coordinator targets=timing-tests-synthetic-clock
Integrity: sha256=0614ddb989c775096c74ceac90a38268894c9163a97a40dcd7c99c32cf1b515e
