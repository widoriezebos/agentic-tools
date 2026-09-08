Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal backlog-ordered-by-priority)
Date: 2026-09-08

# Design critique: the backlog gets a priority and a sequence, revision 1

FINDING IDS: chain-unique, BOP-01, BOP-02, ... never F-n.

Read metasystem/plans/backlog-ordered-by-priority-design.md, revision 1,
written by Astra this hour. This is its one independent critique. After
your findings are disposed, the build proceeds behind the fixtures the
page names, whatever a further read would say, so raise now anything
that would change what gets built.

The contract is the goal record
metasystem/plans/goals/backlog-ordered-by-priority.md, which carries
Wido's sentence verbatim: "Backlog items need to be sorted in order of
priority and then a sequence number. And this should be able to update
after the fact so that we can change priority of the backlog." Judge the
design against that sentence and against the DONE definition beneath it,
not against your own preference for how ranking should work.

# Mandate

1. **Does it do what was asked?** Every clause of the DONE definition:
   the pair on every record, set and changed after the fact by a human
   verb at the enrolled terminal, re-sequencing when a number is
   inserted, recorded as a ledger event like set-pin, an ordered
   listing, a next verb tied to the claim rule and pins, unranked
   sorting last so no bulk edit is needed, and the channel status
   showing the head.
2. **The invariant.** The page requires the ranked open goals of each
   priority to occupy exactly 1..N, once each, and validation refuses
   holes and duplicates. Is that invariant actually maintainable through
   every lifecycle transition the ledger has, including done, park,
   unpark, steal, resume, breach-stop, arc detach and the approval
   sweep? Name any transition that can strand a hole or a duplicate, and
   any that would make a routine human act refuse.
3. **The races.** Two seats publishing rank changes at once, and a human
   act racing a seat's claim. The page leans on the existing publish
   seam. Is the convergence it claims real, and does it hold without
   sleeps?
4. **Authority.** Rank is a human act. Can any agent path set, shift or
   erase a rank, including through reconcile, recovery, the sweep, or a
   stored human string from a dead owner? Rank must not widen what an
   agent may do, and must not narrow what a human already could.
5. **The selection rule.** Does `next` respect one claim per machine,
   pins as a filter rather than a rank, blocked and unapproved goals,
   and does it fail honestly when nothing qualifies? Could it hand two
   machines the same goal?
6. **The canaries.** Section 6 names, for each fixture, the smallest run
   that proves it. Are those the smallest runs, do they actually
   discriminate the behaviour, and does any of them silently depend on a
   battery? A canary that cannot fail for the right reason is worse than
   none.
7. **Scope.** The design must not invent a scoring system, a decay rule,
   or anything that ranks without the human, and must not change what a
   claim or an approval means. Flag any place it does.

# Constraints

Wall-clock budget: 45 minutes. Read-only: write no file other than your
return. Return per the design-critic schema. Say for each finding
whether it changes what gets built, and if it does not, mark it
material=false and keep it short. Gap rule: stop and report a gap; never
fill it silently.
