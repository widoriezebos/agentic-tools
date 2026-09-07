# hook-demands-what-landing-forbids

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: no data is at risk, but a seat is ordered to do what it cannot land, and the correction is lost at the next checkout; novelty 2: the seam is between two enforcers that each work alone; exposure 3: every seat meets the hook and the gate; accumulation 2: the same stale plans are re-reported every turn on every seat until someone who owns them happens to look"
- Tier: 3
- Intent: The stop hook and the landing gate give a seat contradictory orders about plans it does not own. The hook refuses the stop with "Work named in a plan is unblocked and nothing is in flight. Do it now, or record in the plan why it is blocked or waiting on the human." On m1d 2026-09-07 the four plans it named were all stale rather than open - work that had finished, moved to another page, or belonged to another seat's claim - so recording the truth was the right exit and it was written. The agent commit then refused: register-carriage answered "the staged record is not owned by this landing (record-not-owned); carry only new records or records owned by the held goal or actor", and every other class needs a chain or a tier-1 job this seat has no reason to run. So the edit lives only in the working tree: the hook is satisfied because it reads the tree, but the correction cannot land, and the next checkout loses it. DONE means the two agree - a seat that the hook orders to record a plan's true state can land exactly that recording, by whatever narrow class fits (a plan-truth carriage that admits stale-claim corrections to plans it does not own would do), or the hook stops ordering what the gate forbids and names the owner to route it to instead.
- Origin: main
- Next step: Reproduce on m1d: the five plan edits are uncommitted in the working tree right now (handoff-m1-2026-09-02, host-implementer-wall-design, human-goal-verbs-forgiving-design, metasystem-stop-design, metasystem-stop-verb-design) with the exact refusal above. Decide which side gives - a narrow landing class for plan-truth corrections, or a hook that routes to the owning seat instead of ordering the edit - and make the two agree.
- OpenedAt: 2026-09-07T07:23:28Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-07T07:23:28Z 8EPXV9RJ6H7W477GKJA9Z3NPHJ-m1d-25755dc0 open actor=m1d+main-1788764558-63534-a15b0d targets=hook-demands-what-landing-forbids
Integrity: sha256=cc4185632aa674a4895f889639e8e0c03d841c255954bebffff99a8a36e33c93
