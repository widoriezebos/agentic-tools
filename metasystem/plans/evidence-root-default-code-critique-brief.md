# Code critique brief: evidence-root-default (Sol)

You are the code critic (Codex, Sol). Review the implementation on branch
`evidence-root-default`, in the worktree `/Users/wido/LocalStorage/GitHub/agentic-tools-erd`.
The diff to review is `git diff 939e73962..HEAD -- . ':!metasystem/plans'`, excluding
the merges of main: judge only this branch's own commits (80a8fa9b7, e11d4ce89, 79cd1234a,
617727d58, 1b9600fae, 0e35917a1, 498b80c4c and the report commit). This is read-only:
don't edit, commit, or run anything that writes outside a scratch directory. Never read
any `metasystem.conf.local`.

- **Spec:** `metasystem/plans/evidence-root-default-design.md` (revision 4, with
  Dispositions). The builder's deviations are in
  `metasystem/plans/evidence-root-default-build-report.md`. Judge each deviation on
  whether it is safe and whether it keeps the design's intent.
- **Materiality (R-124):** a finding is material only when the slice does not WORK or is
  not SAFE without the fix. Everything else is deferred. The threat model is the
  design's: evidence written inside a checkout or into another checkout's root; an
  explicit setting silently replaced; a reader with a private fallback; the launch
  isolating nothing; a broken pinned check or ratchet.
- **Also check** that no test writes under the real `$HOME` (tests must use the
  `LookupEnv` or `HOME` seams).
- **Budget:** 50 tool calls. One round, then one fix round; there is no second read.
- **Return:** findings `SOL-ER-NN`, each with severity (material or non-material), the
  evidence (path:function, what you read), and the smallest fix. Then Deferred. End with
  exactly one line: `VERDICT: <n> material`.
