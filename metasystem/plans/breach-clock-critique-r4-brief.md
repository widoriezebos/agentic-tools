Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal breach-clock-and-budget-honesty)
Date: 2026-09-02

# Goal

Round-4 critique of metasystem/plans/breach-clock-and-budget-honesty-design.md
revision 5 (landed, in your worktree). Revisions 4 and 5 fold your one
round-3 finding, BCD-R3-001 (metasystem/records/misc/breach-design-critique-r3.md,
landed, with the orchestrator's decision and addendum at its foot): the
claim binding gains a third episode key, `episodeObligationRevision`,
written by `rebindClaimKeepEpisode` at every raise from the obligation live
the moment before (metasystem/internal/goal/verbs.go lines 122-124 and `SetBudget`
from line 487, which today rebinds the claim without the key), and, when no obligation is live at the raise,
INHERITED unchanged from the prior claim binding so a second raise cannot
rewind the start; with `file.Obligation == nil` a discharge proof is
eligible only when that key is non-zero and equals the proof's obligation
revision; the render and parse rule sits beside the existing two episode
keys in metasystem/internal/goal/file.go; `ValidateClaimRevision` gains the
implication; the proof plan holds one test per sequence.

Judge the closure BY ID against the tree: walk the five sequences the design
states (discharge→raise; discharge→set-obligation;
discharge→set-obligation→raise; discharge→raise→set-obligation→raise;
discharge→raise→raise) and any sixth you can construct from set-budget,
set-obligation, discharge and release-and-reclaim, and say for each whether
the start ever moves at a raise in either direction; check the inherit rule
against `bindClaim` and `clearClaimBinding` (a fresh claim episode must
start at 0, a raise must not); check the parse refusal wording and that the
hand-edit mapper in metasystem/internal/goal/reconcilemap.go still refuses
every altered Claimed line so the new key adds no hand-edit surface. Confirm
no regression in what rounds 1 to 3 left standing (Fix 3 whole, Fix 2's
decision, migration and day-token inventory, the rollout table).

Findings material and grounded, quoting the disagreeing text or code, ids
BCD-R4-NNN. Your sandbox is read-only: verify by reading, do not run go.
Zero material findings is an acceptable, closing answer if the reading
supports it.

# Constraints

Wall-clock budget: 40 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
