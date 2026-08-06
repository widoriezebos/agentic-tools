# Local invariants of the metasystem repository itself

These are THIS project's filled-in rules, moved out of the shipped template
when the comb found the template carrying our model names and incident
history into every adopted repository (2026-08-06). The template keeps the
portable, anonymous forms; this file keeps ours.

- Two rosters exist and must never be confused. **Development** of the metasystem and its kit runs Claude for design and coordination (Fable when genuinely complex, Opus otherwise) and Codex `gpt-5.6-sol` at xhigh for implementation, because correctness is worth paying for. A **benchmark run** is the opposite: it measures the metasystem, repeats, and must stay cheap, so it hosts on Opus or Sonnet with `gpt-5.6-luna` delegates, configured in the spec's own manifest roster. Applying the benchmark's cost ceiling to development work cancelled three healthy jobs on 2026-08-05.
- A commit is gated on the suite run that produced its verdict, in one shell chain: `scripts/validate-metasystem.sh && ... && git commit && git push`. Never read a verdict from a log tail, a previous shell, or a check whose failure does not stop the chain. Two pushes on a red suite happened in one day, both that way (IL-17).
- A receipt is appended in the same commit as the work it describes. Bookkeeping-only commits hide the ratio of records to evidence, which is the retro's own inversion test (IL-19).

