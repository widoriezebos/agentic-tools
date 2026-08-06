# Development docs

Design and analysis documents for building the metasystem. This folder sits
beside `metasystem/` on purpose: nothing here ships, and nothing shipped may
reference this folder. There is no third system and no "meta-meta" — the
metasystem plays the same role in this repository as in any adopted one; this
repository's product just happens to be the metasystem's own source, so its
plans live in `metasystem/plans/` like any project's plans, and its measuring
kit lives in `benchmark/` beside it.

## Contents

- `metasystem-inventory.md` — every path in the template classified (SHIPS /
  PROJECT-STATE / RUNTIME) with the deciding rule for each.
- `evidence-index.md` — one line per delegate chain and archived mission in
  the durable evidence store.
- `project-rules-local.md` — THIS repository's filled-in local rules (the
  shipped template keeps only anonymous, portable forms).
- `devin-selftest.md` — the runbook for the laptop that has Devin access.
- `metasystem-design.md`, `source-analysis.md` — the founding design and the
  keep-or-remove provenance of every rule.
- Finished reports: the cross-runtime comparison, the triaged watch-list, the
  superseded remediation plan.
