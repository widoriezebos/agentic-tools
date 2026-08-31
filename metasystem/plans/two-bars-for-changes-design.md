# Two bars for changes — landing provenance design

- Goal: two-bars-for-changes (plans/goals/two-bars-for-changes.md, revision 4)
- Status: design, round 1 — awaiting design critique
- Mode: design only; no code in this change
- Carries: the D90 human ruling (accidental threat model) and the surviving
  decisions of the earlier r3 design record (records/two-bars/two-bars-design.md);
  every carried or superseded decision is dispositioned in §10, per R-25b-m1.

## 0. The whole mechanism in one page (R-11)

Every landing from an agent-held checkout must present exactly one of:

- **Bar (a) — the loop.** `--chain <root-job-id>`: the landing binds to a
  CLOSED delegated chain. The engine verifies the chain is closed (which
  already proves the per-hazard-class critique duties), resolves the chain's
  reviewed candidate TREE digest, and checks byte-for-byte that every staged
  path the chain changed carries exactly the blob the reviewed tree carries.
  The commit is stamped `Landing-Provenance: chain=<root-id> tree=<40-hex>`.
- **Bar (b) — the declared direct fix.** `--direct-fix <class>`: the landing
  declares a typed class (initially: `register-carriage`, `prose-docs`,
  `revert-exact`, `mechanical-defect`). The engine checks the staged paths
  against the class's path rule and the NEVER-DIRECT-FIX floor (the
  instruction-bearing path list plus the two-bars enforcement surface itself,
  read from both the base and the candidate tree). The commit is stamped
  `Landing-Provenance: direct-fix class=<class> ...`.
- **Bar (c) — refusal.** Neither declared, or a declaration that fails its
  check: the commit refuses with a message naming both bars and the exact
  offending paths. `loop` is never refused; only `direct-fix` is challenged;
  the cheap way out of doubt is the loop.

Verification lives in one new engine verb called from `commit.sh` — the one
chokepoint every agent commit already must pass (the pre-commit guard refuses
agent raw `git commit`), and the place the tree is already proved. `land.sh`
re-checks bar (a)'s blob binding after each rebase, because a rebase onto a
moving origin can merge concurrent edits into a reviewed file. Register
carriage (the narrator digest and its kin) rides free on every landing via a
small allowlist, so digest-only landings stay one flag. Direct fixes
aggregate in git history itself; `metasystem report direct-fixes` tallies
them for Wido's review. Rollout is observe-then-enforce via one committed
config key, so four machines adopt without a flag day. Humans stay sovereign
(D90: the model is the honest agent forgetting, not the hostile one).

## 1. Threat model: what the seat can do today

Evidence against the current bytes of this worktree (branch from main):

1. **Ordinary landing never consults a chain.** `scripts/agents/land.sh`
   runs, in order: `verify_checks`, `stage_changes`, `commit_changes`,
   `require_clean_after_commit`, `fetch_origin`, `rebase_origin`
   (land.sh:267-272), then a bounded push-retry loop (land.sh:274-292) and
   the transport sync (land.sh:294-296). No step reads a job record,
   a chain, or a review artifact.
2. **The commit wrapper proves tree identity, not provenance.**
   `scripts/agents/commit.sh` captures the index as `proved_tree`
   (commit.sh:106-109), runs the static re-proof and audit, re-proves the
   index unchanged (commit.sh:235-245), commits with the machine trailer
   (commit.sh:260-263), and rolls back if the landed tree differs from the
   proved tree (commit.sh:264-273). This binds the commit to the *locally
   checked index* — the seat's own unexamined bytes qualify.
3. **The closure gate is real but disconnected from landing.**
   `dispatch.sh close` (scripts/agents/dispatch.sh:2054-2104) runs
   `job close-check` (dispatch.sh:2092) → `CloseCheck`
   (internal/dispatch/close.go:15-127) → `validateHazardCompletion`
   (internal/dispatch/hazard.go:165-214), which for DESIGN-BEARING and
   DESTRUCTIVE-REACH chains requires a completed, fresh-session,
   distinct-model independent-critique job over the exact final work round
   (hazard.go:261-302) per the obligations table (hazard.go:36-52). But
   close only patches `{"chainClosed":true}` onto the root job record
   (dispatch.sh:2094-2101). Nothing downstream consumes it: a landing
   after close is indistinguishable from a landing without any chain.
4. **The candidate digest already exists and is already durable.** The
   conformance review stage snapshots the implementer worktree as a git
   TREE hash — `reviewedTree` — and persists it with the diff artifact in
   `artifacts/agents/<root>/rounds/<n>/review.json`
   (internal/validate/conformance.go:301-453, tree at :303, written at
   :439-451). The merge stage binds the closed code-critic chain to the
   final committed tree (conformance.go:468-479, critic tree equality at
   :1024-1028), and close-check verifies the mirrored `diff.patch` content
   digest (close.go:57-83). So an examined candidate is already named by a
   content digest that survives rebasing — nothing reads it at landing.
5. **The seat's raw `git commit` is already refused; the wrapper is the
   chokepoint.** The pre-commit guard classifies the caller and, for any
   non-HUMAN class, requires the live commit-wrapper ancestry token
   (scripts/agents/pre-commit-guard.sh:31-50), which only `commit.sh`
   mints (commit.sh:53-59). So for an honest agent, every commit — landing
   or direct — flows through `commit.sh`. That is where the two bars bolt on.
6. **Existing refusals prove the pattern.** The landing driver already
   refuses bare rulings ids (land.sh:131-181, invoked at :193) and
   unacknowledged new plan files (land.sh:260-265 grants the flag;
   pre-commit-guard.sh:70 consumes it). The two bars are one more refusal
   of the same species, at the same boundary.

The prohibited combination, concretely: the lease-holding seat can author
bytes, stage them (land.sh:205-224), pass gates that check only static
health and tree identity, and push to shared main (land.sh:274-292) — with
no mechanical trace that anyone or anything examined the content.
plans/seat-governance-record.md:12-39 records this as the record's foremost
open item and names this design as the specified remedy.

## 2. The provenance object

One machine-stamped commit trailer, in the grammar of the existing
`Machine:` trailer (commit.sh:260-263), never typed by an author:

```
Landing-Provenance: chain=<root-job-id> tree=<40-hex-reviewedTree>
Landing-Provenance: direct-fix class=<class-id> [defect="<one line>"] [revert-of=<40-hex-commit>]
```

- Exactly one `Landing-Provenance` trailer per commit. `commit.sh` refuses
  a message argument that already contains one (same scan pattern as the
  claude.ac refusal, commit.sh:65-70): the trailer is engine output, not
  caller input — a hand-written trailer is a forgery even when honest.
- The caller supplies only flags: `--chain <root-id>` XOR
  `--direct-fix <class>`, plus `--defect <text>` (required for
  `mechanical-defect`) or `--revert-of <sha>` (required for
  `revert-exact`). Both `land.sh` and `commit.sh` accept them; `land.sh`
  passes them through to `commit_changes` (today's pass-through shape,
  land.sh:226-232).
- The `tree=` value is resolved by the engine from the chain's own
  review.json, never from the caller. The trailer is therefore a durable,
  greppable audit record: the claim and the digest travel with the commit
  through every rebase and onto both remotes.

Why a trailer and not a side file: the commit is the unit being governed;
a side file can desynchronize from history, needs its own carriage rules,
and duplicates what git already transports. The job records and mirrored
review artifacts remain the deep evidence; the trailer is the join key.

## 3. Bar (a): binding to a closed chain

New engine verb — working name `metasystem job landing-check` — owned by
`internal/dispatch` (which already owns job records, chain membership, and
close semantics). Called by `commit.sh` after the settled-tree re-proof
(commit.sh:235-245) and before `git commit` (commit.sh:263), receiving the
already-proved index tree. Checks, in order, each with its own refusal:

1. **Chain exists and is closed.** The root job record exists in this
   machine's `artifacts/agents/jobs/` and carries `chainClosed: true` —
   the flag only `close_chain` writes after `close-check` passed
   (dispatch.sh:2092-2101). Closure already proved the hazard-class
   critique duties (§1.3), so this design deliberately re-proves nothing
   about critique: it extends the closure gate instead of paralleling it.
2. **Candidate digest resolves.** The highest implementer round under
   `artifacts/agents/<root>/rounds/<n>/` has `review.json`; its
   `reviewedTree` is the candidate digest; its `diff.patch` (whose content
   digest close-check verified against the durable manifest,
   close.go:57-83) yields the chain's changed-path set. Both artifacts
   were written by the host's conformance run, not by the delegate. The
   reviewedTree object is readable from the seat's checkout because
   delegate worktrees share the repository's one object database.
3. **Byte binding.** Let `chainPaths` be the changed-path set. The staged
   change set (index vs HEAD, computed from the proved tree) is
   partitioned: every path must be in `chainPaths` or on the carriage
   allowlist (§5). For every staged path in `chainPaths`, the staged blob
   OID must equal `reviewedTree`'s blob OID at that path (project-prefix
   mapping exactly as conformance's `projectWorkspace` does it,
   conformance.go:260-261); a path the chain deleted must be absent from
   the index. Any mismatch names the path and refuses. Mode/type changes
   compare the full index entry (mode + OID), not the OID alone.
   A landing may bind fewer than all `chainPaths` only as a refusal —
   partial landings of a reviewed candidate are not a thing; the reviewed
   diff lands whole or not at all.
4. **One landing per chain.** After the push succeeds, the driver patches
   `{"landedCommit":"<sha>"}` onto the root record via the existing
   `__record-cas` path (the `chainClosed` patch's mechanism,
   dispatch.sh:2094-2101). `landing-check` refuses a chain that already
   carries `landedCommit`. This makes replaying one examined digest under
   two different landings impossible by construction.

**Rebase behavior (the fleet's moving tip).** The digest is a TREE/content
digest, so it is rebase-stable by nature — but a rebase can still change
landed bytes: when origin moved inside a file the chain also touched, the
rebase textually merges and the resulting blob differs from the reviewed
blob. Therefore `land.sh` re-runs the blob-binding comparison (check 3,
against HEAD instead of the index) after `rebase_origin` and after every
in-loop rebase retry (land.sh:272, 289-290), before the corresponding push
attempt. On mismatch it refuses the push, names the diverged paths, and
names the remedy: the rebased content is no longer the examined content —
dispatch a follow-up round on the current tip, re-run conformance and the
required critique, close, and land again. This is the explicit re-bind
rule: rebasing never silently re-binds; it either preserves the reviewed
blobs or reopens examination. Direct-fix landings need no post-rebase
re-check: their constraints are path-shaped and a rebase does not add
paths to the commit's own change set (`revert-exact` is verified at commit
time only; §4 records this bound honestly).

## 4. Bar (b): the typed direct-fix classes

**The generating rule.** A class is lawful only if it is listed in the
class manifest AND its path rule is disjoint from the NEVER-DIRECT-FIX
floor. The floor is generated, not hand-curated: it is (i) every entry of
`scripts/agents/instruction-bearing-paths.txt` plus the runtimes' declared
instruction files — the list the merge-stage waiver already refuses to
waive (conformance.go:483-530) — and (ii) the two-bars enforcement surface
itself: `scripts/agents/commit.sh`, `scripts/agents/land.sh`,
`scripts/agents/pre-commit-guard.sh`, `internal/dispatch/` (close, hazard,
and the new landing-check), `internal/governance/`, the class manifest,
the carriage allowlist, and the `landing.provenance` key in
`metasystem.conf`. The floor is evaluated from BOTH trees — the base
(HEAD) and the candidate (the proved index) — so a direct fix cannot
delete its own floor entry or the manifest in the same commit (carried
from the r3 design, finding TB-R1-01).

**Initial classes** (the manifest ships with exactly these four; each row
is `id | path rule | required declaration fields`):

| id | path rule | required fields |
| --- | --- | --- |
| `register-carriage` | every staged path is on the carriage allowlist (§5) | none |
| `prose-docs` | every staged path ends `.md` and none is on the floor | none |
| `revert-exact` | staged change set is the byte-exact inverse of the named landed commit's change set (engine applies the inverse diff and compares trees); floor paths still refuse | `revert-of=<sha>` of a commit reachable from origin's branch |
| `mechanical-defect` | no staged path is on the floor | `defect="<one line naming what was broken>"` |

`mechanical-defect` is deliberately the goal's "complement of the
never-direct-fix list": any non-floor path may take it, declared. It is
also deliberately the widest laundering surface, so it carries the audit
weight: it is the class the report (§7) exists to watch, and recurrence in
one area is the promotion trigger severity-tiered-rigor's near-miss
register already defines. The r3 design's defect-identity growth fuse is
NOT built in this slice; §10 dispositions that openly.

**Who may extend the list and how.** The manifest and the allowlist sit on
the floor, so no direct fix can touch them. Extending, widening, or
removing a class therefore takes bar (a) — a delegated design-bearing
chain — AND a recorded human word (a rulings row naming the class), the
same two-key shape the seat record uses for promotion (Law 2,
plans/seat-governance-record.md:192-195). The seat may draft the change;
it cannot enact it alone.

**Honest bounds.** The classes are path-shaped, not semantic: they cannot
tell a design change confined to a non-floor path from a mechanical one.
The floor plus the visible declaration plus aggregation is the accidental
model's answer (D90): an honest seat cannot *forget* to choose a bar, and
a wrong choice is a named, greppable act — not an invisible one.

## 5. Register carriage: the standing exemption

Fleet fact: the steward's narrator digest — written by a background process
into `narrator-digest.log` under the records state directory
(internal/narratordigest/digest.go:59-61) — rides almost every landing,
and register-only landings must stay one flag. So a small, explicit
carriage allowlist (`scripts/agents/register-carriage-paths.txt`, seeded
with the narrator digest log, `memory/rulings.md`, `memory/findings.md`,
and the handoff-note paths `plans/*.md` — the registers the custodian role
carries, plans/seat-governance-record.md:99-105) is exempt from both bars'
path partitioning: carriage paths may ride on ANY landing, bar (a) or (b),
without being chain-bound or class-constrained. A landing that stages
*only* carriage paths declares `--direct-fix register-carriage`. The
allowlist file itself is on the floor. `plans/goals/**` needs no entry:
the goal-ledger fence already refuses it in ordinary commits
(pre-commit-guard.sh:53-60 region). Carriage stays content-unverified by
design: registers are machine-local running state no chain ever reviewed,
and their own guards (rulings id mint check, land.sh:131-181; goal-ledger
fence) remain in force.

## 6. Bar (c): refusal

In enforce mode, an agent-classified commit with no declaration, two
declarations, or a failed check refuses before `git commit` runs (so the
existing no-rollback-needed property holds; nothing to unwind). The
operator-visible message, verbatim shape:

```
landing refused: no provenance — every landing takes one of two bars
  (a) --chain <root-job-id>     bind this landing to a CLOSED delegated chain
  (b) --direct-fix <class>      declare a typed direct fix
      classes: register-carriage | prose-docs | revert-exact | mechanical-defect
  neither fits? the change takes the loop: dispatch it, close the chain, land with --chain
```

A failed check keeps the header `landing refused:` and appends the exact
cause with paths, e.g. `chain q...z is not closed`, `staged path X differs
from reviewed tree <digest>`, `class prose-docs does not cover path Y`,
`chain q...z already landed as <sha>`, `path Z is on the never-direct-fix
floor`. Every refusal is one screen, names the offending object, and names
the lawful next step. Human-classified callers (lease classify, the same
classification pre-commit-guard.sh:31-39 uses) are never refused —
sovereignty per D90 — but a human commit through `commit.sh` still gets
the trailer stamped when flags are present.

## 7. Audit surface

The aggregate IS git history plus one reader:

- `metasystem report direct-fixes [--since <date>]` scans
  `git log origin/<branch>` for `Landing-Provenance: direct-fix`, and
  prints per-class and per-machine counts (the `Machine:` trailer is
  already on every landing, commit.sh:256-263) plus one line per commit
  (sha, class, defect field, subject). The same scan cross-checks bar (a):
  a `chain=` trailer whose root record is missing or whose `tree=` digest
  mismatches the record is reported as an integrity finding.
- Readers: Wido at the review cadence (the record's owner section names
  him, plans/seat-governance-record.md:215-218), and the retro. A repeated
  `mechanical-defect` cluster on one subsystem is the input
  severity-tiered-rigor's near-miss auto-promotion consumes.
- No new ledger file. A parallel append-only landing ledger was considered
  and rejected (§9.3).

## 8. Enforcement point, and migration for four machines

**Where.** The decision check runs inside `commit.sh` as an engine verb
call — not in `land.sh`, and not as shell logic:

- `commit.sh` is the guard-backed chokepoint (§1.5): enforcement there
  covers landings via `land.sh`, direct `commit.sh --push` landings, and
  plain wrapper commits later pushed. Enforcement only in `land.sh` would
  govern one caller of the commit boundary instead of the boundary.
- The check's logic lives in the engine (`internal/dispatch`), so
  land-verb-pruning slice 1 — `metasystem land` internalizing the land.sh
  chain with land.sh as thin caller, commit.sh staying an owner
  (plans/goals/land-verb-pruning.md:6) — moves only call sites, not the
  rule. The post-rebase re-check (§3) is likewise one engine verb `land.sh`
  invokes today and `metasystem land` invokes natively after pruning.
- Scope: the boundary governs the seat's checkout — the lease-held commit
  path. Delegate worktree commits never push (adapters own that) and are
  governed upstream by conformance; they are the *material* bar (a) binds,
  not landings themselves.

**Migration (no flag day).** One committed config key,
`landing.provenance = observe | enforce`, default `observe`:

1. Land the mechanism with `observe` as the default. In observe mode the
   verb runs every check, stamps the trailer when flags are present, and
   on a would-refuse prints one `landing-provenance would refuse: ...`
   line — a would-refuse record in the DRAFT/OBSERVE grammar the
   governance states already use (plans/seat-governance-record.md:192-195).
   Nothing lands differently; each machine picks the mechanism up at its
   next fetch-rebase cycle (every landing rebases onto origin first,
   land.sh:271-272).
2. Run observe across the fleet until each machine has produced clean
   observed landings of all three shapes it uses (chain, direct-fix,
   register-carriage). The report verb over the observe window is the
   evidence.
3. Flip to `enforce` with a one-line config commit carrying Wido's
   recorded word (promotion is human-authorized; the flip is the LIMITED/
   ENFORCED analogue). The flip propagates like any commit; a machine
   enforces from the moment its checkout contains the flip. The skew
   window is bounded and safe in both directions: a not-yet-updated
   machine behaves exactly as today (the status quo, not a breakage), and
   an updated machine refuses only its own future landings with an
   actionable message. No landing ever fails because a *peer* machine is
   behind, since all checks read machine-local state only.

Rollback, if enforce proves wrong in practice: flip the key back by the
same human-word commit. The key is a promotion switch at a true external
boundary, not a dual code path; the observe branch is the enforce branch
minus the exit code.

## 9. Rejected alternatives

1. **Bind to a commit id instead of a tree/content digest.** A rebase onto
   the moving origin rewrites every commit id by construction; four
   machines pushing to one main guarantees rebases. Commit-id binding
   would refuse nearly every real landing or force no-rebase pushes.
   Rejected; the candidate digest is the tree `reviewedTree` plus per-path
   blob comparison, with the post-rebase re-check as the explicit re-bind
   rule (§3).
2. **Remote enforcement (CI or branch protection verifying provenance on
   push).** This is the adversarial model's machinery: it needs durable
   agent identity and cannot read machine-local job records, and D90
   explicitly ruled the accidental model with the adversarial tier as a
   separate future goal (records/two-bars/two-bars-design.md:78-109).
   Rejected here; recorded as the known escalation path, unchanged.
3. **A parallel provenance ledger (append-only landings file or signed
   receipts).** Duplicates what the commit trailer plus the job record
   already say, adds a second writer to every landing (a new concurrent-
   write surface across four machines), and violates the stated constraint
   to extend the existing closure-gate digests rather than invent a
   parallel system. Rejected; the trailer is the join key, the job record
   and mirrored review artifacts are the depth.
4. **Enforce in `land.sh` only.** Leaves `commit.sh --push` and
   commit-then-push-later ungoverned, and dies at land-verb-pruning when
   land.sh becomes a thin shim. Rejected; the boundary is the commit
   wrapper, which the pre-commit guard already makes unavoidable for
   agents (§1.5).

## 10. Relation to the carried r3 design (R-25b dispositions)

The earlier design record (records/two-bars/two-bars-design.md, status r3)
predates goal revision 4, which reshaped the mechanism around the closure
gate's existing digests. Nothing here is a silent weakening; each decision
is dispositioned:

- **Carried unchanged:** the D90 accidental threat model and human
  sovereignty; fail-closed with `loop` never refused; the never-direct-fix
  floor as a conservative FLOOR read from both trees (TB-R1-01); the
  anti-bureaucracy bar (the common case is one flag, the trailer is
  machine-stamped, TB-R1-07); emergency = human-personal, no agent
  override (TB-R1-06); enforcement at the composed commit boundary rather
  than message parsing (TB-R1-02 — today satisfied by the pre-commit
  guard's wrapper-token requirement plus the wrapper verb, which is
  stronger than the hook pair the r3 text predates).
- **Superseded by goal revision 4, strictly stronger:** `Design-Chain:`
  string references and the bespoke gate-witness machinery are replaced by
  binding to the closure gate's own artifacts — `chainClosed`,
  `reviewedTree`, and the digest-verified `diff.patch` (§1.3-1.4). The r3
  witness existed because no candidate digest did; one exists now, with a
  critique-duty gate already in front of it.
- **Deferred, openly:** the defect-identity growth fuse (TB-R1-04). The
  reshaped bar (b) has no general "small code fix under budget" class to
  game by splitting — `mechanical-defect` is unbounded in size but fully
  visible and aggregated, and the floor keeps every contract surface out.
  The residual risk (a design change laundered across several declared
  mechanical-defect commits on non-floor paths) is real, is surfaced by
  §7's report, and its named mechanical guard is severity-tiered-rigor's
  recurrence promotion. If the observe window or the first review shows
  clustering, the fuse returns as its own slice. Deviation recorded here
  per design-principles §Deviations.

**Fit with the proof-pricing trio.** small-change-lane's micro-dispatch
produces a closed single-round chain — that is bar (a) unchanged; nothing
here special-cases it. If that design later wants a declared lane instead,
the class manifest row shape (`id | path rule | required fields`) is the
extension point it rides on, through the §4 extension rule.
severity-tiered-rigor governs rigor *inside* chains; two-bars consumes
closure regardless of the rigor that produced it — orthogonal by
construction. The existing `prose-under-30` merge waiver
(conformance.go:574-599) is inside bar (a)'s chain machinery and is
untouched by this design.

## 11. Self-grade (R-24)

- **Confidence:** high that the mechanism closes the named hole — after
  enforce, no agent landing reaches origin without either a closed chain's
  byte-bound candidate or a typed, floor-checked, permanently visible
  declaration; the refusal is the only third state. High on the line-level
  threat model (every claim in §1 was read from the current worktree
  bytes). Moderate on two operational points: (1) how often the
  post-rebase blob re-check fires in real fleet traffic (same-file
  concurrency on code paths is believed rare because registers are
  carriage-exempt, but this is a belief the observe window must measure);
  (2) whether four initial classes are the right partition — the observe
  window's would-refuse lines are the evidence for adding or splitting
  classes before enforce.
- **The weakest claim, declared:** that `mechanical-defect` without a
  growth fuse is tolerable at enforce time because visibility plus the
  floor plus recurrence-promotion suffices against honest
  misclassification. Its nearest failure mode is exactly the r1 critique's
  laundering scenario, deferred with a named trigger in §10.
- **Wido should reject this design if:** he judges that the direct-fix
  lane must be size- or scope-fused from day one rather than
  visibility-audited — then the defect-identity fuse (TB-R1-04) must be
  pulled back into this slice before implementation; or if he judges
  register carriage too broad an exemption — content-unverified register
  landings are precisely the narrator-relation open item's surface
  (plans/seat-governance-record.md:113-158), and this design deliberately
  does not narrow it; or if he wants the adversarial tier (remote
  enforcement, durable identity) now rather than as the recorded separate
  goal — this design cannot stop a deliberate bypass and does not claim to.
