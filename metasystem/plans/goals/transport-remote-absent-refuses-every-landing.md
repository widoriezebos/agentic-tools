# transport-remote-absent-refuses-every-landing

- State: queued
- Priority: 2
- Sequence: 1
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: it misreports a landing that reached origin as a failed landing, and leaves the VM mirror the validation rule depends on unwritten; novelty 1: a remote configuration plus an honest refusal line, on machinery that already carries a skip flag; exposure 3: the step is required on every landing from this Mac; accumulation 2: every landing pays it and every report has to be read past it"
- Tier: 3
- Intent: What: A landing that reached GitHub is never reported as failed just because this computer has no "transport" mirror set up. Why: The landing lane already skips the mirror step when no mirror exists, but the hand-landing path (work land --message) still requires it and reports a failure for work that is safely on origin. Pros: Truthful landing reports and no false alarms on machines without a mirror. Cons: A machine that should have the mirror but lost it now gets only a one-line note, so the gap may go unnoticed longer.
- Origin: main
- Next step: Next: In internal/landpath/land.go, skip the mirror sync with one plain line when git remote get-url transport fails, as the batch lane already does; add a test where a landing with no transport remote succeeds and names the skip. Done when: work land --message on a checkout without a transport remote reports success and says the mirror step was skipped.
- OpenedAt: 2026-09-09T07:04:53Z
- Revision: 8
- BudgetExceptions: 0

History:
- 2026-09-09T07:04:53Z ZEPGVJX7EACT4X8WJV852K5PWV-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=transport-remote-absent-refuses-every-landing
- 2026-09-09T07:32:48Z SEGWH3575FR5TXSSHS4JFGHC46-m1b-c6925449 set-priority actor=human:Wido targets=account-provenance-carried,adoption-inventory-from-install-set,capped-round-continues-instead-of-restarting,chain-landing-carries-the-reviewed-diff,chain-landing-carries-unrelated-staged-plans,closing-read-follows-the-change-not-the-label,commit-goal-binding,critique-always,design-gate-at-dispatch,enrollment-proves-a-human-not-a-terminal,first-headless-run,fleet-channel-gateway,fleet-join-bootstrap,fleet-pull,gate-governance-records,headless-continuous-delivery-proof,headless-fleet-coordination-proof,hook-enrollment-per-checkout,host-implementer-wall,human-approval-for-execution,idle-every-runtime-enforcement,landing-design-provenance,manifest-floor-at-dispatch,merge-stage-critic-close,metasystem-stop-escalation-proofs,mission-birth-baseline-from-dirty-worktree,never-idle-ironclad,one-approval-gate,recovery-rehearsal,recovery-to-good-state,repo-root-paths-ride-agent-commits-unjudged,role-context-composition,role-lane-packets,role-liveness-watchdog,seat-mutual-awareness,transport-remote-absent-refuses-every-landing,two-bars-for-changes reason=priority-order subject=transport-remote-absent-refuses-every-landing from=unranked to=2:1 requested-sequence=1
- 2026-09-09T07:36:10Z 7FRV8AJERFT1A799NC48KQBVJE-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=transport-remote-absent-refuses-every-landing
- 2026-09-09T07:42:51Z 10RVVPG43QH5A45A000WAQ9VE1-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=transport-remote-absent-refuses-every-landing
- 2026-09-09T08:00:22Z BGDG5WEFTYGMM806189P619A1M-m1b-c6925449 approve actor=human:Wido targets=transport-remote-absent-refuses-every-landing
- 2026-09-11T08:04:19Z AB7K9V2D4MPDKGSXNJ244E2DS2-m1-c6925449 unapprove actor=human:Wido targets=transport-remote-absent-refuses-every-landing reason=Wido 2026-09-11: standing approval withdrawn to regain control of what is built next; re-approve deliberately
- 2026-09-11T21:58:41Z 79JJNEBYSPGBCDTAJDBMMNK743-m1-c6925449 edit actor=human:Wido targets=transport-remote-absent-refuses-every-landing
- 2026-09-30T18:47:03Z VM9AFBJD64KKFE8T6WZDWN3TFS-m1e-b6a4eb0a edit actor=human:wido targets=transport-remote-absent-refuses-every-landing
Integrity: sha256=94ec5e17d609b33ba9efd8812050be6793244218dfb83bd42a3458df21f56da4
