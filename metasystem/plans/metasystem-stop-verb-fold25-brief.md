Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-08

# Fold brief: round 27 of chain stopverb-build1, the seventh code read

The seventh code read (job stopverb-crit11) returned four material
findings and one presentation note on the round-26 tree. All five are
accepted. Its register landed on trunk after this worktree was cut, so
the findings are restated in full below rather than cited.

This round is written as three class fixes and one line, not as five
patches. The reason is in the read itself: three of its four findings
are rules already in the design, unkept in readers nobody enumerated,
and one of those is the third instance of the same fault in the same
line builder. Patching the instance has now failed twice in this chain.

The specification is metasystem/plans/metasystem-stop-verb-design.md.
Sections 16, 17 and 18 are the newest and win where they contradict
earlier text. Section 18.3 is the governing rule for most of this
round: the phase distinction governs every reader, and no reader may
present a closed fence alone as proof of a completed shutdown. The goal
record metasystem/plans/goals/metasystem-stop-verb.md carries Wido's
durability requirement in his own words. Decisions D1 to D114 are the
orchestrator's from the earlier briefs and stand unless one is wrong.

# The corrections

## D115 - SVC11-02 - the three verbs share one scope resolution

Proven by execution: with the engine binary outside an installation
layout, status works, arm classifies the caller and prints the correct
two-line refusal, and stop fails before it classifies anyone, before it
reads the fence, and before it can refuse in the specified shape. The
one line it prints ends "pass --metasystem-root", and metasystem stop
has no such flag.

Two rules are broken and both must hold after this round. The three
verbs resolve their scope one way, which after round 26 means the
installation the caller named. And no refusal names a flag the verb
that produced it does not accept, which is section 15.2.

Test all three verbs in a layout where the binary is not at
<root>/bin/metasystem: each either works or refuses in its own
specified shape, and no refusal names a flag its own verb rejects.

## D116 - SVC11-01 - the run line is derived from what happened

This is the third instance of one fault. SVC10-01 was the suite phrase
appended on any terminal status; SVC10-02 was the hardcoded
ended-unknown; this one prints "launch failed (stopped)" over a record
that says green, because the builder decides from the status the
inventory saw and tests "was it launching" before "was it already
gone".

Do not add a fourth branch. Make the run line derive from the outcome
the stop produced and the record as it stands after the stop, so no
printed run line can contradict the record. Where the outcome does not
carry enough to say what happened, carry more in the outcome rather
than reading stale inventory state.

Then enumerate every branch of that builder in the return, and prove
each one with a test that fixes the record state and asserts the line.
The read found this class three times because no test ever pinned the
whole grammar of that one line.

## D117 - SVC11-03 and SVC11-04 - every reader is enumerated and made phase-aware

Section 18.3 says the phase distinction governs the human-readable
fence messages in up, health and every creation refusal, and that no
reader presents a closed fence alone as proof of a completed shutdown.
Two more readers do not:

- metasystem/scripts/agents/supervision-hook.sh, whose stop-event
  block matches the stopped prefix of the up aggregate line and then
  prints a fixed sentence naming arm, discarding the remedy field on
  that same line, which the engine already set to the stop command for
  an incomplete or unfinished stop.
- internal/goal/turnverdict.go, whose closed-fence branch returns a
  fixed display string and binds the record it read to the blank
  identifier, discarding the phase and the unresolved list. The
  phase-sensitive helpers the up path and health use are in the same
  package and are not called.

Fix both, and then close the class: enumerate every reader of the fence
record in the tree, name them all in the return, and state for each one
whether it now describes the phase or why it does not need to. A reader
that only asks whether creation is allowed does not need the phase; a
reader that tells a person what the state is does. The hook is shell and
the turn verdict is Go, so the shared thing is the description, not
necessarily one function.

## D118 - SVC11-N1 - the up stopped line states its remedy once

The round-26 fold builds the component detail as the phase description
plus the stop command, while the aggregate line of the same output
already carries that command in its remedy field, so a person reads the
same instruction twice. Keep the phase description and the count in the
detail; leave the command to the remedy field.

# Scope

Nothing outside these corrections and their tests. No test is weakened
or deleted to make a correction pass. No new design text: every
correction above is the design as written, applied to code that drifted
from it, except D118 which removes a duplication the fold introduced.

# Verification

Run, and report each result in the return:

- `scripts/agents/go-gate.sh --fast`
- `go test -count=1 -timeout 40m ./internal/stoptransition/ ./internal/run/ ./internal/up/ ./internal/supervise/ ./internal/goal/ ./cmd/metasystem/`
- `bash -n scripts/agents/supervision-hook.sh`

Two failures in that set are pre-existing and not yours:
TestGoalTemporaryAuthorityRefusesPastAndBeyondHorizon in cmd/metasystem
fails because the temporary-authority horizon passed on 2026-09-06, and
goal fixture-review-by-date-rolls-over owns it. A process-group
visibility probe in the same package cannot observe its own spawned
group inside a delegate sandbox; it passes outside one. Report both as
pre-existing and touch neither.

The orchestrator runs the supervision, dispatch, goal-cli, mission and
suite-progress beds outside your sandbox. Do not attempt them, and do
not treat a sandbox failure as evidence about the diff.

Gap rule: stop and report a gap; never fill it silently.
