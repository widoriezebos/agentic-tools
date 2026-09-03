# Tiering machinery, part one: code review (chain str-build1c-cc1)

Reviewed tree d9402254d107333abf6840068281cf68a02b9ad5 (chain str-build1c, round 1). Critic: Claude Fable 5.1. Brief: plans/severity-tiered-rigor-build1-code-critique-brief.md. Two material findings, six notes; one correction round follows.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F-1 | accepted | Four fixture scripts still open goals without --tier; validate goes red. Material. | Correction round (plans/severity-tiered-rigor-build1-fix-brief.md). |
| F-2 | accepted | The TierLaw marker is never installed when no tierless goal remains; the post-marker refusal never takes effect there and the message is false. Material. | Correction round: confirm installs the marker as its own root change. |
| F-3 | noted | The mission-cap obligation stays a documented skip, as the gap brief allowed; residual for part two, which owns the mission fence. | none |
| F-4 | noted | The obligation is proven across the package; one focused test for the follow-up inheritance refusal is added in the correction. | Correction round adds the test. |
| F-5 | noted | dispatch.cap-max is readable from the environment; a design-owner ruling, recorded for revision 4 or the close of this goal. | none |
| F-6 | noted | Confirm takes a bare human name per the gap-2 contract; recorded for the design owner. | none |
| F-7 | noted | Four-member budgets parse the round member as a fixed 3 rather than the tier-3 box; folded in the correction as a small fix. | Correction round. |
| F-8 | noted | Preview after an interrupted confirm refuses already-tiered draft rows; folded in the correction (skip them) so the recovery sentence holds. | Correction round. |
