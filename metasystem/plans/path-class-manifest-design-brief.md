Working Mode: design
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Author the design for goal path-class-manifest: read
metasystem/plans/goals/path-class-manifest.md first; its Intent carries
Wido's order verbatim, the problem with evidence, the four classes and
the done criterion. Write exactly one NEW file named
path-class-manifest-design.md in the metasystem plans directory. Keep it
SHORT: this is one manifest, one verb answer, three consumers, three
deletions, one refusal, and fixtures. Under 300 lines. Every claim about
the tree cites file and line. A design longer than the problem is a
finding against itself.

# Workspace

The delegate worktree the dispatcher created for this job. Read anything;
write only that one new design file.

# The seams, as they are today

- The never-direct-fix floor: metasystem/internal/landing/observe.go,
  function neverDirectFix (around lines 792-812), hard-coded exact names
  and directory prefixes; its two callers at around lines 455-475
  (register carriage) and 775-790 (exact revert).
- Register carriage: metasystem/scripts/agents/register-carriage-paths.txt
  (four entries) loaded by loadCarriagePolicy in the same file; the class
  table metasystem/scripts/agents/landing-classes.json; the promotion file
  metasystem/scripts/agents/landing-promotion.json.
- The critique-waiver rule: metasystem/scripts/agents/instruction-bearing-paths.txt
  and its readers (search scripts/agents and internal for the file name).
- The ownership oracle: metasystem path owner (find its Go owner under
  metasystem/internal and its verb in metasystem/cmd/metasystem) and the
  placement rule in metasystem/plans/memory-architecture-design.md
  (docs, memory, plans, records; the goal engine as sole mutator of goal
  files).
- The goal ledger writers: metasystem/internal/goal (txn.go, the verbs)
  commit plans/goals files themselves.

# What the design must settle

1. THE MANIFEST. One committed file, its location and format (decide
   between a JSON table beside landing-classes.json and a plain-text
   pattern list beside the two lists it replaces; say why), listing every
   top-level path or pattern of the repository with exactly one of four
   classes: behavior, record, ledger, runtime. Give the complete initial
   table for this repository, including the root files (AGENTS.md,
   CLAUDE.md, wow.md, metasystem.conf, go.mod and friends, .gitignore,
   .gitattributes), the harness directories (.claude, .codex, .agents,
   .github), and development/ at the toplevel. Matching rule: longest
   prefix wins; a path matching nothing is unclassified.
2. THE VERB. metasystem path class <path> answers the class from the
   manifest (and refuses an unclassified path with a text that names the
   manifest and the nearest classified ancestor), beside the existing
   path owner verb; state the output shape for scripts (one word) and
   for humans.
3. THE THREE CONSUMERS, each reading the manifest and nothing else:
   (a) the landing evaluator: behavior paths take a reviewed chain only;
   record paths land under register carriage; ledger paths land only as
   goal-verb commits (say how the evaluator recognizes a goal-verb commit
   today); runtime paths refuse to land at all; an unclassified path
   refuses with the verb's text. (b) the critique-waiver rule: a waiver is
   never allowed on a behavior path. (c) register carriage: its allowlist
   becomes "every record path". Specify the exact functions that change
   and what each returns, with file and line.
4. THE DELETIONS: the neverDirectFix table, register-carriage-paths.txt,
   instruction-bearing-paths.txt, and every reader of the two files;
   list them with file and line and confirm nothing else reads them
   (search the fixtures too).
5. RECORD SEMANTICS, stated once: which record files are append-only
   (the digest, receipts, the rulings register), which are revised in
   place by the stream that owns them (a design during its own chain, a
   handoff by its seat), and what the evaluator checks for each (an
   append-only file may only grow; a stream-owned record may change only
   under the goal that owns it, named in the commit's Goal-Item line).
6. THE SECOND BAR AFTERWARDS: state exactly what promoting
   direct-fix-floor-refused means once the manifest is the source, and
   that promotion needs only the one promotion line plus a ruling.
7. FIXTURES, deterministic: one path per class lands or refuses as
   specified; an unclassified path refuses; the waiver refusal on a
   behavior path; the append-only check; the three deletions leave no
   reader (a grep fixture). Name them in the style of
   metasystem/scripts/agents/land-fixtures.sh and the Go tests beside
   observe.go.
8. SIZE: one build slice under 240 reserved minutes if it fits, else two;
   name the diff boundary exhaustively.

Self-grade per the house rule: confidence, weakest claim, reject condition.

# Stop criterion for this chain (Wido's word)

A review finding is material only if it changes what gets built, and the
reviewer must name the artifact that changes. Design, one review, one
fold, one closing review, then build. Write the design so a reviewer can
check every sentence against the tree.

# Constraints

Wall-clock budget: 35 minutes. Design only; no builds, no benchmarks
(R-31). Edit nothing but the design file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the one design file
named under Goal.

# Gap Rule

stop and report a gap; never fill it silently.
