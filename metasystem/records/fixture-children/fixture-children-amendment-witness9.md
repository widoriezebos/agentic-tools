## Amendment, revision 5: two kinds of survivor at teardown

Appended after m1b, which owns `cmd/metasystem/wait_verb_test.go` and the
fixture unit 9 converts, read the cleanup graph and reaped thirteen orphans
of its class on the machine the same morning: `steward run --repo
/T/TestPendingWaitFromChildShell*/002`, aged 16 minutes to 4 hours 12
minutes, directories still present, no live owner, all thirteen dead on a
plain TERM.

The path is the one section 1 names. `finishChild` kills only on its timeout
path; every return before that select leaves the child alive, and because
bash execs away it reads as `metasystem job watch`. Under 3.2 the recorded
ref reaches cleanup step 1, which sends KILL and accepts the result: a live
match is neither "gone" nor an error. The goal page's second condition on
unit 9 says the reap reports instead of killing only when ownership cannot
be proved; here it can, so the child dies with no line about it anywhere.
Whatever failed the test, if anything, names the write or notify error and
never the child. Witness 9 as written, the converted tests green with no
survivor line in `go-tmp` and empty custodian logs, is what that leaves
behind. The leak unit 9 exists to kill would pass its own witness, unnamed
instead of visible.

### The rule

Section 2 gains a seventh consequence, after rule 6, whose leak it names:

7. A child the test brings to exit and that teardown finds still running
   fails the test by pid, exe and argv, whether or not its identity can be
   re-proved; a zombie has exited and is not that child. Only a child the
   fixture declared held is reaped without a word.

The goal's second condition stands as written. What it left unsaid is what
a proved kill means for the test; rule 7 says it. Ownership proof decides
whether a signal is sent, never whether the failure is reported.

### Where the line lies

The line is who ends the child. If the test's body, or production code the
body checks, brings the child to exit before teardown, the child is
finished: alive at teardown, it contradicts the test. If the fixture keeps
the child alive until teardown or until the owner dies, so that cleanup,
the bed's reap or the custodian is the only thing that ever kills it, the
child is held: alive at teardown, it is the scenario.

On this page the finished children are the wait_verb `job watch` child
(notified, then its exit observed), the supervision `up --shutdown` ends
(unrecorded; step 3 and the binary's exit scan already name what they
find), the attention wrapper and its child (production kills the group and
`waitForGroupAbsence` checks) and the `__stopped` leg's runner and detached
member (the ladder and the guard sweep end them, the leg checks). The held
children are the `hang` grandchild, the fake host of witness 11, the shells
of witnesses 3, 4, 5 and 16, which no test cleanup ever reaches, and the two
shells of witness 12's live variant. The transport wrappers of 3.7 sit on
the same line by the same question.

The thirteen m1b reaped were supervision the test's hook started. Under the
goal's first condition the key scan runs after `up --shutdown` or leaves
that supervision to the binary's exit scan in `testenv.Main`; either names
what it finds. Leaving it out of the scan cannot mean accepting it.

### How a fixture declares the kind

The record call is the declaration. `Record` records a finished child.
`Hold` records a held child. There is no third call and no default that
hides: a fixture author who forgets the word gets a failure to read. The
shell helpers follow, `_hold_pid` beside `_record_pid`, and
`harness_fixture_reap` reads the kind from the file it already reads.

### What changes in the mechanism

3.2, cleanup step 1, replaced:

1. `SignalExact(KILL)` each recorded ref. Mismatch or absence is "gone",
   accepted for either kind. A live match on a held child is killed, and
   that is the end of it. A live match on a finished child is read for
   state: a zombie has exited, nothing is sent and nothing is said, and
   step 2 accepts it as it already does; a running process is a survivor,
   killed through `SignalExact` and `t.Errorf` naming pid, exe and argv as
   a finished child found running. Unknown or unreadable sends nothing and
   is `t.Errorf` for either kind, naming the ref as unproven.

Steps 2 to 4 stand; step 3 already fails by name on anything it finds.
`harness_fixture_reap` kills held pids first and says nothing, fails the bed
by name on a live finished pid, then runs `--key` for the unrecorded rest;
3.5's "fails the bed on survivors" means the last two. `finishChild` keeps
its ownership proof and timeout kill as behaviour under test, as 3.7 says,
and the cleanup order the goal's first condition fixes does not move: the
child `finishChild` failed to finish is exactly what step 1 now names.

### State, not a grace

`finishChild`'s timeout branch has two returns after its own KILL. At
:1263 the reap wait expires: the child was SIGKILLed five seconds earlier
(the reap context at :1257 is itself a five-second bound), its exit was not
observed, and the `child.Wait()` goroutine will still reap it. A killed
process that can still be scheduled dies in milliseconds, so after five
seconds the child is in one of two states that want opposite answers. It
is a zombie: exited, not yet reaped by the fixture's own `Wait`, listed by
`ps`, `kill -0` succeeds, no survivor. Or it is still running after
SIGKILL: uninterruptible sleep, or something holding it; rare, and the
finding rule 7 exists to name. The zombie is the likely state under load,
and load is when a five-second reap window expires at all.

A grace after the fact was considered and dropped: the five seconds have
already passed, so a short wait buys nothing, and it cannot tell the two
states apart, so every expiry of that window would fail rule 7 naming a
process that has already exited. Waiting longer does not fix that; reading
the state does. It is exact where a grace is a guess, and it keeps the scan
ignorant of the fixture's kill history, the property that made the grace
attractive. A third outcome, "killed and not yet reaped", was dropped for
needing that history.

The state comes from what the prober already reads: `kinfo_proc.p_stat`
in the `kern.proc.pid` reply on Darwin (`SZOMB`), the third field of
`/proc/<pid>/stat` on Linux (`Z`). `Exact` gains `Zombie bool`, filled by
`Probe`; no `ps` and no new read.

One word, two consumers, wanted in opposite directions; the split is stated
here so the rule is not misread later. For the prober a zombie stays
Alive: its entry exists with the same start time, the pid is not free, and
`Dead` there proves absence and authorizes action (a dead-owner scan, the
custodian's trigger), so a zombie must not be called Dead by the prober.
For rule 7 a zombie is dead: it has exited, nothing of it runs, and it
cannot outlive the run, because whatever reaps its parent's children
reaps it. Step 2's "dead or a zombie" was already this reading; step 1 now
shares it through `Exact.Zombie`. A test that ends its child observes the
exit, as `finishChild` and `waitForGroupAbsence` do; a kill with no wait
leaves a window in which teardown reads a running process, and rule 7 is
right to name it, because that test has not finished its child.

The other return, :1254-1256, is `Kill` failing, which in practice means
the goroutine already reaped the process: step 1 finds "gone". Were the
child running instead, rule 7 names it. Rule 7 handles it either way, and
the list of paths is not closed at four.

### Witness 9

Replaced:

9. Two clauses; the second cannot be met by an accepted kill. First, the
   converted attention and wait_verb tests are green; their `go-tmp` holds
   no survivor line and the custodian logs are empty. Second, each
   conversion is made to leak once, in a throwaway edit recorded in the
   landing note, the form the goal's fourth condition already uses for its
   mutation proofs: `finishChild` returns an error before
   `writePendingWaitJobStatus`; the attention test `t.Fatal`s right after
   its wrapper starts. Each mutated test fails and its failure names the
   live child as a finished child found alive at teardown, with pid, exe
   and argv (`metasystem` and `job watch ...` for wait_verb; the wrapper
   and its child for attention); every named process is gone within two
   seconds; the custodian log is empty; `proc fixture-survivors --key`
   prints nothing. A failure that names only the injected error, the child
   gone, is the leak made invisible and fails the witness.

### The other witnesses that assert an absence

Checked for the same hazard: a test asserts a child exited, the child is
alive at teardown, an accepted kill removes it, the witness still passes.

- 2 asserts no signal went to a child the test watched exit. An accepted
  kill is a signal, so a live child already fails it. Its `sh -c 'exit 0'`
  is finished; under rule 7 a live one fails twice. Unchanged.
- 10 checks "pids gone after the ladder" in the leg's body, before any
  cleanup runs, so a kill at cleanup cannot satisfy it; the second leg
  asserts the custodian's action after the owner's death, a positive
  claim. Runner and detached member are `_record_pid`. Wording unchanged.
- 11's host is held; alive at teardown is the scenario, and the witness
  asserts its death after the owner's. Whether the leash or the custodian
  removed it is 3.4's question, answered by witness 6's leash-disabled
  leg. The host is `_hold_pid`. Wording unchanged.
- 12 counts signals; an accepted extra kill breaks "exactly one KILL". Its
  children are held by design, so the table part and the live variant
  record them with `Hold`; without the word the live variant fails on
  cleaning the first. Wording: `Hold` added.
- 14 names every kill in the verdict and counts them; a `stale` survivor is
  reaped and named. Unchanged.
- 17 is not on the list but is the rule-6 witness, and its child is
  finished (the helper stands for `finishChild`). Both cases gain: the
  cleanup's failure names the child. A third case: the helper KILLs the
  child and returns before observing its exit, as :1263 does, and the
  fixture's own `Wait` is withheld until after cleanup, so the child is a
  zombie at teardown; step 1 reads `Zombie`, no failure names it, and the
  failure text is the helper's alone. No wall-clock wait; the zombie is
  made, not timed. The witness drives the fixture through a recording
  `testing.TB` and reads the failure text.

Witness 5 has the hazard from the kernel's side rather than cleanup's, and
unit 3's build hit it: the witness failed every run on Darwin. The test kills
the helper's whole group, as a job cancel does. That orphans a group with a
stopped member, so the kernel sends the group SIGHUP and then SIGCONT, and the
stopped child dies before any custodian scans. "Gone within two seconds"
then holds with no custodian at all, and "the log names both" can never
hold. The stopped child starts in its own session, where the group kill and
the orphaning do not reach it and it stays stopped until the custodian kills
it. Ignoring HUP is not the fix: SIGCONT would resume the child, and it would
no longer be stopped. The log clause matches `action=kill pid=<n> `, not a
bare pid. Wording: witness 5's stopped child is `Setsid`.

Nothing else on the page reaps without a word: the custodian logs one line
per action, the census prints what `--reap` touches, `testenv.Main` names
the test. Step 1's acceptance of a live recorded ref was the one wordless
kill.

### Where the rule lives

In section 2 as rule 7, in 3.2 step 1 as the branch and in 3.7 as one word
per converted child. Section 2 is the list of consequences each with a
witness, and rule 6 is the same leak seen from the other side. A rule only
in 3.7 covers the fixtures listed there and not the next author's; a rule
only in section 2 leaves the seat to guess a kind per conversion.

### Wording that changes

Section 2: rule 7, witnesses 9 and 17. 3.1: `Exact` gains `Zombie`. 3.2:
step 1 as above; step 2's "dead or a zombie" reads the same field. 3.5:
`harness_fixture_reap` as above. 3.7: the wait_verb child, the attention
wrappers and the `__stopped` pids are `Record` or `_record_pid`; the `hang`
grandchild and the fake host are `Hold` or `_hold_pid`. Witness 9 replaced.
Witness 5: the stopped child starts in its own session. Witness 12: `Hold` in both parts. Witness 17: the failure names the child,
and a third case for the zombie. Unit 5 lists `Hold` and `Zombie`; unit 10
lists `_hold_pid`; unit 12's doctrine paragraph states rule 7 with the rest
of section 2.

### Units

9 stays at about 215. The default kind is the loud one, so its `Record`
calls are the ones 3.7 already lists, and its leak-path evidence is two
throwaway edits recorded in the landing note, not code. 5 grows by about
thirty lines (`Hold`, the branch in step 1, `Exact.Zombie` on both
platforms, which step 2 already needs, witness 17's failure-text assertion
and third case); its number stays, because section 7's rule already splits
it at witness 17 and the half after the split takes the growth. 10 grows
by under ten (`_hold_pid`, the reap's order); about 250 stands. The rest
are unchanged.
