# goal-labels

- State: done
- Intent: Backlog items carry zero or more labels so they can be grouped and listed by label (Wido 2026-08-24)
- Origin: human
- Next step: Appetite: 3h. Add a Labels field (zero or more, short kebab-case tokens) to the goal record: goal open/edit accept --label repeatably (or --labels a,b), goal list/next gain a --label filter, and labels survive reconcile/publish round-trips. Ledger schema change rides the existing goal-file format versioning; fixtures cover open-with-labels, edit-add-remove, filter-by-label, and an unlabeled item staying valid (zero labels is the default, never an error). First use: label today's custody/stewardship family (suite-custody, suite-dispatch-exclusion, return-recollection-on-process-lost, suite-outcomes-as-steward-incidents, incident-proposal-drafting, continuous-self-improvement) so the group lists as one.
- Concluded: Concluded by Wido's standing delegation citing the landed-and-verified deliverable: labels landed c7ea747 (codex-built from the failsafe-converged design, certified 2/0, host proofs green — internal/goal, cmd, the full goal-CLI fixture suite) and the first use executed live: the eight-goal custody family labeled and listed as one group by goal list --label custody. The grammar is explicit (labelRe), carriage follows the Blocked precedent with verb-layer canonicalization, filters compose AND and never touch claim law, labels survive recovery and open --claim, and orphan label flags refuse with the verb named.
- OpenedAt: 2026-08-24T13:28:22Z
- Revision: 4
- Labels: custody

History:
- 2026-08-24T13:28:22Z G787139BM2DZ5R5XM8HTAA79XK-m2-bc1be9cb open actor=human:wido targets=goal-labels
- 2026-08-26T04:29:22Z HSE9AX77RAG7W46Z05A520QJY2-m2-bc1be9cb claim actor=m2+mac-coordinator targets=goal-labels
- 2026-08-26T05:41:08Z D2QF3CJ9F107KFY7GW8A0N01YJ-m2-bc1be9cb edit actor=m2+mac-coordinator targets=goal-labels
- 2026-08-26T05:41:59Z 88J0KF5CFYQ96ZW60RQYTED3VS-m2-bc1be9cb done actor=human:wido targets=goal-labels
Integrity: sha256=34752ef1dd327e694e633dc6dad82a69a2a9247eebe2dde65a40dfc58e2c3d86
