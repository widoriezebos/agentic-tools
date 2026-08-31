Working Mode: design
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal role-lane-packets)
Date: 2026-08-31

# Goal

A design document at plans/role-lane-packets-design.md that makes the
R-25-m1 role lanes mechanical: today the lanes (who authors, who
examines, which model family sits in which seat) are carried by hand in
every launch; the ruling names role-packets.json — the closed-packet
recipe file — as the enforcing surface. The design decides how lanes
enter the packet recipes and the engine-side expectations, and how a
launch on the wrong lane is refused.

# Workspace

The dispatch-created job worktree. Produce exactly one new file:
plans/role-lane-packets-design.md. Touch nothing else — this is a
design round, no code changes.

# Inputs

Read before designing:

- memory/rulings.md rows R-25-m1 (the lane map: DESIGN authored by
  claude+Fable as a fresh-context delegate, DESIGN CRITIQUE codex+Sol,
  IMPLEMENTATION codex+Sol, IMPLEMENTATION CRITIQUE claude+Fable),
  R-25b-m1 (carried designs must not be weakened), R-28-m1 (model tier
  and effort selection WITHIN the lanes is the dispatch delegate's
  recorded judgment until 2026-09-30 — the lane STRUCTURE is Wido's
  word; the design must enforce structure without freezing the
  delegated tier choice), R-23-m1 (cross-family independence is the
  point of the criss-cross).
- plans/goals/role-lane-packets.md — the goal text; note the
  exact-equality fingerprint law it cites: engine and file change
  together.
- scripts/agents/role-packets.json — the current recipe file: per-role
  source slots plus the destructiveReach table (effort tiers and
  critique obligations per hazard class; Ruling O effort floors live
  here and are law).
- internal/dispatch/hazard.go — ResolveHazardConfiguration and
  MinimumHazardConfiguration: the exact-equality refusal pattern the
  goal's fingerprint law refers to.
- metasystem.conf and metasystem.conf.local — today's role.* runtime
  and model keys, including the mode-scoped design mapping
  (mode.design.role.implementer.*) that routes design authorship to
  the Claude family. The design MUST decide the precedence and
  conflict story between conf keys and packet-encoded lanes: who wins,
  what refuses, and what migrates.
- internal/config/model.go CanonicalModel — model names are
  canonicalized for key lookup; lane encoding must survive that.

# Requirements the design must satisfy

1. Every rostered role carries its lane in role-packets.json: the
   runtime family that performs it and the examination relationships
   R-25-m1 fixes (Sol builds and Claude critiques; Fable designs and
   Sol critiques). The encoding must express FAMILY structure as law
   while leaving the model tier and effort selection within a family
   to the R-28-m1 delegation until its expiry.
2. Refusal at launch: a dispatch that puts a role on the wrong lane is
   refused at brief-assembly or admission time, with a typed reason,
   before any budget spends. Strictness guards the R-25 invariant only
   — benign variation (a new model within the lawful family) must not
   refuse.
3. The exact-equality fingerprint law: the engine's expectations and
   the packet file change together; a packet file the engine does not
   expect, or vice versa, refuses loudly. Follow the existing
   hazard-table pattern.
4. The conf-versus-packet precedence question is answered explicitly,
   including migration: what happens to today's role.* conf keys, and
   what a conflict between conf and packets does on day one.
5. Provability: the design names the tests that prove it — extend the
   hazard-configuration test family; a wrong-lane fixture refuses, a
   right-lane fixture launches, a tampered packet file refuses.
6. Cross-family independence stays derivable: the design must make it
   impossible to quietly configure the same family into both the
   builder and its examiner for the classes Ruling O gates.
7. Implementability as one slice: the design ends with a slice plan
   whose first slice is at most 4 hours (R-17), naming exactly what
   the implementer builds and what waits.

# Constraints

- Design only. No code, no packet-file edits, no conf edits.
- The design carries its own self-grade per R-24-m1: confidence, the
  weakest claim, and the condition under which it should be rejected.
- Wall-clock budget: 35 minutes.

# Expected Return

Version-2 implementer JSON. diffBoundary lists exactly
metasystem/plans/role-lane-packets-design.md. Evidence entries cite the
inputs actually read (level "read") — no test runs are expected from a
design round.

# Gap Rule

If a requirement above cannot be satisfied from the named inputs, stop
and report the gap in the return; never fill it silently and never
weaken a requirement.
