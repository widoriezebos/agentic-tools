# Blocker found unattended, 2026-09-10 00:25 local: goal records cannot land

Found by m1 (Fable, unattended) landing the dispatch-cap-necessity
records carriage. Since 1b12f534 ("Make testing risk-selected and retain
proof for landing") the testing contract metasystem/testing.json owns no
metasystem/plans/**, metasystem/records/** or memory/rulings.md, so
land.sh's `test verify --purpose delivery` answers "delivery impact is
unresolved: no surface owns changed path ..." for every goal brief,
dispositions file and shared append-only record, and the commit is
refused ("required shared testing proof is missing or insufficient").
Code chains still land (their paths are owned).

Remedy proposed: a `goal-records` surface owning metasystem/plans/**,
metasystem/records/**, metasystem/memory/rulings.md and
metasystem/memory/receipts.log with the same static groups as the
`instructions` surface, plus a testpolicy test that a records-only
change resolves as delivery.

Not done unattended because the surface belongs to m1c's contract and
the message to m1c was held for Wido's approval. Goal
goal-records-owned-by-the-testing-contract is OPEN (tier 3) and waits
for approval. Wido: approve it at the terminal (or land the surface
yourself); then the untracked records
in this checkout (hp-terminal-grade-for-stopping-acts, dispatch-cap-necessity,
brain-summary-leads-the-stop-display, stop-refusal-fits-on-one-screen)
land with `land-records.sh <goal> <msg>` from the m1 scratchpad.

## Second finding, 2026-09-10 00:30 local: two cmd tests fail on main itself

On main at adad2d08 (with 1b12f534) and on this seat, `go test
./cmd/metasystem/` fails two tests that the chains did not touch:
TestProcessClassifierDataFailureRepairsThenRetriesTheRequestedVerb
("inventory unreadable: ... supervision state is unavailable ...
metasystem.conf lists no metasystem.runtimes") and
TestFrozenPublicVersionOneCorpusRunsAllSixCasesThroughFirstTransitionWorker
("authenticated first-transition worker did not complete all six frozen
cases: exit status 78"). Both pass nowhere on m1; they are 1b12f534's
own. Whether the risk-selected landing battery selects them decides
whether code chains can still land; m1c owns them.

## Third finding, 2026-09-10 01:50 local: engine-changing chains cannot pass the receipt

`landing test-receipt --mode auto` copies the ENROLLED engine
(read-only, built at HEAD) into the isolated candidate worktree and runs
the selected sections with it. For a candidate that changes engine code
or agent scripts, the deep selection includes section/gate-fence-fixtures,
whose dispatch preflight (scripts/agents/dispatch.sh, the engine/script
skew check) refuses: "engine commit <HEAD> is older than checkout commit
<candidate> and engine or agent scripts changed". A candidate-built
engine cannot be enrolled instead (ENROLLMENT_DRIFT refuses non-landed
stamps). So since 1b12f534 no chain that changes the engine can mint a
sufficient receipt on this seat; the one-file testing.json chain landed
because it changes no engine code. Both reviewed chains below are
closed and land-ready the moment the receipt runs the sections with a
proof engine built from the candidate (or exempts the skew preflight
inside the isolated worktree):

- dispatch-cap-necessity: chain dispatch-cap-build2-20260910 (rounds/2,
  reviewed tree 95d34c943b1e25326726767b6bef22da24889f6d, critic
  dispatch-cap-crit3-20260910, no material finding). Receipt attempt
  proof-mtuug70l-d806bab29713016c failed only section/gate-fence-fixtures.
- hp-terminal-grade-for-stopping-acts: chain hp-terminal-build1-20260909
  (rounds/14, reviewed tree 01f26b23645901c0ee1d86effedbc9d1d4b4d4ef,
  critic hp-terminal-crit12-20260909, no material finding).

Landing command for either, from the m1 scratchpad:
`run-landing.sh chain <root> <goal> <rounds/N/diff.patch> <msg> <critic>`.

## 03:05 local: the contract correction lands only without its test

Chain dispatch-cap-surface2-20260910 (goal-records surface + a
testpolicy selection test) was reviewed clean but its receipt failed
section/gate-fence-fixtures: the test file is a Go change, so the skew
preflight refused. Chain dispatch-cap-surface3-20260910 carries the
surface entry alone (the contract-only shape that landed as c0d5b136).
The selection test rides with the first chain that can change Go files
again. The receipt wall itself (enrolled engine copied into the
candidate worktree; skew preflight inside gate-fence) is the item for
m1c / Wido.

## 03:20 local: dispatch-cap-necessity's own budget is drained by the bug it fixes

Its reserved pool (480 minutes) reads used=480: every dispatch charged
its 120-minute cap and kept it (the exact defect of R-49-m1b), so the
critic of the contract-only surface chain could not be admitted under
it. The chain (dispatch-cap-surface3-20260910, tree
aad5716ab6f13c051eb908d59694f693d4de6d83) carries on under
hp-terminal-grade-for-stopping-acts, whose records need the same
surface. Wido: `goal set-budget --id dispatch-cap-necessity --by Wido
--elapsed-limit 1d --attempt-limit 10 --reserved-job-minutes-limit 720
--active-job-limit 1 --review-round-limit 3` (the tier-3 norm) before
its own chain lands; once the settlement box is on main this stops
happening.
