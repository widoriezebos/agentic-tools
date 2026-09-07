Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-07

# Round 13: fold the first code critique (chain stopverb-build1)

The chain's first independent read found six material items, all
accepted. Dispositions are in
records/misc/metasystem-stop-critique-r4.md (landed on main after this
worktree was cut, so it arrives with the rebase); the design gained
section 14 deciding the two classes the earlier sections left open. Two
of the six are the orchestrator's fault, not yours, and are named as such
below. This round folds all six.

Read section 14 of metasystem/plans/metasystem-stop-verb-design.md first.
It wins wherever it contradicts an earlier section.

# Decisions (the orchestrator's; decided, not open)

D48. THE MISSION HANDOVER (SVC-01). `mission start` and `mission resume`
classify their caller through the classifier the launcher already calls
when it resolves its arming identity. A `HUMAN` caller takes the
transition lock, opens the fence through the transition owner's existing
`OpenFence`, releases the lock, and then arms; every other caller is
refused with the stopped sentence and section 9's second line. Fold-10's
decision D42 told you to read the fence before arming and said nothing
about the human path: that omission is the orchestrator's, and this
decision replaces it. Delete the test that asserts a refusal for a human
caller.

D49. ARM'S SECOND LINES (SVC-02). Section 9's table is exact. A stop in
progress: `run: metasystem status --repo <toplevel>`. A survivor of the
last stop: `run: metasystem stop --repo <toplevel>` followed by section
2's words, which name the pid and say it is listed with its start time. A
claim file whose creator liveness is unknown names the file, per section
14.1. Each branch gets the test the remote-job branch already has.

D50. THE FOLLOW-UP DISPATCH PATH (SVC-03). It calls the same fence gate
the ordinary path calls, before the census gate. Add the follow-up form
to the stop-fence scenario's list of driven creation paths.

D51. NO EARLY EXIT (SVC-04), per section 14.1: after the record is
written closed, no failure exits the transaction; each becomes a line and
the run continues. Unparsable claim files are reported by name, removed,
and the removal printed. A claim whose creator liveness is unknown is
waited out once, reported as not stopped, and removed when it predates
the fence closing. Package tests for both, driving the claim directory
directly.

D52. THE UNREADABLE RECORD (SVC-05), per section 14.2: status prints its
inventory plus the unreadable line and never returns an empty report;
arm repairs by replacing the record and printing that it did; stop
refuses with the reason and points at arm. Package tests for all three,
including a record whose schema version is higher than this engine's.

D53. Section 10 now marks each scenario's slice (section 14.4). Nothing
for you to change in the design; use it as the list of what this slice
owns.

D54. TWO SCENARIOS COME BACK (SVC-06). Build mission-stop and
arm-refuses-survivor in this slice. They prove behaviour this slice
claims and their absence is why SVC-01 and SVC-02 had to be found by
reading. mission-stop needs the held fake host that round 5 correctly
called unspecified; it is specified now: under
`METASYSTEM_FAKE_HOST_HOLD=1` the fake host writes `host-ready` in its
turn directory AFTER installing its orderly-signal handler and then
holds; under `METASYSTEM_FAKE_HOST_IGNORE_TERM=1` it additionally
ignores that signal; both are refused outside a fixture-mode root. The
scenario waits for `host-ready` before it stops, which is the readiness
handshake the earlier gap named. mission-stop covers the cooperating
host, the ignore-TERM host, and the resume of D48 through a closed
fence under fixture human authority. wrong-terminal also belongs to this
slice; build it if it is not already there.

D55. Your return names EVERY scenario section 10 assigns to slice 1 that
you did not build, as a gap, whatever any brief deferred. A deferral
recorded in a brief is still a gap in a return.

# Verification

Reported at evidence level ran, with the private caches earlier rounds
used: `scripts/agents/go-gate.sh --fast`; `go test -count=1` over every
package in the boundary; and the slice-1 scenarios named with the exact
bed commands, which the orchestrator runs outside your sandbox, where
all of them work. The orchestrator's last run had every slice-1 scenario
green and all eleven packages green, so a regression is yours to notice.

# Constraints

Wall-clock budget: 120 minutes. This is a large fold: if it does not fit,
finish whole items and name the rest, in the order D48, D50, D51, D52,
D49, D54. Return per the implementer schema with the cumulative diff
boundary listed. Gap rule: stop and report a gap; never fill it
silently.
