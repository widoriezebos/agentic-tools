Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal turn-verdict-hardening)
Date: 2026-09-02

# Goal

Round-3 critique of metasystem/plans/turn-verdict-hardening-design.md
(revision 3, landed, in your worktree). Your round-2 register is
metasystem/records/misc/turn-verdict-hardening-critique-r2.md: five material
items (three partial closures from TVH-R1, two new TVH-R2). Revision 3 folds
all five, re-cuts the build into seven slices under a doubled-estimate rule,
records the dispatching seat's decision that slice 1a is sequenced behind
goal supervision-hook-wrong-root (section 9 ask 7), and the seat answered
ask 9 KEEP (the enrolled-terminal HUMANSTOP form stays; the human at an
enrolled terminal names the seat). Those two seat decisions are decisions,
not findings.

# Your mandate

1. CLOSURE CHECK, one verdict per item, for the five round-2 material
   findings: TVH-R1-R3-NAMES-ILLEGAL-EXIT (the SET form of R3 in section
   1.2.1 — read metasystem/internal/goal/validate.go for the arc rule and
   metasystem/cmd/metasystem/goalsync_mutations.go for the exact verb
   syntax); TVH-R1-FAIL-CLOSED-TABLE-OMITS-PREVERDICT-SHELL-EXITS (the
   single-emitter invariant in section 3.0 against
   metasystem/scripts/agents/supervision-hook.sh);
   TVH-R1-STOP-DEADLINE-DOES-NOT-BOUND-EMISSION (section 3.2: clock at
   entry, what is bounded, what is exempt and why, the marker-lock cap, and
   the decision in 3.2(e) about `up` on the Stop path against
   metasystem/internal/up/up.go); TVH-R2-SLICE1-HIDDEN-WRONG-ROOT-DEPENDENCY
   (section 10 slice 1a's dependency row and the specimen claim as restated);
   TVH-R2-HUMANSTOP-SEAT-AUTHORITY-UNSPECIFIED (section 5.2: the seat derived
   from the caller classification, never from flags or environment; read
   metasystem/internal/humanauthority/authority.go and the classifier in
   metasystem/internal/lease for what the design cites).
2. ATTACK THE SEVEN-SLICE CUT (section 10): is every slice at most 240
   reserved minutes with a correction round intact; does slice 1a plus 1b
   still refuse all three specimens at the verdict boundary; does the
   ordering window the design admits between 1b and 2a (pre-verdict shell
   exits stay fail-open until 2a) hide a specimen escape the seat should
   close earlier?
3. ATTACK SECTIONS 7 AND 8 as rewritten: the ladder's class-A set, the
   unwatched-work row outside HUMANSTOP's reach, the revival-latency
   regression from removing `up` from the Stop path — is any of it a new
   escape or a new refusal loop?
4. NEW FINDINGS only if material and grounded.

Findings quote the disagreeing text or code. Your sandbox is read-only:
verify by reading, do not run go. Declared GAP rows in section 9 are
residuals, not findings, unless one hides an escape. Zero material findings
is an acceptable, closing answer if the reading supports it.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
