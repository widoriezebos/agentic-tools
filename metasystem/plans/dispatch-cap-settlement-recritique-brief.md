Working Mode: design
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-02

# Goal

Round 2 of your critique of metasystem/plans/dispatch-cap-settlement-design.md,
now revision 2 (landed, in your workspace). Round 1 closed on five
accepted findings — DCS-R1-TIMESTAMP-AUTHORITY, DCS-R1-START-PROOF,
DCS-R1-REFUSAL-COVERAGE, DCS-R1-DISCHARGED-HUSK,
DCS-R1-GOVERNED-COMPONENT-PROOF — with the orchestrator's evidence in
metasystem/plans/dispatch-cap-settlement-dispositions.md. Revision 2
folds them: `endedAt` becomes transition-owned; observed minutes run
from the runtime process's own start; every refusal prints the reserved
line; the discharge filter tolerates a husk; the governed components
are proved.

# Inputs: decisions already taken, so you do not re-raise them

- DCS-R1-START-PROOF's mechanism half is REFUTED by the orchestrator: no
  new durable field atomic with the start gate. The exposure (a
  dispatcher dying between spawn and ownership write charges a
  seconds-long process 0) is recorded as known issue KI-45 in
  metasystem/memory/known-issues.md with its bound. Attack the chosen
  start instant (`pidStartedAt`) on its merits; do not re-open the
  atomic-field requirement.

- The rendering question the fold raised is ANSWERED: per-limit breach
  texts stay as today and one reserved segment is appended to every
  refusal line; do not ask for the decorated form back.

# Review brief

Round budget: this is round 2 of three on this chain; failsafe round 3.
Threat model, scope and materiality criterion unchanged from round 1
(metasystem/plans/dispatch-cap-settlement-critique-brief.md). A
finding that grows the mechanism beyond the charge rule, the
settlement shape, the message and the tests must name the requirement
the small design fails.

Verify first that each round-1 finding is actually folded, by reading
the revised sections, and say so per finding id. Then attack revision 2
where it is new: the RecordCAS refusal of `endedAt` (does any lawful
writer break; is the terminal stamp still exactly once); the
`pidStartedAt` arithmetic (epoch seconds against an RFC3339 `endedAt`;
clock domains; a runtime whose pid is reused); the always-printed
reserved line's exact bytes and its consumers; the husk path after a
discharge; whether T8's governed subtest can be satisfied by a wrong
implementation.

Return format: the design-critic schema; stable identifiers
DCS-R2-<name>; for each material finding say whether it is
mechanical-grain or invariant-grade; a clean verdict is
`verdictMaterialCount: 0` with any non-material observations recorded.

# Constraints

Wall-clock budget: 20 minutes. Do not rewrite the design.

# Gap Rule

stop and report a gap; never fill it silently.
