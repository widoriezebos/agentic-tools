# metasystem stop: eighth code critique (chain stopverb-build1, round 27)

Critic stopverb-crit12 (code-critic, Opus 5) on reviewed tree b44a3a38c4847177d9157f9c53152b221a33d149. Three material findings, one high, one medium, one low, and two notes. Two were proven by execution.

## SVC12-01 - high - material=True

CLAIM: The one command that can open a stopped checkout without being called arm gets its caller classified against the wrong directory, and in this repository's own layout that leaves the classifier with no way to recognise an agent at all. When anything runs mission start or mission resume on a stopped checkout, the code decides whether to open the fence by classifying the caller, and it asks the classifier to read its runtime adapters out of the state root, the top of the checkout, instead of the installation directory that holds the engine. Where the metasystem lives in a subdirectory, which is how it lives in this repository, the adapter directory is not there, so the classifier has zero agent-runtime signatures to match against.

EVIDENCE: Proven by execution. cmd/metasystem/process_verbs.go, missionFenceBeforeArm, calls the classifier with the state root as the installation directory and opens the fence on a human answer. Running the classifier both ways against this repository from a scratch copy of the reviewed tree gives different evidence: with the repository top as the installation, no runtime signature is available.

## SVC12-02 - medium - material=True

CLAIM: The shared sentence every creator prints when it discovers it was stopped has no guard for a fence that is not closed, so after a human arms the checkout it tells the person the last stop is incomplete and sends them to run stop again. A creator reads the fence, publishes its record, then reads the fence a second time to learn whether a stop closed underneath it. That second read treats two different things the same way: the fence being closed, and the fence being open at a newer generation, which is exactly what a creator sees when a stop completed and a human then armed the checkout.

EVIDENCE: Proven by execution. Feeding an open, armed record to the shared helpers in internal/stopfence/fence.go returns the description "stop incomplete for /checkout ... 0 unresolved entries from the last stop", the stop command as the remedy, and the incomplete health prefix.

## SVC12-03 - low - material=True

CLAIM: Five of the seven refusals that stop and arm can print name a retry command that drops the installation directory the caller had to supply, so in the layout that needs that flag the printed command does not work. Round 27 added a helper that rebuilds the retry command with the caller's explicit installation flag and used it in exactly one place in each verb, the human-terminal gate. Every other refusal in the same file still composes its second line from the repository path alone.

EVIDENCE: cmd/metasystem/process_verbs.go: the retry-command helper appends the installation flag when the caller supplied one and is called only from the two human-terminal gates. The crash-step refusal, the stop and arm refusal second lines, the fleet-form refusal and the scope refusal all build their own.

## SVC12-N1 - low - material=False

CLAIM: The exported health prefix helper in the fence package is no longer called by anything except its own test. Health re-implements the same three-way phase choice inline, because it also treats a record that claims a completed stop while still listing unresolved entries as incomplete, and the shared helper does not. Both spellings agree today, but the round-27 return presents the fence package as the one shared description every reader uses, and for health that is no longer true.

EVIDENCE: The prefix helper in internal/stopfence/fence.go is referenced only there and in its own test. internal/steward/health.go selects its prefix inline, adding the unresolved-count case the shared helper lacks.

## SVC12-N2 - low - material=False

CLAIM: When a non-human caller is refused by mission start or resume on a stopped checkout, the command the refusal tells them to type omits the root flag the verb requires, so typing it verbatim exits with a usage error. Recorded rather than raised because the design specifies that exact text: the deviation is between the design and the verb's own argument parser.

EVIDENCE: The refusal prints the mission command without a root flag, matching the design's reader table, while cmd/metasystem/missionrunner_verbs.go requires a non-empty root and returns the usage exit code without it.

## Coordinator's reading (m1b, 2026-09-08)

All three material findings accepted, and both notes accepted as work
because each is one line and each closes a drift the round-27 return
itself claimed was closed. Round 28 folds them under D119 to D123.

SVC12-01 matters more than its neighbours and belongs to the class that
Wido's requirement lives in. Every other finding of the last three reads
has been about sentences; this one is a second path that opens the fence,
and it hands the classifier evidence that cannot identify an agent. The
fix is the same shape as D110 and D115: one resolution of the
installation, used by every classifier call site, enumerated in the
return rather than fixed where it was found.

SVC12-02 and SVC12-03 are both the round-27 class fixes applied
incompletely. The shared description was given no guard against being
handed a record that is not closed, and the retry-command helper was
used in two places out of seven. So the round-28 rule is: a helper
introduced to close a class refuses the inputs it cannot describe, and
its call sites are enumerated in the return.

The trend across the last four reads is worth stating plainly: six, then
five, then four, then three, with the wedge class appearing once in that
run and the rest being what the system says about itself. That is
convergence, not a treadmill, but it is not finished while a second
fence-opening path classifies its caller against a directory that holds
no adapters.
