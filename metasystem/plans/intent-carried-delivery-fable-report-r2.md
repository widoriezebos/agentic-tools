# Fable report r2 (final): intent-carried-delivery-design.md

Reviewed at 3848f71b960a24ca6cea5bba2d013e25062ca48b (branch
codex/intent-workflows-20260926, dirty tree; design, dispositions and briefs
uncommitted). Round 2 of 2. 14 tool calls, source only, nothing built or run
beyond read-only git. Criterion applied verbatim to step 1 as revised.

## Dispositions IC-C1..C5

All five joined. IC-C3's pushback is correct on source: `CarryWord`
(goal/verbs.go:4581-4588) holds Goal, History, Workspace, Past, Expires,
Supersedes; `HistoryLine` (file.go:446) has Carried but no code endpoint. Without
a retained base, Diff(project(B), C) after main moved A->B would re-impose C's
copy of every file B changed. The base/workspace tuple is necessary; the patch
and digest are not. One definitional gap: `branch.EndpointTip` (branch, line
65-80) fetches the remote's live main into a temporary ref at composition time,
so "endpoint commit" in the tuple means origin main AT COMPOSE TIME, not the
goal branch's recorded base. State that, since IC-C7 turns on it.

## Findings

### IC-C6  material, BLOCKING, contract-shape  The carried handoff refuses every code-changing goal for a missing RECEIPT line

Source: land.sh's carried path runs `stage_changes` then `run_receipt_line_step`
(land.sh:1071-1072, body 540-549) before any carry work. That step calls
`landing receipt-line --root --tree <staged> --goal G` (land.sh:505-535) and
`fail_step`s on refusal. The decision (landing/receiptline.go:93-183) has exactly
five exemptions: no receipt ledger in the base tree, no changed paths, exact
revert, receipt-only, records-only. There is no carried exemption and land.sh
passes no `--carried` to it. Otherwise, when any changed path classifies as code
(receiptline.go:291-328), it REFUSES `receipt-line-missing` unless the staged
tree appends a `RECEIPT` line naming goal G to `memory/receipts.log`
(registers.go:13). That ledger is tracked at HEAD and on origin/main (verified
with ls-files and ls-tree), so the exemption cannot fire here.

The design's patch is Diff(projected endpoint, projected candidate); the ledger
is a workspace exclusion, so the patch never touches it, and the page states "no
pending receipt in main" as a property. commit.sh writes no receipt row either
(commit.sh:14-244 stamps trailers only). The ordinary branch landing passes this
check only because `compose` writes a row via `appendReceiptRow` (land.go:475-491)
inside the candidate; CandidateOnly's projection strips it.

Failure at first use: stage succeeds, land.sh exits 2 at "receipt line for the
landing", the patch stays staged in main (cleanup retains stage), the verb returns
partial with `land G --using-exception ID`, and that continuation reproduces the
identical refusal every time. No fixture bounds it; the step-6 contract cannot
complete for a goal that changes code.

Tests: DIFFERENT (step 5 must stage a ledger row). Step 1 does NOT WORK without
it. Safe (a refusal), but dead.

Smallest amendment (step 5, after the patch applies, before releasing the lock):
"Append G's RECEIPT line to memory/receipts.log in PRIMARY with the command the
receipt-line decision itself names (`scripts/receipt.sh add --type implement
--outcome shipped --goal G ...`), note naming the exception CODE and opid; stage
the ledger; require `ObserveReceiptLine(PRIMARY, stagedTree, G)` to pass. On
replay append only when that check still fails. Never `appendReceiptRow`: its
`verify=clean|proof=` row is the fabricated ordinary proof this page forbids."
The projected workspace is unchanged (ledger excluded), so the word still
matches; `landing drift` already accepts append-only register growth. Rewrite
"no pending receipt in main" as "no PENDING placeholder row; the real RECEIPT
line is staged with the patch". Add to TestIntentCarriedGoalDeliveryGitAdapter:
"the receipt-line step passes on the real land.sh transaction."

### IC-C7  material, bounded fixture  The endpoint predicate is read on the wrong ref and its remedy loops when local main is behind

Source: composition fetches live origin main into a temp ref with `--refmap=`
(branch EndpointTip, lines 76-79); `refs/remotes/origin/main` is untouched until
step 4's `fetch_origin`. Step 5 checks "expected product endpoint" against the
main checkout's own tree. land.sh then rebases the staged WIP onto
`refs/remotes/origin/main` (land.sh:803-820) and compares the result to the word.

Two states, one predicate:
(a) origin main product moved after compose: land.sh would ask for a supersede;
refusing early with `--replace-exception` is right, and the remedy works because
recomposition fetches the live tip. Disposition IC-C4 holds.
(b) local main behind fetched origin/main in product, origin unchanged since
compose: the tuple's endpoint IS origin main; project(local HEAD) != projected
endpoint; the page refuses "product movement" and prints `--replace-exception`,
which recomposes to the same C and refuses again. The only real repair is moving
local main, which step 1 forbids and no public act names. A primary whose local
main lags origin is the normal seat state (agents land from scratch worktrees;
`Advance` moves local main). This is the more likely first-use refusal.

Tests: DIFFERENT (predicate ref and remedy). WORKS only when local main already
equals origin main; SAFE (refusal preserves bytes). The parent's public-repair
rule makes the looping remedy material.

Smallest amendment: step 5 reads the predicate as
"project(FETCHED refs/remotes/origin/main tree) == tuple projected endpoint,
else refuse with `--replace-exception`"; separately, "if local main is behind
that fetched tip and the index equals HEAD, fast-forward through the existing
`Advance` owner under the same LockPath(PRIMARY) before staging, or refuse
naming that act". Preflight the patch on local HEAD's index after that. Fixture:
TestCarriedStagePreservesCheckout adds "local main behind fetched origin"
asserting either the ff or the named act, never the replacement command.

### IC-C8  minor  Step 4's fetch is described as preceding recording but numbered after it

Step 4 says fetch "BEFORE any staging" and "refuses before recording/staging"
while step 3 records the word. Because composition fetches the code endpoint
itself, a fetch failure after recording leaves a valid word and a truthful
`--using-exception` continuation, so either order is safe. Say which. Suggested:
code and ledger fetch first, then compose, record, status, stage.

## Checked and not raised

- `--staged-only` makes land.sh run `git diff --cached --check` (land.sh:393-396),
  inherited carried behavior. Probe: `git diff --check` over this branch's product
  diff against its merge-base with origin/main reports nothing, so the first
  goal passes; a later goal with trailing whitespace refuses here with the stage
  retained. Existing owner rule, not a design defect; worth one line.
- `Advance` refuses `advance-index-not-empty` (advance.go:74), so a retained
  stage blocks agents rather than being destroyed; consistent with "no
  reset/clean rollback" and truthful reporting.
- The replacement rule (no silent rejoin for the same workspace) inverts today's
  `recordedException` ordering (intent_exception.go:75-79); the page states it.
- Tuple adoption on lost response: exactly-one-match else refuse. Bounded and
  safe.

## Answers to the challenge

Real carried-script handoff: does NOT complete as drafted (IC-C6). Preservation:
holds, refusals retain bytes, index and word. Authority and crash replay: held by
the existing owners as claimed. Material count: two (IC-C6 blocking
contract-shape, IC-C7 bounded fixture), one minor.

## Unexamined

`scripts/receipt.sh add` field schema and its idempotence; `landing drift`'s
exact classification of a staged ledger append; pathclass manifest mapping for
this goal's changed paths; `goal carry --supersede` owner internals; channel
`answer` words; conf.local (not read); no build, test or fixture run.
