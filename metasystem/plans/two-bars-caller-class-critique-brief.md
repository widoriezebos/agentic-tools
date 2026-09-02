Working Mode: design
Orchestrator Identity: m1b+main-1788333346-60696-6a3256 (dispatch delegate under goal two-bars-for-changes)
Date: 2026-09-02

# Goal

Independent critique of metasystem/plans/two-bars-caller-class-design.md
(landed, in your worktree), the design for the caller-class slice of goal
two-bars-for-changes: the commit wrapper branches on the caller's CLASS
so that a worker-classified session can no longer commit on the human
branch. The hole it closes is section C1 of
metasystem/records/misc/design-gate-audit-2026-09-02.md.

# Inputs

The design was authored by implementer job
implementer-178d269e0852ac7a8e897657 (Claude Fable 5.1, design mode,
round 1) against base tree dd45f392. The round reported four gaps; the
orchestrator's dispositions, so you do not re-raise them:

- Coverage-floor home: the brief pointed at the wrong section; the
  design cites the ratchet files and ruling R-18 instead. ACCEPTED.
- The "+human" trailer count: the brief said fifteen (that was the
  floor-refused landing count); the design measured fifty on the day's
  history. ACCEPTED, the measured figure stands.
- Refusal sentence wording: the goal record's sentence binds, as the
  design pinned it. CONFIRMED.
- Implementer-private seams (design section 9): stated as assumptions
  with stop conditions; the implementer gap-stops if one fails. NOTED.

The design's own reject condition (section 10) is a legitimate attack
line: a lawful DELEGATE commit path through the wrapper, or an inner-
half re-check that disagrees with the outer-half classification for a
real launch shape.

# Review brief

Round budget: three focused rounds, agreed before round one; exhaustion
follows the design-critique skill's budget rules, never a silent fourth
round. Failsafe round: 3.

Threat model: agents that are lawful but unannounced or mis-classified —
an agent session whose steward is not armed, a delegate worker with a
shell in a worktree, a stale clone — reaching the commit wrapper's human
branch by accident or by habit. Hostile humans and forged process
ancestry are OUT of scope; the classification machinery in
metasystem/internal/lease/classify.go is taken as given.

Scope: the branch rule and where it lives (wrapper versus engine); the
caller table for `lease run-held` and `lease require-holder` and whether
any lawful delegate call breaks; the human path staying ungated; the
Machine trailer's lineage derivation; the fixture and unit-test list;
the declared non-goals. OUT: the promotion record, the never-direct-fix
floor, the register-carriage allowlist (other legs of the same goal).

Materiality criterion, verbatim: would an implementer working from this
design build something DIFFERENT, or WRONG, because of this finding?
Material findings change a contract, control flow, an outcome mapping,
what a test asserts, a named owner, or reveal a false premise, or leave
the implementer a genuine choice. Prose contradictions with an
unambiguous implementation, counts, naming, and restatements are not
material: record them, do not action them. The verdict line counts only
material findings.

Attack in particular: (1) does the design's engine surface really
distinguish commit authority from run-held authority without breaking a
lawful delegate caller — demand the caller table's evidence; (2) can a
MAIN whose lease epoch moved still slip through; (3) is the human path
provably unchanged; (4) is any fixture case unexpressible in
metasystem/scripts/agents/static-reproof-fixtures.sh as it stands;
(5) does the design ground every claim in file-and-line evidence from
the worktree, or does it guess at an implementer-private seam.

Return format: the design-critic schema; numbered findings with a stable
identifier each, most severe first, each with file, rule, and the
concrete failure it causes; or a clean verdict with observations that do
not gate.

# Constraints

Wall-clock budget: 25 minutes. Read the design and the cited lines; do
not rewrite the design.

# Gap Rule

stop and report a gap; never fill it silently.
