Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-09

# Fold round eight: the top of the process tree is a fact, not an unreadable parent

Follow-up round on chain hp-terminal-build1-20260909; your worktree
carries round seven. The fifth critic (hp-terminal-crit5-20260909) ran
round seven's own live test verbosely on this macOS host and showed
that HPT-13 is not fixed: the walk still cannot reach the top of the
tree, so every human is still refused the terminal grade and enrolling
a terminal is still impossible; only the refusal's name changed.

## HPT-15 (critical): the parent check fires before the new rule can help

The live test logged: "live ancestry stopped at an allowed
machine-specific outcome: ANCESTRY_UNREADABLE: process pid 1,
executable "/sbin/launchd", owner uid 0 was not admitted because the
process's parent is unreadable". In internal/humanauthority/authority.go
the stable read refuses any process whose parent cannot be read, and
both platform readers (internal/identity/enumerate_darwin.go, lines
94-107; internal/identity/enumerate_linux.go, lines 53-63) translate a
reported parent of zero, or a parent equal to the process itself, into
"parent unknown". Process 1 always reports parent zero. That branch
sits below the round-seven root-owned-image rule and is reached first.

The rule, decided: the top of the process tree is a known fact. A
process whose reported parent is zero (or itself) at the top of the
tree is the root; the readers report it as "parent: none, known", never
as "parent unknown"; "unknown" is reserved for a parent the kernel
refused to tell us. The walk ends at the root after reading it under
the round-seven rule (root-owned, protected system image, withheld
arguments admitted and recorded). A proof whose last node is the root
is complete; a proof that stops anywhere else is not.

## HPT-16 (high): the fakes and the live test accept the broken outcome

Every fake process tree gives process 1 a parent of 0 while marking the
parent as known, a shape the real readers could never produce, so the
fakes ran past the check that stops the real walk
(internal/humanauthority/authority_test.go authoritySnapshot lines
52-65 and systemRootSnapshot lines 82-84;
cmd/metasystem/goalsync_mutations_test.go line 37). And the live test
(authority_test.go lines 212-231) fails only on ARGV_UNREADABLE or
ANCESTRY_CYCLE and otherwise logs and passes, so the one outcome that
means "the walk is broken on this machine" counted as acceptable.

After the rule above, the fakes must produce exactly what the readers
produce for a root (build the fake root through the same
representation the readers emit), and the live test must assert that
the walk REACHED the top of the tree: its last node is the root, there
was no read refusal at any node, and the outcome is one of the two
honest ends on a real machine (proven at the terminal grade on an
agent-free terminal, or AGENT_IN_AUTHORITY_CHAIN naming a real runtime
under an agent seat); under `go test` with no controlling terminal use
whatever entry lets the walk run without the terminal requirement, and
say which. Anything else fails the test.

## HPT-18 (low): parent continuity in the terminal walk

The enrolled walk pins the parent's birth identity when it reads the
child and refuses PROCESS_REUSED if the process later found at that id
is a different one; the terminal walk records the parent's number only
and adopts whatever it finds there. Give the terminal walk the same
continuity check.

## HPT-19 (low): the recorded-holder cleanup path never kills

When the holder's start time was recorded and it does not exit within
the bounded wait, cleanup reports and sets status but sends no kill,
unlike the keeper, session and unrecorded-holder paths. Add the same
unconditional kill after the bounded wait.

## HPT-17 (medium, not material, no change)

A root-owned protected system image with withheld arguments is exempt
from the signature check, so a root agent behind /bin/bash or sudo would
pass. Accepted as out of model: an agent running as root can rewrite
the enrollment record or the engine itself. Recorded; do nothing.

## Mandate

1. The root-of-tree rule in the readers and the stable read; the walk
   ends at the root; the proof's last node is the root.
2. Fakes shaped exactly as the readers shape a root; the live test
   asserts the walk reached the root with no refusal (both platforms).
3. HPT-18 and HPT-19 as above.
4. Nothing else changes.

## Proof

Run internal/humanauthority (verbose for the live test, and paste its
log line in your return), internal/identity, and
`go test ./cmd/metasystem/ -run 'TestSyncReqLineage$|TestStoppingRequest|TestGoalEnrollTerminalSucceeds|TestSessionStopAttendedHumanEndsQuietly'`;
gofmt, vet. The orchestrator runs the full packages, the goal-cli bed
and the hook suite on the seat, and a human runs the bed from an
agent-free terminal. Report the round as your own.

## Constraints

Wall-clock budget: 25 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
