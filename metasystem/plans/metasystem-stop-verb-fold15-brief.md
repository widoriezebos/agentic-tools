Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 17: the last finding, found by execution (chain stopverb-build1)

Round 16's fixture work was right: wrong-terminal passes and the
supervision bed is fully green, all eight of its slice-1 scenarios plus
every pre-existing one but `rearm-launch-fails`, which is red on main.
The eleven-package matrix, the dispatch bed and the suite-progress bed
are green.

mission-stop still fails, and NOT because the scenario is wrong. The
orchestrator traced the child directly: the scenario dies on the stop
verb itself, whose non-zero exit aborts the shell, and its output was
being written to a file the cleanup removed. Instrumented, the verb
printed this:

    checkout <repo>
    mission cooperating-host runner pid 2998 pgid 2998 tag ...: stopped (TERM, runner concluded)
    mission cooperating-host turn ... host pid 3234 pgid 3234: stopped (by the runner, TERM)
    NOT STOPPED repo-watcher pid 2767: running: shutdown returned no outcome; did: left it in the fence record
    NOT STOPPED job-reaper pid 2768: running: shutdown returned no outcome; did: left it in the fence record
    stopped <repo>; start again: metasystem arm --repo <repo>

Two defects in five lines, decided in the design's new sections 14.5 and
14.6, which you must read.

# Decisions (the orchestrator's; decided, not open)

D63. Per section 14.5: when the owner returns no outcome for a recorded
component, the caller stops that component by identity rather than
reporting it as a survivor. TERM, the orderly wait, then KILL, with the
identity re-proved beside each signal, and the outcome reported like any
other. Only a component that outlives the ladder, or whose identity
cannot be inspected, stays `NOT STOPPED`. This is the ordinary
crashed-owner case, and the mission bed reaches it because it fabricates
supervisor facts without a live owner.

D64. Per section 14.6: the closing line splits. Everything stopped:
`stopped <toplevel>; start again: metasystem arm --repo <toplevel>`,
exit 0. Anything not stopped: `stop incomplete for <toplevel>; <n> not
stopped, listed above; run: metasystem stop --repo <toplevel>`, exit 1.
Update every scenario and package test that compares the closing line,
and add a package test for each of the two forms.

D65. mission-stop then expects its cooperating case to exit 0 with every
component stopped. Where the scenario runs the verb and a non-zero exit
is legitimate, it captures the status and asserts the lines instead of
letting the shell abort. Its output files stay inside the fixture root,
which is fine, but a failing assertion must print the verb's output to
stderr so a failure is diagnosable without instrumenting the bed: the
orchestrator lost twenty minutes to that today.

D66. Nothing else changes. If the sweep of D63 needs a seam that does
not exist, say so rather than inventing one.

# Verification

Reported at evidence level ran: `scripts/agents/go-gate.sh --fast` and
`go test -count=1` over the boundary, with the private caches earlier
rounds used. Name what you expect from the supervision, mission, goal
and dispatch beds; the orchestrator runs them outside your sandbox and
reports.

# Constraints

Wall-clock budget: 120 minutes. Return per the implementer schema with
the cumulative diff boundary listed. Gap rule: stop and report a gap;
never fill it silently.
