Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal landing-receipt-survives-records-drift)
Date: 2026-09-09

# Build brief: a landing receipt that survives a register append

## The specification, and how to read it

`metasystem/plans/landing-receipt-survives-records-drift-design.md`,
revision 3, landed at 4b0b777a (sha256 5145d7014c44ddc5d15d822bc5ceb5c80103b04296c65b4d08e5f10b8eb44031), is the whole contract; its
critique ladder is closed after two reads
(`metasystem/records/misc/landing-receipt-survives-records-drift-critique-r1.md`,
`-r2.md`). Where this brief and the page disagree, the page wins. Read the
page's "Implementation map" first, then Decisions 3 and 4 and "Fixtures".

You build steps 1 through 9 of the implementation map, in that order, each
step leaving the fast gate green. Step 10, the crossover landing itself, is
the orchestrator's and is not yours.

## What you build, by the page's sections

1. `metasystem/internal/gittree/snapshotscope.go`: `Workspace.Status()`
   and `StatusEntry` (3a), `IsAncestor`, `ResetKeep` and
   `ResetKeepResult`; `metasystem/internal/gittree/detached.go`:
   `NewDetachedCommitWorktree`, `Rebase` and `RebaseResult` — all on
   `gitProbe`, signatures exactly as printed in 3e under "The gittree
   operations advance needs". Package `landing` never imports the standard
   library's exec package.
2. `metasystem/internal/lease/lease.go`: `LockPath`, one exported line
   returning the existing lock path. The lock's refusal text does not change.
3. Decision 1's register set in this new file: `metasystem/internal/landing/registers.go`
   and the switch
   at `metasystem/internal/landing/observe.go:779` reading it.
4. `metasystem/internal/landing/receipt.go`: `receiptPosture`,
   `receiptIdentity`, schema 2, the four reader rules of Decision 4
   including the version-1 compatibility read and both mixed-form refusals.
5. The drift verb (3a, both per-mode rule tables exactly as printed) in
   this new file: `metasystem/internal/landing/drift.go` and the `landing drift` verb in
   `metasystem/cmd/metasystem/landing_verbs.go` with its registry line in
   `metasystem/cmd/metasystem/main.go`.
6. The advance verb (3e: lock, steps A-E, the compare-then-swap, the
   refusal type, every refusal code) in this new file: `metasystem/internal/landing/advance.go` and the
   `landing advance` verb; the eight hand-written rows in
   `metasystem/internal/refusal/register.go` with the sites in
   `advance.go` as landed.
7. `metasystem/internal/behaviorsurface/policy.v2.json` (3c), the row in
   `metasystem/internal/behaviorsurface/policy_test.go`,
   `metasystem/internal/behaviorsurface/consumer_wiring_test.go:84`, the
   three case lists in `metasystem/scripts/agents/static-reproof-fixtures.sh`.
8. `metasystem/scripts/agents/land.sh`: the three edits of 3b, nothing else.
9. `metasystem/scripts/agents/land-fixtures.sh`: the seed, the second
   candidate and its receipt, and the three shell canaries of "Fixtures",
   written exactly as specified. You cannot run this bed in your sandbox
   (see below); write it to the page and stop.

## Canaries first, and red on the untouched tree

The Go passing canary `TestReadTestReceiptSurvivesRegisterAppendAfterReceipt`
is written first and run on the UNTOUCHED tree; it must fail with "the index
or working tree moved after the test receipt was created". Record the
observed text as evidence at level `ran`. A canary that passes before the
repair is a defect in the canary: stop and report it. The refusal canary
`TestReadTestReceiptRefusesNonRegisterDrift` must pass before and after.
Then the proving run the page names:

```sh
cd metasystem && go test ./internal/landing/ -run 'TestCreateTestReceipt|TestReadTestReceipt|TestWorktreeDrift|TestAdvance|TestAppendOnlyRegisters' -count=1 -timeout=3m
```

plus `go test ./internal/gittree/ ./internal/lease/ ./internal/behaviorsurface/ ./internal/refusal/ -count=1 -timeout=3m`
and the fast gate `scripts/agents/go-gate.sh --fast`.

Your sandbox has no network egress and cannot write under the home
directory. The previous job on this goal made the fast gate pass by pointing
Go at the local module cache (`GOPROXY=file://` at the module cache) and
giving staticcheck a writable cache directory under the scratch root; do
the same rather than reporting the gate as unrunnable. The shell beds
(`land-fixtures.sh`, `dispatch-fixtures.sh`, `goal-cli-fixtures.sh`)
create repositories the sandbox forbids; the orchestrator runs them on your
tree. Do not weaken anything to make them runnable, and do not treat their
absence as evidence.

## Boundary — the implementation map is the wall

Your `diffBoundary` is exactly the files the map's steps 1-9 name plus the
new files it creates (`registers.go`, `drift.go`, `advance.go`, their
tests). No goal record, no `AGENTS.md`, no other script. The receipt's
index tree and candidate tree stay exact; only the working-tree projection
is filtered. The append-only rule at `observe.go:779` keeps its behaviour.
If a step cannot be done without crossing this wall, stop and say which.

## Two standing rules

- No round, slice or finding references in source comments (no `LRD-`,
  `LRE-`, `revision 3`). Comments describe the application. The comment
  text the page prints for `concludedAt`-style helpers is fine as printed.
- Keep the page's exact strings: refusal codes, the drift verb's output
  grammar, land.sh's existing messages, the receipt's field names.

## Return

Per the implementer schema. Evidence rows at level `ran` for: the passing
canary's observed fail-before text; the proving run green; the package
runs; the fast gate. `riskiestPart` names what you are least sure of;
the page itself names the union-order and crossover claims it did not
probe. Gap rule: stop and report a gap; never fill it silently. Wall-clock
budget: 120 minutes.
