# Known intermittents in full-suite runs (two root-caused, one open)

## ROOT-CAUSED 2026-08-12 late: the nested-gate test flake

The wandering nested-validation failures ("copied-skills/pruned target
failed validation") were `TestRunHeldRefusesNonHolder` (internal/lease)
failing inside nested adopted-copy gates — named by the gate's new
evidence-preservation on its first firing. Mechanism: `cmd.Start()`
returns after fork but before the child's execve completes; in that window
the kernel reports an empty argv and the auth identity (pid, start,
command) is rightly unreadable. Load — a nested gate inside a full suite —
stretches the window to test-visible width. Both child-probing test
helpers now wait out the window, bounded. Third instance 2026-08-14:
TestGroupOwnsTag (internal/lease) failed the same way inside an
adopted-copy nested gate (owned=false provable=false — the group scan's
argv reads hit the window); its assertion now waits bounded like the
other two. Fourth instance 2026-08-14: TestTerminateGroup
(internal/missionrunner), with a twist — its owned child execs TWICE
(`bash -c 'exec -a sleep-TAG sleep 30'`), the one-shot ownership
precondition passed on bash's transitional argv (which also carries the
tag), and terminateGroup's own re-check then landed in the inner execve
window and rightly skipped the signal ("no longer provably ours"),
leaving the sleep alive for the assertion. The production skip is
correct by design; the test now polls, bounded, for the proof in its
final stable form (a member whose argv[0] IS the tagged name) before
exercising the wind-down, after which no exec transition remains.
Fifth instance 2026-08-14 afternoon: the
absent-tag assertion of TestGroupOwnsTag — that scan must inspect
EVERY live pid, so any process on the machine mid-execve makes it
rightly unprovable for an instant (the deliberate
only-ESRCH-is-absence reading from W1.3); surfaced by a nested gate
under an active machine, now waits bounded like the rest of the
family. Production is not affected: real
callers announce themselves post-exec.

## ROOT-CAUSED 2026-08-14: the fixture identity-table tear

AUTH-R2-005 failed once inside an adopted-copy nested validation: dispatch
refused at its entry authority gate with "control-plane write requires the
authenticated lease holder" instead of reaching the attested-ceiling
refusal the fixture expects. Mechanism, each step verified: the
delegate-caps identity-updater loop rewrote the fake identity table every
20ms with a truncate-in-place `write_text`; a reader between truncate and
write sees an EMPTY table (~0.8% of reads in a direct two-process repro);
`FixtureEntryFor` treats an unparseable table as "no entry", so
`AuthIdentity` fell back to the kernel; the fixture shell's kernel argv no
longer hashes to its announced "caps-fixture" command, its announcement
failed authentication, and the ancestry walk escaped past it into the
REAL process tree — where this Mac's `claude` CLI ancestor (argv[0]
`claude`, matched by claude.sh's signature) classified the caller
DELEGATE, which holder-only writes refuse with exactly that message. Fix:
the updater (and the registration write) now write to a temp file and
rename — readers only ever see a complete table. Mac-only by
construction: the VM's suite has no runtime-CLI ancestor, so the same
tear degraded to HUMAN and passed silently. The sibling harnesses
(supervision-fixtures, mission-fixtures) edit identity tables with single
sequential `write_text` calls while daemons read; same hazard class at
far lower odds — worth the same rename treatment if one ever fires.
Follow-up candidate recorded: `FixtureEntryFor` could distinguish a
corrupt table (refuse loudly) from an absent one (fall back), the same
fail-closed doctrine custodyIdentities already follows; signature change
ripples across the auth path, so it goes solo, not in this batch.

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
