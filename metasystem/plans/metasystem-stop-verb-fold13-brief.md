Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 15: three fixture bugs in the folded tree (chain stopverb-build1)

The orchestrator ran everything on the round-14 tree outside the
sandbox. The fold is good: the eleven-package matrix is green, the
dispatch bed and the suite-progress bed are green, and in the
supervision bed stop-everything, seat-survives, status-is-live,
arm-again and the newly built arm-refuses-survivor all pass, along with
every pre-existing scenario but `rearm-launch-fails`, which is red on
main and owned elsewhere. So four of the six critique findings are
proven fixed by execution, including the claim-directory wedge and the
unreadable-record repair.

Three scenarios fail, all on their own bugs rather than on the product.

# Facts, from the orchestrator's runs

- stop-fence: it drives `mission start` under a closed fence and expects
  the stopped refusal, but gets a full arming report ending `up outcome=
  advisor authority=read-only`. That is now CORRECT product behaviour:
  decision D48 makes a HUMAN caller open the fence and arm, and the
  bed's own driver is classified human. The scenario's intent, in the
  design's words, is `mission start` "as the bed's agent-shaped
  identity".
- wrong-terminal (goal bed): two differences. It expects the caller
  class `UNTRUSTED` and the classifier says `DELEGATE`. And its expected
  checkout path is unresolved (`/var/folders/...` against the engine's
  `/private/var/folders/...`), the same platform symlink the supervision
  bed already resolves.
- mission-stop (mission bed): the scenario aborts at
  `scripts/agents/mission-fixtures.sh` line 505 with `mission: unbound
  variable`, so it has never run.

# Decisions (the orchestrator's; decided, not open)

D56. stop-fence drives `mission start` with an agent-shaped caller, using
the synthetic agent ancestor this bed already uses elsewhere
(`METASYSTEM_FAKE_AGENT_ANCESTOR_PID`), and keeps asserting the two-line
stopped refusal for it. The human path is mission-stop's to prove, not
this scenario's.

D57. wrong-terminal expects the class the classifier actually reports for
that caller, and resolves its expected checkout path the way the other
beds do (`cd … && pwd -P`) before composing the expected refusal. If the
class it reports is not the class the design's table names for that
caller shape, say so as a finding instead of adjusting the expectation.

D58. mission-stop's shell bug is fixed and the scenario actually runs its
three cases: the cooperating held host, the ignore-TERM host, and the
human resume through a closed fence under fixture human authority.

D59. Nothing else changes. If a scenario then fails on the product rather
than on itself, stop and name it; that is a finding, not a fixture bug.

# Verification

Reported at evidence level ran, with the private caches earlier rounds
used: `scripts/agents/go-gate.sh --fast` and `go test -count=1` over the
boundary. The orchestrator runs the four beds outside your sandbox and
will report; name what you expect from each.

# Constraints

Wall-clock budget: 90 minutes. Return per the implementer schema with
the cumulative diff boundary listed. Gap rule: stop and report a gap;
never fill it silently.
