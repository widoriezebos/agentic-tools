# Astra's scoped read of g1-s62 revision 1

Produced 2026-09-27 by Codex on `gpt-6-astra`, read-only, against revision 1 of [g1-s62-one-vocabulary.md](g1-s62-one-vocabulary.md) at `657621cd4`, a scoped read of the join of g1-s58's revision-5 grammar and g1-s59's word table against the object-and-action table, under R-121 and R-124. Verbatim; the disposition is in the design.

---

1. **S62-01 — Low, non-material — U1d also changes goal actions.**

   **Evidence (read):** the design says none of U1d's actions is a goal action, but metasystem/plans/designs/verbs-object-action.md:379 replaces `goal check` and `goal repair` with `goal sync`.

   **Concrete failure:** The deferred-scope statement is inaccurate. No current runtime failure follows: none of the nine admitted actions changes, and D1 already requires unsupported actions' usage text to come from the catalogue.

   **Change:** Describe U1d as changing none of the nine proposable actions, while acknowledging the refused goal actions' rename.

   **Test 1:** No—correcting this statement changes neither the nine mappings nor the catalogue lookup already required. **Test 2:** Passes **WORK and SAFE** without amendment; this is future scope clarification.

**Deferred and non-material**

- Confirmed design commit `657621cd4` on `ui-development`. Read the joined designs and critique records; their accepted folds remain intact.
- **D1:** All nine names and admitted fields match the descriptors. Advanced data flags remain admissible; no hidden or authority field is admitted. Ran the executable's help for all nine and confirmed usage lines for the fourteen named unsupported actions.
- **D2:** The join is writable in `cmd/metasystem` without another command inventory. Direct descriptor access preserves audience and hidden-field checks omitted from `commandCatalogue()`.
- **D3:** The requested source search found no occurrence outside the specified rename scope and exceptions. The explanatory sentence at `DecisionsPane.tsx:939` is covered by the general rename and absence check. Question withdrawal, "Open goal," vocabulary regeneration and "Paused" are accounted for.
- **Persistence:** Stored proposals retain route IDs and body fields; serialized proposal fixtures need no migration. Tool-input fixtures must adopt the public grammar, as the verification section requires.
- No files changed. Tests and browser walkthroughs were not run.

VERDICT: 0 material findings; build as written
