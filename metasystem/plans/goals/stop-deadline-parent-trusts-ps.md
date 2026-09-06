# stop-deadline-parent-trusts-ps

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: a hung Stop is allowed to hang the turn end instead of being refused, nothing unsafe is permitted; novelty 1: one liveness test swapped for the one bash already offers; exposure 3: every seat's every turn end, though only when ps misbehaves (sandboxes, restricted hosts); accumulation 1: it does not compound"
- Tier: 3
- Intent: The Stop hook's deadline parent decides whether its worker is still running by asking ps for the worker's state (deadline_running in scripts/agents/supervision-hook.sh); when ps prints nothing for a live process the parent concludes the worker has finished, leaves its deadline loop and waits for the worker with no deadline at all, so a genuine hang is never refused. Found 2026-09-06 by the code reviewer of goal stop-hook-budget-is-ours (finding SHB-02) after the builder's sandbox, which denies process inspection, showed exactly this: the delaying-engine deadline scenario ran 65 seconds and returned a non-blocking response instead of the expiry. Predates the budget change. DONE means the parent's liveness test does not depend on ps output: an empty or failing ps answer is treated as 'still running' for the purpose of the deadline (the wait keeps its deadline), the kill path still refuses to signal a process whose command line it cannot verify, and a fixture with a ps that prints nothing proves the parent still expires at its deadline.
- Origin: main
- Next step: BUILDING (m1d, 2026-09-06 23:10 CEST). Build brief landed 5aeb19c9 (plans/stop-deadline-parent-trusts-ps-build-brief.md); implementer sdp-build1-20260906 (Sol) running. Plan: validate conformance --stage review --job sdp-build1-20260906; seat replays bash -n and supervision-hook-fixtures.sh outside the sandbox on the reviewed tree; one Fable code critic (brief plans/stop-deadline-parent-trusts-ps-code-critique-brief.md, --reviews sdp-build1-20260906, fresh --op sdp-cc1); if zero material: register-advance, dispatch.sh close, dispositions record, git apply --index --directory=metasystem of rounds/1/diff.patch from the repo root, land.sh -m <msg> --chain sdp-build1-20260906 --direct-fix register-carriage --goal stop-deadline-parent-trusts-ps --staged-only --allow-new-plan --skip-transport (accumulation 1, no full-width receipt expected), go-build, metasystem up, goal done.
- OpenedAt: 2026-09-06T09:38:27Z
- Revision: 5
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T12:52:20Z revision=2 opid=Q0HMY137H80MMNZMS7PMA8FPSE-m1-7cd0bd60 authority=proven digest=a5fa9d9591c44f5e9e0c50ea59e56b49f37baf3001c22e5690302d6a1a60665d
- Sliced: machine=m1d lineage=main-1788683763-71870-f7f607 revision=3 at=2026-09-06T20:46:36Z
- Claimed: machine=m1d lineage=main-1788683763-71870-f7f607 at=2026-09-06T20:45:34Z revision=3 accountingRevision=3
- StopCapability: generation=3 revision=3 machine=m1d claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T09:38:27Z FDAPZ18VF4J2FX061WP8XCFQW8-m1d-62183579 open actor=m1d+main-1788683763-71870-f7f607 targets=stop-deadline-parent-trusts-ps
- 2026-09-06T12:52:20Z Q0HMY137H80MMNZMS7PMA8FPSE-m1-7cd0bd60 approve actor=human:Wido targets=human-goal-verbs-forgiving,repo-root-paths-ride-agent-commits-unjudged,stop-deadline-parent-trusts-ps,stop-hook-open-work-refusal-repeats,verbs-match-intent
- 2026-09-06T20:45:34Z GT72493D10HF6A9F51Y4EQD4B8-m1d-62183579 claim actor=m1d+main-1788683763-71870-f7f607 targets=stop-deadline-parent-trusts-ps
- 2026-09-06T20:46:36Z FZ931HS7JGKBPMD19S2SHQKB1Q-m1d-62183579 slice-start actor=m1d+main-1788683763-71870-f7f607 targets=stop-deadline-parent-trusts-ps
- 2026-09-06T20:47:29Z N8R3MW6EHTH4QZ4T5PP7ZNTTXP-m1d-62183579 edit actor=m1d+main-1788683763-71870-f7f607 targets=stop-deadline-parent-trusts-ps
Integrity: sha256=54a39657efd6df0848a1a40bf7dba5ae504ebce9125d0203134b4d290c032b28
