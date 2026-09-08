Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal backlog-ordered-by-priority)
Date: 2026-09-08

# Build brief: slice 2 of the backlog order, taking work through the rank

Slice 1 landed at f32fbfeb: every goal record can carry a priority and a
sequence, `goal set-priority` is a human act at the enrolled terminal
that re-sequences the affected band, and `goal list` prints the order
with unranked last. Wido has since ranked the backlog, and his words
today are the reason this slice exists: "The backlog is now prioritized
and has a sequence. However, GoalNext does not yet adhere to this."

The specification is
metasystem/plans/backlog-ordered-by-priority-design.md, revision 2,
final. Its critique ladder is closed and no further prose revision will
be written. Sections 1 to 3 are built and landed; this slice builds what
remains.

## Scope

1. **Section 4, `goal next --machine <nick>`.** It returns the first
   approved, unclaimed goal in priority-then-sequence order that is
   unpinned or pinned to that machine, and seats take work through it
   instead of reading records. It respects the one-claim-per-machine
   quota, treats pins as a filter rather than a rank, skips blocked and
   unapproved goals, and fails honestly when nothing qualifies. Note the
   page's own limit, which is not a defect to fix here: `next` is a
   read, not a reservation, so two machines can be handed the same
   candidate and claim publication admits one winner while the loser
   reselects.
2. **Section 5, the channel status head.** The report shows the top of
   the order, with the global and local meanings the page distinguishes,
   so the human sees what the fleet will do next.
3. **The seat projection owners** the page names after section 4. The
   page is explicit that these are diagnostics rather than alternate
   eligibility decisions: display the ranked order rather than pulling
   pinned items ahead, filter the same ordered traversal rather than
   sorting by age, and keep the idle digest insensitive to order so a
   reorder does not reset idle enforcement.
4. **The deferred read-side fetch option**, recorded as BOB-03 in
   metasystem/records/misc/backlog-ordered-by-priority-critique-r2.md.
   Section 3 asks for it on the synced list and next readers; slice 1
   added it to the listing and deliberately left `next` untouched. Add
   it to `next` through the existing projection rather than a second
   fetch path.
5. **The test helper that panics while reporting**, recorded as BOC-03
   in metasystem/records/misc/backlog-ordered-by-priority-critique-r3.md.
   The helper that asserts a priority and sequence pair checks for a nil
   record and then reads two fields off that same nil record inside the
   message meant to explain the nil, so a missing goal panics instead of
   reporting. One line.

## Not in this slice

BOC-01 from the same register stays out: a goal that survives a
neighbour's conclusion gains a done event, and the metrics reader scans
history backwards for the last done verb, so one report calls an open
goal finished. The critic named two candidate repairs, a marker on the
event or a filter on the reading side, and choosing between them is a
design decision this page does not settle. If you find yourself needing
to choose, stop and report it as a gap rather than picking.

Do not change what a claim or an approval means, do not turn `next` into
a reservation, and do not add authority anywhere.

## How to prove it: canary first

Section 6 of the design names, for each fixture, the smallest proving
run and a two-minute ceiling. Use them exactly as written for the rows
that belong to sections 4 and 5, run the subtest for the changed
behaviour first, and widen to the owning package only to answer a
question the canary leaves open.

The pinned-machine row is the one that matters most: rank A at 1:1
pinned to one machine, B at 1:2 unpinned, C at 1:3 pinned to another;
a free first machine selects A, and a free second machine and an
unpinned third both select B, which proves a local pin confers no extra
rank.

Every refusal observation names the reason it requires and keeps every
unrelated prerequisite valid, so only the defence under test can produce
it. That rule is not optional here: four canaries in this program have
been found unable to fail for the right reason, and the last one was
caught by asking whether an unsigned-parsed integer could be negative.
If an assertion you write cannot fail, it is not a test.

## Verification to report

- Each canary you ran, individually, with what it observed.
- `go build ./...`
- `go test` once for the packages you touch, after the canaries pass.
- `scripts/agents/go-gate.sh --fast`, once, at the end. If staticcheck
  cannot write its cache in your sandbox, redirect it and say so.

The orchestrator runs the process-owning beds and the receipt-bound
battery outside your sandbox.

Gap rule: stop and report a gap; never fill it silently.
