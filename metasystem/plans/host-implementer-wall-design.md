# The host-implementer wall (goal host-implementer-wall, D99)

- Status: DRAFT r1 — under critique.
- Goal: host-implementer-wall (Current)
- In flight right now: the r1 design critique (codex xhigh); not a
  dispatch job record, so the open-work scanner cannot see it.
- Next step: none.

## What happened (the evidence this design answers)

bm-2d rep 1, 2026-08-17: the Devin host (swe-1-7) built the entire
mission solution itself — eleven product source files, the
requirements map, a clean build — in one turn, with ZERO dispatch
attempts across its 66 commands. The prompt's loop discipline
("advance streams by designing, dispatching, reviewing, and
certifying"; name anything too small to delegate) was ignored
wholesale. The runner then ACCEPTED a return whose `dispatched`
list was empty while the same turn's workspace carried the
eleven-file diff — the two facts sat in one record and nothing
compared them. The only check that fired was the measuring kit's
delegation floor, at grading time, after the spend. Wido ruled the
episode a total failure of the metasystem: the design→critique→
implement→critique loop is the product, and it was bypassed while
every mechanical stage hummed along.

## The invariant

A HOST TURN NEVER SHIPS IMPLEMENTER WORK. Concretely: at turn
acceptance, the workspace changes a host turn produced must be
attributable to something other than the host doing product work —
certified delegate output being integrated, orchestration state,
or a declared too-small-to-delegate exemption. A host return whose
turn diff contains unattributed product-code writes is a PROTOCOL
ERROR: the return is refused, the turn records the violation, and
the affected stream parks for adjudication (the same human-review
path other protocol errors take). Enforcement is mechanical — the
runner compares two facts it already holds — and never depends on
the model's disposition or the prompt's persuasiveness.

## The check, precisely

1. **Snapshot the workspace state before the host launch** — the
   runner already owns the turn directory; it records the git tree
   state (tracked-file hash set via git status/ls-files digest, plus
   untracked-file inventory under product paths) as `turn.pre`.
2. **Diff after the host exits, before return acceptance.** The
   post-turn state minus `turn.pre` is the turn's OWN writes —
   delegate work happens in separate worktrees and lands through
   certification, so it never appears in the host workspace diff
   uninvited.
3. **Classify each changed path:**
   - ORCHESTRATION SURFACES — allowed: `plans/**` (stream notes,
     ledgers), `artifacts/**` (records the machinery writes),
     `docs/reviews/**`, and the merge/integration writes belonging
     to a `certified` entry in the same return (the certified job's
     conformance diff names its files; integration writes must be a
     subset).
   - DECLARED EXEMPTIONS — allowed with a receipt: the return's
     existing "too small to delegate" declaration becomes a TYPED
     field (`selfWork: [{path, reason}]`) with a size ceiling
     (lines/files) the design sets; the declaration is auditable
     and the ceiling is a fence, not a suggestion.
   - EVERYTHING ELSE — product code (`src/**` and its siblings by
     the target's layout, build files, any path a certified diff
     does not cover): a violation.
4. **On violation:** refuse the return (`error:
   host-implementer-work`), keep the diff as evidence in the turn
   directory, park the affected streams with the protocol-error
   reason, and surface the adjudication ask. The work is NOT
   reverted by the runner — destroying bytes is never the runner's
   call; the parked stream's adjudication decides.

## Boundary questions the critique must attack

- The product-path set: derivable mechanically (everything not in
  the allowed orchestration set) or does it need the target to
  declare its layout? bm-2 targets have `src/`; adopted repos vary.
  Default-deny (anything unclassified is product) is the safe pole.
- The certified-integration subset check: the conformance record
  names the implementer's file boundary today — is that record
  precise enough to authorize the host's merge writes, and what
  happens when the host must resolve around a delegate's diff
  (KI-9: delegates cannot run git; the orchestrator integrates)?
- The exemption ceiling: what size keeps "genuinely too small"
  honest (the D99 run would have needed an exemption 11 files wide
  — any sane ceiling refuses it), and does the ceiling need Wido's
  ruling?
- Devil's bargains: a host could dispatch a sham delegate and
  claim its own writes as integration. The subset check against
  the delegate's conformance diff closes the naive form; the
  critique should hunt residual laundering shapes — and state
  plainly which ones only the grading-time delegation floor still
  catches (defense in depth, both layers stay).
- Mission-runner scope only, or every host turn? The dev-repo
  collaboration model explicitly has the orchestrator implement
  directly (KI-27 records it), so the wall binds MISSION hosts;
  the design must draw that boundary without leaking the mission
  discipline into interactive sessions.
- Failure posture: refuse-and-park is fail-closed for the mission
  but not destructive; is there a case where refusal itself loses
  work worth keeping? (The diff stays on disk as evidence either
  way.)

## What ships with it

- The turn-acceptance check in the runner (Go, table-tested on
  synthetic diffs; a fixture mission where a scripted host writes
  product code and the return is refused with the stream parked).
- The typed `selfWork` return field with its fence, replacing the
  prose-only "name it and the reason".
- The completion-gate rewording in the bm-2 family contracts: the
  gate describes WHAT must be true at the end, and gains one
  sentence saying the loop discipline governs HOW — a literal
  model must find no license for solo builds (the D99 host cited
  exactly that reading).
- The kit's delegation floor stays untouched: grading-time defense
  in depth behind the runtime wall.

## Loop discipline

Codex xhigh. This is an invariant change: no implementation before
the critique converges or the ratified mechanical-grain exit.
