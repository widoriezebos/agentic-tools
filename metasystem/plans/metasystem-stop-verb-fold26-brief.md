Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-08

# Fold brief: round 28 of chain stopverb-build1, the eighth code read

The eighth code read (job stopverb-crit12) returned three material
findings and two notes on the round-27 tree. All five are accepted and
all five are built in this round. Its register landed on trunk after
this worktree was cut, so the findings are restated in full below rather
than cited.

One rule governs this round, and it comes from what the last read found:
a helper introduced to close a class must refuse the inputs it cannot
describe, and its call sites must be enumerated in the return. Round 27
introduced two such helpers and used one of them in two places out of
seven, and gave the other no guard at all. So this round finishes both,
and proves the finish by enumeration rather than by example.

The specification is metasystem/plans/metasystem-stop-verb-design.md.
Sections 16, 17 and 18 are the newest and win where they contradict
earlier text. The goal record
metasystem/plans/goals/metasystem-stop-verb.md carries Wido's durability
requirement in his own words: if he stops it, the supervisor must die
too instead of being started again. Decisions D1 to D118 are the
orchestrator's from the earlier briefs and stand unless one is wrong.

# The corrections

## D119 - SVC12-01 - every classifier call site is given the installation

This is the important one, and it is in the class Wido's requirement
lives in rather than in the class of sentences that read badly.

Mission start and mission resume can open a stopped checkout. They
decide by classifying the caller, and they hand the classifier the state
root, the top of the checkout, as its installation directory. Where the
engine lives in a subdirectory, which is how it lives in this
repository, the runtime adapter directory is not under that root, so the
classifier has no agent-runtime signature to match and cannot recognise
an agent at all. Proven by execution: classifying the same caller with
the repository top as the installation yields no runtime signature.

The fix has the shape of D110 and D115 rather than a patch where the
defect was found. Resolve the installation one way, the same way the
three process verbs now resolve it, and give that value to every
classifier call site. Then enumerate every call of the verb classifier
in the tree in the return, and state for each one which installation it
now passes and why that is the right one.

Test it: with the engine installed in a subdirectory, a caller that
should classify as an agent does classify as an agent on the mission
fence path, and the fence stays closed.

## D120 - SVC12-02 - the shared description refuses a record it cannot describe

A creator reads the fence, publishes, then reads again to learn whether
a stop closed underneath it. That second read treats a closed fence and
an open fence at a newer generation the same way, and hands the open
record to the closed-fence description. After a completed stop and a
human arm, the creator therefore prints that the last stop is incomplete
and sends the person to run stop again.

Two changes, both required:

- The shared description, its remedy and its health prefix refuse a
  record that is not closed, loudly, rather than falling through to a
  final branch that assumes closed. A helper that cannot describe its
  input says so.
- The creator's second read distinguishes the two cases. A closed fence
  keeps today's behaviour. An open fence at a newer generation means a
  stop completed and a human armed the checkout while this creator was
  publishing, so the creator still ends what it started, and the
  sentence it prints says that the checkout was stopped and armed again
  and that the caller may retry. Derive the exact wording from sections
  13.1 and 18; do not invent a new state.

Test both: the helpers refuse an open record, and a creator whose second
read finds an open newer generation prints the retry sentence rather
than the incomplete-stop sentence.

## D121 - SVC12-03 - every refusal preserves the caller's installation flag

Five of the seven refusals stop and arm can print name a retry command
that drops the installation directory the caller had to supply, so in
the layout that needs the flag the printed command does not work. Round
27's retry-command helper is called only from the two human-terminal
gates.

Route every refusal in that file through that one helper: the crash-step
refusal, both verbs' refusal second lines, the fleet-form refusal and
the scope refusal included. Then enumerate all seven refusals in the
return and give the exact second line each one now prints.

This is section 15.2 and section 18.1 together: a refusal names only
flags its own verb accepts, and it names a command that works.

## D122 - SVC12-N1 - one spelling of the phase description

The exported health prefix helper in the fence package is now called by
nothing but its own test, because health re-implements the same
three-way choice inline in order to add a case the shared helper lacks:
a record that claims a completed stop while still listing unresolved
entries is incomplete. Both spellings agree today and can drift
tomorrow.

Move that case into the shared helper and have health call it. If the
shared helper cannot carry the case for a reason you can state, say so
in the return and delete the unused helper instead; what is not
acceptable is two spellings of one rule with only the unused one tested.

## D123 - SVC12-N2 - the mission refusal names a command that works

The refusal a non-human caller gets from mission start or resume on a
stopped checkout prints a command without the root flag the verb
requires, so typing it verbatim exits with a usage error. The design's
reader table specifies that exact text, so the code and the design
disagree and one of them is wrong.

The newest sections decide it: section 18.1 requires a refusal to name a
command that works, and a later section wins over an earlier one. Print
the working command, including the root flag the verb requires, and
record in the return that the design's reader table still shows the
older text so the page can be corrected separately. Do not edit any
design or plan file.

# Scope

Nothing outside these five corrections and their tests. No test is
weakened or deleted to make a correction pass. No new record, verb,
barrier or pass. No design or plan file is edited.

# Verification

Run, and report each result in the return:

- `scripts/agents/go-gate.sh --fast`
- `go test -count=1 -timeout 40m ./internal/stopfence/ ./internal/stoptransition/ ./internal/run/ ./internal/up/ ./internal/supervise/ ./internal/steward/ ./internal/goal/ ./internal/dispatch/ ./internal/missionrunner/ ./cmd/metasystem/`
- `bash -n scripts/agents/supervision-hook.sh`

Two failures in that set are pre-existing and not yours:
TestGoalTemporaryAuthorityRefusesPastAndBeyondHorizon in cmd/metasystem
fails because the temporary-authority horizon passed on 2026-09-06, and
goal fixture-review-by-date-rolls-over owns it. A process-group
ownership probe in the same package cannot observe its own spawned group
inside a delegate sandbox; it passes outside one. Report both as
pre-existing and touch neither.

The orchestrator runs the supervision, supervision-hook, dispatch,
goal-cli, mission and suite-progress beds outside your sandbox. Do not
attempt them, and do not treat a sandbox failure as evidence about the
diff.

Gap rule: stop and report a gap; never fill it silently.
