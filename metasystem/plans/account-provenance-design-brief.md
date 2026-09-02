Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal account-provenance)
Date: 2026-09-02

# Goal

Author the design document account-provenance-design.md, a NEW file you
create in the metasystem plans directory: the run record carries the account
identity that paid for the work, alongside runtime and model, so cost and
capacity attribution survive in records rather than in memory and landing
messages.

# Evidence

- The goal record: metasystem/plans/goals/account-provenance.md (in your
  worktree) — m0 and m0b run a separate Claude account from m1/m2 (a
  distinct capacity pool is why they exist); today every landing hand-writes
  "account Wido@M0" as a conduct rule, which is exactly the
  memory-not-records failure this system keeps killing.
- The record surfaces to consider: the session announcement
  (artifacts/agents/mains/session-*.json — see its current fields), the
  job/launch record (artifacts/agents/jobs/*.json), and the landing message
  convention.

# Design questions

1. WHERE the account identity enters: announcement time (per session),
   launch time (per job), or both — with the rule for which record is
   authoritative when they disagree.
2. PROOF over self-declaration: what can actually attest the account — the
   runtime CLI's own identity surface (whoami-equivalent, config path,
   credential file identity), probed at announcement like the capability
   snapshots — versus an operator-set config key that is honest but
   unproven. If only self-declaration is practical, say so plainly and
   bound the claim (the R-24 discipline: never claim a guarantee the
   mechanism does not refuse).
3. The landing-message conduct rule retires when the record carries it:
   state the retirement condition.
4. Per-runtime coverage: claude and codex both bill; devin is rostered but
   uninstalled — the shape must not need a new edit per runtime (registry
   pattern, like the adapter probes).
5. Fixtures and blast (consumers of the records you touch).

Self-grade with reject condition.

# Constraints

Wall-clock budget: 25 minutes. Design document only; one new file in the
metasystem plans directory. Small-goal scope governor: the simplest record
that makes attribution durable wins; cost DASHBOARDS are other goals'
business (reconciliation-guards, actionable-metrics).

# Expected Return

Version-2 implementer JSON; diffBoundary lists exactly the one new design
file you created.

# Gap Rule

stop and report a gap; never fill it silently.
