# gate-governance-records

- State: approved
- Priority: 2
- Sequence: 14
- Intent: The fix wave's three new refusal gates (dispatch design gate, commit-goal binding, change-class gate) are being built without the discipline every other enforced rule carries: no owner, review date, known-bad fixture, appeal route, or broken-check response declared, and no marking-mode trial before refusal power - the exact shape by which a guard against ceremony-free coding becomes unowned ceremony itself. DONE means: every gate from this fix wave ships with a governance record naming owner, review date, known-bad fixture, appeal route, and broken-check response, and receives refusal power only after a marking-mode trial with recorded false-alarm bounds.
- Origin: main
- Next step: INTENT: govern the new gates like every enforced rule. CONSTRAINTS: follow the existing rule-governance shape used for enforced rules (the seat-governance record and the R-row register pattern with class/due/event columns); the dated-records watch (internal/steward/ruling_sweep.go) must flag an overdue gate review - extend its parse only if the record rides memory/rulings.md rows, never a second watch. FREEDOMS: whether records ride one file or per-gate rows; trial length and false-alarm bounds are per-gate judgment recorded in each record. TEST SHAPE: each gate's known-bad fixture refuses; an overdue review date raises a flag.
- OpenedAt: 2026-09-01T13:20:51Z
- Revision: 5
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=0900df963de6fd1deb71b279a7a1765ed5515145c9657421937023b8f59cf648

History:
- 2026-09-01T13:20:51Z KXNWNMA2TJPZ62GB3H4B03YR3V-m0-c5dbf036 open actor=human:Wido targets=gate-governance-records
- 2026-09-01T20:26:48Z 3H9NRGJC08ZDNX3FCDK9Z3YDE6-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=gate-governance-records
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=gate-governance-records reason=sweep
- 2026-09-08T15:58:02Z VX308MPVV8RQ94PQXDM0JJYYR7-m1-7cd0bd60 set-priority actor=human:Wido targets=gate-governance-records reason=priority-order subject=gate-governance-records from=unranked to=2:15 requested-sequence=15
- 2026-09-08T16:24:03Z 72ERBGBZNWN6FH95PQQA5BTAQ4-m1b-c6925449 done actor=m1b+main-1788680071-18713-e76d5d targets=chain-landing-after-base-move-recertifies,first-headless-run,fleet-channel-gateway,fleet-join-bootstrap,fleet-pull,gate-governance-records,headless-continuous-delivery-proof,headless-fleet-coordination-proof,host-implementer-wall,idle-every-runtime-enforcement,metasystem-stop-escalation-proofs,mission-birth-baseline-from-dirty-worktree,never-idle-ironclad,recovery-rehearsal,recovery-to-good-state,repo-root-paths-ride-agent-commits-unjudged,role-context-composition,role-lane-packets,role-liveness-watchdog,seat-mutual-awareness,two-bars-for-changes reason=priority-order from=2:15 to=2:14
Integrity: sha256=cffab0be0d421b45cc4f3f886421e30f2548c9fd9e8b480f1454f164a14b9119
