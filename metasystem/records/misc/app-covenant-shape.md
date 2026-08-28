# The app covenant: shape extraction (slice one design note)

Read off the benchmark kit's taskrun case (../benchmark/cases/taskrun/0.3/),
per the retrofit ruling: the shape is extracted from a working example,
never invented. What the kit already carries, generalized:

| Covenant section | Kit source | Generalization |
|---|---|---|
| identity | case.json id/title/product.entryPoint | app name, entry point, source paths |
| requirements | spec.md's 26 numbered requirements + requirements-map.json (each id proven by a named check that runs alone) | rows: id, statement ref, proof (executable check id) — the traceability the adequacy gate will read |
| battery | mission.gate (command, metric, threshold, direction) + grader checks | the one command that earns green, with its threshold |
| budgets | metrics with bound/floor (dependency_count bound 0, plan_seconds bound 20) | named budgets: metric, bound, direction — the ratchet's subjects |
| guards | mission.guard (cadence, floor, per-cycle) | standing floors checked every cycle, orthogonal to the battery |
| guardrails | mission.instruments + gate.paths (never-edit enforcement) | the net's declared paths — the app-side home the contract's wall.guardrails derives from |
| goldens | grader/checks.md battery | golden sets with provenance (slice-two surface; the kit's grader is held out by design) |

## Slice-one implementation plan

1. internal/covenant: the schema (covenant.json, schemaVersion 1) and a
   verified reader — sections above, exact-keyed, canonical paths via the
   guardrail grammar where paths appear. App-owned location: covenant.json
   at the adopted repo root (the two-part law: never overwritten by a
   metasystem update, at worst migrated; a re-adoption fixture proves it).
2. Staged minimality, as sharpened by review: only a repo with NO
   covenant.json passes untouched. An app CARRYING one refuses contract
   omission, and covenant.path may only name covenant.json itself — the
   covenant's one home at the app root; a movable covenant is a
   selectable one.
3. Agreement checks at preflight, the battery bound WHOLE: gate command,
   the threshold for the battery's metric (in the contract's own
   threshold grammar, shared so the two can never diverge), and the
   gate direction; the guardrail nets EQUAL as declared, both
   directions refused distinctly. Divergence refuses with both values
   named. The gate binds start AND resume.
4. Fixtures: reader validation (good/malformed/each section), the
   preflight refusal + agreement paths, the re-adoption byte-identity
   fixture for the app section.
5. The kit's own covenant.json is authored WITH machine 2's arc (its
   territory), exactly like the wall.guardrails contract line. The
   package fixture meanwhile carries a REPRESENTATIVE extraction from
   case.json 0.3: three requirement rows spanning the shapes (a
   decision point, a floor-guarded constraint, a budgeted metric) and
   the kit's exact instrument set as guardrails — the full 26-row
   extraction is the kit covenant's own content, landed with it.
6. The two-part law's adoption proof covers the half adoption owns
   today: initial adoption and a same-version re-run preserve the
   app-owned covenant byte for byte (compared with cmp). adopt.sh
   refuses cross-version reruns and routes them to the documented
   upgrade path; upgrade-time survival is that machinery's obligation,
   proven when it exists. The covenant gate binds at start AND resume,
   compares the battery whole (command, the threshold for its metric,
   the direction, and the threshold SET — an undeclared gate.threshold
   key refuses), demands net equality in both directions, and an app
   carrying covenant.json cannot be opted out by contract omission.
