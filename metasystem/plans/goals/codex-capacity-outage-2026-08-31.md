# codex-capacity-outage-2026-08-31

- State: queued
- Intent: Provider incident, recorded per the cost/availability-anomaly conduct: four codex (gpt-5.6-sol) runs died mid-run on 'Selected model is at capacity' on 2026-08-31 - counselor-b1 (~08:0x, after real edits), set-obligation-tempword (first attempt), counselor-b1-r2 (~09:0x, second death on the same slice), set-obligation-tempword-b-r2 (the small fix round, ~09:2x). Between deaths, two short codex runs completed fine (language-sweep-rerun-r2, set-obligation-tempword-b), so capacity is intermittent and longer runs die disproportionately. Each death consumes a dispatch and, on goal-bound work, a recorded attempt. Claude lanes are unaffected.
- Origin: main
- Next step: Bookkeeping + pattern watch. m2 conduct during the outage: one codex job at a time, small slices first (the counselor b1 slice was split in half for this reason), 20-30min retry cadence, no Sol-lane substitution (the takeover word authorizes substitution for Fable lanes only). Close when codex holds steady for a working session; if the intermittency persists into 2026-09-01, this becomes a budget question for Wido: capacity deaths burn attempts that budgets were sized without.
- OpenedAt: 2026-08-31T08:01:48Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-08-31T08:01:48Z WC80WEH42WVZG41D4R0QRK3038-m2-bc1be9cb open actor=m2+mac-coordinator targets=codex-capacity-outage-2026-08-31
- 2026-09-01T20:26:28Z FCF62KV2DWHQWPP3J07MDH3PM4-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=codex-capacity-outage-2026-08-31
Integrity: sha256=3e29146d55603fe91a384ab12ed4c45d26606704ed51a7c6f27edf08a63513e1
