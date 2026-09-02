Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal turn-verdict-hardening)
Date: 2026-09-02

# Goal

Revision 3 of metasystem/plans/turn-verdict-hardening-design.md (revision 2
landed, in your worktree). Sol's round-2 register is
metasystem/records/misc/turn-verdict-hardening-critique-r2.md: four of the
nine TVH-R1 closures are certified real, three are PARTIAL, and two NEW
findings were raised. Fold the five material items below, each closure marked
with the finding identifier. Edit the design in place; the diffBoundary is
that one file.

# The folds, by id

1. TVH-R1-R3-NAMES-ILLEGAL-EXIT (partial). Two defects. (a) The byte-verbatim
   Move strings omit the mandatory `--id` flags — read
   metasystem/cmd/metasystem/goalsync_mutations.go and render the exact
   accepted syntax, lineage flag included. (b) One machine may lawfully hold
   several claims sharing one non-empty arc
   (metasystem/internal/goal/validate.go, the machine-quota rule): if H1 and
   H2 are held in arc A and queued g is in arc B, parking H1 alone still
   fails the quota for g, so R3 would repeat unusable advice on every Stop.
   Specify R3 over the SET of the pair's exhausted claims: the Move must name
   every claim that has to be parked or released before g admits, or R3 must
   recompute admission against the ledger-after-Move and only render a Move
   the engine would then accept. Add the two-arc fixture to slice 1's tests.
2. TVH-R1-FAIL-CLOSED-TABLE-OMITS-PREVERDICT-SHELL-EXITS (partial). Section
   3.0 rule 4 says only emit_stop_payload sets emitted=1, yet P2 and rows F1,
   F2, F5 prescribe direct printf JSON then exit 0 — the EXIT trap then prints
   a second block object. Make every emission path go through the one
   emitter (or have the emitter be the only thing that can print) so exactly
   one JSON response leaves the hook on every path; add a fixture asserting
   single-response on F1, F2, F5 and on a trap exit.
3. TVH-R1-STOP-DEADLINE-DOES-NOT-BOUND-EMISSION (partial). The arithmetic is
   right; the "every step bounded" claim is false: root mapping, runtime
   lookup, payload handling, hook-attempt recording, ancestor discovery,
   lease classification, JSON extraction and response construction are
   outside run-bounded; the clock starts after the engine test so world
   mapping is uncharged; and the marker lock's five-second wait exceeds the
   3.5 s verdict allowance. Fix all three: start the clock at hook entry;
   either bound every external command (git, engine calls, the classifier)
   under run-bounded with a cap in the table, or state precisely which pure
   shell steps are exempt and why they cannot block; give the marker lock a
   cap that fits inside the verdict allowance (or shrink another cap and show
   the new sum). Note also that `up` is sequential, not atomic
   (metasystem/internal/up/up.go): state what a mid-kill leaves behind and
   why the next `up` repairs it, or move `up` out of the Stop path.
4. TVH-R2-SLICE1-HIDDEN-WRONG-ROOT-DEPENDENCY (critical). Sol is right: on
   the affected fleet seats the shipped hook exits at the missing-engine
   branch (metasystem/scripts/agents/supervision-hook.sh lines 26-30) before
   any slice-1 verdict runs. The dispatching seat's DECISION: slice 1 is
   SEQUENCED BEHIND goal supervision-hook-wrong-root landing first (its design
   metasystem/plans/supervision-hook-root-design.md is at revision 3, two
   folds from closure; the machine's single claim slot moves to it next).
   Rewrite section 10 so slice 1's row carries that dependency explicitly,
   state which of slice 1's hook lines then land on top of the root fix, and
   restate the specimen claim honestly: "slice 1 refuses all three specimens
   at the verdict boundary; at the deployed Stop boundary once the root fix
   is in". Slice 2's dependency row is then redundant — say so.
5. TVH-R2-HUMANSTOP-SEAT-AUTHORITY-UNSPECIFIED (high). Section 5.2 names
   ProveOrTemporaryGoalAuthority, but that proof only exposes
   AuthorizesSetObligation and AuthorizesResume
   (metasystem/internal/humanauthority/authority.go), the command has no
   caller-pid or seat contract, and `machine`/`lineage`/`relayedBy` would be
   caller-controlled text. Specify: a HUMANSTOP-scoped authorization method
   (AuthorizesHumanstop, with the ruling text it must cite); the seat written
   into the marker derived from the SAME authenticated classification the
   verdict uses (ClassifyVerbAt over the caller pid, not from flags); a
   marker may only be minted for the minting seat's own pair; and add the
   forged-lineage and cross-seat minting cases to slice 4's tests.

Also fold Sol's declared gap on the 85-builder-minute estimate: the 207-line
two-minute precedent was recovered prewritten work, not authored code. Either
re-estimate from jobs that authored their diff (name them from the job records under the agents artifacts directory) or state the estimate as unsupported and re-cut slice 1
so that even a doubled estimate fits one 120 cap plus one correction round.

Consistency pass over sections 7, 9, 10, 11 (the ladder, the asks — record
the wrong-root sequencing decision there — the slice table, the self-grade,
the reject condition). R-31: no benchmarks.

# Constraints

Wall-clock budget: 40 minutes. Design only; edit nothing but the design file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the design file.

# Gap Rule

stop and report a gap; never fill it silently.
