# code-critic-runtime-has-no-shell, critique round two: dispositions

Chain ccs-build1 after its round-two fold, critic ccs-critic2 (claude,
claude-fable-5-1), reviewed tree b7d5a0f81ae32cb3a2e39e3e370133e13a4222d4,
the whole chain diff. Zero material findings: the closure round. The
critic had no shell (it ran on the live engine). The orchestrator's
seat-side proof: the adapter and command packages, vet, gofmt and bash
-n green on the folded tree; and the live probe with the round-two
engine (its own settings with --scratch and its own argv for a
code-critic record): go test ran, a touch in the reviewed tree and in
the live root were refused, the scratch directory was writable.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F-1 | noted | A hand-built record with no workspace root and no read roots yields an empty denyWrite silently; the dispatcher always records a resolved workspace, so no dispatched record reaches it. | none; recorded here. |
| F-2 | noted | A scratch directory nested inside a denied root would leave the critic without a writable cache; the failure direction is safe (no write escape) and no seat sets TMPDIR under a repository. | none; recorded here. |
| F-3 | noted | denyWrite entries are symlink-resolved while --add-dir and the scratch path are passed as spelled; today's records and the seat's TMPDIR make the lists coincide. | none; recorded here. |
| F-4 | accepted as backlog | Every claude delegate round leaves a Go cache and temp directory under the system temp dir and nothing removes them; gigabytes on a suite day. Outside this brief. | goal claude-delegate-scratch-cleanup. |
| F-5 | noted | Two existing settings tests gained an empty fourth argument because the fold added a parameter; assertions untouched. | none. |
| F-6 | noted | The adapter's TMPDIR export does not reach the delegate's shell (the CLI supplies /tmp/claude-501 there); GOCACHE and GOTMPDIR do. The comments describe what the settings grant. | none; the probe result is on the record. |
| F-7 | accepted as backlog | No test pins the four sandbox switches; a flip of allowUnsandboxedCommands would pass the suite. | goal claude-delegate-scratch-cleanup carries the one-line test hardening too. |
