# Provisioning identity: who commits in a fresh target

- Goal and current status: close KI-31 at its root. Benchmark
  provisioning performs raw `git commit`s in the target it creates,
  so its success depends on the process tree it happens to run
  under: orphaned drivers classify HUMAN and pass the target's
  pre-commit guard; drivers with a live agent ancestor classify as
  an agent and are refused for lacking the wrapper token. The kit
  gate inherited the same shape-dependence through its provisioning
  leg (KI-31 recurrence, 2026-08-10), which blocks any chained
  gates+push run from an agent session — the current 24-commit
  backlog's direct cause. Design note for ONE defect-driven sol
  round under the supervision-lifecycle close rule.
- Next step: none
- In flight right now: nothing in this checkout — the critique job
  runs in the slc-r4 worktree (KI-34) with a tracked waiter.
- Waiting on the human: nothing.

## The decision KI-31 asks for

WHO HOLDS A FRESH TARGET'S LEASE DURING PROVISIONING: the
provisioner itself, as a SAME-LIFETIME identity — the same answer
D-3 gave for fixture custody, and the same mechanics KI-30's fix
already ships for the runner's anchor commits.

## D-P1. The provisioner is the target's first main
## (revised against round 1: PI-R1-001..005 folded)

`provision.sh` creates the target repository, so for the span of
provisioning there is exactly one legitimate writer: the
provisioner. Order is normative:

1. TARGET RESIDUE IS RESOLVED FIRST (PI-R1-004): a target path that
   already exists and carries a lease record is residue of a dead
   provisioning — the provisioner proves the recorded holder DEAD
   by exact identity (three-way: a LIVE holder refuses loudly,
   UNKNOWN refuses loudly — uninspectable is alive) and then
   deletes and re-creates the target wholesale; that deletion IS
   the "disposable target" claim made executable, and no janitor
   duty exists because the next provisioning attempt is the
   recovery. An existing non-empty target with NO lease record is
   foreign and refuses loudly.
2. the announce happens AFTER `adopt.sh` (PI-R1-001: the target's
   helpers do not exist until adoption; nothing is committed before
   adoption, so no commit precedes the identity): the provisioner
   announces itself in the TARGET (`worktree-lease.py announce`,
   runtime carried from the provisioner's own classification) and
   then VERIFIES holdership with `require-holder --caller-pid $$`
   (PI-R1-002: announce alone does not fail against a live holder;
   the verification is the loud refusal);
3. BOTH provisioning commits are wrapper-carried (PI-R1-005: there
   are no pre-guard commits — adoption installs the guard before
   the first commit, so the raw-commit exception of the draft is
   DELETED): the instruments commit and the unsigned-contract
   commit (the latter keeping METASYSTEM_ALLOW_NEW_PLAN=1, the
   guard's front door) run through the TARGET's
   `scripts/agents/commit.sh` — the KI-30 anchor treatment;
4. the provisioner RETIRES its announcement AND RELEASES the
   lease record AT THE END OF ITS COMMIT PHASE, after pushing the
   unsigned contract and before the arming step (PI-R1-003 plus a
   live finding: retirement alone leaves a lease naming a
   retired-but-live pid, which locks the NEXT identity out — KI-33
   verbatim; the departing-main release of the S4-8 fixture
   precedent removes the lease record with the retirement). The
   pre-existing arming step then establishes ITS OWN identity on an
   unheld checkout, exactly as it always has. The human's later
   seal and sign commits are HUMAN commits, sovereign under the
   guard; the runner's resume establishes its own identity exactly
   as today.

## D-P2. What this removes

- The classification dependence: commits are wrapper-carried under
  ANY invocation shape — interactive session, background task,
  orphaned driver. The KI-31 workaround (launch drivers detached)
  stops being load-bearing; detachment goes back to being a
  scheduling choice.
- The kit gate's shape-dependence: its provisioning leg exercises
  the same wrapper path a real run uses, under the gate's own
  process tree.

## D-P3. What this does not touch

- The pre-commit guard itself: no bypass, no new fail-open branch.
  The guard's rule — agent commits go through the wrapper — is
  SATISFIED, not exempted.
- The seal/sign boundary: unchanged; the kit still stops there on
  purpose.
- The runner's anchor path (KI-30's fix): unchanged; this note
  extends its precedent backward to provisioning.

## Numbers and mechanics (decisions, not examples)

The announcement uses the provisioner's exact pid + pidStartedAt
via the target's own helpers; no new identity source. Retirement
uses the existing `retire` verb. A provisioner that dies
mid-provisioning leaves a fresh target with a dead holder — the
target is disposable by definition and re-provisioning replaces it
wholesale; no janitor duty is added.

## Proof

- THE AGENT SHAPE IS FORCED, NOT HOPED FOR (PI-R1-006): the kit
  gate's provisioning leg runs under a deterministic agent
  classification (the census fixture ancestry override, the S4-8
  pattern) and asserts BOTH that the wrapper-carried commits
  succeeded AND that a control raw commit in the same shape is
  REFUSED — a green leg therefore proves the wrapper route exists,
  not that classification happened to say HUMAN.
- The same run orphaned still completes (no regression on the
  KI-31 workaround shape).
- A target whose lease names a LIVE holder refuses provisioning
  loudly; one naming a provably DEAD holder is replaced wholesale;
  UNKNOWN refuses.
- After provisioning returns, the target's lease has no live
  holder and the runner's resume claims it exactly as today.
