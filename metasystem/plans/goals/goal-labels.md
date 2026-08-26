# goal-labels

- State: claimed
- Intent: Backlog items carry zero or more labels so they can be grouped and listed by label (Wido 2026-08-24)
- Origin: human
- Next step: Appetite: 3h. Add a Labels field (zero or more, short kebab-case tokens) to the goal record: goal open/edit accept --label repeatably (or --labels a,b), goal list/next gain a --label filter, and labels survive reconcile/publish round-trips. Ledger schema change rides the existing goal-file format versioning; fixtures cover open-with-labels, edit-add-remove, filter-by-label, and an unlabeled item staying valid (zero labels is the default, never an error). First use: label today's custody/stewardship family (suite-custody, suite-dispatch-exclusion, return-recollection-on-process-lost, suite-outcomes-as-steward-incidents, incident-proposal-drafting, continuous-self-improvement) so the group lists as one.
- OpenedAt: 2026-08-24T13:28:22Z
- Revision: 2
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-26T04:29:22Z

History:
- 2026-08-24T13:28:22Z G787139BM2DZ5R5XM8HTAA79XK-m2-bc1be9cb open actor=human:wido targets=goal-labels
- 2026-08-26T04:29:22Z HSE9AX77RAG7W46Z05A520QJY2-m2-bc1be9cb claim actor=m2+mac-coordinator targets=goal-labels
Integrity: sha256=d99897b2743752fcdee7c59e54115739663565382ba61e89abaec58bc2235c86
