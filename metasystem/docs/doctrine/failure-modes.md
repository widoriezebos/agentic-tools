# Failure modes

- Kind: doctrine
- Id: 01M3MKDZK3MWEYCGBNSSBPZ9RJ
- Status: accepted

Short records of things that went wrong: the symptom, the real cause, and the
rule that now prevents it. Where a test, gate or guard enforces the rule, the
entry names it and the test is the record. Lessons a test fully enforces, and
lessons a design page already states, are not repeated here. Distilled on
2026-09-28 from the finished records removed that day (tag
`records-archive-2026-09-28`).

## Evidence and verification

### A fold reported is not a fold landed

- Symptom: a critic's next round re-reports findings as unfixed, or a
  disposition says "folded" for text that is not in the reviewed tree.
  Recorded five times from 2026-08-07 to 08-11: the fold commit was refused
  by a pre-commit guard and success was reported anyway; a fold script's
  unasserted string replacements missed their anchors silently; folds were
  reverted by another writer between write and add; the critic's worktree
  stayed frozen at round 1 (KI-20).
- Cause: the author treated "I ran the edit" or "the commit command
  returned" as evidence that the change is in the tree the reviewer reads.
- Rule: before you dispatch a round or write "folded", read the reviewed
  commit by SHA and confirm that each finding's change is present, for
  example by grepping every finding id or changed sentence at that SHA. Any
  scripted edit asserts its anchor count and fails loudly on a miss. Give
  the reviewer the SHA, never a branch or worktree name.

### A green read through a pipe

- Symptom: a suite reported green while a fixture had failed.
- Cause: the verdict came from `$?` after `run | tee log`, which is the exit
  status of `tee`.
- Rule: take a verdict only from the run's own verdict file or exit status.
  In shell, use `set -o pipefail` or `PIPESTATUS`, never `$?` after a pipe.

### A formatted wrap hides the cause from errors.Is

- Symptom: a missing job record for a remote-machine survivor read as a live
  survivor forever. `arm` refused and told the person to run `arm` again.
- Cause: the record reader wrapped every failure with `%v`, so
  `errors.Is(err, fs.ErrNotExist)` could never match and the not-found
  branch was dead code.
- Rule: wrap with `%w` wherever any caller branches on the cause, and prove
  the not-found branch by running it, not by reading it.

### A write canary proves writability, never liveness

- Symptom: a delegate works for 40 minutes, and its output is lost because
  the sandbox was read-only. Separately, an agent believes a finished job is
  still running because an old canary file reads as a sign of life.
- Cause: writability of the output was assumed, and liveness was inferred
  from a side-effect file instead of the job record.
- Rule: a delegate's first act creates its declared output as a stub, and a
  failure stops it. Liveness and completion come only from the job record
  (the watcher's DONE/STALE/NEVER-STARTED, `docs/orchestration.md`), never
  from files the job leaves behind.

### An unattributed kill: run a canary pair

- Symptom: a process dies with a bare "stopped" and no log names the sender.
- Cause: unknown; the killer could be a supervisor's policy or a
  machine-wide actor.
- Rule: launch one supervised task and one independent tagged process, watch
  both, and snapshot the agent processes when either dies. The member that
  survives names the killer's category, without root tooling.

## Tests and fixtures

### Collapsed fixture roots hide routing defects

- Symptom: a test passes whether or not the code reads the right root. The
  custody guard refused a second supervision start on every real checkout, and
  a suite-progress leg stopped checking that dispatch relays the job's
  workspace root. Both beds were green.
- Cause: the fixture put roots that production keeps apart (git scope, state
  root, installation, journal or workspace root) in one directory, so reading
  the wrong one gave the right answer.
- Rule: a fixture gives every root production can tell apart its own
  directory. Where a fallback exists, a decoy at the wrong root makes a
  misroute fail visibly.

### A fixture inherits the agent's ancestry

- Symptom: an authority fixture fails with status 78 ("proof caller has no
  authenticated goal reservation context") when an agent session runs it,
  and passes when it runs detached (parent pid 1).
- Cause: `lease classify` walks process ancestry. An inherited Codex or
  Claude ancestor wins before the fixture's explicit terminal input.
- Rule: run authority and identity fixtures detached from the agent session,
  or give them their own ancestry. Never change production identity rules to
  make such a run pass.

### Platform-resolved identities differ

- Symptom: a witness compares a fixture key or path and fails on one OS only.
- Cause: Linux keys carry start ticks and boot id; Linux reports the resolved
  binary, macOS the resolved temp directory.
- Rule: compare keys by `EncodeKey`; take pid and exe path from the probe,
  never from what the test launched. (fixture-children amendment 6,
  2026-09-17)

### Reproducing a load-shaped red

- Symptom: a test goes red on a busy host and green alone.
- Cause: a timing or ordering assumption (R-104-m1e).
- Rule: reproduce it before you fix it. First list stray CPU burners
  (`ps -axo pid=,etime=,%cpu=,command= | awk '$3 > 50'`). Then run the
  suspect package under `-race` next to eight `yes` burners. Then run the bed
  through the engine on the candidate tree. Land the fix with a test that
  fails on the old code. Never retry the red. How a waiting test counts
  progress is in `docs/patience.md`, "Waits in tests: attempts, not
  wall-clock".

### A speed fix aimed at a cost nobody measured

- Symptom: a synthetic clock for the missionrunner package changed its time
  from 582 s to 586 s (2026-08-29).
- Cause: a trace showed that git-backed bed construction and conclusion
  dominate the time. Only 2.8 s of logical waits sat in runner sleeps.
- Rule: trace one representative test before choosing a mechanism. The
  mechanism that worked was one bed template per package, copied with
  `cp -cR` (APFS copy-on-write, 0.06 s). A local git clone was rejected
  because it drops local config, runner refs and pseudorefs that the tests
  assert. The copy must reset checkout-local transport state (`FETCH_HEAD`);
  `testbed_cache_test.go` and `TestResolveTaintMultiTaintDiscipline` enforce
  that.

## Records, schemas and shared state

The companion rule for readers, that old records lack new members, is rule 4
of `docs/design/wire-documents.md`.

### A schema bump announces an optional field and refuses the fleet

- Symptom: a new field added to a wire document fails every dispatch closed.
- Cause: a production gate matches `schemaVersion` exactly
  (`internal/dispatch/attest.go` CensusFresh requires 2), so bumping the
  version to announce the field is a fleet-wide refusal.
- Rule: an optional field that old readers tolerate is added WITHOUT a schema
  bump. Bump only for a change readers must refuse, and ship the gate's
  acceptance first. (census `scanSeq`, 2026-08-19)

### The measuring tool rejects the producer's newer fields

- Symptom: a benchmark trial was ruled invalid because the evidence schema
  refused `generation` and `stateDigest`, fields the census had written since
  the KI-18 fix.
- Cause: the checker pinned one schema version and failed on fields the
  producer added legitimately.
- Rule: versioned evidence carries its schema version, and the reader
  dispatches on the version it declares. A checker never rejects a newer
  producer just because its own copy of the schema is older.

### Two seats mint the same content-addressed version

- Symptom: two machines each registered taskrun@0.3 minutes apart
  (2026-08-23); one registration had to be reconciled away.
- Cause: each seat minted the next version from its own stale view of a
  shared registry.
- Rule: before minting anything content-addressed or sequence-numbered on a
  shared seam outside your claimed goal, pull, then land the mint first and
  keep it small. Ruling ids already follow this by construction
  (R-<n>-<machine>).

## Processes and the kernel

### Self-heal outlives its purpose

- Symptom: a supervisor respawns children that die at once, the process count
  and load climb without bound, and killing the children is futile.
- Cause: the self-heal loop has no liveness condition on the thing it exists
  for, and stopping it relies on a caller's cleanup path (trap, shutdown),
  which is exactly the path that does not run on kill or crash.
- Rule: every self-healing loop checks that its purpose still exists and
  tears itself down when it does not; repeated deaths trip a breaker instead
  of relaunching at full rate; teardown never depends on the caller's
  cleanup. Enforced for supervision by TestPurposeGoneExitsAndTearsDown and
  TestSupersededExit (`internal/supervise/owner_test.go`),
  TestDiskOwnerExitsPurposeGoneWhenTheCheckoutVanishes
  (`owner_disk_bed_test.go`) and TestBreakerTripsAtN (`decide_test.go`).
  Contract and incident: `docs/design/supervision-lifecycle.md` (KI-32,
  2026-08-09).

### macOS FIFO reader misses EOF

- Symptom: a shell leash blocked in `read` after its last writer closed (5 in
  7,200 in a probe).
- Cause: the macOS kernel: a reader entering `read(2)` as the last writer
  closes can miss end of file (not Go's poller).
- Rule: never make a FIFO EOF the safety; a fresh writer opened and closed
  releases it; witnesses close the writer before the reader can be entering
  its read. (fixture-children amendment 6, 2026-09-17)

### macOS loses an immediate SIGCONT

- Symptom: a stopped test child never resumes.
- Cause: SIGCONT sent right after `Wait4(WUNTRACED)` reports the stop is lost
  308 in 9,600 (the kernel wakes the parent before the task is suspended);
  SIGKILL in the same window was never lost.
- Rule: release a stopped child through a pipe, never SIGCONT; killing it at
  once is safe. (fixture-children amendment 6, 2026-09-17)

## Design and review

### Designing against an assumed sequence

- Symptom: a design passes its own review but dies in critique because the
  hooks it adds sit at points the code does not have.
- Cause: the author wrote against a remembered call order. The 2026-08
  stop-loss "last defense" design assumed that the runner dispatches critique
  roots (the host does), that the ledger tail carries lines it does not
  carry, and that conclude-time hooks sit where they do not (the decision it
  led to is in `docs/design/stop-loss-core.md`, Decisions).
- Rule: before designing a mechanism that hooks into an existing sequence,
  read the sequence from the owning package and cite file:line for every step
  the design depends on. A step without an anchor is an assumption and is
  named as one.

### A design paraphrases shipped code

- Symptom: late critique rounds each correct the width of the same one
  predicate (delegate-delivery rounds 6, 7 and 8: three paraphrases of a
  presence gate the shipped adapter already had).
- Cause: the design restates existing behavior in its own words, and every
  restatement drifts.
- Rule: replicate shipped behavior BY REFERENCE (file:line, "the code is the
  authority when prose and code disagree") and pin its shapes as fixture
  legs; never re-describe it.

### Parking on an impossibility wider than its premises

- Symptom: a security design is parked as "signatures impossible" (D86,
  genesis authority), and independently fixable defects are left under a
  "kept, defense-in-depth" label.
- Cause: premises that held (an unenforced Devin sandbox; delegates read the
  whole control plane) ruled out only symmetric secrets in the delegate's
  domain and CLI-only checks. They did not rule out a private signer held by
  another principal (OS user, user-presence keychain, hardware, external
  service) with a readable public key and an integrity-protected verifier.
- Rule: state the premises an impossibility rests on, and park on the actual
  human decision (where the trust boundary lives, or whether local authority
  stays advisory), never on "impossible". This grounds the parked goal
  ledger-authentication, whose key custody is open.

### Both pens narrow the charter

- Symptom: a round that held both the design and the build pen passed its own
  tests while a charter bar had shrunk to what the code checked (the two-bars
  bar (a) became a chain-record shape check relating the landed change to
  nothing), and a shipped mechanism (register carriage) was silently dropped
  from the redesign (2026-09-01).
- Cause: one author writes the requirement and its proof, so the requirement
  drifts toward the code and a deletion reads as a simplification.
- Rule: a both-pens exception (R-35-m0) gets a cross-family critic that
  checks each charter bar against the code relating the landed change to it,
  and the design names every mechanism it drops with its disposition.
  R-25b-m1 bars the silent pen; the critic is what catches it.

### A security review framed as an attack is refused

- Symptom: the Codex critic returns a content-filter refusal and no findings
  on an authority or genesis review.
- Cause: the brief asked the critic to construct exploits.
- Rule: frame authority reviews as properties to verify ("show that no caller
  class below MAIN can reach genesis"), never as exploits to build.
