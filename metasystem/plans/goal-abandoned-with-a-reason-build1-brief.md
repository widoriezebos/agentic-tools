Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-09

# Goal

Goal goal-abandoned-with-a-reason (tier 3, approved by Wido). Its record,
`metasystem/plans/goals/goal-abandoned-with-a-reason.md`, is the contract;
the design is the specification:
`metasystem/plans/goal-abandoned-with-a-reason-design.md`, revision 4,
landed at 0f6ff57d, sha256 7056c055e8794ec32d1c0b88aca29c9394dc77dcfd0efee1c8e9a500f83f7000,
folded through three independent reads
(`metasystem/records/misc/goal-abandoned-with-a-reason-critique-r1.md`,
`metasystem/records/misc/goal-abandoned-with-a-reason-critique-r2.md`,
`metasystem/records/misc/goal-abandoned-with-a-reason-critique-r3.md`).
The design loop was closed by the coordinator under D81: the page is the
spec and its fixtures are the proof. In short: a goal can be recorded as
abandoned, with its reason, by a proven human act; it leaves the live set
the way done goals do, keeps its history and its reason, satisfies no
dependency, is never pruned, and can be reopened.

# This slice: the ledger side

The build is sliced. This is SLICE 1 and it is the ledger side of the
design: sections 2, 3, 4, 5, 7, 8, 9 and 10, plus the two paragraphs of
section 4a headed "The transition is atomic against this checkout's
dispatch" and "The compare half is refusal 7", which specify the lock and
the record-revision compare that section 4's refusals 5 and 7 use.

NOT in this slice, do not build them: section 6 (reopen from abandoned and
the `goal carry` verb; slice 2), the landing side of section 4a (the
observation's goal and revision binding, `landing held`, `commit.sh` and
`land.sh` changes, their refusal-register rows and `held.go`; slice 3),
section 11 (the doc amendment), and the page edit in section 12. Where a
symbol this slice needs is defined in an excluded section (for instance
rule 11's treatment of `carry` lines, which is validation and IS in this
slice, versus the `carry` verb, which is not), build the validation and
leave the verb. The recovery cases in section 9 for `abandon` and
`engine-floor` are in; the `reopen from=abandoned` case and `carry` are
slice 2.

# Facts

- Every line number on the page was read at cab73164 or 96c6098b and the
  page says which; the tree has moved by a handful of landings since.
  Build to the seam by name; a moved line is not a gap, a seam that changed
  shape or vanished is: stop and report it with the seam named.
- The page names the untouched-tree failure of every fixture. Write the
  fixture first, watch it fail where the page says, then build until it
  passes. A fixture that passes on the untouched tree is a defect in the
  fixture and is reported as such.
- The census command in section 3 (`rg` over every non-test `.Done` and
  `.DonePaths` reader) is run by you on the tree you build on, and its
  output is compared with the table in section 5. A reader the table does
  not name is reported in your return with its classification, not
  silently fixed and not silently skipped.
- `goal abandon` is human-only, with `set-priority`'s enrolled proof. Do
  not add a relayed, channel or agent path. The fixtures drive the verb
  with an in-process proof exactly as the `set-priority` tests do.
- The refusal register (`metasystem/internal/refusal/register.go`) gains
  rows only for refusal codes THIS slice emits. The fast gate runs it.

# Decisions (the orchestrator's; decided, not open)

D1. Implement the sections named above exactly as written: the state, the
`Abandoned` field and `AbandonRecord`, the two History keys, the third
map with `Archived` and `Exists`, `parseArchivedAt`, the `goal abandon`
verb with its eleven refusals in the page's order and its effects in one
transaction, the ordered compaction targets, the dependency handling
(`--carried`, `--waive`, `--also`), the lock for every goal in the set and
refusal 7's record-revision compare, every reader row in section 5 that
does not belong to the landing side (rows 13 excluded), prune, the twelve
validation rules of section 8, recovery and the command table of section 9
(minus `held` and `carry`), and section 10's `goal engine-floor` verb,
its root History line, its validation rule and the registry slot check
with the five consumed classes.
D2. The refusal texts are the page's, verbatim. Where the page gives the
sentence, the sentence is the contract.
D3. Journal intent for `abandon` and `engine-floor` is written as the page
says; recovery refuses to replay both.
D4. Nothing in section 15 ("What must not change") moves. `Done` keeps its
name, its meaning and its counts.

# Fixtures (the proof of this slice)

The Go tests the page places in the sections above, by their page names,
in the packages the page names: the `internal/goal` tests for depState,
the archive parse, rule 5, the eleven refusals and their order, the
dependents (uncovered, carried, waived, also), the lock and the
record-revision compare, the fleet floor, the engine-floor line, the
compaction, prune, the split prechecks, the reconcile edit row, the record
binding its event (rule 4) and the History keys by verb (rule 11,
including its `carry`-line clause, built as validation only); the
`turnverdict` idle test; the `reconcilemap`, `recover`, `counselor`,
`metrics` and `evidence` tests; the `internal/supervise` slot-class test.
And the shell scenario `abandoned-with-a-reason` in
`metasystem/scripts/agents/goal-cli-fixtures.sh`, added to its scenario
list, driving the specimen of section 12 end to end.

Not in this slice: every `observe_test.go` and `held_test.go` canary, the
reopen and stop-batch tests of section 6, and the `land-fixtures.sh`
scenario.

# Gate

`scripts/agents/go-gate.sh --fast && scripts/agents/dispatch-fixtures.sh && scripts/agents/goal-cli-fixtures.sh`.
If the sandbox denies the fixture beds their temp directories or the
network, run what it allows (build, vet, the Go tests, the register) and
report exactly which bed did not run and why; the orchestrator runs the
rest outside the sandbox before the landing.

# Return

Your return names: every file touched; every fixture, with where it failed
on the untouched tree and that it passes now; the census output against
the table; every seam that had moved and how you resolved it; anything the
page left you to guess (there should be nothing; if there is, name it
rather than guessing).
