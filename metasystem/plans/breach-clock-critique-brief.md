Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal breach-clock-and-budget-honesty)
Date: 2026-09-02

# Goal

Round-2 critique of metasystem/plans/breach-clock-and-budget-honesty-design.md
(revision 2, landed, in your worktree). Your round-1 register is
metasystem/records/misc/breach-design-critique-r1.md: eight material findings,
BCD-R1-001 through BCD-R1-008. Revision 2 claims to close every one by a
design change verified against the tree, never by softening a requirement or
weakening a refusal; where revision 1 promised what the tree could not
deliver, the promise is replaced by a stronger mechanism and the replacement
is named. It also carries the second duration specimen with the parser's
actual reading: at metasystem/internal/goalbudget/budget.go line 38 a d token
is eight working hours, so a typed 8h stored as 1d was enforced as eight
hours and only the display lied. The goal record's earlier claim that it
enforced twenty-four clock hours was wrong and has been corrected; that is a
decision, not a finding. The document ends with a disposition table for your
round-1 findings and the required self-grade.

# Your mandate

1. CLOSURE CHECK, one verdict per finding, against the tree in your
   worktree:
   - BCD-R1-001: the stop-authority resolver and the resume command path for
     a parked-with-breach goal (section "Verb-by-verb mechanics"; read
     metasystem/cmd/metasystem/goalsync_mutations.go and
     metasystem/internal/dispatch/stop.go).
   - BCD-R1-002: the cancellation-duty invariant "a fence is never off the
     route" (its own section under Fix 3; read FindBreachStops and the
     custodian in metasystem/internal/dispatch/stop.go and the tick in
     metasystem/internal/steward/tick.go).
   - BCD-R1-003: discharge proofs bound to the claim episode (Fix 1; read
     metasystem/internal/dispatch/budget.go and
     metasystem/internal/goal/verbs.go).
   - BCD-R1-004: "One claim per machine, fenced or not" and the enumeration
     of every consumer of the claimed set (read metasystem/cmd/metasystem/goal.go,
     metasystem/internal/goal/goalverbs.go, metasystem/internal/goal/turnverdict.go,
     metasystem/internal/goal/project.go).
   - BCD-R1-005 and BCD-R1-006: the d-token refusal that replaces the era
     marker, and the rollout table (Fix 2; read metasystem/internal/run/run.go,
     metasystem/internal/run/conclude.go and the journal replay in
     metasystem/internal/goal/budget.go).
   - BCD-R1-007: the split parse rule and the mapper contract (read
     metasystem/internal/goal/reconcilemap.go and reconcilepub_test.go).
   - BCD-R1-008: the migration section and the alert-escalation-channel
     specimen's before-and-after (read metasystem/plans/goals/alert-escalation-channel.md).
2. ATTACK THE TWO DECISIONS the designer flagged as riskiest in the
   self-grade: (a) consumed discharge proofs bind to the claim episode rather
   than the exact claim revision, which changes what a human set-obligation
   does to a discharged clock inside one episode — is that a consequence of
   the goal's own thesis, or adjacent scope that needs Wido's word before it
   is built, and is the pinned test sufficient; (b) refusing every d token at
   set time — does any writer, replayer, reader, fixture, or documented
   human grammar still emit or expect a d (search plans/goals, docs, scripts
   and fixtures), and is the one display regression an old binary can write
   correctly bounded and honestly stated.
3. ATTACK THE PROOF PLAN: does every named test sit at the seam that can
   actually fail (the command seam for 001, the dispatch producer seam for
   stop evidence, the mapper for 007), and is any invariant in the design
   stated without a test that would catch its violation.
4. NEW FINDINGS only if material and grounded.

Findings quote the disagreeing text or code. Your sandbox is read-only:
verify by reading, do not run go. Declared gaps and disposition rows are
residuals, not findings, unless one hides an escape. Zero material findings
is an acceptable, closing answer if the reading supports it.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
