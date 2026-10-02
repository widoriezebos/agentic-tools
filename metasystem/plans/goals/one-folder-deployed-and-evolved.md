# one-folder-deployed-and-evolved

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="Moves where every adopted project's state lives and what an upgrade replaces; a wrong boundary loses project state on upgrade (severity 3); a new layout rule and a new upgrade boundary (novelty 2); every adopter and this repository (exposure 3); every agent turn reads these paths (accumulation 2)"
- Tier: 3
- Intent: An adopted repository has one MetaSystem folder, metasystem/, that separates what MetaSystem deploys (replaced only by an upgrade) from what the application evolves as it is built with MetaSystem (plans, designs, decisions, records, testing contract, committed config), so adoption works and newcomers see one folder (Wido 2026-10-02). Spec: plans/designs/one-folder-deployed-and-evolved.md; tier 3; Astra critique under the stop rule before any build.
- Origin: main
- Next step: Astra critique of the design (tier 3: threat model and rabbit-hole risks named up front), then build step 1
- OpenedAt: 2026-10-02T14:27:17Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-02T14:27:17Z 835YY2SYJ5EB97ZYCSK1FZJDB5-m1e-9c612d71 open actor=human:Wido targets=one-folder-deployed-and-evolved
Integrity: sha256=7beb6ad0b0a66040424f1504a5fe0daae885eabd490acbe203cfec3088976c9c
