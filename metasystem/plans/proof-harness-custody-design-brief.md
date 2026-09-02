Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal proof-harness-process-custody)
Date: 2026-09-02

# Goal

Author the design for goal proof-harness-process-custody (read
metasystem/plans/goals/proof-harness-process-custody.md first: two
specimens, twelve leaked CPU-hog loops on m2 and m3, then 488 orphaned
fixture processes and 8,789 stale beds on m1 found on 2026-09-02, and the
done criterion, which now includes a sweep the seat may run). The
recovery analysis (metasystem/plans/recovery-analysis.md, section 4) has
already traced who spawns real machinery, how cleanup is supposed to
work, and why the survivors survived; build on it, do not redo it, and
cite it. Write exactly one NEW file named
proof-harness-custody-design.md in the metasystem plans directory. Every
claim about the tree cites file and line.

# Workspace

The delegate worktree the dispatcher created for this job. Read anything;
write only that one new design file.

# Scope boundary, stated so two designs cannot collide

The recovery umbrella's proposed slice S4 (recovery-analysis.md section
6.2) covers runners and owners recording their bed root and exiting when
it vanishes, and fixtures arming through one custodian. THIS design owns
two things: (a) custody of what a seat-run proof harness spawns (load
generators, busy loops, fake adapters), so a harness killed on any path
leaves zero orphans; and (b) the seat-runnable sweep of machinery that no
live record owns. If you find (a) or (b) cannot be designed without S4's
bed-root rule, say so and state the minimal piece you must take, with the
reason; otherwise leave S4 to the umbrella.

# What the design must settle

1. HARNESS CUSTODY. The goal's direction: a harness runs its load
   generators in one process group it kills whole on every exit path
   (trap on EXIT, kill by negative process-group id), or better, a small
   engine verb (proc load-generate --seconds N --workers K) whose group
   the existing kill-through machinery owns, so no shell job table is
   ever the custodian. Decide between them against the existing owners
   (the proc verbs in metasystem/cmd/metasystem/identity_probes.go and metasystem/internal/census, the custody and kill-through verbs in
   metasystem/cmd/metasystem, the delegate machinery's process-group
   custody that cured the same disease for its own children). Specify
   the verb or the shell contract exactly, and every exit path it covers
   (normal, error, INT, TERM, HUP, KILL of the harness itself, tmux pane
   death). State plainly which path no custody can cover and how the
   sweep catches it.
2. THE SWEEP. A verb a seat may run that finds machinery no live record
   owns (steward runners, supervision owners and components, adapter
   loops, load generators) whose bed or working directory is under a
   temp root or a preserved failure bed, older than a bound, and whose
   ancestry is not a live seat's; kills by exact pid after re-proving
   identity (pid, start, tag) under the census's eye; removes beds whose
   processes are gone and whose age passes the bound; and reports what it
   did in one line per victim. Read metasystem/internal/census (the
   UNTRACKED classification and the scope rule in scope.go) and the
   shared-machine rule in metasystem/docs/orchestration.md (kill only what
   you can prove is yours, by exact pid) and show the sweep obeys it:
   what proves a process is ours (argv under our temp bed, our tag
   prefixes, our engine path) and what refuses (anything else). Decide
   whether the steward runs it on a cadence or only a seat runs it by
   hand, and why.
3. FIXTURES. A harness killed mid-run by each signal leaves zero orphans
   and the census sees nothing unowned; a synthetic bed with a leaked
   runner, an owner and a busy loop is swept and reported; a process
   that merely looks like ours (a lookalike argv outside our beds) is
   refused; the sweep on a clean machine does nothing. Deterministic,
   no sleeps beyond bounded waits, in the style of
   metasystem/scripts/agents/supervision-fixtures.sh.
4. SIZE. One slice of at most 240 reserved minutes if it fits, else two;
   estimates against recorded precedent or marked unsupported.

Self-grade per the house rule: confidence, weakest claim, reject condition.

# Constraints

Wall-clock budget: 40 minutes. Design only; no builds, no benchmarks
(R-31). Edit nothing but the design file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file
named under Goal.

# Gap Rule

stop and report a gap; never fill it silently.
