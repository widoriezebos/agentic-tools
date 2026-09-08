# Backlog ordered by priority: third read of slice 2 (chain bolnext-build1, round 5)

Critic bolnext-crit3 (code-critic, Opus 5) on reviewed tree e13a0040a428d67f4f72f332220ea6c65c2ab861. One material finding, medium, and three notes. The four findings of the two earlier reads are confirmed folded.

## BOP2-01 - medium - material=True

CLAIM: The fleet's chat status post will repeat itself forever on a quiet machine. The report folds the projection's staleness notice into its new backlog line, and that notice names the accepted tree's age rounded to the minute. The channel decides whether to post by hashing the report with only the status header removed, so the age text is inside the hash. The notice appears whenever the accepted tip's own commit is more than thirty minutes old, which is the ordinary state of a repository between ledger commits, and the age grows with the clock. So the hash differs at every cadence tick and the machine posts a fresh status every interval even when the backlog head, its own candidate, the questions, the deliveries and the undelivered count are all unchanged. Before this change the report carried no projection notices, so a quiet machine posted once and stayed silent.

EVIDENCE: internal/goal/project.go:93 builds the notice with the age rounded to the minute; :88-93 measures it from the accepted tip's commit time and :37 sets the thirty-minute threshold. internal/channel/report.go:137-150 appends every projection notice to the backlog line, :122 places that line in the body, :288-299 hashes the report after removing only the header, and :329-331 posts when the interval has elapsed and the hash differs.

The critic named two repairs: fold a fixed staleness phrase with no varying number, or hash the report with the reserved notice removed the way the header already is.

## BOP2-02 - low - material=False

CLAIM: One error that IS about a single goal travels on the new fleet-wide could-not-answer channel: a tier outside 1 to 3 produces a configuration-layer message below the point where goal-specific refusals are separated.

## BOP2-03 - low - material=False

CLAIM: The steward's list of goals pinned to this machine changed from alphabetical to backlog order, because it is built in the same loop whose sort call was removed. The page asked only for the waiting-queue projection to follow the ordered traversal.

## BOP2-04 - low - material=False

CLAIM: A human re-ranking the backlog produces a steward attention event saying the waiting queue changed, which is the intended consequence of making the diagnostics follow the order.

## Coordinator's reading (m1b, 2026-09-08)

BOP2-01 accepted and folded in round 6. It is graded medium and it
changes nothing a machine does, but it violates section 5 of the page in
its own words, that the existing digest and post decision already notice
changed content at the normal cadence and that there is no extra post or
new channel event. The cost falls on the human as a status message every
cadence tick on an idle machine, forever, which is precisely the kind of
noise that trains a reader to stop looking.

The three notes are recorded and not actioned. BOP2-02 is the closest to
worth doing: a malformed tier is a fact about one goal and rides the
fleet-wide channel meant for questions that cannot be answered at all.
It is narrow enough to leave, and the fold brief does not touch it,
because widening the separation is the kind of unasked change that has
cost this chain three rounds already.

Counts for the record: one build round, four folds of which one stopped
correctly on a design gap and one failed on a protocol slip, and three
reads. The findings per read fell from two, to two, to one, and the
severity fell from a fleet stall, to a silent fleet-wide stand-down, to
a repeated chat message.
