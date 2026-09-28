# Build brief: evidence-root-default, step 1 (U1 to U6)

- **Design (the spec):** `metasystem/plans/evidence-root-default-design.md`, revision 4
  (939e73962). Read all of it, including Decided and the Dispositions of Astra's rounds 1
  and 2. Every fold is a rule the code must honour.
- **Workspace:** the worktree `/Users/wido/LocalStorage/GitHub/agentic-tools-erd`, branch
  `evidence-root-default`. Run every Go command from its `metasystem/` directory.
- **Sequencing override (Wido, 2026-09-28): "I want to finish the evidence root first,
  landing asap now".** Do NOT wait for m1e's `embed` or `U9b`. Build against this tree as
  it stands. The Sequencing section's wait is lifted, but its notes on where the
  branches overlap still describe what a later rebase will meet, so keep the stateroot
  and dispatch/delegation hunks minimal.
- **In scope:** units U1 to U6, in order, one commit per unit (more is fine).
  `git commit --no-verify`, staging nothing under `metasystem/plans/goals/`.
- **Out of scope for you:** the migration's last act, dropping the hand-set
  `evidence.root` line from other checkouts' `.local` files and writing `RETIRED.json`
  in the old roots. The coordinator does that on this host after landing. Do build and
  test `WriteRetired` and `metasystem internal evidence retire` (U6), but never run them
  against a real root under `/Users/wido/metasystem-evidence`.
- **Tests:** behaviour tests run with real Git unavailable. Use the design's seams
  (`LookupEnv` answering `HOME`, host fakes). No `t.Setenv`: the parallel ratchet forbids
  new serial tests. Write each test first and watch it fail.
- **Proof, before you return:**
  1. `go run ./cmd/devgate static` passes. It is slow (about 10 minutes); run it once at
     the end, and again only after a fix.
  2. `go test` passes for every package you touched, plus `./internal/refusal/`,
     `./internal/audit/`, `./internal/config/`, `./internal/seat/launch/` and
     `./cmd/metasystem/ -run 'Launch|Machine|Seat|Adopt|Config|Evidence|App|Audit'`.
  3. For every refusal-register row you re-pin, set it wrong once, watch
     `TestHCL03EveryRowedSiteNamesAnEmission` fail, then restore it.
  4. `METASYSTEM_TESTING_WORKERS=9` is this host's worker count.
- **Return:** write the report to `metasystem/plans/evidence-root-default-build-report.md`
  in the worktree and commit it. It covers, per unit: what was built, the tests (named),
  any deviation from the design and why, and the exact proof commands with their
  results. Print the report as your answer, with the branch's last commit.
