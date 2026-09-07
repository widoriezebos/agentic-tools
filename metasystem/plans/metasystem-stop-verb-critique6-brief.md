Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Review brief: the first independent critique of the stop verb (chain stopverb-build1, final work round stopverb-build1-r24)

FINDING IDS: chain-unique, SVC-01, SVC-02, ... never F-n.

Why this review exists: this is the SIXTH independent read of this
chain, now twenty-four build rounds old, and the SECOND after a fold. The first (stopverb-crit4, whose
register is records/misc/metasystem-stop-critique-r4.md) found six
material items on a tree whose tests were all green; all six were
accepted and folded, the design gained sections 14.1 to 14.6, and two
scenarios the orchestrator had cut were brought back because they would
have caught two of the findings by execution. The second read (stopverb-crit5, register
records/misc/metasystem-stop-critique-r5.md) then found three more
material items plus one note the orchestrator accepted anyway, all on a
tree whose whole acceptance was green, and the design gained section 15.
Round 19 folded those. Your job is the same as both: read for judgment,
not for green. Two reads have each found real defects in a green tree;
assume there is a third thing rather than that the well is dry. Chain completion under DESIGN-BEARING reach
requires a fresh-context critic whose reviewed job is the final work
round, and Wido's instruction on this goal is explicit: nothing lands
before it is finished, tested and in order, and this change is too
important to rush. There is budget behind you. If you find something
material, say it plainly; a fold round is expected, not resented.

What this is: `metasystem stop`, `status` and `arm` for one checkout,
slice 1 of the design. The change puts a process-creation fence in front
of every path in the system that starts anything, so its blast radius is
the metasystem's ability to work at all: a fence that misjudges its own
record wedges a checkout, and a fence that fails open lets a stopped
machine resurrect itself.

Specification: metasystem/plans/metasystem-stop-verb-design.md, revision
3 with its section 13 addendum, which wins wherever it contradicts an
earlier section. Its critique ladder is closed
(metasystem/records/misc/metasystem-stop-critique-r1.md, -r2.md, -r3.md).
The goal record is metasystem/plans/goals/metasystem-stop-verb.md and
carries Wido's binding durability requirement in his own words. The
build contract is the brief series
metasystem/plans/metasystem-stop-verb-build-brief.md and the fold briefs
that follow it, whose decisions D1 to D47 are the orchestrator's and are
not yours to relitigate unless one of them is wrong.

The orchestrator persisted the diff and the reviewed tree hash at
metasystem/artifacts/agents/stopverb-build1/rounds/24/review.json (the
diff sits beside it); take reviewedTree from that record. The
orchestrator's runs are listed at the end.

# Mandate

1. **The fence and its durability.** Wido's requirement: after a stop,
   nothing that checkout runs comes back on its own, and only a human arm
   clears it. Is that true of the built code, including a crashed stop, a
   fence record that is unreadable or from a future schema, and a
   concurrent arm? Does anything delete or rewrite the record outside arm
   and stop?
2. **The creation-claim handshake.** Section 13.1 claims completeness: a
   creator either publishes before stop's last pass and is stopped, or
   its own second read sees the closed fence and ends what it started. Is
   that argument sound against the real seams, and are the tests
   deterministic rather than timing-dependent? Name any creator that can
   still leave a process behind.
3. **The order and the deadlines.** Suites before the runs that host
   them; the owner's published teardown ceiling instead of a fixed wait;
   the late-arrivals pass. Does the built order preserve the evidence the
   report reads, and can a slow but cooperating component be force-killed?
4. **Honesty.** Every line comes from an outcome; NOT STOPPED sets exit 1
   and lists the survivor; the escalated registry row is appended only
   when the caller force-killed the owner; the stopped state names its
   actor. Can any line claim more than the code did? The precedence rule
   is section 9's: a gate that fails only because the checkout is
   stopped must never be the reason a caller is given.
5. **The readers.** Enumerate the creation paths in the tree and check
   each against the reader table. A path that creates without reading the
   fence is the finding this review exists to catch.
6. **Authority and the seat.** stop and arm are human acts at a terminal,
   status is open; the seat's own session, announcement and lease survive
   a stop, and its Stop hook cannot resurrect anything.
7. **Scope.** Nothing outside the declared boundary; no test weakened; the
   deferrals to slice 1b (escalation variants, the held fake host) and
   slice 2 (the fleet form, one shared enumeration) are honest rather
   than convenient.

# Constraints

Wall-clock budget: 60 minutes. Return per the code-critic schema with
the reviewedTree from the persisted round-12 review record. Gap rule:
stop and report a gap; never fill it silently. Your sandbox cannot run
the fixture beds or five of the packages; do not treat that as evidence
about the diff, and do not weaken anything to make it runnable.

# Orchestrator runs

On the reviewed tree (bc3b3be2966d7b50c26c0f3b6287f9023d0760a5, which the round-19 review record names; the
worktree is based on main 58f20a94), from this Mac, outside any delegate
sandbox, evidence level ran:

- `go test -count=1` GREEN for all eleven packages of the boundary,
  including the five the delegate sandbox can never run: stopfence,
  stoptransition, up, supervise, steward, run, proofrun, dispatch,
  missionrunner, goal and cmd/metasystem.
- `scripts/agents/supervision-fixtures.sh`: every scenario passed,
  including all five of slice 1's acceptance set (stop-everything,
  seat-survives, status-is-live, stop-fence, arm-again), except
  `rearm-launch-fails`, which is red on main and owned by another goal.
- `scripts/agents/dispatch-fixtures.sh` exit 0, which includes the
  seat-refused scenario; `scripts/agents/goal-cli-fixtures.sh` exit 0;
  `scripts/agents/mission-fixtures.sh` exit 0;
  `scripts/agents/suite-progress-fixtures.sh` exit 0.
- `git apply --check --directory=metasystem` of the round-12 diff against
  current main succeeded; the diff is 62 files.
- The builder ran `scripts/agents/go-gate.sh --fast` green on the same
  tree (formatting, vet, staticcheck, refusal register, build).

So the acceptance is proven and the gate is clean. What is NOT proven is
anything about the code's judgment, which is what you are for.
