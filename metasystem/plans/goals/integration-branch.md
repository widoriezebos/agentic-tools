# integration-branch

- State: queued
- Intent: The branch all work integrates into is designated, not assumed: default is the repo's default branch, but a development branch can be named instead — so protected-branch rules (PR-only merges to master) never block the metasystem's own landing flow (Wido's request 2026-08-24, low priority)
- Origin: human
- Next step: Appetite: 4h scoping slice, LOW PRIORITY (queued behind the app-guardrail program): inventory every place the metasystem assumes the default branch (landing pushes, mission branch targeting, goal-verb publication, provisioning/adoption baselines), then design ONE designation — likely a metasystem.conf key defaulting to the git default branch — consumed everywhere from that inventory. The two-part law applies: the designation is app/repo-side configuration a metasystem update never overwrites. Implementation slice tokened after the scoping lands.
- OpenedAt: 2026-08-23T19:32:07Z
- Revision: 1

History:
- 2026-08-23T19:32:07Z DXYC8H8AP7QBP0ZVBAZ294W0ZJ-m1-bf243850 open actor=human:wido targets=integration-branch
Integrity: sha256=f8455accdc56bc685cc990ba5d37b7058b2e0f07f11389fdb617a13b6434fe95
