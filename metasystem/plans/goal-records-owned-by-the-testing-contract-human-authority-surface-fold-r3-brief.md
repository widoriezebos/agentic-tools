Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal goal-records-owned-by-the-testing-contract)
Date: 2026-09-10

# Fold round three: what the landing receipt refused

Follow-up round on chain ha-surface1-20260910. Round two carried the
reviewed hp-terminal change (its terminal-grade walk goes to the root
of the process tree) together with the surfaces; the critic
ha-surfacecrit2-20260910 found the join exact. The landing receipt on
this Mac then refused three groups. This round folds all three and
nothing else.

## 1. governed-standard: the steward tests' fake reader has a cycle

Nineteen tests in internal/steward fail with "enroll approval fixture
terminal: terminal enrollment refused: ANCESTRY_CYCLE". The fake
reader attentionAuthorityReader in
internal/steward/ledgerattention_test.go answers every pid with
ParentPID 1, pid 1 included, so the walk to the root visits pid 1 twice.
Fix: pid 1 gets no parent (ParentPID 0, ParentKnown true), every other
pid keeps parent 1, as the humanauthority tests' own fixtures do. I
proved this one change through a Go overlay: go test on
internal/steward and internal/gaterun then passes completely here.

## 2. shell-and-dependency-audits: a banned interpreter

The audit refuses "dependency ratchet: banned interpreter perl:
scripts/agents/goal-cli-fixtures.sh:575". The proof-grades scenario
uses perl only to start its headless case in a new session (POSIX
setsid, then exec) so that the walk proves TERMINAL_NOT_REACHED. macOS
has no setsid program and python3 is confined to declared sites. Fix:
give the engine one small verb that does exactly that, in
cmd/metasystem/process_verbs.go beside the other proc verbs (for
example proc detach -- <command> <args>: start the command as a new
session leader with syscall.SysProcAttr Setsid, stdin from /dev/null,
wait, and exit with its status), with one unit test, and call it from
the scenario in place of perl. If an existing internal verb already
starts a plain command as a session leader without a run record, use
that instead and say so.

## 3. section/goal-cli-fixtures: the enrolled-engine wall, demoted for now

The proof-grades scenario fails inside the receipt with "proof-grade
holder is not the live leader of its own controlling-terminal session"
and a probe that carries no terminalId, terminalKnown or
sessionLeaderPid fields: the receipt runs its shell sections with the
checkout's ENROLLED engine (main's), not the candidate's, and main's
proc probe predates the fields this change adds. The same scenario
passes on this Mac with the candidate engine (seat gates of rounds 7 to
14 of the hp-terminal chain). That wall is goal
receipt-beds-run-the-candidate-engine (m1b, in its third critic round
now); until it lands no engine-changing candidate can pass this section
in a receipt. Wido approved the same demotion for
section/gate-fence-fixtures this morning (landed as 7db34a27), and the
seat carries his approval for this one. Fix, in metasystem/testing.json,
the same shape as 7db34a27: the group section/goal-cli-fixtures keeps
its definition and its place in cadence, its obligations become empty,
and it leaves the standard list of surface goal-cli-fixture and the
deep list of surface dispatch-goal-mission. Run metasystem test check
(or the plan verb) to confirm the contract still validates. The
re-promotion is recorded on goal gate-fence-returns-to-per-landing-proof.

## Mandate

1. The three fixes above, nothing else: no other production code, no
   other test, no other contract change.
2. Prove: go build, vet, gofmt; go test on internal/steward,
   internal/gaterun and cmd/metasystem (the two cmd failures that exist
   on main, the process-classifier repair test and the frozen public-v1
   corpus test, are not this chain's); the scenario proof-grades of
   scripts/agents/goal-cli-fixtures.sh with the rebuilt engine, if your
   sandbox can run it; and the shell audit section if it can.

## Constraints

Wall-clock budget: 20 minutes. Return per the implementer schema and
report the round as your own. Stop at a gap that needs a decision no
page has made; report it with the resolution you propose. Never delete
written work.
