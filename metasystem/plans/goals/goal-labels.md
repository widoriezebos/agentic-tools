# goal-labels

- State: queued
- Intent: Backlog items carry zero or more labels so they can be grouped and listed by label (Wido 2026-08-24)
- Origin: human
- Next step: Appetite: 3h. Add a Labels field (zero or more, short kebab-case tokens) to the goal record: goal open/edit accept --label repeatably (or --labels a,b), goal list/next gain a --label filter, and labels survive reconcile/publish round-trips. Ledger schema change rides the existing goal-file format versioning; fixtures cover open-with-labels, edit-add-remove, filter-by-label, and an unlabeled item staying valid (zero labels is the default, never an error). First use: label today's custody/stewardship family (suite-custody, suite-dispatch-exclusion, return-recollection-on-process-lost, suite-outcomes-as-steward-incidents, incident-proposal-drafting, continuous-self-improvement) so the group lists as one.
- OpenedAt: 2026-08-24T13:28:22Z
- Revision: 1

History:
- 2026-08-24T13:28:22Z G787139BM2DZ5R5XM8HTAA79XK-m2-bc1be9cb open actor=human:wido targets=goal-labels
Integrity: sha256=0b0ecc16efccd71b9beed57a55535b54cf8f7283a9a588d92da2420c586151fb
