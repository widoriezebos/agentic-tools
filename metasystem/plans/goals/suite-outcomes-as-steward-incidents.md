# suite-outcomes-as-steward-incidents

- State: queued
- Intent: A red or vanished validation run is an incident the steward notices and acts on, not a log line only the operator's ad-hoc watcher sees (Wido 2026-08-24: an agent should have noticed and acted — none did)
- Origin: human
- Next step: Appetite: 2h. Suites register a run record with the steward at start and stamp the outcome at exit; the steward's tick treats a red outcome or a dead unstamped run as an incident: revive/notify per its existing verdict machinery. Acceptance: a battery killed mid-run or exiting red raises a steward incident visible in the stop message without any hand-armed watcher.
- OpenedAt: 2026-08-24T13:24:28Z
- Revision: 3
- Labels: custody
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-08-24T13:24:28Z PXXTRB93CM8QY3NT8XBRXRYNY9-m2-bc1be9cb open actor=human:wido targets=suite-outcomes-as-steward-incidents
- 2026-08-26T05:40:26Z 2NVQ1RT427KX1VC5FK3ZD6V7AD-m2-bc1be9cb edit actor=m2+mac-coordinator targets=suite-outcomes-as-steward-incidents
- 2026-09-01T20:27:33Z 96FC0JCNZAFDKHE545G3K8XKMT-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=suite-outcomes-as-steward-incidents
Integrity: sha256=ccc62b88f4703d40f9bace6b12e8f1f58c99763c7b1bc6bd41705d8741b32554
