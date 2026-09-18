# Design brief: <page name>

## Revision

Revision: <first draft, or revision N of the named page>

Reason: <why this draft or revision is needed>

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check.

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

<insert the diagnosis for a first draft, or the complete prior revision for
revision N>

Critique findings being answered:

1. <finding, or "none" for a first draft>

Cited code excerpts:

1. `<file>:<start>-<end>`

   ```text
   <insert the cited excerpt>
   ```

Example page:

<insert one page whose structure or level of detail is the example>

## Recurring findings

Filled by the seat from the records of goal retro-flags-repeated-critique-findings for the goals this design touches, or the words `none recorded`.

<recurring finding classes, or none recorded>

## Tool-call budget

Maximum delegate tool calls: <N>

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: <N> words

Cut a draft that exceeds this ceiling. If cutting would make the page
incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique
only through the context pack above. Never resume a delegate from an earlier
revision.

## Page artifact and return shape

Write the page to: <absolute page path>

Return only these two lines:

```text
<absolute page path>
DESIGN: ready (<words> words)
```

or:

```text
<absolute page path>
DESIGN: blocked (<reason>)
```
