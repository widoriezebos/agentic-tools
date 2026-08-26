# suite-outcomes-as-steward-incidents

- State: queued
- Intent: A red or vanished validation run is an incident the steward notices and acts on, not a log line only the operator's ad-hoc watcher sees (Wido 2026-08-24: an agent should have noticed and acted — none did)
- Origin: human
- Next step: Appetite: 2h. Suites register a run record with the steward at start and stamp the outcome at exit; the steward's tick treats a red outcome or a dead unstamped run as an incident: revive/notify per its existing verdict machinery. Acceptance: a battery killed mid-run or exiting red raises a steward incident visible in the stop message without any hand-armed watcher.
- OpenedAt: 2026-08-24T13:24:28Z
- Revision: 2
- Labels: custody

History:
- 2026-08-24T13:24:28Z PXXTRB93CM8QY3NT8XBRXRYNY9-m2-bc1be9cb open actor=human:wido targets=suite-outcomes-as-steward-incidents
- 2026-08-26T05:40:26Z 2NVQ1RT427KX1VC5FK3ZD6V7AD-m2-bc1be9cb edit actor=m2+mac-coordinator targets=suite-outcomes-as-steward-incidents
Integrity: sha256=c7944a86be5419943b5d9917ef8cf3363c0e878e7f92efe0040c3c34a3ff1c72
