Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal code-critic-runtime-has-no-shell, tier 3, hazard DESIGN-BEARING, code critique of chain ccs-build1 round two)
Date: 2026-09-06

# Review brief: code-critic-runtime-has-no-shell, round two

Round budget: three review rounds for the goal's tier-3 box; this is
round two. The orchestrator adjudicates every finding; you edit nothing.

Threat model and scope as in round one
(metasystem/plans/code-critic-runtime-has-no-shell-critique-r1-brief.md).
Round one (ccs-critic1) returned two material findings, both also found
by the orchestrator's live probe of the candidate envelope and both
folded in round two from
metasystem/plans/code-critic-runtime-has-no-shell-fold-r2-brief.md; the
dispositions are in
metasystem/records/misc/code-critic-runtime-has-no-shell-critique-r1-dispositions.md.
You review the WHOLE chain diff (rounds one and two together, the
computed diff of implementer job ccs-build1-r2 against the chain's
base). Reviewed tree (conformance, review stage, round two): b7d5a0f81ae32cb3a2e39e3e370133e13a4222d4. The orchestrator repeated the live probe with the round-two engine (its own adapter claude-settings with --scratch and adapter claude-command for a code-critic record, cwd the reviewed worktree, GOCACHE and GOTMPDIR under the scratch directory): go test -count=1 ./internal/refusal passed; a touch in the reviewed tree and a touch in the live repository root were refused "Operation not permitted"; a touch in the scratch directory succeeded. One nuance for you: inside the delegate the Bash tool reports TMPDIR=/tmp/claude-501, so the adapter export of TMPDIR does not reach the shell (the CLI sets its own writable one); GOCACHE and GOTMPDIR do reach it.

# Goal

Say whether the change ships a defect, violates its brief, or damages
what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to attack

1. The denyWrite list in BuildClaudeSettings
   (metasystem/internal/adapter/claude.go): for the critic set with
   empty write roots it names the workspace root and every requested
   read root, absolute and deduplicated, and nothing else; the paths
   are the same ones the argv's --add-dir carries (ClaudeReadRoots);
   a record with no workspaceRoot or no readRoots does not produce an
   empty or partial list silently.
2. The scratch directory: the adapter
   (metasystem/scripts/agents/adapters/claude.sh) and the settings
   builder name the SAME path (through the `--scratch` flag on
   `adapter claude-settings`, metasystem/cmd/metasystem/adapter_runtime_verbs.go);
   it lies outside every repository root; allowWrite = write roots plus
   scratch for every role; TMPDIR, GOCACHE and GOTMPDIR are exported
   under it in the launch subshell; the directory is created before
   the exec; a missing --scratch adds nothing.
3. Precedence and leakage: a scratch directory nested inside a denied
   root would be dead (deny wins), so confirm it cannot be nested; the
   scratch path is per job and round, so two rounds cannot share a
   cache by accident; nothing of the live root's caches is reachable.
4. Bash stays sandboxed: the three sandbox switches (enabled,
   autoAllowBashIfSandboxed, allowUnsandboxedCommands false) are
   unchanged; the comment at the tool-list selection now states the
   true reason a shell is safe.
5. Tests in metasystem/internal/adapter/runtime_test.go and
   claudecommand_test.go: the critic case asserts denyWrite and
   allowWrite exactly; the implementer case asserts allowWrite = write
   roots plus scratch and no denyWrite; no existing test changed; none
   vacuous.
6. Conformance: only the brief's May-touch files; nothing under plans,
   no other adapter, not dispatch.sh, not the role packets beyond
   round one's sentence.

# Evidence you may run

If your runtime gives you a shell, from the reviewed worktree root (the
metasystem directory): `go test -count=1 ./internal/adapter ./cmd/metasystem`,
`go vet ./internal/adapter`, `gofmt -l ./internal/adapter`,
`bash -n scripts/agents/adapters/claude.sh`. Say whether you had a
shell. Do not edit anything.

# Expected Return

The code-critic role's version-3 JSON with every required property,
the reviewed tree hash carried exactly, findings only, each with a
stable id, severity, a boolean material value, a plain-English claim
and evidence marked ran, read or inferred; one rigor row per material
finding. A round with zero material findings is the closure candidate;
say so plainly and list what you checked.

# Gap Rule

stop and report a gap; never fill it silently.
