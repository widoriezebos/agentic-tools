# follow-up-rebase-drop-failure-names-hash

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: no work is lost, recovery is one stash drop by tag; novelty 1: message text; exposure 1: the rare drop failure on the success path; accumulation 1: one leftover entry per occurrence"
- Tier: 1
- Intent: In the follow-up rebase's success path, when the tagged stash cannot be dropped after a completed re-apply, the refusal names only the tag and not the stash hash, and the leftover entry makes any later rebase of the same round refuse with 'the shared stash already contains the reserved tag' (closing critic of chain followup-rebase-build1, FRC-01, non-material). DONE means the drop-failure refusal names hash and tag like the restore-failure refusal does, and a fixture pins it
- Origin: main
- Next step: in metasystem/scripts/agents/dispatch.sh find the post-apply stash drop in the follow-up rebase path; mirror the restore-failure message; one fixture scenario
- OpenedAt: 2026-09-06T21:45:23Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-06T21:45:23Z PKJZ6KNPYK33SHZE5K03H45KFP-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=follow-up-rebase-drop-failure-names-hash
Integrity: sha256=0dd92c119be39c383be31a04e0fecc7d720cdad502b0a72a2b88743ca3a931ac
