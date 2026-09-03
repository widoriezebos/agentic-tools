Working Mode: build
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Correction round on your slice-1 build (job path-class-build1, reviewed
tree 15b186fa): one defect the orchestrator reproduced with an
installation-relative binary, plus the fixture that would have caught it.
Same design (metasystem/plans/path-class-manifest-design.md revision 2),
same boundary; touch nothing outside it.

# The defect, reproduced

With the engine built into the installation's bin directory and run from
INSIDE the metasystem directory (the installation root, which is where
every seat and every script runs it), a relative input is keyed as
repository-relative and answers unclassified:

    cd <installation>            # the metasystem directory
    metasystem path class --explain internal/goal/txn.go
    -> unclassified row= key=repo:internal/goal/txn.go mode=template

    cd <repository root>
    metasystem path class metasystem/internal/goal/txn.go
    -> behavior

Both name the same file. Section 1 of the design says the lookup key is
installation-relative for paths inside the installation and
repository-relative outside; the resolver must first resolve the input
against the caller's current directory to an absolute path, then decide
which root contains it, then derive the key. The path owner verb does
resolve its input; mirror it.

# The change

1. internal/pathclass/pathclass.go: ResolvePath (or the helper it uses)
   resolves the input against the current directory to an absolute,
   cleaned path before choosing the namespace; a path inside the
   installation keys as install:<relative to installation>; a path
   outside the installation but inside the repository keys as
   repo:<relative to repository root>; anything else is outside.
2. internal/pathclass/pathclass_test.go: TestResolveSameFileThreeInputForms
   asserts one class (behavior) for internal/goal/txn.go named (a)
   relative to the installation directory with the process cwd set
   there, (b) as metasystem/internal/goal/txn.go with the cwd at the
   repository root, (c) absolute; and the same three-way agreement for a
   record path (plans/<file>) and a runtime path (artifacts/<file>).
3. scripts/agents/path-class-fixtures.sh: the real-command fixture runs
   the verb from inside the installation with a relative path and
   asserts behavior for internal/goal/txn.go and record for a plans file.
4. If the landing evaluator's floor or carriage lookup in
   internal/landing/observe.go derives keys from changed paths by the
   same route, confirm by a test that a changed path in the form git
   reports (repository-relative, metasystem/ prefixed) resolves to the
   install namespace; add the test if it is missing.

# Gate

`go test ./internal/pathclass/ ./internal/landing/ ./cmd/metasystem/ -count=1` green; `bash scripts/agents/path-class-fixtures.sh` green; `gofmt -l` empty; `go vet` clean on the touched packages.

# Constraints

Wall-clock budget: 45 minutes. DESIGN-BEARING reach (correction at high effort). R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the files named in The
change.

# Gap Rule

stop and report a gap; never fill it silently.
