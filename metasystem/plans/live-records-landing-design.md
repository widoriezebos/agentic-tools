# Live records and the landing gate: the gated carry convention

- Goal: digest-landing-race (plans/goals/digest-landing-race.md, revision 4)
- Mode: design slice (one 4h design slice per the goal's appetite; this
  document is its artifact)
- Date: 2026-09-02
- Revision: 2. Folds all nine findings of
  records/misc/live-records-critique-r1.md (LR-001 through LR-009).
  Revision 1's carry convention survives as a component; its central
  claim — that a carry commit rides the unmodified commit gate and that
  bounded retries close the local race — was disproved by the critique
  and is replaced, not patched. Revision 1's own reject condition 1
  fired: the carry could not pass `commit.sh` in either landing mode
  without an exception path, so the shape moved and the gate did not.
- Author: revision 1 by implementer-050218716497f94c7b2fdb47; revision 2
  by implementer-9e3fe395ed232932eded4924, both dispatched by
  m0b+main-1788250419-3170380-8a1fb3

## 0. Revision map: where each finding lands

The four critical findings interlock and are dissolved by one mechanism,
the landing gate, not by four separate patches. The gate is a
checkout-wide reader/writer lock: a landing holds it exclusively from
before its first index mutation through its last push; every registered
live-record writer holds it shared around each publish. Inside that
exclusive section the tree cannot re-dirty on registered paths, so one
carry at the top is enough, the guards become deterministic, and the
rebase runs with the writer provably quiet.

| Finding | Disposition in this revision |
| --- | --- |
| LR-001 (critical) — append-only carriage is advisory | §3.5: append-only becomes a hard refusal at both push boundaries (`land.sh` pre-push, `commit.sh --push`) via a new `landing verify-outgoing` check; the observation trailer stays observational for everything else. |
| LR-002 (high) — fifteen manual conflicts unexplained | §4.2: mechanism-level account — the union driver only ever sees committed bytes, and the manual resolutions happened on uncommitted worktree bytes the driver never sees; event-level classification stated as open (the transcripts do not exist) with a named falsifier. |
| LR-003 (critical) — carry breaks the staging contract | §3.3: the carry no longer invokes `commit.sh` at all; it constructs its commit through a temporary index that never touches the caller's index or staging mode, with its own equally-hard, narrower gate. §3.4 bounds where it runs: exactly once, at gate entry. |
| LR-004 (critical) — carry-to-guard race stays open | §3.2: dissolved structurally for gate-compliant writers — no append can occur between the carry and any guard while the landing holds the gate exclusively. For a non-compliant writer the race persists and the unchanged guard still refuses loudly; §8 states which properties depend on compliance. |
| LR-005 (critical) — concurrent rebase destroys bytes | §3.2/§5: the writer is paused (gate held shared per publish, landing holds exclusive across rebase), so git and the writer never race on the pathname. A post-rebase byte re-verification is kept as a proof layer, with the honest note that re-verification alone could not have recovered destroyed bytes — the pause is the fix, the check is the alarm. |
| LR-006 (high) — three lines do not confer enforcement | §3.5: the append-only evaluator becomes registry-driven — every path in `live-records.txt` gets the shape check, replacing the hard-coded two-path switch. §8 separates what the three declarative lines confer (the byte law, enforced) from what writer code must additionally do (gate compliance, for refusal-free landings). |
| LR-007 (high) — no landing-wide mutex exists | §3.2: the gate IS the landing-wide mutex, taken by `land.sh` as its first act, before any index mutation. The direct-`commit.sh`-during-a-landing case is named as the residual and delegated to the session-coexistence goal seam (Wido's goal owns the two-lander problem; this gate stands alone for one checkout, one landing at a time). |
| LR-008 (high) — crash after staging strands bytes | §3.3/§5: the carry never stages into the shared index, so the stranded-staged-bytes state cannot be produced. Every crash window resolves to a dangling object, a completed pure-append commit, or a self-releasing flock. |
| LR-009 (high) — proof plan stubs the mechanisms under test | §6: a new fixture family runs the REAL `commit.sh` (via its supported non-Go adopted-checkout lane) and the real carry verb; legs cover both staging modes, two-machine same-path rebase, a writer during rebase (compliant and non-compliant), and the crash windows. |

## 1. The problem, stated from the traced facts

A live record is a tracked repository file that a background process writes
on its own cadence. Today there is exactly one: `records/narrator-digest.log`,
appended by the steward runner through `internal/narratordigest`
(`narrate.go:94`, `counselor_carriage.go:136` and `:182`,
`ruling_sweep.go`). The writer takes an flock at
`artifacts/agents/steward/narrator-digest.flock` (`digest.go:67-97`), reads
the whole file, appends, and publishes by atomic rename
(`digest.go:110-149`, `:154-194`). It never edits or deletes a line.

`scripts/agents/land.sh` is the landing gate every landing routes through
(records/goals/landing-chain-tooling.md). It refuses at four points a live
writer can trip:

1. `stage_changes` refuses when unstaged changes remain after staging
   (`land.sh:232-235`) or when untracked paths remain (`:236-241`).
2. `require_clean_after_commit` refuses when the tree is not clean after
   the commit (`:255-263`).
3. `git rebase` itself refuses to start on a dirty tree (`:269-271`,
   invoked at `:293` and inside the push-retry loop at `:311`).
4. Pathspec mode refuses a non-empty index up front (`:216-219`).

Any landing slower than the append cadence gets refused: m2 recorded three
refusals in one session, and the counselor-carriage landing (dcc44ca9)
raised the write rate by putting counselor briefs on the same file.

What already shipped, and what it actually covers: the digest-union-merge
goal (IL-34, memory/instruction-ledger.md:100) landed `merge=union` for the
digest and seven other append-only registers (`.gitattributes:2`), and the
append-only carriage evaluator classifies the registered paths at commit
time (`internal/landing/observe.go:467`,
`scripts/agents/register-carriage-paths.txt:1`,
records/two-bars/digest-merge-addendum.md). Revision 1 called the
cross-machine race "solved" by this; the critique disproved both halves of
that claim. First, the evaluator's verdict is only written into a commit
trailer — `commit.sh` keeps policy mismatches non-blocking in observation
mode (`commit.sh:286-311`) and nothing on the push path checks the
verdict, so a rewritten digest can reach origin carrying a would-refuse
trailer (LR-001; the revision-1 commit itself did exactly that). Second,
the union driver only governs merges of committed bytes, and the roughly
fifteen hand-resolved digest collisions on 2026-09-01/02 happened on
UNCOMMITTED worktree bytes the driver never sees (LR-002, §4.2). The
local race — the writer dirtying the tree while a landing is inside the
gate — is therefore not a separate second half of the problem; it is the
cause of the observed cross-machine cost. This design settles it.

Binding constraints from the goal record:

- The no-softening byte-equality law on counselor appends survives: no
  dropped or reordered bytes on any live record.
- A new role adopts the convention without touching land.sh again.
- The first migration proves it on the narrator digest.

## 2. The decision: the gated carry convention

One mechanism with five parts, each of which exists because a specific
finding proved its absence unsound:

1. **The landing gate** (§3.2) — a checkout-wide flock, held exclusively
   by `land.sh` from before its first index mutation through its last
   push, and held shared by every registered live-record writer around
   each publish. This is the missing serializer LR-007 named, and holding
   it across the whole landing is what dissolves LR-004 (no append can
   land between carry and guard) and LR-005 (the writer cannot race the
   rebase on the pathname).
2. **One carry, at gate entry** (§3.3, §3.4) — pending appends that
   accumulated before the gate was acquired are committed as one
   pure-append commit per registered path set, built through a temporary
   index that never touches the caller's index (the LR-003 fix) and
   never stages into the shared index (the LR-008 fix).
3. **Guards untouched** — `stage_changes`, `require_clean_after_commit`,
   the untracked-paths check, rebase's dirty-tree refusal, and the
   pathspec-mode empty-index refusal keep their exact bytes. They are
   the safety layer for every path, registered or not, and for any
   writer that ignores the gate.
4. **Enforced append-only registry machinery** (§3.5) — the registry
   drives the append-only shape check for every registered path (LR-006),
   and the check becomes a hard refusal at both push boundaries (LR-001).
5. **A proof plan that exercises the real machinery** (§6) — real
   `commit.sh`, real carry verb, both staging modes, cross-machine and
   crash interleavings (LR-009).

### 2.1 What this revision reverses, and why

- **The carry no longer rides `commit.sh`.** Revision 1 routed carry
  commits through the real commit wrapper and staked its reject
  condition 1 on that working. The critique proved it cannot: in
  pathspec mode the caller's changes are still unstaged when the carry
  would commit, and `commit.sh`'s LANDING-projection check refuses any
  projected working-tree byte the commit would not record
  (`commit.sh:185-193`); in staged-only mode the caller's bytes are
  already in the shared index and `commit.sh` proves and commits the
  entire index (`commit.sh:144-147`, `:312-341`), so the carry would
  swallow the caller's commit (LR-003). Per the old reject condition,
  the shape moves and the gate does not: `commit.sh` is not weakened,
  not given a carry flag, and not taught any exception. The carry gets
  its own constructor with a narrower and structurally stronger gate
  (§3.3).
- **The writer is no longer untouched.** Revision 1 rejected a landing
  lease even as a component, on three grounds: cross-machine dominance,
  an unbounded writer block, and per-writer lock-path coupling in
  land.sh. The critique dissolved the first ground (the "cross-machine"
  cost is caused by the local race, §4.2) and LR-004/LR-005 proved that
  without a writer pause the local race stays open and the rebase can
  destroy bytes outright. The second and third grounds were real and are
  answered by construction: the writer's wait has a named ceiling and a
  loud, idempotent failure (§3.2), and the gate is one fixed path owned
  by `internal/landing` — land.sh knows no writer's private lock and no
  writer knows land.sh (the digest's own flock at `digest.go:67-69` is
  untouched).
- **The bounded 5-carry retry loop is deleted.** It was a probabilistic
  answer to a race the gate now closes structurally. With writers paused
  for the whole exclusive section, a second carry pass could only ever
  find bytes from a non-compliant writer, which is a defect to surface
  (via the unchanged guard refusal), not to retry around.

### 2.2 The rejected shapes, re-examined

- **Shape (a), untracked-live with landed snapshots** — still rejected,
  unchanged from revision 1: the digest is fleet-shared human truth; the
  fold-in recreates the race; snapshot rotation trips the Stop-hook
  cursor's prefix-SHA guard (`digest.go:241-243`,
  `supervision-hook.sh:165-170`); and it rips out shipped, adopted
  machinery.
- **Shape (c), writers honoring a landing lease** — revision 1 rejected
  it wholesale; revision 2 adopts a disciplined version of it as ONE
  component (the shared side of the gate), because LR-004 and LR-005
  proved the convention unsound without it. What stays rejected is
  shape (c) as the WHOLE answer: a lease alone leaves the pending
  appends uncommitted, so the union driver still never sees them and
  the LR-002 conflict class survives. The carry is what converts local
  bytes into the committed form the union driver can merge; the gate is
  what makes the carry's result stick until the push.
- **Literal re-staging inside the gate** (the goal's sketch (b)) — still
  rejected, and LR-003 sharpened the reason: it mutates the caller's
  declared commit content, and no ordering of re-staging survives
  `commit.sh`'s whole-index proof.

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
live record is a specific path a specific writer owns, and both the carry
constructor and the append-only evaluator must be able to enumerate
exactly what they govern. The registry is read from the HEAD tree by the
engine (matching how `loadCarriagePolicy` reads the carriage allowlist,
`observe.go:476-485`), so a landing cannot smuggle a registry edit and a
rewrite past the boundary in one commit.

### 3.2 The landing gate

One flock at a fixed, engine-owned path:
`artifacts/agents/landing/gate.flock` (gitignored state, one per
checkout). Two acquisition modes, standard flock semantics
(`unix.Flock`, the same primitive `digest.go:73-92` already uses, so it
is portable to both fleet platforms):

- **Exclusive (the landing).** `land.sh` re-execs itself under a new
  engine verb as its first act, before `verify_checks`:
  `metasystem landing with-gate --root <root> -- <argv>`. The verb takes
  `LOCK_EX`, execs the child, and the lock dies with the process —
  a crashed landing self-releases (part of the LR-008 answer). This is
  the landing-wide mutex LR-007 proved missing: two `land.sh` runs in
  one checkout now serialize before either touches the index, where
  today nothing above `commit.sh`'s commit-scoped lease
  (`commit.sh:8-29`) serializes them.
- **Shared (the writers).** A registered writer takes `LOCK_SH` around
  each whole publish (read, append, atomic rename), through a small
  helper in `internal/landing` (exported for writers as the one
  sanctioned gate API). Shared acquisition means concurrent writers do
  not serialize each other beyond their own per-record locks. For the
  digest this is one change inside `internal/narratordigest`: `Append`
  and `AppendPayload` acquire the gate shared before their existing
  flock. Lock ordering is fixed and acyclic — gate first, then the
  record's own lock; the landing side takes only the gate and never any
  record's lock (the carry reads the worktree file without the writer's
  flock, which is safe because publishes are atomic renames) — so no
  deadlock is constructible.
- **The writer's wait has a named ceiling.** A writer polls
  `LOCK_SH | LOCK_NB` with a deadline of 10 minutes — sized as roughly
  twice the landing's worst case (fetch, rebase, three push attempts
  with re-fetch and re-rebase between them, `land.sh:295-313`) — and on
  expiry FAILS the append loudly with a named error and no partial
  write. Both digest entry points are idempotent on retry (`Append`
  dedups by signature, `digest.go:132-135`; `AppendPayload` by source
  marker, `digest.go:172-175`), so a caller that holds its entries and
  retries next tick loses nothing. The build slice must verify each
  caller actually surfaces the error and retries rather than discarding
  (§9 item 7); a caller that discards is a reject condition (§10.3).
  This answers revision 1's own objection to writer pauses — an
  unbounded quiet window — without reopening LR-005: the ceiling can
  only fire against a wedged landing, and a wedged landing is already a
  supervised defect.
- **What the gate does NOT cover, stated plainly.** A direct
  `commit.sh` invocation outside `land.sh` does not take the gate, so a
  human-classified direct commit racing a landing in the same checkout
  remains possible, exactly as it is today. The gate closes
  landing-versus-landing and landing-versus-writer; the
  session-versus-session seam (two agents, one checkout) is owned by
  Wido's session-coexistence goal, which this gate is designed to slot
  under: the goal may later route direct commits through the same gate,
  and nothing here needs to change for that. This design stands alone
  for one checkout with one landing at a time, which is what LR-007
  required of it.

### 3.3 The carry constructor

One engine verb, `metasystem landing carry-live-records --repo <root>`,
implemented in `internal/landing`. It runs only inside the gate's
exclusive section (it refuses if the caller does not hold the gate —
mechanically: `with-gate` exports a token the verb checks, the same
pattern as `commit.sh`'s `__lease-held` re-exec). Behavior, in order:

1. Read the registry from HEAD. Missing registry means empty registry:
   carry is a no-op and land.sh behaves byte-for-byte as today.
2. Refuse (exit 2) if any registered path appears in the caller's staged
   set (`git diff --cached --name-only`). The convention owns registered
   paths; the m2 workaround of hand-staging digest appends is retired,
   not accommodated. The refusal names the path and says "unstage it;
   the carry owns this path."
3. For each registered path, classify worktree bytes against
   `git show HEAD:<path>`:
   - identical, or path clean → skip;
   - HEAD version absent and file present → pure creation, the whole
     file is the append;
   - HEAD bytes are a strict byte prefix of worktree bytes → pending
     append, eligible;
   - anything else → refuse the whole landing, exit 2, printing the
     path, the HEAD byte length, and the first divergent byte offset.
     Never stage, never guess. A rewrite takes a bar by hand, exactly as
     the digest-merge-addendum rules.
4. If any path is eligible, build the carry commit WITHOUT touching the
   shared index: with `GIT_INDEX_FILE` pointed at a private temporary
   index, `git read-tree HEAD`, update only the eligible registered
   paths to their worktree bytes, `git write-tree`, verify the
   HEAD-to-tree diff touches ONLY registered paths and is append-only
   (the same evaluator §3.5 hardens — invoked here as a refusal, so a
   carry commit is proven append-only twice, structurally by the prefix
   classifier and independently by the evaluator), then `git
   commit-tree` with message `live-record carry: <space-separated
   paths>` and the same trailers `commit.sh` stamps: `Machine:` from the
   enrolled nickname (refusing when unenrolled, as `commit.sh:325-327`
   does) and the `Landing-Provenance` pair from a `landing observe` run
   against the carry tree with `--direct-fix register-carriage`.
   Finally advance the current branch ref to the new commit.
5. The constructor requires the checkout lease exactly as `commit.sh`
   does (`lease require-holder`); it runs under land.sh, so the holder
   is whoever lawfully runs the landing.

Why this parallel lane is not "an exception path or a weakened check":
`commit.sh`'s generic gates exist because an arbitrary commit can carry
arbitrary bytes — coverage, static re-proof, and the whole-projection
binding all defend against content no one has classified. A carry commit
is the opposite: its content is closed by construction (only registered
paths, only strict-prefix appends, proven twice, trailer-stamped, under
the same lease). The static proofs read code surfaces
(`commit.sh:121-138`); a commit that provably touches only registered
record files has no bytes those proofs bind. The narrow lane is
register-carriage's existing precedent (`land.sh:9` already names the
class) taken to its mechanical conclusion. What would violate the reject
condition is weakening `commit.sh` itself, and nothing here touches it.

Crash windows of the constructor (the LR-008 answer): it never writes
the shared index, so the stranded-staged-bytes state LR-008 described
cannot be produced by any crash point. A crash before `commit-tree`
leaves a temp file; between `commit-tree` and the ref advance, a
dangling object; after the ref advance, a completed pure-append commit
on the branch — ordinary local git state that the next landing rebases
and pushes. The gate flock self-releases with the dead process. There is
no restart marker because there is no state to restart.

### 3.4 The land.sh call sites

`land.sh` changes in exactly three registry-generic ways; the guards keep
their exact current bytes and messages:

1. **The gate wrapper.** The self-re-exec under
   `landing with-gate` (§3.2), before anything else runs.
2. **One carry call**, inside the gate, before `verify_checks`
   (`land.sh:288`). Because writers are paused for the rest of the
   exclusive section, this single call is sufficient: the post-commit
   window and the pre-rebase windows (first rebase at `:293`, retry
   rebases at `:311`) cannot re-dirty on registered paths. Revision 1's
   three call sites collapse to one, and LR-004's gap — an append after
   the carry returns but before a guard checks — is structurally empty
   for gate-compliant writers.
3. **The hard pre-push check** (§3.5), after the rebase and before each
   push attempt.

An undeclared live path gets no carry and no gate, so it refuses
precisely as today — the guard stays honest for everything outside the
registry, and the refusal experience remains the forcing function that
drives a new self-writing role to declare itself. Likewise a registered
path whose writer bypasses the gate: the guard refusal is the safety
net, loud and byte-preserving, and §8 names gate compliance as the
liveness obligation it is.

### 3.5 Enforced append-only registry machinery

Two changes turn the advisory premise into machinery:

1. **The evaluator becomes registry-driven (LR-006).** The hard-coded
   two-path switch in `registerCarriage` (`observe.go:462-471`, which
   applies `appendOnly` only to `memory/receipts.log` and
   `records/narrator-digest.log`) is replaced: a changed path gets the
   `appendOnly` shape check when it appears in `live-records.txt` (read
   from the base tree), with the two legacy paths folded into the
   registry so no path loses coverage (`memory/rulings.md` keeps its
   distinct rows-only rule). Registering a path IS what subjects it to
   the byte law; three declarative lines now confer real enforcement.
2. **The verdict gets teeth at the push boundary (LR-001).** A new
   engine check, `metasystem landing verify-outgoing --root <root>`,
   recomputes the append-only judgment for every outgoing commit
   (`HEAD` not reachable from `origin/<branch>`) whose parent-to-commit
   diff touches a registered path, and exits non-zero on any violation.
   It is called at both places bytes leave the machine: in `land.sh`
   between rebase and each push attempt, and in `commit.sh`'s
   `--push` branch (`commit.sh:346-364`) before the origin push. A
   would-refuse digest rewrite now stops at the boundary instead of
   riding a trailer to origin. `commit.sh`'s observation mode itself
   stays observational for everything else — the general
   policy-mismatch-never-stops-observe contract (`commit.sh:286-291`)
   is not narrowed; the hard check is a separate, registry-scoped,
   push-time refusal.

### 3.6 Consistency enforcement

`scripts/validate-metasystem.sh` gains one check: every path in
`live-records.txt` must (a) carry `merge=union` per `git check-attr`,
(b) be covered by `register-carriage-paths.txt`, and (c) contain no glob
characters. A registry row without all three is a validation failure, so
the declaration surfaces (registry, union attribute, carriage allowlist)
are one atomic adoption step the suite refuses to let drift apart. Gate
compliance of the writer is not statically checkable and is not
pretended to be; it is proven per-role by the specimen fixture (§6) and
named as a code obligation in §8.

## 4. Union semantics and the conflict trail

### 4.1 Union semantics (design question 2, settled)

- **Where the union lives:** in git's built-in union merge driver via
  `.gitattributes`, per path — already shipped by IL-34. The carry
  mechanism never merges anything: it only appends and refuses. The only
  three-way merges on live records happen inside `git rebase`'s merge
  backend, where the union driver governs — and the carry is what
  guarantees the driver's precondition, namely that both sides' bytes
  are committed.
- **What proves byte-preservation:** four independent layers.
  1. The carry constructor's strict-prefix classifier: bytes reach a
     carry commit only when HEAD's bytes are a verbatim prefix of the
     worktree bytes.
  2. The registry-driven append-only evaluator, invoked as a refusal
     inside the constructor (§3.3.4) and again at both push boundaries
     (§3.5.2), judging every outgoing commit independently of how it
     was made.
  3. The gate's writer pause across the rebase (§3.2): git and the
     writer never write the same pathname concurrently, which is the
     only fix for LR-005 — the interleaving where git's union result
     overwrites a writer publish (or vice versa) destroys bytes that no
     after-the-fact verification can recover, because the losing side's
     bytes exist nowhere else. The post-rebase check below is therefore
     an alarm, not the defense.
  4. A post-rebase re-verification in the carry-aware landing: after
     each rebase, every carried line must appear exactly once in the
     rebased file, in local relative order. A failure here means a
     writer bypassed the gate mid-rebase or the union path misbehaved;
     the landing refuses before pushing (exit 2, naming the path and the
     missing or duplicated line).
- **Non-append divergence:** refuse loudly, never guess — at carry time
  by the prefix classifier; at push time by `verify-outgoing`. The union
  driver cannot detect a one-sided rewrite, which is exactly why the
  refusals sit at the origin-commit and push boundaries.
- **The known residual is inherited, not created:** union merge does not
  guarantee line order inside a merged hunk, and a merge that
  interleaves remote lines before the local cursor prefix trips
  `Pending`'s loud machine-local error (`digest.go:241-243`). Carry
  commits add no new reordering — a carried tail keeps its local
  relative order, and remote lines join only through the same union
  merge that already carries the residual.

### 4.2 The conflict trail, explained honestly (LR-002)

The mechanism-level account: a merge driver merges COMMITTED blobs.
`merge=union` fires only inside the merge machinery — for this repo,
inside `land.sh`'s `git rebase`, whose default merge backend honors
in-tree attributes. The roughly fifteen hand-resolved digest collisions
of 2026-09-01/02 did not take that path. With the m2 workaround in
force ("stage live appends into each landing"), the digest was routinely
DIRTY — uncommitted in the worktree or the index — at fetch/rebase time.
Uncommitted bytes never reach the driver: rebase refuses to start on the
dirty tree, or the checkout step refuses to overwrite the locally
modified file, and the operator reconciles local worktree bytes against
incoming committed bytes BY HAND, outside any merge machinery. Two
further paths bypass the driver entirely and may account for some of the
trail: `git rebase --apply` (and tools that select the apply backend),
which patches without the merge machinery, and stash-based dirty-tree
workarounds whose restore conflicts were resolved manually. The gated
carry closes the dominant class by construction: pending appends become
commits before the rebase, so both sides of every digest merge are
committed and the union driver's precondition finally holds.

Stated plainly, as the critique's declared gap requires: the repository
retains no transcripts of the fifteen resolutions, so the event-level
classification (dirty-tree hand-merges versus apply-backend versus
stash restores versus a miscount) is OPEN and this design does not
pretend to close it. What the design claims is the mechanism-level
account above, and it names its falsifier as a reject condition
(§10.5): if hand digest resolutions recur on land.sh landings after
fleet adoption — that is, on committed-versus-committed merges with the
carry in force — the account is wrong and the cross-machine analysis
reopens.

## 5. Interleavings and crash windows

| Interleaving | Outcome under the gated carry convention |
| --- | --- |
| Writer attempts a publish while a landing holds the gate | The writer blocks on `LOCK_SH` and publishes after the landing releases; entries are delayed one landing, never lost. If the landing wedges past the 10-minute ceiling, the writer fails loudly and idempotently retries next tick (§3.2). |
| Writer appends between gate acquisition and the carry | Impossible for gate-compliant writers: the exclusive lock is already held. This is LR-004's gap, structurally empty. |
| Non-compliant writer (or undeclared path) dirties mid-landing | The unchanged guards refuse exactly as today — loud, byte-preserving, naming the path. Safety never depended on writer compliance; only refusal-free landings do (§8). |
| Writer publishes during git rebase | Impossible for gate-compliant writers — this is LR-005's byte-destruction interleaving, and the pause is its only sound fix (§4.1.3). The post-rebase re-verification (§4.1.4) alarms on a bypass. |
| Two landings race, same checkout | Serialized by the gate's exclusive lock before either touches the index (LR-007). The second landing waits, then sees the first's pushed state after its own fetch/rebase. |
| Two landings race, different machines | Both machines' carry commits are pure appends; the loser's push is rejected, land.sh's fetch/rebase/push retry (`land.sh:295-313`) reruns, and the union driver merges committed appends. No hand resolution — and unlike revision 1, the pre-conditions of this claim (bytes committed on both sides, writer quiet during rebase) are now enforced rather than assumed. |
| Direct `commit.sh` races a landing, same checkout | Open residual, named in §3.2: the gate does not cover direct commits. Owned by the session-coexistence goal seam. |
| Landing dies holding the gate | The flock dies with the process; writers unblock. Whatever the carry completed is ordinary local git state (pure-append commits on the branch); the next landing rebases and pushes them. |
| Landing dies inside the carry constructor | No shared-index writes exist to strand (LR-008): before `commit-tree`, a temp file; before the ref advance, a dangling object; after it, an ordinary commit. |
| Landing dies mid-rebase | Pre-existing recovery story (operator resolves or aborts the rebase), unchanged; carry commits behave like any other local commits. |
| Caller's commit refused after the carry committed | The carry commit stands: a harmless pure-append landing-in-waiting that rides the next successful landing. Deliberately not rolled back — rollback would rewrite a commit whose bytes are already law. |

## 6. Migration and proof: the narrator digest as first specimen

- **Registry:** add the one row for `records/narrator-digest.log`; fold
  the two legacy hard-coded paths into the registry per §3.5.1. Union
  attribute (`.gitattributes:2`) and carriage row
  (`register-carriage-paths.txt:1`) already exist, so §3.6 passes on
  day one.
- **Writer:** `internal/narratordigest` gains the shared-gate
  acquisition with the named ceiling in `Append` and `AppendPayload`
  (§3.2) — the one writer change this revision accepts, with the
  reversal owned in §2.1. Flock, read-modify-write, and atomic rename
  stay exactly as shipped. Callers' retry-on-error behavior is verified
  in the build slice (§9.7).
- **Proof plan (LR-009's fold).** The existing `land-fixtures.sh`
  deliberately reduces `commit.sh` to a stub (`land-fixtures.sh:4-7`,
  `:46-61`), which is precisely why revision 1's plan could not detect
  the LR-003 incompatibility. The specimen legs therefore run in a NEW
  fixture family, `live-records-fixtures.sh`, that uses the REAL
  `commit.sh` through its supported non-Go adopted-checkout lane
  (`commit.sh:158-171`: the fast gate self-skips on a non-Go seed and
  the bundled engine owns policy), the real carry verb, the real gate,
  and real local bare remotes. If that lane cannot in fact host the
  real wrapper in a seed repo, that is reject condition §10.6, not a
  cue to stub. Legs, under the standard fixture-budget ceilings:
  1. *Dirty-before-landing, pathspec mode:* pending digest appends;
     landing succeeds; assert one `live-record carry:` commit exists
     with Machine and provenance trailers, the caller's commit contains
     none of the digest bytes, the tree is clean, and landed digest
     bytes equal the pre-landing worktree bytes.
  2. *Dirty-before-landing, staged-only mode:* same assertions; also
     assert the caller's staged set commits intact (the LR-003
     staged-only failure mode, now impossible by construction).
  3. *Registered path staged by the caller:* assert the carry refuses
     (exit 2) naming the path, and nothing was committed.
  4. *Writer during the landing, compliant:* a fixture writer process
     using the shared-gate helper attempts a publish while the landing
     holds the gate; assert it blocks, the landing completes with no
     refusal, and the writer's bytes land in the NEXT landing's carry.
     Deterministic via lock acquisition ordering — no production test
     hooks.
  5. *Writer during the landing, non-compliant:* a raw writer appends
     mid-landing (via a stubbed inner step, as revision 1's leg 2 did);
     assert the unchanged guard refuses with today's exact message and
     no bytes are lost from the worktree file.
  6. *Two machines, same path:* two clones both carry digest appends;
     loser's push rejected; assert the retry rebase union-merges and
     the final origin file contains every byte from both machines, each
     exactly once (the cross-machine leg LR-009 found missing).
  7. *Non-append divergence:* an edited committed line; assert carry
     refuses (exit 2) naming path and offset; separately, a
     hand-crafted rewriting commit; assert `verify-outgoing` refuses
     the push at both call sites (the LR-001 enforcement leg).
  8. *Crash legs:* kill the carry between `commit-tree` and the ref
     advance (assert: branch unmoved, next landing clean); kill the
     landing after the carry's ref advance, before push (assert: next
     landing pushes the carry commit); assert the gate flock is free
     after each kill (the LR-008 legs).
  9. *Byte-preservation property:* in legs 1, 2, 4, and 6, assert
     `git show HEAD:records/narrator-digest.log` is a byte prefix of
     the worktree file and contains every fixture-emitted byte in
     emission order (per machine, in leg 6).
  10. Go unit tests in `internal/landing` for the prefix classifier
     (identical / creation / strict prefix / edit / truncation / binary
     garbage), the registry parser (glob refusal), the registry-driven
     evaluator selection, and `verify-outgoing` commit enumeration.

## 7. Blast radius (design question 5)

Every current consumer of the digest file:

1. Writers, all through `internal/narratordigest`: steward tick
   narration (`internal/steward/narrate.go:94`), counselor brief
   carriage (`counselor_carriage.go:136`, `:182` — the raw-payload path
   the byte-equality law protects), ruling sweep (`ruling_sweep.go`).
   The package gains the shared-gate acquisition (§3.2); callers gain a
   verified retry obligation (§9.7). This is a change from revision 1,
   owned in §2.1.
2. Readers: `narratordigest.Pending` and `Advance` via
   `metasystem steward digest-pending` / `digest-advance`
   (`cmd/metasystem/steward_verbs.go:188`, `:210`), consumed by the
   Stop hook (`supervision-hook.sh:165-170`). Unchanged; §4.1 names the
   inherited union-reorder residual against the cursor guard.
3. The append-only carriage evaluator (`observe.go:445-474`) and its
   tests (`observe_test.go:38`, `:353`, `:429`, `:478`): the hard-coded
   switch becomes registry-driven (§3.5.1); existing tests keep passing
   because the legacy paths move into the registry.
4. `commit.sh`: one addition in the `--push` branch
   (`verify-outgoing`, §3.5.2). The observation contract and every
   other gate are byte-identical.
5. Declaration rows: `register-carriage-paths.txt:1`,
   `.gitattributes:2`, and the new `live-records.txt`.
6. Fixtures that write the digest directly:
   `scripts/agents/supervision-hook-fixtures.sh:149` (unaffected: they
   do not land).

Every current consumer of land.sh's clean-tree guard:

1. Every fleet landing on every seat — behavior changes only for
   registered paths (carry instead of refusal) plus the new gate
   serialization and the pre-push append-only refusal; guard bytes and
   messages are unchanged.
2. `scripts/agents/land-fixtures.sh` — its three legs keep passing
   (guards unchanged); the reduced commit stub stays confined to it,
   with the specimen proofs living in the new real-wrapper family (§6).
3. `scripts/validate-metasystem.sh:959` (enumeration) and `:1072`
   (`bash -n`) — extended by §3.6.
4. The rebase machinery's precondition — "rebase never starts on a
   dirty tree" is preserved: the carry plus the writer pause make the
   tree actually clean, and nothing is exempted.
5. The two-bars observe/proof machinery in `commit.sh` — untouched for
   caller commits; carry commits carry their own narrower structural
   proof (§3.3) and the same trailers.
6. The m2 handoff's codified workaround ("stage live appends into each
   landing") — superseded AND now refused (§3.3.2); the handoff note
   is updated when the build lands.

## 8. Adoption path for a new self-writing role (design question 3)

What three declarative lines confer (the registry row, the
`merge=union` attribute, the carriage-allowlist row — §3.6 refuses a
partial adoption): the BYTE LAW, enforced. The moment the path is
registered, every commit touching it is judged append-only by the
registry-driven evaluator, every carry of it is prefix-proven, and every
push of a violating commit is refused at both boundaries. This is the
LR-006 fold: enforcement no longer depends on being named in a
hard-coded switch.

What writer code must additionally do (obligations, not declarations):
publish whole-file states atomically (rename), append-only, and acquire
the landing gate shared around each publish via the `internal/landing`
helper. Gate compliance buys LIVENESS — refusal-free landings and
protection from the rebase interleaving. A non-compliant writer's role
still cannot break the byte law (the evaluator and guards hold); it
will simply keep experiencing today's refusals until it complies, and
the specimen fixture leg (§6.4) is how a role proves compliance once.
land.sh is not touched; the engine verbs are not touched; the fixture
family gains one row-parameterized case.

## 9. Build slice (bounded, per the goal's one 4h build slice)

1. `internal/landing`: gate helper (exclusive + shared with ceiling),
   prefix classifier, carry constructor, registry parser,
   registry-driven `appendOnly` selection, `verify-outgoing`; unit
   tests.
2. `cmd/metasystem`: `landing with-gate`, `landing carry-live-records`,
   `landing verify-outgoing`.
3. `scripts/agents/live-records.txt` with the digest row and the two
   folded legacy rows.
4. `land.sh`: the gate re-exec, one carry call, the pre-push
   `verify-outgoing` call. No guard edits.
5. `commit.sh`: the `--push`-branch `verify-outgoing` call. Nothing
   else.
6. `internal/narratordigest`: shared-gate acquisition in `Append` and
   `AppendPayload`.
7. Verify every digest caller (`narrate.go`, `counselor_carriage.go`,
   `ruling_sweep.go`) surfaces gate-ceiling errors and retries
   idempotently; fix any that discard, or stop and reopen (§10.3).
8. `validate-metasystem.sh`: the §3.6 consistency check.
9. `live-records-fixtures.sh`: legs 1–10 of §6.
10. Handoff-note update retiring (and now refusing) the manual
    workaround.

## 10. Self-grade and reject conditions

**Grade: B+.** The four critical findings are dissolved by one
mechanism rather than four patches, and every claim the critique broke
is either repaired with file-and-line grounding or explicitly left open
with a falsifier. Honest debits keep this below revision 1's
(overclaimed) A−: the writer is no longer untouched, and every future
registered writer inherits a code obligation the registry cannot
statically check; the carry commit travels a parallel commit lane whose
safety case, while structurally argued, is new machinery the fixtures
must actually earn; the direct-commit-during-landing hazard and the
event-level LR-002 classification remain open by name; and the writer
ceiling constant is a sized judgment, not a proof.

**Reject this design if any of the following holds at build or review
time — plainly: any one of these seven being true means this revision is
wrong and reopens, with no quiet tuning, no stub, and no gate
weakened:**

1. The carry constructor cannot prove, at commit-construction time,
   that its commit touches only registered paths and is append-only by
   both the prefix classifier and the evaluator — or anyone proposes
   teaching `commit.sh` a carry exception instead. The shape is wrong,
   not the gate.
2. The compliant-writer fixture leg (§6.4) cannot be made deterministic
   without a test hook in production `land.sh`, `commit.sh`, or
   `internal/narratordigest` bytes.
3. Any digest caller is found to discard entries on a gate-ceiling
   error and cannot be given idempotent retry without redesigning its
   cadence — the pause would then be able to lose narration, which the
   byte law forbids.
4. In the first fleet week, the writer's 10-minute ceiling fires
   against a healthy (non-wedged) landing — that falsifies the sizing
   judgment, and the answer is reopening (event-coupled gating or a
   smaller exclusive section), not raising the constant silently.
5. After fleet adoption, hand digest resolutions recur on
   committed-versus-committed merges through land.sh — that falsifies
   §4.2's mechanism-level account of the conflict trail, and the
   cross-machine analysis reopens.
6. The real-`commit.sh` fixture lane (§6) cannot host the true wrapper
   in a seed checkout — the proof plan would then be stubbing the
   mechanism under test again, which is the exact LR-009 defect this
   revision exists to remove.
7. Byte-preservation (§6.9) can be made to fail: any fixture-emitted
   byte missing, duplicated, or reordered in the landed file indicts
   the prefix classifier or the gate, and with either, the whole
   convention.
