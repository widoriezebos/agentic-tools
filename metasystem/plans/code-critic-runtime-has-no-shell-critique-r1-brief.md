Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal code-critic-runtime-has-no-shell, tier 3, hazard DESIGN-BEARING, code critique of chain ccs-build1)
Date: 2026-09-06

# Review brief: code-critic-runtime-has-no-shell, round one

Round budget: three review rounds for the goal's tier-3 box; this is
round one. The orchestrator adjudicates every finding; you edit nothing.

Threat model: a critic or warden delegate on the claude runtime is
given a shell. In scope: any path by which that shell can change the
reviewed tree, the repository, the job record, or the certified diff
(a Bash write that the sandbox does not deny; an allow entry that
outranks a deny; a role string that matches the critic set by
accident; a settings shape claude reads differently from what the
builder intends); an implementer whose envelope narrows or widens by
this change; a GOCACHE export that leaks into the live root or into
another job's cache, or that breaks a codex delegate; a test that pins
the wrong shape; the preamble-quotes validation broken by a packet
edit. Out of scope: the codex, devin and fake adapters, hostile
prompts, the claude CLI's own sandbox implementation.

Scope: the computed diff of implementer job ccs-build1 (round one)
against its base. The brief it implements is
metasystem/plans/code-critic-runtime-has-no-shell-brief.md, landed at
e8cf21d1; it binds. Reviewed tree (conformance, review stage): 5c9ae989953f8bc99ceb9bba02b20eb7adc66038.

# Goal

Say whether the change ships a defect, violates its brief, or damages
what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to attack

1. The envelope in metasystem/internal/adapter/claude.go: the critic
   tool list is chosen only for code-critic, design-critic and warden
   with EMPTY requested writeRoots; any other role with empty roots
   keeps the read-only list; any role with roots keeps the full list
   and acceptEdits; the permission mode for the critic set is dontAsk;
   the role is read from the same record field
   metasystem/internal/adapter/adjudicate.go reads and the set lives in
   one place both builders use.
2. The settings in BuildClaudeSettings: for the critic set with empty
   roots Bash is allowed and Edit, Write, NotebookEdit denied; the
   sandbox block is unchanged (allowWrite empty, the bash-if-sandboxed
   rule, the network rule); confirm that a denied Edit cannot be
   reached through Bash in this settings shape, and that nothing in
   the settings widens for an implementer.
3. The comment where the list is chosen says why a shell is safe, in
   plain English, without round or finding references.
4. GOCACHE in metasystem/scripts/agents/adapters/claude.sh: exported
   inside the launch subshell for every claude delegate; the directory
   is created under the system temporary directory and named by job
   and round; TMPDIR, GOPATH and GOMODCACHE untouched; Bash 3.2 clean;
   the codex adapter untouched.
5. Tests in metasystem/internal/adapter/claudecommand_test.go and
   metasystem/internal/adapter/runtime_test.go: the table covers the
   three role shapes, the settings test asserts allow and deny and the
   empty allowWrite, no existing test changed, none can pass
   vacuously.
6. The role packets metasystem/scripts/agents/roles/code-critic.md,
   design-critic.md and warden.md: one sentence, outside every quote
   block; the preamble-quotes validation still passes.
7. Conformance: only files under the brief's May-touch list; nothing
   under plans, nothing in the other adapters or the permission presets.

# Evidence you may run

If your runtime gives you a shell (it may, for the first time, if the
orchestrator landed this change's engine before you ran; say which
you had), from the reviewed worktree root (the metasystem directory):
`go test -count=1 ./internal/adapter`, `go vet ./internal/adapter`,
`gofmt -l ./internal/adapter`, `bash -n scripts/agents/adapters/claude.sh`.
Do not edit anything; the tree is the reviewed tree.

# Expected Return

The code-critic role's version-3 JSON with every required property,
the reviewed tree hash carried exactly, findings only, each with a
stable id, severity, a boolean material value, a plain-English claim
and evidence marked ran, read or inferred; one rigor row per material
finding. A round with zero material findings is the closure candidate;
say so plainly and list what you checked.

# Gap Rule

stop and report a gap; never fill it silently.
