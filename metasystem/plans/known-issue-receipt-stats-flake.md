# Known intermittents in full-suite runs (one root-caused, one open)

## ROOT-CAUSED 2026-08-12 late: the nested-gate test flake

The wandering nested-validation failures ("copied-skills/pruned target
failed validation") were `TestRunHeldRefusesNonHolder` (internal/lease)
failing inside nested adopted-copy gates — named by the gate's new
evidence-preservation on its first firing. Mechanism: `cmd.Start()`
returns after fork but before the child's execve completes; in that window
the kernel reports an empty argv and the auth identity (pid, start,
command) is rightly unreadable. Load — a nested gate inside a full suite —
stretches the window to test-visible width. Both child-probing test
helpers now wait out the window, bounded. Production is not affected: real
callers announce themselves post-exec.

## STILL OPEN: receipt-stats greps flake in Mac suite runs

Status: OPEN, instrumented, not blocking. Recorded 2026-08-12 after an
extended investigation so the next session starts from evidence, not
theory.

## The signature

One of the receipt fixture's three stats greps
(`validate-metasystem.sh:3794-3796`) fails — `receipts=1`,
`type_improve=1`, or `--all receipts=3` — while the ledger's bytes are
PERFECT in the preserved evidence every time (four lines, three receipts,
one retro, all same-second). Observed six times on 2026-08-12, both at the
outer fixture and inside nested adopted-copy validations (where it
surfaces as "copied-skills/pruned target failed validation" because the
nested stderr is discarded). Roughly every other full suite run on the
Mac.

## What it is NOT (each eliminated by experiment)

- **Not the stats logic**: line-shape based, no time boundary; the
  preserved input replays green; 1,000 direct iterations and 200
  full-fixture iterations in the failing worktree, pipefail on: zero
  failures.
- **Not data corruption**: every preserved ledger byte-perfect; B6's
  durability barrier makes torn appends refuse loudly instead.
- **Not the Linux platform**: the VM has never failed it (7+ suite runs
  green the same day, same commits).
- **Not the leaked supervision fleets alone**: seventeen orphaned
  owner/watcher/reaper processes from Aug 9-12 were reaped (they DID cause
  real scaled-cap timeouts — nested arming at 36s — and the overall flake
  rate dropped after the cleanup), but one receipt failure occurred after
  the cleanup on a quiet machine.
- **Not the heartbeat fsync tax alone**: reclassifying heartbeats as
  volatile (2dd5b12) was correct on its own terms (the conversion had
  added F_FULLFSYNC to a never-synced hot path), but a receipt failure
  followed it.
- **Not stray files in the worktree root**: cleaned; a failure followed.

## What is armed

The `wt-flakefix` worktree carries an uncommitted probe around all three
greps capturing the stats OUTPUT, exit code, ledger bytes, file stat, and
METASYSTEM environment into `${TMPDIR:-/tmp}/receipt-evidence/` at the
moment of failure. Four probed full-suite runs were green before the
session moved on; the next Mac-suite failure at this fixture yields the
first direct observation of the failing invocation. The suspicion ranked
most plausible and unfalsified: a transient failure of the `receipt.sh`
shim's exec (or of the pipeline itself) under whole-machine fsync
pressure, which pipefail converts into a grep miss — the probe records
exit codes precisely to catch this.

## Session decision

Mac full-suite runs remain useful (the probe needs samples), but VM-green
is the validation authority while this stays open: the VM is the plan's
acceptance host and has never exhibited the failure.

## 2026-08-14 firing: landed on an unprobed grep, nested

First Mac-suite hit since the probe was committed, and it dodged the
instrumentation twice over: it fired INSIDE the adopt fixture's
filled-target validation (whose output the outer suite discarded), and the
inner run died in the receipt-relation block — at or near the
`critique_waivers=1` grep, which the probe did not wrap. Evidence:
suite-failures/20260814T002343Z-51987 (outer) containing
.../adopt-default/artifacts/agents/suite-failures/20260814T002343Z-94210
(inner). The inner receipt-relation artifacts are byte-perfect as always
(missing-chain.out carries the exact expected refusal), and the probe dir
stayed empty — consistent with the flake striking an unprobed invocation.
Fixed the observability the same day: the probe now takes the ledger file
(and honors `receipt_stats_sh` for fixture-local copies) and wraps the
critique-waivers grep, and the filled-target validation captures its
output to adopt-filled.out with a tail on failure. The next firing —
outer or nested — names its dying line and leaves probe evidence.
