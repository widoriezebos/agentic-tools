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
2. Staged minimality: the runner refuses a mission ONLY when the contract
   declares a covenant (covenant.path key) and the file is missing,
   invalid, or disagrees with the contract's own gate/guardrails — never
   a universal refusal that would break covenant-less repos. Universal
   presence is inception-era policy, not slice one.
3. Derivation checks at preflight: contract gate command == covenant
   battery command; contract wall.guardrails ⊆ covenant guardrails.
   Divergence refuses with both values named.
4. Fixtures: reader validation (good/malformed/each section), the
   preflight refusal + agreement paths, the re-adoption byte-identity
   fixture for the app section.
5. The kit's own covenant.json is authored WITH machine 2's arc (its
   territory), exactly like the wall.guardrails contract line — the
   fixtures carry a faithful copy extracted from case.json 0.3 meanwhile.
