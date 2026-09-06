# code-critic-runtime-has-no-shell: the live proof after landing

Chain ccs-build1 landed at 07d18614; the engine was rebuilt and
re-armed (generation 13); job ccs-proof1 is the first claude code-critic
dispatched by that engine (brief
plans/code-critic-runtime-has-no-shell-proof-brief.md). Its launch
envelope: tools Bash,Read,Glob,Grep, permission mode dontAsk, Edit,
Write and NotebookEdit denied, sandbox allowWrite = the round's scratch
directory, denyWrite = the repository root. Its return: "I had a
shell"; every evidence command ran (go test, go vet, gofmt, bash -n,
the preamble-quotes validation, a byte-for-byte check of the landed
commit against the certified diff); a touch in the reviewed tree, the
repository root, the plans directory and the home directory were all
refused; the scratch directory was writable; GOCACHE and GOTMPDIR
reached the shell, the adapter's TMPDIR export did not (the CLI
supplies its own writable one).

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F-1 | accepted as backlog | Real and material for the next step, not for this goal: the sandbox refuses loopback binding, so the 13 adapter tests that open a listener fail in a critic's shell ("listen tcp 127.0.0.1:0: bind: operation not permitted"); the CLI's sandbox schema has allowLocalBinding, which the settings never set. Focused tests pass. | goal claude-critic-sandbox-allows-loopback. |
| F-2 | noted | TMPDIR export does not reach the shell; known from the round-two probe. | none. |
| F-3 | accepted as backlog | Scratch directories are never removed (120 MB after one adapter test run). | goal claude-delegate-scratch-cleanup (already open). |
| F-4 | accepted as backlog | A host TMPDIR ending in a slash spells the scratch path with a double slash; cosmetic. | carried on goal claude-delegate-scratch-cleanup's next step. |
| F-5 | noted | Two existing tests gained the new fourth argument; assertions untouched. | none. |
