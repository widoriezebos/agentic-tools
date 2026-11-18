# Recorded batch-planner misses

`manifest.json` pins the nine original failed attempts, base and candidate commit,
source evidence, original failing package/test, and expected current policy route.
The source records are the `panes/impact-20261117` bundle used by the design
`a-batch-is-proved-by-what-it-can-affect.md` (18 November 2026 amendment).
The original 31 October failure belongs to `internal/goal`; its later move to
`internal/testenv` is a mapping, not a replacement failure identity.

Each attempt JSON records the complete repository diff with rename detection off,
including both sides of renames and deleted paths. No existence filter is applied
at the recording head or the test's checkout. The matching `-contract.json` is the
original candidate contract, verified by its SHA-256. `full_command` comes from
that candidate's committed configuration. Eight fixtures take the full route;
those routes never call the impact adapter or expand package tests.

The November 11 impact fixture also records the production `goadapter.UnitImpact`
result (named mode), plus the candidate's native untagged and `batchtest` test
inventories. Its GOOS and GOARCH are recorded in the impact value. These are
recorded host inventories, not a claim that the historical tree was tested on
all platforms. Tests materialize declaration-only Go files from those inventories
and run the real `batchPlan`, consumer selection, prerequisite closure, and
expansion. The bodies are never executed. Git and impact are injected through
`impactSources`; the test does not read Git history or run the impact algorithm.

The historical contract predates the batch roster. `roster_overlay` explicitly
records the four D3 groups derived from the candidate's actual inventory: command
audits, command verb/bed coverage tests, whole testpolicy, and goal/testutil source
audits. Only this policy overlay is appended; the original contract is unchanged.
This prevents current test names and later package moves from being silently
substituted into an old tree. The separate empty-impact test proves unconditional
roster selection without pretending to replay history.

Regenerate explicitly from a checkout containing the original objects, from
`metasystem/` (use an unused scratch path inside that checkout):

```sh
GOFLAGS='-p=4' go run -p 4 ./cmd/metasystem/testdata/batchmisses/generate.go \
  -repo .. -scratch ../impact-fixture-recording
```

The generator creates a standalone local clone in that scratch directory and
never pushes or alters the source checkout's index. Missing objects, unresolved
original failure identities, missing commands, and unexpectedly empty impact are
errors. It refuses a candidate tracking private configuration. Remove the owned
scratch directory after inspecting the generated diff. Normal tests never run the
generator. Recording uses the production adapter in the invoking source tree;
this fixture was first recorded against the adapter at `fa383e7b36`. Re-recording
after changing that adapter is a deliberate evidence update, not an automatic
way to repair a failing planner test.
