Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-09

# Fold round six: the terminal-grade walk must reach the root of the process tree

Follow-up round on chain hp-terminal-build1-20260909; your worktree
carries round five. Round five made the proof-grades scenario run end to
end on the seat for the first time; eight of its nine assertions pass.
The ninth exposed a defect in the grade itself, confirmed by the third
critic (hp-terminal-crit3-20260909; the orchestrator folds its exact
findings into this brief before dispatch, see the end).

## The defect

The goal's sentence: a human at any live terminal of the host, WITH NO
AGENT IN THE ANCESTRY, may stop, park, release, unpark onto a queued
state, and session-stop. `ProveTerminal` in
internal/humanauthority/authority.go walks from the invoker only to its
session leader and stops (`current == sessionPID`). That walk was safe
for the ENROLLED grade because the enrollment record pinned one human
terminal; the terminal grade has no record, so the walk is the whole
proof, and it never looks above the pseudo-terminal. Any agent can
create a pseudo-terminal (`script -q /dev/null metasystem goal park
--by Wido`) and be a human at a terminal. The seat proved it: the bed
runs under a claude process, so the scenario's "unenrolled human
terminal" has an agent in its ancestry, and park, release and unpark
from it were ALLOWED, while `session stop` from the same shell was
refused "caller classifies DELEGATE" by cmd/metasystem/session_stop.go,
whose lease.Classify gate walks the whole ancestry against every
adapter signature. The verbs disagree; session stop is right.

## Mandate

1. In `ProveTerminal`, after the same-terminal walk reaches the session
   leader, continue reading ancestors stably to the root of the process
   tree, checking every one against the installed adapter signatures;
   an agent anywhere above is OutcomeAgent with its runtime named, as
   it is below. The same-terminal requirement stays bounded by the
   session leader (ancestors above it are on no terminal or another
   one, and that is not a refusal). Record the nodes above the leader
   in the proof the way the nodes below are recorded, so the proof
   still says what was checked. `Enroll` uses the same walk and gains
   the same guarantee.
2. Keep session stop's classifier gate as it is.
3. The proof-grades scenario becomes honest about where it runs. The
   refusals it asserts (agent shell, headless caller, unenrolled
   terminal refused for approve) hold everywhere. The allows (park,
   release, unpark onto a queued state, session stop from the unenrolled
   terminal) hold only when the bed itself has no agent in its
   ancestry. Decide that once, at the start of the scenario, with the
   engine's own classification of the bed's process (the same walk the
   verbs use, not a guess from environment variables); on an
   agent-descended bed, each allow assertion must instead be refused
   AGENT_IN_AUTHORITY_CHAIN naming a real runtime, and the scenario log
   says in one line that the allow path was not proven here and where
   it is (an agent-free terminal or the steward's scheduled run). On an
   agent-free bed the allows must succeed exactly as written today.
4. Unit tests in internal/humanauthority cover the new reach of the
   walk: an agent above the session leader refuses; no agent anywhere
   passes; the same-terminal check still ends at the leader.
5. Nothing else changes.

## Proof

Your sandbox cannot run the process-enumerating bed; run the
humanauthority and goal packages and `bash -n`, and say so. The
orchestrator runs the goal-cli bed (expecting the agent-descended shape
of the scenario) and the hook suite on the seat. Report the round as
your own.

## Constraints

Wall-clock budget: 25 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.

## Third critic's findings folded here

hp-terminal-crit3-20260909 reviewed round five (tree
4f71dfc121c7091d593af4cddd2b25e3a0205eac); dispositions are in the
records directory under this goal's name, third critique. Five findings,
all accepted, all yours this round; HPT-08 is mandate items 1 to 4.

HPT-09 (medium): the two keeper start-up failure exits ("did not start
within Ns", "is not alive with a proven start time") run cleanup before
the session start time was recorded. The session block then reads an
empty recorded start against a live start as an ownership mismatch,
prints, sets status, kills nothing, and the unconditional `wait` on the
session blocks the bed for the holder's full 300 seconds (the holder
ignores HUP on purpose). Fix: an empty recorded session start means
never proven, like the keeper and holder blocks; on every failure path
that has a live pipeline, terminate the session and the keeper by their
pid-file values before waiting; bound that wait by the
mission-process-wait cap the other blocks use.

HPT-10 (low): the headless step checks headlessness but does not
establish it: nohup keeps the bed's controlling terminal on macOS, so a
bed run by hand from a terminal window fails with the wrong sentence;
setsid without --wait forks on Linux when the caller is a process-group
leader and returns the parent's zero. Fix: on both platforms the child
creates its own session through perl (`perl -MPOSIX -e 'POSIX::setsid()
or die; exec @ARGV' -- /bin/bash <script>`), keeping the in-child
terminal check as it is.

HPT-11 (low): agent_refuses accepts either AGENT_IN_AUTHORITY_CHAIN or
"caller classifies DELEGATE", so three of the four agent assertions can
pass on the classification gate rather than on the walk this goal
builds. Fix: each act asserts its exact refusal: the ancestry outcome
for park, release and unpark; the classification sentence for session
stop.

HPT-12 (low): the budget init now shells out to bin/metasystem before
the bed's own "bin/metasystem is not built" check. Fix: move that check
above the init.

Mandate item 5 ("nothing else changes") reads with these folded in.
