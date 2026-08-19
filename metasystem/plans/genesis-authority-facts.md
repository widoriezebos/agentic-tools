# Genesis authority: fact sheet (code-grounded, 2026-08-19, HEAD 7606339)

Every claim the redesign rests on, anchored to the shipped code. The
critique attacks judgment; facts it must correct are rounds lost.

## What genesis is

- `goal reconcile` against a root with NO `plans/goals-accepted.json` runs
  in authorization mode `genesis`; every other goal mutation runs
  `holder-only` (`cmd/metasystem/goal.go:41-50`, the mode is chosen by a
  pre-lock `os.Stat` at `goal.go:48`).
- Genesis classification today: the TARGET view, raised to the SOURCE
  view only when `--genesis-from <root>` classifies the caller as an
  announced MAIN (`goal.go:60-67`, `genesisEffective` at `goal.go:119-124`).
  The source root is a verb input: the `--genesis-from` flag, fed by
  `scripts/adopt.sh:288-289` from `METASYSTEM_GENESIS_AUTHORITY_ROOT` (default:
  the adopting checkout), exported by `scripts/adopt-fixtures.sh:21` and
  `benchmark/validate-kit.sh:345` as the LIVE checkout.
- The authority matrix in genesis mode admits HUMAN, MAIN (holder or not),
  and refuses every other class with "genesis admits only the human or a
  main agent" (`internal/authority/authority.go:29-33,38-48`).
- The store re-checks under its lock: a genesis-admitted caller is refused
  every non-genesis arm once a baseline exists (`internal/goal/goalverbs.go:575`,
  test `goalverbs_test.go:518`), and a ledger that already CARRIES GOALS is
  refused to a non-holder (`goalverbs.go:608`, `HasGoals` at
  `internal/goal/goal.go:86`; tests at `goalverbs_test.go:471,499`). A
  non-holder genesis therefore writes a baseline over a goal-free ledger,
  or nothing.
- The first goal mutation on a baselined ledger (`goal open`,
  `declare-free`, `park`, ...) is holder-only against the TARGET: HUMAN, or
  MAIN holding the target's lease (`authority.go:29-33,50-52`; holder =
  an authenticated main when NO lease exists, or the one whose mainId
  matches the lease when one does — `internal/lease/verbs.go:220`; so a
  non-holder MAIN exists only on a root that has a lease). `goal open` may also start a ledger from
  nothing, under the same holder-only rule (`goalverbs.go:169-174`).

## How a caller is classified (per root)

- `lease.Classify(root, pid)` (`internal/lease/classify.go:282-324`): the
  caller itself is checked for an authenticated announcement only
  (`classify.go:291-293`); then each ancestor in turn — an authenticated
  announcement in `<root>/artifacts/agents/mains` makes it MAIN
  (`classify.go:306`); else an adapter-signature match makes it DELEGATE
  (`classify.go:309`, signatures compiled from `<root>/scripts/agents/adapters/*.sh`
  at `classify.go:167`); else supervision/job custody records make it
  SUPERVISION or ADAPTER-SUPERVISOR; no recognised ancestor is HUMAN
  (`classify.go:323`). Nearest recognised ancestor wins.
- Announcements authenticate by pid, start time (or ticks+boot id) and
  command hash (`classify.go:129-160`); nothing binds them to a root.
- Adoption copies EVERY adapter script into the target (`adopt.sh:247-255`),
  so a virgin target classifies any agent-CLI ancestor as DELEGATE and a
  terminal as HUMAN; it carries no announcements and no custody records.
- The adopt skeleton ledger is goal-free by construction
  (`adopt.sh:200-212`: "## Goal-free: declared <ts> by human over <digest>"),
  hand-written in shell, duplicating `goal.ScanDigest`'s rule (sorted
  `plans/*.md` basenames minus goals.md; `internal/goal/goal.go:121-126`).
- A Goal-free declaration is not an empty marker: it carries a timestamp,
  a free-text origin and the plans-stream digest (`goal.go:58-63`,
  `parseFree` at `goal.go:295-314` accepts any origin and digest);
  `HasGoals` ignores it (`goal.go:86`); the turn verdict reads a
  baseline-matched declaration whose digest equals the CURRENT ScanDigest
  as "NOTHING LEFT TO WORK ON" and a stale one as a block
  (`internal/goal/turnverdict.go:408-417`; `turnverdict_test.go:157-194`).
  Renewing it on a baselined ledger is `goal declare-free` (holder-only;
  `goalverbs.go:527`) or a replayed manual edit (holder-only,
  `goalverbs.go:636-670`); on a ledger WITHOUT a baseline it is whatever
  genesis accepts.
- Store verbs carry no authority of their own: `Store.Open`, `DeclareFree`
  use the Caller only to stamp the origin (`goalverbs.go:44-48,327,399`);
  authorization is the command layer's, before the store is called
  (`goalverbs.go:17-24`, `goal.go:73`). The holder bit is sampled there,
  before the store's lock, for EVERY goal verb.

## Observed today (ran it, this machine, macOS arm64)

- Unannounced session: `lease classify --root . --caller-pid $$` →
  `{"class":"DELEGATE"}` for a shell under the interactive Claude CLI
  (pid 37250) whose announcement had been retired. `scripts/adopt.sh <tmp>
  --runtimes none` then failed at the genesis line with "genesis admits
  only the human or a main agent" — the BM-1 failure, reproduced.
- Same session after re-announcing (`arm-supervision.sh --pid 37250 ...`):
  `{"class":"MAIN","holder":true}`; the same adopt passed genesis, and
  `benchmark/validate-kit.sh` passed the genesis leg (it then failed on a
  separate, newer blocker: "always-loaded instructions exceed 1400 words",
  AGENTS.md 923 + wow.md 549 words since 86bd66a).
- The session-start hook cannot re-announce in this checkout layout:
  `supervision-hook.sh claude start` → "could not identify the immediate
  claude agent process" because it passes the git toplevel as
  `find-ancestor --repo` and the toplevel has no `scripts/agents/adapters/`
  (`supervision-hook.sh:56,67`; `internal/census/ancestor_production.go:106`).
  So an interactive session here is announced only while some earlier
  arming survives; a `/clear` (SessionEnd → retire) leaves it DELEGATE-shaped
  for every control-plane verb, not only genesis.

## The three recorded holes (plans/genesis-authority-review.md)

1. Crafted source root with a copied live announcement → source MAIN →
   admitted. Depends on the caller naming the classification root.
2. Adapter-supervisor: every non-MAIN source result is discarded for the
   target view; against a virgin target (no custody records) the walk
   continues to the next recognised ancestor (an agent CLI → DELEGATE, or
   none → HUMAN). Depends on the two-root rule.
3. Authorization-to-lock race: CLOSED since D94/D96 (`goalverbs.go:575`,
   test at `goalverbs_test.go:518`).

## Rulings that bind

- D93 (C'): "unforgeable genesis" is not a product contract; the
  cooperative same-user controls stay; no signer or sandbox machinery.
- D96: a machinery refusal at the source was tried and reverted — it
  broke the adopt fixtures and the kit gate, whose announcement-free
  snapshot sources classify DELEGATE by signature under agent ancestry.
- AGENTS.md (since 86bd66a): strictness guards invariants, never
  conveniences — a refusal must name the invariant it protects.
- Every classified verb accepts `--caller-pid`; `lease run-held`, commit,
  dispatch pass their own `$$` (`scripts/agents/commit.sh:9-17`). A
  classification is steerable by that pid for every verb in the system;
  the goal verbs inherit that posture (`goal.go:42-44`).
