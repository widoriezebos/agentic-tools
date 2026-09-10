# hp-terminal-grade-for-stopping-acts, critique round three: dispositions

Chain hp-terminal-build1-20260909. The third critic
hp-terminal-crit3-20260909 (claude-opus-5) reviewed the fold
hp-terminal-build1-20260909-r5 at tree
4f71dfc121c7091d593af4cddd2b25e3a0205eac and returned two material
findings and three notes. The seat-side gate on the same tree had the Go
packages and the hook suite green and the proof-grades scenario running
end to end for the first time, eight of nine assertions passing; the
ninth (session stop from the scenario's pseudo-terminal, refused "caller
classifies DELEGATE") was the tell for the first finding. Round six
folds all five.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| HPT-08 | accepted | Confirmed with an executed proof: ProveTerminal matches adapter signatures only from the invoker to its session leader and stops, so an agent that opens a pseudo-terminal earns a terminal-grade proof; park, release and unpark accept it, and session stop refuses only through lease.Classify, which walks the whole tree. The goal's own sentence says no agent in the ancestry. | Round six continues the agent-signature walk past the session leader to the root of the process tree, keeps the same-terminal requirement bounded by the leader, records the nodes above the leader in the proof, keeps session stop's gate, makes the scenario assert the allows only on an agent-free bed and expect AGENT_IN_AUTHORITY_CHAIN under an agent seat, and adds unit tests for the new reach. |
| HPT-09 | accepted | Two keeper start-up failure exits run cleanup before the session start was recorded; the session block reads empty-versus-live as an ownership mismatch, kills nothing, and the unconditional wait blocks the bed for the holder's full 300 seconds. | Round six treats an empty recorded session start as never proven, terminates the session and the keeper by their pid-file values on every failure path that has a live pipeline, and bounds the wait by mission-process-wait. |
| HPT-10 | accepted | nohup keeps the bed's controlling terminal on macOS, so a hand-run bed fails the headless step with the wrong sentence; setsid without --wait may fork on Linux and return the parent's zero. Fails loudly, certifies nothing false. | Round six has the headless child make its own session on both platforms through perl's POSIX::setsid and then exec, keeping the in-child terminal check. |
| HPT-11 | accepted | agent_refuses accepts either AGENT_IN_AUTHORITY_CHAIN or "caller classifies DELEGATE", so three agent assertions can pass on the classification gate rather than on the walk this goal builds. | Round six asserts the exact refusal per act: the ancestry outcome for park, release and unpark; the classification sentence for session stop. |
| HPT-12 | accepted | The budget init shells out to bin/metasystem before the bed's own "bin/metasystem is not built" check, so an unbuilt checkout dies inside calibration without the clear line. | Round six moves the built-binary check above the init. |
