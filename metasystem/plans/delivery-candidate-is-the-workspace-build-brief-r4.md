Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal delivery-candidate-is-the-workspace-not-the-ledger)
Date: 2026-09-10

# Round 4 of the same chain: the five land legs, D5 corrected

Round 3 built nothing: it found that decision D5's control cannot pass
(round 1 raised `candidateEngineIdentityVersion` to 2 and the old engine
at 6bc19ba1c accepts only 0 or 1, so stripping the three named fields is
not enough) and stopped before building anything else. The finding is
right; the stop was too wide. This round builds all five legs. Decisions
D1, D2, D3, D4 and D6 of metasystem/artifacts/agents/dcwl-context/build-brief-r3.md (round 3's brief; it lands under plans/ with the chain)
stand unchanged. D5 is replaced by the D5 below. If any other decision
meets a contradiction in the tree, build the legs that do not depend on
it, record the contradiction under `deviations` with the line, and keep
going; a round that returns with three green legs and one named
contradiction is worth more than a round that returns with none.

D5 (replacing round 3's). **The cutover control is the old engine's own
receipt, not a stripped copy of the new one.** In the `receipt-cutover`
leg the old engine (built from 6bc19ba1c as round 3's D5 says, stamped
with the seed's first commit, installed as clone one's `bin/metasystem`,
enrolled as in D3) takes its own schema-2 receipt for the staged
candidate with `landing test-receipt --root . --tree $(git write-tree)
--mode auto --goal fx --cap-min 1`; that receipt carries no `workspace`
and no build identity, and `landing observe --test-receipt` by the old
engine observes it (`mode` `observe`). That is proof 1's receipt and it
is the control. For proof 3, the candidate's engine (the one round 1
built, by explicit path) takes its receipt for the SAME tree in the same
clone, which overwrites the file at the same path with a receipt carrying
`workspace`, `candidateEngineBuildIdentity` and identity version 2; the
old engine's `landing observe --test-receipt` on that file prints
`verdictTrailer` `would-refuse code=chain-test-receipt-refused`. Same
clone, same tree, same path, two engines' receipts: the difference is
the receipt's shape and nothing else, which is what the leg exists to
prove. No field is stripped by hand. Proof 2 (the old receipt refuses at
site 8's first sentence after the peer's ledger move, with no
unknown-verb text in `land.out`) stands as written.

Everything else in round 3's brief stands: the seed with the contract on
`main` (D1), the engine stamped with the seed's first commit through
`METASYSTEM_BUILD_STAMP` and `go-build.sh --out` (D2), enrollment of
clone one with `steward arm` in fixture mode and its explicit `stop
--repo` before the leg returns (D3), the enrolled-engine receipt path
(D4), the four scenarios and the leg count of 13 (D6).

# Proof before you return

- `bash scripts/agents/land-fixtures.sh` green with 13 legs.
- `bash scripts/agents/fixture-bed-scenarios-fixtures.sh` still green.
- `bash scripts/agents/go-gate.sh --fast` green.
- After every leg, no process started by a leg survives; say how you
  checked.

# Return

`diffBoundary` and `files` are repository-root paths (`metasystem/...`).
List under `evidence` every command above with its observed result, and
under `deviations` every departure from the decisions with the line. If a
step of D3 is denied by the sandbox (process enumeration), say exactly
which call and what was denied, keep the legs that do not need it, and
report the rest. Wall-clock expectation: 90 minutes.
