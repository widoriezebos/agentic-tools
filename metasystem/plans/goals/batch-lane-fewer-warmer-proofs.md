# batch-lane-fewer-warmer-proofs

- State: queued
- Risk: severity=3 novelty=2 exposure=2 accumulation=2 basis="Changes how every landing is proven and who is ejected; a wrong rule could land a red on main or eject a good unit; shared by all seats."
- Tier: 3
- Intent: What: The landing lane proves each batch once, with warm build caches, and blames a piece of work only when the evidence says so. Why: Most of this already landed (flake handling, early proof reuse, shared caches). Three things are left: small per-member proofs still run instead of reusing the batch proof; some quick static checks are missing when work joins a batch (their absence let reds reach main on 09-27); and builders start without looking at how busy the computer is. Pros: Faster landings, fewer false blames, fewer reds on main. Cons: It changes the same lane code that is being redesigned right now, so starting before the redesign lands would collide with it.
- Origin: human
- Next step: Next: Wait until landing-lane-runtime-redesign has landed and the lane is switched back on. Then re-read the lane code and design only the three leftovers (per-member proofs reuse the batch proof, the missing static checks run when work joins, builder launches wait for spare CPU), with one Astra critique round. Done when: the design page is accepted and names a test for each leftover.
- OpenedAt: 2026-09-27T18:19:20Z
- Revision: 4
- Labels: efficiency, landing
- BudgetExceptions: 0

History:
- 2026-09-27T18:19:20Z 0M2QF8S8FM3FTNN8CAMPD493XR-m1e-c6925449 open actor=human:Wido targets=batch-lane-fewer-warmer-proofs
- 2026-09-27T18:20:26Z K72PV9EQVEB11663MWTTDJ9KQ2-m1e-c6925449 approve actor=human:Wido targets=batch-lane-fewer-warmer-proofs,pluggable-proof-runner
- 2026-09-30T19:04:28Z CMH317EQ748JHAYHQZ9J5JTDTZ-m1e-b6a4eb0a unapprove actor=human:wido targets=batch-lane-fewer-warmer-proofs reason=Held: it would change the landing lane while the lane is being redesigned; revisit under landing-lane-runtime-redesign (backlog sync 2026-09-30)
- 2026-09-30T19:04:41Z BVPAEN0PY3JZBYSHPCPQHWJCSY-m1e-b6a4eb0a edit actor=human:wido targets=batch-lane-fewer-warmer-proofs
Integrity: sha256=2750bf95259dcace8f6e6f27b332850a37c387d9a33d888a9998c129657ea447
