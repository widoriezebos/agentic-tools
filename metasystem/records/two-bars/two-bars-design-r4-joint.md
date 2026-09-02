# Two bars for changes — joint-round design and observe slice

- Goal: `two-bars-for-changes`
- Revision: joint round after design revision 3
- Status: observe mode implemented; enforcement is not authorized or active
- Authority: the recorded R-25 lane exception, R-35-m0 (Wido's word), grants
  this round both the design and implementation pens. The three charter bars
  remain fixed: closed-chain binding, typed declared direct fix, and refusal.
- Threat model: D90, the honest agent who forgets or misclassifies under
  pressure. Deliberate trailer forgery, hook bypass, and branch-rule bypass by
  a human remain outside this goal.

## 0. The mechanism in one page

Every agent landing declares one primary bar. A closed-chain landing may also
declare the narrowly typed `register-carriage` class for paths that standing
law adds after conformance review:

1. **Bar (a), closed-chain binding.** `--chain <root-job-id>` names a closed
   `DESIGN-BEARING` or `DESTRUCTIVE-REACH` implementer chain. The landing
   passes only when the landing's certified-path change digest equals the
   digest reconstructed from that chain's conformance `diff.patch` and
   `reviewedTree`. Unrelated carriage paths are partitioned out and evaluated
   separately. Design output is input to that implementation chain; a designer
   job is not itself a landing credential.
2. **Bar (b), typed declared direct fix.** The two classes are
   `register-carriage` and `exact-revert`. `register-carriage` admits only the
   protected allowlist and applies append-only rules to the two history
   registers named below. `--direct-fix exact-revert --revert-of <commit>`
   passes only when the
   prospective change is the complete tree-shaped inverse of one single-parent
   commit, the current parent still carries that commit's postimage, no extra
   path changes, and no changed path is on the never-direct-fix floor.
3. **Bar (c), refusal.** Missing, conflicting, malformed, open-chain, wrong
   chain type, unknown class, non-exact revert, and evaluator failures all
   produce a named `would-refuse` verdict.

Observe mode runs now at the final settled-index seam in `commit.sh`. It never
turns bar (c) into an exit failure. It stamps two trailers on every wrapper
commit: exactly one of the eight `Landing-Provenance` forms below, followed by
exactly one verdict form.

```text
Landing-Provenance: chain=<root> change=<64-hex> certified-change=<64-hex>
Landing-Provenance: chain=<root> direct-fix class=register-carriage change=<64-hex> certified-change=<64-hex>
Landing-Provenance: direct-fix class=register-carriage change=<64-hex>
Landing-Provenance: direct-fix class=exact-revert revert-of=<commit> change=<64-hex>
Landing-Provenance: none change=<64-hex-or-unknown>
Landing-Provenance: invalid change=<64-hex>
Landing-Provenance: chain=<root> change=<64-hex>
Landing-Provenance: direct-fix class=exact-revert change=<64-hex>
Landing-Provenance-Verdict: pass bar=a
Landing-Provenance-Verdict: pass bar=a carriage=register-carriage
Landing-Provenance-Verdict: pass bar=b
Landing-Provenance-Verdict: would-refuse code=<reason>
```

The `change` digest is SHA-256 over `gittree.Workspace.Diff(HEAD-tree,
candidate-tree)`. The `certified-change` digest is a canonical SHA-256 stream
of path, before entry, and after entry for only the paths reconstructed by the
chain's certified diff. The evaluator applies that diff to the actual landing
base, so unrelated upstream commits and bundled carriage do not stale bar (a),
while any certified blob, mode, addition, or deletion change does.

The evaluator is deliberately small: one closed-chain lookup and certified
patch, two typed direct classes, one broad floor, and one verdict. The class
manifest enumerates only those compiled rules; it is not a predicate language,
proof graph, fuse, policy-version negotiation mechanism, mode registry, or
automatic clean-window state.

## 1. Facts from the current tree

The mechanism follows these existing seams rather than assumed ones:

- `scripts/agents/commit.sh` captures `proved_tree`, rebuilds the policy engine
  from the prospective source, rechecks the settled tree, and only then calls
  `git commit`. The observe call sits after the settled-tree comparison and
  before that commit. This is the last point where the exact index and final
  message can still be joined without a second commit.
- `scripts/agents/land.sh` already funnels its commit through `commit.sh`; it
  now carries the three provenance flags into that call. Its later fetch and
  rebase preserve commit trailers. The digest describes the delta instead of
  the full parented tree, so unrelated upstream movement does not stale it.
- `scripts/agents/pre-commit-guard.sh` can prove wrapper ancestry only when its
  engine classifies the caller. When classification is unavailable, observe
  mode remains non-refusing but appends a tree-bound
  `classifier-unavailable` line to
  `artifacts/agents/landing-observe.log`; the failure is no longer invisible.
- `internal/gittree` already owns project-prefix normalization, tree snapshots,
  changed paths, and exact mode-plus-object comparisons. The exact-revert
  class adds only the missing single-parent query there.
- `internal/dispatch.CloseCheck` already gives `DESIGN-BEARING` and
  `DESTRUCTIVE-REACH` chains their configured independent-critique closure
  duties. Landing provenance checks the resulting root's declared class and
  `chainClosed` fact; it does not invent a second critique register.
- `internal/validate` conformance already owns implementation review and the
  final code-critic zero-material-findings gate. Its latest implementation
  round writes `review.json` and `diff.patch` under the chain artifact
  directory. That remains the integration gate; landing provenance binds the
  candidate to that reviewed output rather than duplicating conformance.
- The shared origin is GitHub. Local hooks cannot make an unarmed clone run
  code, so local hook arming cannot honestly be the final universal refusal
  boundary.

## 2. Observe-mode contract

### 2.1 Input and output

`metasystem landing observe` accepts:

```text
--root <project-root> --tree <prospective-project-tree>
[--chain <root-job-id>]
[--direct-fix register-carriage]
[--direct-fix exact-revert --revert-of <commit>]
```

The legal combinations are chain alone, chain plus `register-carriage`,
`register-carriage` alone, or `exact-revert` alone. Chain plus any other direct
class, and either class with fields belonging to the other, evaluates
`would-refuse`. A supplied `--revert-of` without `exact-revert`, or an
`exact-revert` declaration without its `--revert-of`, is a
`conflicting-declarations` classification.

The command emits one JSON `Observation` with `schemaVersion`, `mode`, `bar`,
`verdict`, `code`, `provenance`, and `verdictTrailer`. A valid command
invocation returns success for every policy outcome. Bad flag syntax remains a
CLI usage error; the wrapper supplies fixed syntax. If execution or JSON
decoding fails inside the wrapper, the wrapper stamps the fixed fallback:

```text
Landing-Provenance: none change=unknown
Landing-Provenance-Verdict: would-refuse code=evaluator-unavailable
```

This fallback preserves the fact that the check could not decide. It refuses a
managed agent commit with an evaluator-failure message and repair instruction;
a human commit remains sovereign, continues, and carries the fallback stamp.

### 2.2 Bar (a)

A chain declaration passes only when:

- the identifier uses the job-id grammar;
- `artifacts/agents/jobs/<root>.json` is readable, names itself, and has no
  parent;
- its role is `implementer`;
- its `destructiveReach` is `DESIGN-BEARING` or `DESTRUCTIVE-REACH`; and
- `chainClosed` is true.

The locator reads every numeric `artifacts/agents/<root>/rounds/N/review.json`
because conformance stores the review under the *invoked job's* round: the
repository's normal root-id invocation therefore writes `rounds/1` even after
follow-up corrections, while a follow-up-id invocation writes `rounds/N`.
Malformed entries are ignored. One parseable review is decisive; with more
than one, the closed independent critic's `reviewedTree` selects first, the
terminal implementer job recorded in `implementerJob` selects second, and
numeric round is only a deterministic fallback. No parseable review refuses.

The selected `diff.patch` must be regular and apply exactly to the landing's
current `HEAD` tree. Its resulting entries on every certified path must equal
the selected `reviewedTree`; the canonical certified-path digest must then
equal the landing's digest for those same paths. This binds what the chain
produced without demanding whole-tree equality after unrelated base movement.
Every remaining landing path is carriage only when the landing also declares
`--direct-fix register-carriage`; any non-allowlisted remainder would refuse.

The result records
`chain=<root> change=<digest> certified-change=<digest>`, plus
`direct-fix class=register-carriage` when combined carriage exists. This is the charter's
closed-chain binding: conformance remains responsible for clean-code-critic
acceptance, while the landing rechecks the certified output digest. A
mechanical chain is not a loop credential; if the change is not an
exact-revert direct fix it must be dispatched design-bearing.

In-flight design revisions do not land bare under future enforcement. Each
revision rides the implementer chain for the fold round that incorporates it,
and bar (a) binds the combined design-and-code landing to that chain's latest
conformance output. A design-critic chain alone is never a landing credential.

### 2.3 Bar (b)

`register-carriage` restores the round-three standing exemption that the first
joint redesign silently dropped. Its protected policy inputs are
`scripts/agents/landing-classes.json` and
`scripts/agents/register-carriage-paths.txt`; both remain on the broad floor
and therefore cannot change under either direct class. The manifest contains
only the compiled `register-carriage` and `exact-revert` rules. The allowlist
has exactly these entries, the last being the single permitted glob form;
both manifest rows cite the round's recorded R-35-m0 authority:

```text
records/narrator-digest.log
memory/rulings.md
memory/receipts.log
plans/handoff-*.md
```

The design-lane addendum at
`records/two-bars/two-bars-allowlist-addendum.md` removes the nonexistent
`memory/findings.md`, upholds the exclusion of goal-ledger and all other
memory paths, and rules `memory/receipts.log` in. Both `memory/rulings.md` and
`memory/receipts.log` are append-only: against the evaluation boundary's
parent tree, the candidate must preserve every existing byte through its
terminating newline and add only complete trailing lines. Deletion, mode-only
change, truncation, or rewriting an existing line would refuse. The narrator
digest remains content-unverified as round three stated; handoff records are
constrained by the one basename glob.

`exact-revert` is finite and mechanically decisive:

1. `revert-of` resolves to exactly one parent.
2. The class computes every changed path between that parent and the named
   commit.
3. The current `HEAD` tree still has the named commit's mode and object at
   every changed path. This prevents erasing later work.
4. The candidate changes exactly that path set and carries the parent's mode
   and object at every path. Additions, deletions, symlinks, executability, and
   gitlinks are therefore compared as Git records them.
5. No changed path is on the floor.

The initial floor is intentionally conservative. Instruction and build-owner
filenames (`AGENTS.md`, `CLAUDE.md`, `wow.md`, `metasystem.conf`, Go module and
workspace files, `.gitattributes`, `.gitignore`, and the committed
`bin/metasystem` engine) are protected at every
nesting depth. Protected directory names are also matched at every nesting
depth: `.agents/`, `.claude/`, `.codex/`, `.github/`, `bin/`, `cmd/`, `docs/`,
`internal/`, `memory/`, `optional-skills/`, `plans/`, `records/`, `roles/`,
`schemas/`, `scripts/`, `skills/`, and `templates/`. This includes
`internal/goal`, `internal/humanauthority`, and `internal/governance` by
construction. On this metasystem repository, changes to its own code and
governance take bar (a) except for the exact register-carriage entries and
disciplines above; exact-revert remains useful for adopted-project payload
outside the protected project machinery.

There is no generic `mechanical-defect` class. A bug fix that is not the exact
inverse above takes the loop until a future human-authorized design introduces
another mechanically decidable class.

### 2.4 Durable observation

The commit trailers are the primary record. They move with rebase and push and
can be read from origin history after local artifacts disappear. The guard's
append-only line is the secondary record for the one situation in which no
engine can decide whether a raw caller is an agent:

```text
schemaVersion=1 boundary=pre-commit tree=<oid|unknown> verdict=would-refuse code=classifier-unavailable
```

There is no global "unresolved" counter. Each commit and guard event is a
self-contained observation. A later corrected landing does not rewrite an
earlier event, and promotion does not infer cleanliness by subtracting events.

## 3. Full protected-branch enforcement design, deliberately not active

Local hooks cannot enforce a rule in a clone where they were never installed.
Therefore universal enforcement is a GitHub protected-branch required check,
not a local mode bit. The later enforcement slice will:

1. run the evaluator from the protected base branch, never from candidate
   policy bytes;
2. inspect every incoming commit before GitHub admits it to the protected
   branch;
3. require exactly one provenance trailer and one `pass` verdict trailer;
4. recompute the change digest from the incoming commit's actual parent and
   tree;
5. recompute both typed bar (b) rules from the protected manifest and
   allowlist, including append-only carriage against each outgoing commit's
   own parent;
6. recompute bar (a) by resolving the stamped root's conformance output,
   applying its certified diff to the incoming parent, comparing the
   certified-path digest, and separately evaluating every remaining path as
   declared carriage. The
   stamped pass is never accepted as evidence by itself.

Using the protected base evaluator removes the stale-policy handshake problem:
a candidate cannot weaken the check it is currently passing. A policy change
itself touches the floor, so it must arrive as bar (a); only later candidates
are judged by that newly protected base.

Raw pushes from unarmed or post-`git init` clones still reach GitHub, but a
missing pass trailer fails the required check. That is the mechanical answer
to the unarmed-checkout gap. Local `pre-commit` remains an early explanation,
not the universal authority.

The managed agent commit gate has one narrower promotion before that universal
check: `scripts/agents/landing-promotion.json` lists the exact observation
codes that refuse managed agent commits. It mirrors
`scripts/agents/landing-classes.json` only in the house convention of a
schema-versioned top-level object with named typed content. Its decoder is
deliberately stricter than the sibling class-manifest loader: the only
top-level fields are integer `schemaVersion` with value 1 and string-array
`refuseCodes`; unknown fields, trailing JSON, unknown codes, duplicate codes,
and unparseable content make the whole record malformed. An absent file
preserves the pre-promotion observe-everything state, while a malformed file
refuses every managed agent commit so a damaged policy cannot silently undo a
human promotion. The evaluator reads the record from the landing base tree,
and the file itself remains on the never-direct-fix floor. Human commits do not
consume this local refusal decision. The shipped two-code promotion is
authorized by ruling R-40-m0, which lands with this implementation chain and
is therefore cited here before it appears in this worktree's base register.

Full protected-branch promotion remains a later human act. It requires a
separate reviewed enforcement implementation, a successful required-check
rehearsal against an exact origin range, and Wido enabling that check on the
protected branch. The human reviews the range's complete observation report,
including missing trailers and `would-refuse` events; there is no automatic
"zero unresolved" claim and no four-machine registry to guess at.

Human branch-rule bypass remains sovereign and outside D90. Deliberate forged
pass trailers also remain outside D90 because the remote check has no durable
agent signing identity. Those are the already separated adversarial goal.

## 4. Failure behavior

- A policy mismatch returns a `would-refuse` observation. An unpromoted code
  continues; a promoted code refuses a managed agent commit while leaving the
  human path unchanged.
- An evaluator crash, missing verb, invalid JSON, or unavailable proof engine
  refuses a managed agent commit with the fixed evaluator-unavailable verdict,
  staged paths, and repair instruction. A human commit remains sovereign and
  carries the fallback trailers.
- An unreadable landing base refuses a managed agent commit with its distinct
  base-unreadable verdict and restoration instruction; it is never reported as
  a classification failure.
- A pre-commit classification failure appends the fallback guard event and the
  existing human-sovereignty fail-open continues.
- Existing static proof, coverage, audit, Git, lease, and new-plan refusals are
  unchanged. They are not two-bars enforcement and may still stop a commit.
- A rebase preserves the trailers. If it changes the landing's delta, the
  stored digest no longer describes the resulting commit; observe mode merely
  records this for later reporting, while the future remote check refuses it.
- `landing observe` returns every policy outcome as data. The managed wrapper
  changes its own exit status only when that outcome carries refusing mode.

## 5. Tests and observable proof

The focused command is:

```bash
go test ./internal/landing -run TestObserve -v -count=1
```

It proves by name:

- `TestObserveChainBoundLandingEvaluatesBarA`, including refusal for an open
  chain, non-implementer role, non-design-bearing reach, parented root,
  unreadable root record, both real conformance layouts, a tampered certified
  path, a certified change bundled with append-only receipt carriage, and a
  non-allowlisted protected path beside that bundle;
- `TestObserveDeclaredDirectFixEvaluatesPerClassRule`, including refusal when
  an extra path is smuggled into the inverse and when the inverse changes the
  committed engine under `bin/`; it also proves append passes and rewrite
  refuses for both append-only registers, and proves the allowlist and class
  manifest cannot change under carriage;
- `TestObserveUndeclaredLandingRecordsWouldRefuse`; and
- `TestObserveVerdictSurvivesLanding`, which also checks the wrapper wiring,
  pushes a commit to a bare origin, and reads both trailers from the landed
  commit.

`pre-commit-guard-fixtures.sh` separately proves a classifier failure remains
non-refusing and writes the durable fallback event.

## 6. Joint-round dispositions of the twelve open findings

Every row below is a **JOINT-ROUND REDESIGN** under the one-round authority.

| Finding | Disposition and code fact that forced it | Resolution |
| --- | --- | --- |
| TB-LP-R3-01 | JOINT-ROUND REDESIGN — delete `mechanical-defect`; its untyped state assertion could classify any byte replacement. `internal/gittree` can decide a complete inverse without a predicate language. | §2.3 and `internal/landing/observe.go` |
| TB-LP-R3-02 | JOINT-ROUND REDESIGN — delete reusable red/green proofs and their rebase rule. Exact-revert is recomputed from the actual parent and candidate trees; the change digest exposes delta changes. | §2.3, §3 |
| TB-LP-R3-03 | JOINT-ROUND REDESIGN, PARTLY RESTORED FROM R3 after F2-3 — the policy-version handshake remains deleted because the later required check runs protected-base code, but the class manifest is restored as the finite data declaration for `register-carriage` and `exact-revert`. The first joint redesign silently dropped carriage and therefore its manifest. | §2.3, §3 |
| TB-LP-R3-04 | JOINT-ROUND REDESIGN — local arming is no longer claimed universal. The real origin is GitHub and an unarmed clone runs no local code, so the later refusal owner is a protected-branch required check. | §1, §3 |
| TB-LP-R3-05 | JOINT-ROUND REDESIGN — delete proof and transcript blobs. Exact-revert needs only reachable commit and tree objects already transported by Git; the durable observation is in the landing commit. | §2.3–§2.4 |
| TB-LP-R3-06 | RESTORED FROM R3, ADDENDUM-RULED after F2-3 — the prior joint row's “one compiled class” premise was false because it silently dropped the round-three `register-carriage` mechanism. Restore the protected two-class manifest and exact allowlist; apply the design-lane addendum's removal of `memory/findings.md`, addition of append-only `memory/receipts.log`, and retained append-only `memory/rulings.md`. Mechanical-defect and free predicate authorization remain deleted. | §2.3, `scripts/agents/landing-classes.json`, `scripts/agents/register-carriage-paths.txt`, and the allowlist addendum |
| TB-LP-R3-07 | JOINT-ROUND REDESIGN — replace the incomplete enumerated floor with broad protected roots. `internal/goal`, `internal/humanauthority`, and `internal/governance` all fall under `internal/`. | §2.3 and `internal/landing/observe.go` |
| TB-LP-R3-08 | JOINT-ROUND REDESIGN — delete the guessed global mode owner and four-machine promotion registry. Observe is non-refusing code; enforcement exists only when Wido later enables the reviewed GitHub required check. | §3 |
| TB-LP-R3-09 | JOINT-ROUND REDESIGN, REOPENED AND RE-RESOLVED after F-A/F-D/F2-2/F2-3 — restrict bar (a) credentials to implementer roots and bind the landing's certified-path digest to the selected conformance diff and reviewed tree; a closed record or whole-tree equality is insufficient. The locator follows conformance's root-id and follow-up-id storage layouts. In-flight design revisions ride their fold round's implementer chain, and post-review running registers ride only declared, typed carriage. | §1, §2.2 and `internal/landing/observe.go` |
| TB-LP-R3-10 | JOINT-ROUND REDESIGN — landing no longer treats the canonical finding register as clean-critique truth. Existing conformance reads the final code-critic return and zero material count directly. | §1, §2.2 |
| TB-LP-R3-11 | JOINT-ROUND REDESIGN — the R-35-m0 joint-round instruction is the explicit human authority to build the observe slice while redesigning it. The matrix below covers only that slice and is complete; enforcement remains a later design and build. | §7 |
| TB-LP-R3-12 | JOINT-ROUND REDESIGN — delete ambiguous windows and resolutions. Each commit carries its own verdict; classifier failure writes a fallback event; promotion reviews an exact range instead of calculating “unresolved.” | §2.4 and `scripts/agents/pre-commit-guard.sh` |

Cross-family correction F2-3 is therefore **RESTORED FROM R3 PLUS
ADDENDUM-RULED**, not a new implementer choice: the joint redesign's dropped
carriage class returns, its path list is narrowed exactly by the design-lane
addendum, and chain-plus-carriage is an explicit two-check grammar rather than
an implicit exemption.

## 7. Observe-slice obligation matrix

This matrix covers only the human-authorized observe slice. The later remote
refusal slice must receive its own design, critique, implementation, and human
promotion decision.

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| TB-OBS-01 | CRITICAL | §2.1 | Every syntactically valid evaluation returns pass or would-refuse without a policy exit failure | `internal/landing` | internal/landing/observe.go | internal/landing/observe_test.go | `go test ./internal/landing -run TestObserve` | DONE | none |
| TB-OBS-02 | CRITICAL | §2.2 | Bar (a) requires a closed design-bearing implementer root, locates conformance output under either real storage layout, digest-binds certified paths, and evaluates every bundled remainder as declared carriage; five invalid root shapes, tampering, and non-carriage extras refuse | `internal/landing` | internal/landing/observe.go | `TestObserveChainBoundLandingEvaluatesBarA` | `go test ./internal/landing -run TestObserve` | DONE | none |
| TB-OBS-03 | CRITICAL | §2.3 | Exact-revert checks the whole inverse, postimage, changed-path equality, and broad floor including `bin/`; register-carriage reads protected policy from the base tree, admits only the exact allowlist, and enforces append-only rulings and receipts | `internal/landing` and `internal/gittree` | internal/landing/observe.go, internal/gittree/commit.go, scripts/agents/landing-classes.json, scripts/agents/register-carriage-paths.txt | `TestObserveDeclaredDirectFixEvaluatesPerClassRule` | `go test ./internal/landing -run TestObserve` | DONE | none |
| TB-OBS-04 | CRITICAL | §2.4 | Missing declaration records bar (c) and both verdict trailers survive the push | `scripts/agents/commit.sh` | scripts/agents/commit.sh | `TestObserveUndeclaredLandingRecordsWouldRefuse`, `TestObserveVerdictSurvivesLanding` | `go test ./internal/landing -run TestObserve` | DONE | none |
| TB-OBS-05 | HIGH | §1, §2.1 | land.sh carries declaration flags and commit.sh observes the settled project tree; an unavailable evaluator refuses managed agents honestly while the human path stamps the fallback and remains sovereign | `scripts/agents` | scripts/agents/land.sh, scripts/agents/commit.sh | `TestObserveVerdictSurvivesLanding`, `static-reproof-fixtures.sh` | `go test ./internal/landing -run TestObserve`; `bash scripts/agents/static-reproof-fixtures.sh` | DONE | none |
| TB-OBS-06 | HIGH | §2.4 | An unevaluable pre-commit classification writes a durable would-refuse event without refusing | `scripts/agents/pre-commit-guard.sh` | scripts/agents/pre-commit-guard.sh | scripts/agents/pre-commit-guard-fixtures.sh | `bash scripts/agents/pre-commit-guard-fixtures.sh` | DONE | none |

## 8. Scope and non-claims

This round does not install a GitHub workflow, change branch protection, add a
remote report, or claim universal provenance enforcement. It refuses only the
named managed-agent cases above. It does not claim that the current local
observe stamp stops deliberate forgery. It does not replace conformance, chain
close, leases, static proof, or the human emergency path.

The implemented guarantee is narrow and testable: every successful managed
wrapper commit gets a durable declaration and observation verdict; both direct
classes are mechanically bounded; missing and conflicting declarations refuse
managed agent commits under the strict promotion record; all other policy
codes remain observable. Universal enforcement still requires the separately
reviewed remote check and Wido's explicit protected-branch promotion.
