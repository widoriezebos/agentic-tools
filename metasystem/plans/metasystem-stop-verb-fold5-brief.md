Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 6: the three defects, the fleet unblock, and the five acceptance scenarios (chain stopverb-build1)

Round 5 raised four gaps; all four are answered below. Three are
implementation defects it found honestly and this round fixes. The
fourth, the held fake host, is genuinely unspecified and moves out of
this slice with the scenarios that need it. The specification stays
metasystem/plans/metasystem-stop-verb-design.md, section 13 winning over
earlier sections; the boundary stays what rounds 2, 3 and 5 used.

# Decisions (the orchestrator's; decided, not open)

D20. THE SCOPE CUT, the orchestrator's own call. This chain's acceptance
is the human-visible contract of the goal record: one word stops
everything, status prints the same list without acting, arm starts it
again, a second stop says nothing is running, and a delegate cannot
stop the metasystem. That is five scenarios plus the seat-refused
scenario round 5 already passed: stop-everything, seat-survives,
status-is-live, stop-fence, arm-again. Build exactly those five. The
escalation and per-family variants (arm-refuses-survivor, mission-stop
and its variants, proof-run-stop and its variants, slow-owner,
crash-recovery, remote-job, ignored-signal, wrong-terminal) and the held
fake host they need move to a follow-up slice the orchestrator opens;
the design's section 10 stays the full list. Do not build them here. If
the five are green and time is left, add wrong-terminal and
arm-refuses-survivor, which need no new process seam, and stop.

D21. The narrator line, the defect round 5 named: section 6 requires
`narrator: stopped with the steward runner` beside the steward-runner
line, and the steward family emits only the runner line. Emit both.

D22. The remote-job refusal renders exactly two physical lines: no
newline inside the sentence handed to the refusal renderer. The sentence
names the job and its owning machine; the second line carries the words
that name the cancellation command for that machine, the shape
section 9's other rows use.

D23. `METASYSTEM_STOP_CRASH_AFTER` takes a step number of section 4 as
revision 3 numbers it, never a family index. The value is the step after
which stop exits without cleanup.

D24. THE FLEET UNBLOCK, and it is this chain's to land because this
chain owns the file. In metasystem/scripts/agents/dispatch-fixtures.sh
the approve of the scratch goal `fixture-serving` near line 1795 uses
`--fixture-human-authority` and drops its `--temporary-human-word` and
`--review-by` arguments, with the run-time date helper removed there if
nothing else uses it. That repo is tailored to
`metasystem.runtimes=fake` near line 470, so the fixture-mode predicate
holds and the grant is lawful, exactly as the budget fixture already
does near line 1273. Ruling R-82-m1b records that the relayed-word path
stays retired and names this as its fix; the expired horizon is why
every full-battery landing on the fleet is red, so this one change is
the unblock. Leave metasystem/scripts/agents/channel-fixtures.sh alone:
it is not in the landing battery and another goal owns it. Do not touch
metasystem/internal/governance/types.go.

D25. Source comments describe the application, never this round, a
critique, or a finding id. Fixture-only seams are refused outside a
fixture-mode root.

# Verification

Reported at evidence level ran, with the private caches earlier rounds
used: `scripts/agents/go-gate.sh --fast`; `go test -count=1` over every
package in the boundary; `scripts/agents/dispatch-fixtures.sh` FULLY
GREEN, which D24 now makes possible and which is this round's most
important single result; `scripts/agents/goal-cli-fixtures.sh` green;
and for the supervision bed, the five new scenarios written with the
exact command, since the orchestrator runs that bed outside your sandbox.
State which scenarios fail against the round-5 tree before your change.

# Constraints

Wall-clock budget: 120 minutes. D24 comes first and alone unblocks the
fleet: if nothing else fits, land that green. Then D21 to D23, then the
five scenarios. Return per the implementer schema with the cumulative
diff boundary listed. Gap rule: stop and report a gap; never fill it
silently.
