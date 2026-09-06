Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-06

# Goal

Goal metasystem-stop-verb (tier 3, approved by Wido). Its record,
metasystem/plans/goals/metasystem-stop-verb.md, is the contract: one
intuitive word stops everything the metasystem runs for a checkout, in
order, printing one line per thing and one line saying how to start
again; a second stop says nothing is running; status prints the same
list without acting. The specification is
metasystem/plans/metasystem-stop-verb-design.md, revision 3 with its
section 13 addendum, closed by three critique rounds (registers
metasystem/records/misc/metasystem-stop-critique-r1.md,
metasystem/records/misc/metasystem-stop-critique-r2.md and
metasystem/records/misc/metasystem-stop-critique-r3.md). Section 13
folds the closing critique as build decisions and wins over any earlier
section it contradicts. This is SLICE 1 (section 13.6): the per-checkout
vertical; the fleet form `--all` is refused with the sentence section
13.3 names and is built in slice 2.

# Facts

- The design's tree references were read by seam name on 2026-09-06.
  Build to the seam by name; a moved line is not a gap, a seam that
  changed shape or vanished is.
- Every decision is in the design: the verbs and grammar (1), the fence,
  its lock, the transition owner and the generation handshake (2), the
  inventory (3), the order of stopping (4), the seat's own session (5),
  the report and the line grammar (6), authority and the headless
  handover (7), the fleet form with the withdrawn directory-gone promise
  (8), the refusals (9), the fixtures and seams (10), agnosticism (11)
  and scope (12). Do not re-decide them; where the tree contradicts the
  design, stop and report the gap with the seam named.
- The six round-3 findings are folded in section 13 (creation claims as
  the completion barrier, the owner-published teardown ceiling, orphaned
  turns inventoried on their own, the proof-run record, the withdrawn
  directory-gone probe, the slicing). None is carried open.

# Decisions (the orchestrator's; decided, not open)

D1. Implement slice 1 of revision 3 as written, section 13 included: the
two new packages `internal/stopfence` (record, lock, reader, creation
claims) and `internal/stoptransition`, the command file
`cmd/metasystem/process_verbs.go` with the dispatch and usage lines in
`cmd/metasystem/main.go`, the `stopfence creating-close` verb the shell
seam needs, the proof-run record, and the family changes section 12
lists minus the fleet form.

D2. Boundary: exactly section 12. Existing: metasystem/cmd/metasystem/main.go,
metasystem/internal/up, metasystem/internal/supervise, metasystem/internal/steward,
metasystem/internal/dispatch, metasystem/internal/run, metasystem/internal/proofrun,
metasystem/internal/missionrunner, metasystem/internal/goal,
metasystem/scripts/agents/dispatch.sh, metasystem/scripts/agents/supervision-hook.sh,
metasystem/scripts/agents/adapters/fake.sh, and the fixture scripts section
10 names (metasystem/scripts/agents/supervision-fixtures.sh,
metasystem/scripts/agents/mission-fixtures.sh,
metasystem/scripts/agents/suite-progress-fixtures.sh,
metasystem/scripts/agents/dispatch-fixtures.sh,
metasystem/scripts/agents/goal-cli-fixtures.sh). New: the two packages
with their tests and `cmd/metasystem/process_verbs.go`. Anything beyond
this list is a gap, not a change. The design's own reject list holds: no
creator holds the transition lock through a process start, no signal is
sent on a command-line shape without a record identity, no registry event
outside the reducer's contract.

D3. Source comments describe the application, never this round, the
critique, or a finding id.

D4. Fixture-only seams are refused outside a fixture-mode root, the shape
of the existing crash-on-start seam.

# Verification

Required, run from the worktree and reported at evidence level ran:
`scripts/agents/go-gate.sh --fast`, then `scripts/agents/dispatch-fixtures.sh`,
then `scripts/agents/goal-cli-fixtures.sh`, then the supervision, mission
and suite-progress fixture beds section 10 joins. Every scenario and
package test section 10 names exists and passes; state in the return
which of them fail against the tree before your change. The orchestrator
runs `metasystem stop`, `status` and `arm` live on this Mac after landing;
do not claim the live proof from the sandbox.

# Constraints

Wall-clock budget: 120 minutes. If the slice does not fit one round, stop at a green gate with the families done so far listed and the rest named as a gap; a follow-up round continues. Return per the implementer schema with
the diff boundary listed. Gap rule: stop and report a gap; never fill it
silently.
