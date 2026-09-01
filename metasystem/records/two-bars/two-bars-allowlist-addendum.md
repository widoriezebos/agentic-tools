# Two-bars design addendum: `memory/receipts.log` and the carriage allowlist

Design-lane addendum to `plans/two-bars-for-changes-design.md` §5, in that
design's voice, via R-25b's deviation path. The implementer lawfully
gap-stopped on one question the seeded allowlist left open: does
`memory/receipts.log` belong on `scripts/agents/register-carriage-paths.txt`?

## Ruling: yes — `memory/receipts.log` rides carriage, APPEND-ONLY

The carriage exemption exists for exactly one shape of file: a running-state
register that standing law forces onto ordinary landings, so that omitting it
from the allowlist would make the bars indict the honest actor. The receipt
log is the purest instance of that shape in the tree:

1. **The law forces it onto every shipping landing.** AGENTS.md (Completion):
   "If the task changed the repository, append a receipt with
   `scripts/receipt.sh add`." A receipt describes the landing it rides with,
   so it cannot pre-exist the chain's reviewed tree — a delegate's reviewed
   tree cannot contain the receipt for a landing that has not happened. Off
   the allowlist, bar (a)'s path partition therefore fails on every honest
   receipt-bearing landing, and in observe mode every such landing writes a
   false would-refuse. A flood of false positives from the most honest
   landings destroys the observe window's evidentiary value and, with it, the
   observe→enforce promotion decision the governance obligation depends on.
2. **It is engine-written and shape-checked.** `scripts/receipt.sh` is a shim
   that execs `metasystem receipt`; `receipt check` parses the pipe-delimited
   lines and fails on garbage (validate-metasystem.sh:2414). This is a
   stronger mechanical position than the narrator digest, which §5 already
   carries content-unverified as a deliberate bounded residue.
3. **The against-case, weighed.** Every carriage entry widens the
   unexamined-byte lane, and `note=` is free text. The bound is the same
   discipline §5 imposed on rulings: `memory/receipts.log` carriage is
   APPEND-ONLY — the engine refuses carriage when the staged diff for the
   path deletes or modifies any existing line; only trailing additions
   qualify, re-checked at the push boundary against each outgoing commit's
   own parent-to-commit diff (§8), exactly as the rulings add-rows-only rule
   is. A rewrite of receipt history is a falsified completion record and
   takes a bar. What remains unexamined is one appended free-text line per
   landing — the same residue class as the digest, and narrower than r1's
   withdrawn lanes.

## Ruling: `memory/findings.md` comes OFF the seeded list

Restating the list verbatim forces a stance on every entry, and the tree
contradicts this one: no file `memory/findings.md` exists, nothing in
`scripts/`, `docs/`, or `AGENTS.md` writes or reads it, and the only mention
anywhere is the design's own §0/§5 prose. An allowlist row whose path has no
writer and no law is a dormant free-text lane anyone can instantiate at will
— precisely the widening the against-case names — bought for nothing. It is
struck. If a chain in flight creates a real register by that name, its
reinstatement is one allowlist line and rides that chain's own bar (a)
landing; the allowlist sits on the floor, so the re-add is examined by
construction.

## The revised §5 seeded list, verbatim

- Seeded entries: the narrator digest log, `memory/rulings.md`
  (ADD-ROWS-ONLY, as §5 defines), `memory/receipts.log` (APPEND-ONLY, as
  defined above), and `plans/handoff-*.md` (the one permitted glob form,
  §4).

## The other registers the tree shows, each ruled

- **Goal-ledger materializations (`plans/goals/**`): no entry, §5 upheld.**
  The engine publishes ledger commits through plumbing
  (`git commit-tree`, internal/goal/txn.go:284), so they never pass the
  porcelain pre-commit chokepoint and generate no would-refuse noise; their
  bar is the goal-side `ValidateCommit` every machine runs. On ordinary
  landings the fence (pre-commit-guard.sh:62-68) already refuses the paths.
  A carriage entry here would be strictly harmful: it would let a hand edit
  ride one flag around the goal-verb monopoly. `plans/goals.md` and
  `plans/goals-accepted.json` do not exist in this tree (they appear only in
  fixture path lists) and get no entry either.
- **Goal journal files (`artifacts/agents/goal-transactions/`): no entry.**
  The path is gitignored; it never reaches any commit boundary. Allowlist
  rows are for committed registers only.
- **The remaining `memory/` files (`known-issues.md`, `flake-registry.md`,
  `instruction-ledger.md`, `backlog-notes.md`, `proposal-drafts.md`,
  `llm-wiki-pattern.md`): no entries.** They fail the carriage test: no
  standing law forces them onto a landing that ships other work, and their
  entries are authored seat judgments, not engine-emitted records of the
  landing itself. Authored content takes the loop — the same principle that
  withdrew `prose-docs`. If the observe window shows this costing more than
  the fleet bears, the lawful adjustment is a bar (a) allowlist change, never
  a quiet widening.
