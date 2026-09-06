Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal claude-implementer-read-roots-writable, tier 3, hazard DESIGN-BEARING, code critique of chain cir-build1 round two)
Date: 2026-09-06

# Review brief: claude-implementer-read-roots-writable, round two (first critique)

Round budget: three review rounds for the goal's tier-3 box; this is
round one. The orchestrator adjudicates every finding; you edit nothing.

Threat model: a claude implementer delegate with a shell and a write
root (its job worktree). In scope: any read root the settings leave
writable (the live repository root above all); a write root wrongly
denied (an implementer that can no longer edit its own worktree); a
denyWrite entry that shadows the scratch directory or a nested write
root; the critic shape changed by accident; a test that pins the
wrong list. Out of scope: the codex, devin and fake adapters, the
argv, hostile prompts.

Scope: the computed diff of implementer job cir-build1-r2 against the
chain base (rounds one and two together). Round one denied the read
roots outright and the orchestrator's live probe refused it before any
critique (the sandbox lets a denied ancestor win over a nested allow, so
the implementer's own worktree was refused); round two, from
metasystem/plans/claude-implementer-read-roots-writable-fold-r2-brief.md,
denies the entries around the write root's ancestor chain instead. The brief it implements is
metasystem/plans/claude-implementer-read-roots-writable-brief.md; it
binds. Reviewed tree (conformance, review stage, round two): c9fe326ed29c8219fd16daa928659eb1851e44ba. The orchestrator repeated the live implementer probe with the round-two engine (a real implementer record, cwd its worktree under the repository, 130 denyWrite entries): a touch in the worktree succeeded, go test ran, and a new file in metasystem/, a touch of metasystem/go.mod, a new file under internal/ and a new file in a sibling worktree were all refused "Operation not permitted"; the scratch directory was writable.

# Goal

Say whether the change ships a defect, violates its brief, or damages
what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to attack

1. The denyWrite construction in BuildClaudeSettings
   (metasystem/internal/adapter/claude.go): for a root with no write
   root under it, the root itself is denied; for a root that contains
   a write root, every sibling entry along the ancestor chain is denied
   and no ancestor or the write root is; path containment is on
   resolved paths with a separator boundary (so /a/b does not shadow
   /a/bc); the key emitted only when non-empty; the critic result
   byte-identical to before this chain; what stays open (new entries
   directly inside an ancestor) is said in the comment. Check the
   directory read: symlinked entries, an unreadable ancestor, and a
   write root that is not under any read root.
2. Precedence: with a worktree under the live root (the fleet's
   layout: artifacts/agents/worktrees/<job> inside the repository), is
   the live root denied while the nested worktree stays writable? If
   the sandbox lets deny win over a nested allow, the implementer
   loses its worktree; the orchestrator's live probe below is the
   evidence; judge the code against it.
3. The scratch directory stays allowed and is never denied.
4. Tests in metasystem/internal/adapter/runtime_test.go: the
   implementer case asserts denyWrite = [live root] and allowWrite =
   [worktree, scratch]; the nested-read-root case asserts no denial;
   the critic case unchanged; none vacuous.
5. Conformance: only the two May-touch files; nothing else.

# Evidence you may run

From the reviewed worktree root (the metasystem directory):
`go test -count=1 -run 'Settings|Claude' ./internal/adapter`,
`go vet ./internal/adapter`, `gofmt -l ./internal/adapter`. Do not
edit anything.

# Expected Return

The code-critic role's version-3 JSON with every required property,
the reviewed tree hash carried exactly, findings only, each with a
stable id, severity, a boolean material value, a plain-English claim
and evidence marked ran, read or inferred; one rigor row per material
finding. A round with zero material findings is the closure candidate;
say so plainly and list what you checked.

# Gap Rule

stop and report a gap; never fill it silently.
