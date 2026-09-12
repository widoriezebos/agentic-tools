Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal stop-refusal-fits-on-one-screen, reopened)
Date: 2026-09-09

# Review brief: the allow path and the health line (chain one-screen-build2b-20260909)

FINDING IDS: chain-unique, OSR-11, OSR-12, ... never F-n and never a
reused id (OSR-01 to OSR-10 belong to the first slice's chain).

Report `round` as 1 in your return: it is this job's own round.

Round budget: 1 focused round, then at most one correction and its
re-review (tier 3). R-60-m1's rule: material only if it changes what
gets built and names the artifact.

Why this slice exists: the first slice bounded the blocking refusal.
The first Stop message after it landed was an ALLOW, and the hook
appended its check-in tail unbounded: the whole health line, sixteen
checks with remedies, about 2,400 runes, and wrong, reporting a live
supervisor dead because health read the repository root instead of the
installation root on this template-layout seat.

Threat model: the root split reading the wrong root on a seat where the
repository root IS the installation (an adopted repository, the common
case: nothing may change there); a role that still reads under the
repository root after the split (compare every artifact-backed role,
the remedies it prints, and the alert episode records); the hook still
passing the repository root as --repo so the preview and the ordinary
health verb disagree; the compact preview dropping a dead or unknown
check, or counting a dead check as alive; the full health line written
non-atomically, outside the verdict's flock, or under a name that
collides across sessions; report bound-message trimming the verdict
line of an allow message instead of its tail; a start-event notice
routed through the bound when the brief said non-start only (say
whether that is right); the blocking path changed at all (it must not
be); the new template-layout fixture passing for a reason other than
the bound (the first slice's lesson: a fixture that cannot fail for its
own reason); a change outside the eight named files. Out: wording of
the compact line; taste.

Scope: the computed diff of the implementer job under review, eight
files: main.go, report.go, report_verbs_test.go and steward_verbs.go
under cmd/metasystem; health.go and health_test.go under
internal/steward; supervision-hook.sh and supervision-hook-fixtures.sh
under scripts/agents. Contract: the second-slice build brief,
stop-refusal-fits-on-one-screen-build2-brief.md in the plans directory
(new, not yet committed), and the goal record
metasystem/plans/goals/stop-refusal-fits-on-one-screen.md. The
implementer's return is under the job's first round in the agents
artifacts; it says which proofs its sandbox could not run (the hook
suite and the combined package test); the orchestrator replays both
before your return lands.

# Mandate

1. Every artifact-backed health role, its remedy text and the alert
   records read the installation root when one is given and the
   repository root otherwise, and ordinary health and hook preview
   agree; a test proves a runner recorded only under the installation
   is reported alive.
2. The hook preview renders the aggregate, every check that is not
   alive with its remedy, and one alive count; the full line goes to
   the sibling file beside the verdict, written atomically.
3. Every allow-path system message the hook emits goes through report
   bound-message and the same constant as the blocking path; the
   blocking path is unchanged in behaviour.
4. The template-layout allow fixture asserts the bound, the aggregate,
   the absence of alive remedies, and the runner reported alive, and
   would fail if the seeding stopped reaching the emission.
5. Nothing outside the eight files changed.

If nothing material remains, say so; that closes the chain and the fix
lands.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
one-screen-build2b-20260909; if your sandbox cannot run it, compute the
tree by hand as the earlier critics did and say so.

# Gap Rule

Stop adding at a gap that needs a decision no page has made; keep what
is written; report the gap with the resolution you propose. Never
delete written work.
