Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-09

# Fold brief: round two of the terminal grade for stopping acts (chain hp-terminal-build1-20260909)

You built round one. The orchestrator's seat-side gate found two
defects in your own test scaffolding, and the critic
hp-terminal-crit1-20260909 may add findings, listed at the end of this
brief if it returned before dispatch. Two scenarios of
goal-cli-fixtures.sh that fail seat-side (brain-stop-seeded and
brain-stop-corrupt) are NOT yours: they fail on main today because a
landing earlier today reordered the brain seat's Stop display; leave
them alone and say so in your return.

## HPT-01: the Go test seeds identifiers outside the ledger's alphabet

TestUnparkGradeFollowsTheStandingApproval in
metasystem/internal/goal/authority_test.go fails at setup,
deterministically: its four operation ids are minted from strings that
contain the letter U, and the ledger's identifier alphabet omits I, L,
O and U, so every seeded goal file is rejected as malformed ("opid
01J5X00000000000000000UG00-mac-a-eecc0a50 is not <ulid>-<machine>-<hash8>";
the machine token must not contain a hyphen either), and the seeded
file has no history lines of its own. Mint ids the way the package's
other tests do ("01J5X0000000000000000000BY-human-1a2b3c4d" is one), use
a machine token without a hyphen, and seed the history lines the
validator requires as the neighbouring tests in
metasystem/internal/goal/verbs_test.go do.

## HPT-02: the scenario's human assertions pass --lineage and so prove nothing

In the proof-grades scenario, `human_runs release`, `human_runs park` and
`human_runs unpark-to-queued` pass `--lineage human-terminal`. Before
this change the builder proved human ancestry only when it had to
derive a lineage itself, so with a lineage supplied those three
assertions pass on the untouched tree. Drop --lineage from every
human_runs call so the builder must prove and derive the lineage, and
assert the derived terminal-<id>-0 lineage where the goal record shows
it. The agent_refuses calls keep their agent-shell lineage.

## HPT-03, while the file is open

In metasystem/internal/refusal/register.go move the TERMINAL_NOT_ENROLLED
row before TERMINAL_NOT_REACHED so the block stays alphabetical.

## The proof-grades scenario's holder is killed and its ownership cannot be proven

In metasystem/scripts/agents/goal-cli-fixtures.sh your new scenario
starts a holder, `"$proof_grade_holder_command" 300 &` at line 339
(the fake agent binary under the fixture's tmp), then runs the
proof-grades script under `/usr/bin/script` for a pseudo-terminal. Seat
side, on macOS, the run ends with "line 431: 95967 Killed: 9
\"$proof_grade_holder_command\" 300" and cleanup reports "could not
prove ownership of proof-grade holder pid 95967". The holder is
SIGKILLed before cleanup, most likely when the pseudo-terminal session
ends and its process group is torn down, and the fixture's ownership
check then finds no such command. Make the scenario hold its own
terminal session leader deterministically: either start the holder
inside the `script` session so it is the session leader the walk must
reach and let the session's end reap it, or start it outside with its
own process group (setsid, and note the platform's own tool) and prove
ownership by pid and start time rather than by command text, and kill
it yourself. Then the scenario asserts what the goal needs: an
unenrolled human parks, releases, unparks and session-stops and is
refused for approve with TERMINAL_NOT_ENROLLED named; an agent shell is
refused for all of them; a headless caller is refused. The
orchestrator will replay the bed; say which run proves the scenario.

## Not in scope

The brain-stop scenarios; the refusal texts; any row beyond the four.

## Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l . (empty)`;
`go test ./internal/humanauthority/ -count=1` green;
`go test ./internal/goal/ -run 'Unpark|Grade|Authority|SessionStop' -count=1`
green; `bash -n scripts/agents/goal-cli-fixtures.sh`; say plainly what
else your sandbox could run.

## Constraints

Wall-clock budget: 40 minutes; return before it ends even if something
is red, naming it. DESIGN-BEARING reach (tier 3). Declare the boundary
as every file that differs from main. Stop adding at a gap that needs a
decision no page has made; keep what is built and green; report the
gap with the resolution you propose. Never delete built work.
