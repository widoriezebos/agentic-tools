Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, follow-up round under goal enroll-terminal-refuses-macos-terminal)
Date: 2026-09-06

# Correction round 2 for chain enroll-login-build1

The code critic (job enroll-login-crit1, finding register round 1) found
one material item, ELN-01, which the orchestrator confirmed by running
it: on the reviewed tree, `go test ./cmd/metasystem/ -run
ClaimLaunchCapabilitySurvivesRelativeExecutableProcessShape` fails. The
helper in metasystem/cmd/metasystem/claim_launch_process_darwin_test.go
(line 19) requires the kernel executable path of a process launched as
`bin/metasystem.test` to be RELATIVE, and the darwin reader decided in
D3 of metasystem/plans/enroll-terminal-login-node-build-brief.md now
reports the executed image's ABSOLUTE path (observed: the absolute path
of the copied test binary under the temporary root). The whole command
package otherwise passes on the reviewed tree; the orchestrator ran it.

# The decision (D8; decided, not open)

The relative-path fragility that test reproduced came from the old
reader, which returned the exec path embedded in the process arguments,
spelled the way the launcher spelled it. The new reader returns the
kernel's record of the executed image, so a relative launch yields an
absolute path, and the relative shape can no longer occur through this
reader. The test keeps its name and its purpose: a delegate launched
through a relative executable path is authorized by the claim-launch
capability. Its helper changes as follows and nothing else in the test
changes:

- Replace the precondition at lines 18 to 21 with the new contract: the
  kernel path is absolute and resolvedDelegatePath of the kernel path
  equals resolvedDelegatePath of os.Executable; fail with a message
  saying the executable reader must report the executed image's
  absolute path for a relative launch.
- Delete the legacy-comparison assertion at lines 26 to 32. It asserted
  that the old reader's relative shape defeated a path comparison; that
  comparison is no longer how authorization works, and the shape no
  longer exists.
- Keep the capability preflight and consumption assertions (lines 33 to
  43) unchanged, and keep the parent's relative launch (the copied test
  binary run as `bin/metasystem.test` with the temporary root as the
  working directory) unchanged, so the test still exercises a relative
  launch.

One non-material wording item from the same review (ELN-03) is folded
too, because D5 requires a refusal's stated reason to be true: in
metasystem/internal/humanauthority/authority.go, the second-read branch
that refuses when the first read had readable arguments and the second
read withheld them keeps its outcome ARGV_UNREADABLE, and its reason
becomes "the process's arguments changed from readable to withheld
between observations". Add one table case for that sequence to
TestSystemLoginAdmissionRequiresStableWithheldArgumentsAndRootOwner in
metasystem/internal/humanauthority/authority_test.go: a root-owned
login readable on the first read and withheld on the second refuses
ARGV_UNREADABLE with that reason.

# Gate

From the metasystem directory: `gofmt -l .` prints nothing; `go vet
./...`; `go build ./...`; `go test ./internal/humanauthority/ -count=1`;
`go test ./cmd/metasystem/ -count=1 -run
'ClaimLaunch|EnrollTerminal|HumanAuthority|Authority'`. If the
relative-executable test cannot run in your sandbox (it copies and
re-executes the test binary), say so plainly; the orchestrator runs the
whole command package afterwards.

# Constraints

Wall-clock budget: 20 minutes. Only the changes above; nothing else
moves. Declare the boundary as every file that differs from main (the
round-1 files plus the test file). Gap rule: stop and report a gap;
never fill it silently.
