Working Mode: design
Orchestrator Identity: <dispatching seat>+<its session main> (dispatch delegate under goal account-provenance)
Date: 2026-09-02

# Goal

Round-2 (closing) critique of metasystem/plans/account-provenance-design.md
(revision 2, landed, in your worktree). Your round-1 register is
metasystem/records/misc/account-provenance-critique-r1.md: eight material
findings, folded by id in the design's fold-record table. Wido's word on
the goal (metasystem/plans/goals/account-provenance.md): until this lands,
m0 stamps "Wido@M0" in every landing message. Four declared gaps ride the
fold's return; one of them is a surface question the owner must judge:
the design adds two seam verbs in the existing adapter verb family
(account-object and codex-account) and a leaf Go package
internal/account.

# Your mandate

1. CLOSURE CHECK, one verdict per round-1 finding, against the tree:
   identity-verb-collision (the rename to 'account' and the four callers
   of the existing identity verb left alone: metasystem/cmd/metasystem/identity_probes.go,
   the adapters under metasystem/scripts/agents/adapters);
   codex-proof-bound (the narrowed grade and the Go seam that decodes the
   credential without printing token material); authority-and-disagreement
   (point observations only); retirement-condition (checked on the records
   by the landing evaluator, metasystem/internal/landing/observe.go, and
   printed by metasystem/scripts/agents/commit.sh; read how the evaluator
   resolves the calling main by ancestry and what happens when it runs
   outside the seat's process tree); non-gating-time-bound (the bounded
   executor, twenty-second ceiling, own process group, group kill,
   'unattested' with cause 'timeout'); runtime-registry-coverage
   (metasystem/internal/runtimes/runtimes.go); semantic-validation-fixtures
   (each named fixture pins the contract it claims); devin-unresolved-mapping
   (the unattested floor and the named live observation).
2. JUDGE THE SURFACE QUESTION for the owner: two new adapter-family verbs
   and a new leaf package, against the rule that an existing owner is
   preferred over a new surface (metasystem/docs/project-adaptation.md,
   metasystem/skills/take-a-step-back/SKILL.md) and the design's own
   'simplest durable record' rule. Could the existing adapter verbs or
   metasystem/internal/capability carry this without a new surface? Give
   a verdict the owner can act on: keep as designed, narrow, or return to
   design.
3. ATTACK THE RETIREMENT PATH end to end: from the announcement's account
   capture at metasystem up through job composition to the commit
   script's printed verdict, is there a state where the stamp retires
   while a record still lacks attested identity, or never retires on a
   healthy seat?
4. ATTACK THE FIXTURES AND THE BUILD SIZE against the goal's box
   (4h/6/240m/1 at the R-45 attempt count): does the build with its
   fixtures fit one slice with a correction round intact?
5. NEW FINDINGS only if material and grounded. Zero material findings is
   an acceptable, closing answer if the reading supports it; this is the
   closing round the goal's resume recipe names.

Findings quote the disagreeing text or code. Your sandbox is read-only:
verify by reading, do not run go. Declared gaps are residuals, not
findings, unless one hides a false claim.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
