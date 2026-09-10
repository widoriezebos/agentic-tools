Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-09

# Review brief: the terminal grade for stopping acts after the eleventh round (chain hp-terminal-build1-20260909, reviewing round hp-terminal-build1-20260909-r11)

FINDING IDS: chain-unique, continue the chain's sequence at HPT-28,
never F-n and never a reused id. Report `round` as 1 in your return:
it is this job's own round. You are the ninth critic on this chain.

Round budget: 1 focused round, then at most one correction and its
re-review (tier 3). R-60-m1's rule: material only if it changes what
gets built and names the artifact.

Why this review exists: the third critic (hp-terminal-crit3-20260909)
confirmed with an executed proof that ProveTerminal in
internal/humanauthority/authority.go matched adapter signatures only
from the invoker to its session leader, so an agent that opened a
pseudo-terminal earned a terminal-grade proof and park, release and
unpark accepted it (HPT-08, critical). It also found the cleanup of the
proof-grades scenario in scripts/agents/goal-cli-fixtures.sh blocking
the bed for 300 seconds on two keeper failure paths (HPT-09), a
headless step that does not establish headlessness on macOS (HPT-10), an
agent-refusal helper accepting either of two refusals (HPT-11), and the
budget init running the binary before the "not built" check (HPT-12).
Round six folds all five; the dispositions are in the records directory
under this goal's name, third critique.

Round seven, from the fourth critic (hp-terminal-crit4-20260909) and the
seat gate: HPT-13 (critical) showed the round-six walk to the root ending
ARGV_UNREADABLE on every macOS host, because process 1 is root-owned
/sbin/launchd with withheld arguments; every real human was refused the
terminal grade and enrollment was broken. Round seven admits a
withheld-arguments process only when root-owned with a system-owned
executable the user cannot have written, records it, and continues to
process 1; it makes the fake process tables model the real root (five
command-package tests failed ANCESTRY_CYCLE on a self-parented fake
root); it adds a live walk of the test's own ancestry; and it folds
HPT-14 (holder orphaned when the readiness allowance expires between
waits). Attack the new rule first: a user-owned image made root-owned
by any route the invoking user controls; a system image that can carry
an agent (an agent launched by launchd as the user is BELOW launchd and
still caught, but say so with evidence); the executable's ownership
read unstably; the live test asserting too little or too much; the
fake roots now realistic but the walk's cycle check weakened.

Round eight, from the fifth critic (hp-terminal-crit5-20260909): HPT-15
(critical) showed round seven still stopping at process 1, now with
ANCESTRY_UNREADABLE, because both platform readers report a parent of
zero as "parent unknown" and the stable read refuses an unreadable
parent before the root-owned-image rule is reached; HPT-16 (high) showed
the fakes shaped as no reader can shape a root and the live test
tolerating the broken outcome. Round eight makes the top of the tree a
known fact (parent none-and-known at the root; unknown reserved for a
kernel refusal), ends the walk at the root, requires a proof's last node
to be the root, shapes the fakes as the readers do, and makes the live
test assert the walk reached the root with no refusal; it also gives
the terminal walk the enrolled walk's parent-continuity check (HPT-18)
and the recorded-holder cleanup its kill (HPT-19). HPT-17 (root agents
behind stock images) is recorded as out of model, no change. Run the
live test verbosely on your host as the fifth critic did and paste its
line; that single executed fact decides this review. Then attack: a
fake root still shaped unlike the readers; "parent none" reachable
below the real root (a process whose parent has exited and been
reparented, a container's pid namespace on Linux); the continuity
check refusing legitimate reparenting; the terminal-missing entry
(no tty under go test) letting the live test skip the walk.

Round nine, from the sixth critic (hp-terminal-crit6-20260909), whose
host and the seat both saw the round-eight walk reach process 1:
HPT-20 (critical) the --arc cascade forms of release, park and unpark
decided on the bare presence of --by with no proof; HPT-21 (medium) a
matched agent was reported only at the top of the tree, so an
unreadable ancestor above it renamed the refusal; HPT-22 (low) one
extra classifier iteration against process 0. Round nine routes the arc
forms through the proof-carrying builder and requireHuman per member
act, adds the three arc forms to the scenario, makes an agent match the
outcome on any later exit, and adds the guard. Attack first: any other
command-line route to park, unpark, release or session stop that
bypasses the proof (other flags, the brain channel, relayed words,
scripts under scripts/agents that call these verbs); a cascade whose
members mix grades; an agent match recorded but the proof still
validating as proven.

Round ten, from the seventh critic (hp-terminal-crit7-20260909), whose
review of round nine found only small things: HPT-23 the refusal
register citing each human-authority code one line above its constant;
HPT-24 a test name contradicting its rewritten body; HPT-25 journal
replay of park, unpark and release refused with the wrong wording since
the replay carries no proof. Round ten fixes the eight cited lines,
renames the test, and names the three verbs as proof-bearing in the
replay switch with the explicit re-run wording. This is the closing
review: confirm the three folds, re-run the live walk on your host,
and unless something material remains say so plainly so the chain
lands.

Round eleven, from the eighth critic (hp-terminal-crit8-20260909):
HPT-26 (high) round ten's journal replay refused every stored park,
unpark and release, stranding a dead machine's own release behind a
human act. Round eleven rebuilds the three through the real verb and
refuses, with the explicit re-run wording, only when the verb's own
human-authority requirement fires on a proof-less replay; tests cover
the healing cases and the three refusals. HPT-27 (unprotected register
citations) is recorded for a later goal. Attack: a replayed request
that carries a stale authority from the journal text; the healing
cases widened beyond the live verb's own conditions; the refusal
closing the entry in a state that blocks later recovery. This is the
closing review; unless something material remains, say so plainly so
the chain lands.

Threat model for this review, in order: the walk above the session
leader stopping early (a parent read failing silently, a cycle, pid 1
or launchd treated as a terminal boundary), or matching signatures with
a different set than the walk below; the same-terminal requirement
leaking above the leader (a human's real terminal refused because its
ancestors have no terminal); the proof's recorded nodes not saying what
was checked above the leader; the unit tests proving order or presence
rather than the refusal of an agent ABOVE the leader; session stop's
lease.Classify gate loosened; the scenario deciding "agent-free bed"
from environment variables or a guess rather than the engine's own
walk, or asserting the allows on an agent-descended bed, or accepting
the classification sentence where the ancestry outcome is due; the
keeper, holder or session left alive on any failure path, or a cleanup
wait that is unbounded; the headless child keeping a terminal on
macOS; the built-binary check still after the init; any regression of
rounds one to five; any change outside the declared boundary.

Scope: the computed diff of round hp-terminal-build1-20260909-r11 (the
whole chain's diff against main, since the worktree carries every
round). Contract: the goal record
metasystem/plans/goals/hp-terminal-grade-for-stopping-acts.md, the
design plans/human-proof-fits-the-act-design.md revision 2 (sections
"The two proofs" and the stopping rows of section 1), and the sixth
fold brief in the plans directory under this goal's name.

# Mandate

1. An agent anywhere in the ancestry, above or below the session
   leader, refuses with AGENT_IN_AUTHORITY_CHAIN naming the runtime;
   no agent anywhere passes at the terminal grade; the same-terminal
   check ends at the leader.
2. The proof records the nodes above the leader as it records those
   below.
3. The unit tests pin exactly that, and the existing tests still hold.
4. The proof-grades scenario asserts the refusals everywhere and the
   allows only on an agent-free bed, decided by the engine's walk;
   under an agent-descended bed the allows are refused
   AGENT_IN_AUTHORITY_CHAIN with a real runtime named, and the log says
   the allow path was not proven here.
5. HPT-09 to HPT-12 are folded as their dispositions say.
6. Nothing outside the boundary changed.

If nothing material remains, say so; that closes the chain and it
lands.

# Constraints

Wall-clock budget: 25 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
hp-terminal-build1-20260909-r11; if your sandbox cannot run it, read the
tree from the review record beside the diff and say so. The
orchestrator runs the humanauthority, goal and cmd packages, the
goal-cli bed (agent-descended shape) and the hook suite on the seat
before your return lands.

# Gap Rule

Stop adding at a gap that needs a decision no page has made; keep what
is written; report the gap with the resolution you propose. Never
delete written work.
