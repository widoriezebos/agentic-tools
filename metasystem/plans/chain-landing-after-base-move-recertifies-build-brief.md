Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal chain-landing-after-base-move-recertifies)
Date: 2026-09-08

# Build brief: a reviewed chain recertifies after its base moves

The specification is
metasystem/plans/chain-landing-after-base-move-recertifies-design.md,
revision 2, final. Read it whole before you touch anything. Its
critique ladder is closed: one independent read (job clbm-crit1,
disposition
metasystem/records/misc/chain-landing-after-base-move-recertifies-critique-r1.md),
five material findings, all folded. There will be no further design
revision and the page says so.

The contract behind it is
metasystem/plans/goals/chain-landing-after-base-move-recertifies.md.

## Why this exists, in one paragraph

When main touches a file a reviewed chain also touched, the landing
applies the certified patch to current HEAD, finds the result differs
from the critic's reviewed tree, and refuses `chain-output-mismatch`.
No candidate tree can satisfy that check, so a finished, green,
independently reviewed chain becomes unlandable. It has happened three
times: m1c's chain at c1525b90, and twice on this machine yesterday,
where one landing went in only because a human authorised a
`--no-verify` bypass. A headless node has no such escape, which is why
this goal sits on the fleet path.

## Build the whole page

The design's implementation boundary names the primary targets and I am
not narrowing it: the recertification owner, the Git merge operation,
landing binding and CLI plumbing, landing transport handling for the
explicit proof reference, their named Go tests, and the command help,
refusal and evidence-retention entries needed to expose the path.

Five obligations carry the goal, and the page gives each an owner and a
smallest proof:

1. Original review preservation. The critic's reviewed tree and round
   artifacts stay byte-identical, and the review accounting fields are
   untouched.
2. Exact base recovery.
3. Deterministic no-overlap construction, through the single text
   predicate the page defines. That predicate is owned by ONE function
   and used by the producer, the proof recomputation and the content
   canary alike; do not write a second copy of that rule anywhere.
4. Exact merged-candidate binding, so a candidate that quietly changed
   a certified file still refuses.
5. Refusal after target movement, with the named park rather than a
   rebase loop.

## Hold these lines

- No merge-only critic lane. A proof failure refuses and parks. It does
  not dispatch a critic, reopen a register, or acquire a review
  allowance.
- No edits to `reviewRoundLimit`, `criticRoundsConsumed`, finding
  registration, goal budgets, approval semantics or the mission wall.
- No fake implementer round to store the merge.
- The area-width test command is an explicit coordinator input with no
  default and no inference. There is no `true`.
- Do not weaken the binding to make the path work. Re-aim it exactly as
  the page specifies.

## How to prove it: canary first, never a battery

Wido's instruction, 2026-09-08: canaries, not batteries. The page names
three canaries with their smallest run, their required observation and a
ceiling each. Run them exactly as written, in that order, and report
each one individually with what it observed.

The refusal observations matter as much as the acceptance. The page was
corrected on precisely this point, because its first revision's refusal
canaries required only that the landing not pass, which many wrong
implementations also satisfy. Every refusal canary you run must fail for
the reason named, with every unrelated prerequisite valid, so the only
thing that can produce the refusal is the defence under test. If you
find a refusal observation you cannot make discriminating, say so in the
return rather than settling for a weaker assertion.

Do not run the fixture beds and do not run a battery. Run
`scripts/agents/go-gate.sh --fast` once, at the end. If staticcheck
cannot write its cache in your sandbox, redirect the cache and say so.

## Verification to report

- Each of the three named canaries, individually, with its observation.
- `go build ./...`
- `go test` for the packages you touched, once, after the canaries pass.
- `bash -n` for each shell file you edit.
- `scripts/agents/go-gate.sh --fast`, once, at the end.

The orchestrator runs the process-owning beds and the receipt-bound
battery outside your sandbox.

Gap rule: stop and report a gap; never fill it silently. The page names
four things that are outside it, including globally unique sequence
numbers of any kind, semantic independence claims, and any change to
what a claim or approval means. If the build seems to need one of those,
stop and say so.
