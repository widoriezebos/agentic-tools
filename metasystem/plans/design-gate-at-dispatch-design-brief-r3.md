# Design brief: design-gate-at-dispatch

## Revision

Revision: revision 3 of plans/designs/design-gate-at-dispatch.md

Reason: Codex on Astra's critique round 2 (job design-critic-ba61e618ee427038e15c207a-r2) returned one material finding, accepted by the seat. Every other part of revision 2 stands; change only what this finding needs.

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check. Paths are relative to the
installation `metasystem/` of `/Users/wido/LocalStorage/GitHub/agentic-tools-m1k` (main at `b03b29ff1` or later).

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

The complete prior revision is the page itself, `/Users/wido/LocalStorage/GitHub/agentic-tools-m1k/metasystem/plans/designs/design-gate-at-dispatch.md` (revision 2, on main at `1522add72`). Revise it in place.

Critique findings being answered:

1. DESIGN-GATE-RECORD-IDENTITY (material, high). "Dispatch records from different repositories can overwrite each other. The proposed computer-wide filename uses only the goal and unit names. Two projects building a goal named setup with the default unit main therefore write the same file. Keeping the worktree inside that file does not prevent replacement." Evidence: the page's line 67 (the record path `~/.metasystem/unit/.design-gate/<goal>/<unit>.json`) and line 153; `internal/launch/unit_named.go:308` keeps identically named units of different repositories apart and its locks use that identity at `:144`; `internal/launch/unit_named_test.go:189` covers two worktrees sharing one store.

The seat's decision (decisions file `artifacts/agents/design-critic-ba61e618ee427038e15c207a/rounds/2/decisions.md`): key the dispatch record by the project's goal-ledger identity as well, `.design-gate/<ledger-identity>/<goal>/<unit>.json`, so the checkouts and the lane of one project on this computer share it (the landing check finds it by goal) and different projects never collide; a test builds the same goal and unit name in two beds with different identities and finds both records. Establish which accessor yields that identity (the goal ledger's tree identity: `internal/goal/fetchadvance.go:122` `treeIdentityFor` is the comparison the accepted-ref repair uses; there may be a cheaper exported reader of the root record's identity), say what the gate does when the identity cannot be read (the `unchecked` path: the record is not written, the fifth pair is printed, the build goes on), and say what a later build of the same goal and unit in the same project does (it replaces the record: the latest dispatch is the one a landing compares against). Update the unit 1 test list and section 6's new effects accordingly.

Also, in section 11, name where Wido's relayed answers are recorded: peer messages from seat m1e to m1k on 2026-10-03, ids d-736ef4245428a4aaa24f1e34c8 (the governance rows, the two-line messages, rows through the hand-in note) and d-583da87ff9a7a58510a21da4c7 (unit 1 first; the budget), and ruling R-145-m1e in `memory/rulings.md`. Add a line to section 10 for this finding.

Cited code excerpts: open them yourself at the lines above.

Example page:

The prior revision itself.

## Recurring findings

none recorded

## Tool-call budget

Maximum delegate tool calls: 25

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: 3800 words

Cut a draft that exceeds this ceiling. If cutting would make the page
incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique
only through the context pack above. Never resume a delegate from an earlier
revision.

## Page artifact and return shape

Write the page to: /Users/wido/LocalStorage/GitHub/agentic-tools-m1k/metasystem/plans/designs/design-gate-at-dispatch.md

Keep the head (same Id, `Status: draft`). Update the revision line to revision 3, naming the finding folded; mark each changed passage "(revision 3)". Run `bin/metasystem design review /Users/wido/LocalStorage/GitHub/agentic-tools-m1k/metasystem/plans/designs/design-gate-at-dispatch.md --check-only` before returning.

Return only these two lines:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1k/metasystem/plans/designs/design-gate-at-dispatch.md
DESIGN: ready (N words, N the page's word count)
```

or:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1k/metasystem/plans/designs/design-gate-at-dispatch.md
DESIGN: blocked (the reason)
```

When blocked, stop and return `BLOCKED:` with the refusal's first line, what you tried and the decision needed; your seat asks.
