# Genesis authority: one root, and a ledger-shape rule for machinery

- Goals: provision-genesis-authority (the kit's provisioning leg fails at
  adoption's genesis line) and genesis-authority-design (genesis
  authorization that cannot be laundered through a caller-named root).
- Status: DESIGNED 2026-08-19; critique rounds 1–3 folded (the budget is
  spent; §9 records the stop judgment honestly); re-expressed
  after the Mac session's scope addendum (the ledger SEEDING in adopt.sh and
  the ledger format are being rewritten there for the parallel backlog, so
  this arc changes only the AUTHORITY ADMISSION path). Ledger in §9.
  Facts: `plans/genesis-authority-facts.md`. The review it answers:
  `plans/genesis-authority-review.md`.
- Rulings that bind: D93 (C') — unforgeable genesis is not a product
  contract; the cooperative same-user controls stay; no signer, sandbox or
  registry machinery. D96 — refusing machinery at the SOURCE broke the
  delegated validation flows once; not to be retried. AGENTS.md —
  strictness guards invariants, never conveniences.
- The one decision for Wido, stated checkably: **any caller that is
  neither the human nor the target's lease holder — a non-holder MAIN, a
  DELEGATE, a helper — is admitted to genesis when, and only when, the
  ledger it would baseline is goal-free and the checkout's live branch
  (HEAD) tracks no goal ledger.** That is: it may create a control plane
  that says "no intent here", on a root whose history has none, and nothing
  else. The human and the holder keep today's rule. If refused, §7 names
  the fallback.

## 1. What genesis confers, and to whom it must stay closed

Genesis is `goal reconcile` on a root without an accepted baseline. Round 1
refuted the draft's premise that "a goal-free baseline confers nothing": a
Goal-free declaration carries a timestamp, an origin and the plans-stream
digest, and a CURRENT digest is the turn verdict's all-clear (facts §"What
genesis is"). So the rule must keep a non-holder from turning arbitrary
declaration bytes into a baseline. Under the store's lock it already keeps
a non-holder from baselining any ledger that carries goals (`HasGoals`).

Two flows meet at genesis and today's code cannot tell them apart:

- **Adoption.** `adopt.sh` writes a goal-free skeleton whose digest is
  the root's plans-stream digest (the engine's rule, reproduced in shell;
  the Mac session owns and is rewriting that part) into a root with no
  ledger, then reconciles. The caller is whoever provisions: a human, an
  announced main, an interactive session whose announcement lapsed, the
  adopt fixtures and the kit gate from sterile snapshots under agent
  ancestry, the kit gate inside a delegate sandbox. The TARGET carries no
  announcements, so it classifies every agent-ancestry caller DELEGATE —
  which is why a caller-named SOURCE root entered the verb, and with it
  holes 1 and 2.
- **Reconcile-genesis proper.** A ledger that exists without its baseline:
  a hand-written ledger before the first baseline, or an initialized
  project whose `goals-accepted.json` vanished. The principals are the
  target's own — its human, its announced main, its lease holder — and
  the target's registry names them.

The distinguishing fact is not WHO calls but WHAT would be baselined.
Adoption's ledger says exactly one thing — "no intent here" — plus a
declaration whose digest either matches the root's plans stream (the turn
verdict's all-clear) or does not (a BLOCK until the root's principal
declares). Nothing a contract-following non-holder could want is in such
a baseline: it cannot add goals (HasGoals), it cannot renew a declaration
an accepted baseline already holds (a baseline exists → holder-only), and
the declaration's digest, timestamp and origin are its own words exactly
as adoption's skeleton is the adopter's words today — a current digest is
computable by anyone from filenames, so requiring it would protect nothing
and would refuse adoption into a target that already carries other
`plans/*.md` (round 3). So the rule admits every non-holder to exactly
that ledger shape, on a root whose live branch carries no ledger, and to
nothing else. Round 3 also showed that "MAIN, holder or not" is the wrong
cut: a MAIN without the lease exists only where a lease exists, i.e. on an
initialized root — so the rule is HUMAN, HOLDER, and then everyone else
under one condition.

## 2. The rule

**R1 — one root.** `goal reconcile` without a baseline runs in mode
`genesis`, classified against `--root` only — the root being written, as
for every goal verb. `--genesis-from` and `METASYSTEM_GENESIS_AUTHORITY_ROOT`
are removed from `cmd/metasystem/goal.go`, `scripts/adopt.sh:288-289`,
`scripts/adopt-fixtures.sh:21`, `benchmark/validate-kit.sh:345`;
`genesisEffective` and its table test go.

**R2 — the genesis row of the authority matrix** (`internal/authority`,
`Authorize("genesis", classification, "")`): HUMAN → admitted; MAIN with
`holder` → admitted (on a virgin root every authenticated MAIN is the
holder, `verbs.go:220`); any other caller → admitted iff
`classification["adoptionShaped"] == true`, refused otherwise with
"genesis admits a non-holder only for a goal-free ledger on a checkout
whose history carries none". The flag is computed by the command layer
from the root's ledger (R3) and carried in the classification map beside
`class`, `holder`, `jobId` — the matrix stays the one table that decides.

**R3 — adoption-shaped, owned by `internal/goal` (new `genesis.go`),
one function used twice.** `AdoptionShaped(root, ledgerBytes) (bool, reason
string, err error)`: the ledger parses; `!HasGoals()`; and the HEAD guard
below reports no tracked ledger. The command layer calls it before
authorization to fill the matrix flag; the store's genesis arm calls it
again UNDER THE LOCK for every non-HUMAN non-holder and refuses if it no
longer holds (the same belt-and-braces the existing `HasGoals` guard
already uses at `goalverbs.go:608` — that guard is this rule's first
half). `Caller` gains no field; the store's arm reads `caller.Class` and
`caller.Holder`.

The HEAD guard, arm by arm: root not inside a git work tree
(`rev-parse --is-inside-work-tree` fails with "not a git repository") →
no tracked ledger; inside one with an unborn HEAD (`rev-parse --verify -q
HEAD^{commit}` exits 1) → none; `ls-tree --name-only HEAD -- plans/goals.md`
run in the root (git resolves the path against the root's own prefix, so a
nested checkout and a toplevel one ask the same question) non-empty →
tracked, empty → none; git not runnable or any other failure → not
adoption-shaped, with the error (a probe that cannot read refuses rather
than authorizes). What the guard protects: a ledger the checkout's live
branch carries cannot be deleted and re-baselined by machinery — the one
case where rm-then-reconcile would launder a ledger rewrite into a merge.
What it does not claim: that the root "never had" a ledger; an adopted pair
is untracked until the operator's first commit, and if it is deleted before
that, adoption re-run recreates a goal-free pair — anything the deleted
pair held was lost to the rm, and git never had it either.

**R4 — the lock-time guards, unchanged plus one.** A genesis-admitted
caller is refused every non-genesis arm once a baseline exists
(`goalverbs.go:575`); a non-holder is refused a ledger with goals
(`goalverbs.go:608`); and — new — a non-HUMAN non-holder is refused when
the checkout's HEAD tracks a ledger. `refuseMissionSeat` already guards
the whole verb (`goalverbs.go:560-564`).

**R5 — adoption.** `adopt.sh` keeps writing its skeleton (the Mac's
region, untouched). At the reconcile line it first asks the engine,
read-only, whether the target already carries a ledger whose accepted
baseline matches (`goal list`'s `baselineMatches`); if so it leaves the
pair alone — a re-run over a healthy pair is not a write and must not be
refused as one (today's line would refuse a DELEGATE-shaped caller there
in holder-only mode; the same-SHA early exit usually hides it). Otherwise
it runs `goal reconcile --root "$target"` — no second root. The skeleton is
goal-free by construction, so every caller is admitted into a root whose
HEAD tracks no ledger; when the target already carried other `plans/*.md`
the declaration's digest is stale from birth and the target's turn verdict
BLOCKS until its principal declares — the safe direction (the seeding
rewrite should compute the digest over the target's actual plans set;
named in the report, not changed here). A re-run after an interrupted
adoption finds the skeleton present and reconciles it on the same terms.

**R6 — what remains caller-steerable, stated plainly.** `--root` names the
object of the write; `--caller-pid` names the ancestry, as for every
classified verb in the system (facts §rulings). Neither is a second
classification root. The classifier reads HUMAN when the process table
cannot be walked (`classify.go:323`, `classify_test.go:47-55`): in a sandbox
that denies process visibility a delegate is HUMAN-shaped for this verb
exactly as for `goal open` and every holder-only verb — the classifier's
recorded C' posture, system-wide, not widened here; an "indeterminate"
class for a truncated walk is named as follow-up. A deliberate same-user
actor can still write the control-plane files directly or craft a target's
registry; C' territory, and nothing `cp` does not buy.

## 3. Why this closes the review's holes

- Hole 1 (crafted source root → MAIN): the mechanism is gone; no verb
  classifies against a root other than the one it writes.
- Hole 2 (source results discarded; helper → target HUMAN): gone with the
  two-root rule; under R2 every class is judged once, against the target.
- Hole 3 (authorization-to-lock race): closed since D94/D96, unchanged;
  the new shape rule is re-checked under the same lock.
- Round 1's premise finding: a non-holder may baseline only a goal-free
  ledger on a root whose live branch has none — adoption's own statement,
  nothing more; its declaration is its own words, as adoption's is.

## 4. Each flow, under the rule

| Flow | Target class | Outcome |
| --- | --- | --- |
| Human at a terminal adopting | HUMAN | admitted (unchanged) |
| Announced main session adopting | DELEGATE (its announcement lives in its own checkout) | admitted: adoption-shaped |
| Interactive session whose announcement lapsed (the Mac today) | DELEGATE | admitted: adoption-shaped — and every OTHER goal verb still refuses it, correctly; the remedy for those is announcing (hook defect reported separately) |
| Adopt fixtures / kit gate from a sterile snapshot, agent ancestry | DELEGATE | admitted, no env hook |
| Kit gate inside a delegate sandbox, process table visible | DELEGATE | admitted (today: refused) |
| Kit gate where the process table is denied | HUMAN | admitted (unchanged; R6 residual) |
| Nested adoption; interrupted adoption re-run (skeleton present, no baseline) | DELEGATE | admitted: adoption-shaped |
| Hand-written ledger, no baseline, by the target's human or announced main | HUMAN / MAIN | accepted if parse-legal; goals only if holder (unchanged) |
| Same, by a DELEGATE-shaped caller or a non-holder MAIN: goals present, or HEAD tracks the ledger | DELEGATE / MAIN non-holder | refused |
| Adoption into a target that already carried other `plans/*.md` | any | admitted; the declaration is stale from birth, the target's verdict blocks until its principal declares |
| Re-run of adoption over a healthy pair | any | no write: adopt.sh skips reconcile when the baseline matches |
| Holder restoring a deleted baseline over a populated ledger | MAIN + holder | re-baseline (unchanged) |
| Delegate in its worktree deletes the tracked baseline of a goal-free ledger and reconciles | DELEGATE | refused: HEAD tracks the ledger |
| Mission active in the root | any | refused by the mission-seat guard (unchanged) |

## 5. Properties and proofs

- P1 Availability: every "admitted" row passes; proof = `adopt-fixtures.sh`
  green from agent ancestry with the env export removed; `validate-kit.sh`
  provisioning leg green from this (announced) session AND after retiring
  the announcement (DELEGATE-shaped).
- P2 Caller bytes never enter a ledger without holder-grade authority
  except a goal-free first baseline on a root whose HEAD tracks none: proof = `authority_test.go`
  genesis row (HUMAN; MAIN holder; MAIN non-holder and machinery with and
  without the flag); store tests for the genesis arm (non-holder + goals →
  refused; non-holder + HEAD-tracked → refused; non-holder + goal-free
  untracked → accepted, stale digest included; HUMAN + anything
  parse-legal goal-free → accepted as today; holder + goals → accepted);
  command-layer test of `goalCaller` in `cmd/metasystem` (the lease
  package's child-process pattern: a signature matching our own command
  makes the child DELEGATE) — DELEGATE + reconcile on an adoption-shaped
  root → admitted with `Genesis`; on a goal-bearing root → refused; DELEGATE
  + open → holder-only refusal; HUMAN → admitted. Plus the adopt fixture
  asserting, after adoption from agent ancestry, that `goal open` in the
  target is refused when the fixture's own classification there is not
  HUMAN/holder.
- P3 The HEAD guard's arms: non-repository, unborn HEAD, committed without
  the ledger, committed with it, nested prefix (repository at a parent
  directory, root a subdirectory), a broken git invocation → not shaped.
- P4 Race guards: existing tests; the new arm is under the same lock.

## 6. Changes, by owner

| Owner | Change |
| --- | --- |
| `internal/goal` (new `genesis.go` + tests) | `AdoptionShaped`, the HEAD-guard probe (git exec'd directly; no dependency on the wall program's tree package) |
| `internal/goal/goalverbs.go` | the genesis arm gains the machinery re-check under the lock (one hunk beside line 608) |
| `internal/authority` | genesis row: machinery admitted iff `adoptionShaped` |
| `cmd/metasystem/goal.go` | goalCaller: no `--genesis-from`, no `genesisEffective`; computes the flag pre-lock |
| `cmd/metasystem/goal_test.go` | `TestGenesisEffective` → command-layer goalCaller boundary test |
| `scripts/adopt.sh:280-290` | read-only baseline-match skip, then reconcile without `--genesis-from`; comment |
| `scripts/adopt-fixtures.sh` | drop env; post-adoption holder-only assertion |
| `benchmark/validate-kit.sh` | drop env |
| decisions doc | one entry for Wido's ruling; `plans/genesis-authority-review.md` stays as the record |

Not changed, by the Mac session's addendum: adopt.sh's skeleton writing
and non-git target handling; the ledger format.

## 7. Refuted alternatives

- G (round-0 draft): admit DELEGATE at genesis unconditionally because the
  baseline is "worthless". Refuted by GA-R1-01.
- G2 (round-1 rewrite): a `goal seed` verb writing the skeleton
  engine-side, reconcile-genesis HUMAN/MAIN only. Survived round 2 with
  amendments; set aside by the Mac's addendum (the seeding bytes and their
  format are being rewritten there). The doctrine point survives as a
  recommendation for that rewrite: adoption's ledger should be
  verb-written, not shell-written, and its digest computed over the
  target's actual plans set.
- G′a: keep "never machinery" and discover the authority root from a
  per-user registry or the ancestors' cwd/engine location. Refuted:
  same-user-writable (C' concedes it), staleness semantics, and it still
  refuses the lapsed session and the sandboxed kit — machinery for a gate
  that the shape rule makes unnecessary.
- G′b: a capability minted by the source's holder. Refuted: the grant
  must name its source to be verified (`--genesis-from` with a nonce), the
  harness would again choose which root mints for snapshot/nested flows,
  and it certifies the SOURCE's holder, which the target's ledger never
  checks.
- Fallback if Wido refuses the shape rule: today's matrix row (HUMAN/MAIN
  only) against the target — which re-breaks every agent-ancestry flow the
  kit must run under until the session-start hook announces reliably, and
  never admits the sandboxed kit gate.

## 8. Obligation matrix

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GA-1 | CRITICAL | §2 R1 | genesis classifies against `--root` only; no second root, no env var | cmd/metasystem/goal.go | goalCaller | goalCaller boundary test; adopt-fixtures without the env | validate-kit.sh genesis leg, announced and lapsed | DONE | — |
| GA-2 | CRITICAL | §2 R2 | matrix genesis row: HUMAN; MAIN holder; everyone else iff adoptionShaped | internal/authority | Authorize | TestGenesisMode | — | DONE | — |
| GA-3 | CRITICAL | §2 R3–R4 | AdoptionShaped (goal-free, no HEAD-tracked ledger) re-checked under the lock for every non-HUMAN non-holder | internal/goal | genesis.go, Reconcile | genesis arm tests, HEAD-guard arm tests | — | DONE | — |
| GA-4 | HIGH | §2 R5 | adopt.sh, fixtures, kit carry no genesis root plumbing | scripts/adopt.sh, scripts/adopt-fixtures.sh, benchmark/validate-kit.sh | the scripts | adopt-fixtures.sh green from agent ancestry | validate-kit.sh provisioning leg | DONE | — |
| GA-5 | HIGH | §5 P2 | after adoption from agent ancestry, `goal open` in the target is holder-only | cmd/metasystem/goal.go (unchanged) | Authorize holder-only | adopt-fixtures assertion + goalCaller test | — | DONE | — |

## 9. Critique ledger

Round budget: three focused rounds on one critic chain (design-critic,
codex gpt-5.6-sol, job `genesis-authority-critique-2`).
Round 1 → `plans/genesis-authority-dispositions-r1.md` (5 material: 4
accepted, 1 refuted). Round 2 → `plans/genesis-authority-dispositions-r2.md`
(6 material: all accepted; the amendments were first expressed on the
seed verb and are carried into this re-expression — see that file's
closing note). Code critique (implementation layer) →
`plans/genesis-authority-code-critique.md` (9 findings, 4 material, all
folded; the GIT_DIR steering hole it found in the new probe is closed with
its test). Round 3 → `plans/genesis-authority-dispositions-r3.md`
(4 material: 3 accepted — the uniform non-holder cut, dropping the
digest-truthfulness term, the healthy-pair skip — and 1 refuted on the
mechanism with its claim text corrected). The budget is spent and round 3
was not empty, so this is NOT a claim of convergence: two of round 3's
four findings were introduced by the previous round's own re-expression
(the truthfulness term; the moot-mapping of the adopt skip), one restated
round 1's disagreement on the decision already reserved for Wido, and one
was a factual error; the architecture (one root, one shape rule under the
lock) was not challenged. Per the skill's diminishing-returns rule the
loop stops here, implementation is the next source of truth, and the
rounds are retained verbatim under `artifacts/agents/genesis-authority-critique-2/rounds/`
and in the dispositions files.
