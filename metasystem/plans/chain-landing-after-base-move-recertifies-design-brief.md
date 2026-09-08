Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal chain-landing-after-base-move-recertifies)
Date: 2026-09-08

# Design brief: a reviewed chain must still be able to land after its base moves

Deliverable: a new file
plans/chain-landing-after-base-move-recertifies-design.md (a NEW file,
created by this round), revision 1. Write the design only. No code, no
fixtures, no other file.

## Authority to author this design

The composed role prompt's mode table says the design itself is never
delegated. For design AUTHORING that line is superseded by Wido's
standing word in metasystem/memory/rulings.md:

- R-87-m1c, 2026-09-07, class=standing, verbatim: "I want designs done
  by codex + Astra xhigh". Design authoring uses codex:gpt-6-astra at
  xhigh.
- R-86-m1b, 2026-09-07, verbatim: "I would rather use Codex using
  Astra." The design CRITIQUE stays on gpt-5.6-sol, because a different
  model reading the design is the point of that lane.

He restated it today, 2026-09-08: "Astra design, canaries and keen eye
on looping." So writing this page and settling the decisions in it is
your task, not a conflict to stop on. What remains the orchestrator's:
the dispositions of the critique that follows, certification, the
ledger, and the receipt.

## The problem, and it is live

The contract is metasystem/plans/goals/chain-landing-after-base-move-recertifies.md.
Read it first; its DONE definition is the specification and this brief
adds evidence, not scope.

When trunk touches a file a reviewed chain also touched, between the
chain's base and its landing, the landing refuses and there is no way
through. The mechanism, traced in the current tree:

- `bindCertifiedChange` in metasystem/internal/landing/observe.go:394
  applies the chain's certified patch to the CURRENT HEAD tree, then
  compares the resulting entries for the certified paths against the
  same paths in the critic's `reviewedTree`
  (metasystem/internal/landing/observe.go:400-417). If they differ it
  returns "certified diff and reviewed tree disagree", which
  metasystem/internal/landing/observe.go:165 turns into the refusal
  `chain-output-mismatch`.
- Because the comparison is against the reviewed tree rather than
  against the chain's own base, NO candidate tree can satisfy it once
  trunk has moved a certified file. Rebuilding the candidate does not
  help; the check is not about the candidate.
- The dispatcher already sees the situation and only warns:
  metasystem/scripts/agents/dispatch.sh:2271-2277 prints
  "WORKTREE-BEHIND: the chain worktree is behind N commits, none on
  this chain's files" when the follow-up rebase plan declines to
  rebase. The plan itself is
  `PlanFollowUpRebase` in
  metasystem/internal/dispatch/followup_rebase.go:31, which rebases only
  when a trunk commit touched the chain's own files.
- A merge changes the tree the critic certified, so conformance and the
  critic's reviewed tree must be recomputed, and the review budget may
  already be spent. The round accounting lives in
  metasystem/internal/dispatch/finding_register.go:22-23
  (`reviewRoundLimit`, `criticRoundsConsumed`), set at
  metasystem/internal/dispatch/build.go:615-616.

Three real sightings, all in the ledger or tonight's records:

1. m1c landing chain shr-build1 at c1525b90 on 2026-09-06: the
   engine-rearm landing had touched the supervision hook and both
   supervision fixture files after the chain's base. Every hunk applied
   cleanly, the merged candidate passed the suite and the receipt
   seat-side, and the landing still recorded would-refuse because the
   certification named the pre-merge tree.
2. m1b, 2026-09-08, chain stopverb-build1: trunk commit 8ed97738
   touched two certified files. The orchestrator hand-merged, the tree
   was green across twelve packages and seven beds, and the landing
   refused. It went in with `git commit --no-verify`, bypassing the
   wrapper guard and the coverage ratchet, which a human had to
   authorise. Recorded in the commit message of a3131fbb.
3. m1b, 2026-09-08, chain backlogorder-build1: landed cleanly at bar a
   only because trunk had not moved a certified file in the interval.

## Why this matters more than its tier suggests

The goal sits on the critical path to headless fleet delivery. With two
or more nodes landing into one trunk, trunk moves under nearly every
chain, so this stops being an edge case and becomes the normal case. A
headless node cannot do what the orchestrator did on sighting 2: there
is no human present to authorise a bypass, and a node that bypasses the
evaluator unattended is worse than one that stalls.

## What the design must settle

The goal's DONE definition names the shape and one open decision. Settle
both.

1. **The named path.** Merge main into the chain worktree, recompute
   conformance, and then either:
   - a bounded re-review of the merge: one critic round that reviews
     ONLY the merge and does not count against the goal's box; or
   - a mechanical proof that no hunk of the chain overlaps a hunk main
     added to the same file, after which no re-review is needed.

   Decide which, or decide when each applies, and say why. If you choose
   the mechanical proof, define overlap precisely enough that two
   implementations agree, and say what happens when the proof cannot be
   established. If you choose the bounded re-review, say exactly how it
   is exempted from the round accounting above without opening a hole
   through which an ordinary review round could escape the box.
2. **What the evaluator compares.** After the merge the certification
   names a pre-merge tree. Say what the landing must compare instead, so
   that a merged candidate can pass bar a while a candidate that quietly
   changed a certified file still refuses. This is the heart of the
   goal: do not weaken the binding, re-aim it.
3. **Who does the merge, and when.** The dispatcher already computes
   behind-ness and prints the hint. Say whether the merge belongs to the
   follow-up path, to the landing lane, or to an explicit verb a
   coordinator runs, and what happens if the merge conflicts.
4. **The headless case.** State what an unattended node does when the
   path cannot complete: refuse and park with a named reason, or retry
   after a fresh pull. It must never bypass the evaluator.

## Prove it with a canary, not a battery

Wido's instruction, 2026-09-08: canaries, not batteries. For every
fixture you name, also name the SMALLEST run that proves it: one bed
scenario by name, or one Go package, or one test, with a ceiling. The
acceptance here is naturally a canary: one chain whose base moved,
landing at bar a instead of `chain-output-mismatch`. Build the fixture
list around that single observation and its refusal twin, rather than
around a full landing battery.

## Constraints

Stay inside the intent. Do not redesign the landing evaluator, the
critique register, or what a claim or approval means. Do not widen what
an agent may do. Where the intent is ambiguous, say so and give one
recommendation rather than listing options. This design gets one
revision and one independent critique, then the build proceeds behind
the fixtures you name; if you believe a second revision is needed, say
so plainly in the return instead of writing one.

Gap rule: stop and report a gap; never fill it silently.
