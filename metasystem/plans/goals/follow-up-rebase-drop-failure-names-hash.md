# follow-up-rebase-drop-failure-names-hash

- State: queued
- Priority: 3
- Sequence: 37
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: no work is lost, recovery is one stash drop by tag; novelty 1: message text; exposure 1: the rare drop failure on the success path; accumulation 1: one leftover entry per occurrence"
- Tier: 1
- Intent: In the follow-up rebase's success path, when the tagged stash cannot be dropped after a completed re-apply, the refusal names only the tag and not the stash hash, and the leftover entry makes any later rebase of the same round refuse with 'the shared stash already contains the reserved tag' (closing critic of chain followup-rebase-build1, FRC-01, non-material). DONE means the drop-failure refusal names hash and tag like the restore-failure refusal does, and a fixture pins it
- Origin: main
- Next step: in metasystem/scripts/agents/dispatch.sh find the post-apply stash drop in the follow-up rebase path; mirror the restore-failure message; one fixture scenario
- OpenedAt: 2026-09-06T21:45:23Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-06T21:45:23Z PKJZ6KNPYK33SHZE5K03H45KFP-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=follow-up-rebase-drop-failure-names-hash
- 2026-09-08T16:01:17Z 1AW3KVJVKRV1G3KNKQE59ZAJEP-m1-7cd0bd60 set-priority actor=human:Wido targets=follow-up-rebase-drop-failure-names-hash reason=priority-order subject=follow-up-rebase-drop-failure-names-hash from=unranked to=3:37 requested-sequence=37
Integrity: sha256=2b44c909a16c38adc106e2098682ea427d61efe56f6158bcb234706f9d5c0211
