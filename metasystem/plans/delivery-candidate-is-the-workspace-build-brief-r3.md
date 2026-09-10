Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal delivery-candidate-is-the-workspace-not-the-ledger)
Date: 2026-09-10

# Round 3 of the same chain: the five land legs, with the seed the tree allows

Round 2 landed the harness (ceiling, budget, process group) and the new
bed, all green, and stopped before the five land legs on two gaps in the
page's section 8: the first-transition seed cannot mint a receipt under
the protected-policy corpus, and the cutover control cannot isolate one
field because the old engine's strict decoder rejects every field this
chain and the engine-bed chain added. Both gaps are real; both are
answered below by recipes that already exist in the tree. This round
builds the five legs on those answers. Keep every byte of rounds 1 and 2
unless a leg proves it wrong; if one does, change the least you can and
say so under `deviations`.

# Decisions (the orchestrator's; decided, not open). They amend section 8.1 and the cutover leg of 8.2; the page gets a coordinator note saying so.

D1. **The seed carries the contract on `main`; no first transition.** The
leg's seed commits `metasystem.conf` with `testing.contract=testing.json`,
`metasystem.runtimes=fake`, `dispatch.cap-min=1`, `dispatch.cap-max=120`,
and a `testing.json` with two `sh -c` command groups producing junit
reports, one of them declaring `inputs: ["payload.txt", "scripts/**"]`,
built the way metasystem/cmd/metasystem/landing_verbs_test.go:1340-1400
builds its contract (copy that contract's shape; it is the landed recipe
for a contract-bearing fixture repository). Every clone sets
`metasystem.steward.landing-ref refs/remotes/origin/main` in its local
git config (`:1355`). The fx goal is opened, approved and claimed with the
bed's existing verbs (metasystem/scripts/agents/land-fixtures.sh:207-223).
With a contract on the policy base, `RequireFirstTransition` never runs.

D2. **The engine in the seed is the candidate's own build, stamped with the
seed's first commit.** Commit the seed once WITHOUT `bin/metasystem`
(H0). Build the engine from this worktree's source with
`METASYSTEM_BUILD_STAMP=$H0 bash "$root/scripts/agents/go-build.sh" --out "$leg_root/engine"`
(the override at metasystem/scripts/agents/go-build.sh:57-58 skips the
dirty-tree stamp; `--out` leaves `bin/metasystem` alone). Copy it to the
seed's `bin/metasystem`, commit again (H1), push, clone. Source
authentication then resolves stamp H0 as a landed ancestor of the
landing ref (metasystem/internal/steward/rearm_resolver.go:224-250), and
the ENGINE projection of H0 equals H1's (the seed's `bin/` is not an
engine path).

D3. **Clone one is enrolled with `steward arm`, the way the supervision bed
arms a scratch checkout.** `metasystem.runtimes=fake` makes the clone a
fixture root (metasystem/internal/fixtureauth/fixtureauth.go:291), so
`"$leg_local/bin/metasystem" steward arm --repo "$leg_local"` mints the
fixture enrollment (metasystem/internal/steward/runner.go:654-656;
metasystem/scripts/agents/supervision-fixtures.sh:1077 is the model).
Give it the fake process-identity table the Go recipe and that bed give
(`METASYSTEM_FAKE_PROCESS_IDENTITY_FILE` holding
`{"<pid>":{"terminal":true}}`, landing_verbs_test.go:1457-1460 and
supervision-fixtures.sh:1055) and the receipt canary's minimal
environment (landing_verbs_test.go:1576-1594). Stop the runner with
`"$leg_local/bin/metasystem" stop --repo "$leg_local"` before the leg
returns and assert nothing it started survives; a leg that leaves a
runner is red.

D4. **`landing test-receipt --mode auto` in clone one then takes a
schema-2 receipt carrying `workspace`** through the enrolled engine, the
production path (`trustedPolicyEngine`'s enrolled branch,
metasystem/cmd/metasystem/test.go around `steward.OpenEnrolledBinary`).
That is the receipt the legs land, refuse, and cut over on, exactly as
8.2 lists them.

D5. **The cutover leg's old engine is built from 6bc19ba1c**, the last
`main` before this chain (it includes the engine-bed landing 016b83e2f),
not from 2fbd77535: `git -C "$root" archive 6bc19ba1c metasystem/` into
`$leg_root/old-src`, then `METASYSTEM_BUILD_STAMP=$H0 bash old-src/metasystem/scripts/agents/go-build.sh --out "$leg_root/old-engine"`
(`git archive` needs the seat repository's object store; the worktree
shares it). That engine is installed as the cutover leg's clone-one
`bin/metasystem` and enrolled as in D3. **The control strips exactly the
fields this chain added and nothing else**: the receipt's `workspace`,
the receipt's `candidateEngineBuildIdentity`, and
`testing.candidateEngineBuildIdentity`. The old engine then decodes the
control (its decoder knows every remaining field), refuses the unstripped
receipt with `chain-test-receipt-refused`, and the three proofs of 8.2
stand as written: an old receipt lands on an unmoved tip; the same
landing refuses at site 8's first sentence after the ledger move; the
field-bearing receipt is refused and the stripped control observed.

D6. The four scenarios are `ledger-move-lands` (with the drift checks
inside it), `records-move-lands`, `input-move-refuses`, `receipt-cutover`;
land-fixtures' success line moves from 9 to 13 legs. Each scenario builds
one engine (the cutover leg two); state the build in the leg header as
its cost.

# Proof before you return

- `bash scripts/agents/land-fixtures.sh` green with 13 legs.
- `bash scripts/agents/fixture-bed-scenarios-fixtures.sh` still green.
- `bash scripts/agents/go-gate.sh --fast` green.
- After every leg, no process started by a leg survives (the harness's
  reap plus your explicit `stop --repo`); say how you checked.

# Return

`diffBoundary` and `files` are repository-root paths (`metasystem/...`).
List under `evidence` every command above with its observed result, and
under `deviations` every departure from D1 to D6 with the line. If a step
of D3 is denied by the sandbox (process enumeration), say exactly which
call and what was denied and stop there. Wall-clock expectation: 90
minutes.
