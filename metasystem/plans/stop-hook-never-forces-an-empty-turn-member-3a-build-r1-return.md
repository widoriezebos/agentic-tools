BUILD: blocked sandbox loopback binds in cmd/metasystem check

## Start state

`git rev-parse HEAD`

```text
92743c95110f8682d84599fb88f9cdf156b9d3b0
```

`git status --porcelain`

```text
```

`shasum -a 256 /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct/stop-hook-members-design-r2.frozen.md`

```text
9ba7a005e4336b6313231d319c3c8b7a175cb7158a55e3033956e7fbbdff7461  /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1b/127d7cba-2e99-40bb-822e-6c6c855d496f/scratchpad/direct/stop-hook-members-design-r2.frozen.md
```

## Changed files

New files included separately in `dm-stopinc-3a.diff`:

- `metasystem/internal/stopincident/stopincident.go`
- `metasystem/internal/stopincident/stopincident_test.go`
- `metasystem/internal/steward/stopincident.go`

`git diff --numstat HEAD`

```text
13	1	metasystem/cmd/metasystem/report.go
1	0	metasystem/cmd/metasystem/steward_verbs.go
37	5	metasystem/internal/report/stopblock.go
43	10	metasystem/internal/report/stopblock_test.go
6	3	metasystem/internal/steward/intervene.go
29	0	metasystem/internal/steward/notify.go
128	0	metasystem/internal/steward/notify_test.go
5	2	metasystem/internal/steward/tick.go
198	0	metasystem/internal/steward/tick_test.go
6	2	metasystem/scripts/agents/supervision-hook.sh
1	1	metasystem/testing-parallel-ratchet.json
2	2	metasystem/testing.json
```

`git status --porcelain`

```text
 M metasystem/cmd/metasystem/report.go
 M metasystem/cmd/metasystem/steward_verbs.go
 M metasystem/internal/report/stopblock.go
 M metasystem/internal/report/stopblock_test.go
 M metasystem/internal/steward/intervene.go
 M metasystem/internal/steward/notify.go
 M metasystem/internal/steward/notify_test.go
 M metasystem/internal/steward/tick.go
 M metasystem/internal/steward/tick_test.go
 M metasystem/scripts/agents/supervision-hook.sh
 M metasystem/testing-parallel-ratchet.json
 M metasystem/testing.json
?? metasystem/internal/steward/stopincident.go
?? metasystem/internal/stopincident/
```

## Red before green

`go test -count=1 -run 'TestStopIncidentDrainBeforeNarrator|TestStopIncidentDrainFromHookLog|TestStopIncidentOutageRecoveryOutage|TestStopIncidentDeliveryUnconfirmed|TestHookLogLinesRoundTrip|TestStopRefusalRecordKeepsTheClass' ./internal/stopincident/ ./internal/steward/ ./internal/report/`

```text
github.com/widoriezebos/agentic-tools/metasystem/internal/stopincident: no non-test Go files in /Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3a/metasystem/internal/stopincident
# github.com/widoriezebos/agentic-tools/metasystem/internal/stopincident [github.com/widoriezebos/agentic-tools/metasystem/internal/stopincident.test]
internal/stopincident/stopincident_test.go:25:12: undefined: AppendAck
internal/stopincident/stopincident_test.go:29:27: undefined: ReadHookLog
internal/stopincident/stopincident_test.go:39:10: undefined: Incident
internal/stopincident/stopincident_test.go:39:173: undefined: SourceHookLog
internal/stopincident/stopincident_test.go:43:5: undefined: Identity
internal/stopincident/stopincident_test.go:44:50: undefined: Identity
internal/stopincident/stopincident_test.go:46:5: undefined: Identity
internal/stopincident/stopincident_test.go:49:5: undefined: Identity
FAIL	github.com/widoriezebos/agentic-tools/metasystem/internal/stopincident [build failed]
FAIL	github.com/widoriezebos/agentic-tools/metasystem/internal/steward [build failed]
FAIL	github.com/widoriezebos/agentic-tools/metasystem/internal/report [build failed]
FAIL
```

## Checks

`gofmt -l ./internal/stopincident ./internal/steward ./internal/report ./cmd/metasystem`

```text
```

`go vet ./internal/stopincident/ ./internal/steward/ ./internal/report/ ./cmd/metasystem/`

```text
```

`go vet -tags batchtest ./internal/stopincident/ ./internal/steward/ ./internal/report/ ./cmd/metasystem/`

```text
```

`go test -count=1 -run 'TestStopIncidentDrainBeforeNarrator|TestStopIncidentDrainFromHookLog|TestStopIncidentOutageRecoveryOutage|TestStopIncidentDeliveryUnconfirmed|TestHookLogLinesRoundTrip|TestStopRefusalRecordKeepsTheClass' ./internal/stopincident/ ./internal/steward/ ./internal/report/`

```text
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/stopincident	0.259s
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/steward	1.307s
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/report	0.495s
```

`go test -count=1 -tags batchtest -run 'TestStopIncidentDrainBeforeNarrator|TestStopIncidentDrainFromHookLog|TestStopIncidentOutageRecoveryOutage|TestStopIncidentDeliveryUnconfirmed|TestHookLogLinesRoundTrip|TestStopRefusalRecordKeepsTheClass' ./internal/stopincident/ ./internal/steward/ ./internal/report/`

```text
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/stopincident	0.239s
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/steward	1.367s
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/report	0.477s
```

`go test -count=1 ./internal/stopincident/ ./internal/steward/ ./internal/report/`

```text
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/stopincident	0.117s
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/steward	159.977s
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/report	10.535s
```

`go test -count=1 -skip 'TestProcFixtureSurvivors|TestProcessCensus|TestCustodianWatches|TestLauncherDeath|TestAllPids|TestProcessesWithWithheldArguments' ./cmd/metasystem/`

```text
ambiguous-session-correlation:a,b
behavior-surface output: write /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestBehaviorSurfaceSelectFailsOnFlushError2511353285/001/input: bad file descriptor
behavior-surface output: write /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestBehaviorSurfaceListFailsOnFlushError1862847639/002/input: bad file descriptor
behavior-surface output: write /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestBehaviorSurfaceDirectWritersFailOnOutputErrorpolicy2291203184/001/input: bad file descriptor
behavior-surface output: write /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestBehaviorSurfaceDirectWritersFailOnOutputErrorclassify1503008379/001/input: bad file descriptor
behavior-surface output: write /var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/TestBehaviorSurfaceDirectWritersFailOnOutputErrordigest699434434/001/input: bad file descriptor
brain start-delivered: the declaration changed before delivery acknowledgment
proc find-ancestor: --all-hosts and --runtime are mutually exclusive
--- FAIL: TestWaitChannelAnswer (0.02s)
    channel_verbs_test.go:84: fake did not start: listen tcp 127.0.0.1:0: bind: operation not permitted
--- FAIL: TestChannelStatusPostSeedsAnUnbootedBrainStatus (0.00s)
    channel_verbs_test.go:228: fake did not start: listen tcp 127.0.0.1:0: bind: operation not permitted
--- FAIL: TestTelegramPeekWorksWithoutConfiguredAdapterOrChatID (0.00s)
    channel_verbs_test.go:270: fake did not start: listen tcp 127.0.0.1:0: bind: operation not permitted
--- FAIL: TestTelegramPeekTokenNeverAppearsInErrors (0.00s)
    --- FAIL: TestTelegramPeekTokenNeverAppearsInErrors/redirect (0.00s)
panic: httptest: failed to listen on a port: listen tcp6 [::1]:0: bind: operation not permitted [recovered, repanicked]

goroutine 331 [running]:
testing.tRunner.func1.2({0x101e87c68, 0x7e36086301e0})
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2123 +0x1a0
testing.tRunner.func1()
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2126 +0x2c8
panic({0x101e87c68?, 0x7e36086301e0?})
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/runtime/panic.go:859 +0x120
net/http/httptest.newLocalListener()
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/net/http/httptest/server.go:226 +0xd4
net/http/httptest.NewUnstartedServer(...)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/net/http/httptest/server.go:284
net/http/httptest.NewServer({0x101f65810, 0x101f6ebd0})
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/net/http/httptest/server.go:267 +0x28
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestTelegramPeekTokenNeverAppearsInErrors.func2()
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3a/metasystem/cmd/metasystem/channel_verbs_test.go:294 +0x2c
github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem.TestTelegramPeekTokenNeverAppearsInErrors.func5(0x7e3608aa0248)
	/Users/wido/LocalStorage/GitHub/agentic-tools-m1b/.claude/worktrees/stopinc3a/metasystem/cmd/metasystem/channel_verbs_test.go:313 +0x38
testing.tRunner(0x7e3608aa0248, 0x7e360847a000)
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2193 +0xc4
created by testing.(*T).Run in goroutine 315
	/opt/homebrew/Cellar/go/1.27.1/libexec/src/testing/testing.go:2258 +0x3b8
FAIL	github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem	2.777s
FAIL
```

`go test -count=1 -run 'TestProofAttempt|TestLandFixture|TestEveryPackageUsesSharedMain|TestBedsResolveStopReportsThroughTheEngine' ./cmd/metasystem/`

```text
ok  	github.com/widoriezebos/agentic-tools/metasystem/cmd/metasystem	0.438s
```

`bash -n scripts/agents/supervision-hook.sh`

```text
```

`bash -n scripts/agents/supervision-hook-fixtures.sh`

```text
```

`go build ./...`

```text
```

`go build -tags batchtest ./...`

```text
```

`go test -count=1 -run 'TestTestEnvironmentStandardInventoryMatchesPackageTests' ./internal/testenv/`

```text
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/testenv	0.524s
```

`go test -count=1 -run 'TestRepositoryManifestClassifiesEveryTrackedPath' ./internal/pathclass/`

```text
ok  	github.com/widoriezebos/agentic-tools/metasystem/internal/pathclass	0.354s
```

`go run ./cmd/metasystem audit parallel-ratchet --root .`

```text
parallel ratchet passed
```

`bash scripts/agents/go-gate.sh --fast`

```text
go gate: effective Go: go version go1.27.1 darwin/arm64
go gate: GOTOOLCHAIN: auto
stop decision surface: base 92743c95110f8682d84599fb88f9cdf156b9d3b0; added 0, moved 0, removed 0
go gate: fast mode passed (dependency ratchet, parallel ratchet, gofmt, vet, staticcheck, refusal register, SessionStart exit audit, Stop decision surface audit, build); the full gate remains the landing requirement
```

Runtime verification through the real CLI entrypoint:

`go run ./cmd/metasystem report stop-block --class infrastructure --refusal-record "$verification_dir/session.json" --session session-a --cause 'supervision arming failed' --remedy 'repair supervision' --cause-code supervision-arming-failed --component supervision-arming --generation 4 --deadline-end 1789819260 'degraded infrastructure'`

```text
{"systemMessage":"Metasystem allowed stopping with degraded infrastructure (occurrence 1).\nCause: supervision arming failed\nRemedy: repair supervision\ndegraded infrastructure"}
```

The written version-one record contained class `infrastructure`, identity `654d5cced3fcf4c1`, cause code `supervision-arming-failed`, component `supervision-arming`, generation `4`, and deadline end `1789819260`.

## Mutations

TestStopIncidentDrainBeforeNarrator

```text
--- FAIL: TestStopIncidentDrainBeforeNarrator (1.02s)
    tick_test.go:109: stop incidents queued=0 before narrator failure, want 1: {Queued:0 Pending:0 Unconfirmed:false Detail:}
```

TestStopIncidentDrainFromHookLog

```text
--- FAIL: TestStopIncidentDrainFromHookLog (0.10s)
    tick_test.go:160: hook drain={Queued:3 Pending:3 Unconfirmed:false Detail:}, want exactly the two current infrastructure incidents
```

TestStopIncidentOutageRecoveryOutage

```text
--- FAIL: TestStopIncidentOutageRecoveryOutage (0.08s)
    tick_test.go:231: recovered hook drain={Queued:1 Pending:1 Unconfirmed:false Detail:}, want acknowledgment repair only
```

TestStopIncidentDeliveryUnconfirmed

```text
--- FAIL: TestStopIncidentDeliveryUnconfirmed (0.15s)
    --- FAIL: TestStopIncidentDeliveryUnconfirmed/ledger_append_failure_never_redelivers (0.06s)
        notify_test.go:333: stamped retry delivered=1 calls=2 err=<nil>, want removal without redelivery
```

TestHookLogLinesRoundTrip

```text
--- FAIL: TestHookLogLinesRoundTrip (0.00s)
    stopincident_test.go:44: outcome changed identity: b7e48dcf32e5f30f != 39aedf2767d01e59
```

TestStopRefusalRecordKeepsTheClass

```text
--- FAIL: TestStopRefusalRecordKeepsTheClass (0.03s)
    stopblock_test.go:227: ReadRefusalRecords returned 0 incidents, want 1: []
```

`git diff --stat HEAD` before the first mutation:

```text
 metasystem/cmd/metasystem/report.go           |  14 +-
 metasystem/cmd/metasystem/steward_verbs.go    |   1 +
 metasystem/internal/report/stopblock.go       |  42 +++++-
 metasystem/internal/report/stopblock_test.go  |  53 ++++++--
 metasystem/internal/steward/intervene.go      |   9 +-
 metasystem/internal/steward/notify.go         |  29 ++++
 metasystem/internal/steward/notify_test.go    | 128 ++++++++++++++++++
 metasystem/internal/steward/tick.go           |   7 +-
 metasystem/internal/steward/tick_test.go      | 184 ++++++++++++++++++++++++++
 metasystem/scripts/agents/supervision-hook.sh |   8 +-
 metasystem/testing.json                       |   4 +-
 11 files changed, 454 insertions(+), 25 deletions(-)
```

`git diff --stat HEAD` after the sixth mutation was restored:

```text
 metasystem/cmd/metasystem/report.go           |  14 +-
 metasystem/cmd/metasystem/steward_verbs.go    |   1 +
 metasystem/internal/report/stopblock.go       |  42 +++++-
 metasystem/internal/report/stopblock_test.go  |  53 ++++++--
 metasystem/internal/steward/intervene.go      |   9 +-
 metasystem/internal/steward/notify.go         |  29 ++++
 metasystem/internal/steward/notify_test.go    | 128 ++++++++++++++++++
 metasystem/internal/steward/tick.go           |   7 +-
 metasystem/internal/steward/tick_test.go      | 184 ++++++++++++++++++++++++++
 metasystem/scripts/agents/supervision-hook.sh |   8 +-
 metasystem/testing.json                       |   4 +-
 11 files changed, 454 insertions(+), 25 deletions(-)
```

The later required parallel-ratchet check added the separately owned one-line baseline update to `testing-parallel-ratchet.json`; it was not present during mutation comparison.

## Sandbox reds

- The required broad `cmd/metasystem` package run is red only because loopback bind is refused. `TestWaitChannelAnswer`, `TestChannelStatusPostSeedsAnUnbootedBrainStatus`, and `TestTelegramPeekWorksWithoutConfiguredAdapterOrChatID` report `fake did not start: listen tcp 127.0.0.1:0: bind: operation not permitted`; `TestTelegramPeekTokenNeverAppearsInErrors/redirect` panics when `httptest` cannot bind `[::1]:0`.
- `scripts/agents/supervision-hook-fixtures.sh` is HOST-ONLY and was not executed because this sandbox refuses its `nice(5)` call, as declared in the brief. Its Bash syntax check is green.
- A first attempt to background the whole-package check printed `nice(5) failed: operation not permitted`; the check was rerun by polling one foreground test session and passed.

## New tests

- `TestStopIncidentDrainBeforeNarrator` — package `internal/steward`, group `context-standard`
- `TestStopIncidentDrainFromHookLog` — package `internal/steward`, group `context-standard`
- `TestStopIncidentOutageRecoveryOutage` — package `internal/steward`, group `context-standard`
- `TestStopIncidentDeliveryUnconfirmed` — package `internal/steward`, group `context-standard`
- `TestHookLogLinesRoundTrip` — package `internal/stopincident`, group `context-standard`
- `TestStopRefusalRecordKeepsTheClass` — package `internal/report`, group `wait-stop-standard`

The parallel-ratchet audit required one additional contract-owned placement: `testing-parallel-ratchet.json` raises the steward serial-test ceiling from 398 to 401 for the three tests that must not call `t.Parallel()` because they replace package variables.

Testing contract hunk:

```diff
diff --git a/metasystem/testing.json b/metasystem/testing.json
index f9df33fa1..cd7116ce4 100644
--- a/metasystem/testing.json
+++ b/metasystem/testing.json
@@ -50 +50 @@
-    {"id":"wait-stop-standard","kind":"integration","adapter":"go","cwd":"metasystem","inputs":["metasystem/go.mod","metasystem/go.sum","metasystem/internal/goal/**","metasystem/internal/adapter/**","metasystem/internal/run/**","metasystem/internal/proofrun/**","metasystem/cmd/metasystem/wait_verb.go","metasystem/cmd/metasystem/wait_verb_test.go","metasystem/scripts/agents/adapters/**","metasystem/scripts/agents/supervision-fixtures.sh","metasystem/docs/design/turn-verdict-delivery-contract.md","metasystem/internal/report/**"],"outputs":[],"tools":[{"id":"go","executable":"go","versionArgs":["version"]},{"id":"bash","executable":"bash","versionArgs":["--version"]},{"id":"git","executable":"git","versionArgs":["--version"]}],"obligations":["budget-stop-authority","runtime-custody"],"platforms":["any"],"targetMs":60000,"packages":["internal/goal","internal/adapter","cmd/metasystem","internal/report"],"tests":["TestPendingWaitTurnVerdict","TestPendingWaitIdleBacklog","TestWaitDeliveryContract","TestPendingWaitInstalledVerdicts","TestWaitPathSelectorIsValidated","TestNextStepNamesAPendingHumanWord","TestIdleBacklogContinuationSkipsAGoalWaitingOnAHumanWord","TestIdleBacklogContinuationLeavesAHeldGoalThatWaitsOnAHumanWord","TestIdleBacklogWithOnlyHumanWaitingGoalsPreparesNoContinuation","TestMapStopOutputWritesTheResponseRecord","TestMapStopOutputAcceptsChangedWording","TestStopLineIsRenderedFromTheReportReference","TestPruneRemovesTheResponseRecordWithItsReport","TestReportStopResponseResolvesUnderChangedWording","TestReportStopResponseRefusesAnUnreadableResponse","TestBedsResolveStopReportsThroughTheEngine","TestToolGateAllowlist","TestToolGateNeverDeniesLandingWaitOrAgent","TestToolGateAllowEmitsNothing","TestToolGateCeilingColumnEqualsTrigger","TestToolGateMemoryNoteRule","TestWaitRegisterLocalRecordsTheTrackedProcess","TestWaitRegisterHumanNeedsAQuestionAndADeadline","TestRegisteredLocalAndHumanWaitsInstalledVerdicts","TestLocalWaitAllowsTheStopWhileItsProcessLives","TestLocalWaitOfADeadProcessDoesNotAllowTheStop","TestHumanWaitAllowsTheStopUntilItsDeadline","TestLocalWaitCoversTheJobItNames","TestToolGateDeadlineCountsFromTheShellBirth","TestToolGateFallsBackToEntryWhenBirthUnreadable","TestToolGateClassifiesBeforeReading","TestToolGateReadOptionsAreNonBlocking","TestToolGateAllowsNativeSubagentCalls","TestToolGateSubagentCallsWriteNoRow","TestToolGateAllowsPastItsDeadline","TestToolGateNoDecisionWhenTheCallStoreIsBusy","TestToolGateLeavesTheCursor","TestToolGateWritesDecisionRows","TestToolGateObserveModeAllowsAndRecordsTheDenyDecision","TestToolGateDenyModeDenies","TestClaudeJobSettingsInstallNoToolGate","TestWaitingLinesUseWaitEndForLiveLocalWait","TestWaitingLinesUseWaitEndForPendingHumanWait"],"race":false,"coverage":false},
+    {"id":"wait-stop-standard","kind":"integration","adapter":"go","cwd":"metasystem","inputs":["metasystem/go.mod","metasystem/go.sum","metasystem/internal/goal/**","metasystem/internal/adapter/**","metasystem/internal/run/**","metasystem/internal/proofrun/**","metasystem/cmd/metasystem/wait_verb.go","metasystem/cmd/metasystem/wait_verb_test.go","metasystem/scripts/agents/adapters/**","metasystem/scripts/agents/supervision-fixtures.sh","metasystem/docs/design/turn-verdict-delivery-contract.md","metasystem/internal/report/**"],"outputs":[],"tools":[{"id":"go","executable":"go","versionArgs":["version"]},{"id":"bash","executable":"bash","versionArgs":["--version"]},{"id":"git","executable":"git","versionArgs":["--version"]}],"obligations":["budget-stop-authority","runtime-custody"],"platforms":["any"],"targetMs":60000,"packages":["internal/goal","internal/adapter","cmd/metasystem","internal/report"],"tests":["TestPendingWaitTurnVerdict","TestPendingWaitIdleBacklog","TestWaitDeliveryContract","TestPendingWaitInstalledVerdicts","TestWaitPathSelectorIsValidated","TestNextStepNamesAPendingHumanWord","TestIdleBacklogContinuationSkipsAGoalWaitingOnAHumanWord","TestIdleBacklogContinuationLeavesAHeldGoalThatWaitsOnAHumanWord","TestIdleBacklogWithOnlyHumanWaitingGoalsPreparesNoContinuation","TestMapStopOutputWritesTheResponseRecord","TestMapStopOutputAcceptsChangedWording","TestStopLineIsRenderedFromTheReportReference","TestPruneRemovesTheResponseRecordWithItsReport","TestReportStopResponseResolvesUnderChangedWording","TestReportStopResponseRefusesAnUnreadableResponse","TestBedsResolveStopReportsThroughTheEngine","TestToolGateAllowlist","TestToolGateNeverDeniesLandingWaitOrAgent","TestToolGateAllowEmitsNothing","TestToolGateCeilingColumnEqualsTrigger","TestToolGateMemoryNoteRule","TestWaitRegisterLocalRecordsTheTrackedProcess","TestWaitRegisterHumanNeedsAQuestionAndADeadline","TestRegisteredLocalAndHumanWaitsInstalledVerdicts","TestLocalWaitAllowsTheStopWhileItsProcessLives","TestLocalWaitOfADeadProcessDoesNotAllowTheStop","TestHumanWaitAllowsTheStopUntilItsDeadline","TestLocalWaitCoversTheJobItNames","TestToolGateDeadlineCountsFromTheShellBirth","TestToolGateFallsBackToEntryWhenBirthUnreadable","TestToolGateClassifiesBeforeReading","TestToolGateReadOptionsAreNonBlocking","TestToolGateAllowsNativeSubagentCalls","TestToolGateSubagentCallsWriteNoRow","TestToolGateAllowsPastItsDeadline","TestToolGateNoDecisionWhenTheCallStoreIsBusy","TestToolGateLeavesTheCursor","TestToolGateWritesDecisionRows","TestToolGateObserveModeAllowsAndRecordsTheDenyDecision","TestToolGateDenyModeDenies","TestClaudeJobSettingsInstallNoToolGate","TestWaitingLinesUseWaitEndForLiveLocalWait","TestWaitingLinesUseWaitEndForPendingHumanWait","TestStopRefusalRecordKeepsTheClass"],"race":false,"coverage":false},
@@ -60 +60 @@
-    {"id":"context-standard","kind":"unit","adapter":"go","cwd":"metasystem","inputs":["metasystem/go.mod","metasystem/go.sum","metasystem/cmd/**","metasystem/internal/**","metasystem/scripts/**","metasystem/docs/orchestration.md","metasystem/metasystem.conf","metasystem/testing.json"],"outputs":[],"tools":[{"id":"go","executable":"go","versionArgs":["version"]},{"id":"bash","executable":"bash","versionArgs":["--version"]},{"id":"git","executable":"git","versionArgs":["--version"]}],"obligations":[],"platforms":["any"],"targetMs":30000,"packages":["internal/usage","internal/runtimes","internal/output","internal/steward","internal/goal","cmd/metasystem"],"tests":["TestCodexReaderDecodesEachLineOnce","TestUnchangedEmptyTranscriptDoesNotRepublishCursor","TestLatestCallReturnsThePreviousReadTime","TestLatestCallNonBlockingReturnsBusy","TestRegisterSessionNonBlockingReturnsBusy","TestEveryDeclarationDeclaresContextSample","TestRuntimeContextSampleVerb","TestRoleContextRendersBoundCeilingAndUnknowns","TestHealthLineCarriesContextBudget","TestContextBudgetDoesNotAttributeUsageToAStaleHolder","TestContextHolderResolutionSkipsForeignMalformedAnnouncements","TestContextBudgetReturnsBusyUnknownWithoutWaiting","TestContextBudgetReturnsRegistryBusyUnknownWithoutWaiting","TestRoleContextNamesTheNewestSpill","TestNewestSinceReturnsOnlyANewerRegularFile","TestContextStatusVerbPrintsTheRoleLine","TestContextStatusExitCodesForReadableUnknownAndCeilingBreach","TestContextStatusJSONOmitsCursorHistory","TestContextStatusPropagatesEvidenceErrors","TestContextTranscriptOverrideUsesPrivateEvidence","TestContextTranscriptOverridePreservesTheNextHealthRead","TestContextTranscriptOverrideIgnoresLiveStoreFailures","TestContextTranscriptOverrideDisposesPrivateCursor","TestContextStatusLabelsTranscriptDiagnostics","TestCallRegistrationsReadsStrictSnapshot","TestCallSessionsDiscoversPairsWithoutReadingSamples","TestContextReportVerbPublishesTheWeek","TestContextReportComputesTheWeek","TestContextReportPropagatesRecoveryError","TestContextReportDeduplicatesTranscriptReplays","TestContextReportHandlesFallbackAndConflictingIdentities","TestContextReportWindowAndCoverage","TestContextReportExcludesTranscriptDiagnostics","TestContextCostRoleFromHealthLine","TestContextTestingContractSelectsProof","TestHandoffWritesAVerifiedStateFile","TestHandoffManifestPreservesOverflow","TestHandoffRefusals","TestHandoffRetriesNonceCollisionWithoutOverwriting","TestHandoffCleansDirectoryPublicationFailure","TestHandoffPublicationIsExclusiveAndReverified","TestLiveHandoffUsesNoExpiryClock","TestLiveHandoffReadDoesNotWaitOrWrite","TestHandoffAcceptsOnlyTheActiveContinuation","TestHandoffIgnoresConcurrentUnrelatedRecords","TestSecondHandoffSupersedesTheFirst","TestConcurrentHandoffsDoNotCross","TestCancelHandoffReportsItsExactPartialOutcome","TestVerifyHandoffStateUsesExactLifecycleRecord","TestHandoffRejectsInvalidLiveAuthority","TestContextPruneKeepsLiveHandoffs","TestContextPruneSerializesWithConsumption","TestContextPruneStopsOnUsageError","TestContextPruneDefaultAgeComposesWithUsageFloor","TestContextPruneRefusesRedirectedHandoffTrees","TestContextPruneRechecksBeforeRemoval","TestContextPruneReportsRemovalBeforeSyncFailure","TestContextPruneRetainsDamageAndRefusesBadBounds","TestHandoffAndDiagnosticsHaveSeparateLifetimes","TestTurnVerdictAllowsTheStopUnderARecordedHandoff","TestHandoffAllowanceCarriesFrozenFacts","TestContextHandoffVerb","TestContextVerifyAndCancel","TestContextPruneVerb","TestContextVerbUsage","TestHandoffRecordsDeclaredTasksFromTheTranscript","TestHandoffRecordsNoEmptyDelegateField","TestHandoffNoteSameSecondPassesWithANewDigest","TestHandoffNoteDirectoryIsPerRuntime","TestHandoffStateWithoutANoteStillReads","TestHandoffStateVerifierChecksLessonsNoteOrder","TestMemoryDirectoryResolvesTheProjectDirectory","TestMemoryDirectoryRefusesEscapes","TestAdapterClaudeToolGateVerb"],"race":false,"coverage":false},
+    {"id":"context-standard","kind":"unit","adapter":"go","cwd":"metasystem","inputs":["metasystem/go.mod","metasystem/go.sum","metasystem/cmd/**","metasystem/internal/**","metasystem/scripts/**","metasystem/docs/orchestration.md","metasystem/metasystem.conf","metasystem/testing.json"],"outputs":[],"tools":[{"id":"go","executable":"go","versionArgs":["version"]},{"id":"bash","executable":"bash","versionArgs":["--version"]},{"id":"git","executable":"git","versionArgs":["--version"]}],"obligations":[],"platforms":["any"],"targetMs":30000,"packages":["internal/stopincident","internal/usage","internal/runtimes","internal/output","internal/steward","internal/goal","cmd/metasystem"],"tests":["TestCodexReaderDecodesEachLineOnce","TestUnchangedEmptyTranscriptDoesNotRepublishCursor","TestLatestCallReturnsThePreviousReadTime","TestLatestCallNonBlockingReturnsBusy","TestRegisterSessionNonBlockingReturnsBusy","TestEveryDeclarationDeclaresContextSample","TestRuntimeContextSampleVerb","TestRoleContextRendersBoundCeilingAndUnknowns","TestHealthLineCarriesContextBudget","TestContextBudgetDoesNotAttributeUsageToAStaleHolder","TestContextHolderResolutionSkipsForeignMalformedAnnouncements","TestContextBudgetReturnsBusyUnknownWithoutWaiting","TestContextBudgetReturnsRegistryBusyUnknownWithoutWaiting","TestRoleContextNamesTheNewestSpill","TestNewestSinceReturnsOnlyANewerRegularFile","TestContextStatusVerbPrintsTheRoleLine","TestContextStatusExitCodesForReadableUnknownAndCeilingBreach","TestContextStatusJSONOmitsCursorHistory","TestContextStatusPropagatesEvidenceErrors","TestContextTranscriptOverrideUsesPrivateEvidence","TestContextTranscriptOverridePreservesTheNextHealthRead","TestContextTranscriptOverrideIgnoresLiveStoreFailures","TestContextTranscriptOverrideDisposesPrivateCursor","TestContextStatusLabelsTranscriptDiagnostics","TestCallRegistrationsReadsStrictSnapshot","TestCallSessionsDiscoversPairsWithoutReadingSamples","TestContextReportVerbPublishesTheWeek","TestContextReportComputesTheWeek","TestContextReportPropagatesRecoveryError","TestContextReportDeduplicatesTranscriptReplays","TestContextReportHandlesFallbackAndConflictingIdentities","TestContextReportWindowAndCoverage","TestContextReportExcludesTranscriptDiagnostics","TestContextCostRoleFromHealthLine","TestContextTestingContractSelectsProof","TestHandoffWritesAVerifiedStateFile","TestHandoffManifestPreservesOverflow","TestHandoffRefusals","TestHandoffRetriesNonceCollisionWithoutOverwriting","TestHandoffCleansDirectoryPublicationFailure","TestHandoffPublicationIsExclusiveAndReverified","TestLiveHandoffUsesNoExpiryClock","TestLiveHandoffReadDoesNotWaitOrWrite","TestHandoffAcceptsOnlyTheActiveContinuation","TestHandoffIgnoresConcurrentUnrelatedRecords","TestSecondHandoffSupersedesTheFirst","TestConcurrentHandoffsDoNotCross","TestCancelHandoffReportsItsExactPartialOutcome","TestVerifyHandoffStateUsesExactLifecycleRecord","TestHandoffRejectsInvalidLiveAuthority","TestContextPruneKeepsLiveHandoffs","TestContextPruneSerializesWithConsumption","TestContextPruneStopsOnUsageError","TestContextPruneDefaultAgeComposesWithUsageFloor","TestContextPruneRefusesRedirectedHandoffTrees","TestContextPruneRechecksBeforeRemoval","TestContextPruneReportsRemovalBeforeSyncFailure","TestContextPruneRetainsDamageAndRefusesBadBounds","TestHandoffAndDiagnosticsHaveSeparateLifetimes","TestTurnVerdictAllowsTheStopUnderARecordedHandoff","TestHandoffAllowanceCarriesFrozenFacts","TestContextHandoffVerb","TestContextVerifyAndCancel","TestContextPruneVerb","TestContextVerbUsage","TestHandoffRecordsDeclaredTasksFromTheTranscript","TestHandoffRecordsNoEmptyDelegateField","TestHandoffNoteSameSecondPassesWithANewDigest","TestHandoffNoteDirectoryIsPerRuntime","TestHandoffStateWithoutANoteStillReads","TestHandoffStateVerifierChecksLessonsNoteOrder","TestMemoryDirectoryResolvesTheProjectDirectory","TestMemoryDirectoryRefusesEscapes","TestAdapterClaudeToolGateVerb","TestStopIncidentDrainBeforeNarrator","TestStopIncidentDrainFromHookLog","TestStopIncidentOutageRecoveryOutage","TestStopIncidentDeliveryUnconfirmed","TestHookLogLinesRoundTrip"],"race":false,"coverage":false},
```

## Helper pids

- PID 61382 was the required whole-package test background attempt; the sandbox ended it immediately after refusing background `nice(5)`. It is not live. The check was rerun to completion in one polled session.
- PIDs 13748 and 13961 were background fast-gate wrapper attempts; the sandbox reaped both when their launching shells exited. Neither is live. The gate was rerun to completion in one polled session.
- No helper process remains running.

## Commit message

Deliver stop incidents to the steward

Drain infrastructure Stop incidents before narrator work, merge hook-log and refusal-record sources by stable identity, and persist delivery in a two-day append-only steward ledger. Retry unconfirmed deliveries without sending a stamped notification twice, repair hook acknowledgments, and expose the drain in steward tick output.

Add the hook coordinates, refusal-record incident history, six mutation-proven tests, testing-contract registrations, and the serial-test ratchet accounting required by the three package-variable tests.

Goal-Unit: stop-hook-never-forces-an-empty-turn/stop-incidents-reach-the-steward
