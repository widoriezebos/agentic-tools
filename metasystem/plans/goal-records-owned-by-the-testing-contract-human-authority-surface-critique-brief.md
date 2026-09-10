Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal goal-records-owned-by-the-testing-contract)
Date: 2026-09-10

# Review brief: the human-authority and report packages get owning test surfaces (chain ha-surface1-20260910)

FINDING IDS: chain-unique, HAS-01, HAS-02, ... never F-n. Report `round`
as 1 in your return: it is this job's own round. One focused round.

Why: the landing receipt refused two reviewed chains for unowned
paths: metasystem/internal/humanauthority/** plus six command files
(goalsync_mutations, identity, session_stop and their tests), and
metasystem/internal/report/**. The implementer added two surfaces
(human-authority, report-scan) with unit groups shaped like
goal-decision-standard, per rule 6 of the contract design.

Scope: the computed diff of job ha-surface1-20260910
(metasystem/testing.json only). Contract:
metasystem/plans/application-testing-contract-design.md.

# Mandate

1. The two surfaces own exactly the listed paths (files that do not
   exist skipped), depend on dispatch-goal-mission, and select their
   new unit group plus the named existing groups; no other surface
   lost a path; the new groups run the two packages' tests.
2. The contract validates; a change under each new surface resolves
   as delivery with no unresolved impact (probe the real selection as
   the earlier critics did).
3. Nothing outside metasystem/testing.json changed.

If nothing material remains, say so; that closes the chain and it
lands.

# Constraints

Wall-clock budget: 10 minutes. Return per the code-critic schema with
the reviewedTree from the review record beside the computed diff (the
conformance validator refuses from a sandbox; say so).

# Gap Rule

Stop adding at a gap that needs a decision no page has made; keep what
is written; report the gap with the resolution you propose. Never
delete written work.
