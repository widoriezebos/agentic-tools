# Fable: manual review final critique

Round 2 of 2 on SAME provider session; failsafe round 2. Root read the entire
round-1 report and joined IM-C1..3 and N1..6 in intent-manual-review-dispositions.md.
Read revised designs/intent-manual-review.md in full and that disposition table.
Parent/planning chains remain closed, unfinished concurrent Opus code is not the
review subject. Design is root-authored; you report only.

Criterion verbatim:
> Would an implementer working from this design build **step 1** DIFFERENT, or WRONG, because of this finding?
> Does step 1 WORK, and is it SAFE, without this finding?

Also challenge honest buildability of the REQUIRED manual-delivery second slice;
we cannot declare the user's full capability request complete with diagnostics only.
The fold deletes the new manual registry and CommitPatch, uses CommitStaged's real
consistent index under its token, uses branch range work visibility, and replaces
invented manual attempt N with --after COMMIT (publicly supplied immutable version).

Root found limits in your suggested remedy: index emptiness says nothing about a
separate dirty source; whole branch tree includes later units/read records; --cached
only application can leave destination files behind the index. The revised design
uses --index for a distinct clean destination and checked staging of already-present
captured paths for source=destination. Replay compares the selected unit's actual
parent/tree; amendment compares against named prior commit and current unit.
Existing push journal handles uncertain publication; no invented commit opid proof.
The report's suggested review G --work NAME still collides for G=changes, so there
is explicit review goal G accepting all goal flags and collision-safe continuations.

Challenge those concrete boundaries, especially whether required patch application,
staging interruption and comparison can truly reuse owners without another registry.
Do not ask for a general importer, workflow or new policy. Distinguish named bounded
fixture corrections from an invariant/contract shape failure. Report any real defect;
max two rounds is not permission to certify an unsafe design.

At most 24 tool calls / 10 minutes; <=1500 words. Stable ids IM-C4 onward, answers
to both materiality tests, exact smallest correction, reviewed SHA, unexamined scope.
Read-only source; no edits except plans/intent-manual-review-fable-report-r2.md.
No product changes, agents, builds, tests, goals, commits or secret configuration.
