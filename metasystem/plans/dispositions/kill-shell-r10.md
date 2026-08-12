# Dispositions: kill-shell plan, round 10

Job: design-critic-20260812t003205z-17fd (codex gpt-5.6-sol, xhigh).
5 findings, 5 material (three critical), all accepted — three by
SEVERANCE after naming the generating cause.

## The generating-cause severance

Rounds 8-10's criticals all orbit one question: how do adopted
targets, which are forbidden to rebuild, receive a working engine —
across platforms (an ARM64 Mac binary is dead in an Ubuntu runner),
across Git causality (a binary cannot embed the HEAD that commits
it), and across the gitignore reality (nothing tracks the copied
binary today). Every mechanical patch this loop produced broke on
the next round because the question is not mechanical: it is an
ADOPTION-CONTRACT decision — plausibly Go source in the payload, or
per-platform release artifacts, or a rebuild allowance — that
belongs to the human under the reserved-decisions rule (contracts
consumed outside this repository) and interacts directly with
go-production-grade's Linux phase.

SEVERED: engine delivery to adopted targets becomes a named open
human decision recorded in this plan, with the options enumerated.
Kill-shell Phase E scopes to the TEMPLATE's bootstrap; freshness,
publication, and every binary rule are template-only until ruled.

| id | disposition |
| --- | --- |
| KS-R10-001 | accepted — absorbed into the severance: the plan stops claiming adoption commits the binary (it does not; the copy lands ignored), and the delivery mechanism awaits the human's ruling. |
| KS-R10-002 | accepted — absorbed into the severance: one committed binary cannot serve multi-platform hosts and CI; the options table records this as the constraint that kills the naive answer. |
| KS-R10-003 | accepted — absorbed into the severance for adopted targets; in the template the freshness rule re-states itself causally: the stamp must match the SOURCE tree state (HEAD of tracked source at build time), and the binary is never committed in the template, so no self-referential commit exists. |
| KS-R10-004 | accepted — template fix, ordered protocol: the publisher REGISTERS its own gate-run marker FIRST, then consults the fence (which exempts its own chain), then claims the publication lock, then renames. Register-then-check makes admission and replacement one protocol; two racing first-builds both register, both see each other, and the fence adjudicates. |
| KS-R10-005 | accepted — registry script entries carry a source→install path PAIR; the optional-skill relocation (optional-skills/... to skills/...) is representable, and projection follows the pair, not path identity. |
