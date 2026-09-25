# Review brief: <chain name>

Round budget: <N focused rounds — agreed before round one; exhaustion
follows the critique skills' budget rules, never a silent round N+1>

Threat model: <who and what this review defends against, stated
plainly. A TRUE finding outside this model closes as out-of-scope,
citing this section — accepted as fact, rejected as work. Example:
"two trusted operators, no adversaries; accidents and crashes are
in scope, hostile inputs are not.">

Scope: <the files, behaviors, or contracts under review — and
what is explicitly OUT, including any agreed ceiling on fix work>

An independent read starts through its intent, never as an in-process agent: `metasystem build` runs a unit's preliminary read after its proof, `metasystem review unit RUN` requests that unit's committed review, and `metasystem review job J` or `review commit SHA --goal G` reads an existing job or commit. A reader launched by one of these reads its frozen subject and never starts another review. A standalone read of a bare diff with no job or commit behind it remains the expert `metasystem internal launch start --kind read --diff-file` path.

## Prepared copy

Path: <absolute path to the reader's private copy>

Contents: <commit SHA, or base and candidate identifying the exact diff>

## Checklist

Each item names its complete read boundary. Use line ranges for source and
document checks, and fixture row names or ranges for fixture checks.

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

No single command may wait longer than 240 seconds; run a longer one in the background with its output to a file and poll the file. A wait that outlives the host turn is registered with `metasystem wait register`.

1. `<file>:<start>-<end>` — <behavior or contract to check>
2. `<fixture file>:<row name or start-end>` — <scenario to check>

## Tool-call budget

Maximum reader tool calls: <N>

Stop when this number is reached. In the findings file, list every checklist
item or part of an item that the budget did not allow you to check.

## Findings artifact and return shape

Write findings to: <absolute findings file path>

Inside that file, number findings from most severe to least severe. Each
finding names the file, rule, and concrete failure it causes. If there are no
material findings, record AGREE and any non-gating observations there.

Return only these two lines; never return the findings themselves:

```text
<absolute findings file path>
VERDICT: land
```

or:

```text
<absolute findings file path>
VERDICT: fix first (<N> material findings)
```

## Scoped confirmation read after a fold

Prepared copy path: <absolute path to the copy containing the fold>

Contents: <commit SHA, or base and candidate identifying the exact folded diff>

Folded findings (this is the whole checklist; do not repeat or widen the
original review):

1. `<finding id>` — `<file>:<start>-<end>` or `<fixture>:<row>` — <folded correction to confirm>

Maximum reader tool calls: <N>

Write findings to: <absolute confirmation findings file path>

Use the same two-line return shape above. Report any folded finding left
unchecked when the tool-call budget is reached.
