# Backlog ordered by priority: closing build critique (chain backlogorder-build1, round 2)

Critic backlogorder-crit3 (code-critic, Opus 5) on reviewed tree eaa78a1dca2ae64674694277ee9d9f7abaaf725c. One material finding, graded low, and six notes. It confirmed the previous read's class is closed: it checked every new canary against the question of whether it can fail for the right reason and found none dead.

## BOC-01 - low - material=True

CLAIM: A goal that merely survives a neighbour's conclusion gains a history event whose verb is done, and one existing reader treats that as the goal having been concluded. When a ranked goal is finished, the build compacts the rest of that priority and records the shift on each survivor using the departing act's own verb, which for a conclusion is done. The measurement code in internal/metrics/compute.go decides when a goal concluded by scanning history backwards for the last done verb, so `metasystem metrics --goal <id>` reports a finished lifecycle for an open goal, dated to the moment a different goal finished. This cannot wedge the queue, cannot refuse any human act and cannot corrupt the ledger; it makes one existing report state something untrue about an open goal.

EVIDENCE: Confirmed by execution in two steps. Concluding the middle of three ranked goals while the third is claimed leaves the live claimed survivor with history verbs open, approve, claim, done. Handing that history to the existing reader concludingEpoch returns concluded=true for a record whose state is claimed. A search of the whole reviewed tree found this to be the only place that infers meaning from a history verb on a goal record, so the equivalent split compaction event is currently harmless.

## Notes, none material

- BOC-02: split now records its event targets alphabetically rather than parent-first, including where no goal carries a rank. The recorded set is unchanged; only the order in the line differs.
- BOC-03: one new test helper reads two fields off a nil record inside the message meant to explain that the record is nil, so a missing goal panics instead of reporting.
- BOC-04: the compaction explanation is appended to an existing event's text, and one record-grammar predicate reads such text with an exact word-count shape.
- BOC-05: the refusal for a hand edit that touches a rank names set-priority as the remedy, which does not fit the case where someone concludes a goal by hand and deletes its rank lines in the same edit.
- BOC-06: the captured-tip refusal was widened to name the repair route, and one page under plans/ still quotes the old wording.
- BOC-07: no new canary is dead. One is weak rather than dead: the malformed-rank grammar cases assert only that some complaint mentions Priority or Sequence.

## Coordinator's reading (m1b, 2026-09-08)

Slice 1 lands with BOC-01 open, and this is the coordinator's call
rather than the critic's, so the reasoning is recorded here.

The finding is real and the critic's own grading is the reason it does
not block: it cannot wedge the queue, cannot refuse a human act and
cannot corrupt the ledger. What it does is make one report,
`metasystem metrics --goal <id>`, describe an open goal as finished. The
critic also names where the repair belongs: either the reading side
ignores a compaction event, or a marker distinguishes a compaction from
a conclusion. Choosing between those two is a design decision, and the
design mandated the verb reuse in words without considering this
consequence, so a fix needs a design round, a build round and a fresh
read. That is three attempts against one remaining in this goal's box.

So BOC-01 is carried into slice 2, which has to touch this area anyway,
and it is written into the goal's next step with the two candidate
repairs and the exact reader that misreads the event. It is unreachable
until the first rank is written, because no goal carries a priority
today, which is the whole grandfathered-backlog property the design was
built around.

The six notes are recorded and not actioned. BOC-03 is the one worth
picking up early in slice 2: a helper that panics while reporting a
failure costs a debugging session the first time a test fails for a
real reason.

The read also confirmed what the previous cycle set out to fix. The
first read of this build found a bed broken by a shape change that
thirty canaries could not see, and a canary clause that could never
fail. This read looked for both classes and found neither, which is the
first time in either of tonight's chains that a read came back without
a build-changing defect.
