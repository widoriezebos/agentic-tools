Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-09

# Fold brief: round three of the terminal grade for stopping acts (chain hp-terminal-build1-20260909)

You built rounds one and two. Round two's Go packages are green
seat-side (internal/humanauthority 80.4 percent, internal/goal 82.4
percent, cmd/metasystem green), and the goal-cli bed passes every
scenario except the two brain-stop scenarios that fail on main today
(not yours) and your proof-grades scenario, which fails with no message
at all. The preserved evidence explains both facts:

- proof-grade-holder.log contains exactly `^D`. The holder script
  (`printf "$$" > pid file; exec fake-agent 300`) is started under
  `/usr/bin/script -q /dev/null bash holder.sh` with no real standard
  input, so `script` reads end-of-file at once, ends the pseudo-terminal
  session and the holder with it; the pid file names a process that is
  already gone when the walk needs a live session leader.
- The scenario's assertion output went to that pseudo-terminal, so when
  it failed nothing reached the bed's log or the evidence directory.

Fold these two, and the critic's round-two findings if any are listed
at the end of this brief.

## Keep the pseudo-terminal session alive for the scenario's life

Give the `script` session a standard input that stays open until the
scenario ends: on Darwin, `/usr/bin/script -q /dev/null bash holder.sh
< <(sleep 600)` or a FIFO the scenario writes to and closes at cleanup;
on Linux the equivalent with `-c`. Wait, bounded, until the holder's pid
file exists and its process is alive before running the first
assertion. At cleanup, close the input, then terminate the holder by pid
plus start time (round two's ownership check) and confirm it is gone.
Prove liveness the way the walk will see it: the holder must have a
controlling terminal and be its session's leader; assert both with the
engine's own `proc probe` before the first human_runs.

## Capture the scenario's output outside the pseudo-terminal

Every assertion inside the proof-grades script writes its diagnostics to
a file under the scenario's log root, and the outer bed prints that file
into its own log and into the preserved evidence when the scenario
fails, so a failure is never silent again. Assert at the end of the
scenario that the file exists and is non-empty on failure paths (a
deliberate failing assertion in a self-check run, removed before
return, is enough to prove it).

## Not in scope

The brain-stop scenarios; the refusal texts; any row beyond the four.

## Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l . (empty)`;
`bash -n scripts/agents/goal-cli-fixtures.sh`; `go test ./internal/humanauthority/ -count=1`
green; say plainly what else your sandbox could run. The orchestrator
replays the goal-cli bed seat-side; its proof-grades scenario passing
there is the acceptance.

## Constraints

Wall-clock budget: 30 minutes; return before it ends even if something
is red, naming it. DESIGN-BEARING reach (tier 3). Declare the boundary
as every file that differs from main. Stop adding at a gap that needs a
decision no page has made; keep what is built and green; report the
gap with the resolution you propose. Never delete built work.

## The second critic's findings, folded here

- HPT-04 (material): on Linux, util-linux `script -c` does not return the
  child's exit status unless given `--return`, and the fixture's next
  line is an unconditional `exit 0`, so on the validation host the
  scenario cannot fail. Pass `--return` on Linux, capture the status on
  both platforms, and fail the scenario on a non-zero inner run.
- HPT-05 (material): the headless assertion (the bed's lines 410 to 420
  in the reviewed tree) runs from the bed's own child, which keeps the
  controlling terminal of whoever started the bed, so it is not
  headless; and the loop at lines 400 to 404 deletes every adapter
  signature from the clone except fake.sh, so a real agent in the
  ancestry is no longer recognised. Run the headless case with no
  controlling terminal (setsid on Linux; on Darwin a child started
  through nohup with standard input from /dev/null, and assert with
  `ps -o tty=` that it has none), and keep the real adapter signatures
  in the clone, removing only what the fake runtime needs, with a
  comment saying why.
- HPT-06 (note): restore the old check order in ProveTerminal (agent
  signature first, then the terminal) so the shape "an agent on another
  terminal" keeps its name AGENT_IN_AUTHORITY_CHAIN.
