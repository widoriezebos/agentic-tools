Working Mode: design
Orchestrator Identity: m3+main-1788172645-85501-aa86ee (dispatch delegate under goal fable-model-alias)
Date: 2026-09-03

# Goal

Author a one-page design (under 100 lines) for goal fable-model-alias
(read metasystem/plans/goals/fable-model-alias.md first). Wido's order,
verbatim: "i want claude-fable-5 to be an alias for claude-fable-5.1 to
avoid running into DESSIGNM-BEARING", refined minutes later: "I want it
to be the alias for the latest class 5 model, is that possible?". So
claude-fable-5 is not a retired id to be mapped away: it is the FAMILY
POINTER, meaning "the latest Claude Fable 5.x model", whose target moves
by a one-line configuration landing on Wido's word each time a new 5.x
ships (today claude-fable-5-1). The dispatching seat's answer to "is
that possible" is yes, as a tracked pointer; automatic discovery of
"latest" from the API is NOT wanted (the lane's model must not change
without his word and a landing, R-46-m0b). The situation it answers: on
2026-09-03 a seat whose machine-local roster still named claude-fable-5
was refused every DESIGN-BEARING dispatch with
REFUSED-HAZARD-CONFIGURATION ("runtime claude has no executable
maximal-effort mapping for destructiveReach DESIGN-BEARING") because the
tracked line `runtime.claude.maximal-models=claude-fable-5-1` (landed by
Wido, d081ef07, ruling R-46-m0b) no longer lists the old id, and the
gate compares ids literally. The fleet has several clones per machine,
each with its own uncommitted metasystem.conf.local; some still say
claude-fable-5. Wido wants the family id to MEAN the latest one, everywhere,
instead of a refusal.

# Workspace

The delegate worktree the dispatcher created for this job. Read
anything; write exactly one NEW file, fable-model-alias-design.md, in
the metasystem plans directory.

# What the design must settle

1. WHERE THE ALIAS IS APPLIED. The roster resolves a role's model in
   metasystem/internal/dispatch/roster.go (ResolveRoster: the
   role.<role>.model.<runtime> and role.default.model.<runtime> keys,
   the --model override, RequestedPair and RosterPair). The maximal gate
   is runtimeProvesMaximalExecution in
   metasystem/internal/dispatch/hazard.go. The cap rows are
   cap.min.<role>.<runtime>.<model> in metasystem/internal/dispatch/cap.go.
   Job records carry requestedModel, effectiveModel and canonicalModelKey
   (claim_fingerprint.go, claim.go). Specify the single point where the
   alias rewrites the id so that EVERY later consumer (the hazard gate,
   the cap lookup, the fingerprint, the record fields, the adapter's
   --model argument, the composition's obligations) sees only the
   canonical id, and state which of those consumers would still see the
   old id if the alias were applied anywhere later. Decide whether the
   --model override on `delegate` is aliased too (the dispatching seat
   says yes: the same word covers it).
2. WHERE THE ALIAS TABLE LIVES. Two candidates: a tracked configuration
   key family in metasystem/metasystem.conf, e.g.
   `runtime.claude.model-alias.claude-fable-5=claude-fable-5-1` (the
   pointer; a later 5.2 is one landing of this line), validated
   in metasystem/internal/config/validate.go like maximalModelsKey (no
   empty tokens, no alias to itself, no alias whose target is itself an
   alias, target must be listed wherever the source would have needed to
   be); or a hard-coded retired-id map in the roster code. Choose one and
   say why; the dispatching seat prefers the tracked key because the
   pointer must move by a configuration landing on Wido's word, never a
   code build, and never by the engine asking the API what is latest
   (state that non-goal explicitly). Whatever you choose, the old
   id must never reach the Claude CLI (R-46-m0b: "claude-fable-5 is
   retired from dispatch and survives only in historical records and as
   arbitrary fixture strings").
3. CAP ROWS. A seat's conf.local may carry
   cap.min.code-critic.claude.claude-fable-5=30 and no row for the new
   id. State whether the cap lookup reads the row keyed by the canonical
   id only (the seat must add rows for the new id, refusal names the
   missing row), or falls back to the aliased id's row. Pick the one that
   cannot silently give a role a smaller cap than its operator wrote.
4. RECORDS AND REPORTING. A job dispatched through the alias must be
   readable as such afterwards: name the record field (or the existing
   RosterModel/RosterPair fields) that keeps the roster's literal id
   beside the canonical one, so a later "which seats still write the old
   id" sweep is one grep over the job records in the agents control plane. State the
   metasystem.conf comment line that documents the key.
5. TESTS. Name each test and what it discriminates, at least: a roster
   on claude-fable-5 dispatches DESIGN-BEARING with effectiveModel
   claude-fable-5-1 and passes runtimeProvesMaximalExecution; the alias
   validator refuses a self-alias, a chained alias and an empty token; a
   --model claude-fable-5 override resolves the same way; a cap row on
   the old id behaves as item 3 decides; a fixture id that is NOT aliased
   passes through unchanged (the existing composition_test fixtures on
   claude-fable-5 as arbitrary strings, see
   metasystem/plans/fable-5-1-rollover-design.md item 3, must keep their
   meaning or be renamed, say which).
6. NON-GOALS. No new CLI verb; no change to R-25-m1 lane structure; no
   change to the maximal-models line's meaning; no aliasing of codex or
   devin ids unless the design shows it costs nothing (say so either
   way); no migration of any seat's conf.local.
7. THE RULING ROW. Specify the rulings row that records Wido's word
   (R-71-m3, dated 2026-09-03, quoting both orders verbatim including
   the spelling), as R-46-m0b did for the rollover.

# Constraints

- Under 100 lines. Sections numbered as above; every claim about the
  code cites file and function.
- Read metasystem/plans/fable-5-1-rollover-design.md first: this design
  extends it and must not contradict its live-round safety argument.
- KNOWN SANDBOX LIMIT: reading only; do not run the engine.
- Wall-clock budget: 30 minutes.

# Expected Return

Version-2 JSON; the design file path; the open questions the design
could not settle listed as gaps, not answered silently.

# Gap Rule

stop and report a gap; never fill it silently — especially anywhere
the order underdetermines a choice (items 1 and 3 are the likely ones).
