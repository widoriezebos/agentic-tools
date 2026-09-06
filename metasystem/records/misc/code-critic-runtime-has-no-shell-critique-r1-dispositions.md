# code-critic-runtime-has-no-shell, critique round one: dispositions

Chain ccs-build1 after its round-one build, critic ccs-critic1 (claude,
claude-fable-5-1, xhigh), reviewed tree
5c9ae989953f8bc99ceb9bba02b20eb7adc66038. The critic had no shell (it
ran on the live engine, before this chain lands). The orchestrator's
seat-side proof: the adapter package, vet, gofmt, bash -n and the
preamble-quotes validation green on the candidate; and a LIVE PROBE of
the candidate envelope (the candidate engine's own `adapter
claude-settings` and `adapter claude-command` for a code-critic record,
launched from a worktree with a probe prompt), which found both
material defects the critic found, and a second probe that found the
remedy the sandbox honours.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F-1 | accepted | Critical and real: the claude sandbox treats the working directory and every --add-dir root as writable when allowWrite is empty (probe: a touch in the reviewed tree succeeded from the tree as cwd and from a scratch cwd with the tree added), and dispatch.sh runs every non-implementer role with the live repository root as its workspace, so the round-one settings would let a critic's shell write the live checkout. The brief's premise was wrong, not the builder's conformance. Remedy proven by probe: sandbox.filesystem.denyWrite naming the tree plus allowWrite naming one private directory outside it refuses a touch in the tree and in the cwd while go test runs. | Folded in round two (plans/code-critic-runtime-has-no-shell-fold-r2-brief.md): denyWrite = workspace root plus read roots for the critic set; one private scratch directory per delegate round in allowWrite. |
| F-2 | accepted | High and real: the sandbox does not let a delegate write the system temporary directory, so the GOCACHE export pointed at a place go could not initialise (probe: "failed to initialize build cache ... operation not permitted"). | Folded in round two: the scratch directory is named in allowWrite and carries TMPDIR, GOCACHE and GOTMPDIR. |
| F-3 | noted | A critic with a shell and the record's network rule of allow can reach outbound side effects the read-only list never offered (a push with stored credentials, an upload). Outside this brief's threat model; real. | none in this chain; backlogged as goal claude-critic-shell-network-deny for the record's requested network rule. |
| F-4 | noted | The settings test should pin the three sandbox switches and, after the fold, the denyWrite list. The fold brief pins denyWrite and allowWrite; the switches are not in it (the amendment missed the landed brief). | the closing critic decides whether the switches need a third round; otherwise recorded here. |
| F-5 | noted | The record's tools grade still says read-only while the critic gets a shell; description drift in dispatch's vocabulary, no enforcement keys on it for claude. | none; recorded here. |
