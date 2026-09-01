# breach-stop-wedges-seat

- State: queued
- Intent: A breach-stopped claim wedges the whole machine: release is refused (only resume clears the fence, and resume is a human act), while the one-claim quota rejects every new claim as long as the stopped claim stands - so one budget breach freezes the seat until a human types resume, violating the standing order that a parked stream never prevents claiming the next item. DONE means a breach-stopped goal parks without holding the quota slot, or release becomes lawful on a stopped claim, with the fence on RESUMING that goal preserved
- Origin: main
- Next step: Appetite: 1h. Discovered live 2026-09-01 morning (records/misc/idle-loss-2026-09-01.md, the wedge specimen is in the ledger refusals at tips 4d8bff0e/88599665). CONSTRAINT: the budget law stays intact - only the quota interaction changes; a human word must still gate resuming the breached goal itself. Prove with a fixture: breach-stop a claim, then claim another goal successfully
- OpenedAt: 2026-09-01T06:49:32Z
- Revision: 1

History:
- 2026-09-01T06:49:32Z FP70QX8HBN6WY1V8PX52K60QHN-m3-a5da21ff open actor=m3+mac-m3 targets=breach-stop-wedges-seat
Integrity: sha256=2be6dc246539879ca3abcfa5b68206fdbfdcaadef27290b689722367487cf089
