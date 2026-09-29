# Code critique brief: public-error-messages (Sol)

You are the code critic (Codex, Sol). Review branch `public-error-messages` in the worktree
`/Users/wido/LocalStorage/GitHub/agentic-tools-emsg`. The diff is
`git diff $(git merge-base HEAD origin/main)..HEAD -- . ':!metasystem/plans'`. This is
read-only: don't edit, commit, or run anything that writes outside a scratch directory.
Never read any `metasystem.conf.local`.

- **Spec:** the audit at
  `/Users/wido/LocalStorage/agentic-tools-evidence/error-messages-20260928/public-audit.md`,
  and the builder's report `metasystem/plans/public-error-messages-build-report.md`, which
  lists one line per EM id.
- **Question per fix:** is the new message TRUE, does it say what went wrong in the
  reader's terms, and does the command it names exist and work? Did any fix change
  behaviour (exit codes, what runs) beyond what the audit's finding required? Check
  especially EM-07 and EM-20, where exit codes or resolution changed.
- **Materiality (R-124):** material only if a message is still false or misleading, a
  named command doesn't exist, or a behaviour change breaks a caller. Wording taste is
  deferred.
- **Budget:** 40 tool calls, one round.
- **Return:** findings `SOL-EM-NN`, each with severity, evidence (path:function) and the
  smallest fix; then Deferred. End with exactly one line: `VERDICT: <n> material`.
