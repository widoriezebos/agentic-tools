# goal-labels

- State: claimed
- Intent: Backlog items carry zero or more labels so they can be grouped and listed by label (Wido 2026-08-24)
- Origin: human
- Next step: Appetite: 3h. Add a Labels field (zero or more, short kebab-case tokens) to the goal record: goal open/edit accept --label repeatably (or --labels a,b), goal list/next gain a --label filter, and labels survive reconcile/publish round-trips. Ledger schema change rides the existing goal-file format versioning; fixtures cover open-with-labels, edit-add-remove, filter-by-label, and an unlabeled item staying valid (zero labels is the default, never an error). First use: label today's custody/stewardship family (suite-custody, suite-dispatch-exclusion, return-recollection-on-process-lost, suite-outcomes-as-steward-incidents, incident-proposal-drafting, continuous-self-improvement) so the group lists as one.
- OpenedAt: 2026-08-24T13:28:22Z
- Revision: 3
- Labels: custody
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-26T04:29:22Z

History:
- 2026-08-24T13:28:22Z G787139BM2DZ5R5XM8HTAA79XK-m2-bc1be9cb open actor=human:wido targets=goal-labels
- 2026-08-26T04:29:22Z HSE9AX77RAG7W46Z05A520QJY2-m2-bc1be9cb claim actor=m2+mac-coordinator targets=goal-labels
- 2026-08-26T05:41:08Z D2QF3CJ9F107KFY7GW8A0N01YJ-m2-bc1be9cb edit actor=m2+mac-coordinator targets=goal-labels
Integrity: sha256=353fd690143457b98d05de322f40ae0008f25dd1035e271d50aa7fa4777eea7e
