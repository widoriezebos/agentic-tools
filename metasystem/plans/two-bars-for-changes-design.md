# Two bars for changes — landing provenance design

- Goal: two-bars-for-changes (plans/goals/two-bars-for-changes.md, revision 4)
- Status: design, round 3 — every ACCEPTED finding of the Sol round-2
  critique (records/two-bars/two-bars-lp-critique-r2.md, TB-LP-R2B-01
  through -14) folded; the round-1 disposition rows the round-2 critique
  proved overstated are corrected in §10.2. Finding trajectory:
  25 (round 1) → 14 (round 2) → this revision aims at zero.
- Mode: design only; no code in this change
- Carries: the D90 human ruling (accidental threat model) and the surviving
  decisions of the earlier r3 design record (records/two-bars/two-bars-design.md);
  every carried, restored, revised, or superseded decision is dispositioned
  in §10, per R-25b-m1.

## 0. The whole mechanism in one page (R-11)

Every landing from an agent-held checkout must present exactly one of:

- **Bar (a) — the loop.** `--chain <root-job-id>`: the landing binds to a
  CLOSED delegated chain whose independent critique COMPLETED and ended
  CLEAN (zero open material findings) — no chain lands under bar (a)
  without that examination, whatever its hazard class. The engine
  resolves the chain's reviewed candidate TREE digest (sealed write-once
  at close, pinned by a git ref) and checks byte-for-byte that every
  staged path the chain changed carries exactly the blob the reviewed
  tree carries. Stamp: `Landing-Provenance: chain=<root-id> tree=<40-hex>`.
- **Bar (b) — the declared direct fix.** `--direct-fix <class>`: a typed
  class (`register-carriage`, `revert-exact`, `mechanical-defect`). The
  engine checks the staged paths against the class's path rule and the
  NEVER-DIRECT-FIX floor (instruction-bearing paths plus the whole
  enforcement surface, read from BOTH trees). `mechanical-defect`
  additionally requires a defect identity, a consumed red-then-green
  proof pinned as a reachable git blob and cited in the trailer, and a
  per-identity growth fuse that blows when one "defect" quietly becomes
  a refactor. Stamp: `Landing-Provenance: direct-fix class=<class> ...`.
- **Bar (c) — refusal.** Neither declared, or a failed check: refuse,
  naming both bars and the offending paths. `loop` is never refused;
  only `direct-fix` is challenged; the cheap way out of doubt is the loop.

Enforcement lives at TWO boundaries, both fail-closed in enforce mode:
the COMMIT boundary (`commit.sh` calls the engine verb, mints the
trailer; a composed commit-msg hook proves the exact trailer set in the
FINAL message; the pre-commit guard refuses when it cannot classify or
cannot run the engine) and the PUSH boundary (the §8 evaluator — one
enumerated, exhaustive checklist — re-runs against each outgoing
commit's actual trees, with history-shaped invariants joined over
origin PLUS the whole outgoing range). The push evaluator runs from
BOTH the wrappers (land.sh, commit.sh --push — they ride the checkout,
so a rebase arms them) and a composed pre-push hook (covers raw `git
push`), against the remote tip git itself reports. A stale installed
engine fails closed on a policy-version handshake; it can never
silently pass old rules. Register carriage (the narrator digest,
findings, handoff notes, appended rulings rows) rides free via a small
allowlist. `metasystem report direct-fixes` reads the trailers, the
sealed close records, and the pinned proof blobs out of git for Wido's
review; in observe mode the commit-boundary verdict trailer plus a
machine-local push observation log make every would-refuse durable.
Promotion observe→enforce is a governance obligation
(internal/governance): DRAFT/OBSERVE cannot refuse; ENFORCED requires
the recorded human authorization the machinery already checks. Humans
stay sovereign (D90: the model is the honest agent forgetting, not the
hostile one).

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
Landing-Provenance: direct-fix class=<class-id> [defect-id=<slug>] [defect="<one line>"] [proof=<40-hex-blob-oid>] [revert-of=<40-hex-commit>]
Landing-Provenance-Verdict: pass | would-refuse code=<reason-code>   (observe mode, COMMIT boundary only, §8)
```

**Grammar, settled (TB-LP-R1-23).** `<root-job-id>` and `<class-id>`
match the existing job-id grammar (`validJobID`). `<slug>` is
`[a-z0-9-]{1,64}`. `defect=` is one double-quoted line: UTF-8, 1–200
bytes, and any byte below 0x20, any `"`, or the substring
`Landing-Provenance` inside the value REFUSES the commit — malformed
input is rejected, never escaped, so every parser sees one canonical
form. `proof=` is the 40-hex git blob OID of the canonical direct-fix
proof record (§4) — content-bound by construction (the OID is the hash
of the record's bytes) and required exactly for `mechanical-defect`.
`revert-of` must name a NON-MERGE commit (exactly one parent)
reachable from `origin/<branch>`; reverting a merge is ambiguous without
a parent choice and takes the loop. A missing required field, an unknown
field, or two provenance trailers is a refusal naming the field.

**Exact trailer cardinality, proved on the FINAL message (TB-LP-R1-16,
TB-LP-R2B-08).** The r1 argument-scan is withdrawn as enforcement:
`land.sh` passes `-F` plus a filename (land.sh:226-232), so an argument
scan reads the filename, not the message, and `-C`/`-c`/`--amend`/editor
paths never pass through arguments at all. Restored from the r3 record
(two-bars-design.md:24-31, :113-130): a composed `commit-msg` hook
validates the final message — the COMPLETE expected trailer block,
byte-equal to what the engine verb wrote into the wrapper token file
(the token commit.sh already mints, commit.sh:57-58, carries the
expected trailer bytes for this one commit; consume-on-use). The
cardinality rule is exact: exactly one `Landing-Provenance` trailer
always; exactly one `Landing-Provenance-Verdict` trailer when the
commit-boundary evaluation ran in observe mode; exactly zero in enforce
mode (a would-refuse never commits there). An author-supplied trailer,
a second trailer, a missing trailer, or any byte difference from the
token refuses at commit-msg. Hook composition follows the restored r3
lifecycle with the order pinned in §8: the pre-existing hook runs
FIRST (it may still rewrite the message), the provenance validator runs
LAST, over the final bytes.

- The caller supplies only flags: `--chain <root-id>` XOR
  `--direct-fix <class>`, plus `--defect-id <slug>`, `--defect <text>`,
  and `--proof <blob-oid>` (all three required for `mechanical-defect`)
  or `--revert-of <sha>` (required for `revert-exact`). Both `land.sh`
  and `commit.sh` accept them; `land.sh` passes them through to
  `commit_changes`.
- The `tree=` value is resolved by the engine from the chain's sealed
  close record, never from the caller.

Why a trailer and not a side file: the commit is the unit being governed;
a side file can desynchronize from history, needs its own carriage rules,
and duplicates what git already transports. The job records, the sealed
close facts, the pinned proof blobs, and the mirrored review artifacts
remain the deep evidence; the trailer is the join key.

## 3. Bar (a): binding to a closed, examined chain

New engine verb — working name `metasystem job landing-check` — owned by
`internal/dispatch`. Called at the commit boundary by `commit.sh` after
the settled-tree re-proof (commit.sh:235-245) and before `git commit`,
receiving the already-proved index tree; re-run at the push boundary
(§8) against each outgoing commit. Checks, in order, each with its own
refusal:

1. **Chain exists and is closed.** The root job record in this machine's
   `artifacts/agents/jobs/` carries `chainClosed: true`.
2. **The critique completed AND concluded clean — for EVERY bar (a)
   landing (TB-LP-R1-05, -08; TB-LP-R2B-03).** Closure alone is not
   examination, and bar (a) is the examined lane, full stop. The verb
   requires: the root record carries `independentCritiqueJobRef` (the
   reference hazard closure validates for DESIGN-BEARING and
   DESTRUCTIVE-REACH chains, hazard.go:261-301), AND the chain's
   canonical critique register reports zero open material finding
   identifiers (the existing `job critique-open-finding-ids` join). A
   chain with a critique job but no folded register fails closed. A
   chain WITHOUT a completed independent critique — a MECHANICAL chain
   (hazard.go:36-41), or a chain whose critique was waived
   (conformance.go:532-600) — cannot land under bar (a) AT ALL, on any
   path: the round-2 lane that let such a chain land non-floor paths
   was the TB-LP-R1-08 bypass with a smaller path set, and it is
   deleted. The remedy the refusal names: dispatch an independent
   critique over the chain's final work round (the hazard machinery
   already validates exactly that reference on any chain that carries
   it), fold to a clean register, and land; or, when the change fits a
   bar (b) class, declare it. Hazard-class CLOSURE minimums are
   unchanged — landing-check imposes its own requirement at its own
   boundary; it does not rewrite `requiredConfigurationByHazard`.
3. **Candidate facts come from the CLOSE-TIME SEAL, write-once and
   close-owned (TB-LP-R1-04, -17, -22; TB-LP-R2B-02).** Closing gains
   three named obligations: (i) close-check verifies `review.json` is
   mirrored with a matching digest exactly as it already verifies
   `diff.patch` (close.go:57-83); (ii) a DEDICATED close-seal operation
   — the same locked read-decide-write species as the register's other
   owned metadata (record.go:80-87) — stamps `reviewedTree`,
   `baseTree`, and `diffPatchSha256` onto the root record in the same
   close step that sets `chainClosed`. The three fields join
   `dedicatedMetadataFields` (record.go:80-87), NOT
   `terminalMetadataFields`: the generic same-status RecordCAS path
   refuses every dedicated field today (record.go:526-528), so no
   generic record patch can rewrite the seal after close — the round-2
   placement in the terminal-field allowlist was precisely the
   rewritable lane RecordCAS accepts (record.go:530-535) and is
   withdrawn. The dedicated operation is write-once: it refuses when
   any of the three fields is already present, and no operation
   updates them afterward — evidence drift is a refusal at landing,
   never a re-seal. (iii) `review.json` gains a `baseTree` field — the
   boundary base tree the review stage already resolves
   (conformance.go:274) — written beside `reviewedTree`
   (conformance.go:439-443). `landing-check` reads `reviewedTree` and
   `baseTree` from the sealed root record and verifies the local
   `review.json` and `diff.patch` digests still match the close-time
   seal and the durable mirror manifest; any mismatch refuses as
   evidence tampering/loss, never silently re-resolves.
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
7. **One landing per chain — a whole-range history invariant
   (TB-LP-R1-13; TB-LP-R2B-10, -14).** The authoritative invariant: at
   most one commit in the union of origin history and the COMPLETE
   outgoing range carries this chain's trailer. The push evaluator (§8)
   joins across that union — so two outgoing commits citing one chain
   refuse together, not pass separately — and refuses any outgoing
   commit whose `chain=<root-id>` already appears in a commit reachable
   from the authoritative remote tip. This needs no atomicity with the
   push: a crash after push leaves the trailer in origin, so the retry
   refuses; a crash before push leaves nothing, so the retry lawfully
   lands; two racing machines resolve through the push fast-forward
   rule — the loser rebases, re-checks, sees the winner's trailer, and
   refuses. There is NO post-push record write: the round-2
   `landedCommit` bookkeeping patch is withdrawn because its expressly
   supported commit-then-push-later path has no post-success owner —
   the report derives a chain's landing commit from the origin trailer
   scan it already performs (§7), which is correct on every push path
   by construction. No `terminalMetadataFields` change exists in this
   design.

**Rebase behavior (the fleet's moving tip).** The tree digest is
rebase-stable, but a rebase can change landed bytes: when origin moved
inside a file the chain also touched, the rebase textually merges and
the resulting blob differs from the reviewed blob. The r1 answer —
re-check inside `land.sh` only — governed one caller and left
`commit.sh --push` and commit-then-push-later unchecked (TB-LP-R1-03).
The answer since round 2: every outgoing agent commit is re-verified
against its ACTUAL post-rebase trees before any push, by every push
path (§8). `land.sh` keeps an early in-loop re-check after each rebase
(land.sh:272, :289-290) purely for a better message closer to the
cause; the push evaluator is the enforcement. On a bar (a) mismatch the
refusal names the diverged paths and the remedy: the rebased content is
no longer the examined content — dispatch a follow-up round on the
current tip, re-run conformance and the required critique, close, and
land again. Rebasing never silently re-binds; it either preserves the
reviewed blobs or reopens examination. Direct-fix commits are ALSO
re-checked at the push boundary against their post-rebase trees
(TB-LP-R1-02): class rule and floor are re-evaluated from the outgoing
commit's parent tree and its own tree (§4's both-trees rule), and the
`mechanical-defect` proof re-binds by change set, or refuses, under the
explicit rule in §8 (TB-LP-R2B-09).

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
commit-msg and pre-push guard scripts, `scripts/agents/go-gate.sh` (it
builds the proof engine), `internal/dispatch/`, `internal/governance/`,
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
`{"schemaVersion": 1, "enginePolicyVersion": <int>, "classes": [{"id",
"pathRule", "requiredFields", "fuse", "authorizedBy"}]}` where
`pathRule` is one of the three TYPED rules below (never a free
pattern), `requiredFields` lists the trailer fields the class demands,
`fuse` (only on `mechanical-defect`) is `{"maxAggregateLines": <int>,
"maxSubsystems": <int>}`, `authorizedBy` names the rulings-register row
(R-id) that authorized the class (checked as below), and
`enginePolicyVersion` is the policy-surface handshake §8 reads
(TB-LP-R2B-06). Duplicate ids, an unknown `pathRule`, a missing field,
unparseable JSON, or an unreadable file: every direct fix refuses (fail
closed); bar (a) is unaffected. The carriage allowlist
(`scripts/agents/register-carriage-paths.txt`) contains exact
repository-relative paths, one per line, plus at most a trailing
basename glob of the single form `<dir>/<prefix>-*.md` — no other
wildcard semantics exist (TB-LP-R1-21's mixed-wildcard ambiguity is
closed by construction).

**Initial classes** (the manifest ships with exactly these three):

| id | path rule (typed) | required fields | evidence |
| --- | --- | --- | --- |
| `register-carriage` | every staged path on the carriage allowlist (§5) | none | none |
| `revert-exact` | tree-shaped inverse with the postimage precondition (below); floor paths still refuse | `revert-of=<sha>` | the inverse check itself |
| `mechanical-defect` | no staged path on the floor | `defect-id=<slug>`, `defect="<one line>"`, `proof=<blob-oid>` | a consumed, pinned direct-fix proof (below) |

**`prose-docs` is withdrawn (TB-LP-R1-06, -07; TB-LP-R2B-04).** The
round-2 class admitted every non-floor `.md` outside `plans/` with no
evidence at all — a complete honest-misclassification route: a design
document under `docs/design/` declared "prose" is exactly the
laundering the bars exist to stop, and no path rule can tell a design
change from a typo. The class is removed from the manifest rather than
patched, because the predecessor's requirement is mechanically
decisive evidence or no direct fix. Documentation changes now take one
of the surviving lawful routes: a genuine documentation DEFECT (a typo,
a stale path, a broken link) is a `mechanical-defect` with a `state`
assertion (red: the defective bytes are present; green: they are not)
— mechanically decisive, fused, and visible; NEW documentation content
is authored work and takes the loop. If the observe window shows this
costing more than the fleet bears, the lawful adjustment is a new
manifest class landed through bar (a) with its own `authorizedBy`
ruling — never a quiet re-widening.

**`revert-exact`, with the clobber precondition (TB-LP-R2B-13).**
Defined tree-shaped so it is rebase-stable and re-checkable at the push
boundary, in two parts:

- *Precondition — nothing later touched the reverted paths:* for every
  path in the named commit's change set
  (`ChangedPaths(revert-of^, revert-of)`), the reverting commit's
  PARENT tree must record exactly the entry (mode + OID) that
  `revert-of` recorded — the postimage. A path that any later commit
  changed fails the precondition, and the refusal names the path and
  the remedy (the revert takes the loop, or the specific defect takes
  `mechanical-defect` with its own proof). Without this, a mechanically
  declared revert of an old commit could erase legitimate later work —
  the round-2 rule permitted any reachable non-merge commit with no
  parent-state check at all.
- *The inverse:* for every path in that change set, the reverting
  commit's own tree must record exactly the entry that `revert-of^`
  recorded — including absence for paths the named commit added — and
  the reverting commit's change set must contain no other non-carriage
  paths. The named commit has exactly one parent (§2).

**The direct-fix evidence bar, restored whole (TB-LP-R1-15, -17;
TB-LP-R2B-05; r3 keep-list two-bars-design.md:38-49, :163-178).**
`mechanical-defect` requires a proof, produced before the commit by a
new engine verb `metasystem proof direct-fix`, durable and joinable:

- **The proof record** is one canonical JSON document with fields
  `{schemaVersion, defectId, assertionKind, assertion, baseTree,
  candidateTree, redOutcome, greenOutcome, redEvidence, greenEvidence,
  gate, createdAt, nonce}` — one reusable assertion evaluated against
  BOTH immutable trees: red (the assertion fails / the bad state holds)
  against `baseTree` (HEAD at proof time), green against
  `candidateTree` (the proved index tree). `assertionKind` is `test`
  (one named test command, accommodating a newly added regression test,
  per the r3 definition) or `state` (a structured before/after
  repository-state assertion — the first-class proof kind for the
  no-failing-test cases: a stray binary, unstaged files, a doc typo; r3
  TB-R1-07). `redEvidence` and `greenEvidence` are the 40-hex git blob
  OIDs of the captured red and green execution transcripts, written by
  the verb into the object database — the restored evidence hashes.
  `gate` is the restored gate-witness block: `{command, engineSha256,
  outcome, tree}` — the proof verb runs the commit boundary's own
  static gate (`go-gate.sh --fast` plus the audit, the same pair
  commit.sh:114-133 and :228-234 run) against the candidate and
  records the exact command identity, the sha256 of the proof-built
  engine, the final zero outcome, and the tree it judged. §10.1
  dispositions how this revises the carried full-suite witness.
- **Durability and the join (the round-2 record was neither).** The
  verb writes the canonical record as a git blob (`hash-object -w`) and
  pins it — together with its two evidence blobs — with one ref,
  `refs/metasystem/proof/<record-blob-oid>`: reachable, gc-safe,
  machine-local, never pushed, deleted by the same evidence-retention
  archival step as the reviewed-tree refs (§3.5) — the identical
  pinning pattern, not a new subsystem. The trailer's
  `proof=<record-blob-oid>` (§2) is the content-bound join key: the
  report resolves the commit's proof with one `git cat-file` and can
  verify every retained field. The round-2 design's trailer carried no
  proof identifier and its local record was joinable to nothing; both
  defects are closed by the blob OID in the trailer.
- **Consumption — one proof, one landing, enforced where it is
  observable.** Commit boundary: `landing-check` requires
  `candidateTree` to equal the proved index tree and the machine-local
  consumption note (`artifacts/agents/proofs/<record-blob-oid>.json`)
  to be unconsumed, then stamps `consumedBy` there — the fast, early
  refusal for accidental reuse. Push boundary (authoritative, §8): at
  most one commit in origin history plus the outgoing range carries
  this `proof=` OID — the same whole-range uniqueness join as
  one-landing-per-chain, so a crashed or raced consumption note can
  never double-land a proof. A stale or consumed proof refuses with
  the one-step remedy (re-run the proof verb).
- `register-carriage` and `revert-exact` need no proof: their evidence
  is their path rule or tree equality itself. This is the honest scope
  of the r1 "superseded wholesale" claim, corrected: the closure-gate
  artifacts replaced the witness machinery for bar (a) ONLY; bar (b)'s
  evidence bar is restored, not superseded.

**The growth fuse, joined over the whole range (TB-LP-R1-24;
TB-LP-R2B-10; r3 two-bars-design.md:152-161).** Every
`mechanical-defect` declares `defect-id`; the engine aggregates — across
every commit citing the same `defect-id` in the union of commits
reachable from the authoritative remote tip AND the complete outgoing
range, plus (at the commit boundary) the staged change — the total
changed lines (numstat) and the set of subsystems touched (subsystem :=
first path segment under the installation prefix), measured against the
immutable pre-defect base (the parent of the first commit in that union
citing the id). The whole-range join means splitting one defect across
two outgoing commits cannot undercount the fuse: each per-commit check
sees the union. The fuse blows — refusal, naming the aggregate and the
loop as the remedy — when the aggregate exceeds the manifest's
`maxAggregateLines` (initial: 200) or the subsystem count exceeds
`maxSubsystems` (initial: 1). A blown fuse is not an error state to
clear; the remaining work takes bar (a).

**Who may extend the list and how — the human word, TYPED and
PRE-RECORDED (TB-LP-R1-11; TB-LP-R2B-11).** The manifest and the
allowlist sit on the floor, so a change to either takes bar (a) through
an independently critiqued chain (§3.2). The human word is additionally
a mechanical join with two teeth the round-2 check lacked:

- *The cited ruling must pre-exist in ORIGIN.* For every class row the
  landing adds or modifies, `authorizedBy` must name a rulings-register
  row that exists in `memory/rulings.md` as read from the authoritative
  remote tip's tree — commits already accepted by origin — never from
  the candidate tree and never from the same outgoing range. The
  round-2 candidate-tree read let one landing append its own row
  through carriage and cite it, collapsing the human key into the
  examination key; origin-reachability restores the order Law 2
  requires (plans/seat-governance-record.md:191-195): Wido's word is
  recorded and landed FIRST (rulings rows ride carriage, §5), the
  machinery change lands after, citing it.
- *The ruling must name the exact object.* The cited row's text must
  contain the literal token `landing-class=<class-id>` for the class it
  authorizes. An existing but unrelated row no longer passes; the check
  stays mechanical (one row lookup, one substring test) and the burden
  falls on the mint, where the human is already writing the ruling.
- *Removal narrows; it needs no row of its own.* Deleting a class only
  shrinks what direct fixes can do — the conservative direction — and a
  deleted row can carry no field; removal is governed by bar (a)'s
  critiqued chain alone. The observe→enforce promotion is governed
  separately by internal/governance's recorded human authorization
  (§8), unchanged.

**Honest bounds.** The classes are path- and evidence-shaped, not
semantic: within its path rule, `mechanical-defect` cannot tell a
design change from a mechanical one — what it CAN demand is a declared
defect identity, a red-then-green proof over immutable trees, a fuse,
and permanent visibility, which is the accidental model's answer (D90):
an honest seat cannot *forget* to choose a bar, a wrong choice is a
named greppable act with evidence attached, and a "small fix" cannot
quietly become a refactor. What remains out of scope is the deliberate
liar — §12.

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
  is closed). The rule is re-checked at the push boundary against each
  outgoing commit's own parent-to-commit diff (§8). The existing
  mint-suffix check remains in force.
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
      classes: register-carriage | revert-exact | mechanical-defect
  neither fits? the change takes the loop: dispatch it, close the chain, land with --chain
```

A failed check keeps the header and appends the exact cause with paths,
e.g. `chain q…z is not closed`, `chain q…z has no completed independent
critique`, `chain q…z has open material findings`, `staged path X
differs from reviewed tree <digest>`, `path Y changed after <revert-of>
— revert-exact would erase later work`, `chain q…z already landed as
<sha>`, `path Z is on the never-direct-fix floor`, `defect-id lease-scan
aggregate 312 lines exceeds the 200-line fuse`, `proof <oid> already
consumed by <sha>`. Every refusal is one screen, names the offending
object, and names the lawful next step.

**Fail-closed chokepoints (TB-LP-R1-01).** The r1 design inherited the
guard's deliberate fail-open (pre-commit-guard.sh:29-39: no engine or a
classification error leaves the commit untouched) while claiming the
wrapper was unavoidable — under enforcement those cannot both be true.
Revised rule: when enforcement is in force (§8), the composed guards
FAIL CLOSED — an absent or non-executable engine, an engine that does
not know the landing-check verb, an engine that fails the
policy-version handshake (§8), or a classification error refuses the
commit or push with the remedy named (`build bin/metasystem` / `run
metasystem up`). Human sovereignty is preserved without a classifier: a
positively classified HUMAN caller passes as today, and the sovereign
escape for a human on a broken checkout is `git commit --no-verify` — a
deliberate act that D90 places out of the accidental model for agents
(an agent using it is the adversarial tier, recorded, not defended
against). In observe mode the guards keep today's fail-open, so a
half-built checkout cannot brick landings before the mechanism has ever
been armed. Human-classified callers are never refused by the bars; a
human commit through `commit.sh` still gets the trailer stamped when
flags are present.

## 7. Audit surface

The aggregate IS git history plus one reader — with its evidence scope
stated honestly (TB-LP-R1-20):

- `metasystem report direct-fixes [--since <date>]` scans
  `git log origin/<branch>` for `Landing-Provenance` trailers and prints
  per-class, per-machine, and per-defect-id counts plus one line per
  commit (sha, class, defect fields, proof OID, subject).
- **Integrity cross-check, machine-scoped.** Job records, proof
  consumption notes, and review artifacts are machine-local, so one
  machine can verify only its own landings (the r1 claim to cross-check
  EVERY chain trailer was not implementable). The report verifies
  trailers whose `Machine:` trailer names this machine's enrolled
  nickname (commit.sh:260-263): for bar (a), the root record exists,
  `tree=` matches the sealed `reviewedTree`, and the chain's landing
  commit — DERIVED from this same origin trailer scan, the only
  authority; there is no recorded `landedCommit` field (§3.7,
  TB-LP-R2B-14) — is unique in history; for `mechanical-defect`, the
  `proof=` blob resolves (one `git cat-file`), its fields match the
  trailer, and its red/green evidence blobs resolve. Peer machines'
  trailers are listed as `peer-scope: verify on <machine>` — a stated
  boundary, never a false integrity finding. The fleet picture is the
  union of the four per-machine reports at Wido's review cadence; that
  union is a read, not a new replication mechanism.
- **The observe-window evidence** is two-part (§8, TB-LP-R2B-08): the
  commit-boundary verdict trailers in history, plus each machine's
  push-boundary observation log (`artifacts/agents/landing-observe.log`)
  — the report prints unresolved push-time `would-refuse` lines beside
  the trailer counts, so a push-time failure that reached origin behind
  a commit-time `pass` is visible, not lost.
- **The durable audit join (TB-LP-R1-17), restored from r3
  (two-bars-design.md:180-193):** the join key (trailer) plus the
  write-once close-time seal on the root record (§3.3), the
  reviewed-tree ref (§3.5), and the pinned proof blobs (§4) make
  provenance reconstructible after local round files are archived; and
  the new global rule itself gets an instruction-ledger entry
  (`plans/instruction-ledger.md`) with expected effect and a later
  verdict, written as part of the rollout's first step (§8). A defined
  history-audit pass — the report over `--since` the enforce flip —
  names any commit whose provenance cannot be reconstructed, noting (as
  r3 did) that a missing classification is indistinguishable from a
  sovereign human commit after the fact.
- Readers: Wido at the review cadence
  (plans/seat-governance-record.md:215-218), and the retro. A repeated
  `mechanical-defect` cluster on one subsystem now trips the FUSE
  mechanically (§4) before it becomes a retro finding.
- No new ledger file for landings. A parallel append-only landing
  ledger was considered and rejected (§9.3); the observation log is
  observe-mode measurement data, machine-local and non-authoritative,
  not a provenance ledger.

## 8. Enforcement points, engine identity, and migration

**Two boundaries, one rule.** The decision check runs at the COMMIT
boundary (mint) and the PUSH boundary (verify), never as shell logic.

**Commit boundary.** `commit.sh` calls `job landing-check` on the
proved index (§3, §4); the commit-msg hook proves the final message
(§2); the pre-commit guard proves wrapper ancestry (§6). Engine
identity here: the PROOF-BUILT engine from `go-gate.sh --fast`
(commit.sh:114-133) — the existing deliberate choice that a policy edit
is judged by the policy in the prospective bytes; a floor-touching
change reaches this point only through an independently critiqued chain
(§3.2), so the self-judging window is examined bytes judging
themselves, which the accidental model accepts.

**Push boundary — one exhaustive evaluator (TB-LP-R1-03, -10;
TB-LP-R2B-09, -10, -12).** The evaluator is
`job landing-check --push-range <remote-sha> <local-sha>`, and its
checklist is EXHAUSTIVE — an implementation that omits a numbered item
does not conform. Let R be the authoritative remote tip and, for each
outgoing commit C (oldest first), P its parent:

1. *Authoritative tip (TB-LP-R2B-12).* In the pre-push hook, R is the
   remote SHA git itself supplies on the hook's standard input for the
   ref being pushed — the advertised remote state the push will be
   applied to; no fetch happens inside the hook and no remote-tracking
   ref is trusted. If R is not all-zeros and not present in the local
   object database, fail closed naming `git fetch origin` (the push
   would be refused as non-fast-forward regardless). If R is all-zeros
   (a new ref), origin-history joins are vacuous and the outgoing range
   is every commit reachable from the local SHA and from no
   `refs/remotes/origin/*`. In the wrappers (below), R is the tip their
   own explicit fetch just wrote.
2. *Policy handshake (TB-LP-R2B-06).* The engine's compiled
   `landingPolicyVersion` must be ≥ the manifest's
   `enginePolicyVersion` as read from R's tree AND from each C's tree.
   A lower engine version fails closed naming the rebuild remedy. This
   closes the round-2 hole where verb ignorance caught only a
   pre-landing-check binary: any later rule change bumps
   `enginePolicyVersion` in the same landing that changes the rules
   (both ride the floor, so both arrive through one critiqued chain),
   and a gitignored stale binary (.gitignore:2) that knows the verb but
   not the rules now refuses instead of passing. A binary too old to
   know the handshake at all fails the verb-ignorance check already in
   force (§6).
3. *Trailer shape.* Exactly one grammar-valid `Landing-Provenance`
   trailer on C; at most one `Landing-Provenance-Verdict` trailer; in
   enforce mode an outgoing C carrying `would-refuse` refuses (an
   unresolved observe-time failure never crosses the flip).
4. *Bar (a) binding.* §3's checks 1–6 against C's ACTUAL trees: chain
   closed, critique completed and clean, seal digests intact, byte
   binding of `ChangedPaths(P, C)` against `chainPaths` and
   `reviewedTree`, carriage partition.
5. *Bar (b) class and floor.* Class rule from the manifest and floor
   from BOTH trees (P and C), per §4.
6. *Direct-fix proof binding, with the rebase rule (TB-LP-R2B-09).* At
   the commit boundary the proof binds exactly: `candidateTree` equals
   the proved index tree. At the push boundary that equality is
   generally false after any rebase (the reapplied commit's tree
   includes origin's other changes), so the proof re-binds by CHANGE
   SET: for every path in
   `ChangedPaths(proof.baseTree, proof.candidateTree)`, C's tree must
   record exactly `proof.candidateTree`'s entry (including absence),
   and `ChangedPaths(P, C)` minus carriage must be a subset of the
   proof's change set. A rebase that textually merged a proved path
   breaks the entry equality and refuses with the one-step remedy: the
   assertion is reusable by design — re-run `proof direct-fix` on the
   current tip and re-mint the commit through the wrapper. Rebinding is
   never silent.
7. *Whole-range uniqueness (TB-LP-R2B-10).* One-landing-per-chain
   (§3.7) and one-landing-per-proof (§4) joined over commits reachable
   from R PLUS the complete outgoing range.
8. *Whole-range fuse (TB-LP-R2B-10).* The §4 aggregate over the same
   union.
9. *Manifest authorization (TB-LP-R2B-09, -11).* If
   `ChangedPaths(P, C)` touches `landing-classes.json`: C must be a bar
   (a) landing (the manifest is on the floor), and every added or
   modified class row's `authorizedBy` must resolve per §4 against
   `memory/rulings.md` read from R's tree.
10. *Rulings carriage (TB-LP-R2B-09).* If `ChangedPaths(P, C)` touches
    `memory/rulings.md`, the §5 add-rows-only rule applies to the P→C
    diff.
11. *Mode.* Enforcement mode is read by the engine from the checkout's
    committed governance state. Observe: evaluate everything, append
    one line per outgoing commit to the observation log (below), never
    refuse. Enforce: refuse on the first failure, naming it.

The §0 sentence "the push boundary re-runs the checks" means exactly
this list — the round-2 one-page claim of "every check" against a
three-item detailed list was a contradiction, resolved by making the
detailed list exhaustive and the one-pager point here.

**Who runs the evaluator (TB-LP-R2B-07).** Three callers, two of which
need no installation step:

- `land.sh`: after its final rebase, inside the push-retry loop, before
  each `git push` (it already fetches there, land.sh:289-290).
- `commit.sh --push`: gains one `git fetch origin <branch>` before its
  push (today it fetches nothing, commit.sh:278-285), then runs the
  evaluator over the fetched range. Both wrappers ride the CHECKOUT, so
  a rebase alone arms them — the round-2 cutover claim becomes true for
  every wrapper-mediated push without any hook installation.
- The composed pre-push hook: covers raw `git push` (including
  commit-then-push-later), using stdin's R per item 1. Engine identity
  for all three push callers is FIXED (TB-LP-R1-10): the installed
  `bin/metasystem`, fail-closed when absent, non-executable, ignorant
  of the verb, or version-behind the handshake — never a rebuild from
  candidate bytes at push time (a push must not compile code, and the
  prospective-bytes argument does not apply: the pushed commits are
  already committed history).

**Hook lifecycle, pinned (TB-LP-R1-12, -16; TB-LP-R2B-07).**

- *Where hooks go:* installation resolves the hooks directory through
  `git rev-parse --path-format=absolute --git-path hooks`, exactly as
  goal-fence enrollment already does (goalsync_verbs.go:67-80), so
  `core.hooksPath` is honored; a hooks directory outside both the
  repository's common dir and its toplevel is SHARED and refuses
  installation with the same named remedy enrollment uses
  (goalsync_verbs.go:81-99).
- *Composition order:* the composed wrapper invokes any pre-existing
  hook FIRST and the metasystem guard LAST — for commit-msg this is
  what makes the validation bind the FINAL bytes: no later hook can
  rewrite the message after the trailer check (the round-2 lifecycle
  left this unordered).
- *When hooks arm:* `metasystem up` installs/composes pre-commit,
  commit-msg, and pre-push at EVERY session start (idempotent), which
  also heals a post-adoption `git init` at the next session; and up's
  arming step PROBES each installed hook with the existing
  `METASYSTEM_GUARD_PROBE` nonce mechanism (pre-commit-guard.sh:21-24,
  exit 42 acknowledgment) — a hook that does not acknowledge fails the
  arming step as a typed up outcome. Arming is an explicit, observable
  act, not an assumption about loaded shells.

**Promotion is a governance obligation, not a bare key
(TB-LP-R1-11, -25).** `landing.provenance` is registered as an
obligation in the existing internal/governance state machine
(ObligationObserve / ObligationEnforced, types.go:14-16): DRAFT and
OBSERVE record would-refuse outcomes and cannot refuse; ENFORCED
requires the complete recorded human authorization the machinery
already validates (types.go:145-173), checked by the guards at the base
action boundary — the seat cannot flip enforcement by editing a config
line, because the obligation state and its authorization are what the
check reads, and the state file rides the floor.

**Observe mode produces durable evidence at BOTH boundaries
(TB-LP-R1-25; TB-LP-R2B-08).** The two boundaries run at different
times, so one trailer cannot honestly attest both; the evidence is
split with each half durable where it can be:

- *Commit boundary:* the verb stamps, beside the provenance trailer,
  `Landing-Provenance-Verdict: pass` or `would-refuse
  code=<reason-code>` — scoped, by definition, to the COMMIT-time
  evaluation of the proved index. It rides history; cardinality per §2.
- *Push boundary:* the evaluator appends one line per outgoing commit
  to the machine-local, append-only observation log
  `artifacts/agents/landing-observe.log`
  (`<date> <sha> push <pass|would-refuse code=...>`), including raw
  commits that carry no trailer at all (in observe, the composed hooks
  log what they would have refused instead of refusing) — the
  round-2 design had no durable record for a push-time failure and
  claimed the commit-time trailer covered it, which it cannot: the
  trailer is minted before the rebase the push check judges.

Promotion evidence is mechanical and two-part per machine: clean
verdict trailers in origin history over the observe window for every
landing shape that machine uses, AND zero unresolved `would-refuse`
lines in its observation log (§7 prints both). A console line proves
nothing; the trailer and the log prove the window. In enforce mode the
verdict trailer is not stamped (a would-refuse never commits) and the
log is not written (a would-refuse never pushes).

**Migration for four machines (no flag day), with the bootstrap named
(TB-LP-R1-12; TB-LP-R2B-07).**

1. **Bootstrap exception, explicit and singular:** the landing that
   introduces the mechanism is executed by the old wrapper and cannot
   check or stamp itself. It is landed as an ordinary bar-(a)-in-spirit
   landing — a closed, critiqued chain, with the chain id named in the
   commit message body — and this design records that one landing as
   the mechanism's ungoverned genesis. No general exception exists;
   the exception is this named commit, once. The same landing writes
   the instruction-ledger entry (§7).
2. Land in OBSERVE (the obligation's initial state). A machine picks
   the mechanism up through two channels, both named and now honestly
   scoped: its next fetch-rebase brings the scripts — which arms the
   WRAPPER-side evaluator immediately, because the wrappers ride the
   checkout and need no installation — and its next `metasystem up`
   installs, composes, and PROBES the hooks, which is the point
   raw-git coverage arms. The round-2 claim that a rebase alone
   delivered the full flip was wrong precisely because hook
   installation is a separate act (goalsync_verbs.go resolves a hooks
   directory a rebase never writes); the split above is the corrected
   statement.
3. Run observe until each machine's history and observation log show
   the §8 two-part evidence. Flip to ENFORCED through the governance
   machinery's recorded human authorization (Wido's word). The flip's
   prerequisite is per-machine evidence, and the design is honest
   about what is NOT mechanically provable fleet-wide: hook arming is
   machine-local state with no shared registry (building one was
   considered and rejected, §9.7), so Wido flips on the four
   per-machine observe reports, each of which includes that machine's
   up arming outcome.
4. **The residual skew window, honestly bounded:** wrapper-mediated
   pushes cannot skew — the wrapper enforcement travels with the
   checkout the rebase just updated. The residual is a machine that
   raw-pushes without having run `up` since the flip: no pre-push hook
   runs. Its wrapper-made commits were still checked and stamped at
   commit time; what it can miss is the post-rebase re-verification
   and the whole-range joins, until its next session start arms the
   hooks. That window is the accidental model's accepted residue —
   session starts run `up`, so it closes at the machine's next
   session, and the §7 history audit sees anything that crossed it.
   No landing ever fails because a *peer* machine is behind; all
   checks read machine-local state and the authoritative remote tip.

Rollback, if enforce proves wrong in practice: the obligation returns to
OBSERVE by the same recorded human word. The observe branch is the
enforce branch minus the exit code (plus the verdict trailer and log).

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
   parallel it. Rejected; the trailer is the join key. (The observe
   log is not this: it is machine-local measurement, never authority.)
4. **Enforce in `land.sh` only.** Governs one caller of the boundary
   and dies at land-verb-pruning. Rejected; the boundaries are the
   commit wrapper and the push evaluator's three callers (§8).
5. **Rebuild the engine from candidate bytes at the push boundary.**
   Would let an enforcement change judge itself with unexamined code
   and make every push a compile. Rejected; the push-boundary engine is
   the installed binary, fail-closed on absence, verb ignorance, or a
   failed policy-version handshake (§8, TB-LP-R1-10, TB-LP-R2B-06).
6. **A cryptographic seal on review.json.** Same-user signatures prove
   nothing the accidental model needs; the write-once close-seal onto
   the root record plus mirror-manifest digests catches accidental and
   after-the-fact drift, which is the governed threat (§3.3).
7. **A shared fleet registry of hook-arming state.** Would make the
   enforce flip mechanically provable fleet-wide, but adds a second
   cross-machine writer for one bit per machine that the per-machine
   observe report already carries. Rejected (§8.3); the flip is a
   human act over four per-machine reports.
8. **Keeping `prose-docs` with a narrower path rule.** Every narrowing
   still admits some directory where design prose lives, because the
   distinction is semantic and path rules are not. Rejected; the class
   is withdrawn and doc defects take `mechanical-defect` with a state
   assertion (§4, TB-LP-R2B-04).
9. **A `landedCommit` bookkeeping field on the root record.** Its only
   honest writer would be a post-push driver that the supported
   commit-then-push-later path does not have; a field that is
   sometimes absent for lawful landings poisons the integrity report.
   Rejected; the report derives the landing commit from the origin
   trailer scan (§7, TB-LP-R2B-14).

## 10. Dispositions

### 10.1 The carried r3 design record (R-25b-m1)

- **Carried unchanged:** the D90 accidental threat model and human
  sovereignty; fail-closed with `loop` never refused; the
  never-direct-fix floor as a conservative FLOOR read from both trees
  (TB-R1-01); emergency = human-personal, no agent override (TB-R1-06);
  the anti-bureaucracy bar — the common case is one flag plus, for
  `mechanical-defect` only, one proof-verb run (TB-R1-07).
- **Restored (round 2 began this; round 3 completes it):** the
  composed pre-commit + commit-msg hook enforcement of the final
  message with the composition ORDER pinned (TB-R1-02, §2, §8); the
  direct-fix evidence bar — red-then-green assertion against both
  immutable trees, structured state assertions, evidence-transcript
  hashes, a gate-witness block, nonce with consume-on-use, and a
  content-bound REACHABLE proof reference in the trailer
  (TB-R1-03/-05/-07, §4); the defect-identity growth fuse over
  reachable history, now joined over the whole outgoing range
  (TB-R1-04, §4); the durable audit join — reachable references,
  critique-closure join, instruction-ledger entry, defined history
  audit (TB-R1-05, §3, §4, §7); the settle-the-contracts build gate
  and the design-obligation matrix in the validator's canonical
  columns (§11).
- **Revised, openly (design-lane act under R-25b-m1, for Wido's
  review):** the carried keep-list's "full-validator zero-outcome
  identity" (two-bars-design.md:38-49) demanded the witness be
  finalized by the whole validator at the end of the full suite. Per
  direct fix, that would put an hours-long full-suite run inside the
  proof verb, which the equally carried anti-bureaucracy bar (TB-R1-07)
  forbids; the two carried decisions conflict at this point. This
  design resolves the conflict in favor of the anti-bureaucracy bar:
  the proof's `gate` block records the identity, engine digest, tree,
  and final zero outcome of the commit boundary's own static gate
  (go-gate --fast plus the audit — the pair every landing already
  passes, commit.sh:114-133, :228-234), and full validation remains
  the separately governed weight-triggered run it is today. The
  full-suite witness question stays with the D33 witness design where
  it lives. This is a recorded revision of a carried decision, not a
  silent weakening; Wido may overrule it at review.
- **Superseded by goal revision 4, strictly stronger, scope stated
  honestly:** `Design-Chain:` string references and the bespoke
  full-validator gate-witness machinery are replaced FOR BAR (a) ONLY
  by the closure gate's own sealed artifacts (`chainClosed`, the
  write-once seal, the critique register). Bar (b)'s witness machinery
  is not superseded; it is restored above.

### 10.2 The Sol landing-provenance critique, round 1 — corrected

The round-2 revision claimed all 25 ACCEPTED findings folded; the
round-2 critique proved eleven of those claims overstated. The rows
below are the corrected record: rows the round-2 critique did not
contradict are stated as folded in round 2; every row round 2
overstated says so, and names the round-3 fold that completes it.

- **TB-LP-R1-01** — folded r2 §6: in enforce, the guards fail CLOSED on
  a missing/non-executable engine, an unknown verb, or a classification
  error; observe keeps today's fail-open; the human escape is the
  sovereign `--no-verify`, out of the accidental model by D90. (Round 3
  adds the policy-handshake failure to the fail-closed causes, §8.)
- **TB-LP-R1-02** — r2 overstated (TB-LP-R2B-09): the push re-check was
  claimed but the evaluator was not exhaustive. Completed r3 §8: the
  enumerated checklist re-evaluates class rule, floor, proof binding,
  fuse, manifest authorization, and rulings carriage from each outgoing
  commit's actual trees.
- **TB-LP-R1-03** — r2 overstated (TB-LP-R2B-09, -12): push-boundary
  coverage was claimed while the evaluator omitted checks and assumed a
  fetched tip two covered paths never establish. Completed r3 §8: the
  exhaustive evaluator, stdin's authoritative remote SHA in the hook,
  an explicit fetch in `commit.sh --push`, and wrapper-side enforcement
  that needs no installation.
- **TB-LP-R1-04** — r2 overstated (TB-LP-R2B-02): the seal fields were
  put in `terminalMetadataFields`, the exact allowlist generic
  same-status patches may rewrite (record.go:530-535). Completed r3
  §3.3: the fields join `dedicatedMetadataFields` with a close-owned
  write-once operation; generic patches are refused by the existing
  check at record.go:526-528.
- **TB-LP-R1-05** — folded r2 §3.2, strengthened r3: landing-check
  requires the completed independent-critique reference AND zero open
  material finding ids for EVERY bar (a) landing; closure alone never
  stands in for an acceptable outcome.
- **TB-LP-R1-06** — folded r2 §5: `plans/*.md` leaves carriage
  (handoff-notes glob only); rulings carriage is add-rows-only with the
  rewrite lane closed; the narrator digest remains carriage as the §12
  scope boundary, named, not silent.
- **TB-LP-R1-07** — r2 overstated (TB-LP-R2B-04): excluding `plans/`
  from `prose-docs` still left every other non-floor `.md` as an
  evidence-free lane. Completed r3 §4: `prose-docs` is withdrawn; doc
  defects take `mechanical-defect` with a state assertion, new doc
  content takes the loop.
- **TB-LP-R1-08** — r2 overstated (TB-LP-R2B-03): the "universal
  examination floor" still let an uncritiqued chain land any non-floor
  path — the same bypass, smaller path set. Completed r3 §3.2: bar (a)
  requires a completed, clean independent critique for every landing;
  the uncritiqued lane is deleted.
- **TB-LP-R1-09** — folded r2 §4: the floor enumeration includes
  `cmd/metasystem/`, `internal/lease/`, `internal/config/`,
  `internal/gittree/`, `internal/validate/`, `go-gate.sh`, the guard
  scripts, and `metasystem.conf` whole.
- **TB-LP-R1-10** — r2 overstated (TB-LP-R2B-06): verb ignorance only
  catches a binary older than the verb, not one older than the rules.
  Completed r3 §8: the `enginePolicyVersion` handshake against the
  remote tip's and each outgoing commit's manifest, fail-closed on any
  version the engine has not implemented.
- **TB-LP-R1-11** — r2 overstated (TB-LP-R2B-11): the authorizedBy
  check proved only that some row existed, in a tree the landing itself
  could write. Completed r3 §4: the cited ruling must pre-exist in
  origin history and carry the literal `landing-class=<class-id>`
  token; promotion continues through internal/governance's recorded
  human authorization.
- **TB-LP-R1-12** — r2 overstated (TB-LP-R2B-07): the cutover argument
  assumed a rebase delivers the pre-push hook, but rebases update hook
  SOURCE, not Git's hooks directory. Completed r3 §8: wrapper-side
  enforcement that rides the checkout (armed by the rebase itself),
  hooks installed and probed at every `metasystem up`, and the honest
  residual (raw pushes before the next session start) named and
  bounded.
- **TB-LP-R1-13** — r2 overstated (TB-LP-R2B-10): per-commit checks
  against origin alone let two outgoing commits each pass while jointly
  violating the invariant. Completed r3 §3.7/§8: uniqueness and fuse
  joins run over origin PLUS the complete outgoing range.
- **TB-LP-R1-14** — folded r2 §3.5: the conformance review stage writes
  `refs/metasystem/reviewed/<root-id>` at the reviewed tree, with its
  lifecycle (never pushed, deleted at evidence archival) and its
  refusal-on-loss named.
- **TB-LP-R1-15** — r2 overstated (TB-LP-R2B-05, -09): the "restored"
  evidence bar had no trailer join, no evidence hashes, no gate
  identity, and no durability. Completed r3 §4: the proof is a pinned
  git blob cited by OID in the trailer, carrying evidence-transcript
  blobs and the gate block, with whole-range consumption uniqueness.
- **TB-LP-R1-16** — r2 overstated (TB-LP-R2B-07): hook composition was
  promised with no ordering, so a pre-existing commit-msg hook could
  rewrite the message after validation. Completed r3 §2/§8: pre-existing
  hook first, provenance validator last, over the final bytes.
- **TB-LP-R1-17** — r2 overstated (TB-LP-R2B-05): the audit join could
  not reach the direct-fix evidence at all. Completed r3 §4/§7: the
  write-once seal, the reviewed-tree ref, the pinned proof blobs, and
  the report's cat-file join make every locally-verifiable landing
  reconstructible.
- **TB-LP-R1-18** — r2 overstated (TB-LP-R2B-01): the r2 matrix used
  columns the named validator does not recognize, so the "restored
  gate" was not executable (`no design-obligation rows found`, exit 1).
  Completed r3 §11: the matrix uses the validator's exact canonical
  header (designobligations.go:18-23) and the gate command is named
  with its pass condition.
- **TB-LP-R1-19** — NOTED; answered in §12 with the explicit scope
  boundary, not a mechanism.
- **TB-LP-R1-20** — folded r2 §7: the integrity cross-check is
  machine-scoped by the `Machine:` trailer; peer trailers report as
  peer-scope, never as findings; the fleet picture is the union of
  per-machine reports.
- **TB-LP-R1-21** — r2 overstated in part (TB-LP-R2B-11): schema, typed
  rules, and fail-closed behavior were pinned, but the authorization
  half remained an existence check. Completed r3 §4: the typed,
  origin-reachable, class-naming ruling join.
- **TB-LP-R1-22** — folded r2 §3.3/§3.4: `review.json` gains
  `baseTree`; `chainPaths` is recomputed via the null-delimited
  `ChangedPaths` primitive; no diff.patch parsing exists.
- **TB-LP-R1-23** — folded r2 §2: byte encoding, length bound,
  reject-not-escape rule, embedded-trailer refusal, and the non-merge
  parent rule for `revert-of` are pinned. (Round 3 adds the `proof=`
  field to the settled grammar.)
- **TB-LP-R1-24** — r2 overstated in part (TB-LP-R2B-10): the fuse
  shipped but aggregated only origin history plus the staged change, so
  a split across one push undercounted. Completed r3 §4/§8: the
  aggregate joins the whole outgoing range.
- **TB-LP-R1-25** — r2 overstated (TB-LP-R2B-08): the verdict trailer
  cannot attest a push-time result minted after the commit. Completed
  r3 §8: the trailer is scoped to the commit boundary with exact
  cardinality, and the push boundary writes the machine-local
  observation log; promotion evidence reads both.

### 10.3 The Sol landing-provenance critique, round 2 — how each ACCEPTED finding is folded

- **TB-LP-R2B-01** — FOLDED §11: the obligation matrix is rewritten in
  the validator's exact canonical columns (`| Obligation id | Severity
  | Design source | Required behavior | Owner | Code proof | Test proof
  | Runtime proof | Status | Next action |`,
  designobligations.go:18-23, and the shipped template
  docs/design/design-obligation-gate.md:24-25); the gate command is
  named verbatim with its two-phase contract: at design close it
  parses every row and refuses with the enumerated blocks-completion
  list (that refusal IS the implementation-readiness gate); at
  implementation close it must exit 0.
- **TB-LP-R2B-02** — FOLDED §3.3: the seal fields move from
  `terminalMetadataFields` to `dedicatedMetadataFields`
  (record.go:80-87) with a close-owned, write-once dedicated
  operation; the existing RecordCAS refusal at record.go:526-528 makes
  every generic patch — including same-status terminal metadata
  updates — unable to touch them, which is exactly the ownership
  mechanism the finding pointed at.
- **TB-LP-R2B-03** — FOLDED §3.2: the uncritiqued-chain lane is
  deleted; every bar (a) landing requires a completed, clean
  independent critique, matching §0's promise and the goal's charter;
  a MECHANICAL or critique-waived chain lands only by acquiring the
  same critique evidence (the hazard machinery already validates a
  voluntarily attached critique reference) or by fitting a bar (b)
  class.
- **TB-LP-R2B-04** — FOLDED §4: `prose-docs` is withdrawn entirely
  (rejected-alternative §9.8 records why narrowing could not save it);
  doc defects ride `mechanical-defect` with a state assertion; new doc
  content takes the loop.
- **TB-LP-R2B-05** — FOLDED §2/§4/§7: the trailer gains
  `proof=<40-hex-blob-oid>`; the proof record becomes a pinned,
  reachable git blob carrying red/green evidence-transcript blob OIDs
  and the gate block (command identity, engine sha256, tree, final
  zero outcome); the report joins commit → proof with one cat-file.
  §10.1 records the one open revision (fast gate in place of the full
  suite) as a design-lane decision for Wido, not a silent weakening.
- **TB-LP-R2B-06** — FOLDED §8.2: the `enginePolicyVersion` handshake —
  the manifest (floor-resident, critiqued-chain-only) declares the
  policy-surface version; the installed engine refuses any tree whose
  declared version exceeds its compiled version; every rule change
  bumps the version in the same landing, so a stale binary refuses
  instead of passing old rules.
- **TB-LP-R2B-07** — FOLDED §8: wrapper-side push enforcement rides the
  checkout and arms on rebase with no installation; hooks resolve their
  directory via `git-path hooks` exactly as enrollment does
  (goalsync_verbs.go:67-80) with the shared-hooksPath refusal;
  composition order is pinned (pre-existing first, validator last);
  `metasystem up` installs and PROBES the hooks every session (healing
  post-adoption `git init`); the migration text replaces the false
  rebase-delivers-the-hook claim with the two-channel statement and
  the bounded raw-push residual.
- **TB-LP-R2B-08** — FOLDED §2/§8: the verdict trailer is explicitly
  scoped to the commit-boundary evaluation with exact cardinality
  (observe: exactly one; enforce: exactly zero; byte-equal to the
  token); push-time observe outcomes get their own durable record, the
  machine-local observation log, which the report reads beside the
  trailers; in enforce, an outgoing commit carrying `would-refuse`
  refuses at push.
- **TB-LP-R2B-09** — FOLDED §8: the push evaluator is one exhaustive
  eleven-item checklist including proof binding, the fuse, manifest
  authorization, and rulings carriage; §0 now points at that list
  instead of claiming "every check" over a shorter one; the
  rebased-proof rule is explicit — change-set rebinding with
  entry-exact comparison, refusal plus one-step re-proof when a rebase
  textually merged a proved path.
- **TB-LP-R2B-10** — FOLDED §3.7/§4/§8: one-landing-per-chain,
  one-landing-per-proof, and the fuse aggregate all join over origin
  history PLUS the complete outgoing range, evaluated oldest-first, so
  a split across one push is seen whole.
- **TB-LP-R2B-11** — FOLDED §4: the authorizedBy join requires the
  cited rulings row to pre-exist in ORIGIN history (never the
  candidate tree, never the same push) and to contain the literal
  `landing-class=<class-id>` token; the human key is recorded and
  landed before the machinery change that cites it, restoring Law 2's
  order (plans/seat-governance-record.md:191-195); class removal, the
  conservative direction, needs bar (a) alone.
- **TB-LP-R2B-12** — FOLDED §8.1: the hook's authoritative remote tip
  is the remote SHA git supplies on pre-push stdin — established for
  every ref update by git's own advertisement, no fetch inside the
  hook, no trust in tracking refs; an R absent from the local object
  database fails closed naming the fetch; the all-zeros case has a
  defined rule; `commit.sh --push` gains its own fetch for the
  wrapper-side check.
- **TB-LP-R2B-13** — FOLDED §4: `revert-exact` gains the postimage
  precondition — every path in the named commit's change set must
  still carry `revert-of`'s entry in the reverting commit's parent
  tree — so a revert can never erase later work; the refusal names the
  touched path and the loop.
- **TB-LP-R2B-14** — FOLDED §3.7/§7/§9.9: the `landedCommit` record
  write is withdrawn (no post-success owner exists on the supported
  commit-then-push-later path); the report derives every landing
  commit from the origin trailer scan, correct on all push paths by
  construction; with §3.3's move to `dedicatedMetadataFields`, this
  design no longer changes `terminalMetadataFields` at all.

## 11. Obligation matrix and build order (the executable readiness gate)

The gate of record is the named validator
(`runValidateDesignObligations`, cmd/metasystem/main.go:86), run
verbatim as:

```bash
bin/metasystem validate design-obligations --runtime-required --file plans/two-bars-for-changes-design.md
```

Its contract over the matrix below (canonical columns per
internal/validate/designobligations.go:18-23): at design close the
command parses every row and exits 1 enumerating each CRITICAL/HIGH row
as `blocks completion` — that enumerated refusal is the
implementation-readiness gate, executable today, replacing round 2's
unparseable table (which returned `no design-obligation rows found`).
The implementation slice's completion gate is the SAME command at exit
0, reachable only when every CRITICAL/HIGH row is `DONE` with a
code-shaped owner and concrete code, test, and runtime proof — the
validator checks those cells mechanically
(designobligations.go:148-155).

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| TB-O01 | CRITICAL | §2 | Trailer + verdict grammar with `proof=` field; reject-not-escape; exact cardinality | `internal/dispatch` landing-check parser | internal/dispatch/landingcheck.go | internal/dispatch/landingcheck_test.go malformed-byte table | observe-window commit carrying the stamped trailer set | MISSING | build step 2 |
| TB-O02 | CRITICAL | §2, §8 | commit-msg hook: token byte-equality on final message; composed with pre-existing hook first, validator last | `scripts/agents/commit-msg-guard.sh` | scripts/agents/commit-msg-guard.sh | fixture over `-m`, `-F`, `--amend`, `-c`, editor, pre-existing message-rewriting hook | armed-checkout raw-commit refusal transcript | MISSING | build step 4 |
| TB-O03 | CRITICAL | §3.3 | Close-seal dedicated write-once operation; `reviewedTree`/`baseTree`/`diffPatchSha256` in `dedicatedMetadataFields`; review.json digest in mirror manifest | `internal/dispatch` close + record | internal/dispatch/close.go, internal/dispatch/record.go | close fixtures incl. tamper-after-close and re-seal refusal | close of a real chain then generic-patch refusal | MISSING | build step 3 |
| TB-O04 | HIGH | §3.4 | review.json gains `baseTree`; chainPaths via null-delimited `ChangedPaths` | `internal/validate` conformance | internal/validate/conformance.go | fixture with quoted/newline paths and deletions | review of a real round shows baseTree | MISSING | build step 3 |
| TB-O05 | HIGH | §3.5 | `refs/metasystem/reviewed/<root>` written at review, refusal-on-loss, archival delete | `internal/validate` + evidence retention | internal/validate/conformance.go | gc fixture: worktree removed, landing still resolves | post-gc landing on a real chain | MISSING | build step 3 |
| TB-O06 | CRITICAL | §3.6 | Byte binding incl. mode/type and deletions; partial-landing refusal | `internal/dispatch` landing-check | internal/dispatch/landingcheck.go | blob/mode/delete fixtures | observe-window bar-(a) landing | MISSING | build step 2 |
| TB-O07 | CRITICAL | §3.7, §8 | One-landing-per-chain and one-landing-per-proof joined over origin plus the whole outgoing range | `internal/dispatch` landing-check push range | internal/dispatch/landingcheck.go | split-commit and crash-window fixtures | two-commit push refusal transcript | MISSING | build step 5 |
| TB-O08 | CRITICAL | §3.2 | Completed clean independent critique required for every bar (a) landing; uncritiqued chains refused with named remedy | `internal/dispatch` landing-check | internal/dispatch/landingcheck.go | fixtures: open-material chain, MECHANICAL chain, waived chain | refusal transcript naming the critique remedy | MISSING | build step 2 |
| TB-O09 | CRITICAL | §4 | Manifest schema incl. `enginePolicyVersion`; typed rules; fail-closed malformed handling | `scripts/agents/landing-classes.json` + landing-check | scripts/agents/landing-classes.json | malformed-manifest table tests | direct fix under the shipped manifest | MISSING | build step 2 |
| TB-O10 | CRITICAL | §4 | Floor generation from both trees incl. the enumerated enforcement surface | `internal/dispatch` landing-check | internal/dispatch/landingcheck.go | fixture: floor entry added by fetched tip | push-boundary floor refusal transcript | MISSING | build step 2 |
| TB-O11 | CRITICAL | §4 | Proof verb: pinned blob + ref, red/green evidence blobs, gate block, nonce | new `proof direct-fix` verb in `cmd/metasystem` | internal/dispatch/proof.go | red-green, state-assertion, and stale-proof fixtures | one real proof produced and consumed | MISSING | build step 3 |
| TB-O12 | CRITICAL | §4, §8 | Proof binding: commit-exact tree equality; push change-set rebinding; refusal on textual merge | `internal/dispatch` landing-check | internal/dispatch/landingcheck.go | rebased-proof fixture with merged path | post-rebase push transcript | MISSING | build step 5 |
| TB-O13 | HIGH | §4, §8 | Fuse aggregate over origin plus outgoing range against the pre-defect base | `internal/dispatch` landing-check | internal/dispatch/landingcheck.go | split-commit laundering fixture | blown-fuse refusal transcript | MISSING | build step 5 |
| TB-O14 | CRITICAL | §4 | authorizedBy join: origin-reachable ruling carrying `landing-class=<id>` token | `internal/dispatch` landing-check | internal/dispatch/landingcheck.go | dangling-id and same-push-row fixtures | manifest-change landing transcript | MISSING | build step 2 |
| TB-O15 | HIGH | §4 | revert-exact postimage precondition plus tree-shaped inverse | `internal/dispatch` landing-check | internal/dispatch/landingcheck.go | later-work-clobber fixture | revert landing transcript | MISSING | build step 2 |
| TB-O16 | HIGH | §5, §8 | Carriage allowlist semantics; rulings add-rows-only at commit and push boundaries | `internal/dispatch` landing-check | internal/dispatch/landingcheck.go | rulings-rewrite fixture | carriage landing transcript | MISSING | build step 2 |
| TB-O17 | CRITICAL | §6, §8 | Fail-closed guards in enforce (absent engine, unknown verb, policy-version mismatch); observe fail-open | `scripts/agents/pre-commit-guard.sh` + pre-push guard | scripts/agents/pre-commit-guard.sh | missing-engine and stale-engine fixtures | broken-checkout refusal transcript | MISSING | build step 4 |
| TB-O18 | CRITICAL | §8 | Pre-push evaluator: stdin remote SHA, unknown-object fail-closed, zero-SHA rule, eleven-item checklist | new `scripts/agents/pre-push-guard.sh` + landing-check | scripts/agents/pre-push-guard.sh | fixtures: raw push, stale tracking ref, new-branch push | raw-push refusal transcript | MISSING | build step 4 |
| TB-O19 | CRITICAL | §8 | Wrapper-side push checks: land.sh retry loop and commit.sh --push with its new fetch | `scripts/agents/land.sh` + `scripts/agents/commit.sh` | scripts/agents/commit.sh | fixtures: land.sh retry loop, commit.sh --push, push-later | wrapper push transcript on an unhooked checkout | MISSING | build step 4 |
| TB-O20 | CRITICAL | §8 | Hook install via `git-path hooks`, shared-hooksPath refusal, compose order, probe at every `metasystem up` | `cmd/metasystem` up + adoption | cmd/metasystem/goalsync_verbs.go | arming fixture with pre-existing hooks and core.hooksPath | up output showing hook probe outcomes | MISSING | build step 4 |
| TB-O21 | CRITICAL | §8 | Governance obligation registration; promotion only through recorded human authorization | `internal/governance` | internal/governance/types.go | promotion-without-authorization refusal fixture | observe→enforce flip transcript | MISSING | build step 5 |
| TB-O22 | HIGH | §7, §8 | Observe verdict trailer at commit; push observation log; report reads both | `internal/dispatch` + report verb | internal/dispatch/landingcheck.go | observe-window fixture incl. push-time would-refuse | one observe-window report | MISSING | build step 5 |
| TB-O23 | HIGH | §7 | Report: trailer scan, landing-commit derivation, proof cat-file join, machine scoping | `cmd/metasystem` report direct-fixes | cmd/metasystem/report_verbs.go | report fixture over synthetic history | report run over real origin history | MISSING | build step 5 |
| TB-O24 | HIGH | §7, §8 | Bootstrap landing named in-message; instruction-ledger entry written in the same landing | `scripts/agents/land.sh` rollout step | plans/instruction-ledger.md | ledger-entry presence check in the bootstrap fixture | the bootstrap landing itself | MISSING | build step 1 |

Build order (r3's five steps, mapped): (1) contracts are settled by
THIS document (§2 grammar, §4 manifest/proof schemas, §3.3 seal
schema, §8 hook lifecycle and evaluator checklist, this matrix) and the
bootstrap/ledger step (TB-O24); (2) the pure landing-check evaluator,
table-tested (TB-O01, -O06, -O08, -O09, -O10, -O14, -O15, -O16); (3)
seal, ref, and proof machinery (TB-O03, -O04, -O05, -O11); (4) hooks,
guards, and boundary integration (TB-O02, -O17, -O18, -O19, -O20); (5)
whole-range joins, governance registration, observe evidence, and the
report plus the end-to-end common-path fixture proving the
one-extra-flag claim (TB-O07, -O12, -O13, -O21, -O22, -O23).

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

- **Trajectory, honestly:** round 1 drew 25 accepted findings; round 2
  drew 14, eleven of which showed round 2's fold claims were themselves
  overstated — the recurring failure was designing against assumed code
  structure. This round was written against the read seams
  (designobligations.go, record.go's three field classes, close.go,
  the guard and wrapper scripts, goalsync_verbs.go), and where the
  code already had the needed mechanism (dedicated metadata ownership,
  the git-path hooks resolution, the guard-probe nonce, pre-push
  stdin), the design now names that mechanism instead of inventing
  one. The convergence moves are deletions, not additions: the
  uncritiqued bar (a) lane, the `prose-docs` class, and the
  `landedCommit` write are gone rather than patched.
- **Confidence:** high that the mechanism closes the chartered hole —
  after enforce, no agent landing reaches origin without either a
  closed AND cleanly examined chain's byte-bound candidate, or a typed,
  floor-checked, proof-backed, fused, permanently visible declaration;
  the exhaustive push evaluator makes the claim hold across rebases,
  all push paths, and the cutover. Moderate on three operational
  points: (1) the push evaluator's cost (an engine call per outgoing
  commit plus origin-history scans) is believed cheap but unmeasured —
  the observe window measures it; (2) how often the change-set proof
  rebinding and the postimage precondition fire in real fleet traffic;
  (3) whether three classes and the initial fuse numbers (200 lines, 1
  subsystem) are the right partition — the observe evidence adjusts
  both before enforce, through the governed manifest path.
- **The weakest claims, declared:** (1) requiring a clean independent
  critique for EVERY bar (a) landing raises the cost of MECHANICAL
  delegated chains — the direct price of deleting the R2B-03 bypass.
  If the observe window shows honest mechanical chains routinely
  stalling on critique dispatch, the lawful relief is cheaper critique
  dispatch or a new evidence-bearing manifest class through bar (a) —
  never re-opening the uncritiqued lane. (2) Withdrawing `prose-docs`
  routes doc work through `mechanical-defect` state assertions or the
  loop; if that proves heavier than the fleet bears, the same governed
  manifest path is the relief. (3) The §10.1 gate-block revision
  (fast gate in place of the carried full-suite witness) is a
  design-lane decision awaiting Wido's eyes.
- **Wido should reject this design if:** he judges the checked
  machinery (two guarded boundaries, write-once seal, two ref
  families, proof blobs, fuse, policy handshake) past what R-11
  tolerates for the accidental model — the honest fallback is
  narrowing bar (b) to `register-carriage` + `revert-exact` and
  forcing everything else through the loop, since the critiques showed
  every softer variant unsound; if he wants the full-suite witness
  kept as carried rather than the §10.1 revision; if he wants the
  second governance open item remedied in the same slice rather than
  reserved (§12); or if he wants the adversarial tier now rather than
  as the recorded separate goal.
