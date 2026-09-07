Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 19: fold the closing critique (chain stopverb-build1)

The closing critique read the tree whose whole acceptance is green: five
fixture beds, all eight slice-1 scenarios, the eleven-package matrix. It
found three material items and one note the orchestrator accepted
anyway. Dispositions are in records/misc/metasystem-stop-critique-r5.md
and the design's new section 15 decides each; read section 15 first, it
wins over earlier sections. Four of the five decisions live in one file,
metasystem/internal/stoptransition/transition.go, and none of them
changes the acceptance the beds already prove.

# Decisions (the orchestrator's; decided, not open)

D69. Per 15.1: every printed `NOT STOPPED` reason writes a matching
`notStopped` entry, a family whose records could not be read included,
as an entry naming the family and the reason with no pid. The durable
phase comes from the same count as the closing line. Arm refuses over an
unreadable-family survivor, names the family, and points at a second
stop. Package tests: a family whose inventory read fails leaves a
record whose phase is stop-incomplete carrying that entry, and the next
arm refuses over it.

D70. Per 15.2: arm's second line over a creator of unknown liveness
names `metasystem stop --repo <toplevel>` with section 2's words. Audit
every branch of the refusal renderer for the same fault: no refusal may
name the command that just produced it. Test each branch.

D71. Per 15.3: the crashed-stop re-inventory records as survivors only
what stop would act on. Untracked processes and adopted-custody runs are
listed and never written to `notStopped`. Test with both kinds present.

D72. Per 15.4: a later pass that stops what an earlier pass could not
prints its outcome and removes the earlier survivor entry, so the exit
status and the record describe the end of the run. Test the retry that
succeeds and the retry that does not.

D73. Per 15.5: `run launch` reads the fence before its other gates.

D74. Nothing else. SVC5-05, the turn verdict's early answer, is recorded
as not actioned here and rides another goal; do not touch it.

# Verification

Reported at evidence level ran, with the private caches earlier rounds
used: `scripts/agents/go-gate.sh --fast`, and `go test -count=1 -timeout
30m` over the boundary (the goal package's suite runs close to the
default ten-minute ceiling, so pass the flag). Name what you expect from
the five beds; the orchestrator runs them outside your sandbox and
reports.

# Constraints

Wall-clock budget: 120 minutes. The goal has budget for this round and a
further critique. Return per the implementer schema with the cumulative
diff boundary listed. Gap rule: stop and report a gap; never fill it
silently.
