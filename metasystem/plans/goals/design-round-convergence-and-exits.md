# design-round-convergence-and-exits

- State: approved
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="The design loop decides what is built for every goal (exposure 3); a wrong exit builds an unconverged design, recoverable (severity 3); the rule is proven by hand on five designs (novelty 2); several units (accumulation 2)"
- Tier: 3
- Intent: Design critique rounds converge and exit by a recorded rule (accept, split, fold as acceptance items) in the machinery, split off review-drops-and-design-convergence under the size cap
- Origin: main
- Next step: Design to convergence from plans/design-round-convergence-and-exits-design-brief.md (small: units at most 250 production lines, goal at most 5 units), then build by hand with delegates
- OpenedAt: 2026-10-07T16:15:47Z
- FirstClaimAt: 2026-10-14T18:19:27Z
- Revision: 5
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-07T16:15:55Z revision=2 opid=2VQZ72MCZTP3MHD8AE49CY99XR-m1e-718ba0eb authority=proven digest=50610ba890f1ea856e76b51a671442689860877604afa9452dbc44e39cc2a02b episode=2
- Sliced: machine=m1e lineage=main-1790454088-93948-21671b revision=3 at=2026-10-14T18:28:27Z

History:
- 2026-10-07T16:15:47Z 13GVPBC0PJ4ZH63T2V13J8CZ9A-m1e-718ba0eb open actor=human:Wido targets=design-round-convergence-and-exits
- 2026-10-07T16:15:55Z 2VQZ72MCZTP3MHD8AE49CY99XR-m1e-718ba0eb approve actor=human:Wido targets=design-round-convergence-and-exits
- 2026-10-14T18:19:27Z 3YRHX7KQ7DHSTKH5VSHQB7T492-m1e-f456f182 claim actor=m1e+main-1790454088-93948-21671b targets=design-round-convergence-and-exits reason=areas unknown for goal lane-first-run-fixes; areas unknown for goal lane-recovers-and-pauses
- 2026-10-14T18:28:27Z FRVYTDY7KQA6GK7KT4S2CQMJTJ-m1e-f456f182 slice-start actor=m1e+main-1790454088-93948-21671b targets=design-round-convergence-and-exits
- 2026-10-14T19:00:46Z S420KS86TD231RWQHT5M5ZFXPN-m1e-cd3db6d0 release actor=human:Wido targets=design-round-convergence-and-exits displaced=m1e+main-1790454088-93948-21671b@2026-10-14T18:19:27Z reason=Whole-change read of the replacement 77cd00e1 (code-critic-3655f054f60e0779f47077cf): the tree is red on current main (beds without proof.full), four acceptance items half built, and the goal was built as sixteen units against its five-unit authority without a split proposal; the candidate stays on the branch with its findings retained; Wido decides between accepting the size with its impact or a re-split (decision record 2026-10-14 21:00, Astra-critiqued).
Integrity: sha256=44dcdcd920183fa81d53940f9bf6458793f0ed732b4ed04ce5ead388ca7f7b4c
