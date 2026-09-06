Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal enroll-terminal-refuses-macos-terminal)
Date: 2026-09-06

# Goal

Goal enroll-terminal-refuses-macos-terminal (tier 3, approved by Wido on
2026-09-06). Its record, metasystem/plans/goals/enroll-terminal-refuses-macos-terminal.md,
is the contract. In short: `goal enroll-terminal` refuses every shell opened
in the standard macOS Terminal. Terminal.app starts each tab through a
setuid-root login program whose arguments the operating system withholds
from a user process, and the enrollment walk in
metasystem/internal/humanauthority/authority.go (the function stableRead)
refuses ARGV_UNREADABLE for any process whose arguments it cannot read.
Wido hit exactly this on 2026-09-06.

# Facts measured on this Mac (m1b, Darwin 25.6.0, 2026-09-06; evidence level: ran)

The Terminal.app chain, bottom up: the interactive shell (pid 61288,
argv `-zsh`, uid 501) has as parent the login program (pid 61287,
effective uid 0, real uid 501, saved uid 0, comm `login`, executable
`/usr/bin/login`, argv WITHHELD), whose parent is Terminal (pid 1280,
uid 501, argv readable), whose parent is launchd (pid 1). The login
program is the SESSION LEADER of the shell (getsid of 61288 is 61287),
so both the enrollment walk and the later proof's session-leader check
read it.

What a uid-501 process can read about that root login today, reader by
reader (package metasystem/internal/identity):

- KernelProber.Probe, argv through sysctl kern.procargs2: denied, so
  ArgvKnown is false.
- ExecutablePath on darwin, also through kern.procargs2: denied, so
  ExecutableKnown is false.
- ParentPid on darwin, through proc_info flavor PROC_PIDTBSDINFO (3):
  denied, so ParentKnown is false.
- ControllingTerminalIdentity, through sysctl kern.proc.pid (the
  kinfo_proc entry): answers, tdev 268435462, the same tty as the shell.
- That SAME kinfo_proc entry also answers parent 1280, effective uid 0,
  real uid 501, comm `login`. The typed accessor unix.SysctlKinfoProc
  exposes them as Eproc.Ppid, Eproc.Ucred.Uid, Eproc.Pcred.P_ruid and
  Proc.P_comm.
- proc_info flavor PROC_PIDPATHINFO (11), call number
  PROC_INFO_CALL_PIDINFO (2), argument 0, a 4096-byte buffer, the call
  libproc's proc_pidpath makes: answers `/usr/bin/login` for the root
  login, `/bin/zsh` for the shell, `/sbin/launchd` for pid 1. It is not
  subject to the same-user check that denies the other two flavors.

So today stableRead refuses the login three times over (argv,
executable, parent), and the fix has two halves: readers that answer
for every live process, and a rule that admits exactly the system login
program.

The tmux shape, the workaround the refusal will name: a shell in a tmux
session Wido started from Terminal (pid 8458 under the tmux server
8457, both uid 501, the server's parent is launchd) is its own session
leader and has a controlling terminal. Its walk is one node and every
reader answers, so enrollment from it succeeds under today's code.

# The decisions (the orchestrator's; decided, not open)

D1. The rule: the system-login node. stableRead admits a process whose
argument read failed ONLY when, on both of its reads, all three hold:
the executable path is one of the known system login programs of the
platform (darwin: exactly `/usr/bin/login`; linux: the list is empty in
this goal, so the rule never fires there); the process's EFFECTIVE user
id is 0; and the argument read failed (ArgvKnown false) both times.
Every other process with ArgvKnown false still refuses ARGV_UNREADABLE.
Why this is safe: the argument check exists to match the adapter
signatures, which identify agent runtimes running as the human's own
user; a setuid-root system login program is the operating system's own,
its path is the kernel's record of the executed image (not argument
token zero), and that image is protected by system integrity
protection. Why effective uid and not real uid: measured above,
Terminal.app's login runs with real uid 501 and effective uid 0. Keep
the list as one named, platform-specific value with a comment stating
the invariant it protects.

D2. The owner is a fact the walk reads. Snapshot gains `OwnerUID uint32`
and `OwnerKnown bool`. stableRead requires OwnerKnown on both reads (an
unknown owner refuses ANCESTRY_UNREADABLE) and the same owner on both
reads (a change refuses ANCESTRY_CHANGED). Add `ProcessOwner(pid int64)
(uid uint32, ok bool)` to the identity package: darwin from the
kinfo_proc entry's effective uid (Eproc.Ucred.Uid); linux from the
effective column of the Uid line of the process's status file under
/proc. Tests: the test process's own owner equals os.Geteuid(); pid 1's
owner is 0 on both platforms; a dead pid answers false.

D3. The two darwin readers a user process cannot use on a root process
change source to the calls that answer for every live process:

- ParentPid on darwin (metasystem/internal/identity/enumerate_darwin.go)
  reads the parent from the kinfo_proc entry (sysctl kern.proc.pid,
  Eproc.Ppid), the same call kernelTerminalIdentity in
  metasystem/internal/identity/terminal_darwin.go already makes. Its
  contract stays what its comment says today: false when the process is
  gone, when the parent is not positive, or when the parent equals the
  pid. The existing tests TestParentPidMatchesGroundTruth and
  TestParentPidDeadProcess stay as they are. This widens what every
  ancestry walk on darwin can see: callers in metasystem/internal/gaterun,
  metasystem/internal/validate, metasystem/internal/lease and
  metasystem/cmd/metasystem/identity_probes.go stop at launchd instead of
  at the first root-owned process; none of them rely on stopping early,
  and their package tests are part of the gate below.
- ExecutablePath on darwin (kernelExecutablePath in
  metasystem/internal/identity/identity_darwin.go) reads proc_info
  PROC_PIDPATHINFO as measured above, instead of the exec path embedded
  in kern.procargs2. The argv reader keeps using kern.procargs2
  unchanged. TestKernelExecutablePathForSelf stays green.

D4. The audit record. Node gains `ArgvWithheld bool` with json tag
`argvWithheld,omitempty`, true for a system-login node. Its
ArgumentDigest is the digest the code computes today over the joined
argument list, which for no arguments is the digest of the empty string.
Proof.Valid does not change: a system-login node carries a non-empty
ArgumentDigest and no AgentRuntime. The enrollment record's shape does
not change.

D5. The refusal names what it saw and the shape that works. When the
enrollment walk refuses because of a process it could not read
(ARGV_UNREADABLE or ANCESTRY_UNREADABLE), the error text names the pid,
the executable path when known, the owner uid when known, and why the
process was not admitted (for a withheld argv: "the operating system
withholds this process's arguments and it is not a known system login
program"), and ends with the workaround sentence: "run goal
enroll-terminal from a shell whose ancestry up to its session leader is
owned by you, for example a shell inside a tmux session you started
from Terminal". The outcome constants do not change and the proof's
recorded Outcome values do not change. Enroll's error text still
contains the outcome code so existing tests and readers keep matching
on it. Implement by letting stableRead return the refusing process's
facts alongside the outcome (a small struct or a typed error); never by
re-reading the process.

D6. The pin. In metasystem/internal/humanauthority/authority_test.go,
with the existing fake tree reader (extend authoritySnapshot with
OwnerUID 501 and OwnerKnown true so every existing case keeps its
meaning):

- (a) the Terminal.app shape: Terminal (pid 1280, owner 501, argv the
  Terminal path, parent 1, no terminal), login (pid 61287, owner 0,
  argv withheld on every read, executable `/usr/bin/login`, parent
  1280, terminal tty-1), shell (pid 61288, owner 501, argv `-zsh`,
  parent 61287, tty-1), session leader 61287. Enroll from the shell
  succeeds and records the login's ref as SessionLeader; a command under
  the shell (pid 70, owner 501, parent 61288, tty-1) then proves
  HUMAN_AUTHORITY_PROVEN with a proof that Valid accepts.
- (b) the tmux shape: shell (pid 8458, owner 501, parent 8457, tty-2),
  tmux server (pid 8457, owner 501, parent 1, no terminal), session
  leader 8458: Enroll succeeds with the shell itself as SessionLeader.
- (c) refusals, each asserting the outcome and the named facts in the
  error text: a root-owned process with withheld argv whose executable
  is `/usr/bin/sudo` refuses ARGV_UNREADABLE naming the pid, the path,
  owner 0 and the workaround sentence; a uid-501 process with withheld
  argv and executable `/usr/bin/login` refuses ARGV_UNREADABLE; the
  login shape with argv withheld on the first read and readable on the
  second refuses ANCESTRY_CHANGED; the login shape with an unreadable
  executable refuses ANCESTRY_UNREADABLE; the login shape with an
  unknown owner refuses ANCESTRY_UNREADABLE; owner 0 on the first read
  and 501 on the second refuses ANCESTRY_CHANGED.
- (d) the existing "argv unreadable" cases keep refusing as they do
  today (their fixture process is user-owned).
- (e) in the identity package: the ProcessOwner tests of D2, and one
  test that walks the test process's own ancestry looking for a live
  process whose owner is 0 and whose argv is withheld, skips with a
  message when none exists, and otherwise asserts that ParentPid,
  ExecutablePath and ProcessOwner all answer for it. When the gate runs
  from a Terminal.app shell, as the orchestrator's will, this exercises
  the real root login.

D7. Non-goals. No change to Proof.Valid's rules, the temporary-word
path, the fleet enrollment record, the census or signature matching,
the linux login list (empty), or any document; no new dependency; no
change to the goal record; nothing under plans.

Known boundary, stated so it is not reported as a gap: an agent that
starts its own session (tmux, script, a pseudo-terminal) already gets a
shell that is its own session leader, and this rule neither opens nor
closes that door. The rule only admits a setuid-root system login
program as a node that cannot be an agent.

# Gate

From the metasystem directory: `gofmt -l .` prints nothing; `go vet
./...`; `go build ./...`; `go test ./internal/identity/
./internal/humanauthority/ -count=1`; then the callers of the widened
parent reader, `go test ./internal/gaterun/ ./internal/validate/
./internal/lease/ -count=1`; then `go test ./cmd/metasystem/ -count=1
-run 'EnrollTerminal|HumanAuthority|Authority'`. Report each command and
what it showed, with the evidence level marked.

# Constraints

Wall-clock budget: 60 minutes; return before it ends even if something is
red, naming it. DESIGN-BEARING reach (tier 3); a code critic reviews the
tree next. Declare the boundary as every file that differs from main.
Gap rule: stop and report a gap with your proposed contract written out;
never fill it silently. The decisions above are decided; a fact above
that your machine contradicts is a gap to report, not a decision to
remake.
