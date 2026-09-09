Working Mode: design
Orchestrator Identity: m1 (lineage main-1788940932-18533-7fa6c2, dispatch delegate under goal human-proof-fits-the-act)
Date: 2026-09-09

# Design brief: the proof a human act demands fits what the act enables

Deliverable: human-proof-fits-the-act-design.md, a NEW file you create
in the metasystem plans directory, revision 1. Write the design only. No
code, no fixtures, no other file.

## Authority to author this design

The composed role prompt's mode table says the design itself is never
delegated. For design AUTHORING that line is superseded by Wido's
standing word in metasystem/memory/rulings.md (R-86-m1b and R-87-m1c,
2026-09-07: design authoring is a dispatched delegate round), and by his
word this morning, 2026-09-09, verbatim: "We have Fable back, so I want
you to switch back to Claude using Fable 5.1 for designs now". The lane
structure of R-25 is unchanged; the model on the design lane is
claude-fable-5-1 again, which is you. Writing this page and settling the
decisions in it is your task, not a conflict to stop on. What remains
the orchestrator's: the dispositions of the critique that follows,
certification, the ledger, and the receipt.

## The problem, and it is live

The contract is metasystem/plans/goals/human-proof-fits-the-act.md. Read
it first; its DONE definition is the specification and this brief adds
today's evidence, not scope.

A human act is proven by walking the invoking process's ancestry: it
refuses any agent signature and must arrive at the exact terminal
enrolled in that checkout (Prove and Enroll in
metasystem/internal/humanauthority/authority.go; the enrollment record
is the file human-terminal.json in the authority directory under the
checkout's agents artifacts, which binds one terminal id, one terminal
process and one session leader). The
mechanism is right and stays: an agent with a shell in the checkout
would otherwise approve its own goals and raise its own budgets by
typing --by. Its shape is what costs.

The goal record carries four defects seen on 2026-09-07. This morning
added three more, on m1, in one hour, with Wido at the keyboard:

5. **A stopped goal wedges its machine.** actionable-metrics was
   breach-stopped on 2026-09-08 by a session that later died. Its claim
   stayed, and the claim still counted against the machine quota of one
   claim, so m1 could claim nothing. Clearing it needed goal resume and
   then goal release, both human acts, so an agent seat with work to do
   and a human present could not start until the human typed at the
   enrolled terminal. The wedge itself (a batch that no longer binds)
   belongs to goal stop-batch-strands-a-resumable-goal; what belongs
   HERE is that release, which only halts a claim, demanded the full
   enrolled walk, and that a stopped claim held the slot at all.
6. **resume makes the human retype the ledger.** goal resume requires
   the complete five-member budget tuple on the command line, and then
   refuses unless it equals the standing approved budget byte for byte
   ("resume cannot change the human-approved budget"). The ledger
   already holds those five values; the human is made to copy them
   from the goal file into flags to prove nothing. The refusal for a
   missing tuple names the five flags but not the values, so the human
   cannot succeed without reading the goal file first.
7. **The enrolled terminal was alive and the refusal still did not say
   which one.** The enrollment bound the zsh inside tmux session
   "human" (enrolled 2026-09-06, alive three days later). Wido typed in
   another tab, was refused TERMINAL_NOT_REACHED, and the refusal did
   not name the enrolled terminal, did not say a different tab was the
   cause, and did not say that goal enroll-terminal from the current tab
   would work with no prior enrollment. The orchestrator had to read
   the enrollment file and ps to tell him which tmux session to attach.

The population of human-only acts today, from the engine's own help
text: goal approve, unapprove, set-budget, accept-risk, set-obligation,
resume, classify-sweep, set-priority, enroll-terminal, migrate, repair,
steal, a release of another lineage's claim, reconcile with --by; brain
declare and withdraw; session stop; the process verbs arm and stop. The
refusal codes the authority layer owns are in
metasystem/internal/refusal/register.go lines 33 to 39 and 43 to 45
(AGENT_IN_AUTHORITY_CHAIN, ANCESTRY_CHANGED, ANCESTRY_CYCLE,
ANCESTRY_UNREADABLE, ARGV_UNREADABLE, PROCESS_REUSED,
TERMINAL_NOT_REACHED, APPROVAL_REQUIRED, APPROVAL_EXPIRED,
RELAY_AFTER_ENROLLMENT), with their sites in authority.go and
metasystem/internal/goal/approval.go.

## What the design must settle

The goal's DONE definition names the shape. Settle it verb by verb.

1. **The grading table.** For every human-only act above, say which
   proof it takes and why it sits there: the WEAK proof (a human at any
   live terminal of this host: no agent signature anywhere in the
   ancestry, the walk reaching a terminal, enrolled or not) or the FULL
   proof (the enrolled-terminal walk, unchanged). The goal's rule: acts
   that only stop, park or release work take the weak proof; acts that
   grant or widen authority keep the full walk. Place resume: it
   reopens admission under a standing approval, so say whether that is
   "widening" or "continuing" and why. Place release of a foreign
   lineage's claim, steal, and the process verbs arm and stop. Say what
   must remain impossible: an agent, with or without --by, passing
   either proof.
2. **The enrollment shape.** Either several terminals hold an enrollment
   at once (say how many, how one is retired, what the record looks
   like), or one terminal holds it and every TERMINAL_NOT_REACHED
   refusal states the enrolled terminal by a name a human recognises
   (the tmux session name when there is one, the tty otherwise), states
   that the current terminal is not it, and prints the one enroll
   command that would make it so. Pick one and say why.
3. **The refusal text for each outcome code** the authority layer owns.
   Every refusal prints exactly one command that would have succeeded
   from where the human sits, or says in words that none would and why.
   Write the texts. Goal human-goal-verbs-forgiving owns the same rule
   for every goal verb's OTHER refusals; do not redesign that, cite it,
   and keep this page to the authority layer's codes.
4. **resume's budget.** Say how resume takes the standing approved
   budget from the ledger instead of the command line, and what a human
   who wants a DIFFERENT budget types instead (set-budget already exists
   for that). If you keep a way to pass the tuple explicitly, the
   refusal on a mismatch prints the standing values.
5. **A stopped goal and the machine's claim slot.** Say whether a
   breach-stopped goal's claim counts against the machine quota, and
   why. If it does not, say what still prevents a machine from holding
   two live claims by stopping one. If it does, say how a machine
   recovers with the weak proof alone.
6. **The relayed word.** The goal's fourth defect: a human present in an
   agent session gives an instruction and the seat can record nothing.
   Say what happens to a human word spoken to a seat: a graded relay
   that records the word with its provenance stated and unverified, or
   an explicit statement that no such path exists and the human always
   types it, so the seat stops trying. --temporary-human-word with
   --review-by is retired in fact (goal fixture-review-by-date-rolls-over
   records why); say whether it comes back in a form that binds.

## Prove it with a canary, not a battery

For every fixture you name, also name the SMALLEST run that proves it:
one Go test, one bed scenario by name, with a ceiling. The acceptance
here is naturally four canaries: an agent shell refused for both an
approve and a stop; a human shell at an unenrolled terminal allowed to
stop and refused for an approve with the enroll command printed; that
printed command running and then the approve succeeding; a dead
enrolled terminal recovered by one enroll from a fresh terminal. Build
the fixture list around those and their twins, not a full battery. The
human-authority package has a tree reader for fixtures
(metasystem/internal/humanauthority/authority_test.go); name the shapes
it needs.

## Constraints

Stay inside the intent. Do not weaken what an agent may do: the design
is judged first on what remains impossible. Do not redesign the goal
ledger, the claim quota's purpose, or what approval means. Where the
intent is ambiguous, say so and give one recommendation rather than
listing options. This design gets one revision and one independent
critique on the design-critic lane, then the build proceeds behind the
fixtures you name; if you believe a second revision is needed, say so
plainly in the return instead of writing one.

Stop adding at a gap that needs a decision no page has made; keep what
is written; report the gap with the resolution you propose. Never
delete written work.
