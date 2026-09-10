Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-09

# Fold round four: the proof-grade holder's stdin must be a pipe, not a FIFO

This is a follow-up round on chain hp-terminal-build1-20260909. Your
worktree carries round three. One defect remains, found by the seat gate
on macOS; nothing else in the tree is in question.

## The defect

scripts/agents/goal-cli-fixtures.sh, scenario proof-grades: round three
feeds the pseudo-terminal holder through a named pipe (mkfifo, held open
on descriptor 9). On macOS that kills the holder before it starts:

    script: tcgetattr/ioctl: Operation not supported on socket

XNU implements a FIFO as a pair of sockets, so script(1)'s tcgetattr on
its stdin fails with EOPNOTSUPP instead of ENOTTY, and script exits with
that error. The scenario then reports "proof-grade holder did not start".
Seat evidence: the holder log in the gate's preserved failure directory
holds exactly that one line.

Proven on this machine, same script(1), same holder shape:

- FIFO on stdin: script dies with the line above.
- /dev/null on stdin: script sends ^D into the pseudo-terminal (the
  round-two defect).
- An anonymous pipe on stdin, kept open by a long-lived process
  (`sleep 300 | script -q /dev/null /bin/bash holder.sh`): the holder
  starts, owns a terminal, and survives its full life.

Linux pipes and FIFOs both answer ENOTTY, so the pipe works there too;
one shape for both platforms.

## Mandate

1. Replace the FIFO with an anonymous pipe whose write end is held by a
   keeper process that lives at least as long as the holder (a plain
   sleep copied from /bin/sleep is fine, but do NOT name it
   metasystem-fake-agent: the leak check counts those). Remove the
   mkfifo, the descriptor-9 open, and the `proof_grade_holder_input_open`
   bookkeeping that existed only for the FIFO.
2. The keeper is reaped wherever the holder and its script session are
   reaped today, on every exit path, so the bed leaves no process behind.
3. Both platform branches use the pipe. The Linux branch keeps
   `--return`.
4. Nothing else changes. Round three's other work stands; the earlier
   rounds' certified behaviour must not move.

## Proof

Your sandbox cannot enumerate processes, so you cannot run the scenario
to a pass; run `bash -n` on the script and say so in the return. The
orchestrator runs the goal-cli bed and the hook suite on the seat before
the critic sees the round. Report the round as your own.

## Constraints

Wall-clock budget: 15 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
