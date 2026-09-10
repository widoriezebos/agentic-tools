Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal delivery-candidate-is-the-workspace-not-the-ledger)
Date: 2026-09-10

# Round 2 of the same chain: the beds

Round 1 (this chain's first round) built everything in the design outside
section 8's shell work: the projection, the receipt field, the observer's
step 0, the verify core, `landing workspace`, land.sh site 8, the engine
build identity and its lookup, the refusal row, the docs sentence, and
their unit tests; the fast gate and the nine existing land legs were green.
Keep every byte of that round unless a bed proves it wrong; if one does,
change the least you can and say so under `deviations`.

This round is section 8 of
metasystem/plans/delivery-candidate-is-the-workspace-design.md (revision
3, sha256 1622cf7cfc4fd10259e23eabb0fdae2d1ba7a59a67666cea679f2b951bcca83b):

1. **8.3, the harness.** metasystem/scripts/agents/fixture-bed-scenarios.sh
   and metasystem/scripts/agents/fixture-budget.sh: the `bed-scenario` cap
   with its 1-through-120 override, the poll in place of the plain `wait`,
   the self-initialised budget (idempotent; inherited when set), and the
   process-group run and reap (`set -m` around the `&`, stdin from
   `/dev/null`, `fixture_bed_reap_group`: TERM the group, five-second
   grace, KILL, wait the child, verify the group is empty; the parent's
   traps call the same function). The log lines are the page's.
2. **8.3, the new bed.** Write this new file: scripts/agents/fixture-bed-scenarios-fixtures.sh (under metasystem/),
   with the four scenarios `budget-standalone`, `budget-inherited`,
   `ceiling-reaps-group`, `signal-reaps-group`, exactly as the page
   describes them; registered as `section/fixture-bed-scenarios-fixtures`
   in metasystem/testing.json beside `section/land-fixtures`, with a
   surface over the two harness scripts and the bed the way
   `land-fixture` selects its bed; run by metasystem/scripts/validate-metasystem.sh
   through `run_section` beside land-fixtures, with the syntax check and
   the fixture-script list entries.
3. **8.1 and 8.2, the land legs.** metasystem/scripts/agents/land-fixtures.sh:
   the contract-bearing seed of 8.1, the peer clone's ledger move, and the
   five legs of 8.2 (`ledger-move-lands`, `records-move-lands`,
   `input-move-refuses`, `receipt-cutover` with the real older engine built
   once from a `git archive` of 2fbd77535 through `go-build.sh --out` with
   `METASYSTEM_BUILD_STAMP=cutover-2fbd77535`, and the drift checks inside
   `ledger-move-lands`), with the success line's leg count moved to the
   page's number.

# Facts for this round

- Your worktree carries round 1 staged. The other beds that source the
  harness are brain, mission, second-session, return-schema and
  witness-gate (`grep -l fixture-bed-scenarios metasystem/scripts/agents/*.sh`);
  mission-fixtures.sh:17 calls `harness_fixture_budget_init` itself and
  must keep working; the others call nothing and must start working
  standalone through the harness's own initialisation.
- `setsid` is absent on this host (macOS); bash 3.2's job control is the
  page's chosen mechanism. Probe before you rely on a behaviour, as the
  page did.
- The cutover leg's older engine is built from a commit that predates the
  candidate-engine landing 016b83e2f as well as this chain; that is
  intended: it is the engine a seat runs before either lands. Its
  `landing test-receipt --mode auto` writes neither `workspace` nor a
  candidate engine digest, which is the shape the leg needs.
- The sandbox denies process enumeration (`sysctl`), which is why round 1
  reported two dispatch-fixture scenarios and one Go test red; the
  orchestrator reruns those outside the sandbox. Do not try to work
  around the sandbox; report what it blocks.

# Proof before you return

- the new bed (`bash scripts/agents/fixture-bed-scenarios-fixtures.sh`) green, four
  scenarios.
- `bash scripts/agents/land-fixtures.sh` green with the page's leg count.
- `bash scripts/agents/return-schema-fixtures.sh` green with no budget
  initialisation of its own, and `bash scripts/agents/mission-fixtures.sh`
  still green.
- `bash scripts/agents/go-gate.sh --fast` green (testing.json and the
  validate script are gate inputs).
- `go test ./cmd/metasystem/... -run 'Testing|Contract'` green (the
  contract pins on testing.json's group and surface lists).

# Return

`diffBoundary` and `files` are repository-root paths (`metasystem/...`).
List under `evidence` every command above with its observed result, and
under `deviations` every departure from the page with the line. Wall-clock
expectation: 100 minutes; at the cap return what is green and say what is
red and why.
