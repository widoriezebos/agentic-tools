# Two bars for changes (backlog item 2)

- Status: r3 — the human ruled the fork (D90: the ACCIDENTAL model,
  "stop an honest agent from forgetting the rule"), and the
  Opus-window special review (plans/opus-window-review-twobars.md)
  found the r2 fold left four r1 findings materially open. r3 folds
  its dispositions below. NOT yet implementation-ready: the review's
  readiness gaps (the settle-the-contracts step and the
  design-obligation matrix) are the named remaining work before any
  build. The adversarial layer stays a separate future goal.
- Goal: two-bars-for-changes (design r3; implementation queued
  behind the ACP flip and the settle-step below)
- In flight right now: none.
- Waiting on the human: nothing — the fork is ruled.
- Next step: none.

## r3 dispositions (from the special review, all adopted)

- **Emergency (TB-R1-06), CHOSEN:** human-personal. A genuine
  emergency safety fix is committed by the human (already
  sovereign) with immediate reconciliation. NO agent authorization
  machinery is built; the one-use-token alternative is recorded as
  a future requirement's option, not this design's.
- **Hook scope (TB-R1-02), corrected:** raw `git commit` BY HABIT is
  an honest-forgetting case and IS in scope — an agent's raw commit
  with hooks active must be refused; only deliberate bypass
  (`--no-verify`, hook tampering, `core.hooksPath` games) is
  adversarial and out of scope. Hook composition must handle
  existing pre-commit AND commit-msg hooks, `core.hooksPath`, and
  post-adoption `git init` (adoption today skips installation when
  a hook exists — that gap closes as part of this work).
- **Witness owner (TB-R1-03), corrected:** the tree-bound witness is
  finalized by the WHOLE owning validator at the END of the full
  suite — never by go-gate mid-run (the D33 witness lands before
  later fixture families, which preserves the false-green hole).
  gaterun's live markers are process state, not outcomes, and gain
  no witness role.
- **The accidental-model keep/skip line** (the review's table,
  adopted verbatim as the build boundary): KEEP the both-trees
  manifest read, the commit-msg hook, the candidate-tree-OID
  witness binding, the full-validator zero-outcome identity, local
  nonce + consume-on-use witness lifecycle, one reusable
  red-then-green assertion evaluated against both immutable trees
  (defined to accommodate newly-added regression tests), the
  defect-identity growth fuse over reachable history, and
  content-bound reachable audit refs. SKIP cryptographic
  authentication, protected custody, signatures, authorship
  inference over trailer-less history, agent emergency tokens,
  remote CI enforcement, and --no-verify resistance.
- **Build order, replacing the old P1/P2:** (1) SETTLE THE
  CONTRACTS — the trailer grammar including Defect-ID, the manifest
  path/schema/failure-behavior/initial protected set, budgets and
  subsystem mapping with parent/amend/merge rules, the proof and
  witness schemas, the audit-chain schema (design + findings +
  dispositions + closure, immutable and reachable), the
  hook-composition lifecycle, and the design-obligation matrix this
  document still lacks (the obligation gate refuses implementation
  without rows); (2) the pure change-class evaluator, table-tested;
  (3) the immutable candidate/baseline runner with the
  full-validator witness finalization; (4) commit.sh + pre-commit +
  commit-msg integration with idempotent composition, tested over
  -a, pathspecs, message reuse, editor changes, amend, and raw
  agent git; (5) the audit join, the instruction-ledger entry, the
  history-report scope, and the end-to-end common-path fixture
  proving the one-extra-line claim.

## The human's rule (2026-08-08, backlog-notes item 2)

A change takes ONE of two bars: the DESIGN LOOP (design →
adversarial critique → converge → implement) for design changes, or
a DIRECT FIX for mechanical defects — declared, so the choice is
auditable, with a NEVER-DIRECT-FIX set that no "it's just a bug"
can launder. The human's stated worry is a rule that "gets
forgotten under time pressure, which is exactly when it matters":
the concern is an honest agent FORGETTING, not a hostile one
tampering. That framing is what the fork below turns on.

## The threat-model fork (RESOLVED: accidental, D90)

The human ruled the accidental model. What follows is why the fork
existed and what the ruling commits to. r1 proved the two
enforcement targets need different machinery and cannot share one:

- **ACCIDENTAL model (recommended):** stop an honest agent from
  MISCLASSIFYING or forgetting under pressure. Local hooks plus a
  tree-bound gate witness suffice. Human sovereignty stays intact (a
  human commit is trusted). This matches the human's "forgotten
  under time pressure" framing and is buildable now without a new
  identity system.
- **ADVERSARIAL model:** stop a misaligned agent from DELIBERATELY
  laundering a design change (`--no-verify`, hook tampering,
  `core.hooksPath` games, forged same-user proof files). Note the
  boundary (r3): raw `git commit` and `-a`/pathspec use BY HABIT are
  honest-forgetting cases the local hooks DO cover — with hooks
  active they are classified or refused like any commit; only
  deliberate hook bypass is adversarial. r1 shows the adversarial
  tier cannot be built with local hooks and string trailers: it
  needs REMOTE enforcement (CI that rejects a push whose commits
  lack a valid tree-bound attestation), DURABLE agent identity (a
  signing key, not a same-user file), and it CANNOT coexist with an
  unverifiable human exemption — an unverifiable "sovereign human
  commit" is itself the bypass (TB-R1-05).

Recommendation: build the ACCIDENTAL model, name the adversarial
layer as a separate future goal (it is really "remote enforcement +
durable agent identity", a sibling of the parked genesis-authority
impossibility). Everything below is the accidental model; the
adversarial requirements are recorded so the escalation is a known
step, not a surprise.

## The five conditions, corrected

1. **Classification DECLARED, checked where the message and tree are
   final (TB-R1-02).** The trailer set — `Change-Class:
   loop|direct-fix` with a `Design-Chain:` or `Defect-Proof:` ref —
   lives in the commit message, but `commit.sh -m` parsing is NOT
   the enforcement boundary: `-a`, pathspec, `--include/--only`,
   `--amend`, and `-C/-c/--fixup` all bypass it. Enforcement is a
   COMPOSED pair: a `pre-commit` hook freezes and classifies the
   actual candidate tree (the staged index as Git will write it) and
   writes a tree-bound classification token; a `commit-msg` hook
   validates the final message against that token. `commit.sh`
   remains the orchestrator (it appends canonical trailers from CLI
   flags, TB-R1-07) but is not trusted as the only gate. Adoption
   must COMPOSE an existing pre-commit hook rather than skip
   installation (TB-R1-02, adopt.sh:369). Scope, per the r3
   disposition: raw `git commit` BY HABIT is an honest-forgetting
   case and IS in scope — with hooks active it must be refused for
   an agent; only DELIBERATE bypass (`--no-verify`, hook tampering,
   `core.hooksPath` games) is adversarial and out of scope.

2. **The NEVER-DIRECT-FIX manifest is a conservative FLOOR, read
   from BOTH trees (TB-R1-01).** It is a denylist of path patterns
   (and, for files that mix a contract with ordinary code, of the
   whole file — accepting that an innocent helper edit there takes
   the loop, because a marker around only the struct lets a
   direct-fix change `SchemaVersion` or field population without
   touching it, e.g. internal/supervise/disk.go). The manifest is
   evaluated from the BASE and the CANDIDATE tree, so a direct fix
   cannot delete its own entry or a file's marker in the same
   commit. It is explicitly a FLOOR: catching the listed paths, NOT
   proof that every unlisted edit is mechanical. "Human rulings" are
   NOT in the path manifest — they are unmarkable scattered prose;
   they rely on the decisions-doc / instruction-ledger audit
   (condition below), not commit-time path enforcement.

3. **When in doubt, the loop wins — fail-closed.** An unclassified
   or ambiguous commit is refused; `loop` is never refused, only
   `direct-fix` is challenged, so the cheap way out of doubt is the
   loop.

4. **Growth is a FUSE against a DEFECT IDENTITY, not a per-commit
   size cap (TB-R1-04).** A per-commit budget is trivially gamed by
   splitting one design change across many under-budget direct-fix
   commits. Instead a `direct-fix` declares a DEFECT IDENTITY, and
   scope aggregates across every commit citing that identity against
   an immutable pre-defect base; the fuse blows when the aggregate
   crosses the budget or touches a second subsystem, forcing
   reclassification to `loop`. This is a growth fuse, not a semantic
   classifier — it cannot tell design from mechanics, only stop a
   "small fix" from quietly becoming a refactor.

5. **The EVIDENCE bar is TREE-BOUND, not a string (TB-R1-03).** r1
   showed gaterun records only pid/start/name and the D33 witness
   describes HEAD, not a pending staged tree, and can be green while
   later fixtures fail. Sound checks need: the gate witness records
   its OUTCOME (final zero exit) and the CANDIDATE TREE OID it
   validated; a `direct-fix`'s `Defect-Proof` is ONE REUSABLE
   ASSERTION evaluated against the immutable baseline AND candidate
   trees with a red THEN green outcome and evidence hashes (defined
   to accommodate newly added regression tests, r3); the witness is
   FINALIZED by the whole owning validator at the end of the full
   suite (never go-gate mid-run, r3), and `commit.sh`/the hooks
   locate and bind it to the candidate tree OID. For
   mechanical fixes with no natural failing test (a stray binary,
   unstaged files — the draft's own cases, TB-R1-07), the proof is a
   structured BEFORE/AFTER repository-state assertion, a first-class
   proof kind, not a fabricated test reference.

## The audit join (TB-R1-05)

"Declared and audited" needs a durable join, not a path that merely
exists. `Design-Chain` and `Defect-Proof` refs are CONTENT-BOUND,
REACHABLE references (a bare blob OID is not durable unless
reachable; plans are task-local and deletable, plans/README.md). A validator joins the commit trailer to
critique CLOSURE (the dispositions table, validate critique-closed)
and to the decisions-doc / instruction-ledger; the new global rule
itself gets an instruction-ledger entry with expected effect and a
later verdict. A defined history-audit pass reports commits whose
classification cannot be reconstructed — noting that a missing
classification is indistinguishable from a sovereign human commit
after the fact, which is exactly why the accidental model does not
claim to catch a determined bypass.

## Emergency (TB-R1-06) — CHOSEN (r3): human-personal

A genuine emergency safety fix is committed by the HUMAN personally
(already sovereign), with immediate reconciliation
(docs/collaboration.md:68). No agent override exists and no agent
authorization machinery is built. The one-use tree-bound token idea
is recorded only as a future requirement's option, NOT part of this
design; a reusable env var or generic "emergency" trailer stays
forbidden either way.

## The anti-bureaucracy test (TB-R1-07)

The common mechanical change must stay near one extra line. The
wrapper APPENDS the canonical trailers from CLI flags (the agent
writes `--direct-fix --proof <ref>`, not hand-formatted trailers),
and structured before/after proofs cover the no-failing-test cases,
so ordinary cleanup neither takes a ceremonial loop nor fabricates a
proof. Friction lands only on a change touching a manifest path,
which SHOULD stop and take the loop, and every refusal names the
exact path and the one-step reclassification.

## Prototype plan — SUPERSEDED by the r3 five-step build order

The original P1/P2 split is retired: it mixed pure policy with
witness consumption and assigned witness production to
gaterun/go-gate, which the r3 disposition corrects (the whole
validator finalizes the witness). The build follows the five-step
order in the r3 block at the top of this document, beginning with
settle-the-contracts and the design-obligation matrix.

## Loop discipline

Codex xhigh. The fork is RULED (D90: accidental). The next critique
runs over the settled contracts (step 1 of the build order) before
any code: the trailer grammar with Defect-ID, the manifest schema
and initial set, the proof/witness schemas with whole-validator
finalization, the audit-chain schema, and the hook-composition
lifecycle.
