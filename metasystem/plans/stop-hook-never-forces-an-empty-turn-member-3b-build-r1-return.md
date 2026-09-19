BUILD: blocked required checks red
## Start state

`git rev-parse HEAD`

```text
1e84763e202245843cca98b1a41265d39f3fb51e
```

`git status --porcelain`

```text
```

Page SHA from `shasum -a 256 /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct/stop-hook-members-design-r2.frozen.md`:

```text
9ba7a005e4336b6313231d319c3c8b7a175cb7158a55e3033956e7fbbdff7461  /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct/stop-hook-members-design-r2.frozen.md
```

## Changed files

`git diff --numstat HEAD`

```text
281	0	metasystem/cmd/metasystem/runtime_conformance_test.go
2	2	metasystem/internal/hooks/degraded_stop_forms_test.go
8	0	metasystem/internal/hooks/testdata/stop-degraded-forms.golden
103	0	metasystem/scripts/agents/adapters/fake.sh
4	2	metasystem/scripts/agents/stop-degraded-forms.sh
35	7	metasystem/scripts/agents/supervision-hook.sh
1	1	metasystem/testing.json
```

`git status --porcelain`

```text
 M metasystem/cmd/metasystem/runtime_conformance_test.go
 M metasystem/internal/hooks/degraded_stop_forms_test.go
 M metasystem/internal/hooks/testdata/stop-degraded-forms.golden
 M metasystem/scripts/agents/adapters/fake.sh
 M metasystem/scripts/agents/stop-degraded-forms.sh
 M metasystem/scripts/agents/supervision-hook.sh
 M metasystem/testing.json
```

No new file exists. The diff is `dm-stopinc-3b.diff` in the return directory.

## Red before green

```text
--- FAIL: TestDegradedStopFormsMatchTheGoldenContract (0.03s)
    degraded_stop_forms_test.go:114: golden has 18 forms; want 26
--- FAIL: TestFakeStopReplay (3.94s)
    --- FAIL: TestFakeStopReplay/FakeReplayUnreadableHookState (0.39s)
        runtime_conformance_test.go:57: arming-1: exit status 2
            stderr=Usage:
              scripts/agents/adapters/fake.sh identity
FAIL
```

The missing `stop-replay` action and the unchanged 18-row golden both failed before production changes.

## Checks

`gofmt -l ./internal/hooks ./cmd/metasystem`

```text
```

`GOCACHE=/tmp/stopinc-3b-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-3b-staticcheck go vet ./internal/hooks/ ./cmd/metasystem/`

```text
```

`GOCACHE=/tmp/stopinc-3b-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-3b-staticcheck go vet -tags batchtest ./internal/hooks/ ./cmd/metasystem/`

```text
```

`GOCACHE=/tmp/stopinc-3b-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-3b-staticcheck go test -count=1 -run 'TestFakeStopReplay|TestDegradedStopFormsMatchTheGoldenContract|TestDegradedStopRendererIsSideEffectFree' ./internal/hooks/ ./cmd/metasystem/`

```text
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/hooks	1.970s
--- FAIL: TestFakeStopReplay (0.75s)
    --- FAIL: TestFakeStopReplay/FakeReplayDeadlineUnreadable (6.80s)
        runtime_conformance_test.go:107: deadline-1 hook diagnostics: stop deadline: worker 30752 left running, command line unverifiable
        runtime_conformance_test.go:130: scan process fixture survivors: identity: enumerate fixture processes: identity: sysctl kern.proc.all: operation not permitted
        fixture.go:360: scan process fixture survivors: identity: enumerate fixture processes: identity: sysctl kern.proc.all: operation not permitted
    --- FAIL: TestFakeStopReplay/FakeReplayUnreadableHookState (10.74s)
        runtime_conformance_test.go:89: scan process fixture survivors: identity: enumerate fixture processes: identity: sysctl kern.proc.all: operation not permitted
        fixture.go:360: scan process fixture survivors: identity: enumerate fixture processes: identity: sysctl kern.proc.all: operation not permitted
FAIL
fixture exit scan: test="TestFakeStopReplay/FakeReplayDeadlineUnreadable" error=identity: enumerate fixture processes: identity: sysctl kern.proc.all: operation not permitted
fixture exit scan: test="TestFakeStopReplay/FakeReplayUnreadableHookState" error=identity: enumerate fixture processes: identity: sysctl kern.proc.all: operation not permitted
FAIL	github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem	12.703s
FAIL
```

`GOCACHE=/tmp/stopinc-3b-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-3b-staticcheck go test -tags batchtest -count=1 -run 'TestFakeStopReplay|TestDegradedStopFormsMatchTheGoldenContract|TestDegradedStopRendererIsSideEffectFree' ./internal/hooks/ ./cmd/metasystem/`

```text
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/hooks	0.681s
--- FAIL: TestFakeStopReplay (0.65s)
    --- FAIL: TestFakeStopReplay/FakeReplayDeadlineUnreadable (8.08s)
        runtime_conformance_test.go:107: deadline-1 hook diagnostics: stop deadline: worker 41190 left running, command line unverifiable
        runtime_conformance_test.go:130: scan process fixture survivors: identity: enumerate fixture processes: identity: sysctl kern.proc.all: operation not permitted
        fixture.go:360: scan process fixture survivors: identity: enumerate fixture processes: identity: sysctl kern.proc.all: operation not permitted
    --- FAIL: TestFakeStopReplay/FakeReplayUnreadableHookState (11.24s)
        runtime_conformance_test.go:89: scan process fixture survivors: identity: enumerate fixture processes: identity: sysctl kern.proc.all: operation not permitted
        fixture.go:360: scan process fixture survivors: identity: enumerate fixture processes: identity: sysctl kern.proc.all: operation not permitted
FAIL
fixture exit scan: test="TestFakeStopReplay/FakeReplayUnreadableHookState" error=identity: enumerate fixture processes: identity: sysctl kern.proc.all: operation not permitted
fixture exit scan: test="TestFakeStopReplay/FakeReplayDeadlineUnreadable" error=identity: enumerate fixture processes: identity: sysctl kern.proc.all: operation not permitted
FAIL	github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem	12.431s
FAIL
```

`GOCACHE=/tmp/stopinc-3b-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-3b-staticcheck go test -count=1 ./internal/hooks/`

```text
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/hooks	5.890s
```

`GOCACHE=/tmp/stopinc-3b-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-3b-staticcheck go test -count=1 -skip 'TestProcFixtureSurvivors|TestProcessCensus|TestCustodianWatches|TestLauncherDeath|TestAllPids|TestProcessesWithWithheldArguments|TestWaitChannelAnswer|TestChannelStatusPostSeedsAnUnbootedBrainStatus|TestTelegramPeekWorksWithoutConfiguredAdapterOrChatID|TestTelegramPeekTokenNeverAppearsInErrors' ./cmd/metasystem/`

```text
ambiguous-session-correlation:a,b
behavior-surface output: write /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestBehaviorSurfaceSelectFailsOnFlushError4004611817/001/input: bad file descriptor
behavior-surface output: write /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestBehaviorSurfaceListFailsOnFlushError696555021/002/input: bad file descriptor
behavior-surface output: write /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestBehaviorSurfaceDirectWritersFailOnOutputErrorpolicy3247009582/001/input: bad file descriptor
behavior-surface output: write /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestBehaviorSurfaceDirectWritersFailOnOutputErrorclassify3863031823/001/input: bad file descriptor
behavior-surface output: write /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestBehaviorSurfaceDirectWritersFailOnOutputErrordigest2024812077/001/input: bad file descriptor
brain start-delivered: the declaration changed before delivery acknowledgment
proc find-ancestor: --all-hosts and --runtime are mutually exclusive
job claim-launch: --root, --opid, --session, --dispatch-mode, --runtime, --model, --role, --launch-mode, --permission-envelope-digest, and --input-hash are required
counselor brief currently requires --dry-run; steward carriage is outside this slice
counselor brief takes no positional arguments
flag provided but not defined: -unknown
Usage of counselor brief:
  -dry-run
    	print the current advisory brief without writing state
  -metasystem-root string
    	metasystem checkout root (defaults to installed binary root)
counselor brief output: closed output
usage: metasystem covenant evidence [--root DIR] [--json]
evidence refused: docs/covenant-evidence.md: docs/covenant-evidence.md does not exist in the tree
usage: metasystem covenant validate [--root DIR]
covenant refused: cannot read /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestCovenantValidateVerb2450994347/002/covenant.json: open /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestCovenantValidateVerb2450994347/002/covenant.json: no such file or directory
covenant refused: the covenant must contain exactly [schemaVersion identity requirements battery budgets guards guardrails]
{"detail":"","outcome":"confirmed","tip":"0a0702fa3bcc393d0631a48d1a9e8f726268510a"}
{"detail":"","outcome":"confirmed","tip":""}
flag provided but not defined: -lock-held
Usage of job breach-stop:
  -goal string
    	goal id
  -revision uint
    	exact accepted goal revision
  -root value
    	checkout root
job id collision: job-cli
job record-cas: --root, --job, --expect, --status, and --patch are required
job critique-register-advance: --repo, --root-job, and --round-job are required
job critique-open-finding-ids: --repo and --root-job are required
job critique-close: --root-job is required
job critique-close: flag --root-job repeated; authority-bearing flags parse strictly
job critique-close: flag --repo repeated; authority-bearing flags parse strictly
goal list: --history and --pretty shape the JSON records; add --json
goal list: --history and --pretty shape the JSON records; add --json
control-plane write requires the authenticated lease holder
goal park does not take --blocks
goal park does not take --under; a power of attorney covers approve, set-budget and unpark
goal open does not take --tiers, --verbs or --expires
no accepted tree; the first fetch or the migration bootstraps it
job cap-continuation: --parent, --worktree and --output are required
job cap-continuation: positional arguments are not accepted
job cap-continuation: a continuation after a cap needs a parent in timeout with budget-cap; the parent is completed with error (none)
{
  "detail": "",
  "identity": "01J5XM00000000000000000000",
  "outcome": "confirmed",
  "tip": "ccf3c3390113fcb3a27f337bd7ba8960383f4739"
}
single-machine mode: multi-machine guarantees are void here; joining a fleet is the backlog-local-promotion goal
continue your claimed goal: standing-validation
advanced=false tip=51f54d759c0c697f69c08a25ff0a1f11d089a0cb already at the canonical tip
claimed=1 approved=0 queued=0 parked=0 done=0 abandoned=0 tip=51f54d759c0c697f69c08a25ff0a1f11d089a0cb
! single-machine mode: multi-machine guarantees are void here; joining a fleet is the backlog-local-promotion goal
0:0 claimed tier 3 standing-validation pin=- claim=mac-cli :: Run it.
{"goal":{"Id":"standing-validation","State":"claimed","Priority":0,"Sequence":0,"Tier":3,"Risk":null,"Intent":"Govern validation.","Origin":"main","StopSurfaceMoves":false,"NextStep":"Run it.","Conclude":"","OpenedAt":"2026-08-30T08:00:00Z","Revision":4,"Blocked":null,"Labels":null,"Arc":"","Pinned":"","Budget":{"elapsedLimit":"4h","attemptLimit":4,"reservedJobMinutesLimit":240,"activeJobLimit":2,"reviewRoundLimit":3},"BudgetExtension":null,"BudgetExceptions":0,"NormApproval":null,"Approved":{"By":"human:Wido","At":"2026-08-30T08:06:00Z","Revision":3,"EpisodeRevision":0,"Opid":"01ARZ3NDEKTSV4RRFFQ69G5FAZ-mac-cli-ca0df2c9","Authority":"proven","Digest":"2183c7e49da620b7583a75ad0303a568269017a0cb18ba2cad3a9f551de3c929","ReviewBy":""},"Sliced":null,"Ratified":null,"Claimed":{"Machine":"mac-cli","Lineage":"m1","At":"2026-08-30T08:05:00Z","HandedOver":{"FromMachine":"","FromLineage":"","FromEpoch":0,"Batch":""},"Revision":2,"AccountingRevision":2,"EpisodeAt":"","EpisodeRevision":0,"EpisodeObligationRevision":0,"IdleSeconds":0},"Obligation":{"Revision":4,"BudgetRevision":2,"State":"DRAFT","Owner":"Wido","AuthorizedBy":"","AuthorizedAt":"","AuthorityOperation":"","ReviewPolicy":"","ReviewOutcome":"","AuthorityOutcome":"TEMPORARY_HUMAN_WORD","AuthorityReviewBy":"2026-09-07","AuthorityRuling":"R-33-m1","TemporaryHumanWord":"Wido renews this obligation","Effects":["authorize-spend"],"AuthorizedEffects":null,"Assumptions":{"Recurrence":"single-experiment","Platform":"darwin/arm64","ToolchainIdentity":"go-fixture","SurfaceDigest":"fixture-surface","MaxActiveJobs":1,"TimingEnvelopeSeconds":60,"ObservationSource":"run-terminal-record"},"Triggers":{"ValueJudgment":"unknown","Reversibility":"reversible","SevereHarm":"no","UnfamiliarApproach":"no","TestDiscrimination":"strong","CorrelatedAssumptionRisk":"no","AuthorityScopeChange":"no","DestructiveReach":"none"}},"ReviewObligations":null,"AcceptedRisks":null,"ReadItems":null,"StopCapability":null,"StopFence":null,"Landing":null,"Episode":null,"Parked":null,"Abandoned":null,"Legacy":null,"History":[]},"root":"/var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestForeignLandedAuthorityKeepsGoalTreeUsable3800564358/001","tip":"51f54d759c0c697f69c08a25ff0a1f11d089a0cb","where":"live","world":"synced"}
goal carry --by takes the human name without the human: actor prefix
usage: metasystem util hold --tag TAG [--stopped-file FILE]
hook configuration event SessionStart has 0 handlers with the expected matcher, type, command, runtime action, timeout, and synchronous behavior; want 1
hook configuration event Stop has 0 handlers with the expected matcher, type, command, runtime action, timeout, and synchronous behavior; want 1
--- FAIL: TestGroupOwnedLiveNonOwnerExitsNotOwned (0.00s)
    identity_probes_test.go:142: process group 10642 ownership outcome = INDETERMINATE, want NOT-OWNED
proc setsid: a command is required after --
proc setsid: exec: "/nonexistent/metasystem-no-such-command": stat /nonexistent/metasystem-no-such-command: no such file or directory
landing batch join gate fast gate: staticcheck: unused assignment
injected lost lease
no machine nickname is enrolled and hostnames are never published: run  git config metasystem.goal.machine <nickname>  once on this machine
no machine nickname is enrolled and hostnames are never published: run  git config metasystem.goal.machine <nickname>  once on this machine
injected construction failure
injected construction failure
test receipt refused: supplied tree f33b7d27317b01340676828baf8df84bfeb698e6 differs from the real index tree 63a2b53e366113bf4904e48f7ab928308abaa66a or working-tree projection 63a2b53e366113bf4904e48f7ab928308abaa66a
reviewedTree=17a273076efa16e30ef60653fe96713a54d15c5b
diffArtifact=/var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestChainLandingRecertifiesAfterBaseMove1026146652/001/integration/metasystem/artifacts/agents/impl/rounds/1/diff.patch
recertification=metasystem/artifacts/agents/landing/recertifications/impl/25b6a0f72f6846774fa6405507d6f074138930b1a8ecdf0cec522d74149830fe/record.json
certifiedTree=014d3e976fbc732ed9a1ccc1b9f8d487af2c9177
diffArtifact=metasystem/artifacts/agents/landing/recertifications/impl/25b6a0f72f6846774fa6405507d6f074138930b1a8ecdf0cec522d74149830fe/diff.patch
recertification=metasystem/artifacts/agents/landing/recertifications/impl/25b6a0f72f6846774fa6405507d6f074138930b1a8ecdf0cec522d74149830fe/record.json
certifiedTree=014d3e976fbc732ed9a1ccc1b9f8d487af2c9177
diffArtifact=metasystem/artifacts/agents/landing/recertifications/impl/25b6a0f72f6846774fa6405507d6f074138930b1a8ecdf0cec522d74149830fe/diff.patch
merge critique accepted with model independence: implementer job impl uses effective model implementer-model, code-critic chain critic uses effective model critic-model, and both agree on tree 17a273076efa16e30ef60653fe96713a54d15c5b
recertification=metasystem/artifacts/agents/landing/recertifications/impl/25b6a0f72f6846774fa6405507d6f074138930b1a8ecdf0cec522d74149830fe/record.json
certifiedTree=014d3e976fbc732ed9a1ccc1b9f8d487af2c9177
reviewedTree=c0873caf71f724fbf4f8d72ea626499e0f7b50fd
diffArtifact=/private/var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestRecertifiedLandingParksOnOriginMove298940763/001/local/artifacts/agents/park-chain/rounds/1/diff.patch
recertification=artifacts/agents/landing/recertifications/park-chain/720f833569438fb017f4e39ca20037eb974643b9c804a2839441ea5a057d416e/record.json
certifiedTree=12b0edb896acb4650b1e7c0a285e57327084b893
diffArtifact=artifacts/agents/landing/recertifications/park-chain/720f833569438fb017f4e39ca20037eb974643b9c804a2839441ea5a057d416e/diff.patch
reviewedTree=7aac5afb7a43ef804f161b55bf83f4ea4ec0affe
diffArtifact=/private/var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestRecertifiedLandingPublishesNestedSchemaTwoCandidate2032271865/001/local/metasystem/artifacts/agents/park-chain/rounds/1/diff.patch
recertification=metasystem/artifacts/agents/landing/recertifications/park-chain/492850c236ac7ebff49cbe0f8831c0cdbcb8b183836a9e368f73424860fb1777/record.json
certifiedTree=30bbbe617af9f054c50d4cff59403dc188da0d83
diffArtifact=metasystem/artifacts/agents/landing/recertifications/park-chain/492850c236ac7ebff49cbe0f8831c0cdbcb8b183836a9e368f73424860fb1777/diff.patch
reviewedTree=4ea93429a4cde73c69d29941828d3e9f4ade6bfb
diffArtifact=/private/var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestLandingTestReceiptModeWithoutTreePublishesIndexReceipt2485849929/001/local/artifacts/agents/park-chain/rounds/1/diff.patch
recertification=artifacts/agents/landing/recertifications/park-chain/e24f01d328d8c3f14d711ed30eaaf8393816f550fc846fb89d906e7a4cc8bf0e/record.json
certifiedTree=da58cf1382871d9d935e540e235835f8abc30e4b
diffArtifact=artifacts/agents/landing/recertifications/park-chain/e24f01d328d8c3f14d711ed30eaaf8393816f550fc846fb89d906e7a4cc8bf0e/diff.patch
test receipt refused: supplied tree f33b7d27317b01340676828baf8df84bfeb698e6 differs from the real index tree f33b7d27317b01340676828baf8df84bfeb698e6 or working-tree projection 1a06180aa4fb8d9c596730e05d0d35dc89b55210
cannot read metasystem configuration: /private/var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/metasystem-landing-receipt.756664034/worktree-756664034/metasystem.conf: open /private/var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/metasystem-landing-receipt.756664034/worktree-756664034/metasystem.conf: no such file or directory
landing observe --carried requires --judge live, or --judge base with --live-failure
landing observe --carried requires --judge live, or --judge base with --live-failure
{"schemaVersion":1,"outcome":"refused","reason":"receipt-line-missing","detail":"the landing changes code (payload.txt) but its staged memory/receipts.log appends no RECEIPT line for goal fx; write the line with scripts/receipt.sh add --type implement --outcome shipped --goal fx --built-by coordinator --note \"\u003cwhat landed and how it was verified\u003e\" and include memory/receipts.log in the landing","ledger":"memory/receipts.log","goal":"fx","command":"scripts/receipt.sh add --type implement --outcome shipped --goal fx --built-by coordinator --note \"\u003cwhat landed and how it was verified\u003e\"","codePaths":["payload.txt"]}
{"schemaVersion":1,"outcome":"pass","reason":"receipt-line-appended","detail":"the landing appends the RECEIPT line of epoch 2 (goal fx) to memory/receipts.log","ledger":"memory/receipts.log","goal":"fx","codePaths":["payload.txt"]}
usage: metasystem landing receipt-line --root INSTALLATION --tree TREE [--goal ID] [--direct-fix CLASS]
usage: metasystem launch round-task --tag <tag> --round <2|3> --previous <file> --out <file> [--constraints <n>]
flag provided but not defined: -bad
Usage of launch round-task:
  -constraints string
    	constraint count (default "0")
  -out string
    	output file
  -previous string
    	previous task file
  -round int
    	task number
  -tag string
    	launch tag
usage: metasystem launch round-task --tag <tag> --round <2|3> --previous <file> --out <file> [--constraints <n>]
usage: metasystem launch round-task --tag <tag> --round <2|3> --previous <file> --out <file> [--constraints <n>]
usage: metasystem launch round-task --tag <tag> --round <2|3> --previous <file> --out <file> [--constraints <n>]
flag provided but not defined: -poll
Usage of launch start:
  -brief string
    	brief file
  -diff-file string
    	diff file for a read
  -dir string
    	working directory (default ".")
  -effort string
    	reasoning effort
  -goal string
    	goal id
  -input value
    	additional input file (repeatable)
  -kind string
    	launch kind
  -model string
    	model
  -output value
    	output file to copy into launch state (repeatable)
  -package string
    	directory selected from a split diff
  -page string
    	page file
  -resume-session string
    	Claude session id
  -tag string
    	launch tag
  -unit value
    	unit name from --units-page (repeatable)
  -units-page string
    	page containing the units table
  -wide
    	use the wide read window
hook delegate query could not read job record job-absent.json: open /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestLeaseHookDelegateCLIExitContract2270172983/001/artifacts/agents/jobs/job-absent.json: no such file or directory
concluded legacy-goal
{"detail":"","outcome":"confirmed","tip":"106b3f862acedc333a1fad1468bfa906e44685ad"}
the metasystem is stopped for /private/var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestMissionFenceClassificationUsesTheNestedInstallationAndKeepsF3679519898/001 since 2026-09-08T04:00:00Z, by stop pid 72
at an agent-free terminal, run: metasystem mission start --root /private/var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestMissionFenceClassificationUsesTheNestedInstallationAndKeepsF3679519898/001 --mission <id>
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
Usage:
  metasystem mission start --mission <id> [--foreground]
  metasystem mission resume --mission <id> [--foreground]
  metasystem mission status --mission <id>
  metasystem mission answer --mission <id> --ask <ask-id> --answer <text>
  metasystem mission resolve-taint --mission <id> --taint <n> --by <name> --reason <text>
      (--restore <treeId> | --adopt --waives <claim> [--waives <claim> ...])
resolve refused: taint resolution is a human-reserved act; this caller classifies UNTRUSTED
resolve refused: taint resolution is a human-reserved act; this caller classifies UNTRUSTED
--- FAIL: TestProofAttemptBinaryLaunchesUseSharedIsolation (0.04s)
    proof_fixture_test.go:135: runtime_conformance_test.go launches a metasystem fixture outside proofBinaryFixture.command
--- FAIL: TestLandingBatchBinaryIgnoresAmbientProofHostLoad (5.62s)
    proof_fixture_test.go:214: without reason token observed="", want an integer of at least 1
expected goal and accounting revisions must be supplied together
prove-round: chain implementer-1 round 2 tree eb1cefbc088fa7a6774f41a749c779143c12e991 goal goal-a purpose diagnostic
prove-round: the run recorded no attempt for tree eb1cefbc088fa7a6774f41a749c779143c12e991 (test run exit 7)
prove-round: chain implementer-1 round 2 tree eb1cefbc088fa7a6774f41a749c779143c12e991 goal goal-a purpose diagnostic
prove-round: the run recorded no attempt for tree eb1cefbc088fa7a6774f41a749c779143c12e991 (test run exit 7)
prove-round: chain implementer-1 round 2 tree eb1cefbc088fa7a6774f41a749c779143c12e991 goal goal-a purpose diagnostic
prove-round: tree eb1cefbc088fa7a6774f41a749c779143c12e991 is proved by the retained attempts admission reused; no new attempt
prove-round: chain implementer-1 round 2 tree 76364cd5537fb2d1cc9809aa3c0f9d11cf16e234 goal goal-a purpose diagnostic
prove-round: the run recorded no attempt for tree 76364cd5537fb2d1cc9809aa3c0f9d11cf16e234 (test run exit 0)
job prove-round refused: job shared-1 runs in the shared checkout, so its tree is this checkout's own index: prove it as your own work with metasystem test run
job prove-round refused: job goalless-1 carries no goal, and a proof binds one
job prove-round refused: round 3 of job implementer-1 (implementer-1-r3) is running; a round is proved after it has returned
prove-round: chain older-1 round 1 tree eb1cefbc088fa7a6774f41a749c779143c12e991 goal goal-a purpose diagnostic
prove-round: the run recorded no attempt for tree eb1cefbc088fa7a6774f41a749c779143c12e991 (test run exit 0)
supervise component reaper: proof attempt proof-mu8gyc4t-9634b59a8ea7dd02: reconciled: reconciled: launcher pid 1073741817 started 100 is dead; 0 recorded processes confirmed ended; no terminal was committed
--- FAIL: TestRuntimeFilePlacement (0.02s)
    runtime_placement_test.go:86: cross-runtime code in per-runtime files:
        fake.sh:455: [fake file] hook_config="$bed/scripts/enforcement/claude-code-hooks.json"
unknown or non-adoptable runtime: unknown
host setup check: 10 registration path(s) require setup
unknown runtime: nope
no static enforcement map declared for fake
usage: metasystem runtime list [--adoptable|--with-adapter|--with-host|--with-common-lifecycle]
usage: metasystem runtime adoption-default
unknown runtime: ghostrt
usage: metasystem runtime dirs <runtime>
no shipped enforcement config declared for fake
no live self-check declared for codex
no session environment declared for codex
no start context declared for codex
unknown runtime: unknown
usage: metasystem runtime context-sample <runtime>
session stop refused: this is human-reserved; caller classifies DELEGATE
j1|120
supervise blocking-reserved-cap: refusing to arm: job record unreadable: /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestBlockingReservedCapFailsClosed4284248452/001/jobs/j2.json: invalid character 'b' looking for beginning of object key string
supervise blocking-reserved-cap: refusing to arm: job record capMin is malformed: /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestBlockingReservedCapFailsClosed4284248452/001/jobs/j3.json
supervise blocking-reserved-cap: refusing to arm: fence counters unreadable: /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestBlockingReservedCapFailsClosed4284248452/001/missions/m1/fences.json: invalid character 'b' looking for beginning of object key string
supervise blocking-reserved-cap: refusing to arm: reservation jx is malformed in /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestBlockingReservedCapFailsClosed4284248452/001/missions/m1/fences.json
supervise blocking-reserved-cap: refusing to arm: METASYSTEM_FAKE_PROCESS_IDENTITY_FILE is set but metasystem.runtimes is not fake
supervise blocking-reserved-cap: refusing to arm: METASYSTEM_FAKE_PROCESS_IDENTITY_FILE is set but metasystem.runtimes is not fake
{"cadence":"tick","component":"landing-owner","error":"CADENCE_FETCH_REFUSED: BATCH_LAND_PUSH_REFUSED: fetch origin/main: fatal: 'origin' does not appear to be a git repository\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights\nand the repository exists.: exit status 128"}
{"cadence":"tick","component":"landing-owner","error":"CADENCE_FETCH_REFUSED: BATCH_LAND_PUSH_REFUSED: fetch origin/main: fatal: 'origin' does not appear to be a git repository\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights\nand the repository exists.: exit status 128"}
{"cadence":"tick","component":"landing-owner","error":"CADENCE_FETCH_REFUSED: BATCH_LAND_PUSH_REFUSED: fetch origin/main: fatal: 'origin' does not appear to be a git repository\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights\nand the repository exists.: exit status 128"}
{"cadence":"tick","component":"landing-owner","error":"CADENCE_FETCH_REFUSED: BATCH_LAND_PUSH_REFUSED: fetch origin/main: fatal: 'origin' does not appear to be a git repository\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights\nand the repository exists.: exit status 128"}
{"cadence":"tick","component":"landing-owner","error":"CADENCE_FETCH_REFUSED: BATCH_LAND_PUSH_REFUSED: fetch origin/main: fatal: 'origin' does not appear to be a git repository\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights\nand the repository exists.: exit status 128"}
{"cadence":"tick","component":"landing-owner","error":"CADENCE_FETCH_REFUSED: BATCH_LAND_PUSH_REFUSED: fetch origin/main: fatal: 'origin' does not appear to be a git repository\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights\nand the repository exists.: exit status 128"}
{"cadence":"tick","component":"landing-owner","error":"CADENCE_FETCH_REFUSED: BATCH_LAND_PUSH_REFUSED: fetch origin/main: fatal: 'origin' does not appear to be a git repository\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights\nand the repository exists.: exit status 128"}
supervise owner: refusing to arm: /proc is mounted hidepid=2, which makes another user's live process indistinguishable from a dead one and breaks identity's three-way liveness guarantee
metasystem test: the landing ref moved under the run (ours=3fb54acf99a37095f54a89c095ee77bb2800c102 engine=070c8b2c893534bf8b584f82d96eaaad0d376000); restarting preparation once
metasystem test: the landing ref moved under the run (ours=16ea89f19bcb9ab183487189313094efbacb6db7 engine=8992b48b504b0a6606c24580767ab799e47cb7ad); restarting preparation once
metasystem test: the landing ref moved under the run (ours=base-a engine=base-b); restarting preparation once
panic: test timed out after 10m0s
	running tests:
		TestFrozenPublicVersionOneSelectionProbesRunAgainstCandidateExecutable (4s)
		TestFrozenPublicVersionOneSelectionProbesRunAgainstCandidateExecutable/remove-required-provider (2s)

goroutine 41000 [running]:
testing.(*M).startAlarm.func1()
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2959 +0x2c4
created by time.goFunc
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/time/sleep.go:182 +0x38

goroutine 1 [chan receive]:
testing.(*T).Run(0x6ae74e472248, {0x105659292?, 0x6ae74e299688?}, 0x1061d4860)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2266 +0x3cc
testing.runTests.func1(0x6ae74e472248)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2742 +0x38
testing.tRunner(0x6ae74e472248, 0x6ae74e2997b8)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
testing.runTests({0x105612275, 0x30}, {0x105647a90, 0x3f}, 0x6ae74e256018, {0x106244838, 0x21b, 0x21b}, {0x1061cc668?, 0x6ae74e880040?, ...})
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2740 +0x400
testing.(*M).Run(0x6ae74e4adcc0)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2600 +0x578
github.com/widoriezebos/agentic-tools/metasystem/internal/testenv.Main(0x6ae74e4adcc0, {0x0, 0x0, 0x0})
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/internal/testenv/testenv.go:221 +0x734
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestMain(0x6ae74e4adcc0)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/ambient_controls_test.go:40 +0x3b4
main.main()
	_testmain.go:1124 +0x88

goroutine 115 [chan receive, 9 minutes]:
testing.(*T).Parallel(0x6ae74e472488)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:1957 +0x194
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestAuditParallelRatchetVerbRefusesAndLowers(0x6ae74e472488)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/audit_test.go:46 +0x28
testing.tRunner(0x6ae74e472488, 0x1061d4338)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 1
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8

goroutine 2677 [chan receive, 9 minutes]:
testing.(*T).Parallel(0x6ae74e473208)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:1957 +0x194
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestGateUnitRedOutputAndExitCode(0x6ae74e473208)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/gate_unit_test.go:53 +0x28
testing.tRunner(0x6ae74e473208, 0x1061d4888)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 1
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8

goroutine 34584 [chan receive, 1 minutes]:
testing.(*T).Parallel(0x6ae74efee488)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:1957 +0x194
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestCandidateExtensionIsRefusedUntilCandidateBecomesAuthority(0x6ae74efee488)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/proof_run_test.go:528 +0x34
testing.tRunner(0x6ae74efee488, 0x1061d45a0)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 1
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8

goroutine 177 [chan receive, 9 minutes]:
testing.(*T).Parallel(0x6ae74e4e8248)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:1957 +0x194
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestAuditStopDecisionSurfaceRefusesRecordTheGoalParserRejects(0x6ae74e4e8248)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/audit_test.go:155 +0x28
testing.tRunner(0x6ae74e4e8248, 0x1061d4340)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 1
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8

goroutine 2676 [chan receive, 9 minutes]:
testing.(*T).Parallel(0x6ae74e472fc8)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:1957 +0x194
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestGateUnitGreenOutputAndAggregatedSteps(0x6ae74e472fc8)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/gate_unit_test.go:13 +0x28
testing.tRunner(0x6ae74e472fc8, 0x1061d4880)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 1
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8

goroutine 33714 [chan receive, 1 minutes]:
testing.(*T).Parallel(0x6ae74e662908)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:1957 +0x194
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestProcDefaultSignalsRestoresIgnoredSignals(0x6ae74e662908)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/proc_signals_test.go:32 +0x24
testing.tRunner(0x6ae74e662908, 0x1061d4e58)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 1
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8

goroutine 14811 [syscall, 7 minutes]:
os/signal.signal_recv()
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/runtime/sigqueue.go:149 +0x104
os/signal.loop()
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/os/signal/signal_unix.go:23 +0x1c
created by os/signal.Notify.func2.1 in goroutine 15061
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/os/signal/signal.go:164 +0x28

goroutine 33156 [chan receive, 2 minutes]:
testing.(*T).Parallel(0x6ae74efeed88)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:1957 +0x194
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestLaunchWaitNamesTheCapAndThePendingState(0x6ae74efeed88)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/launch_verbs_wait_test.go:13 +0x28
testing.tRunner(0x6ae74efeed88, 0x1061d4ca0)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 1
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8

goroutine 37090 [chan receive]:
testing.(*T).Parallel(0x6ae74efee6c8)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:1957 +0x194
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestLandedRearmRefusesARepositoryFailureReadingTheLandingRefWithGitDetail(0x6ae74efee6c8)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/rearm_on_landed_test.go:311 +0x20
testing.tRunner(0x6ae74efee6c8, 0x1061d4bb0)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 1
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8

goroutine 34583 [chan receive, 1 minutes]:
testing.(*T).Parallel(0x6ae74efee248)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:1957 +0x194
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestCandidateCannotEscapeAuthorityElapsedLimit(0x6ae74efee248)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/proof_run_test.go:509 +0x28
testing.tRunner(0x6ae74efee248, 0x1061d4570)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 1
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8

goroutine 33818 [chan receive, 1 minutes]:
testing.(*T).Parallel(0x6ae74e8ef448)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:1957 +0x194
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestLandFixtureScenarioRegistryMatchesCount(0x6ae74e8ef448)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/proof_fixture_test.go:373 +0x28
testing.tRunner(0x6ae74e8ef448, 0x1061d4b78)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 1
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8

goroutine 37091 [chan receive]:
testing.(*T).Parallel(0x6ae74efee908)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:1957 +0x194
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestLandedRearmRefusesARepositoryFailureResolvingCheckoutHeadWithGitDetail(0x6ae74efee908)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/rearm_on_landed_test.go:365 +0x20
testing.tRunner(0x6ae74efee908, 0x1061d4bb8)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 1
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8

goroutine 33713 [chan receive, 1 minutes]:
testing.(*T).Parallel(0x6ae74e6626c8)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:1957 +0x194
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestProcDefaultSignalsHelper(0x6ae74e6626c8?)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/proc_signals_test.go:15 +0x1c
testing.tRunner(0x6ae74e6626c8, 0x1061d4e50)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 1
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8

goroutine 19751 [chan receive, 7 minutes]:
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.batchOwnerSignals.func2()
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/landing_batch_owner.go:495 +0x28
created by github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.batchOwnerSignals in goroutine 19773
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/landing_batch_owner.go:495 +0x174

goroutine 40946 [syscall]:
syscall.syscalln(0x1049510d0, {0x6ae74e3424a8, 0x6, 0x6})
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/runtime/sys_darwin.go:27 +0x20
syscall.syscall6(0x14e342678?, 0x133b4d0e8?, 0x10835b5c0?, 0x90?, 0x106252e40?, 0x6ae74e36b050?, 0x6ae74e342548?)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/syscall/syscall_darwin.go:407 +0x28
syscall.wait4(0x6ae74e342578?, 0x10497102c?, 0x90?, 0x1061b5380?)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/syscall/zsyscall_darwin_arm64.go:44 +0x4c
syscall.Wait4(0x1055b8a53?, 0x6ae74e3425b4, 0x6ae74e771490?, 0x1055c211a?)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/syscall/syscall_bsd.go:144 +0x28
os.(*Process).pidWait.func1(...)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/os/exec_unix.go:64
os.ignoringEINTR2[...](...)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/os/file_posix.go:273
os.(*Process).pidWait(0x6ae74eb43c40)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/os/exec_unix.go:63 +0x9c
os.(*Process).wait(0x0?)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/os/exec_unix.go:28 +0x24
os.(*Process).Wait(...)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/os/exec.go:347
os/exec.(*Cmd).Wait(0x6ae74e946680)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/os/exec/exec.go:944 +0x34
os/exec.(*Cmd).Run(0x6ae74e946680)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/os/exec/exec.go:643 +0x38
os/exec.(*Cmd).CombinedOutput(0x6ae74e946680)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/os/exec/exec.go:1061 +0x84
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.runFrozenSelectionProbe({_, _}, {{0x6ae74f10b200, 0x7f}, 0x0, {0x0, 0x0}, {0x0, 0x0}, {0x0, ...}, ...}, ...)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/test_protection.go:239 +0x1b04
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestFrozenPublicVersionOneSelectionProbesRunAgainstCandidateExecutable.func1(0x6ae74efef208)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/test_test.go:1109 +0x90
testing.tRunner(0x6ae74efef208, 0x6ae74e5f8080)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 40889
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8

goroutine 34582 [chan receive, 1 minutes]:
testing.(*T).Parallel(0x6ae74efee008)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:1957 +0x194
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestOneLiveChargedAttemptCountsForBothLenses(0x6ae74efee008)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/proof_run_test.go:457 +0x28
testing.tRunner(0x6ae74efee008, 0x1061d4d68)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 1
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8

goroutine 33639 [chan receive, 2 minutes]:
testing.(*T).Parallel(0x6ae74f73c6c8)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:1957 +0x194
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestCaptureCommandOutputAllowsNestedAndConcurrentDisjointStreams(0x6ae74f73c6c8)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/output_capture_test.go:96 +0x28
testing.tRunner(0x6ae74f73c6c8, 0x1061d45e8)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 1
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8

goroutine 37674 [chan receive]:
testing.(*T).Parallel(0x6ae74efeefc8)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:1957 +0x194
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestFakeStopReplay(0x6ae74efeefc8)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/runtime_conformance_test.go:27 +0x24
testing.tRunner(0x6ae74efeefc8, 0x1061d4810)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 1
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8

goroutine 40999 [IO wait]:
internal/poll.runtime_pollWait(0x1336b9e00, 0x72)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/runtime/netpoll.go:351 +0xa0
internal/poll.(*pollDesc).wait(0x6ae74e3453e0?, 0x6ae74e862600?, 0x1)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/internal/poll/fd_poll_runtime.go:84 +0x28
internal/poll.(*pollDesc).waitRead(...)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/internal/poll/fd_poll_runtime.go:89
internal/poll.(*FD).Read(0x6ae74e3453e0, {0x6ae74e862600, 0x200, 0x200})
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/internal/poll/fd_unix.go:170 +0x22c
os.(*File).read(...)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/os/file_posix.go:30
os.(*File).Read(0x6ae74e4174f8, {0x6ae74e862600?, 0x6ae74e9d52c0?, 0x0?})
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/os/file.go:144 +0x68
bytes.(*Buffer).ReadFrom(0x6ae74e91be00, {0x1061cc5a8, 0x6ae74e4174f8})
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/bytes/buffer.go:229 +0x90
io.copyBuffer({0x1061cc828, 0x6ae74e91be00}, {0x1061cc5a8, 0x6ae74e4174f8}, {0x0, 0x0, 0x0})
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/io/io.go:415 +0x14c
io.Copy(...)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/io/io.go:388
os.genericWriteTo(0x6ae74e7d6e78?, {0x1061cc828?, 0x6ae74e91be00?})
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/os/file.go:295 +0x44
os.(*File).WriteTo(0x1061066e0?, {0x1061cc828?, 0x6ae74e91be00?})
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/os/file.go:273 +0x5c
io.copyBuffer({0x1061cc828, 0x6ae74e91be00}, {0x1061cc668, 0x6ae74e4174f8}, {0x0, 0x0, 0x0})
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/io/io.go:411 +0x98
io.Copy(...)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/io/io.go:388
os/exec.(*Cmd).writerDescriptor.func1()
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/os/exec/exec.go:613 +0x40
os/exec.(*Cmd).Start.func2(0x0?)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/os/exec/exec.go:768 +0x30
created by os/exec.(*Cmd).Start in goroutine 40946
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/os/exec/exec.go:767 +0x6d8

goroutine 40889 [chan receive]:
testing.(*T).Run(0x6ae74efeeb48, {0x1055b8c63?, 0x6ae74e2999d0?}, 0x6ae74e5f8080)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2266 +0x3cc
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestFrozenPublicVersionOneSelectionProbesRunAgainstCandidateExecutable(0x6ae74efeeb48)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/test_test.go:1108 +0x1294
testing.tRunner(0x6ae74efeeb48, 0x1061d4860)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 1
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8
FAIL	github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem	600.635s
FAIL
```

Exit status: 1. The run timed out after ten minutes in unrelated `TestFrozenPublicVersionOneSelectionProbesRunAgainstCandidateExecutable/remove-required-provider`; it also printed the sandbox-only `TestGroupOwnedLiveNonOwnerExitsNotOwned` process-ownership red.

`GOCACHE=/tmp/stopinc-3b-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-3b-staticcheck go test -count=1 -run 'TestProofAttempt|TestLandFixture|TestEveryPackageUsesSharedMain|TestBedsResolveStopReportsThroughTheEngine|TestCapabilityDeclarationsMatchRegistrations|TestCapabilityFlagsBackedByExecutables' ./cmd/metasystem/`

```text
ok  	github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem	0.560s
```

`bash -n scripts/agents/supervision-hook.sh`

```text
```

`bash -n scripts/agents/stop-degraded-forms.sh`

```text
```

`bash -n scripts/agents/adapters/fake.sh`

```text
```

`bash scripts/agents/adapters/fake.sh selftest`

```text
!!!!!!!!!!!!!!!! STALE FIXTURE ENGINE WARNING !!!!!!!!!!!!!!!!
bin/metasystem is older than Go source: /Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3b/metasystem/cmd/metasystem/runtime_conformance_test.go
fixture results may describe the old engine; continuing because pinned fixture engines are allowed
!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

runtime fake is outside metasystem.runtimes
{"outcome":"REFUSED-INTERNAL","headline":"refused","detail":"runtime fake is outside metasystem.runtimes"}
```

Exit status: 1. This is not an allowed sandbox-only red, so the build is blocked.

`GOCACHE=/tmp/stopinc-3b-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-3b-staticcheck go build ./...`

```text
```

`GOCACHE=/tmp/stopinc-3b-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-3b-staticcheck go build -tags batchtest ./...`

```text
```

`GOCACHE=/tmp/stopinc-3b-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-3b-staticcheck go test -count=1 -run 'TestTestEnvironmentStandardInventoryMatchesPackageTests' ./internal/testenv/`

```text
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/testenv	0.583s
```

`GOCACHE=/tmp/stopinc-3b-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-3b-staticcheck go test -count=1 -run 'TestRepositoryManifestClassifiesEveryTrackedPath' ./internal/pathclass/`

```text
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/pathclass	0.367s
```

`GOCACHE=/tmp/stopinc-3b-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-3b-staticcheck go run ./cmd/metasystem audit parallel-ratchet --root .`

```text
parallel ratchet passed
```

`git diff HEAD -- testing-parallel-ratchet.json`

```text
```

`GOCACHE=/tmp/stopinc-3b-go-cache GOPROXY=file:///Users/wido/go/pkg/mod/cache/download GOSUMDB=off STATICCHECK_CACHE=/tmp/stopinc-3b-staticcheck bash scripts/agents/go-gate.sh --fast`

```text
go gate: effective Go: go version go1.27.1 darwin/arm64
go gate: GOTOOLCHAIN: auto
stop decision surface: base 1e84763e202245843cca98b1a41265d39f3fb51e; added 0, moved 0, removed 0
go gate: fast mode passed (dependency ratchet, parallel ratchet, gofmt, vet, staticcheck, refusal register, SessionStart exit audit, Stop decision surface audit, build); the full gate remains the landing requirement
```

## Mutations

- `TestFakeStopReplay/FakeReplayUnreadableHookState`: `runtime_conformance_test.go:76: run 3 delivery-unconfirmed presence = true`; adding the sentence whenever the refusal record failed was caught because run three still had a writable hook log.
- `TestFakeStopReplay/FakeReplayDeadlineUnreadable`: `runtime_conformance_test.go:111: deadline run 4 qualifiers do not match fault both: "Task unknown; Stop allowed; needs supervision repair; stop deadline expired; record update failed; condition log failed. The steward must restore supervision. Status unavailable."`
- `TestFakeStopReplay/FakeReplayDeadlineUnreadable`: `runtime_conformance_test.go:127: deadline seam was honored without the fixture owner tag: "stop-condition infrastructure stop-deadline-expired stop-deadline - 1000000004 degraded-allow\n2026-09-19T14:03:23Z stop response outcome=deadline-expired-allow elapsed=789826603s\n"`

Before the first mutation:

```text
 .../cmd/metasystem/runtime_conformance_test.go     | 282 +++++++++++++++++++++
 .../internal/hooks/degraded_stop_forms_test.go     |   4 +-
 .../hooks/testdata/stop-degraded-forms.golden      |   8 +
 metasystem/scripts/agents/adapters/fake.sh         | 103 ++++++++
 metasystem/scripts/agents/stop-degraded-forms.sh   |   6 +-
 metasystem/scripts/agents/supervision-hook.sh      |  42 ++-
 metasystem/testing.json                            |   2 +-
 7 files changed, 435 insertions(+), 12 deletions(-)
```

After all three mutations were restored:

```text
 .../cmd/metasystem/runtime_conformance_test.go     | 282 +++++++++++++++++++++
 .../internal/hooks/degraded_stop_forms_test.go     |   4 +-
 .../hooks/testdata/stop-degraded-forms.golden      |   8 +
 metasystem/scripts/agents/adapters/fake.sh         | 103 ++++++++
 metasystem/scripts/agents/stop-degraded-forms.sh   |   6 +-
 metasystem/scripts/agents/supervision-hook.sh      |  42 ++-
 metasystem/testing.json                            |   2 +-
 7 files changed, 435 insertions(+), 12 deletions(-)
```

The first mutation initially exposed an omitted lawful wrapper call, `lease require-holder`; the test was corrected to admit that known engine call and the mutation was rerun to the intended run-three red above.

## Sandbox reds

- `TestFakeStopReplay` is HOST-ONLY in this sandbox. The deadline parent cannot verify `ps -p "$deadline_worker" -o command=` and reports `stop deadline: worker PID left running, command line unverifiable`; fixture cleanup then fails with `identity: sysctl kern.proc.all: operation not permitted`. Every response, qualifier, fixed-epoch, seam-off, JSON-shape and replay-call assertion completed before that cleanup red.
- `scripts/agents/supervision-hook-fixtures.sh` is HOST-ONLY. It reported `brain hook fake channel did not start`, then `proc fixture-survivors: process table is unreadable: process enumeration failed: identity: sysctl kern.proc.all: operation not permitted`. The deadline-expiry, deadline-missing-engine, deadline-record-failure, deadline-log-failure and deadline-restricted-process scenarios passed before the final sandbox cleanup failure.
- The whole `cmd/metasystem` package printed `TestGroupOwnedLiveNonOwnerExitsNotOwned ... INDETERMINATE, want NOT-OWNED`, consistent with denied process inspection, and later timed out in unrelated `TestFrozenPublicVersionOneSelectionProbesRunAgainstCandidateExecutable/remove-required-provider`.

## New tests

- `TestFakeStopReplay/FakeReplayUnreadableHookState`, package `cmd/metasystem`, group `wait-stop-standard`.
- `TestFakeStopReplay/FakeReplayDeadlineUnreadable`, package `cmd/metasystem`, group `wait-stop-standard`.
- Existing `TestDegradedStopFormsMatchTheGoldenContract` and `TestDegradedStopRendererIsSideEffectFree`, package `internal/hooks`, remain in `hook-start-audit-standard` through its `tests: all` contract.
- The golden now contains 26 rows; `want 18` became `want 26`.
- Runs five and six both answer exactly the engine-missing form, `replay-calls.log` is empty for both, and the delivery-unconfirmed sentence appears only in run four.

`testing.json` hunk:

```diff
diff --git a/metasystem/testing.json b/metasystem/testing.json
index d226ec70f..efe8555c9 100644
--- a/metasystem/testing.json
+++ b/metasystem/testing.json
@@ -47,7 +47,7 @@
     {"id":"carry-landing-standard","kind":"unit","adapter":"go","cwd":"metasystem","inputs":["metasystem/go.mod","metasystem/go.sum","metasystem/internal/landing/**","metasystem/internal/goal/branch/**","metasystem/cmd/metasystem/channel_verbs.go","metasystem/cmd/metasystem/channel_verbs_test.go","metasystem/cmd/metasystem/goalsync_mutations.go","metasystem/cmd/metasystem/goalsync_mutations_test.go","metasystem/cmd/metasystem/landing_verbs.go","metasystem/cmd/metasystem/landing_verbs_test.go"],"outputs":[],"tools":[{"id":"go","executable":"go","versionArgs":["version"]}],"obligations":["carried-landing-landing"],"platforms":["any"],"targetMs":20000,"packages":["internal/landing","internal/landing/batch","cmd/metasystem"],"tests":["TestHCL33ExpiredReservationNotDebt","TestHCL37EvaluatorUnavailablePrecedesUnneeded","TestHCL51CarriedCounselorAppendBelongsToItsRow","TestHCL57CarriedMatchAndRedBattery","TestHCL58BaseJudgeFenceOwners","TestHCL58GenerationZeroNeedsFixtureAuthority","TestHCL12KindCarryRequiresWants","TestHCL54CarryByRejectsStoredActorPrefix","TestHCL55CarriedJudgeGrammar","TestHCL80TrailingWhitespaceWhyIsAdmitted","TestBatchBranchReaderCertifiesThreeBuildMember","TestBatchBranchReaderRefusesReaderRecord","TestBatchBranchMembersCheckEachGoalTransition","TestBatchBranchMemberRefusesMovedTrunk","TestBatchBranchAllowsOnlyOneMemberPerGoal","TestObserveAttestedBranchDeclarationEnforcesBoundary","TestAttestedBoundaryValidatesTheClosureBundle"]},
     {"id":"carry-plumbing-standard","kind":"unit","adapter":"go","cwd":"metasystem","inputs":["metasystem/go.mod","metasystem/go.sum","metasystem/internal/dispatch/**","metasystem/internal/config/**","metasystem/internal/testpolicy/**"],"outputs":[],"tools":[{"id":"go","executable":"go","versionArgs":["version"]}],"obligations":["carried-landing-plumbing"],"platforms":["any"],"targetMs":10000,"packages":["internal/dispatch","internal/config","internal/testpolicy"],"tests":["TestHCL09BadFormRefused","TestHCL09ReviewReferenceAdmits","TestHCL09TwoChainsUnite","TestHCL08CapKeyDefaultOne","TestHCL08CapLocalRefused","TestHCL31ContractOwnsFivePackages","TestHCL34PlanExecutesEveryFixture"]},
     {"id":"goal-decision-standard","kind":"unit","adapter":"go","cwd":"metasystem","inputs":["metasystem/go.mod","metasystem/go.sum","metasystem/internal/goal/**","metasystem/cmd/metasystem/goal_branch.go","metasystem/cmd/metasystem/goal_branch_test.go","metasystem/cmd/metasystem/main.go","metasystem/cmd/metasystem/goalsync_mutations.go","metasystem/cmd/metasystem/goalsync_mutations_test.go","metasystem/internal/dispatch/**","metasystem/internal/readsubject/**","metasystem/cmd/metasystem/goal_list.go","metasystem/cmd/metasystem/goal_list_test.go",".gitattributes","metasystem/internal/testpolicy/contractgit/**"],"outputs":[],"tools":[{"id":"go","executable":"go","versionArgs":["version"]}],"obligations":["budget-stop-authority"],"platforms":["any"],"targetMs":30000,"packages":["internal/goal","internal/goal/branch","cmd/metasystem"],"tests":["TestGoalDoneWithoutLocalBranchSkipsUnreadableRemote","TestGoalDoneWithLocalBranchStillSweepsUnreadableRemote","TestBudgetTupleIsCompletePositiveAndCanonical","TestElapsedBreachDurationAppliesGraceAtTheStopBoundary","TestBudgetValidationNamesEveryInvalidLimit","TestStoredBudgetRequiresCompleteNumericLimits","TestHandedOverClaimGuards","TestHandedOverClaimRoundTripsWithoutChangingExistingClaims","TestHandedOverClaimsDoNotConsumeLandingSlots","TestHandedOverClaimsUseOneDerivedLandingPair","TestHandedOverRecordRequiresEveryCoordinate","TestGoalHandoverAppliesCompleteFieldTable","TestHandedOverClaimReleaseClearsRecord","TestHandedOverClaimConclusionClearsRecord","TestUnitDigestIgnoresDiffConfiguration","TestValidateRangeRefusals","TestValidateRangeCleanRangePasses","TestGoalBranchCheckPrintsKinds","TestGoalHandBackClearsHandedOverAndValidates","TestGoalHandBackRefusals","TestGoalHandoverTargetLiveness","TestGoalBranchCheckFetchesCurrentOriginTip","TestGoalBranchGitKeepsStderrOutOfObjectIDs","TestValidateRangeAllowsBranchBehindEndpoint","TestCommitCreatesBranchAndAmendsUnit","TestCommitRefusesNonHolder","TestCommitRefusesWithoutPushProtocol","TestCommitHasSingleAmendDecision","TestCommitRefusalsPreserveCheckout","TestAmendKeepsUnstagedTrackedEdits","TestAmendRefusesBeforeOverwritingUnstagedTrackedEdit","TestAmendInstallFailureRestoresCheckout","TestAmendRechecksClaimBeforeBranchUpdate","TestPushPublishesAmendAndSecondCloneAdopts","TestPushReconcilesUnknownOutcomeAndPreparedTransaction","TestPushAndCommitAdoptDescendantRemoteAfterLandedCrash","TestPushLeaseAndClaimMovementRefuseWithoutRetry","TestPushUnknownNotLandedDoesNotWedgeLaterAmend","TestPushAdoptsRemoteAdvanceAndCommitUsesIt","TestPushAndCommitRefuseDivergedRemote","TestPushAdoptionValidatesRangeAndCleansFetchRef","TestAttestationReaderRecordIntegrity","TestAttestationRequiresFastGateAndNamesChangedTests","TestAttestationCriticRootSourceValidates","TestAttestationCarryPreservesUnitAndFoldBytes","TestAttestationCarryRefusesChangedUnitOrFold","TestAttestationCarryChecksEveryEarlierFold","TestReadRefusalsPreserveCheckout","TestReadAdoptionRefusalsLeaveCheckoutUntouched","TestReadAdoptionKeepsUntrackedScratchFile","TestGoalBranchClaimRequiresMachineAndLineage","TestGoalBranchCommitAndPushUseEndpointRemote","TestGoalBranchHelpNamesPush","TestGoalBranchStatusReportsAbsentOrigin","TestGoalBranchTestingContractIncludesReadDependencies","TestGoalBranchLandPrepRoutesLocalRedCandidate","TestStatusLandReadyPrefixAndParkSafety","TestParkRecordsPushedBranchSummary","TestParkCommandEdgeSkipsUnreadableRemoteOnlyWithoutLocalBranch","TestParkCommandEdgeChecksEndpointOnlyForLocalBranch","TestTransportMirrorAndLeasedDelete","TestGoalLandingPreparationSeries","TestGoalLandingPreimageRetryAndCanaryFence","TestGoalLandingRetryIdentitySurvivesASecondClone","TestGoalLandingLastRefusesUnitBeyondPrefix","TestLandingRedLoopClassificationAndBudget","TestValidateRangeAcceptsBuildListsAndRejectsDuplicateUnitAcrossBuilds","TestAmendDropsReadOfReplacedBuildAndReplaysLaterPlan","TestGoalBranchPushUsesLedgerFailureClassification","TestPushAdoptionCrashCannotStageAReversal","TestPushAdoptionKeepsUntrackedScratchFile","TestCommitAdoptsRemoteWithoutEndpointCheckout","TestGoalBranchCommitIsTheGuardedCommitWrapper","TestFailedCommitLeavesCheckoutAndRefsUnchanged","TestGoalBranchCommitAcceptsBuildUnitList","TestGoalLandingPublicationAndVerification","TestGoalBranchSweepRules","TestOpenClaimDoneLifecycle","TestLastArcGoalConclusionRaisesRetroDebtWhenSweepFails","TestGoalBranchSweepListsParkedDoneAbandonedAndOrphan","TestStatusAndReadBindWholeBuildList","TestAmendReplacesWholeBuildList","TestGoalLandingKeepsBuildUnitList","TestGoalBranchLastLandingSweepsGoalBranch","TestGoalListJSONKeepsItsShapeAndRequiresHistoryFlag","TestEmptyReadItemListPreservesRecordBytesAndDigest","TestReadItemsRoundTripBesideNextStep","TestReadItemsAddIsIdempotent","TestReadItemIDsStayStableAcrossAddCalls","TestReadItemsCloseNeedsExactlyOneWay","TestReadItemsFixedRefusesUnknownCommit","TestReadItemsMoveIsAtomicAndRefusesDoneTarget","TestReadItemsAcceptedRefusesBlankReason","TestReadItemsCloseRefusesClosedItem","TestDoneRefusesOpenReadItemIDsAndPassesWhenClosed","TestRetroReadItemsListsOpenAndFlagsConcludedDefect","TestGoalShowAndNextPrintOpenReadItemFixUnit","TestGoalReadItemsListJSONShape","TestReadItemsFileSkipsBlanksAndComments","TestGoalBranchGitRunnersSupplyTestingMergeDriver","TestBranchLandingSyncMergesTestingContractBySurface","TestEveryTerminalGoalTransitionRefusesOpenReadItems","TestDoneReadItemRefusalRemedyExecutes","TestCadenceClaimDeduplicatesTreeAcrossMachines","TestCadenceRedPublishesOwnerlessUntilHumanNamesGoal","TestCadenceStateAbsentLeavesLedgerBytesUnchanged","TestCadenceDeadClaimIsRecoverableAfterLease","TestGoalBranchVerbsRunFromTheHoldersLinkedWorktree","TestGoalBranchCommitRefusesToMoveAnArmedCheckout","TestClaimQuotaRefusalNamesTheHeldGoalAndRelease","TestGoalBranchHolderResolutionPreservesInstallationSubdirectory","TestTrunkRedBatchGuardsExcludeCadenceRecords","TestGoalBranchReadRunsGateDispatchesAndCollectsClosedCritic","TestGoalBranchReadRedGateAndUncleanClosureDispatchNothingFurther","TestLandingMessageAcceptsAttestedBranchProvenance","TestGoalBranchReadDelegateUsesBinarySeamAndReturnsWithoutWaiting","TestCommitReadRefusesUnrecordedFastGateRun","TestAdoptRemoteTipRestoresHeadWhenTheRefUpdateFails","TestBindLandedUnitRefusesOmittedFold","TestCriticAttestationSurvivesFreshCloneWithoutJobStore","TestRecoveryRefusesParkWhenGoalBranchIsUnpushed","TestRecoveryCompletesParkWhenGoalBranchIsPushed","TestParkFailsClosedWithoutBranchChecker","TestRestampVerbRefusesTheByFlag","TestReadInstallRefusalLeavesNothingStaged","TestReadCommitKeepsDirtyLedgerWithoutAdoption","TestAmendInstallFailureRollsBackMovedCheckout","TestAmendDroppingReadStillChecksReplayedTree","TestPlainCommitKeepsDirtyLedgerWithoutAdoption","TestAdoptionUntrackedCollisionRefusesStale","TestGoalLandingCommitDatesComeFromReceiptStamp","TestLandThroughIgnoresStopFenceReason","TestGoalLandingRefusesFoldWithStalePreimage","TestMirrorDeleteReconcilesUnknownOutcome","TestLandPushRerunAfterPublishedCrashReportsLanded","TestLandPushRechecksClaimBeforePush","TestParkRefusalDoesNotClaimOriginState","TestSweepRefusesDirtyGoalWorktree","TestSweepRefusesTransportTipWithUnlandedCommit","TestSweepCleansLeftoversWhenOriginBranchIsGone","TestSweepRequiresExactConcludedDroppedDeclaration","TestParkKeepsNextStepNarrativeAndAddsBranchSummary","TestSyncRequestPublishesEpochAuthorityForTheHolderOnly","TestHolderSetBudgetRebindsEpochForProofAdmission","TestBudgetEpisodeRevisionTransitions","TestApprovalEpisodeRevisionGrammar","TestBudgetEpisodeRevisionLegacyMinimum","TestLegacyApprovalWithReadItemsRoundTripsByteIdentical","TestHistoryLineResumedField","TestRecoveryRefusesJournaledSetBudgetAuthority","TestSetBudgetLiftsCompletedFenceInOneTransaction","TestSetBudgetFencedSameTupleRefusesWithoutMutation","TestSetBudgetFencedIncompleteBatchNamesRemedy","TestSetBudgetFencedOtherClaimNamesConflict","TestRebindEpochFollowsTheAuthenticatedHolderOnly","TestSweepRefusesUnlandedLocalTip","TestSweepRefusalPreservesRefsAndWorktree","TestSweepRefusalNamesEveryUnlandedCommit","TestSweepKeepsLocalRefMovedAfterCheck","TestSweepLocalTipExemptions","TestSweepAbandonedStillValidatesRemoteRange","TestWithLockDeadlineUsesInjectedClock","TestPublishRetryDeadlineUsesInjectedClock"]},
-    {"id":"wait-stop-standard","kind":"integration","adapter":"go","cwd":"metasystem","inputs":["metasystem/go.mod","metasystem/go.sum","metasystem/internal/goal/**","metasystem/internal/adapter/**","metasystem/internal/run/**","metasystem/internal/proofrun/**","metasystem/cmd/metasystem/wait_verb.go","metasystem/cmd/metasystem/wait_verb_test.go","metasystem/scripts/agents/adapters/**","metasystem/scripts/agents/supervision-fixtures.sh","metasystem/docs/design/turn-verdict-delivery-contract.md","metasystem/internal/report/**"],"outputs":[],"tools":[{"id":"go","executable":"go","versionArgs":["version"]},{"id":"bash","executable":"bash","versionArgs":["--version"]},{"id":"git","executable":"git","versionArgs":["--version"]}],"obligations":["budget-stop-authority","runtime-custody"],"platforms":["any"],"targetMs":60000,"packages":["internal/goal","internal/adapter","cmd/metasystem","internal/report"],"tests":["TestPendingWaitTurnVerdict","TestPendingWaitIdleBacklog","TestWaitDeliveryContract","TestPendingWaitInstalledVerdicts","TestWaitPathSelectorIsValidated","TestNextStepNamesAPendingHumanWord","TestIdleBacklogContinuationSkipsAGoalWaitingOnAHumanWord","TestIdleBacklogContinuationLeavesAHeldGoalThatWaitsOnAHumanWord","TestIdleBacklogWithOnlyHumanWaitingGoalsPreparesNoContinuation","TestMapStopOutputWritesTheResponseRecord","TestMapStopOutputAcceptsChangedWording","TestStopLineIsRenderedFromTheReportReference","TestPruneRemovesTheResponseRecordWithItsReport","TestReportStopResponseResolvesUnderChangedWording","TestReportStopResponseRefusesAnUnreadableResponse","TestBedsResolveStopReportsThroughTheEngine","TestToolGateAllowlist","TestToolGateNeverDeniesLandingWaitOrAgent","TestToolGateAllowEmitsNothing","TestToolGateCeilingColumnEqualsTrigger","TestToolGateMemoryNoteRule","TestWaitRegisterLocalRecordsTheTrackedProcess","TestWaitRegisterHumanNeedsAQuestionAndADeadline","TestRegisteredLocalAndHumanWaitsInstalledVerdicts","TestLocalWaitAllowsTheStopWhileItsProcessLives","TestLocalWaitOfADeadProcessDoesNotAllowTheStop","TestHumanWaitAllowsTheStopUntilItsDeadline","TestLocalWaitCoversTheJobItNames","TestToolGateDeadlineCountsFromTheShellBirth","TestToolGateFallsBackToEntryWhenBirthUnreadable","TestToolGateClassifiesBeforeReading","TestToolGateReadOptionsAreNonBlocking","TestToolGateAllowsNativeSubagentCalls","TestToolGateSubagentCallsWriteNoRow","TestToolGateAllowsPastItsDeadline","TestToolGateNoDecisionWhenTheCallStoreIsBusy","TestToolGateLeavesTheCursor","TestToolGateWritesDecisionRows","TestToolGateObserveModeAllowsAndRecordsTheDenyDecision","TestToolGateDenyModeDenies","TestClaudeJobSettingsInstallNoToolGate","TestWaitingLinesUseWaitEndForLiveLocalWait","TestWaitingLinesUseWaitEndForPendingHumanWait"],"race":false,"coverage":false},
+    {"id":"wait-stop-standard","kind":"integration","adapter":"go","cwd":"metasystem","inputs":["metasystem/go.mod","metasystem/go.sum","metasystem/internal/goal/**","metasystem/internal/adapter/**","metasystem/internal/run/**","metasystem/internal/proofrun/**","metasystem/cmd/metasystem/wait_verb.go","metasystem/cmd/metasystem/wait_verb_test.go","metasystem/scripts/agents/adapters/**","metasystem/scripts/agents/supervision-fixtures.sh","metasystem/scripts/agents/supervision-hook.sh","metasystem/scripts/agents/stop-degraded-forms.sh","metasystem/scripts/enforcement/claude-code-hooks.json","metasystem/docs/design/turn-verdict-delivery-contract.md","metasystem/internal/report/**"],"outputs":[],"tools":[{"id":"go","executable":"go","versionArgs":["version"]},{"id":"bash","executable":"bash","versionArgs":["--version"]},{"id":"git","executable":"git","versionArgs":["--version"]}],"obligations":["budget-stop-authority","runtime-custody"],"platforms":["any"],"targetMs":60000,"packages":["internal/goal","internal/adapter","cmd/metasystem","internal/report"],"tests":["TestPendingWaitTurnVerdict","TestPendingWaitIdleBacklog","TestWaitDeliveryContract","TestPendingWaitInstalledVerdicts","TestWaitPathSelectorIsValidated","TestNextStepNamesAPendingHumanWord","TestIdleBacklogContinuationSkipsAGoalWaitingOnAHumanWord","TestIdleBacklogContinuationLeavesAHeldGoalThatWaitsOnAHumanWord","TestIdleBacklogWithOnlyHumanWaitingGoalsPreparesNoContinuation","TestMapStopOutputWritesTheResponseRecord","TestMapStopOutputAcceptsChangedWording","TestStopLineIsRenderedFromTheReportReference","TestPruneRemovesTheResponseRecordWithItsReport","TestReportStopResponseResolvesUnderChangedWording","TestReportStopResponseRefusesAnUnreadableResponse","TestBedsResolveStopReportsThroughTheEngine","TestToolGateAllowlist","TestToolGateNeverDeniesLandingWaitOrAgent","TestToolGateAllowEmitsNothing","TestToolGateCeilingColumnEqualsTrigger","TestToolGateMemoryNoteRule","TestWaitRegisterLocalRecordsTheTrackedProcess","TestWaitRegisterHumanNeedsAQuestionAndADeadline","TestRegisteredLocalAndHumanWaitsInstalledVerdicts","TestLocalWaitAllowsTheStopWhileItsProcessLives","TestLocalWaitOfADeadProcessDoesNotAllowTheStop","TestHumanWaitAllowsTheStopUntilItsDeadline","TestLocalWaitCoversTheJobItNames","TestToolGateDeadlineCountsFromTheShellBirth","TestToolGateFallsBackToEntryWhenBirthUnreadable","TestToolGateClassifiesBeforeReading","TestToolGateReadOptionsAreNonBlocking","TestToolGateAllowsNativeSubagentCalls","TestToolGateSubagentCallsWriteNoRow","TestToolGateAllowsPastItsDeadline","TestToolGateNoDecisionWhenTheCallStoreIsBusy","TestToolGateLeavesTheCursor","TestToolGateWritesDecisionRows","TestToolGateObserveModeAllowsAndRecordsTheDenyDecision","TestToolGateDenyModeDenies","TestClaudeJobSettingsInstallNoToolGate","TestWaitingLinesUseWaitEndForLiveLocalWait","TestWaitingLinesUseWaitEndForPendingHumanWait","TestFakeStopReplay"],"race":false,"coverage":false},
     {"id":"obligationstate-standard","kind":"unit","adapter":"go","cwd":"metasystem","inputs":["metasystem/go.mod","metasystem/go.sum","metasystem/internal/obligationstate/**"],"outputs":[],"tools":[{"id":"go","executable":"go","versionArgs":["version"]}],"obligations":["budget-stop-authority"],"platforms":["any"],"targetMs":5000,"packages":["internal/obligationstate"],"tests":"all"},
     {"id":"mission-decision-standard","kind":"unit","adapter":"go","cwd":"metasystem","inputs":["metasystem/go.mod","metasystem/go.sum","metasystem/internal/missionrunner/**"],"outputs":[],"tools":[{"id":"go","executable":"go","versionArgs":["version"]}],"obligations":["budget-stop-authority"],"platforms":["any"],"targetMs":6000,"packages":["internal/missionrunner"],"tests":["TestProjectFences","TestProjectFencesRefusesBadReservations","TestConcludeTurnStatusDecision","TestAcquireLeaseLifecycle","TestVerifyStateShapeRefusals","TestStopDeadRunnerReleasesLeaseAndClosesOrphanHost","TestStopLiveRunnerSignalsOwnedGroup"]},
     {"id":"goal-full-coverage","kind":"unit","adapter":"go","cwd":"metasystem","inputs":["metasystem/go.mod","metasystem/go.sum","metasystem/internal/goal/**","metasystem/scripts/agents/coverage-ratchet.json","metasystem/scripts/agents/coverage-ratchet-linux.json"],"outputs":[],"tools":[{"id":"go","executable":"go","versionArgs":["version"]}],"obligations":["goal-package-coverage"],"platforms":["any"],"targetMs":1800000,"packages":["internal/goal"],"tests":"all","coverage":true,"shards":4},
```

## Helper pids

No manual helper PID was started. Backgrounded required checks were run through managed shell sessions and waited to completion. The hook fixture bed reported owner PID 54361 and exited; its child-survivor scan could not be certified because `kern.proc.all` is denied in this sandbox.

## Commit message

Report unconfirmed stop incident delivery

Tell the user when neither the hook log nor the refusal record accepted an incident. Add the fixture-owned deadline seam and fake stop replay proof action.

Goal-Unit: stop-hook-never-forces-an-empty-turn/stop-notice-says-delivery-unconfirmed
