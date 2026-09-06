# breach-stop-wedges-seat

- State: approved
- Intent: A breach-stopped claim wedges the whole machine: release is refused (only resume clears the fence, and resume is a human act), while the one-claim quota rejects every new claim as long as the stopped claim stands - so one budget breach freezes the seat until a human types resume, violating the standing order that a parked stream never prevents claiming the next item. DONE means a breach-stopped goal parks without holding the quota slot, or release becomes lawful on a stopped claim, with the fence on RESUMING that goal preserved
- Origin: main
- Next step: Appetite: 1h. Discovered live 2026-09-01 morning (records/misc/idle-loss-2026-09-01.md, the wedge specimen is in the ledger refusals at tips 4d8bff0e/88599665). CONSTRAINT: the budget law stays intact - only the quota interaction changes; a human word must still gate resuming the breached goal itself. Prove with a fixture: breach-stop a claim, then claim another goal successfully
- OpenedAt: 2026-09-01T06:49:32Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=a5613449ea270548e47fef27b8599ce8535d99b3cedbe90e7dc356ce2a6ecae1

History:
- 2026-09-01T06:49:32Z FP70QX8HBN6WY1V8PX52K60QHN-m3-a5da21ff open actor=m3+mac-m3 targets=breach-stop-wedges-seat
- 2026-09-01T20:26:21Z 7D9VYWG8J44X9MSVSGN4Q491G4-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=breach-stop-wedges-seat
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=breach-stop-wedges-seat reason=sweep
Integrity: sha256=b2a26fc59949e0546d2f7ff0454ebf4073790f5e707e99431c367bbd2517d236
