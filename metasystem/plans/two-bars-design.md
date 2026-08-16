# Two bars for changes (backlog item 2)

- Status: DRAFT r1 — grounded in the commit wrapper (scripts/agents/commit.sh)
  and the pre-commit guard (scripts/agents/pre-commit-guard.sh). Under critique.
- Goal: two-bars-for-changes
- In flight right now: the r1 design critique (codex xhigh). It is
  not a dispatch job record, so the open-work scanner cannot see it
  (KI-34's blind spot); fold it on return.
- Next step: none.

## The human's rule (2026-08-08, backlog-notes item 2)

A change takes ONE of two bars: the DESIGN LOOP (design →
adversarial critique → converge → implement) for design changes, or
a DIRECT FIX for mechanical defects. The rule was run for one
unattended session and worked; the human asked whether it should be
standard, "maybe with a few conditions". The conditions ARE the
design — without them, "it's just a bug" becomes the escape hatch
that launders every design change into a direct fix.

Tonight is fresh evidence for both bars. The worktree observer
looked like a report feature but its critique found a DESIGN defect
(the reclaim proof was unsound); the loop caught it and it was
reverted (D89) — a design change that correctly took the loop. The
same session had genuine direct fixes (a stray build binary, a
commit that left files unstaged) where a design chain would have
been pure ceremony.

## The five conditions, made mechanical

1. **The classification is DECLARED, not assumed.** Every agent
   commit carries a trailer naming the bar it took and its basis, so
   the choice is auditable after the fact:
   - `Change-Class: direct-fix` with `Defect-Proof: <failing→passing
     evidence ref>`; or
   - `Change-Class: loop` with `Design-Chain: <plan or critique
     artifact ref>`.
   A commit with neither trailer is refused by the wrapper. Human
   commits stay sovereign (the guard already exempts them).

2. **A named NEVER-DIRECT-FIX set, enforced by path.** Some changes
   are design changes whatever they look like. The set is a declared
   MANIFEST of path patterns and, where a file mixes concerns, of
   in-file markers, covering:
   - INVARIANTS (the properties tests and predicates assert);
   - CONTRACTS/SCHEMAS other tools parse (the pinned ACP schema, the
     return schemas, the config domain, wire documents);
   - AUTHORITY boundaries (internal/authority, the lease/holder
     rules, genesis);
   - SAFETY mechanisms (supervision reaper/watchdog, the janitor
     destructive paths, stop-loss);
   - HUMAN RULINGS encoded in code or docs.
   A commit that touches a never-direct-fix path with
   `Change-Class: direct-fix` is REFUSED with the specific path
   named. Such a change must be `loop` with a `Design-Chain` ref.
   The manifest is itself a never-direct-fix contract (it cannot be
   edited by a direct fix), closing the "widen the escape hatch"
   move.

3. **When in doubt, the loop wins.** The default on an unclassified
   or ambiguous change is refusal, not silent proceed — the wrapper
   fails closed. Choosing `loop` is always allowed and never
   refused; only `direct-fix` is challenged. So the cheap path out
   of doubt is to take the loop, never to guess.

4. **A direct fix that grows escalates.** A `direct-fix` commit
   whose staged diff exceeds a declared size/scope budget (lines,
   files, or touching a second subsystem) is refused with the
   instruction to reclassify as `loop`. The budget is the
   design's number to set; the point is that "small mechanical fix"
   cannot quietly become a refactor.

5. **The EVIDENCE bar does not move.** Skipping the loop skips
   critique, NEVER proof. A `direct-fix` still requires the
   `Defect-Proof` ref (a test that failed before and passes after)
   and the same green gate every commit needs. The wrapper checks
   the gate witness for both classes; the loop adds critique on top,
   it does not replace proof.

## The mechanism (not memory)

commit.sh is the one agent commit path (it already takes the lease,
mints the wrapper token, and runs git commit). It gains a
classification gate BEFORE `git commit`:

1. Read the intended message (the `-m` value or the message file).
2. Require a valid `Change-Class` trailer; refuse if absent
   (fail-closed, condition 3).
3. For `direct-fix`: refuse if the staged set intersects the
   never-direct-fix manifest (condition 2), or exceeds the scope
   budget (condition 4); require a `Defect-Proof` ref (condition 5).
4. For `loop`: require a `Design-Chain` ref resolving to a real
   plan/critique artifact.
5. Both: the existing gate-witness requirement stands (condition 5).

The manifest and the trailer grammar are Go-owned (a `change class`
verb the wrapper calls, table-tested), so the rule is data the
suite checks, not prose a tired agent forgets — the human's standing
objection. The pre-commit guard already proves this hook shape works
(it fail-closes on new plan files and on a missing wrapper token).

## The anti-bureaucracy test (the real design question)

The rule earns its place only if the COMMON change stays one extra
line. It does: an ordinary mechanical fix adds
`Change-Class: direct-fix` + a `Defect-Proof` ref it already has
(the failing test it wrote). The friction lands exactly where it
should — on a change touching an invariant, schema, authority, or
safety path, which SHOULD stop and take the loop. If the manifest is
too broad it will nag on innocent edits (the KI-23 lesson: noise
teaches skimming); the design keeps it PRECISE, listing the specific
contract-bearing paths, not whole directories, and every refusal
names the exact path and the exact reclassification, so the way out
is always one obvious step.

## Prototype plan

P1: the `change class` verb — parse+validate a trailer set, evaluate
a staged file list against the manifest, evaluate the scope budget;
pure Go with table tests for each refusal and each pass. P2: wire it
into commit.sh before `git commit`, fail-closed; a fixture proving a
direct-fix on an authority path is refused while a direct-fix on an
ordinary path with proof passes, and a loop on the authority path
with a design-chain ref passes. The manifest ships small and grows
only through the loop (it is on its own list).

## Loop discipline

Codex xhigh. The critique should attack: whether the never-direct-fix
manifest can be specified precisely enough to avoid nagging without
leaving a laundering gap; whether the trailer belongs in the message
(commit.sh sees it) versus a commit-msg hook (the pre-commit hook
does not see the message); whether the scope budget is gameable
(split one design change across many small direct-fix commits);
whether the gate-witness/Defect-Proof checks are real or theater
given the wrapper cannot re-run the gate; and whether a human
override path is needed for the genuine emergency without reopening
the escape hatch.
