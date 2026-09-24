COMMON RULES for a headless DESIGN delegate launched by seat <seat> on <date> (Claude Fable, own process, uncapped window).
- Work from <repository root> (R0); the engine and ledger are R0/metasystem. Read R0/metasystem/AGENTS.md and the design routing in wow.md first.
- Your brief skeleton is R0/metasystem/scripts/agents/templates/design-brief.md; follow it. Your goal record: `bin/metasystem goal show --id <goal>` from R0/metasystem. The Intent's DONE is the round boundary: design exactly it, no more; name out-of-scope explicitly.
- Build against the accepted revision: grep R0/metasystem/plans and records/misc for your goal id and read the page that supersedes before designing.
- Every claim about code carries file:line. Every rule gets a witness that fails without it and a named mutation. Units at most 300 changed lines each, in landing order, each with its witness. Builds will be Codex gpt-6-sol in a worktree, reads Opus, critique by another model; say which unit is first.
- Write ONLY your design page (path in your brief). Do not edit other files, build engines, run test suites, touch the testrun lock, claim or release goals, or commit. Machine load is shared: no go test runs.
- Respect the tool-call and word ceilings in your goal's Next step (defaults: 60 tool calls, 4000 words). A launching brief may override these defaults.
- FINISH: the last message is exactly one line `DONE <tag> <page path> <open questions for the seat: count>`. If a Stop hook asks for more work afterwards, reply that the design brief is complete and stop.
