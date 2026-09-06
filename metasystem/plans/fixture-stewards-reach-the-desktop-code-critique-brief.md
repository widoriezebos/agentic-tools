Working Mode: implement
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal fixture-stewards-reach-the-desktop)
Date: 2026-09-06

# Review brief: fixture stewards stop reaching the desktop (chain fsrd-build1b-20260906)

FINDING IDS: chain-unique, FSD-01, FSD-02, ... never F-n.

Round budget: one focused round, then at most one correction and its
re-review. Material only if it changes what gets built and names the
artifact.

Threat model, in order of harm: a REAL steward (human-terminal or
temporary-word enrollment, or a legacy identity without the field)
that stops reaching the desktop, which is the alert channel failing at
its one job; a fixture steward that still reaches the desktop through
any path (the alert episode path in alert_episode.go, DeliverPending,
a mint that stamps the wrong kind, a machine rebuild that loses the
kind, a restart that re-stamps a fixture as human); the fixture
predicate in lease.ClassifyAt marking a real terminal as fixture-granted
or a fixture terminal as real; a delivery that returns an error for a
fixture and so blocks a launch gated on delivery; the notifications log
growing without bound or written outside the fixture's own root; the
osascript shim in the suites masking a real failure or not failing the
suite when invoked; a change outside the named owners.

Out of scope: the fixture beds' other leaks (goal fixtures-leak
records); taste.

Scope: the computed diff of the implementer job under review.
Contract: metasystem/plans/fixture-stewards-reach-the-desktop-build-brief.md
and the goal record
metasystem/plans/goals/fixture-stewards-reach-the-desktop.md.

# Mandate

1. Every mint path (arm, arm with a temporary word, restart, machine
   rebuild) stamps the right enrollment kind, and an identity without
   the field reads as human-terminal; name the test for each.
2. Every delivery path consults the kind: a fixture never reaches
   osascript, a human always can, a configured command wins for both;
   name the path you traced for the alert episode and for pending
   deliveries.
3. The fixture predicate is exactly "HUMAN class granted by a fixture
   authorization, or metasystem.runtimes=fake", and a real terminal can
   never satisfy it; say how you know.
4. The three suites' osascript shims fail the suite when invoked, and
   the health suite proves the fixture-local delivery.
5. Nothing outside the named owners changed.

If nothing material remains, say so; that closes the chain.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
fsrd-build1b-20260906.

# Gap Rule

stop and report a gap; never fill it silently.
