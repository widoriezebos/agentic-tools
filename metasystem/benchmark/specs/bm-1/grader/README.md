# Held-out grader

Nothing here is ever copied into the repository the agents work in.
Provisioning copies `../spec.md` and `../seed/` only.

Not yet built. Version 0.1 builds exactly what `../manifest.json` records under
`grader.v01Scope`: the acceptance suite with per-requirement metric lines, the
behavioural batteries, the peripheral metrics, `seed_respected`, and the
calibration probes with their recorded scores.

There is no reference implementation in this benchmark, and no mutation testing
in v0.1: the earlier design for both was refuted under critique (BA-2-2) and a
later mutation version needs a design of its own before it is built.
