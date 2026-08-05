# Held-out grader

Nothing here is ever copied into the repository the agents work in.
Provisioning copies `../spec.md` and `../seed/` only.

Not yet built. Item S-C in `plans/benchmark-spec-bm1-design.md` builds:

- the acceptance suite, one or more tests per requirement of `../spec.md`
- the careful reference implementation
- the flawed variants, as patches against the reference
- the mutant corpus, as patches against the reference, each declaring the
  requirement it targets
- the checks that emit `metric=<name>=<value>` lines in the mission gate grammar
