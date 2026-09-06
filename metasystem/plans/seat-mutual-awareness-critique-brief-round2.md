Working Mode: design
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal seat-mutual-awareness)
Date: 2026-09-06

# Design critique, round 2: seats see each other and ask each other

FINDING IDS: chain-unique, continue the sequence: SMA-C-11, SMA-C-12, ... never F-n.

The design under review is
metasystem/plans/seat-mutual-awareness-design.md, revision 2, landed on
main as commit 2e109816; its SHA-256 is
809d8e3213f16f223bf306f3a10c43dc33f63f9e0f5f4ea25939bb7fda808abd.
The "Declared Outputs" digest line the dispatcher stamps into your
prompt is the digest of the outputs manifest, not of the design; do not
stop on that difference. Round 1 is
metasystem/records/misc/seat-mutual-awareness-critique-r1.md (ten
material findings, all accepted); the designer answered them in the
text and in the dispositions table at the end of the design, working
from metasystem/plans/seat-mutual-awareness-fold2-brief.md. The goal
record is metasystem/plans/goals/seat-mutual-awareness.md. Wido's
binding word (a single-use code on anything inbound from outside; a
seat's words carry no human authority) stands.

Round budget: this is the second of the goal's review rounds and
should close the ladder if the design holds. Material only if an
implementer working from the design would build something different or
wrong.

# Mandate

1. Each round-1 finding: is the disposition's answer in the design
   text actually sufficient, or does it move the problem? Rule per
   finding, briefly; reopen with a new id only where the answer fails.
2. The enable marker (the human's one-time act that fences the first
   writer): is it fail-closed against an old checkout in every path the
   round-1 finding named (the guard regexp, the generic plan-record
   class, ordinary landing of a new plan record), and does an upgraded
   validator's refusal of a spoiled tip leave the fleet a way out that
   the design names? This is the design's riskiest addition; test it
   hardest.
3. The membership record and the unreachable/unknown split: can a seat
   still ask a machine that will never answer without the asker
   learning it from the record?
4. The deadline and late-answer rules: exactly one outcome per case,
   including a timeout omitted, a target that never runs a steward, an
   answer that lands in the same second as the deadline.
5. Recovery: does each seat verb's journal entry carry everything its
   record needs, and are the recovery outcomes named for every path the
   round-1 finding listed?
6. The proof matrix and the estimate: does the matrix certify the
   riskiest behaviour now, and is the raise the design names (three
   days, ten attempts, 480 job-minutes) honest for the slices it lists?

Ground every finding in file-and-line evidence read in the worktree.
Plain English throughout.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema; the
declared outputs manifest names the one record you write.

# Gap Rule

stop and report a gap; never fill it silently.
