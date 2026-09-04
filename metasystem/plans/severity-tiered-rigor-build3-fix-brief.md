Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-04

# Goal

Correction round on chain str-build3 (your reviewed tree 71f3ac42).
The code review (str-build3-cc1) found two material defects and five
notes. Fold the two defects and the one note named below; the rest are
settled (dispositions in
metasystem/records/misc/severity-tiered-rigor-build3-critique-cc1.md).

# The defects

F-1: the tier-1 evaluator takes the goal's tier only from the cited
root job record, a snapshot written at dispatch. The goal file at the
landing base is already parsed for the held-goal check but its Tier is
never compared, so a goal raised to tier 2 after its root was
dispatched still admits tier-1 landings. Fix: in the tier-one
evaluator (metasystem/internal/landing/tierone.go, or the held-goal
check it calls) compare the parsed goal file's Tier against 1 and
refuse with a code that names the goal's current tier; one fixture: a
goal edited to tier 2 after its tier-1 root refuses the landing.

F-2: `landing test-receipt` runs the caller's command under the generic
local execution bound (five minutes, hard kill of the process group),
which the full battery of a gateWidth-full root exceeds. Fix: a
dedicated bound for the receipt command, `landing.receipt-bound-min`
in metasystem.conf (validated; default 40), applied only to the
receipt command; measure the exact full battery string once on this
host through the verb and record the minutes in the return; the
default must exceed that measurement with margin.

# The note to fold

F-6: two proofs are narrower than the code: add a foreign-tree receipt
fixture at the expected path (so the in-file tree comparison is what
refuses), and a fixture where the working tree differs from the
supplied tree while the index matches, before the command runs.

# The notes, settled

F-3 (a hand-written receipt passes on content), F-4 (record paths
under tier-1 without append-only), F-5 (cmd/metasystem not on the
floor): recorded for the design owner, not changed in this round.
F-7 (the command package's timing test red): pre-existing on main.

# Gate

As in metasystem/plans/severity-tiered-rigor-build3-brief.md. Declare
the boundary as every file that differs from main.

# Constraints

Wall-clock budget: 60 minutes; return before it ends even if something
is red, naming it. Gap rule: stop and report a gap with your proposed
contract written out.
