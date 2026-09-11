Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, coordinator under goal human-carried-landing-carry, closing read of chain hcl-build2-20260911, final work round 1)
Date: 2026-09-11

# Review brief: closing read of the carried-landing implementation

FINDING IDS: chain-unique, continue the carry's register: HCL-C-35, HCL-C-36, ... never F-n.

## What you are reading

The implementation chain hcl-build2-20260911 for goal
human-carried-landing-carry (metasystem/plans/goals/human-carried-landing-carry.md),
final work round 1 (job hcl-build2-20260911), reviewed tree
b38cf7977ec3ce21a5f04be0fa686dec9b27730a (the installation subtree the conformance review names as
reviewedTree). The specification is
metasystem/plans/human-carried-landing-carry-design.md, revision 6
(landed ab513438b), read three times on the Sol lane
(metasystem/records/misc/human-carried-landing-carry-critique-r1.md,
-r2.md, -r3.md) and folded three times; the third read's four findings
were folded without a fourth read, so this read verifies them as named
checks (mandate 1).

Write your register as this new file: `metasystem/records/misc/human-carried-landing-carry-code-read-r1.md`
(if your runtime is read-only, return it in the job return; the
coordinator projects it).

This is the independent critique of the FINAL work round that a
DESTRUCTIVE-REACH chain needs to close. A material finding here is folded
and re-read, so make each one exact. Read the diff against its base and
the design side by side. A material finding is: a behaviour the design
specifies that the code does not have; a behaviour the code has that the
design forbids; a way a carry lands what the human did not name, or
refuses a verified human on the machine's own judgement; a test the
design names that is missing or does not test what it says; or a
refusal, ask or trailer that differs from the page's text.

## Settled, do not re-derive

The design's decisions. The coordinator reruns outside the sandbox the
tests and beds the implementer's sandbox could not; do not repeat those
runs, read the code.

## Mandate, in order of consequence

1. **The four named checks (revision 6 record).** HCL-C-33: the third
   debt arm scans every proven carry word with a reachable `Carry:`
   trailer and no `carried` row, regardless of consumption or expiry, and
   the two-seat fixture covers abandonment and expiry. HCL-C-25: replay
   compares the goal target before AlreadyApplied. HCL-C-03: the base
   judge's owner fence includes internal/governance (and the others the
   page names). HCL-C-34: the owning surfaces execute every new command
   fixture. One line each: met by the code at file:line, or not.
2. **One word, one defect (05, the match rule).** Trace a code word and a
   group word through `landing observe --carried` and commit.sh: a code
   word hits only with O.Code equal and R sufficient; a group word only
   when G is the only insufficiency. Find any landing with two defects
   that lands, or a single named defect that asks. Check `unneeded` asks
   and leaves the word open.
3. **The transaction under the lease (05, 06).** land.sh's re-exec under
   `lease run-held`; the `carrying` row published before the push and
   counted as debt; the local journal entry; the single push; recovery
   that keeps the rebased commit; `goal carried` closing the row, the
   obligation, the counter, the counselor line; `landing carry-status`'s
   states and land.sh's branch for each, including `superseded`.
4. **Consumption and replay (06).** The ancestry scan anchored on the
   word's ledger commit, never a clock; the full-payload replay with the
   target; supersede's complete predicate inside the mutation.
5. **The authority wire and the fence (02).** `authorityGeneration`
   parse and render; generation zero refused outside a fixture-authorized
   root; the format-version fence and `goal carry --raise-format`; what an
   engine at ab513438b does with a ledger holding the new row.
6. **The obligation and its discharges (07).** The full-id finding; the
   two discharges; the carried accepted-risk record's fields; `done`
   refusing an open carry word or obligation.
7. **The cap, the debt, the counter (08).** `metasystem.budget.carry-open-max`
   as budget law; the seat binding and `carry-seat-mismatch`;
   `--transfer`; the health lines; the counselor `carried-landings` line.
8. **The commit subject (09).** The four seams; what `dispatch.sh close`
   does with a critic chain on a commit subject.
9. **The register flip (03, HCL-C-11).** Every Pending row flipped, the
   three ledger-meaning overrides, the fifteen ask codes as Question
   rows, `TestHCL03NoPendingAfterSlice2` checking the real entry points.
10. **Contract ownership (HCL-C-31, -34).** testing.json's new surface
    and groups; `test check` green; the selection fixture asserting
    execution.
11. **The beds.** The land legs, the goal-cli scenarios and the dispatch
    scenario the page names: each proves what its oracle says; name any
    assertion that would pass on the base tree.
12. **What must not change.** The identity gate equals `goal set-budget`'s;
    a record failure is never carried; the temporary word is not a proof;
    nothing from the shared testing contract or the workspace receipt
    weaker; `sync-transport.sh` untouched.

## What is not in your mandate

Style, comment density, re-deriving the design. Do not run beds.

## Return

Findings by id with the file and line in the reviewed tree, the design
section, the claim, the evidence, and what changes if it stands; then the
four check lines; then a verdict line: closable as is, or one fold naming
which findings. Wall-clock budget: 60 minutes.
