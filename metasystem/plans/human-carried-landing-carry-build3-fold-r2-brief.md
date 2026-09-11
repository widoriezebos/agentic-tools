Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal human-carried-landing-carry, follow-up round on chain hcl-build3-20260911)
Date: 2026-09-11

# Fold: the second read, and what the host found

Round 1 of this chain (reviewed tree faccdb3b25ea9fc1d9ca6fa10a87864fe7a2fbfa)
was read on Opus: `metasystem/artifacts/agents/hcl-context/code-read-r2.md`
(hcl-cc2-20260911), sixteen findings, twelve material, one critical. The
coordinator also ran on the host the beds your sandbox could not. The
decisions below are binding, one per item; the design
metasystem/plans/human-carried-landing-carry-design.md revision 6 and the
first fold (`metasystem/artifacts/agents/hcl-context/build2-fold-r1-brief.md`)
stay the specification. Rule of the round, unchanged: no fixture claims
what it does not drive, and no waiting on the sandbox (name a
sandbox-bound test within five minutes and move on).

# What the host found on your round-1 tree

- **The manifest duplicate.** The 3-way rebase kept the chain's
  `install:testing.json behavior` (line 7 of
  metasystem/scripts/agents/path-classes.txt) beside trunk's identical
  line from a584e5c8c (line 14). The manifest is then unreadable and every
  landing in the land bed refuses (`register-carriage-policy-unreadable`;
  exit 3 with "path class manifest line 14 duplicates install:testing.json"
  in carried mode): 12 of 21 legs red. Delete the chain's line; with that
  one change the coordinator's scratch run passes 17 of 21 legs.
- **carried-ledger-path** then fails on its own assertion: the landing
  refuses with exit 3 and "ledger path plans/goals/illicit.md changes only
  through a goal verb", but the leg looks for the code name. The page says
  every exit-3 ask prints the observation's refusal: print the code
  (`ledger-path-not-goal-verb`) in that line and keep the leg's assertion.
- **carried-two-seat, carried-debt-abandoned, carried-debt-expired** fail
  at "the second seat's green battery did not pass": seat B's `test run`
  refuses with "no surface owns changed path payload-b.txt". Own the second
  seat's payload in the fixture contract (or reuse the owned payload
  path); the legs' own assertions have not yet run, make them pass.

# The read's defects (fold every one)

1. **HCL-C-50 (critical), prefix-aware.** `print_carried_advisory` reads
   the goal file with `git show "<tip>:plans/goals/<goal>.md"`, which git
   resolves from the repository top; under this repository's `metasystem/`
   prefix it fails, the wrapper exits after the local commit and the intent
   entry, and the reservation stays open. Read the goal file prefix-aware
   (the `<tip>:./plans/goals/...` form from the installation directory, or
   through the engine), audit every other `git show <rev>:<path>` in
   land.sh and commit.sh for the same fault, and add one carried leg that
   installs under a prefix directory the way this repository does
   (`carried-prefixed`: the same fresh landing as carried-fresh, installed
   under `metasystem/`), so the class is caught.
2. **HCL-C-51 (high), the seat's second carry.** In carried mode an
   append-only change to records/counselor/carried-landings.jsonl is
   admitted when every added line is a `cl-<opid>` line of a carried row
   on the tree (the wrapper wrote it); a brand-new file under
   records/counselor/ with no goal is refused like an existing one. Leg:
   two carried landings on one seat in sequence, the second carrying the
   first's counselor line.
3. **HCL-C-52 (medium), the trap closes the row.** In the EXIT trap, when
   the wrapper's own created intent entry exists (it is the owner),
   terminalize that entry first, then abandon the reservation; the trap
   no longer discards the abandon's refusal silently but prints it. Leg:
   a failure injected between the intent entry and the push (the fixture
   pause seam) leaves a closed row.
4. **HCL-C-53, -54, -55, -56 (low).** The pre-push obligation line appends
   `:battery-red` when the battery is red. The printed `goal carry
   --supersede` line passes the bare name to `--by` (no `human:` prefix),
   and the command layer refuses a `human:`-prefixed `--by`. `landing
   observe --carried --judge base` without `--live-failure`, or with a
   judge that is neither live nor base, exits 2 (usage). land.sh guards
   every `"${array[@]}"` expansion that can be empty under bash 3.2
   `set -u` (verify_checks' pathspecs at least), since the reruns run with
   nothing staged; a rerun with nothing staged is driven by carried-crash.

# The proofs that must exist before this lands (HCL-C-57 to -60)

5. **HCL-C-57, the match and the red battery.** A table test over the
   carried match (decideCarriedMatch or observeCarried) with these rows,
   each asserting the exact refusal or landing: a code word with a
   sufficient result lands; a code word with an insufficient result asks
   carry-refusal-mismatch; two failures under one name ask; an uncovered
   obligation beside the named group asks; a discrepancy beside the named
   group asks; a verify error asks carry-battery-unverified; a group word
   whose only insufficiency is that group lands; a closed chain whose only
   red group is named lands. And ONE land leg, `carried-red-battery`: a
   group word lands on a red battery through commit.sh's red path (`test
   verify --carried --json`, the group-list parse, `Carried-Battery: red
   missing=... failing=...`, the obligation `carried:<commit>:battery-red`).
6. **HCL-C-58, the fences.** In internal/landing: the base-judge fence
   table (one changed file per owner the page names, each asks
   carry-base-judge-blind) and the generation-zero test (a terminal word
   with authorityGeneration 0 refused outside a fixture-authorized root,
   accepted inside one). The dead-live-judge and no-judge legs are NOT
   built in this round; they are named under `deviations` as follow-ups.
7. **HCL-C-60, the verb refusals.** In internal/goal: the replay mismatch
   table (the goal target, then each of the fourteen fields, one row
   each, refused naming the field); the supersede precondition table
   (expired, consumed, foreign seat without --transfer, non-carry target,
   in-flight target, push-before-row); `done` refusing an open word and
   passing an expired one; the obligation-debt and in-flight-debt asks at
   the word. Abandon from another seat and the reservation's CAS are
   follow-ups.
8. **HCL-C-59, recovery.** One leg, `carried-crash-local`: a crash before
   the push (after the local carried commit) and the rerun through the
   `local:` branch that keeps the rebased commit and pushes it. The
   `ledger:` repair leg and the goal recover tests are follow-ups.

# Follow-ups, recorded, not built

HCL-C-61 (the forward-path legs, the cap and unneeded asks at the landing,
main-only, the record-failure stop list, the seven-step order, two
commands), the two judge legs of HCL-C-58, the remaining recovery legs
and goal recover tests of HCL-C-59, the abandon-from-another-seat and CAS
tests of HCL-C-60, and HCL-C-64's weaker assertions are carried as named
follow-ups on the goal by the coordinator. List each id you leave out
under `deviations` so the record is exact. HCL-C-62, -63, -65 are recorded
and not actioned.

# Proof before you return

`go build ./... && go vet ./...`; `go test ./internal/goal ./internal/landing
./cmd/metasystem ./internal/refusal` (sandbox-bound tests named, not
waited for); `bash scripts/agents/go-build.sh` then `metasystem test check
--root .`; `bash scripts/agents/go-gate.sh --fast`; `bash -n` on every
changed script; the land bed's carried legs individually with
`--fixture-bed-child <leg>` as far as the sandbox allows (they need
supervision and bare repositories; if they cannot run, say which, the
coordinator runs them on the host).

# Return

`diffBoundary` and `files` are repository-root paths. Under `evidence`
every command with its observed result; under `deviations` every id left
out with the reason. Order: the host section, then 1 to 4, then 5 to 8.
Return BEFORE the 120-minute cap: at 90 minutes stop adding proofs, run the
proof list and return. A return at 90 minutes with items 7 and 8 named as
missing beats a timeout.
