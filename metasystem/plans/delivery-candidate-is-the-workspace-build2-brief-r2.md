Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal delivery-candidate-is-the-workspace-not-the-ledger)
Date: 2026-09-11

# Round 2 of this chain: the runner is ended with the verb that started it, and the cutover leg's old engine is enrolled at its own stamp

Round 1 carried the two green rounds of the first chain and built the
five land legs. Outside the sandbox the coordinator ran the bed on the
round-1 tree: the ledger, records and input legs meet their behavioural
assertions and then go red at the same step as in the sandbox, and it is
not the sandbox: `stop --repo` on a clone armed only through `steward
arm` reports "NOT STOPPED untracked inventory: process census failed:
supervision-state: supervision state is unavailable" (there is no
supervision owner in a clone that was never brought `up`), exits
non-zero, and the bed's `stop_receipt_runner` treats that as failure
although the runner is dead. So:

# Decision D3, amended: end the runner with `steward disarm`

The runner was started by `steward arm --repo`; end it with
`steward disarm --repo` (the steward family's "end the runner" verb,
metasystem/cmd/metasystem/main.go, the `steward` table), never with the
whole-checkout `stop`. `stop_receipt_runner` runs `steward disarm --repo
"$checkout"` through the same environment, then keeps its existing
five-second check that the recorded runner pid is gone and its survivor
message. A non-zero exit from `steward disarm` is a leg failure; the
untracked-inventory census of `stop` is no longer consulted.

One leg is also red on a real rule: `receipt-cutover`. The old engine (built
from 6bc19ba1c) predates the skew-rule landing 92b31f774, so its source
check still requires the enrollment's landed commit to EQUAL the stamp
(`git show 6bc19ba1c:metasystem/internal/steward/rearm_resolver.go`,
lines 271-272: "enrollment records landed source %q but the executable
stamp resolves to %q"). The bed stamps every engine with the seed's first
commit H0 and then commits the candidate engine as H1
(metasystem/scripts/agents/land-fixtures.sh, the `is_workspace_receipt_scenario`
block after the goal claim), so clone one's tip is H1 and the old engine,
stamped H0, refuses before it can take its receipt. The current engine
passes the same pair because the skew rule accepts an older stamp whose
ENGINE projection equals the tip's; the old engine cannot.

Keep every byte of round 1 unless this leg proves it wrong.

# Decision D5, third and final form (replaces round 4's D5 of the first chain)

In the `receipt-cutover` leg only:

1. **One commit, no engine in the tree.** The seed commits everything
   but `bin/metasystem` (that is H0) and pushes; no "install candidate
   engine" commit follows. `bin/` stays untracked in that leg's clones
   (the seed's `.gitignore` already ignores `bin/`; keep it so). The tip
   every clone sees is H0.
2. **Both engines are stamped H0 and installed untracked.** The old
   engine, built from `git archive 6bc19ba1c` with
   `METASYSTEM_BUILD_STAMP=$H0` through that tree's `go-build.sh --out`,
   is installed as clone one's `bin/metasystem` (`install_cutover_engine`
   as today). The candidate engine, built from this worktree with the
   same stamp, is installed the same way as the PEER clone's
   `bin/metasystem`. Enrol each clone with its own engine through
   `steward arm --repo` (`arm_receipt_runner` as today): for the old
   engine the enrollment's landed commit is H0 and its stamp is H0, so
   its exact rule is satisfied; for the candidate engine the same pair
   passes under the skew rule.
3. **Proof 1 and proof 2 are unchanged**: clone one, with the old engine,
   stages `payload.txt`, takes its own schema-2 receipt (no `workspace`,
   no build identity), `landing observe` reads `pass bar=a`; `land.sh`
   lands on the unmoved tip; after the peer's ledger move the same
   landing refuses at site 8's first sentence with no unknown-verb text.
4. **Proof 3 takes the field-bearing receipt in the peer clone.** The peer
   stages the identical `payload.txt` change so `git write-tree` names the
   same candidate tree as clone one's; the candidate engine takes its
   schema-2 receipt there (`landing test-receipt --mode auto --goal fx
   --cap-min 1`), which carries `workspace`, `candidateEngineBuildIdentity`
   and identity version 2. Copy that receipt file into clone one's
   `artifacts/agents/landing/receipts/` at the same file name (the path is
   the candidate tree). The old engine's `landing observe --test-receipt`
   on it prints `verdictTrailer` `would-refuse code=chain-test-receipt-refused`.
   The control is proof 1's receipt, observed `pass bar=a` by the same
   old engine at the same path a moment earlier; nothing is stripped by
   hand.
5. End both runners with `steward disarm --repo` before the leg returns,
   as the other legs now do.

# Proof before you return

- `bash scripts/agents/land-fixtures.sh` with 13 legs; report each leg's
  own outcome. If `steward disarm` needs process enumeration the sandbox
  denies, say so with the step's output; the coordinator reruns the bed
  outside the sandbox.
- `bash scripts/agents/go-gate.sh --fast` green.

# Return

`diffBoundary` and `files` as before (thirty paths plus nothing new
unless the leg needs a helper). List under `evidence` every command above
with its observed result, and under `deviations` every departure from D5
with the line. Wall-clock expectation: 60 minutes.
