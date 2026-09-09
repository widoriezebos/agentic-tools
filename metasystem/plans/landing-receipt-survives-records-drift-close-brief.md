Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal landing-receipt-survives-records-drift)
Date: 2026-09-09

# Review brief: the read that closes chain lrsrd-build1 (final work round lrsrd-build1-r2, round 2 of chain lrsrd-build1)

FINDING IDS: chain-unique, LRF-01, LRF-02, ... never F-n.

## Why this read exists

Chain completion under DESIGN-BEARING reach requires a fresh-context critic
whose reviewed job is the final work round. This is that read. Read for
judgment, not for green: the tests the builder ran are listed at the end
and they passed; what is not proven is anything about the code's judgment.
This change touches the landing proof itself, the transport rebase, and a
lock every lease holder shares; a defect here wedges landings for every
seat, so assume there is something to find.

Specification: `metasystem/plans/landing-receipt-survives-records-drift-design.md`
revision 3 (4b0b777a), critique ladder closed at
`metasystem/records/misc/landing-receipt-survives-records-drift-critique-r2.md`.
Build contract: `metasystem/plans/landing-receipt-survives-records-drift-build-brief.md`.
The orchestrator persisted the diff and the reviewed tree at
`metasystem/artifacts/agents/lrsrd-build1/rounds/2/review.json`; take
`reviewedTree` from that record.

## Mandate, in order of consequence

1. **The receipt loses no proof (Decision 2, Decision 4).** Only the
   working-tree projection is filtered; the index tree and the candidate
   tree stay exact. Confirm from the code that a receipt for tree T is
   still unlandable against any index other than T, that the version-1
   compatibility read compares raw with raw and version 2 filtered with
   filtered with no crossing branch, and that both mixed-form refusals
   are real on a tree containing the registers.
2. **Advance never touches a register, and the swap is guarded (3e).**
   Confirm the verb opens, writes, moves or restores no register file on
   any path; that the checkout mutation lock is taken before precondition
   A and released after the post-reset HEAD check; that the compare (last
   HEAD read) and the swap (`reset --keep`) sit under it; that every
   refusal leaves the branch at L; and that a register in the overlap is
   the contended refusal with the bytes byte-identical.
3. **The drift verb's two rule tables (3a).** Walk the porcelain v1 states
   against the code for both modes; the tolerance requires the append
   shape against the INDEX blob; `--require-empty-index` treats every
   non-blank index column as staged before the register exception.
4. **The gittree operations.** All four go through the bounded probe;
   `RebaseResult.Conflicted` means the abort has run and no worktree is
   left; `ResetKeep` answers `Moved` false with the branch unmoved.
   Package landing imports no exec.
5. **The register rows and land.sh.** Eight hand-written rows present with
   sites that match `advance.go` as landed; land.sh changed at exactly the
   three sites of 3b and nowhere else; the existing messages verbatim.
6. **The canaries failed for the right reason.** The Go passing canary red
   on the untouched tree with the moved message, observed not derived; the
   refusal canary green before and after; the version-rule cases a-f.
7. **The wall.** The diff is exactly the implementation map's files plus
   the new ones; no goal record, no `AGENTS.md`; no finding references in
   source comments.

## Constraints

Wall-clock budget: 45 minutes. Return per the code-critic schema with
`reviewedTree` from the persisted round record. Gap rule: stop and report a
gap. Your sandbox cannot run the fixture beds; the orchestrator ran them
and the results are below. Do not weaken anything to make them runnable.

## Orchestrator runs

On the reviewed tree (1120f4df48806fddb7067ca753b0cd28386d62e2, base commit 4b0b777a, 23 files),
from this Mac, outside any delegate sandbox:

- The orchestrator ran the page's landing proving selection and then
  `go test` over gittree, lease, behaviorsurface, refusal and landing on the
  build's tree: all five green, including the lease package's
  `TestGroupOwnsTag`, which the builder's sandbox could not prove (process
  table enumeration denied there) and which passes here. `go build
  ./cmd/metasystem/` is clean.
- The builder's evidence, all at level ran: the passing canary red on the
  untouched tree for both registers with the exact text "the index or
  working tree moved after the test receipt was created"; the proving
  selection green; the fast gate green (formatting, vet, staticcheck, the
  refusal register, build); the static re-proof fixtures green with the new
  narrator cases; `landing drift --root .` emitting the specified
  tab-separated grammar; `git diff --check` clean; both shell scripts pass
  `bash -n`.
- Round 1 of this chain stopped on four page gaps after writing the two
  receipt canaries; the coordinator disposed them (revision 3.4) and round 2
  built the map. The canaries are unchanged from round 1.
- Process-owning beds, run by the orchestrator on the reviewed tree:
  RUNNING at the time of this dispatch (`land-fixtures.sh` with its three
  new canaries first, then `dispatch-fixtures.sh`, then
  `goal-cli-fixtures.sh`). Their result joins the register before
  certification. A failed bed folds the chain whatever this read finds; do
  not wait on them and infer nothing from their absence here.

So the acceptance is proven as far as Go can prove it and the gate is
clean. What is NOT proven is anything about the code's judgment, which is
what you are for. Two claims the page itself flags as unprobed deserve your
reading: git's union ordering in the contended-register repair, and the
schema crossover.
