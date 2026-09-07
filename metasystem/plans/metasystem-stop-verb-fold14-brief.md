Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 16: the two scenarios that cannot reach their own human half (chain stopverb-build1)

Round 15's three fixes all worked. On the round-15 tree, outside the
sandbox, the supervision bed is FULLY green including stop-fence, which
now drives the mission path as an agent, and every pre-existing scenario
passes but `rearm-launch-fails`, red on main and owned elsewhere. The
eleven-package matrix, the dispatch bed and the suite-progress bed are
green.

Two scenarios remain red, and they fail at the same point for the same
reason.

# Facts, from the orchestrator's runs

- wrong-terminal (goal bed): its refusal half now matches the grammar
  character for character, class `DELEGATE` and the resolved path. It
  then fails in its SECOND half, where the same command must SUCCEED
  under fixture human authority: the output file is empty and the bed
  prints the refusal again, `stop is a human act at a terminal; this
  caller is DELEGATE`.
- mission-stop (mission bed): identical, and it never reaches its three
  cases. Its stop call is refused with the same DELEGATE sentence.
- Both beds run under the orchestrator's own delegate session, so their
  ambient class IS delegate. The supervision bed's scenarios call the
  same verb successfully, so a working mechanism exists in this tree:
  they present the classifier an agent-free ancestry through the bed's
  own identity fixtures rather than borrowing ambient class, which the
  comment above `become_main` in
  metasystem/scripts/agents/supervision-fixtures.sh states in as many
  words: ambient class is not stable, and a fixture that drives the
  control plane must BE a main rather than borrow what its ancestry
  happens to produce.

# Decisions (the orchestrator's; decided, not open)

D60. wrong-terminal's success half and every stop, status and resume call
in mission-stop present the classifier an agent-free ancestry the way the
supervision bed's scenarios do, using each bed's own identity fixture,
rather than relying on ambient class. The refusal halves keep their
delegate ancestry: proving both classes is the point of wrong-terminal.

D61. If, having done that, the gate still refuses a fixture-granted human
classification, STOP and name it as a finding rather than working around
it. Section 7 of the design says a fixture-granted HUMAN classification
is admitted as it is for arm, and the gate's own comment in
cmd/metasystem/process_verbs.go (a file this chain created, cited without
the tree prefix because admission judges prefixed paths against the base
commit) says fixture-granted classifications are explicit authority. If that is not true in the code,
it is a product defect and this round's job is to report it precisely,
not to hide it.

D62. Nothing else changes.

# Verification

Reported at evidence level ran: `scripts/agents/go-gate.sh --fast` and
`go test -count=1` over the boundary, with the private caches earlier
rounds used. Name what you expect from the goal and mission beds; the
orchestrator runs them outside your sandbox and reports.

# Constraints

Wall-clock budget: 90 minutes. Two of the goal's twenty attempts remain
after this round, one of which is reserved for the closing critique, so
this is the last fixture round: if something does not fit, name it
precisely and stop. Return per the implementer schema with the
cumulative diff boundary listed. Gap rule: stop and report a gap; never
fill it silently.
