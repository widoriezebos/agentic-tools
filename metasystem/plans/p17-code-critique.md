Four material findings.

1. **HIGH — The check misses the exact defect it claims to prevent.**  
   [runtime_placement_test.go:62](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/runtime_placement_test.go:62) searches for `\b<runtime>\b`. There is no word boundary inside `DevinPermissionMode`, `TestDevinSettle`, or `WriteFakeReturn`, so all evade the check—contradicting its claim at [line 18](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/runtime_placement_test.go:18). Directly probing the implemented expression confirmed all three misses.  
   Minimal fix: tokenize Go identifiers and compare CamelCase/snake-case components; add regression cases for these three names. Rename neutral `fakeEnv` to avoid confusing it with the registered fake runtime.

2. **MEDIUM — One cross-runtime dependency remains in the audited seam.**  
   [devincollect_test.go:42](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/devincollect_test.go:42) constructs a fake-runtime job, and [line 49](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/devincollect_test.go:49) calls fake-runtime production helper `WriteFakeReturn` from a Devin-owned test. This unchanged file violates item 17’s “move each stray” mandate and makes Devin collection tests depend on fake-runtime behavior.  
   Minimal fix: construct a schema-valid Devin return using a neutral test helper or local fixture and a Devin job record.

3. **MEDIUM — The text filtering hides semantic code and mishandles the promised exemptions.**  
   [runtime_placement_test.go:27-28](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/runtime_placement_test.go:27) uses line regexes instead of Go syntax, then erases every string at [line 55](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/runtime_placement_test.go:55). A wrong binding such as `RegisterRecoverer("claude", ...)` in Codex code is therefore invisible, although those literals select runtime behavior—the real shape appears at [codex.go:21](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/usage/codex.go:21). Conversely, block comments and multiline raw strings are not properly exempt.  
   Minimal fix: use Go’s parser/token positions; ignore actual comments and fixture data, but inspect runtime-selector, path, and command literals.

4. **MEDIUM — The durable check omits two areas item 17 explicitly names.**  
   The mandate includes command verb files and `scripts/agents/adapters/*.sh` at [backlog-notes.md:309](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/backlog-notes.md:309), while the globs cover only three internal Go packages at [runtime_placement_test.go:21](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/runtime_placement_test.go:21). The current shell adapters read clean, but that state is not protected against recurrence.  
   Minimal fix: extend enforcement to runtime-named command and shell-adapter files using language-appropriate tokenization; exempt neutral shared plumbing.

The parser body is an exact move, its wrapper preserves the exported signature, and the relocated tests/helpers retain their bodies and discovery context. Focused Go tests could not run because the read-only sandbox denied creation of Go’s temporary build directory.

REVISE
