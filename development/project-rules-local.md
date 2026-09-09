# Local invariants of the metasystem repository itself

These are THIS project's filled-in rules, moved out of the shipped template
when the comb found the template carrying our model names and incident
history into every adopted repository (2026-08-06). Shared defaults live in
`metasystem/metasystem.conf`; this file keeps the development-specific rules.

- Two rosters exist and must never be confused. **Development** of the metasystem and its kit uses Codex `gpt-6-astra` at xhigh for design authoring and Codex `gpt-5.6-sol` at xhigh for implementation, because correctness is worth paying for. A coordinator using a different model delegates design authoring through the design-mode roster. A **benchmark run** is the opposite: it measures the metasystem, repeats, and must stay cheap, so it hosts on Opus or Sonnet with `gpt-5.6-luna` delegates, configured in the spec's own manifest roster. Applying the benchmark's cost ceiling to development work cancelled three healthy jobs on 2026-08-05.
- Use focused canaries while changing code, then the shared testing contract in `metasystem/docs/project-rules.md` to select the required verification. Complete independent groups across every stage of the admitted selection and repair failures as a batch before another expensive run. Routine low-risk work does not require the full battery. Commit and landing consume matching successful proof without repeated tests or builds. Use the supported failure-stopping wrappers and actual structured results; never infer a verdict from a log tail or an unrelated earlier green.
- Committed implementation and test logic belongs in Go; Bash is for plumbing, as defined in `metasystem/docs/architecture.md`. Do not introduce other implementation or test languages into Git. Temporary local tooling is outside this restriction.
- A receipt is appended in the same commit as the work it describes. Bookkeeping-only commits hide the ratio of records to evidence, which is the retro's own inversion test (IL-19).
