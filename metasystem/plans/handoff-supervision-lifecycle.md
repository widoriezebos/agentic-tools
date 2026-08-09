# supervision-lifecycle

- Owner: Claude Fable orchestrator session of 2026-08-09 (handing off);
  main checkout, plan docs uncommitted on `main`.
- Goal and current status: the design that gives the supervision owner
  a lifetime (KI-32 incident) is in a sol critique loop nearing
  convergence. Material counts: rounds 1-3 found 13, 10, 13; after
  the independent Fable review and rewrite, rounds 4-8 found 14, 18,
  10, 9, 6 — every finding accepted and folded same-day, dispositions
  under `plans/dispositions/supervision-lifecycle-round-*.md`. No
  reframe has survived since round 4; rounds 7-8 were crash recovery
  of the recovery machinery and platform resolution limits.
- In flight right now: nothing in this checkout — CRITIQUE ROUND 11
  runs as job `supervision-lifecycle-r12` (codex `gpt-5.6-sol`,
  dispatched 2026-08-09 ~20:50Z; job ids run one ahead of critique
  rounds since the r10 id was burned by a payload collision —
  briefs must live OUTSIDE `artifacts/agents/<job-id>/`) in the
  isolated worktree
  `/Users/wido/LocalStorage/GitHub/agentic-tools-slc-r4`, whose job
  records the main-checkout scanner cannot see (KI-34). Rounds 9
  and 10 are adjudicated and folded (dispositions round-9,
  round-10). The lease note below is superseded: the prior holder
  died and this session (pid 90299) took over the worktree lease at
  epoch 3, joining the live supervision owner. The orchestrator
  session holds a tracked `--wait` on the running job. The
  UNATTENDED-RUN mission and its rulings:
  `plans/unattended-run-20260809.md`.
- Decisions made (and who made them):
  - The human (2026-08-09): apply all critique findings to the
    design; loop sol until BOTH sol and the orchestrator agree no
    material issues block implementation; implement after that.
  - Standing rules (the human, earlier): stay on Fable for all
    claude-side work; design critic is codex `gpt-5.6-sol`, model
    named EXPLICITLY in every dispatch; no untracked background
    processes — dispatch.sh or tracked background tasks only.
  - Design decisions are recorded in the two documents; D-6's numbers
    are settled. Do not re-litigate: dead-man's switch (dropped),
    derived identity (dropped), custody scope (same-lifetime
    provisioners only), the ACCEPTED consequences named in D-1/D-2,
    REG-6's stated kill residual.
- Waiting on the human: nothing.
- Dead ends (do not retry without new evidence):
  - Deriving checkout identity from path/fingerprint/inode
    (SLC-R3-001/002: the fingerprint hashes supervision code+config,
    not the checkout).
  - Deciding owner currency from state-file CONTENT (SLC-R4-001).
  - A dead-man's switch / renewal lease (SLC-R1-007..009).
  - Process custody for cohorts (SLC-R4-008: their lifecycle is
    multi-invocation by design).
  - Dispatching design critique from the MAIN checkout while another
    session holds its lease — that refusal is what created the
    worktree (see below).

## The loop, mechanically (repeat until CONVERGED)

1. Read the round's `return.json`. Verify EVERY finding against the
   committed code and the documents before accepting — sol's findings
   have all been real so far, but verification is the job.
2. Fold accepted findings into `plans/supervision-lifecycle.md` and
   `plans/supervision-registry.md` with a bias to SIMPLIFY (several
   rounds closed by deleting mechanism: the custody-bound event, the
   tombstone rule, the ownerless-lock window). Update the Proof list
   and the critique record; write
   `plans/dispositions/supervision-lifecycle-round-N.md` in the
   existing table format.
3. Copy the changed plan files into the worktree
   (`cp metasystem/plans/{supervision-lifecycle,supervision-registry}.md
   <worktree>/metasystem/plans/` plus the new dispositions file).
   Canonical copies live in the MAIN checkout; the worktree copies
   exist only for the critic.
4. Derive the next brief from
   `<worktree>/metasystem/artifacts/agents/supervision-lifecycle-r9/brief.md`:
   bump the round number, refresh the chain history and the
   fold-summary paragraph, keep the fold-failure rule, the material
   bar ("blocks implementation, not could-be-more-precise"), and the
   explicit CONVERGED / NOT-CONVERGED verdict line.
5. Dispatch, tracked, model named:
   `<worktree>/metasystem/scripts/agents/dispatch.sh dispatch
   --role design-critic --runtime codex --model gpt-5.6-sol
   --job-id supervision-lifecycle-rN --brief <file> --wait`
   (run as a background task so completion re-invokes the session).
6. CONVERGED means: sol's verdict line says so AND your own
   verification agrees. Record it in the design header. Then tear
   down the worktree: run `metasystem/scripts/agents/arm-supervision.sh
   --repo <worktree> --shutdown` from inside it, then
   `git worktree remove` it and delete branch
   `session/agentic-tools-slc-r4`.

Worktree background: the main checkout's lease was held by another
live session (claude pid 90299, started Aug 5), so dispatch refused
OWNED-ELSEWHERE and `second-session.sh` created the isolated writer.
If that session is gone when you start, you can dispatch from the
main checkout instead; if you need a fresh worktree,
`second-session.sh` is the paved path — remember to copy
`metasystem.conf.local` in (the isolation script does not) and run
`scripts/agents/adapters/codex.sh probe` once there.

## Implementation (after convergence)

Follow the design's "Implementation order" exactly: registry
contract → D-1 exits and teardown → D-2 breaker and ceiling → D-4
gate + janitor with D-3 custody/ledger → D-5 loudness.

Three items are NAMED implementation items because the shipped code
violates the design today:
- the owner EXIT trap kills whatever the live state file names —
  replace with held-identity teardown (SLC-F-001,
  arm-supervision.sh:358-366, 317-324);
- teardown and terminal appends must not depend on checkout-resident
  helpers like process-census.py — held identities and system
  binaries only (SLC-R7-005);
- `run-cohort.sh` has NO teardown today — the ledger + entry
  recovery is NEW driver behavior, not wiring (SLC-R4-009).

The Proof section is the acceptance list; extend
`scripts/agents/supervision-fixtures.sh` to cover it. Never weaken a
Proof to make a test pass — if implementation exposes a design
defect, fold it and send the design back to sol for one more round.
Code critique per roster: role.code-critic is claude
`claude-fable-5` (`metasystem.conf.local`). The main checkout carries
unrelated uncommitted delegate-caps changes by another agent — do
not commit or touch them. Until D-4 lands, the operational
workaround stands: kill owners before components.

- Next step: none
  (when round 9's return lands: adjudicate it, fold or converge, and
  proceed per the loop above — the waiting orchestrator session is
  armed on the return file.)
