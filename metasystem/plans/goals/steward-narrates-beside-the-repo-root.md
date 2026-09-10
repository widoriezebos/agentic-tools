# steward-narrates-beside-the-repo-root

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a stray untracked file and a broken stash restore, nothing granted or destroyed; novelty 1: one root resolution in the steward tick's digest write; exposure 2: every template-layout seat on every landing; accumulation 1: one file rewritten in place"
- Tier: 2
- Intent: The steward writes its landing highlight (A landing moved the repository storyline to commit X) into records/narrator-digest.log under the repository root, beside the installation, instead of the installation's own records/narrator-digest.log. Seen on m1 2026-09-10 at 08:17:41Z and again at 08:41Z right after landing a633b7a08, with the seat armed as up --repo <repository root>. The stray untracked file then makes every git stash restore that carries it fail (could not restore untracked files from stash), which broke two scripted landings' restores tonight. Done means: on the template layout every digest write of the steward tick lands in the installation's records directory, a test covers the repository-root arming, and no file appears under <repository root>/records.
- Origin: main
- Next step: Find the digest write in the steward tick that resolves its root from the --repo flag instead of the installation root (internal/steward/tick.go around the NarrateDigest call), fix the resolution, add the template-layout test, then a short chain: Sol builds, Opus critiques.
- OpenedAt: 2026-09-10T08:43:34Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-10T08:43:34Z Z6F607MMCBXMPPD22WJ813C7EK-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=steward-narrates-beside-the-repo-root
Integrity: sha256=97177801e5d6d6c1e16e6e451b9a39596dd4ddfe211ffbbaeb9e1d8e66304448
