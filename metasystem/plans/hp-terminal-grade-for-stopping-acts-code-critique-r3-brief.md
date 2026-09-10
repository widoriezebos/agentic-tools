Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-09

# Review brief: the terminal grade for stopping acts after the fifth round (chain hp-terminal-build1-20260909, reviewing round hp-terminal-build1-20260909-r5)

FINDING IDS: chain-unique, continue the sequence: HPT-08, HPT-09, ...
never F-n and never a reused id. Report `round` as 1 in your return:
it is this job's own round. This is the third and last review round the
tier-3 box allows; if nothing material remains, say so and the chain
lands; a material finding closes only by the human's accept-risk or by
his raising the budget, so make it count.

You are the third critic on this chain. The first found seeded ids
outside the ledger's alphabet and human assertions that supplied a
lineage (both folded). The second found the scenario unable to fail on
Linux (`script -c` without --return, then exit 0) and a headless
assertion that was not headless, plus two notes; the seat-side gate
found the scenario's pseudo-terminal holder dying at once for want of
standard input and its assertion output lost to the pseudo-terminal.
All of it was folded in round three. Dispositions are in the records
directory under misc, hp-terminal-grade-for-stopping-acts-critique-r1-
and -r2-dispositions.md (new, not yet committed). The two brain-stop
scenarios fail on main since d533caf17 and belong to goal
brain-summary-leads-the-stop-display.

Round four (after the seat gate on round three): the proof-grade
holder's stdin was a named pipe, which script(1) on macOS refuses
(XNU FIFOs are socket pairs; tcgetattr answers EOPNOTSUPP, not ENOTTY,
and script exits). Round four feeds the holder from an anonymous pipe
held open by a keeper process (a copied sleep named
proof-grade-input-keeper, 600 seconds, its pid recorded) and reaps the
keeper in the same ownership-checked cleanup as the holder. Attack
that: a keeper that outlives the bed on any exit path; a keeper counted
by the leak check; a cleanup that kills a recycled pid; the Linux
branch losing --return; anything else in the fixture moving.

Round five (the seat gate on round four, three defects proven on the
seat): the goal-cli bed never called harness_fixture_budget_init, so its
first harness_fixture_cap died "fixture cap scale is not initialized";
the scenario's holder, keeper and agent shell were copies of /bin/sleep
and /bin/bash, which macOS kills at exec (rc 137, sandbox on or off), so
the scenario had never run on any round; and the cleanup compared an
empty recorded start with an empty live start, found them equal, and
reported a keeper that never existed as one that would not exit. Round
five inits the budget right after the first source, makes the three
processes `exec -a <name> /bin/sleep|/bin/bash` scripts, guards the
keeper-start read, and lets only a non-empty proven start earn the
ownership check, the TERM and the bounded wait. Attack that: the agent
shell's argv[0] signature not matching what the census classifier reads;
a holder or keeper the cleanup now silently skips while alive; the init
calibrating in every scenario child instead of once; any scenario other
than proof-grades changing behaviour.

THE FINDING THIS REVIEW MUST SETTLE FIRST (seat gate on round five,
proof-grades now runs end to end on the seat, 8 of 9 assertions pass):
the one that fails is `session stop --by Wido` from the scenario's
unenrolled pseudo-terminal, refused "human-reserved; caller classifies
DELEGATE". That refusal is RIGHT: the seat runs under a claude process,
so every process the bed creates, its pseudo-terminals included, has an
agent in its ancestry, and cmd/metasystem/session_stop.go gates on
lease.Classify, which walks the whole ancestry against every adapter
signature. The eight that pass include park, release and unpark from
that SAME shell: goal park/release/unpark at the terminal grade admit
it. internal/humanauthority/authority.go ProveTerminal walks from the
invoker only to its session leader (`current == sessionPID` ends the
walk) and cmd/metasystem/goalsync_mutations.go does not refuse a
DELEGATE-class caller who says --by. Under the enrolled grade this was
safe because the enrollment record pinned one human terminal; the
terminal grade drops the record and keeps the short walk, so any agent
can `script -q /dev/null metasystem goal park --by Wido` and be "a
human at a terminal". The goal's own sentence says "with no agent in
the ancestry"; the build stops looking at the session leader. The
fixture's pass under this seat is that attack succeeding, not a human
being admitted. Confirm or refute this reading against the code; if
confirmed, it is material and names the fix: the terminal-grade walk
must continue past the session leader to the root of the process tree
checking agent signatures (the same-terminal requirement stays bounded
by the session leader), session stop keeps its gate, and the fixture
must expect the allow assertions to be refused AGENT_IN_AUTHORITY_CHAIN
on an agent-descended bed and to pass only on an agent-free one.

Threat model for this round: the scenario still unable to fail on
either platform by any route (status not checked, output not captured,
a supplied lineage, a fixture grant, an assertion whose failure exits
0); the holder still dying, or surviving as a leak; the headless case
still holding a terminal; the adapter-signature loop still removing a
real signature; the check-order restoration in ProveTerminal changing
anything but the name of one refusal; any regression of what rounds one
and two certified (ProveTerminal equals Enroll's walk without the
write; Grade empty on refusal; ValidFor unchanged; the helper on
exactly the four rows; no lineage supplied by any human assertion); a
change outside the declared boundary.

Scope: the computed diff of round hp-terminal-build1-20260909-r5 (the
chain's cumulative diff against its base). Contract: the build brief
and the two fold briefs in the plans directory (not yet committed), the
goal record metasystem/plans/goals/hp-terminal-grade-for-stopping-acts.md,
and the design metasystem/plans/human-proof-fits-the-act-design.md
revision 2.

# Mandate

1. The proof-grades scenario fails when any of its assertions fails, on
   Darwin and on Linux, and its output reaches the bed's log.
2. The holder lives for the scenario and is gone after it; the headless
   case has no controlling terminal; real adapter signatures stay.
3. Nothing certified in the earlier rounds has regressed; nothing
   outside the declared boundary changed.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
hp-terminal-build1-20260909-r5; if your sandbox cannot run it, read the
tree from the review record beside the diff and say so. The orchestrator
runs the full packages, the coverage floors and the goal-cli bed before
your return lands; do not spend your budget on them.

# Gap Rule

Stop adding at a gap that needs a decision no page has made; keep what
is written; report the gap with the resolution you propose. Never
delete written work.
