# Two bars for changes (backlog item 2)

- Status: CONVERGED for implementation on the ACCIDENTAL model — the
  human ruled the fork (D90, 2026-08-17): "stop an honest agent from
  forgetting the rule". r1 critique (plans/two-bars-critique-r1.md,
  7 findings, 6 structural) folded into r2; the one open question
  (the threat-model fork) is now answered. The adversarial layer is
  a SEPARATE future goal, not built here.
- Goal: two-bars-for-changes (design done; implementation queued
  behind the ACP flip)
- In flight right now: none.
- Waiting on the human: nothing — the fork is ruled.
- Next step: none (queued). When picked up, implement the accidental
  model per the sections below (the composed pre-commit + commit-msg
  hooks, the both-trees manifest floor, the tree-OID-bound witness,
  the defect-identity growth fuse, the immutable-ref audit join,
  human-personal or one-use emergency) — D81 implementation-first,
  this document is the spec; a code critique gates the ship.

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
- **ADVERSARIAL model:** stop a misaligned agent from deliberately
  laundering a design change (raw `git commit`, `--no-verify`,
  `-a`/pathspec bypasses, forged same-user proof files). r1 shows
  this cannot be done with local hooks and string trailers: it needs
  REMOTE enforcement (CI that rejects a push whose commits lack a
  valid tree-bound attestation), DURABLE agent identity (a signing
  key, not a same-user file), and it CANNOT coexist with an
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
   installation (TB-R1-02, adopt.sh:369). In the accidental model,
   `--no-verify` and raw `git` are out of scope (they are the
   adversarial layer); the design says so plainly.

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
   validated; a `direct-fix`'s `Defect-Proof` is the SAME command
   run against the immutable baseline AND candidate trees with a red
   THEN green outcome and evidence hashes; `commit.sh`/the hooks
   locate and bind the witness to the candidate tree OID. For
   mechanical fixes with no natural failing test (a stray binary,
   unstaged files — the draft's own cases, TB-R1-07), the proof is a
   structured BEFORE/AFTER repository-state assertion, a first-class
   proof kind, not a fabricated test reference.

## The audit join (TB-R1-05)

"Declared and audited" needs a durable join, not a path that merely
exists. `Design-Chain` and `Defect-Proof` refs are IMMUTABLE
`commit:path` or blob identities (plans are task-local and
deletable, plans/README.md). A validator joins the commit trailer to
critique CLOSURE (the dispositions table, validate critique-closed)
and to the decisions-doc / instruction-ledger; the new global rule
itself gets an instruction-ledger entry with expected effect and a
later verdict. A defined history-audit pass reports commits whose
classification cannot be reconstructed — noting that a missing
classification is indistinguishable from a sovereign human commit
after the fact, which is exactly why the accidental model does not
claim to catch a determined bypass.

## Emergency (TB-R1-06)

Pick ONE, no reusable escape:

- **Human-personal:** a genuine emergency safety fix is committed by
  the HUMAN personally (already sovereign), with immediate
  reconciliation (docs/collaboration.md:68). No agent override
  exists. Simplest; recommended for the accidental model.
- **One-use authorization:** if an agent must act, the human mints a
  ONE-USE token bound to the exact candidate tree OID, reason,
  expiry, the specific checks skipped, and a mandatory
  receipt/handoff reconciliation. A reusable env var or generic
  "emergency" trailer is forbidden — it reopens the hatch.

## The anti-bureaucracy test (TB-R1-07)

The common mechanical change must stay near one extra line. The
wrapper APPENDS the canonical trailers from CLI flags (the agent
writes `--direct-fix --proof <ref>`, not hand-formatted trailers),
and structured before/after proofs cover the no-failing-test cases,
so ordinary cleanup neither takes a ceremonial loop nor fabricates a
proof. Friction lands only on a change touching a manifest path,
which SHOULD stop and take the loop, and every refusal names the
exact path and the one-step reclassification.

## Prototype plan (after the human rules the fork)

P1: the `change class` verb — parse+validate the trailer set;
evaluate a candidate-tree file list against the manifest read from
both trees; aggregate direct-fix scope by defect identity; bind and
verify a tree-OID gate witness; pure Go, table tests per refusal and
pass. P2: the composed pre-commit (tree-bound token) + commit-msg
(final message) hooks, commit.sh trailer-append, adoption hook
composition, the witness outcome+OID extension in gaterun/go-gate,
and the audit-join validator; fixtures for a direct-fix refused on a
manifest path, a direct-fix passing on an ordinary path with a
before/after proof, a loop passing with an immutable design-chain
ref, and a split-commit defect-identity fuse blowing.

## Loop discipline

Codex xhigh; resumes after the human rules the accidental/adversarial
fork. If accidental (recommended), the next critique attacks the
tree-bound witness binding and the composed-hook completeness; if
adversarial, the design first has to solve durable agent identity
and remote enforcement, which is a different and larger goal.
