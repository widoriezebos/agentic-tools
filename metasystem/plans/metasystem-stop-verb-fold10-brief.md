Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 11: the precedence rule proves itself, and two smaller things (chain stopverb-build1)

The orchestrator ran the supervision bed on the round-10 tree outside the
sandbox. arm-again now PASSES, joining seat-survives and status-is-live,
so three of the five acceptance scenarios and seat-refused are green and
the ordering fix landed correctly for the dispatcher. Two scenarios
remain, and one older scenario went red for the first time.

# Facts, from the orchestrator's run

- stop-fence: `mission start` under a closed fence did not print the
  stopped refusal. It printed `mission start refused: supervision did not
  arm:` followed by a full arming report ending `up outcome=advisor
  authority=read-only`. So the launcher tried to arm BEFORE reading the
  fence, and the caller was told about a lease holder instead of being
  told the checkout is stopped. This is the same defect round 10 fixed
  for the dispatcher, in the mission launcher: exactly what section 9's
  precedence paragraph now forbids.
- stop-everything: refused with `log path escapes the repo and /tmp:
  /private/var/folders/...`. The scenario writes its run log under
  `$TMPDIR`, which on this platform resolves outside both permitted
  roots.
- census-lifecycle: red for the first time, having passed on the round-6
  through round-9 trees. Its output ends with a `git status` showing
  untracked `artifacts/`, `bin/` and `brief.md` and the line `nothing
  added to commit but untracked files present`. The orchestrator has
  confirmed `stopfence.Read` never writes, so a read-that-writes is not
  the cause, and is re-running the bed to separate a flake from a
  regression; treat the cause as open.

# Decisions (the orchestrator's; decided, not open)

D42. PRODUCT: `mission start` and `mission resume` read the fence before
they attempt to arm, so a stopped checkout is told it is stopped rather
than told about arming or a lease holder. This is section 9's precedence
rule, the same one round 10 applied to the dispatcher: a gate that fails
only because the checkout is stopped must never be the reason a caller
is given. Add a package test at the seam you change, as round 10 did.

D43. stop-everything writes its run log inside the scratch repository,
which the guard permits, not under `$TMPDIR`.

D44. census-lifecycle: diagnose before touching it. Determine whether
round 10's fence-before-census ordering in
metasystem/scripts/agents/dispatch.sh changed what that scenario leaves
behind, and say so plainly in your return. If round 10 caused it, fix
the cause rather than the assertion. If it is a pre-existing flake, say
that and change nothing. The orchestrator's re-run result will be in the
next brief if you need it; do not weaken the scenario either way.

D45. Nothing else changes.

# Verification

Reported at evidence level ran, with the private caches earlier rounds
used: `scripts/agents/go-gate.sh --fast`; `go test -count=1` over every
package in the boundary; and the five scenarios named with the exact
supervision-bed command, which the orchestrator runs outside your
sandbox.

# Constraints

Wall-clock budget: 120 minutes. Return per the implementer schema with
the cumulative diff boundary listed. Gap rule: stop and report a gap;
never fill it silently.
