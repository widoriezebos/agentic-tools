Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Design critique: section 18 of the stop design (goal metasystem-stop-verb)

FINDING IDS: chain-unique, S18-01, S18-02, ... never F-n.

Section 18 was written by a design round in answer to the fifth code
read of the built slice, which found three material failures in what a
person is told: a refusal with no way forward, one thing printed up to
five times, and status calling an incomplete stop simply stopped. The
design round's job was to decide those three by applying the page's
existing rules rather than by adding machinery.

Read the whole page: the amendment is cumulative on sections 1 to 17 and
wins where it contradicts them. Take section 18 from the design round's
worktree; the orchestrator has not edited it.

Why this review exists, plainly: five amendments to this page (sections
13 to 17) were written by the coordinator between build rounds and never
critiqued as design, and one of them, the completeness barrier of 13.1,
became the most defect-dense part of the slice and produced the worst
finding of two consecutive code reads before section 17 simplified it
away. Section 18 is the first amendment to get a design read before it
becomes a build brief. Judge it on that history.

# Mandate

1. Does section 18 decide all three findings, at the page's level, so an
   implementer has no room to invent? Name any decision that is a
   restatement of the problem rather than a decision.
2. Does it introduce a mechanism, a record, a verb, a barrier or a pass?
   It claims it does not. If it does, say so: that claim is the thing
   most worth checking, given 13.1's history.
3. Does it contradict any earlier section other than where it says it
   wins, and is each of those wins deliberate and stated?
4. Its invariant, that the number of final NOT STOPPED lines, the number
   of durable survivor entries and the closing count are equal: is that
   consistent with sections 15.1, 15.3, 15.4, 17.1 and 17.4, including
   for adopted runs and untracked processes which are reported but never
   counted?
5. Its identity rule for suppressing repeated lines, the family's
   recorded thing and process incarnation including machine: can two
   genuinely different things collide under it, or one thing appear
   twice, in any family the page names?
6. Are the proofs it requires nameable in the beds section 10 assigns,
   without renaming or removing an existing scenario?

Material only if it changes what gets built and you name the artifact it
would change. Polish is not material; the slice is otherwise green and
about to land.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema. Gap
rule: stop and report a gap; never fill it silently.
