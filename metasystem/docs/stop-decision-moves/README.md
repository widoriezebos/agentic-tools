# Stop decision moves

The Stop decision surface records test assertions and fixture literals that
say whether work must stop. The audit discovers every Go `_test.go` file in
the module and every `scripts/agents/**/*-fixtures.sh` fixture bed.

The audit compares the working tree with its landing base. New assertions are
reported and allowed. Removed or changed assertions are refused because a
change is one removed line plus one added line. Reordering assertions inside a
file does not change the surface.

A landing may move an assertion only when the goal ledger permits it and the
landing adds an exact declaration. Generate that declaration instead of
writing it by hand:

```sh
metasystem audit stop-decision-surface --declare \
  --goal <goal-id> --reason '<why this Stop decision moves>'
```

The command writes `docs/stop-decision-moves/<goal>-<digest>.txt`. The audit
checks the ledger goal, reason, digest, and exact multiset of removed lines.
An older declaration cannot authorize a later change.
