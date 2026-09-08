# metasystem stop: seventh code critique (chain stopverb-build1, round 26)

Critic stopverb-crit11 (code-critic, Opus 5) on reviewed tree 213aa359c171f749166e3788c17860bc18bde6f1. Four material findings, two high and two medium, plus one presentation note. Two were proven by execution in a scratch copy of the tree.

Every finding is about what a person is told. None is about whether the fence holds. That split matters and is discussed at the end.

## SVC11-01 - high - material=True

CLAIM: The stop report tells a person that a run's launch failed and that this stop ended it, when neither is true. A run record whose status is still "launching" is inventoried without a liveness probe, and by the time the run step reaches it the run may have finished on its own: the repository watcher assesses every non-terminal run on each pass and is not stopped until two families later, and a creator that outlived the ten-second creation-claim wait can still bind and finish its work. The line builder decides what to print from the status the record had at inventory time rather than from what the stop did or the record it wrote, and its test for "was it launching" comes before its test for "was it already gone".

EVIDENCE: Proven by execution twice. A run inventoried as launching that completes and exits zero before the run step: stop prints "launch failed (stopped)" and the record it leaves reads green. A run inventoried as launching whose creator binds it to a real process group before the run step: stop prints the same words and the record reads ended-unknown. In both cases the outcome stop actually produced was already-gone or a genuine conclusion.

## SVC11-02 - high - material=True

CLAIM: A person at a terminal can be locked out of stop in a layout where arm and status both work, and the refusal names a flag stop rejects. The three verbs are supposed to share one scope resolution. After the round-26 fold they no longer do: arm asks the caller classifier to use the installation the person named, while stop still derives that root from wherever the running engine binary sits on disk. When the binary is not laid out as <root>/bin/metasystem, stop fails before it classifies anyone, before it reads the fence and before it can refuse in the two-line shape the design requires, and the single line it prints ends "pass --metasystem-root", which metasystem stop has no such flag for.

EVIDENCE: Proven by execution. The critic built the engine from the reviewed tree into a directory that is not an installation layout, made a scratch clone with its own bin/metasystem, and ran all three verbs with the same arguments. status printed the checkout and "nothing is running" with exit 0. arm classified the caller and printed the correct two-line refusal with exit 1. stop printed one line, "cannot resolve the installed engine", and named a flag it does not accept.

## SVC11-03 - medium - material=True

CLAIM: The notice the seat shows a person when their session ends contradicts the engine output it just read, in exactly the case the round-26 fold was written for. The supervision hook runs up, matches the aggregate line's stopped prefix, and prints a fixed sentence telling the person to start again with arm. It discards the remedy field on that same line, which the engine sets to the stop command whenever the last stop was incomplete or unfinished. So after a stop that left survivors, the one line a person actually reads at the seat sends them to arm, which refuses while any survivor stands.

EVIDENCE: scripts/agents/supervision-hook.sh, the stop-event block, sets the notice from a fixed string on the stopped prefix. internal/up/up.go, which round 26 changed, sets the aggregate remedy from the phase-sensitive helper, so the line the hook parsed already carried the stop command.

## SVC11-04 - medium - material=True

CLAIM: The turn verdict shown at the end of a turn says the checkout is intentionally quiet until arm is run, for every closed fence, including one whose stop never finished and one that left survivors alive. There is no phase branch and no mention of the unresolved entries.

EVIDENCE: internal/goal/turnverdict.go: the closed-fence branch returns a fixed display string and binds the record it read to the blank identifier, discarding its phase and unresolved list. The phase-sensitive helpers the up path and health both use sit in the same fence package and are not called.

## SVC11-N1 - low - material=False

CLAIM: The up path's stopped line states its remedy twice: the round-26 fold builds the component detail as the description plus the stop command, while the aggregate line of the same output already carries that command in its remedy field. Redundant, not misleading.

EVIDENCE: internal/up/up.go, the closed-fence branch, composes the detail from the description helper plus the command helper and sets the remedy from the same command helper; the renderer prints both lines.

## Coordinator's reading (m1b, 2026-09-08)

All four material findings accepted, and the note accepted too because
one line closes it. Round 27 folds them under decisions D115 to D118.

The shape of this read is the reason round 27 is written as three class
fixes rather than four patches. Every finding is a reader that decides
what to say from something other than what happened:

- SVC11-01 is the third instance of the run line deciding from the
  status the inventory saw rather than from the outcome and the record
  the stop wrote. SVC10-01 and SVC10-02 were the first two. So the
  correction is not another branch: the run line is derived from the
  outcome and the record after the stop, with every branch of that
  builder proven against a record state.
- SVC11-03 and SVC11-04 are the same rule unkept in two more readers.
  Section 18.3 says the phase distinction governs every reader, and
  nobody ever enumerated them. So the correction enumerates every reader
  of the fence record in the tree, names them in the return, and makes
  each one use the shared phase-aware description.
- SVC11-02 is the one true regression of the fold before it: round 26
  gave arm the installation the person named and left stop deriving its
  root from the binary's own location, so the verbs that were specified
  to share one scope resolution stopped sharing it.

What this read did NOT find is worth recording as plainly as what it
did. Nothing in it touches whether the fence holds, whether a creator
can slip past it, whether a stop leaves a process alive, or whether the
durable record and the survivor list are correct. Those are the
properties Wido asked for, and they have now survived three consecutive
reads without a finding. The defects that remain are in the sentences
the system says about itself, which is a real class and is why the
design has a section about it, but it is a different class from the one
that could wedge a checkout.
