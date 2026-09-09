# transport-remote-absent-refuses-every-landing

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: it misreports a landing that reached origin as a failed landing, and leaves the VM mirror the validation rule depends on unwritten; novelty 1: a remote configuration plus an honest refusal line, on machinery that already carries a skip flag; exposure 3: the step is required on every landing from this Mac; accumulation 2: every landing pays it and every report has to be read past it"
- Tier: 3
- Intent: scripts/agents/land.sh runs sync-transport.sh as a REQUIRED final step, which pushes origin's branch head to a remote named transport. No checkout on this Mac has that remote: neither agentic-tools-m1b nor agentic-tools-m1c lists it in git remote -v. So every landing from this machine pushes to origin, then fails its last step with git exit 128 and the truncated message 'and the repository exists.', and reports a failed landing for work that is actually on origin/main. Tonight that happened on 48d8bc39. The standing rule that the VM validates both transport and main means the mirror is wanted, not that the step should be dropped; land.sh also already carries --skip-transport, which is the wrong default to reach for because it hides the gap. DONE means either the transport remote is configured wherever the rule expects it and the step passes, or the step states plainly that transport is unconfigured and says which remedy applies, and either way a landing that reached origin is never reported as failed.
- Origin: main
- Next step: Reproduce with bash scripts/agents/sync-transport.sh main, which fails with git exit 128 because no remote named transport exists; confirm with git remote -v in both agentic-tools-m1b and agentic-tools-m1c. Then settle the question the rule leaves open, which is where the transport mirror lives now, because the standing rule that the VM validates transport and main assumes a remote that no checkout on this Mac has. That is Wido's answer to give, not a URL to invent. Once it is known, either configure it as part of checkout setup, which belongs with host-runtime-setup's territory, or make the step report the absence in one plain line naming the remedy. Either way land.sh must stop reporting a landing that already reached origin/main as a failure, and the fixture is a landing whose transport step cannot run.
- OpenedAt: 2026-09-09T07:04:53Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-09T07:04:53Z ZEPGVJX7EACT4X8WJV852K5PWV-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=transport-remote-absent-refuses-every-landing
Integrity: sha256=206fb9aa3128cf3d6f9301ddf8305c0ee6ad52c6f6939af8b39a0c9430675286
