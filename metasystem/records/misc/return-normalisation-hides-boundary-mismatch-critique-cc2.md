# The validator normalisation fix: code review, second round (chain rnb-build1-cc2)

Reviewed tree 48973e9446b3afefdd3d2c9f24e895661f31e655 (chain rnb-build1, round 2). Critic: Claude Fable 5.1. One material finding; a third round follows.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| RNB-01 | resolved | The leg tests the mismatch before any review record exists; the exact refusal text appears. | none |
| RNB-02 | resolved | The unit test mirrors the fixture's order on a fresh round. | none |
| RNB-21 | accepted | The untracked-plan, committed-plan, uncommitted-plan and control-plane-change legs re-run review after the round's first success and will meet the immutability refusal instead of their own. | Round 3 moves those legs before the first successful review, or onto fresh rounds, keeping their refusal texts. |
| RNB-22 | resolved | The validator package ran green seat-side on the round 2 tree (57.9 seconds). | none |
