# Two bars for changes — landing provenance design

- Goal: two-bars-for-changes (plans/goals/two-bars-for-changes.md, revision 4)
- Status: design, round 2 — every ACCEPTED finding of the Sol round-1
  critique (records/two-bars/two-bars-lp-critique-r1.md) folded; the one
  NOTED finding (TB-LP-R1-19) answered with the explicit scope boundary
  in §12. Disposition rows: §10.
- Mode: design only; no code in this change
- Carries: the D90 human ruling (accidental threat model) and the surviving
  decisions of the earlier r3 design record (records/two-bars/two-bars-design.md);
  every carried, restored, or superseded decision is dispositioned in §10,
  per R-25b-m1.

## 0. The whole mechanism in one page (R-11)

Every landing from an agent-held checkout must present exactly one of:

- **Bar (a) — the loop.** `--chain <root-job-id>`: the landing binds to a
  CLOSED delegated chain whose closure included an independent critique
  with zero open material findings. The engine resolves the chain's
  reviewed candidate TREE digest (sealed at close, pinned by a git ref)
  and checks byte-for-byte that every staged path the chain changed
  carries exactly the blob the reviewed tree carries. Stamp:
  `Landing-Provenance: chain=<root-id> tree=<40-hex>`.
- **Bar (b) — the declared direct fix.** `--direct-fix <class>`: a typed
  class (`register-carriage`, `prose-docs`, `revert-exact`,
  `mechanical-defect`). The engine checks the staged paths against the
  class's path rule and the NEVER-DIRECT-FIX floor (instruction-bearing
  paths plus the whole enforcement surface, read from BOTH trees).
  `mechanical-defect` additionally requires a defect identity and a
  consumed red-then-green proof, and a per-identity growth fuse blows
  when one "defect" quietly becomes a refactor. Stamp:
  `Landing-Provenance: direct-fix class=<class> ...`.
- **Bar (c) — refusal.** Neither declared, or a failed check: refuse,
  naming both bars and the offending paths. `loop` is never refused;
  only `direct-fix` is challenged; the cheap way out of doubt is the loop.

Enforcement lives at TWO boundaries, both fail-closed in enforce mode:
the COMMIT boundary (`commit.sh` calls the engine verb, mints the
trailer; a composed commit-msg hook proves exactly one trailer in the
FINAL message; the pre-commit guard refuses when it cannot classify or
cannot run the engine) and the PUSH boundary (a pre-push guard re-runs
every check against each outgoing commit's actual trees — so rebases,
`commit.sh --push`, and commit-then-push-later are all covered by
construction, and one chain lands at most once because the guard refuses
when origin already carries the chain's trailer). Register carriage (the
narrator digest, findings, handoff notes, appended rulings rows) rides
free via a small allowlist. `metasystem report direct-fixes` reads the
trailers out of git history for Wido's review; in observe mode a verdict
trailer makes every would-refuse durable. Promotion observe→enforce is a
governance obligation (internal/governance): DRAFT/OBSERVE cannot
refuse; ENFORCED requires the recorded human authorization the machinery
already checks. Humans stay sovereign (D90: the model is the honest
agent forgetting, not the hostile one).

## 1. Threat model: what the seat can do today

Evidence against the current bytes of this worktree (branch from main):

1. **Ordinary landing never consults a chain.** `scripts/agents/land.sh`
   runs, in order: `verify_checks`, `stage_changes`, `commit_changes`,
   `require_clean_after_commit`, `fetch_origin`, `rebase_origin`
   (land.sh:267-272), then a bounded push-retry loop (land.sh:274-292)
   with a fetch-and-rebase inside it (land.sh:289-290), and the
   transport sync (land.sh:294-296). No step reads a job record, a
   chain, or a review artifact — and the commit happens BEFORE the
   fetch and rebase, so whatever was checked at commit time is not what
   is pushed when origin moved.
2. **The commit wrapper proves tree identity, not provenance.**
   `scripts/agents/commit.sh` captures the index as `proved_tree`
   (commit.sh:106-109), runs the static re-proof and audit, re-proves the
   index unchanged (commit.sh:235-245), commits with the machine trailer
   (commit.sh:260-263), and rolls back if the landed tree differs from the
   proved tree (commit.sh:264-273). This binds the commit to the *locally
   checked index* — the seat's own unexamined bytes qualify. Its direct
   push path (`--push`, commit.sh:278-295) neither fetches nor rebases,
   and on rejection tells the caller to resolve and push later — a push
   that today runs no check at all.
3. **The commit chokepoint claim is only as strong as the guard, and
   the guard fails open.** The pre-commit guard requires the live
   wrapper-ancestry token for any non-HUMAN caller
   (pre-commit-guard.sh:39-48) — but only when `bin/metasystem` is
   executable and classification succeeds; an absent engine, a
   non-executable engine, or a classification error leaves a raw agent
   `git commit` untouched (pre-commit-guard.sh:29-39). Today that is a
   deliberate sovereignty choice; under enforcement it is the hole an
   honest agent on a half-built checkout falls through (TB-LP-R1-01).
4. **The closure gate is real but disconnected from landing — and
   closure alone proves less than it seems.** `dispatch.sh close` runs
   `job close-check` → `CloseCheck` (internal/dispatch/close.go:15-127)
   → `validateHazardCompletion` (internal/dispatch/hazard.go:165-214),
   which for DESIGN-BEARING and DESTRUCTIVE-REACH chains requires a
   completed, fresh-session, distinct-model independent-critique job
   over the exact final work round (hazard.go:261-301). But (i) nothing
   downstream consumes `chainClosed`; (ii) the validator never opens the
   critic's return or its findings — a critique that ENDED with open
   material findings still closes (TB-LP-R1-05); (iii) a MECHANICAL
   chain requires no critique at all (hazard.go:36-41, early return at
   :193-198), and a non-mission Markdown chain can waive critique
   (conformance.go:532-559, :574-600) — so "closed" does not mean
   "examined" (TB-LP-R1-08).
5. **The candidate digest exists but is neither sealed nor pinned.**
   Conformance's review stage snapshots the implementer worktree as tree
   `reviewedTree` and persists it with the diff artifact in
   `artifacts/agents/<root>/rounds/<n>/review.json`
   (conformance.go:297-303, :439-451); close-check verifies the
   mirrored `diff.patch` content digest (close.go:57-83) but performs no
   seal over `review.json`, which stays same-user mutable after closure
   (TB-LP-R1-04). And the tree object itself is an unreferenced
   `write-tree` result (gittree.go:246-276) readable only while the
   disposable delegate worktree keeps it alive — after worktree removal
   and git gc it can vanish (TB-LP-R1-14; plans/README.md:3-5 calls the
   worktrees disposable).
6. **Existing refusals prove the pattern.** The landing driver already
   refuses bare rulings ids (land.sh:131-181, invoked at :193) and
   unacknowledged new plan files (land.sh:260-265; pre-commit-guard.sh:70).
   The two bars are more refusals of the same species, at the same
   boundaries.

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
Landing-Provenance: direct-fix class=<class-id> [defect-id=<slug>] [defect="<one line>"] [revert-of=<40-hex-commit>]
Landing-Provenance-Verdict: pass | would-refuse code=<reason-code>   (observe mode only, §8)
```

**Grammar, settled (TB-LP-R1-23).** `<root-job-id>` and `<class-id>`
match the existing job-id grammar (`validJobID`). `<slug>` is
`[a-z0-9-]{1,64}`. `defect=` is one double-quoted line: UTF-8, 1–200
bytes, and any byte below 0x20, any `"`, or the substring
`Landing-Provenance` inside the value REFUSES the commit — malformed
input is rejected, never escaped, so every parser sees one canonical
form. `revert-of` must name a NON-MERGE commit (exactly one parent)
reachable from `origin/<branch>`; reverting a merge is ambiguous without
a parent choice and takes the loop. A missing required field, an unknown
field, or two provenance trailers is a refusal naming the field.

**Exactly one trailer, proved on the FINAL message (TB-LP-R1-16).** The
r1 argument-scan is withdrawn as enforcement: `land.sh` passes `-F` plus
a filename (land.sh:226-232), so an argument scan reads the filename,
not the message, and `-C`/`-c`/`--amend`/editor paths never pass through
arguments at all. Restored from the r3 record (two-bars-design.md:24-31,
:113-130): a composed `commit-msg` hook validates the final message —
exactly one `Landing-Provenance` trailer, byte-equal to the trailer the
engine verb wrote into the wrapper token file (the token commit.sh
already mints, commit.sh:57-58, carries the expected trailer bytes for
this one commit; consume-on-use). An author-supplied trailer, a second
trailer, or a missing trailer refuses at commit-msg. Hook composition
follows the restored r3 lifecycle: adoption and `metasystem up` install
pre-commit, commit-msg, and pre-push guards by COMPOSING any existing
hook, never by skipping installation.

- The caller supplies only flags: `--chain <root-id>` XOR
  `--direct-fix <class>`, plus `--defect-id <slug>` and `--defect <text>`
  (both required for `mechanical-defect`) or `--revert-of <sha>`
  (required for `revert-exact`). Both `land.sh` and `commit.sh` accept
  them; `land.sh` passes them through to `commit_changes`.
- The `tree=` value is resolved by the engine from the chain's sealed
  close record, never from the caller.

Why a trailer and not a side file: the commit is the unit being governed;
a side file can desynchronize from history, needs its own carriage rules,
and duplicates what git already transports. The job records, the sealed
close facts, and the mirrored review artifacts remain the deep evidence;
the trailer is the join key.

## 3. Bar (a): binding to a closed, examined chain

New engine verb — working name `metasystem job landing-check` — owned by
`internal/dispatch`. Called at the commit boundary by `commit.sh` after
the settled-tree re-proof (commit.sh:235-245) and before `git commit`,
receiving the already-proved index tree; re-run at the push boundary
(§8) against each outgoing commit. Checks, in order, each with its own
refusal:

1. **Chain exists and is closed.** The root job record in this machine's
   `artifacts/agents/jobs/` carries `chainClosed: true`.
2. **The critique actually concluded clean (TB-LP-R1-05, -08).** Closure
   alone is not examination. The verb additionally requires: the root
   record carries `independentCritiqueJobRef` (the reference hazard
   closure validates for DESIGN-BEARING/DESTRUCTIVE-REACH chains,
   hazard.go:261-301), AND the chain's canonical critique register
   reports zero open material finding identifiers (the existing
   `job critique-open-finding-ids` join); a chain with a critique job
   but no folded register fails closed. A chain WITHOUT a completed
   independent critique (a MECHANICAL chain, or a critique-waived
   prose chain) may still land under bar (a), but is then subject to
   the same NEVER-DIRECT-FIX floor as bar (b): if any staged non-carriage
   path is on the floor (§4), the landing refuses and names the remedy —
   floor paths change only through an independently critiqued chain.
   This makes the floor the universal examination floor: no path on it
   reaches origin, under either bar, without independent examination —
   a wrong hazard declaration or a waiver can no longer skip it.
3. **Candidate facts come from the CLOSE-TIME SEAL, not from mutable
   round files (TB-LP-R1-04, -17, -22).** Closing gains three named
   obligations: (i) close-check verifies `review.json` is mirrored with
   a matching digest exactly as it already verifies `diff.patch`
   (close.go:57-83); (ii) the close patch stamps onto the root record,
   beside `chainClosed`: `reviewedTree`, `baseTree`, and the sha256 of
   `diff.patch` (schema change: these join `terminalMetadataFields`,
   record.go:92-95, written only by the close operation); (iii)
   `review.json` gains a `baseTree` field — the boundary base tree the
   review stage already resolves (conformance.go:274) — written beside
   `reviewedTree` (conformance.go:439-443). `landing-check` reads
   `reviewedTree` and `baseTree` from the sealed root record and
   verifies the local `review.json` and `diff.patch` digests still match
   the close-time seal and the durable mirror manifest; any mismatch
   refuses as evidence tampering/loss, never silently re-resolves.
4. **The changed-path set is computed from trees, never parsed from the
   patch (TB-LP-R1-22).** `chainPaths` :=
   `gittree.ChangedPaths(baseTree, reviewedTree)` — the null-delimited
   primitive (gittree.go:329-343) that is safe for every valid git
   filename. No diff.patch parsing exists anywhere in the mechanism.
5. **The reviewed tree is pinned by a ref (TB-LP-R1-14).** The
   conformance review stage, immediately after writing `review.json`,
   writes `refs/metasystem/reviewed/<root-job-id>` pointing at the
   `reviewedTree` object (one `git update-ref` in the shared repository;
   delegate worktrees share the one object database). The ref keeps the
   tree and its blobs alive across worktree removal and gc. It is never
   pushed; it is deleted only by the same evidence-retention step that
   archives the chain's local artifacts. `landing-check` refuses when
   the ref or the object is unreadable, naming the evidence loss.
6. **Byte binding.** The staged change set (index vs HEAD, from the
   proved tree) is partitioned: every path must be in `chainPaths` or on
   the carriage allowlist (§5). For every staged path in `chainPaths`,
   the staged index entry (mode + blob OID, project-prefix mapped
   exactly as conformance's `projectWorkspace` does it,
   conformance.go:259-261) must equal `reviewedTree`'s entry at that
   path; a path the chain deleted must be absent from the index. Any
   mismatch names the path and refuses. Partial landings are refusals:
   the reviewed diff lands whole or not at all.
7. **One landing per chain — enforced where it is observable
   (TB-LP-R1-13).** The authoritative invariant is history-shaped: at
   most one commit reachable from `origin/<branch>` carries this
   chain's trailer. The PUSH-boundary check (§8) refuses any outgoing
   commit whose `chain=<root-id>` already appears in a commit reachable
   from the fetched `origin/<branch>` tip (one `git log --grep` over
   the fixed trailer string). This needs no atomicity with the push: a
   crash after push leaves the trailer in origin, so the retry refuses;
   a crash before push leaves nothing, so the retry lawfully lands; two
   racing machines resolve through the push fast-forward rule — the
   loser rebases, re-checks, sees the winner's trailer, and refuses.
   AFTER a successful push the driver additionally patches
   `{"landedCommit":"<sha>"}` onto the root record for the report's
   convenience — a bookkeeping write, not the enforcement, and a named
   schema change (`landedCommit` joins `terminalMetadataFields`,
   record.go:92-95; today it would be refused at :530-535).

**Rebase behavior (the fleet's moving tip).** The tree digest is
rebase-stable, but a rebase can change landed bytes: when origin moved
inside a file the chain also touched, the rebase textually merges and
the resulting blob differs from the reviewed blob. The r1 answer —
re-check inside `land.sh` only — governed one caller and left
`commit.sh --push` and commit-then-push-later unchecked (TB-LP-R1-03).
The r2 answer moves the re-check to the push boundary itself (§8): every
outgoing agent commit is re-verified against its ACTUAL post-rebase
trees before any push, by every push path. `land.sh` keeps an early
in-loop re-check after each rebase (land.sh:272, :289-290) purely for a
better message closer to the cause; the pre-push guard is the
enforcement. On a bar (a) mismatch the refusal names the diverged paths
and the remedy: the rebased content is no longer the examined content —
dispatch a follow-up round on the current tip, re-run conformance and
the required critique, close, and land again. Rebasing never silently
re-binds; it either preserves the reviewed blobs or reopens examination.
Direct-fix commits are ALSO re-checked at the push boundary against
their post-rebase trees (TB-LP-R1-02): the r1 claim that "a rebase
cannot add paths" was true of the commit's own change set but ignored
that the FLOOR is tree-relative — a fetched tip can add a floor entry or
move a protected path, so class rule and floor are re-evaluated from the
outgoing commit's parent tree and its own tree (§4's both-trees rule,
applied to the trees that will actually be pushed).

## 4. Bar (b): the typed direct-fix classes

**The generating rule.** A class is lawful only if it is listed in the
class manifest AND its path rule is disjoint from the NEVER-DIRECT-FIX
floor. The floor is generated, not hand-curated: it is
(i) every entry of `scripts/agents/instruction-bearing-paths.txt` plus
the runtimes' declared instruction files — the union the merge-stage
waiver already refuses to waive (conformance.go:483-530) — and
(ii) the two-bars enforcement surface, enumerated to include the code
that CONTROLS the check itself (TB-LP-R1-09): `scripts/agents/commit.sh`,
`scripts/agents/land.sh`, `scripts/agents/pre-commit-guard.sh`, the new
pre-push guard script, `scripts/agents/go-gate.sh` (it builds the
proof engine), `internal/dispatch/`, `internal/governance/`,
`internal/validate/`, `internal/lease/` (the human exemption),
`internal/config/`, `internal/gittree/` (the tree comparisons),
`cmd/metasystem/` (verb routing and parsing, main.go:92-140), the class
manifest, the carriage allowlist, and `metasystem.conf` (whole file: a
path rule cannot scope to one key, and behavior configuration is
floor-worthy; the human-word promotion commits are human-sovereign and
unaffected).

The floor is evaluated from BOTH trees — base and candidate: at the
commit boundary HEAD and the proved index; at the push boundary the
outgoing commit's parent tree and its own tree — so a direct fix cannot
delete its own floor entry or the manifest in the same commit (carried
from r3, TB-R1-01), and a rebase that brings a new floor entry is seen
(TB-LP-R1-02). Reading either tree's instruction list or manifest fails
→ fail closed for every direct fix (the loop stays open).

**The manifest contract (TB-LP-R1-21).** Path:
`scripts/agents/landing-classes.json`, committed, on the floor. Schema:
`{"schemaVersion": 1, "classes": [{"id", "pathRule", "requiredFields",
"fuse", "authorizedBy"}]}` where `pathRule` is one of the four TYPED
rules below (never a free pattern), `requiredFields` lists the trailer
fields the class demands, `fuse` (only on `mechanical-defect`) is
`{"maxAggregateLines": <int>, "maxSubsystems": <int>}`, and
`authorizedBy` names the rulings-register row (R-id) that authorized the
class (§ human word, below). Duplicate ids, an unknown `pathRule`, a
missing field, unparseable JSON, or an unreadable file: every direct fix
refuses (fail closed); bar (a) is unaffected. The carriage allowlist
(`scripts/agents/register-carriage-paths.txt`) contains exact
repository-relative paths, one per line, plus at most a trailing
basename glob of the single form `<dir>/<prefix>-*.md` — no other
wildcard semantics exist (TB-LP-R1-21's mixed-wildcard ambiguity is
closed by construction).

**Initial classes** (the manifest ships with exactly these four):

| id | path rule (typed) | required fields | evidence |
| --- | --- | --- | --- |
| `register-carriage` | every staged path on the carriage allowlist (§5) | none | none |
| `prose-docs` | every staged path ends `.md`, none on the floor, none under `plans/` | none | none |
| `revert-exact` | tree-shaped inverse (below); floor paths still refuse | `revert-of=<sha>` | the inverse check itself |
| `mechanical-defect` | no staged path on the floor | `defect-id=<slug>`, `defect="<one line>"` | a consumed direct-fix proof (below) |

`prose-docs` excludes `plans/` entirely (TB-LP-R1-06, -07): design
documents live there, and a design doc declared "prose" is exactly the
laundering the bars exist to stop; handoff notes ride carriage (§5), new
plan files already need the explicit acknowledgment
(pre-commit-guard.sh:70-86), and everything else under plans/ takes the
loop.

`revert-exact`, defined tree-shaped so it is rebase-stable and
re-checkable at the push boundary: for every path in the named commit's
change set (`ChangedPaths(revert-of^, revert-of)`), the reverting
commit's tree must record exactly the entry (mode + OID) that
`revert-of^` recorded — including absence for paths the named commit
added — and the reverting commit's own change set must contain no other
non-carriage paths. The named commit has exactly one parent (§2).

**The direct-fix evidence bar, restored (TB-LP-R1-15, r3 keep-list
two-bars-design.md:38-49, :163-178).** `mechanical-defect` requires a
proof, produced before the commit by a new engine verb
`metasystem proof direct-fix`, recorded machine-locally with a nonce and
a consume-on-use lifecycle:

- The proof record: `{defectId, baseTree, candidateTree, assertionKind,
  assertion, redOutcome, greenOutcome, nonce, consumedBy}` — one
  reusable assertion evaluated against BOTH immutable trees: red (the
  assertion fails / the bad state holds) against `baseTree` (HEAD at
  proof time), green against `candidateTree` (the proved index tree).
  `assertionKind` is `test` (one named test command, accommodating a
  newly added regression test, per the r3 definition) or `state` (a
  structured before/after repository-state assertion — the first-class
  proof kind for the no-failing-test cases: a stray binary, unstaged
  files; r3 TB-R1-07).
- `landing-check` consumes it: `candidateTree` must equal the proved
  index tree, the nonce must be unconsumed, and consumption stamps
  `consumedBy=<commit intent>` — one proof, one landing. A stale or
  consumed proof refuses with the one-step remedy (re-run the proof
  verb).
- `register-carriage`, `prose-docs`, and `revert-exact` need no proof:
  their evidence is their path rule or tree equality itself. This is
  the honest scope of the r1 "superseded wholesale" claim, corrected:
  the closure-gate artifacts replaced the witness machinery for bar (a)
  ONLY; bar (b)'s evidence bar is restored, not superseded.

**The growth fuse, restored (TB-LP-R1-24, r3 two-bars-design.md:152-161).**
The r1 claim that severity-tiered-rigor's near-miss register substitutes
for the fuse is withdrawn: that register does not exist and its own
design forbids depending on it (severity-tiered-rigor-design.md:16-17).
The fuse ships in THIS slice, r3-shaped: every `mechanical-defect`
declares `defect-id`; the engine aggregates, across every commit
reachable from `origin/<branch>` citing the same `defect-id` plus the
staged change, the total changed lines (numstat) and the set of
subsystems touched (subsystem := first path segment under the
installation prefix), measured against the immutable pre-defect base
(the parent of the first commit citing the id). The fuse blows —
refusal, naming the aggregate and the loop as the remedy — when the
aggregate exceeds the manifest's `maxAggregateLines` (initial: 200) or
the subsystem count exceeds `maxSubsystems` (initial: 1). A blown fuse
is not an error state to clear; the remaining work takes bar (a).

**Who may extend the list and how — the human word, CHECKED
(TB-LP-R1-11).** The manifest and the allowlist sit on the floor, so a
change to either takes bar (a) through an independently critiqued chain
(§3.2). The human word is additionally a mechanical join, not prose: a
landing whose staged set changes `landing-classes.json` is checked by
`landing-check` — every added or modified class row's `authorizedBy`
must name a rulings-register row that EXISTS in the candidate tree's
`memory/rulings.md`; a missing or dangling id refuses. The same
two-key shape as seat-record Law 2 (plans/seat-governance-record.md:191-195):
the chain is the examination key, the recorded ruling is the human key,
and the check runs at the base action boundary, not in prose. The
observe→enforce promotion is governed the same way (§8).

**Honest bounds.** The classes are path- and evidence-shaped, not
semantic: they cannot tell a design change confined to a non-floor path
from a mechanical one. The floor, the required proof, the visible
declaration, the fuse, and aggregation are the accidental model's answer
(D90): an honest seat cannot *forget* to choose a bar, a wrong choice is
a named greppable act, and a "small fix" cannot quietly become a
refactor. What remains out of scope is the deliberate liar — §12.

## 5. Register carriage: the standing exemption, narrowed

Fleet fact: the steward's narrator digest (internal/narratordigest,
written into `narrator-digest.log`) rides almost every landing, and
register-only landings must stay one flag. The carriage allowlist
(`scripts/agents/register-carriage-paths.txt`, on the floor) is exempt
from both bars' path partitioning. Narrowed from r1 (TB-LP-R1-06):

- Seeded entries: the narrator digest log, `memory/findings.md`,
  `memory/rulings.md`, and `plans/handoff-*.md` (the one permitted glob
  form, §4). NOT `plans/*.md`: top-level design documents are the
  opposite of running-state registers, and r1's entry made this design
  itself carriage-exempt — an unexamined-byte lane for the exact
  content the bars govern.
- `memory/rulings.md` carriage is ADD-ROWS-ONLY: the engine applies the
  same staged-diff row parse the minting check already performs
  (land.sh:145-173) and refuses carriage when the rulings diff removes
  or rewrites any existing row — a rewrite of a recorded ruling is a
  governance change and takes a bar (the r1 remove-and-add rewrite lane
  is closed). The existing mint-suffix check remains in force.
- The narrator digest stays carriage and stays content-unverified. That
  is a deliberate, bounded residue: pending narration is rewriteable
  repository content (plans/seat-governance-record.md:123-132) and its
  verification is the governance record's SECOND open item, which this
  design deliberately does not remedy — the explicit scope boundary in
  §12, not a silent gap.

A landing that stages *only* carriage paths declares
`--direct-fix register-carriage`. `plans/goals/**` needs no entry: the
goal-ledger fence already refuses it in ordinary commits
(pre-commit-guard.sh:62-68).

## 6. Bar (c): refusal — and the guard that can no longer fail open

In enforce mode, an agent-classified commit with no declaration, two
declarations, or a failed check refuses before `git commit` runs (the
existing no-rollback-needed property holds). The operator-visible
message, verbatim shape:

```
landing refused: no provenance — every landing takes one of two bars
  (a) --chain <root-job-id>     bind this landing to a CLOSED, examined delegated chain
  (b) --direct-fix <class>      declare a typed direct fix
      classes: register-carriage | prose-docs | revert-exact | mechanical-defect
  neither fits? the change takes the loop: dispatch it, close the chain, land with --chain
```

A failed check keeps the header and appends the exact cause with paths,
e.g. `chain q…z is not closed`, `chain q…z has open material findings`,
`staged path X differs from reviewed tree <digest>`, `class prose-docs
does not cover path Y`, `chain q…z already landed as <sha>`, `path Z is
on the never-direct-fix floor`, `defect-id lease-scan aggregate 312
lines exceeds the 200-line fuse`. Every refusal is one screen, names the
offending object, and names the lawful next step.

**Fail-closed chokepoints (TB-LP-R1-01).** The r1 design inherited the
guard's deliberate fail-open (pre-commit-guard.sh:29-39: no engine or a
classification error leaves the commit untouched) while claiming the
wrapper was unavoidable — under enforcement those cannot both be true.
Revised rule: when enforcement is in force (§8), the pre-commit and
pre-push guards FAIL CLOSED — an absent or non-executable engine, an
engine that does not know the landing-check verb, or a classification
error refuses the commit or push with the remedy named (`build
bin/metasystem` / `run metasystem up`). Human sovereignty is preserved
without a classifier: a positively classified HUMAN caller passes as
today, and the sovereign escape for a human on a broken checkout is
`git commit --no-verify` — a deliberate act that D90 places out of the
accidental model for agents (an agent using it is the adversarial tier,
recorded, not defended against). In observe mode the guards keep today's
fail-open, so a half-built checkout cannot brick landings before the
mechanism has ever been armed. Human-classified callers are never
refused by the bars; a human commit through `commit.sh` still gets the
trailer stamped when flags are present.

## 7. Audit surface

The aggregate IS git history plus one reader — with its evidence scope
stated honestly (TB-LP-R1-20):

- `metasystem report direct-fixes [--since <date>]` scans
  `git log origin/<branch>` for `Landing-Provenance` trailers and prints
  per-class, per-machine, and per-defect-id counts plus one line per
  commit (sha, class, defect fields, subject).
- **Integrity cross-check, machine-scoped.** Job records and review
  artifacts are machine-local, so one machine can verify only its own
  landings (the r1 claim to cross-check EVERY chain trailer was not
  implementable). The report verifies bar (a) trailers whose `Machine:`
  trailer names this machine's enrolled nickname (commit.sh:260-263):
  root record exists, `tree=` matches the sealed `reviewedTree`,
  `landedCommit` matches. Peer machines' trailers are listed as
  `peer-scope: verify on <machine>` — a stated boundary, never a false
  integrity finding. The fleet picture is the union of the four per-
  machine reports at Wido's review cadence; that union is a read, not a
  new replication mechanism.
- **The durable audit join (TB-LP-R1-17), restored from r3
  (two-bars-design.md:180-193):** the join key (trailer) plus the
  close-time seal on the root record (§3.3) plus the reviewed-tree ref
  (§3.5) make provenance reconstructible after local round files are
  archived; and the new global rule itself gets an instruction-ledger
  entry (`plans/instruction-ledger.md`) with expected effect and a later
  verdict, written as part of the rollout's first step (§8). A defined
  history-audit pass — the report over `--since` the enforce flip —
  names any commit whose provenance cannot be reconstructed, noting (as
  r3 did) that a missing classification is indistinguishable from a
  sovereign human commit after the fact.
- Readers: Wido at the review cadence
  (plans/seat-governance-record.md:215-218), and the retro. A repeated
  `mechanical-defect` cluster on one subsystem now trips the FUSE
  mechanically (§4) before it becomes a retro finding.
- No new ledger file. A parallel append-only landing ledger was
  considered and rejected (§9.3).

## 8. Enforcement points, engine identity, and migration

**Two boundaries, one rule.** The decision check runs at the COMMIT
boundary (mint) and the PUSH boundary (verify), never as shell logic:

- Commit boundary: `commit.sh` calls `job landing-check` on the proved
  index (§3); the commit-msg hook proves the final message (§2); the
  pre-commit guard proves wrapper ancestry (§6). Engine identity here:
  the PROOF-BUILT engine from `go-gate.sh --fast` (commit.sh:114-133) —
  the existing deliberate choice that a policy edit is judged by the
  policy in the prospective bytes; a floor-touching change reaches this
  point only through an independently critiqued chain (§3.2), so the
  self-judging window is examined bytes judging themselves, which the
  accidental model accepts.
- Push boundary (TB-LP-R1-03, -10): a pre-push guard hook — installed
  and composed exactly like the pre-commit guard — reads the refs being
  pushed, computes the outgoing range `origin/<branch>..<local>` from
  the freshly fetched remote tip, and for an agent-classified caller
  runs `job landing-check --commit <sha>` for every outgoing commit:
  trailer present and well-formed, bar (a) blob binding against the
  commit's ACTUAL tree, bar (b) class rule and floor against the
  commit's parent tree and own tree, one-landing-per-chain against
  origin history (§3.7). Engine identity here is FIXED (TB-LP-R1-10):
  the installed `bin/metasystem`, fail-closed when absent,
  non-executable, or ignorant of the verb (§6) — never a rebuild from
  candidate bytes at push time (a push must not compile code, and the
  prospective-bytes argument does not apply: the pushed commits are
  already committed history). Enforcement MODE is read from the
  checkout's committed governance state (below), so a stale binary can
  refuse (verb missing → fail closed) but can never silently enforce
  old rules as pass.
- This covers, by construction, every path the r1 design claimed but
  did not cover: `land.sh` landings (its in-loop rebases push through
  the same hook), direct `commit.sh --push` (commit.sh:278-295),
  commit-then-push-later, and any wrapper-mediated push after any
  number of rebases. It also survives land-verb-pruning: hooks and
  engine verbs move with the boundary, not with the caller.

**Promotion is a governance obligation, not a bare key
(TB-LP-R1-11, -25).** `landing.provenance` is registered as an
obligation in the existing internal/governance state machine
(ObligationObserve / ObligationEnforced, types.go:14-16): DRAFT and
OBSERVE record would-refuse outcomes and cannot refuse; ENFORCED
requires the complete recorded human authorization the machinery already
validates (types.go:145-173), checked by the guards at the base action
boundary — the seat cannot flip enforcement by editing a config line,
because the obligation state and its authorization are what the check
reads, and the state file rides the floor.

**Observe mode produces durable evidence (TB-LP-R1-25).** In observe,
the verb runs every check and stamps, beside the provenance trailer, a
`Landing-Provenance-Verdict: pass` or
`Landing-Provenance-Verdict: would-refuse code=<reason-code>` trailer
(§2). The verdict rides the commit into shared history, so the
promotion evidence is mechanical: the report over the observe window
shows, per machine, clean `pass` landings of every shape that machine
uses and zero unresolved `would-refuse` lines. A console line proves
nothing; a trailer in origin's history proves the window. In enforce
mode the verdict trailer is not stamped (a would-refuse never commits).

**Migration for four machines (no flag day), with the bootstrap named
(TB-LP-R1-12).**

1. **Bootstrap exception, explicit and singular:** the landing that
   introduces the mechanism is executed by the old wrapper and cannot
   check or stamp itself. It is landed as an ordinary bar-(a)-in-spirit
   landing — a closed, critiqued chain, with the chain id named in the
   commit message body — and this design records that one landing as
   the mechanism's ungoverned genesis. No general exception exists;
   the exception is this named commit, once. The same landing writes
   the instruction-ledger entry (§7).
2. Land in OBSERVE (the obligation's initial state). A machine picks
   the mechanism up through two channels, both named: its next
   fetch-rebase brings the scripts and hooks source; its next
   `metasystem up` (session start) installs/composes the hooks and is
   the point a machine's enforcement actually arms — hook installation
   is added to up's arming duties precisely so adoption is an explicit,
   observable act, not an assumption about loaded shells.
3. Run observe until each machine's history shows the §8 verdict-trailer
   evidence. Flip to ENFORCED through the governance machinery's
   recorded human authorization (Wido's word).
4. **The cutover skew window, honestly bounded:** a lagging machine
   with an already-loaded old `land.sh` commits before fetching
   (land.sh:267-272) — but the PUSH runs the pre-push hook from the
   CHECKOUT as it exists at push time, which the in-flight rebase just
   updated to contain the flip; the loaded shell cannot carry stale
   enforcement past the push boundary. The residual window is a machine
   that has neither rebased nor run `up` since the flip: it behaves
   exactly as today until its next landing (whose rebase delivers the
   flip before its push) or its next session start. No landing ever
   fails because a *peer* machine is behind; all checks read
   machine-local state and the fetched origin tip.

Rollback, if enforce proves wrong in practice: the obligation returns to
OBSERVE by the same recorded human word. The observe branch is the
enforce branch minus the exit code (plus the verdict trailer).

## 9. Rejected alternatives

1. **Bind to a commit id instead of a tree/content digest.** A rebase
   rewrites every commit id by construction; four machines pushing to
   one main guarantees rebases. Rejected; the candidate digest is the
   sealed `reviewedTree` plus per-path entry comparison, with the
   push-boundary re-check as the explicit re-bind rule (§3, §8).
2. **Remote enforcement (CI or branch protection verifying provenance
   on push).** The adversarial model's machinery: it needs durable
   agent identity and cannot read machine-local job records; D90 ruled
   the accidental model with the adversarial tier as a separate future
   goal (records/two-bars/two-bars-design.md:78-109). Rejected here;
   recorded as the known escalation path, unchanged.
3. **A parallel provenance ledger (append-only landings file or signed
   receipts).** Duplicates what the trailer plus the sealed record
   already say, adds a second concurrent writer across four machines,
   and violates the constraint to extend the closure gate rather than
   parallel it. Rejected; the trailer is the join key.
4. **Enforce in `land.sh` only.** Governs one caller of the boundary
   and dies at land-verb-pruning. Rejected; the boundaries are the
   commit wrapper and the push hook (§8).
5. **Rebuild the engine from candidate bytes at the push boundary.**
   Would let an enforcement change judge itself with unexamined code
   and make every push a compile. Rejected; the push-boundary engine is
   the installed binary, fail-closed on absence or verb ignorance
   (§8, TB-LP-R1-10).
6. **A cryptographic seal on review.json.** Same-user signatures prove
   nothing the accidental model needs; the close-time seal onto the
   root record plus mirror-manifest digests catches accidental and
   after-the-fact drift, which is the governed threat (§3.3).

## 10. Dispositions

### 10.1 The carried r3 design record (R-25b-m1)

- **Carried unchanged:** the D90 accidental threat model and human
  sovereignty; fail-closed with `loop` never refused; the
  never-direct-fix floor as a conservative FLOOR read from both trees
  (TB-R1-01); emergency = human-personal, no agent override (TB-R1-06);
  the anti-bureaucracy bar — the common case is one flag plus, for
  `mechanical-defect` only, one proof-verb run (TB-R1-07).
- **Restored in r2 (the r1 round had silently dropped them):** the
  composed pre-commit + commit-msg hook enforcement of the final
  message (TB-R1-02, §2); the direct-fix evidence bar — red-then-green
  assertion against both immutable trees, structured state assertions,
  nonce with consume-on-use (TB-R1-03/-07, §4); the defect-identity
  growth fuse over reachable history (TB-R1-04, §4); the durable audit
  join — content-bound reachable references, critique-closure join,
  instruction-ledger entry, defined history audit (TB-R1-05, §3, §7);
  the settle-the-contracts build gate and the design-obligation matrix
  (§11).
- **Superseded by goal revision 4, strictly stronger, scope now stated
  honestly:** `Design-Chain:` string references and the bespoke
  full-validator gate-witness machinery are replaced FOR BAR (a) ONLY
  by the closure gate's own sealed artifacts (`chainClosed`, sealed
  `reviewedTree`/`baseTree`/diff digest, the critique register). Bar
  (b)'s witness machinery is not superseded; it is restored above.

### 10.2 The Sol landing-provenance critique, round 1 — how each ACCEPTED finding is folded

- **TB-LP-R1-01** — folded §6: in enforce, both guards fail CLOSED on a
  missing/non-executable engine, an unknown verb, or a classification
  error; observe keeps today's fail-open; the human escape is the
  sovereign `--no-verify`, out of the accidental model by D90.
- **TB-LP-R1-02** — folded §3/§8: direct-fix commits lose their rebase
  exemption; class rule and floor are re-evaluated at the push boundary
  from the outgoing commit's parent and own trees; `revert-exact` is
  redefined tree-shaped (§4) so it too is re-checkable post-rebase.
- **TB-LP-R1-03** — folded §8: enforcement moves to the push boundary
  (pre-push guard over the outgoing range), covering `commit.sh --push`
  failed-push retries and commit-then-push-later by construction;
  land.sh's in-loop re-check becomes a courtesy, not the enforcement.
- **TB-LP-R1-04** — folded §3.3: close seals `reviewedTree`, `baseTree`,
  and the diff digest onto the root record and verifies `review.json`
  in the mirror manifest; landing-check verifies the local files
  against the seal and manifest, refusing on mismatch.
- **TB-LP-R1-05** — folded §3.2: landing-check independently requires
  the completed independent-critique reference AND zero open material
  finding ids in the canonical critique register; closure alone no
  longer stands in for an acceptable outcome.
- **TB-LP-R1-06** — folded §5: `plans/*.md` leaves carriage
  (handoff-notes glob only); rulings carriage is add-rows-only with the
  rewrite lane closed; the narrator digest remains carriage as the §12
  scope boundary, named, not silent.
- **TB-LP-R1-07** — folded §4: `mechanical-defect` regains the proof
  and defect-identity requirements and the fuse; `prose-docs` excludes
  `plans/`; the residual semantic gap is stated in §4's honest bounds
  and bounded by fuse + floor + visibility.
- **TB-LP-R1-08** — folded §3.2: the floor becomes the universal
  examination floor — a chain without a completed clean independent
  critique (MECHANICAL, or waived) cannot land floor paths; a wrong
  hazard declaration can no longer skip examination of the enforcement
  and instruction surface.
- **TB-LP-R1-09** — folded §4: the floor enumeration adds
  `cmd/metasystem/`, `internal/lease/`, `internal/config/`,
  `internal/gittree/`, `internal/validate/`, `go-gate.sh`, the pre-push
  guard, and `metasystem.conf` whole.
- **TB-LP-R1-10** — folded §8: engine identities fixed per boundary —
  proof-built engine at commit (existing pattern), installed binary at
  push, fail-closed on absence or verb ignorance; candidate-bytes
  rebuild at push explicitly rejected (§9.5).
- **TB-LP-R1-11** — folded §4/§8: class-manifest changes carry a
  checked `authorizedBy` rulings-row join verified by landing-check;
  observe→enforce runs through internal/governance's recorded
  human-authorization machinery at the base action boundary.
- **TB-LP-R1-12** — folded §8: the bootstrap landing is a named,
  singular, recorded exception; adoption arms at `metasystem up` (hook
  install as an arming duty); the loaded-shell cutover hole is closed
  because the push runs the checkout's post-rebase hook, and the
  residual window is stated and bounded.
- **TB-LP-R1-13** — folded §3.7: one-landing-per-chain is enforced as a
  history invariant at the push boundary (origin trailer scan), which
  needs no push atomicity and survives crashes and races;
  `landedCommit` becomes bookkeeping, with its `terminalMetadataFields`
  schema change named (record.go:92-95, :530-535).
- **TB-LP-R1-14** — folded §3.5: the conformance review stage writes
  `refs/metasystem/reviewed/<root-id>` at the reviewed tree, with its
  lifecycle (never pushed, deleted at evidence archival) and its
  refusal-on-loss named.
- **TB-LP-R1-15** — folded §4 and §10.1: the direct-fix evidence bar is
  restored (proof verb, both-trees red-then-green, state assertions,
  nonce consume-on-use); the "superseded wholesale" claim is corrected
  to bar-(a)-only.
- **TB-LP-R1-16** — folded §2: the argument scan is withdrawn as
  enforcement; a composed commit-msg hook validates the final message
  against the engine-written token trailer; hook composition lifecycle
  restored from r3.
- **TB-LP-R1-17** — folded §3.3/§3.5/§7: seal on the root record, the
  reviewed-tree ref, the instruction-ledger entry, and a defined
  machine-scoped history audit replace the r1 gesture at one.
- **TB-LP-R1-18** — folded §11: the settle-the-contracts gate and the
  design-obligation matrix are restored; §11's matrix rows are the
  build gate's input and implementation is refused without them
  (`validate design-obligations` exists, cmd/metasystem/main.go:86).
- **TB-LP-R1-19** — NOTED; answered in §12 with the explicit scope
  boundary, not a mechanism.
- **TB-LP-R1-20** — folded §7: the integrity cross-check is
  machine-scoped by the `Machine:` trailer; peer trailers report as
  peer-scope, never as findings; the fleet picture is the union of
  per-machine reports.
- **TB-LP-R1-21** — folded §4: manifest path, JSON schema, typed path
  rules, duplicate/malformed fail-closed behavior, the single permitted
  allowlist glob form, and both-trees read-failure behavior are all
  pinned.
- **TB-LP-R1-22** — folded §3.3/§3.4: `review.json` gains `baseTree`;
  `chainPaths` is recomputed via the null-delimited `ChangedPaths`
  primitive; no diff.patch parsing exists.
- **TB-LP-R1-23** — folded §2: byte encoding, length bound,
  reject-not-escape rule, embedded-trailer refusal, and the non-merge
  parent rule for `revert-of` are pinned.
- **TB-LP-R1-24** — folded §4: the substitute claim is withdrawn with
  the citation (severity-tiered-rigor-design.md:16-17); the
  defect-identity fuse ships in this slice with concrete initial
  numbers in the manifest.
- **TB-LP-R1-25** — folded §8: observe stamps a durable
  `Landing-Provenance-Verdict` trailer; promotion evidence is the
  report over history, not console lines.

## 11. Obligation matrix and build order (the restored readiness gate)

Implementation is refused until every row below has an owner and a
verifying check (the r3 build gate, two-bars-design.md:50-65; matrix
structure per `validate design-obligations`):

| # | Obligation | Owning subsystem | Verified by |
| --- | --- | --- | --- |
| 1 | Trailer + verdict grammar, reject-not-escape (§2) | internal/dispatch (landing-check) | table-driven parser tests incl. malformed bytes |
| 2 | commit-msg hook: exactly-one-trailer vs token, composed install (§2) | scripts/agents + adoption/up | fixture over `-m`, `-F`, `--amend`, `-c`, editor, pre-existing hooks |
| 3 | Sealed close: review.json digest in manifest; reviewedTree/baseTree/diff-sha on root record; schema fields added to terminalMetadataFields (§3.3) | internal/dispatch (close, record) | close-check fixtures incl. tamper-after-close |
| 4 | review.json `baseTree`; chainPaths via ChangedPaths (§3.4) | internal/validate (conformance) | fixture with quoted/newline paths and deletions |
| 5 | `refs/metasystem/reviewed/<root>` write, refusal-on-loss, archival delete (§3.5) | internal/validate + evidence retention | gc fixture: worktree removed, landing still resolves |
| 6 | Byte binding incl. mode/type and deletions; partial-landing refusal (§3.6) | internal/dispatch (landing-check) | blob/mode/delete fixtures |
| 7 | One-landing-per-chain origin scan; landedCommit bookkeeping patch (§3.7) | internal/dispatch + pre-push guard | crash-window and race fixtures |
| 8 | Critique-outcome check: critique ref + empty open-material register; floor rule for un-critiqued chains (§3.2) | internal/dispatch | fixtures: open-material chain, MECHANICAL chain on floor path |
| 9 | Class manifest schema, typed rules, fail-closed malformed handling (§4) | scripts/agents/landing-classes.json + landing-check | malformed-manifest table tests |
| 10 | Floor generation from both trees incl. §4's enumerated surface | internal/dispatch | fixture: floor entry added by fetched tip |
| 11 | Direct-fix proof verb, both-trees red/green, state assertions, nonce consume-on-use (§4) | new `proof direct-fix` verb | red-green and stale-proof fixtures |
| 12 | Defect-identity fuse aggregate over origin history (§4) | internal/dispatch | split-commit laundering fixture |
| 13 | `authorizedBy` rulings join for manifest changes (§4) | internal/dispatch | dangling-id fixture |
| 14 | Carriage allowlist semantics; rulings add-rows-only (§5) | landing-check | rulings-rewrite fixture |
| 15 | Fail-closed guards in enforce; observe fail-open; verb-ignorance refusal (§6, §8) | pre-commit/pre-push guards | missing-engine and stale-engine fixtures |
| 16 | Pre-push guard: outgoing-range verification, all push paths (§8) | new guard script + landing-check | fixtures: land.sh retry loop, commit.sh --push, raw wrapper push-later |
| 17 | Governance obligation registration + human-authorized promotion (§8) | internal/governance | promotion-without-authorization refusal fixture |
| 18 | Verdict trailer in observe; report incl. machine-scoped cross-check (§7, §8) | report verb | observe-window fixture |
| 19 | Hook install/compose in `metasystem up`; bootstrap-landing instruction-ledger entry (§8) | up + adoption | arming fixture with pre-existing hooks |

Build order (r3's five steps, mapped): (1) contracts are settled by
THIS document (§2 grammar, §4 manifest/proof schemas, §3.3 seal schema,
§2 hook lifecycle, this matrix); (2) the pure landing-check evaluator,
table-tested (rows 1, 6, 8-14); (3) seal, ref, and proof machinery
(rows 3-5, 11); (4) hooks and boundary integration (rows 2, 15, 16, 19);
(5) governance registration, observe evidence, report, and the
end-to-end common-path fixture proving the one-extra-flag claim
(rows 7, 17, 18).

## 12. Scope boundary: what this design deliberately does not remedy (TB-LP-R1-19)

The seat-governance record carries two open items
(plans/seat-governance-record.md:12-39, :113-148). This goal charters
the FIRST — the landing-provenance gap — and this design closes it. The
SECOND — the narrator-plus-accepting-custodian combination in one
actor — is deliberately NOT remedied here: the narrator digest's
carriage lane stays content-unverified (§5), and the actor that narrates
to Wido still performs the acceptance act. That is a scope boundary, not
an oversight: the choice of remedy (an actor boundary on narration, or
moving the acceptance act to another actor) is reserved to Wido, and the
debt is tracked where it lives — the governance record's second open
item, standing for his decision at the 2026-11-30 review (R-30-m1), or
sooner if he judges the interim intolerable. No mechanism in this design
claims to narrow it, and nothing here forecloses either remedy.

Also out of scope, unchanged from D90: the deliberate liar — hook
tampering, `--no-verify` by an agent, forged same-user records. The
adversarial tier (remote enforcement, durable identity) remains the
recorded separate goal (§9.2).

## 13. Self-grade (R-24)

- **Confidence:** high that the mechanism closes the chartered hole —
  after enforce, no agent landing reaches origin without either a
  closed AND cleanly examined chain's byte-bound candidate, or a typed,
  floor-checked, proof-backed, fused, permanently visible declaration;
  the push boundary makes the claim hold across rebases, all push
  paths, and the cutover. High on the line-level threat model (every §1
  claim was read from the current worktree bytes). Moderate on three
  operational points: (1) the pre-push guard's cost (an engine call per
  outgoing commit plus one trailer scan of origin history) is believed
  cheap but unmeasured — the observe window measures it; (2) how often
  the post-rebase blob re-check fires in real fleet traffic; (3)
  whether four classes and the initial fuse numbers (200 lines, 1
  subsystem) are the right partition — the observe window's durable
  verdict trailers are the evidence for adjusting both before enforce.
- **The weakest claim, declared:** that the restored proof-and-fuse
  machinery keeps the common mechanical fix near one extra step (one
  `proof direct-fix` run plus flags) rather than re-creating the
  ceremony the r3 anti-bureaucracy bar forbids. If the observe window
  shows honest fixes routinely fighting the proof verb, the state-
  assertion kind must be made cheaper before enforce — not waived.
- **Wido should reject this design if:** he judges the growth in
  checked machinery (two guarded boundaries, seal, ref, proof, fuse)
  past what R-11 tolerates for the accidental model — the fallback is
  the r1 shape, which the critique showed unsound at five structural
  points, so the real alternative is narrowing bar (b) to
  `register-carriage` + `revert-exact` only and forcing everything else
  through the loop; or if he wants the second governance open item
  remedied in the same slice rather than reserved (§12); or if he wants
  the adversarial tier now rather than as the recorded separate goal.
