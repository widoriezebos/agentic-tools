Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal account-provenance)
Date: 2026-09-02

# Goal

Revision 2 of metasystem/plans/account-provenance-design.md (revision 1,
landed; edit it in place, bump the revision line and the claimant named in
its header to the current claimant, m1): fold all eight material findings
of metasystem/records/misc/account-provenance-critique-r1.md, by id. The
goal record (metasystem/plans/goals/account-provenance.md) carries Wido's
word: until this lands, m0 stamps "Wido@M0" in every landing message. Every
closure is a design change verified against the tree, never a softened
claim; where revision 1 promised what the tree cannot deliver, replace the
promise with a mechanism and name the replacement. The design's own rule
stands: the simplest durable record, no new artifact class, no dashboard.

# Workspace

The delegate worktree the dispatcher created for this job. Edit exactly one
existing file, the design; nothing else.

# Direction per finding

- account-provenance-r1-identity-verb-collision: the proposed adapter verb
  name collides with the existing identity verb that returns runtime
  version and configuration hash (metasystem/cmd/metasystem/identity_probes.go,
  metasystem/internal/config, the adapter scripts under
  metasystem/scripts/agents/adapters). Name the account verb distinctly,
  or extend the existing verb's output additively with the account
  object; state the callers of the existing verb and prove none breaks.
- account-provenance-r1-codex-proof-bound: state the Codex grade honestly:
  what a mutable local credential file plus a separate login report can
  prove (claims present, login reported) and what it cannot (issuance,
  validity, binding). Either lower the grade to what is proven or specify
  the verification that earns the higher grade; no grade names more than
  its evidence.
- account-provenance-r1-authority-and-disagreement: replace interval and
  cause claims with point observations: an announcement captures the
  identity at announce time, a job composition captures it at compose
  time, and a difference between two captures is recorded as two
  observations, not as a switch. If an interval claim is wanted, specify
  the re-capture that makes it true (at every paid call boundary) or drop
  it.
- account-provenance-r1-retirement-condition: the landing-message stamp
  retires only when the records carry usable provenance: specify the
  condition on the actual records (both identifiers present and attested
  on the announcement and on every job capture since), not on one
  attestation enum; state who checks it and where.
- account-provenance-r1-non-gating-time-bound: make "never gates dispatch"
  true: a deadline for the identity command, process containment (the
  same group custody and kill-through the adapters use), and the recorded
  outcome when it hangs (unattested, with the cause), for every runtime
  surface, especially the network-dependent Devin one.
- account-provenance-r1-runtime-registry-coverage: distinguish account
  capture for an already-registered runtime (adapter script only) from
  registering a new runtime (engine edit; read
  metasystem/internal/runtimes/runtimes.go), and correct the
  extensibility claim and the reject condition accordingly.
- account-provenance-r1-semantic-validation-fixtures: pin the semantic and
  secrecy contract of the account object with fixtures: attested requires
  identity; unattested carries no identity; unknown fields refused;
  timestamps validated; sensitive errors never stored raw; name each
  fixture and where it runs.
- account-provenance-r1-devin-unresolved-mapping: resolve the Devin
  mapping in the design: which fields, which grade, which failure
  behavior, from the surface the tree documents
  (metasystem/scripts/agents/adapters/devin.sh and the capability snapshot
  shape); if the surface cannot be observed from the sandbox, specify the
  mapping conditionally with an explicit unattested floor and name the
  one live observation the build must record before mapping, as a gap the
  build reports, not a design decision left to it.

Fold record: add a section mapping each finding id to its fold. Self-grade
per the house rule.

# Constraints

Wall-clock budget: 40 minutes. Design only; edit nothing but the design
file. R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file.

# Gap Rule

stop and report a gap; never fill it silently.
