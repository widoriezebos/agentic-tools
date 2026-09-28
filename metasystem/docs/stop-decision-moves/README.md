# Stop decision moves

The Stop decision surface records test assertions and fixture literals that
say whether work must stop. The audit discovers every Go `_test.go` file in
the module and every `scripts/agents/**/*-fixtures.sh` fixture bed.

The audit compares the working tree with its landing base. New assertions are
reported and allowed. Removed or changed assertions are refused because a
change is one removed line plus one added line. Reordering assertions inside a
file does not change the surface.

A changed assertion whose Stop decision is untouched is reported as
`reworded` and needs no declaration: the removed and the added line are in
the same Go test file and have the same Go tokens, except for string literals
that name no decision (none of block, allow, decision, refusal or spent in
them). Renaming a command inside an assertion is a reword. Flipping a
comparison, changing `block` to `allow`, touching a Stop identifier or its
operand, or deleting the assertion is a move. Fixture-bed lines are never
reworded.

A landing may move an assertion only when its goal is allowed stop-test
changes and the landing adds an exact declaration. Allowing it is a person's
act, run at the enrolled terminal:

```sh
metasystem goal allow <goal-id> stop-test-changes \
  --reason '<why this goal moves a Stop decision>'
```

The goal record then carries the sealed line `- StopSurface: moves`, and
`metasystem goal show <goal-id>` says `Allowed: stop-test changes`. Anyone
withdraws it with `metasystem goal disallow <goal-id> stop-test-changes`. A
person who edits that line by hand publishes it with
`metasystem goal sync --publish --goal <goal-id> --by <name>`, which records
the edit as the allow or disallow it is and asks for the person's proof for
an allow.

Generate the declaration instead of writing it by hand:

```sh
metasystem test declare-moves <goal-id> --reason '<why this Stop decision moves>'
```

The command writes `docs/stop-decision-moves/<goal>-<digest>.txt`. The audit
checks the ledger goal, reason, digest, and exact multiset of removed lines.
An older declaration cannot authorize a later change.
