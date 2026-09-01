# Live records and the landing gate: the carry convention

- Goal: digest-landing-race (plans/goals/digest-landing-race.md, revision 4)
- Mode: design slice (one 4h design slice per the goal's appetite; this
  document is its artifact)
- Date: 2026-09-02
- Author: implementer-050218716497f94c7b2fdb47, dispatched by
  m0b+main-1788250419-3170380-8a1fb3

## 1. The problem, stated from the traced facts

A live record is a tracked repository file that a background process writes
on its own cadence. Today there is exactly one: `records/narrator-digest.log`,
appended by the steward runner through `internal/narratordigest`
(`narrate.go:94`, `counselor_carriage.go:136` and `:182`,
`ruling_sweep.go`). The writer takes an flock at
`artifacts/agents/steward/narrator-digest.flock` (`digest.go:73-97`), reads
the whole file, appends, and publishes by atomic rename
(`digest.go:110-148`, `:154-194`). It never edits or deletes a line.

`scripts/agents/land.sh` is the landing gate every landing routes through
(records/goals/landing-chain-tooling.md). It refuses at four points a live
writer can trip:

1. `stage_changes` refuses when unstaged changes remain after staging
   (`land.sh:232-235`) or when untracked paths remain (`:236-241`).
2. `require_clean_after_commit` refuses when the tree is not clean after
   the commit (`:255-263`).
3. `git rebase` itself refuses to start on a dirty tree (`:269-271`,
   invoked at `:293` and inside the push-retry loop at `:310-311`).
4. Pathspec mode refuses a non-empty index up front (`:216-219`).

Any landing slower than the append cadence gets refused: m2 recorded three
refusals in one session, and the counselor-carriage landing (dcc44ca9)
raised the write rate by putting counselor briefs on the same file.

Half of the original goal is already shipped. The digest-union-merge goal
(IL-34, memory/instruction-ledger.md:100) landed `merge=union` for the
digest and seven other append-only registers (`.gitattributes:2`), and the
append-only carriage refusal guards the registered paths at the commit and
push boundaries (`internal/landing/observe.go:467`,
`scripts/agents/register-carriage-paths.txt:1`,
records/two-bars/digest-merge-addendum.md). That solves the CROSS-MACHINE
race: two landings that both appended merge by union at rebase time. What
remains — and what this design settles — is the LOCAL race: the writer
dirties the tree while a landing is inside the gate, and the clean-tree
checks refuse. The m0b seat's roughly fifteen hand-resolved digest
collisions on 2026-09-01/02 are the cost trail.

Binding constraints from the goal record:

- The no-softening byte-equality law on counselor appends survives: no
  dropped or reordered bytes on any live record.
- A new role adopts the convention without touching land.sh again.
- The first migration proves it on the narrator digest.

## 2. The decision: shape (b), refined into the carry convention

Chosen shape: a declared live-record registry that land.sh consults
generically, with one refinement over the goal's sketch — the gate is
never re-staged "inside" or around; instead a **carry step runs in front
of each unchanged guard** and makes the tree genuinely clean by committing
the live appends as ordinary git commits. The guards' bytes and semantics
do not change at all.

The seam in one sentence: **before each point where land.sh requires a
clean tree, a bounded carry step commits any pending appends on declared
live-record paths as their own pure-append commit, after proving byte-wise
that they are appends; everything else still refuses exactly as today.**

### Why not shape (a), untracked-live with landed snapshots

- The digest is fleet-shared human truth: its bytes reach other machines
  and the returning human through git. Untracked-live keeps each machine's
  narration local until a fold-in, and the fold-in step re-creates the
  identical race at fold time (the fold is itself a landing of a file the
  writer is still appending to).
- The Stop-hook check-in reads `Pending` against a machine-local cursor
  whose prefix SHA-256 hard-errors when prefix bytes change
  (`digest.go:241-243`, `supervision-hook.sh:165-170`). Snapshot rotation
  rewrites prefix bytes by construction and would trip this on every
  rotation, or force a cursor-recovery mechanism this goal does not need.
- It rips out machinery that just landed and is adopted: the union
  attribute, the append-only carriage refusal, and IL-34 all treat the
  digest as tracked. Job artifacts fit the untracked pattern because they
  are machine-local and mirrored by the reaper; the digest is not.

### Why not shape (c), writers honoring a landing lease

- The race that costs the most is cross-machine (the ~15 union
  resolutions), and a machine-local lease cannot govern another machine's
  writer at all. The lease only ever addresses the local refusals.
- A lease held across `fetch`, `rebase`, and up to three `push` attempts
  (`land.sh:295-313`) blocks writers for network-scale time with no bound;
  the writer's flock acquisition loops uninterruptibly
  (`digest.go:81-86`). An unbounded quiet window for narration, sized by
  push latency, is the same defect class as an uncapped wait.
- It couples land.sh to every writer's lock path (the digest flock lives
  in writer-private state, `digest.go:67-69`), which is exactly the
  "touch land.sh for each new role" coupling the goal forbids — inverted,
  but the same coupling.
- The lease is also rejected as a *component* (no short quiesce inside the
  carry loop): the bounded retry below suffices, and a measured reject
  condition (§10) guards the case where it would not.

### Why the carry refinement beats literal re-staging inside the gate

The goal's sketch (b) says "registered paths get re-staged atomically
between clean-tree check and commit." Re-staging into the caller's commit
has three defects the carry avoids:

- It mutates the caller's declared commit content after the caller named
  it, in both pathspec and `--staged-only` mode, so the commit message no
  longer describes the commit.
- It leaves the post-commit and pre-rebase windows uncovered; those need a
  second mechanism anyway (the tree re-dirties after the commit exists).
- Amending or re-staging after `commit.sh` has run re-opens the two-bars
  proof work that commit performed.

The carry commit is one mechanism used at every window, and every crash
inside it leaves only ordinary local git state (§5).

## 3. The mechanism, mechanically

### 3.1 The registry

New file `scripts/agents/live-records.txt`, same idiom as
`register-carriage-paths.txt`: one repository-relative path per line, `#`
comments naming the writer and its cadence. First content:

```
# path                          writer                     cadence
records/narrator-digest.log     # steward runner (narratordigest) per tick
```

Globs are not permitted in this registry (unlike the carriage list): a
live record is a specific path a specific writer owns, and the carry step
must be able to enumerate exactly what it may commit.

### 3.2 The engine verb

One new engine verb, `metasystem landing carry-live-records --repo <root>`,
implemented in `internal/landing` (the package that already owns the
append-only carriage rule). Behavior, in order:

1. Read the registry. A missing registry file means an empty registry
   (carry is a no-op and land.sh behaves byte-for-byte as today).
2. For each registered path, compare worktree bytes against
   `git show HEAD:<path>`:
   - identical, or path clean → skip;
   - HEAD version absent and file present → pure creation, treat the whole
     file as the append;
   - HEAD bytes are a strict byte prefix of worktree bytes → pending
     append, eligible for carry;
   - anything else → **refuse the whole landing, exit 2**, printing the
     path, the HEAD byte length, and the first divergent byte offset.
     Never stage, never guess. A rewrite takes a bar by hand, exactly as
     the digest-merge-addendum rules.
3. If any path is eligible, stage ONLY the registered eligible paths and
   commit through `scripts/agents/commit.sh` with the canonical message
   `live-record carry: <space-separated paths>` on the existing
   register-carriage lane (the lane land.sh already exposes as
   `--direct-fix register-carriage`, `land.sh:9`). The commit is a pure
   append by construction and passes the append-only carriage refusal
   without any new exception path.
4. Loop steps 2–3 up to 5 attempts (the writer may append between the
   prefix check and the `git add`; the atomic-rename write means each read
   sees a consistent file). If registered paths are still dirty after 5
   carries, refuse loudly: print the path, the attempt count, and the byte
   growth observed across attempts, and exit 2. Five is a constant, not
   configuration: a carry takes milliseconds, the steward appends on a
   minutes-scale tick, and a writer that outruns five carries is a defect
   to investigate, not to tune around.

The verb performs index operations, so it inherits the pre-existing rule
that one checkout runs one landing at a time (the checkout lease from
`metasystem up` names the single holder); it adds no new concurrency
surface beyond what `git add` in land.sh already has.

### 3.3 The land.sh call sites

land.sh gains exactly three calls to the verb, all registry-generic, so a
new role never touches land.sh:

1. After `verify_checks`, before `stage_changes` — clears pending appends
   so the "unstaged changes remain" and empty-index refusals see an honest
   tree. In pathspec mode the carry commits complete before the caller's
   staging begins, so the empty-index requirement (`land.sh:216-219`)
   still holds when the caller's own staging starts.
2. After `commit_changes`, before `require_clean_after_commit` — covers
   appends that arrived during commit.
3. Inside a wrapper around every `rebase_origin` invocation (the first
   rebase at `land.sh:293` and each retry-loop rebase at `:310-311`) —
   covers appends that arrive between push attempts.

**No guard changes.** `stage_changes`, `require_clean_after_commit`, the
untracked-paths check, and rebase's own dirty-tree refusal keep their
exact current bytes and messages. An undeclared live path gets no carry,
so it refuses precisely as today — the guard stays honest for everything
outside the registry, which is the whole answer to "what refuses an
undeclared live path." The refusal experience (three per session) is the
forcing function that drives a new self-writing role to declare itself;
nothing at land time can mechanically distinguish "a daemon wrote this"
from "a human forgot this," and this design does not pretend to.

### 3.4 Consistency enforcement

`scripts/validate-metasystem.sh` gains one check: every path in
`live-records.txt` must (a) carry `merge=union` per `git check-attr`, and
(b) be covered by `register-carriage-paths.txt`. A registry row without
both is a validation failure. This makes the three declaration surfaces
(registry, union attribute, carriage allowlist) one atomic adoption step
that the suite refuses to let drift apart.

## 4. Union semantics (design question 2, settled)

- **Where the union lives:** in git's built-in union merge driver via
  `.gitattributes`, per path — already shipped by IL-34. The carry
  mechanism itself never merges anything: it only appends and refuses.
  Rebase and stash-free operation mean the only three-way merges on live
  records happen inside `git rebase`, where the union driver governs.
- **What proves byte-preservation:** three independent layers.
  1. The carry verb's strict-prefix check: bytes reach a carry commit only
     when HEAD's bytes are a verbatim prefix of the worktree bytes, so a
     carry can neither drop nor reorder anything.
  2. The existing push-boundary append-only recheck
     (`internal/landing/observe.go:467` and the §8 push-boundary rule in
     the digest-merge-addendum): every outgoing commit's own
     parent-to-commit diff for a registered path must be trailing
     additions only, independently of how the commit was made.
  3. The fixture assertion (§6): after a landing with a live writer,
     `git show HEAD:records/narrator-digest.log` is a byte prefix of the
     worktree file, and every byte the fixture writer emitted appears
     exactly once, in emission order.
- **Non-append divergence:** refuse loudly, never guess — at carry time by
  the prefix check (exit 2, path, prefix length, first divergent offset);
  at merge time the union driver cannot detect a one-sided rewrite, which
  is exactly why the addendum put the refusal at the origin commit; that
  standing mechanism is unchanged.
- **The known residual is inherited, not created:** union merge does not
  guarantee line order inside a merged hunk, and a merge that interleaves
  remote lines before the local cursor prefix trips `Pending`'s loud
  machine-local error (`digest.go:241-243`). The addendum documents this
  residual for union generally; carry commits add no new reordering — a
  carried tail keeps its local relative order, and remote lines join only
  through the same union merge that already carries the residual.

## 5. Interleavings and crash windows (design question 1's cases)

| Interleaving | Outcome under the carry convention |
| --- | --- |
| Writer appends during the carry's stage | The writer publishes by atomic rename, so `git add` reads either the pre- or post-append file, never a torn one. If new bytes land after the add, the next guard sees a dirty registered path and the bounded loop carries again (≤5), then refuses loudly with the observed growth. |
| Two landings race, different machines | Both machines' carry commits are pure appends; the loser's push is rejected, land.sh's existing fetch/rebase/push retry (`land.sh:295-313`) reruns, the union driver merges the appends, and the pre-rebase carry inside the retry wrapper handles bytes that arrived meanwhile. No hand resolution. |
| Two landings race, same checkout | Unchanged pre-existing hazard: two concurrent land.sh runs already collide on the shared index with or without this design. The checkout lease (one live holder per checkout) is the standing guard; this design adds no new writer beyond land.sh's own process. |
| Landing dies between carry and push | Everything the carry did is ordinary local git state: pure-append commits on the branch, clean tree. The next landing rebases and pushes them like any local commits. No sideline files, no recovery journal, no custom crash state — this is the decisive reason carry commits beat sideline-and-restore designs. |
| Landing dies mid-rebase | Pre-existing recovery story (operator resolves or aborts the rebase), unchanged by this design; carry commits in the rebase behave like any other local commits. |
| Caller's commit refused after a carry committed | The carry commit stands: a harmless pure-append landing-in-waiting that rides the next successful landing. Deliberately not rolled back — rollback would rewrite a commit whose bytes are already law. |

## 6. Migration: the narrator digest as first specimen (design question 4)

- **Registry:** add the one row for `records/narrator-digest.log`. Its
  union attribute (`.gitattributes:2`) and carriage row
  (`register-carriage-paths.txt:1`) already exist, so the consistency
  check passes on day one.
- **Writer:** `internal/narratordigest` is untouched — zero changes. The
  shape was chosen so the writer keeps its flock, read-modify-write, and
  atomic-rename discipline exactly as shipped. The steward runner is
  untouched.
- **Fixture set** (extending the `land-fixtures.sh` seed-repo pattern,
  which already copies land.sh and stubs inner scripts):
  1. *Dirty-before-landing leg:* seed digest has pending appends; landing
     succeeds; assert a `live-record carry:` commit exists, the caller's
     commit contains none of the digest bytes, the tree is clean, and the
     landed digest bytes equal the pre-landing worktree bytes.
  2. *Mid-landing append leg:* the seed's `commit.sh` stub appends a line
     to the digest before exiting, deterministically dirtying the
     post-commit window with no test hook in production code; assert the
     post-commit carry commits it and the landing completes.
  3. *Non-append leg:* seed digest has an edited committed line; assert
     the landing refuses with exit 2 naming the path and offset, and no
     commit was created.
  4. *Outrun leg:* stub arranges a re-append after every carry; assert the
     loop refuses after 5 attempts naming the growth.
  5. *Undeclared leg:* a dirty unregistered file still refuses with
     land.sh's existing message, byte-identical.
  6. *Byte-preservation property:* in legs 1–2, assert
     `git show HEAD:records/narrator-digest.log` is a byte prefix of the
     worktree file and contains every fixture-emitted byte in order.
  7. Go unit tests in `internal/landing` for the prefix classifier
     (identical / creation / strict prefix / edit / truncation / binary
     garbage).

## 7. Blast radius (design question 5)

Every current consumer of the digest file:

1. Writers, all through `internal/narratordigest`: steward tick narration
   (`internal/steward/narrate.go:94`), counselor brief carriage
   (`internal/steward/counselor_carriage.go:136`, `:182` — the raw-payload
   path the byte-equality law protects), ruling sweep
   (`internal/steward/ruling_sweep.go`). None change.
2. Readers: `narratordigest.Pending` and `Advance` via
   `metasystem steward digest-pending` / `digest-advance`
   (`cmd/metasystem/steward_verbs.go:188`, `:210`), consumed by the Stop
   hook (`scripts/agents/supervision-hook.sh:165-170`) to deliver
   check-ins to the human — the ultimate consumer. The cursor's
   prefix-SHA guard is unchanged; §4 names the inherited residual.
3. The append-only carriage refusal (`internal/landing/observe.go:467`)
   and its tests (`observe_test.go:38`, `:353`, `:429`, `:478`): carry
   commits must pass it — they do, being pure appends; the fixture legs
   prove it rather than assume it.
4. Declaration rows: `register-carriage-paths.txt:1`, `.gitattributes:2`.
5. Fixtures that write the digest directly:
   `scripts/agents/supervision-hook-fixtures.sh:149`.

Every current consumer of land.sh's clean-tree guard:

1. Every fleet landing on every seat (all landings route through land.sh
   per records/goals/landing-chain-tooling.md) — behavior changes only
   for registered paths, from "refuse" to "carry then re-check."
2. `scripts/agents/land-fixtures.sh` (three-leg fixture; legs asserting
   refusal messages keep passing because guard messages are unchanged).
3. `scripts/validate-metasystem.sh:959` (enumeration) and `:1072`
   (`bash -n`) — extended by the §3.4 consistency check.
4. The rebase machinery's own precondition: "rebase never starts on a
   dirty tree" is preserved — the carry makes the tree actually clean
   rather than exempting anything.
5. The two-bars observe/proof machinery in `commit.sh`, which assumes the
   committed tree matches the settled project tree at commit time: each
   carry is its own commit through `commit.sh`, so the assumption holds
   per commit.
6. The m2 handoff's codified workaround ("stage live appends into each
   landing") — superseded by this convention; the handoff note should be
   updated when the build lands.

## 8. Adoption path for a new self-writing role (design question 3)

One landing that adds three lines: the registry row, the `merge=union`
attribute, and the carriage-allowlist row (the validation check refuses a
partial adoption). The role's writer must publish whole-file states
atomically (rename) and append-only — the properties the prefix check
verifies on every landing thereafter. land.sh is not touched; the engine
verb is not touched; the fixture set gains one row-parameterized case if
the role wants its own specimen proof.

## 9. Build slice (bounded, per the goal's one 4h build slice)

1. `internal/landing`: prefix classifier + carry loop + unit tests.
2. `cmd/metasystem`: the `landing carry-live-records` verb.
3. `scripts/agents/live-records.txt` with the digest row.
4. land.sh: the three call sites (a wrapper around `rebase_origin`, two
   direct calls), no guard edits.
5. `validate-metasystem.sh`: the consistency check; land-fixtures legs
   1–6.
6. Handoff-note update retiring the manual workaround.

## 10. Self-grade

**Grade: A−.** The shape is decided against the stated interleavings with
file-and-line grounding; the guards are untouched, so the honest-tree
invariant is preserved rather than weakened; every crash window resolves
to ordinary git state; the writer is untouched; adoption is three
declarative lines plus a suite check. The minus: the bounded loop is a
probabilistic answer to the writer-outruns-landing case (justified by the
cadence gap, but a judgment about rates, not a proof), and the same-checkout
concurrent-landing hazard is inherited rather than closed.

**Reject this design if any of the following holds at build or review
time:**

1. A carry commit cannot pass the existing commit gate
   (`commit.sh` + the append-only carriage refusal) without adding an
   exception path or weakening any check — the design claims it passes as
   a pure append on the existing register-carriage lane; if that claim is
   false, the shape is wrong, not the gate.
2. The mid-landing-append fixture (§6 leg 2) cannot be written
   deterministically without a test hook in production land.sh or
   commit.sh bytes.
3. After the first fleet week, any landing refusal with cause
   "registered path still dirty after 5 carries" occurs — that falsifies
   the cadence-gap judgment the bounded loop rests on, and the design
   must be reopened (a quiesce component or event-coupled carry), not
   the constant tuned upward silently.
4. Byte-preservation leg 6 can be made to fail: any fixture-emitted byte
   missing, duplicated, or reordered in the landed file indicts the
   prefix check and with it the whole convention.
