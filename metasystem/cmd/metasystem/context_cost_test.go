package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	usagepkg "github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

const (
	contextCostInitialCalls = 6638
)

type contextCostSourceStats struct {
	Bytes int64
	Lines int
	IDs   int
}

type contextCostSnapshot struct {
	cursor   []byte
	samples  []byte
	registry []byte
	calls    int
	markers  int
}

type contextCostProcess struct {
	command *exec.Cmd
	cancel  context.CancelFunc
	started time.Time
	output  bytes.Buffer
	done    chan struct{}
	err     error
}

type contextCostBarrierProcess struct {
	process *contextCostProcess
	ready   *os.File
	release *os.File
}

type contextCostStop struct {
	label      string
	output     string
	report     string
	elapsed    time.Duration
	role       steward.RoleVerdict
	reportRole string
	evidence   steward.ComponentEvidence
}

type contextCostBed struct {
	t            *testing.T
	runtime      string
	session      string
	outer        string
	installation string
	home         string
	engine       string
	wrapper      string
	hook         string
	transcript   string
	healthRecord string
	testBinary   string
	readerHelper string
	argvRecord   string
	startedAt    int64
	lastHookGen  int
	maxLabel     string
	maxElapsed   time.Duration
}

func TestTurnVerdictPrintsTheContextLine(t *testing.T) {
	root := contextTurnVerdictRoot(t, "context.ceiling.tokens=240000\ncontext.handoff.margin.tokens=140000\n")
	transcript := writeContextCommandTranscript(t, root, "context-line", 120500, 1, true)
	code, output, problem := captureChannelOutput(t, func() int {
		return runReportTurnVerdict([]string{"--root", root, "--session", "context-line", "--transcript", transcript, "--runtime", "claude"})
	})
	var verdict struct {
		Display string `json:"display"`
	}
	want := "CONTEXT: 121K of trigger 100K (proof line 150K, maximum 200K, ceiling 240K)"
	if code != 0 || problem != "" || json.Unmarshal([]byte(output), &verdict) != nil ||
		strings.Count(verdict.Display, "CONTEXT:") != 1 || !strings.Contains(verdict.Display, want) ||
		!contextLineRidesTheTrailer(verdict.Display, want) {
		t.Fatalf("turn verdict context: code=%d stderr=%q output=%q display=%q", code, problem, output, verdict.Display)
	}
}

// contextLineRidesTheTrailer: the line sits directly above the full-verdict
// path, never above the ladder the Stop beds read by position
// (goal-cli-fixtures.sh:1082-1090 and :1121-1127).
func contextLineRidesTheTrailer(display, want string) bool {
	lines := strings.Split(display, "\n")
	return len(lines) >= 2 && lines[0] != want && lines[len(lines)-2] == want &&
		strings.HasPrefix(lines[len(lines)-1], "Full turn verdict")
}

func TestTurnVerdictUnknownSampleNeverBlocks(t *testing.T) {
	tests := []struct {
		name, config, reason string
		args                 func(*testing.T, string) []string
	}{
		{"missing flags", "", "--transcript and --runtime are required", func(_ *testing.T, root string) []string {
			return []string{"--root", root, "--session", "missing-flags"}
		}},
		{"sampling error", "", "invalid runtime name", func(t *testing.T, root string) []string {
			path := writeContextCommandTranscript(t, root, "sampling-error", 120500, 1, true)
			return []string{"--root", root, "--session", "sampling-error", "--transcript", path, "--runtime", "INVALID"}
		}},
		{"long sampling error", "", "too long", func(_ *testing.T, root string) []string {
			path := filepath.Join(root, strings.Repeat("x", goal.TurnVerdictDisplayRuneLimit*2))
			return []string{"--root", root, "--session", "long-error", "--transcript", path, "--runtime", "claude"}
		}},
		{"no sample", "", "no call recorded yet", func(t *testing.T, root string) []string {
			path := filepath.Join(root, "empty.jsonl")
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			return []string{"--root", root, "--session", "no-sample", "--transcript", path, "--runtime", "claude"}
		}},
		{"invalid config", "context.ceiling.tokens=100000\ncontext.handoff.margin.tokens=100000\n", "CONTEXT_CONFIG_INVALID", func(t *testing.T, root string) []string {
			path := writeContextCommandTranscript(t, root, "invalid-config", 120500, 1, true)
			return []string{"--root", root, "--session", "invalid-config", "--transcript", path, "--runtime", "claude"}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := contextTurnVerdictRoot(t, test.config)
			code, output, problem := captureChannelOutput(t, func() int { return runReportTurnVerdict(test.args(t, root)) })
			var verdict struct {
				ShouldBlock bool    `json:"shouldBlock"`
				BlockSource *string `json:"blockSource"`
				Display     string  `json:"display"`
			}
			decodeErr := json.Unmarshal([]byte(output), &verdict)
			unknownLine := ""
			for _, line := range strings.Split(verdict.Display, "\n") {
				if strings.HasPrefix(line, "CONTEXT: unknown (") {
					unknownLine = line
				}
			}
			if code != 0 || problem != "" || decodeErr != nil || verdict.ShouldBlock ||
				verdict.BlockSource != nil || len([]rune(verdict.Display)) > goal.TurnVerdictDisplayRuneLimit || strings.Count(verdict.Display, "CONTEXT:") != 1 ||
				!strings.Contains(verdict.Display, "CONTEXT: unknown (") || !strings.Contains(verdict.Display, test.reason) ||
				!contextLineRidesTheTrailer(verdict.Display, unknownLine) {
				t.Fatalf("unknown context changed verdict: code=%d stderr=%q output=%q verdict=%+v", code, problem, output, verdict)
			}
		})
	}
}

func contextTurnVerdictRoot(t *testing.T, contextConfig string) string {
	t.Helper()
	root := contextCommandRoot(t)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"+contextConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_GOAL_NOW", "2026-09-16T12:00:00Z")
	return root
}

func TestStopHookPassesTranscriptAndRuntime(t *testing.T) {
	candidate := contextCostCandidateEngine(t, declaredContextCostCandidateEngine)
	bed := newContextCostBed(t, "claude", candidate, "")
	process := bed.startStop("argument-witness")
	<-process.done
	process.cancel()
	if process.err != nil {
		t.Fatalf("shipped Stop command failed: %v\n%s", process.err, process.output.String())
	}
	data, err := os.ReadFile(bed.argvRecord)
	if err != nil {
		t.Fatalf("read recorded turn-verdict argv: %v", err)
	}
	args := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
	for flag, want := range map[string]string{"--transcript": bed.transcript, "--runtime": "claude"} {
		found := false
		for index := 0; index+1 < len(args); index++ {
			if args[index] == flag && args[index+1] == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("recorded report turn-verdict argv omitted %s %q: %q", flag, want, args)
		}
	}
}

func TestContextStopFitsDurationBudget(t *testing.T) {
	if os.Getenv("METASYSTEM_CONTEXT_COST_PROOF") != "1" {
		t.Skip("large full-Stop context proof requires METASYSTEM_CONTEXT_COST_PROOF=1")
	}
	candidate := contextCostCandidateEngine(t, declaredContextCostCandidateEngine)
	readerHelper := contextCostReaderHelper(t)
	if info, err := os.Stat(candidate); err != nil || info.Mode()&0o111 == 0 {
		t.Fatalf("built candidate %s is not executable: %v", candidate, err)
	}

	for _, runtimeName := range []string{"claude", "codex"} {
		t.Run(runtimeName, func(t *testing.T) {
			bed := newContextCostBed(t, runtimeName, candidate, readerHelper)
			stats := writeContextCostSource(t, runtimeName, bed.transcript, bed.outer)
			t.Logf("%s generated source: bytes=%d complete-lines=%d distinct-ids=%d", runtimeName, stats.Bytes, stats.Lines, stats.IDs)
			if runtimeName == "codex" && (stats.Bytes < 264233967 || stats.Lines < 54471 || stats.IDs != contextCostInitialCalls) {
				t.Fatalf("Codex source is below its production-scale floor: %+v", stats)
			}
			if runtimeName == "claude" && (stats.Bytes < 47800000 || stats.IDs < 2000) {
				t.Fatalf("Claude source is below its production-scale floor: %+v", stats)
			}

			baseline := bed.runStop("baseline-no-holder")
			t.Logf("%s baseline without announced holder: elapsed=%s role-ms=%d role=%s", runtimeName, baseline.elapsed, baseline.role.DurationMillis, baseline.role.Line())
			bed.announceHolder()

			cold := bed.runStop("cold-large-source")
			bed.requireLiveRole(cold)
			warmStop := cold
			for index := 1; index <= 3; index++ {
				unchanged := bed.runStop(fmt.Sprintf("unchanged-%d", index))
				bed.requireLiveRole(unchanged)
				warmStop = unchanged
			}
			warmRole := warmStop.role
			bed.requireExpectedLiveRole("warm reference", warmRole)
			warmSnapshot := bed.snapshot()
			if warmSnapshot.calls != contextCostInitialCalls {
				t.Fatalf("%s warm committed calls=%d, want %d", runtimeName, warmSnapshot.calls, contextCostInitialCalls)
			}

			bed.runDiagnosticOverrides(warmSnapshot)
			afterDiagnostic := bed.runStop("after-diagnostic-overrides")
			bed.requireLiveRole(afterDiagnostic)
			if afterDiagnostic.role.Status != warmRole.Status || afterDiagnostic.role.Reason != warmRole.Reason {
				t.Fatalf("%s diagnostic changed the next live verdict: before=%s after=%s", runtimeName, warmRole.Line(), afterDiagnostic.role.Line())
			}
			bed.requireSnapshot("diagnostic and following unchanged Stop", warmSnapshot, bed.snapshot())

			appendContextCostCall(t, runtimeName, bed.transcript, contextCostInitialCalls, bed.outer)
			appended := bed.runStop("one-appended-call")
			bed.requireLiveRole(appended)
			if got := bed.snapshot().calls; got != contextCostInitialCalls+1 {
				t.Fatalf("%s appended committed calls=%d, want %d", runtimeName, got, contextCostInitialCalls+1)
			}

			replaceContextCostSource(t, bed.transcript)
			restarted := bed.runStop("inode-restart")
			bed.requireLiveRole(restarted)
			restartedSnapshot := bed.snapshot()
			if want := 2 * (contextCostInitialCalls + 1); restartedSnapshot.calls != want {
				t.Fatalf("%s restart history calls=%d, want %d", runtimeName, restartedSnapshot.calls, want)
			}
			bed.requireReport(contextCostInitialCalls + 1)

			bed.runLockedReaderContention()
			bed.runConcurrentColdRead()
			final := bed.runStop("final-unchanged")
			bed.requireLiveRole(final)
			finalSnapshot := bed.snapshot()
			if finalSnapshot.calls != contextCostInitialCalls+1 {
				t.Fatalf("%s final committed calls=%d, want %d", runtimeName, finalSnapshot.calls, contextCostInitialCalls+1)
			}
			cursorInfo, err := os.Stat(usagepkg.CursorPath(bed.installation, runtimeName, bed.session))
			if err != nil {
				t.Fatal(err)
			}
			samplesInfo, err := os.Stat(usagepkg.SamplesPath(bed.installation, runtimeName, bed.session))
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%s final evidence: cursor-bytes=%d samples-bytes=%d committed-calls=%d dominant-stop=%s elapsed=%s", runtimeName, cursorInfo.Size(), samplesInfo.Size(), finalSnapshot.calls, bed.maxLabel, bed.maxElapsed)
		})
	}
}

// TestContextCostHealthHelper is the fixture-owned transport for the exact
// production health evaluation invoked by the Stop hook. Its ordinary test
// path is empty; only the opt-in bed launches it as a subprocess.
func TestContextCostHealthHelper(t *testing.T) {
	if os.Getenv("METASYSTEM_CONTEXT_COST_HEALTH_HELPER") != "1" {
		return
	}
	repoRoot := os.Getenv("METASYSTEM_CONTEXT_COST_HEALTH_ROOT")
	installationRoot := os.Getenv("METASYSTEM_CONTEXT_COST_HEALTH_INSTALLATION")
	recordPath := os.Getenv("METASYSTEM_CONTEXT_COST_HEALTH_RECORD")
	if repoRoot == "" || installationRoot == "" || recordPath == "" {
		fmt.Fprintln(os.Stderr, "context cost health helper received incomplete coordinates")
		os.Exit(3)
	}
	verdict := steward.PreviewHealthAt(repoRoot, installationRoot, time.Now().UTC(), nil)
	for _, role := range verdict.Roles {
		if role.Role != steward.RoleContext {
			continue
		}
		data, err := json.Marshal(role)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
		if err := os.WriteFile(recordPath, append(data, '\n'), 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
		if err := json.NewEncoder(os.Stdout).Encode(steward.NewHookHealthPreview(verdict)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
		os.Exit(verdict.ExitCode())
	}
	fmt.Fprintln(os.Stderr, "context cost health helper found no context-budget role")
	os.Exit(3)
}

func TestContextCostRoleFromHealthLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		line string
		want string
	}{
		{
			line: "HEALTH unhealthy — stop-hook-duration=alive (12ms); context-budget=alive (120 thousand tokens this call, bound 150, ceiling 200); ledger-attention=unknown (pending)",
			want: "context-budget=alive (120 thousand tokens this call, bound 150, ceiling 200)",
		},
		{
			line: "context-budget=alive (120 thousand tokens this call, bound 150, ceiling 200; newest spill: trace.txt (42 bytes))",
			want: "context-budget=alive (120 thousand tokens this call, bound 150, ceiling 200; newest spill: trace.txt (42 bytes))",
		},
		{line: "HEALTH healthy — stop-hook-duration=alive", want: ""},
	}
	for _, test := range tests {
		if got := contextCostRoleFromHealthLine(test.line); got != test.want {
			t.Errorf("context role from %q = %q, want %q", test.line, got, test.want)
		}
	}
}

func TestContextCostReportRoleLineDecodesStructuredHealth(t *testing.T) {
	t.Parallel()
	role := steward.RoleVerdict{Role: steward.RoleContext, Status: steward.HealthAlive,
		Reason: "handoff --note <configured-note-path> --no-delegates"}
	health := steward.NewHookHealthPreview(steward.HealthVerdict{
		Schema: 1, Aggregate: "healthy", Roles: []steward.RoleVerdict{role},
	})
	encoded, err := json.MarshalIndent(health, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	report := "## Console text\n\ncontext-budget=dead (unrelated prose)\n\n## Health\n\n```json\n" + string(encoded) + "\n```\n"
	if got := contextCostReportRoleLine(report); got != role.Line() {
		t.Fatalf("decoded report role=%q, want %q", got, role.Line())
	}
}

func contextCostCandidateEngine(t *testing.T, declaredCandidate string) string {
	t.Helper()
	// TestMain captures this only for the explicitly enabled context-cost
	// proof, then removes METASYSTEM_BIN from the package environment.
	if declaredCandidate != "" {
		absolute, err := filepath.Abs(declaredCandidate)
		if err != nil {
			t.Fatal(err)
		}
		return absolute
	}
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(t.TempDir(), "metasystem")
	cache := os.Getenv("GOCACHE")
	if cache == "" {
		cache = filepath.Join(t.TempDir(), "go-build-cache")
	}
	environment := make([]string, 0, len(os.Environ())+1)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "GOCACHE=") {
			environment = append(environment, value)
		}
	}
	environment = append(environment, "GOCACHE="+cache)
	command := exec.Command("go", "build", "-o", candidate, "./cmd/metasystem")
	command.Dir = moduleRoot
	command.Env = environment
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build context cost candidate with GOCACHE=%s: %v\n%s", cache, err, output)
	}
	return candidate
}

func contextCostReaderHelper(t *testing.T) string {
	t.Helper()
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(t.TempDir(), "context-cost-reader.test")
	command := exec.Command("go", "test", "-c", "-o", helper, "./internal/usage")
	command.Dir = moduleRoot
	command.Env = os.Environ()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build context cost reader helper: %v\n%s", err, output)
	}
	return helper
}

func newContextCostBed(t *testing.T, runtimeName, candidate, readerHelper string) *contextCostBed {
	t.Helper()
	outer := t.TempDir()
	installation := filepath.Join(outer, "metasystem")
	home := filepath.Join(outer, "fixture-home")
	stubDirectory := filepath.Join(outer, "stub-bin")
	for _, directory := range []string{
		filepath.Join(outer, "development"), filepath.Join(installation, "bin"),
		filepath.Join(installation, "plans"), home, stubDirectory,
	} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(outer, "development", "metasystem-design.md"), []byte("context cost fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(filepath.Join(installation, "scripts"), os.DirFS(filepath.Join(sourceRoot, "scripts"))); err != nil {
		t.Fatal(err)
	}
	copyContextCostFile(t, candidate, filepath.Join(installation, "bin", "metasystem"), 0o755)
	config := "metasystem.version=1\nmetasystem.engine-delivery=source\nmetasystem.runtimes=claude,codex\nsteward.stop-slow-sec=15\n"
	if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	goals := "# Goals\n\n## Goal-free: declared 2026-09-07T00:00:00Z by human over context cost fixture\n"
	if err := os.WriteFile(filepath.Join(installation, "plans", "goals.md"), []byte(goals), 0o644); err != nil {
		t.Fatal(err)
	}
	runContextCostSetupCommand(t, outer, nil, "git", "init", "-q", "-b", "main")
	runContextCostSetupCommand(t, outer, nil, "git", "config", "user.name", "fixture")
	runContextCostSetupCommand(t, outer, nil, "git", "config", "user.email", "fixture@example.invalid")
	runContextCostSetupCommand(t, outer, nil, "git", "config", "metasystem.goal.machine", "context-cost")
	runContextCostSetupCommand(t, outer, nil, "git", "add", "development", "metasystem/metasystem.conf", "metasystem/plans", "metasystem/scripts")
	runContextCostSetupCommand(t, outer, nil, "git", "commit", "-qm", "context cost fixture")

	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("cannot establish fixture process identity: state=%s err=%v", state, err)
	}
	wrapper := filepath.Join(stubDirectory, "metasystem")
	argvRecord := filepath.Join(outer, "turn-verdict.argv")
	wrapperSource := `#!/usr/bin/env bash
set -euo pipefail
if [[ ${1:-} == report && ${2:-} == turn-verdict ]]; then
  printf '%s\0' "$@" >"${METASYSTEM_CONTEXT_COST_ARGV_RECORD:?}"
fi
if [[ ${1:-} == health && ${2:-} == --hook-preview ]]; then
  METASYSTEM_CONTEXT_COST_HEALTH_HELPER=1 \
    "${METASYSTEM_CONTEXT_COST_TEST_BINARY:?}" -test.run '^TestContextCostHealthHelper$'
  exit $?
fi
if [[ ${1:-} == up ]]; then
  printf '%s\n' 'up outcome=already-healthy'
  exit 0
fi
if [[ ${1:-} == proc && ${2:-} == find-ancestor ]]; then
  printf '{"runtime":"%s","pid":%s,"pidStartedAt":%s}\n' \
    "${METASYSTEM_CONTEXT_COST_RUNTIME:?}" "${METASYSTEM_CONTEXT_COST_PID:?}" "${METASYSTEM_CONTEXT_COST_STARTED:?}"
  exit 0
fi
exec "${METASYSTEM_CONTEXT_COST_REAL_ENGINE:?}" "$@"
`
	if err := testexec.WriteFile(wrapper, []byte(wrapperSource), 0o755); err != nil {
		t.Fatal(err)
	}
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	bed := &contextCostBed{
		t: t, runtime: runtimeName, session: "context-cost-" + runtimeName,
		outer: outer, installation: installation, home: home,
		engine: filepath.Join(installation, "bin", "metasystem"), wrapper: wrapper,
		hook:         filepath.Join(installation, "scripts", "agents", "supervision-hook.sh"),
		healthRecord: filepath.Join(outer, "hook-context-role.json"), testBinary: testBinary,
		readerHelper: readerHelper, argvRecord: argvRecord,
		startedAt: exact.StartedAt.Unix(),
	}
	if runtimeName == "claude" {
		bed.transcript = filepath.Join(home, ".claude", "projects", contextCostClaudeSlug(outer), bed.session+".jsonl")
	} else {
		bed.transcript = filepath.Join(home, ".codex", "sessions", "2026", "09", "13", "rollout-context-cost-"+bed.session+".jsonl")
	}
	t.Setenv("HOME", home)
	return bed
}

func (bed *contextCostBed) environment(wrapper bool) []string {
	blocked := []string{
		"METASYSTEM_BIN=", "METASYSTEM_CONTEXT_COST_REAL_ENGINE=", "METASYSTEM_CONTEXT_COST_RUNTIME=",
		"METASYSTEM_CONTEXT_COST_PID=", "METASYSTEM_CONTEXT_COST_STARTED=", "METASYSTEM_HOOK_DELEGATE_",
		"METASYSTEM_CONTEXT_COST_TEST_BINARY=", "METASYSTEM_CONTEXT_COST_HEALTH_HELPER=",
		"METASYSTEM_CONTEXT_COST_HEALTH_ROOT=", "METASYSTEM_CONTEXT_COST_HEALTH_INSTALLATION=",
		"METASYSTEM_CONTEXT_COST_HEALTH_RECORD=",
		"METASYSTEM_CONTEXT_COST_READER_HELPER=", "METASYSTEM_CONTEXT_COST_READER_ROOT=",
		"METASYSTEM_CONTEXT_COST_READER_RUNTIME=", "METASYSTEM_CONTEXT_COST_READER_SESSION=",
		"METASYSTEM_CONTEXT_COST_READER_TRANSCRIPT=", "METASYSTEM_CONTEXT_COST_READER_HOME=",
		"METASYSTEM_CONTEXT_COST_READER_TOPLEVEL=",
		"HOME=",
	}
	environment := make([]string, 0, len(os.Environ())+7)
	for _, value := range os.Environ() {
		skip := false
		for _, prefix := range blocked {
			if strings.HasPrefix(value, prefix) {
				skip = true
				break
			}
		}
		if !skip {
			environment = append(environment, value)
		}
	}
	environment = append(environment, "HOME="+bed.home)
	if wrapper {
		environment = append(environment,
			"METASYSTEM_BIN="+bed.wrapper,
			"METASYSTEM_CONTEXT_COST_REAL_ENGINE="+bed.engine,
			"METASYSTEM_CONTEXT_COST_RUNTIME="+bed.runtime,
			fmt.Sprintf("METASYSTEM_CONTEXT_COST_PID=%d", os.Getpid()),
			fmt.Sprintf("METASYSTEM_CONTEXT_COST_STARTED=%d", bed.startedAt),
			"METASYSTEM_CONTEXT_COST_TEST_BINARY="+bed.testBinary,
			"METASYSTEM_CONTEXT_COST_ARGV_RECORD="+bed.argvRecord,
			"PATH="+filepath.Dir(bed.wrapper)+string(os.PathListSeparator)+os.Getenv("PATH"),
			"METASYSTEM_CONTEXT_COST_HEALTH_ROOT="+bed.installation,
			"METASYSTEM_CONTEXT_COST_HEALTH_INSTALLATION="+bed.installation,
			"METASYSTEM_CONTEXT_COST_HEALTH_RECORD="+bed.healthRecord,
		)
	}
	return environment
}

func (bed *contextCostBed) announceHolder() {
	bed.t.Helper()
	output := runContextCostSetupCommand(bed.t, bed.outer, bed.environment(false), bed.engine,
		"lease", "announce", "--root", bed.installation, "--session", bed.session,
		"--pid", fmt.Sprint(os.Getpid()), "--start", fmt.Sprint(bed.startedAt),
		"--tag", "context-cost-holder", "--runtime", bed.runtime)
	announcement := strings.TrimSpace(output)
	if announcement == "" {
		bed.t.Fatalf("%s holder announcement was unreadable: %s", bed.runtime, output)
	}
	if _, err := os.Stat(announcement); err != nil {
		bed.t.Fatalf("%s holder announcement path %q: %v", bed.runtime, announcement, err)
	}
}

func (bed *contextCostBed) startStop(label string) *contextCostProcess {
	bed.prepareStopObservation()
	payload := fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"Stop","transcript_path":%q,"fixture_leg":%q}`+"\n", bed.session, bed.outer, bed.transcript, label)
	process := startContextCostProcess(bed.t, bed.outer, bed.environment(true), strings.NewReader(payload), "bash", bed.hook, bed.runtime, "stop")
	return process
}

func (bed *contextCostBed) startStopBehindBarrier(label string) *contextCostBarrierProcess {
	bed.prepareStopObservation()
	payload := fmt.Sprintf(`{"session_id":%q,"cwd":%q,"hook_event_name":"Stop","transcript_path":%q,"fixture_leg":%q}`+"\n", bed.session, bed.outer, bed.transcript, label)
	return startContextCostBarrierProcess(bed.t, bed.outer, bed.environment(true), strings.NewReader(payload), "bash", bed.hook, bed.runtime, "stop")
}

func (bed *contextCostBed) startColdReader() *contextCostBarrierProcess {
	bed.t.Helper()
	environment := append(bed.environment(false),
		"METASYSTEM_CONTEXT_COST_READER_HELPER=1",
		"METASYSTEM_CONTEXT_COST_READER_ROOT="+bed.installation,
		"METASYSTEM_CONTEXT_COST_READER_RUNTIME="+bed.runtime,
		"METASYSTEM_CONTEXT_COST_READER_SESSION="+bed.session,
		"METASYSTEM_CONTEXT_COST_READER_TRANSCRIPT="+bed.transcript,
		"METASYSTEM_CONTEXT_COST_READER_HOME="+bed.home,
		"METASYSTEM_CONTEXT_COST_READER_TOPLEVEL="+bed.outer,
	)
	return startContextCostCoordinatedProcess(bed.t, bed.outer, environment, nil, bed.readerHelper,
		"-test.run", "^TestContextCostColdReaderHelper$", "-test.count=1")
}

func (bed *contextCostBed) prepareStopObservation() {
	bed.t.Helper()
	if err := os.Remove(bed.healthRecord); err != nil && !os.IsNotExist(err) {
		bed.t.Fatal(err)
	}
}

func (bed *contextCostBed) runStop(label string) contextCostStop {
	bed.t.Helper()
	return bed.finishStop(label, bed.startStop(label))
}

func (bed *contextCostBed) finishStop(label string, process *contextCostProcess) contextCostStop {
	bed.t.Helper()
	output, elapsed, err := waitContextCostProcess(process)
	if err != nil {
		bed.t.Fatalf("%s %s Stop failed after %s: %v\n%s", bed.runtime, label, elapsed, err, output)
	}
	if strings.Contains(output, "deadline expired") || strings.Contains(output, "DEADLINE_EXPIRED") {
		bed.t.Fatalf("%s %s Stop reached the deadline path: %s", bed.runtime, label, output)
	}
	report := bed.stopReport(output)
	if !strings.Contains(report, "context-budget=") {
		bed.t.Fatalf("%s %s production Stop report omitted context-budget: response=%s\nreport=%s", bed.runtime, label, output, report)
	}
	roleData, err := os.ReadFile(bed.healthRecord)
	if err != nil {
		bed.t.Fatalf("%s %s production health observation: %v", bed.runtime, label, err)
	}
	var role steward.RoleVerdict
	if err := json.Unmarshal(roleData, &role); err != nil {
		bed.t.Fatalf("%s %s production health observation is malformed: %v", bed.runtime, label, err)
	}
	reportRole := contextCostReportRoleLine(report)
	if reportRole == "" || reportRole != role.Line() {
		bed.t.Fatalf("%s %s Stop report role and captured evaluation diverged: report-role=%q captured=%q response=%s\nreport=%s", bed.runtime, label, reportRole, role.Line(), output, report)
	}
	data, err := os.ReadFile(steward.ComponentEvidencePath(bed.installation, "supervision-hook"))
	if err != nil {
		bed.t.Fatalf("%s %s hook evidence: %v", bed.runtime, label, err)
	}
	var evidence steward.ComponentEvidence
	if err := json.Unmarshal(data, &evidence); err != nil {
		bed.t.Fatalf("%s %s hook evidence is malformed: %v", bed.runtime, label, err)
	}
	if evidence.Generation <= bed.lastHookGen || evidence.AttemptSeq < 1 || evidence.LastAttempt.IsZero() ||
		evidence.LastCompletion.IsZero() || evidence.LastCompletion.Before(evidence.LastAttempt) ||
		evidence.Result != steward.ComponentOK || evidence.Outcome != "EMITTED" ||
		evidence.SuccessAttemptSeq != evidence.AttemptSeq || evidence.LastStopElapsedSec == nil {
		bed.t.Fatalf("%s %s hook attempt/completion evidence is incomplete: %+v", bed.runtime, label, evidence)
	}
	if *evidence.LastStopElapsedSec >= 57 {
		bed.t.Fatalf("%s %s worker evidence reports %ds, want below 57s", bed.runtime, label, *evidence.LastStopElapsedSec)
	}
	bed.lastHookGen = evidence.Generation
	if role.DurationMillis < 1 {
		bed.t.Fatalf("%s %s context role carried no duration: %+v", bed.runtime, label, role)
	}
	if elapsed > bed.maxElapsed {
		bed.maxElapsed = elapsed
		bed.maxLabel = label
	}
	bed.t.Logf("%s Stop %s: elapsed=%s worker-elapsed=%ds context-role-ms=%d", bed.runtime, label, elapsed, *evidence.LastStopElapsedSec, role.DurationMillis)
	return contextCostStop{label: label, output: output, report: report, elapsed: elapsed, role: role, reportRole: reportRole, evidence: evidence}
}

func (bed *contextCostBed) stopReport(output string) string {
	bed.t.Helper()
	helper := filepath.Join(bed.installation, "scripts", "agents", "fixture-stop-report.sh")
	command := `source "$1"; fixture_stop_status_report "$2" "$3" "$4" "$5"`
	report, code, err := runContextCostCommand(bed.outer, bed.environment(false), nil, "bash", "-c", command,
		"context-cost-stop-report", helper, output, bed.installation, bed.runtime, bed.session)
	if err != nil || code != 0 {
		bed.t.Fatalf("%s Stop response did not name a readable identity-bound report: code=%d err=%v response=%s\n%s", bed.runtime, code, err, output, report)
	}
	return report
}

func (bed *contextCostBed) requireLiveRole(stop contextCostStop) {
	bed.t.Helper()
	bed.requireExpectedLiveRole(stop.label, stop.role)
	want := "context-budget=alive (120 thousand tokens this call, trigger 105, proof line 150, proof maximum 200, ceiling 250; over the trigger: run metasystem context handoff --root " + bed.installation + " --note <configured-note-path> --no-delegates)"
	if stop.reportRole != want {
		bed.t.Fatalf("%s %s Stop report role=%q, want %q", bed.runtime, stop.label, stop.reportRole, want)
	}
}

func (bed *contextCostBed) requireExpectedLiveRole(label string, role steward.RoleVerdict) {
	bed.t.Helper()
	if role.Status != steward.HealthAlive || !strings.Contains(role.Reason, "120 thousand tokens this call") {
		bed.t.Fatalf("%s %s live role=%s", bed.runtime, label, role.Line())
	}
}

func contextCostReportRoleLine(report string) string {
	const healthSection = "## Health\n\n```json\n"
	start := strings.Index(report, healthSection)
	if start < 0 {
		return ""
	}
	section := report[start+len(healthSection):]
	end := strings.Index(section, "\n```")
	if end < 0 {
		return ""
	}
	var health steward.HookHealthPreview
	if err := json.Unmarshal([]byte(section[:end]), &health); err != nil {
		return ""
	}
	return contextCostRoleFromHealthLine(health.Line)
}

func contextCostRoleFromHealthLine(line string) string {
	const prefix = "context-budget="
	start := strings.Index(line, prefix)
	if start < 0 {
		return ""
	}
	role := line[start:]
	searchFrom := len(prefix)
	for searchFrom < len(role) {
		next := strings.Index(role[searchFrom:], "; ")
		if next < 0 {
			break
		}
		next += searchFrom
		candidate := role[next+2:]
		equals := strings.IndexByte(candidate, '=')
		if equals > 0 && contextCostHealthRoleName(candidate[:equals]) {
			return strings.TrimSpace(role[:next])
		}
		searchFrom = next + 2
	}
	return strings.TrimSpace(role)
}

func contextCostHealthRoleName(value string) bool {
	for _, character := range value {
		if (character < 'a' || character > 'z') && character != '-' {
			return false
		}
	}
	return value != ""
}

func (bed *contextCostBed) snapshot() contextCostSnapshot {
	bed.t.Helper()
	calls, markers, err := usagepkg.Calls(bed.installation, bed.runtime, bed.session, time.Time{})
	if err != nil {
		bed.t.Fatal(err)
	}
	return contextCostSnapshot{
		cursor:   readContextCostFile(bed.t, usagepkg.CursorPath(bed.installation, bed.runtime, bed.session)),
		samples:  readContextCostFile(bed.t, usagepkg.SamplesPath(bed.installation, bed.runtime, bed.session)),
		registry: readContextCostFile(bed.t, filepath.Join(bed.installation, "artifacts", "agents", "context", "sessions.jsonl")),
		calls:    len(calls), markers: len(markers),
	}
}

func (bed *contextCostBed) requireSnapshot(label string, before, after contextCostSnapshot) {
	bed.t.Helper()
	if !bytes.Equal(before.cursor, after.cursor) || !bytes.Equal(before.samples, after.samples) ||
		!bytes.Equal(before.registry, after.registry) || before.calls != after.calls || before.markers != after.markers {
		bed.t.Fatalf("%s %s changed live evidence: before(cursor=%d samples=%d registry=%d calls=%d markers=%d) after(cursor=%d samples=%d registry=%d calls=%d markers=%d)",
			bed.runtime, label, len(before.cursor), len(before.samples), len(before.registry), before.calls, before.markers,
			len(after.cursor), len(after.samples), len(after.registry), after.calls, after.markers)
	}
}

func (bed *contextCostBed) runDiagnosticOverrides(live contextCostSnapshot) {
	bed.t.Helper()
	empty := filepath.Join(bed.outer, "diagnostic-empty.jsonl")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		bed.t.Fatal(err)
	}
	distinct := filepath.Join(bed.outer, "diagnostic-distinct.jsonl")
	content := contextCostCallLine(bed.runtime, "diagnostic-"+bed.runtime, 180001, 1, bed.outer) + contextCostMarkerLine(bed.runtime, 2)
	if err := os.WriteFile(distinct, []byte(content), 0o600); err != nil {
		bed.t.Fatal(err)
	}
	for _, diagnostic := range []string{empty, distinct} {
		output, code, err := runContextCostCommand(bed.outer, bed.environment(false), nil, bed.engine,
			"context", "status", "--root", bed.outer, "--runtime", bed.runtime,
			"--session", bed.session, "--transcript", diagnostic)
		if err != nil || code != 0 || !strings.Contains(output, "diagnostic transcript override") {
			bed.t.Fatalf("%s diagnostic %s: code=%d err=%v output=%s", bed.runtime, filepath.Base(diagnostic), code, err, output)
		}
		bed.requireSnapshot(filepath.Base(diagnostic), live, bed.snapshot())
	}
}

func (bed *contextCostBed) requireReport(wantCalls int) {
	bed.t.Helper()
	output, code, err := runContextCostCommand(bed.outer, bed.environment(false), nil, bed.engine,
		"context", "report", "--root", bed.outer, "--week", "2026-09-07")
	if err != nil || code != 0 || !strings.Contains(output, "verdict=PASS") {
		bed.t.Fatalf("%s report: code=%d err=%v output=%s", bed.runtime, code, err, output)
	}
	callsPath := filepath.Join(bed.installation, "artifacts", "reports", "coordinator-context", "2026-09-07", "calls.jsonl")
	if got := contextCostCompleteLines(bed.t, callsPath); got != wantCalls {
		bed.t.Fatalf("%s normalized report calls=%d, want %d", bed.runtime, got, wantCalls)
	}
}

func (bed *contextCostBed) runLockedReaderContention() {
	bed.t.Helper()
	lockPath := usagepkg.CursorPath(bed.installation, bed.runtime, bed.session) + ".lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		bed.t.Fatal(err)
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		bed.t.Fatal(err)
	}
	locked := true
	defer func() {
		if locked {
			_ = unix.Flock(int(lock.Fd()), unix.LOCK_UN)
		}
	}()

	reader := startContextCostBarrierProcess(bed.t, bed.outer, bed.environment(false), nil, bed.engine,
		"context", "report", "--root", bed.outer, "--week", "2026-09-07")
	stopProcess := bed.startStopBehindBarrier("held-cursor-lock")
	defer cleanupContextCostBarrier(reader)
	defer cleanupContextCostBarrier(stopProcess)
	waitContextCostBarrierReady(bed.t, reader)
	waitContextCostBarrierReady(bed.t, stopProcess)
	releaseContextCostBarrier(bed.t, reader)
	releaseContextCostBarrier(bed.t, stopProcess)
	select {
	case <-reader.process.done:
		bed.t.Fatalf("%s blocking report finished before the held cursor lock was released: %v\n%s", bed.runtime, reader.process.err, reader.process.output.String())
	default:
	}
	stop := bed.finishStop("held-cursor-lock", stopProcess.process)
	if stop.role.Status != steward.HealthUnknown || !strings.Contains(stop.role.Reason, "busy") || !strings.Contains(stop.reportRole, "context-budget=unknown") {
		bed.t.Fatalf("%s held-lock Stop report did not expose a bounded busy role: role=%s response=%s\nreport=%s", bed.runtime, stop.role.Line(), stop.output, stop.report)
	}
	select {
	case <-reader.process.done:
		bed.t.Fatalf("%s report did not wait on the held real cursor lock: %v\n%s", bed.runtime, reader.process.err, reader.process.output.String())
	default:
	}
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_UN); err != nil {
		bed.t.Fatal(err)
	}
	locked = false
	output, _, err := waitContextCostProcess(reader.process)
	if err != nil || !strings.Contains(output, "verdict=PASS") {
		bed.t.Fatalf("%s report did not complete after the explicit lock release: %v\n%s", bed.runtime, err, output)
	}
}

func (bed *contextCostBed) runConcurrentColdRead() {
	bed.t.Helper()
	for _, path := range []string{
		usagepkg.CursorPath(bed.installation, bed.runtime, bed.session),
		usagepkg.SamplesPath(bed.installation, bed.runtime, bed.session),
	} {
		if err := os.Remove(path); err != nil {
			bed.t.Fatal(err)
		}
	}
	reader := bed.startColdReader()
	defer cleanupContextCostBarrier(reader)
	waitContextCostBarrierReady(bed.t, reader)
	stop := bed.runStop("concurrent-cold-large-source")
	if stop.role.Status != steward.HealthUnknown || !strings.Contains(stop.role.Reason, "busy") || !strings.Contains(stop.reportRole, "context-budget=unknown") {
		bed.t.Fatalf("%s concurrent cold Stop report did not expose a bounded busy role while the reader held the cursor: role=%s response=%s\nreport=%s",
			bed.runtime, stop.role.Line(), stop.output, stop.report)
	}
	releaseContextCostBarrier(bed.t, reader)
	readerOutput, _, readerErr := waitContextCostProcess(reader.process)
	if readerErr != nil || !strings.Contains(readerOutput, "context-cost-cold-reader=live prompt-tokens=120000") {
		bed.t.Fatalf("%s coordinated cold reader did not finish live after release: %v\n%s", bed.runtime, readerErr, readerOutput)
	}
	if got := bed.snapshot().calls; got != contextCostInitialCalls+1 {
		bed.t.Fatalf("%s coordinated cold reader committed calls=%d, want %d", bed.runtime, got, contextCostInitialCalls+1)
	}
}

func startContextCostProcess(t *testing.T, directory string, environment []string, stdin io.Reader, name string, args ...string) *contextCostProcess {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = directory
	command.Env = environment
	command.Stdin = stdin
	process := &contextCostProcess{command: command, cancel: cancel, started: time.Now(), done: make(chan struct{})}
	command.Stdout = &process.output
	command.Stderr = &process.output
	if err := command.Start(); err != nil {
		cancel()
		t.Fatalf("start %s %v: %v", name, args, err)
	}
	go func() {
		process.err = command.Wait()
		close(process.done)
	}()
	return process
}

func waitContextCostProcess(process *contextCostProcess) (string, time.Duration, error) {
	<-process.done
	elapsed := time.Since(process.started)
	process.cancel()
	return process.output.String(), elapsed, process.err
}

func startContextCostBarrierProcess(t *testing.T, directory string, environment []string, stdin io.Reader, name string, args ...string) *contextCostBarrierProcess {
	t.Helper()
	launcher := `printf x >&3; IFS= read -r -n 1 <&4; exec "$@"`
	commandArgs := append([]string{"-c", launcher, "context-cost-barrier", name}, args...)
	return startContextCostCoordinatedProcess(t, directory, environment, stdin, "bash", commandArgs...)
}

func startContextCostCoordinatedProcess(t *testing.T, directory string, environment []string, stdin io.Reader, name string, args ...string) *contextCostBarrierProcess {
	t.Helper()
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	releaseRead, releaseWrite, err := os.Pipe()
	if err != nil {
		readyRead.Close()
		readyWrite.Close()
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = directory
	command.Env = environment
	command.Stdin = stdin
	command.ExtraFiles = []*os.File{readyWrite, releaseRead}
	process := &contextCostProcess{command: command, cancel: cancel, started: time.Now(), done: make(chan struct{})}
	command.Stdout = &process.output
	command.Stderr = &process.output
	if err := command.Start(); err != nil {
		cancel()
		readyRead.Close()
		readyWrite.Close()
		releaseRead.Close()
		releaseWrite.Close()
		t.Fatalf("start coordinated process %s %v: %v", name, args, err)
	}
	readyWrite.Close()
	releaseRead.Close()
	go func() {
		process.err = command.Wait()
		close(process.done)
	}()
	return &contextCostBarrierProcess{process: process, ready: readyRead, release: releaseWrite}
}

func waitContextCostBarrierReady(t *testing.T, process *contextCostBarrierProcess) {
	t.Helper()
	var signal [1]byte
	_, err := io.ReadFull(process.ready, signal[:])
	_ = process.ready.Close()
	if err != nil {
		t.Fatalf("context cost child readiness failed: %v", err)
	}
}

func releaseContextCostBarrier(t *testing.T, process *contextCostBarrierProcess) {
	t.Helper()
	if _, err := process.release.Write([]byte{'x'}); err != nil {
		_ = process.release.Close()
		t.Fatalf("release context cost child: %v", err)
	}
	if err := process.release.Close(); err != nil {
		t.Fatal(err)
	}
}

func cancelAndWaitContextCostProcess(process *contextCostProcess) {
	process.cancel()
	<-process.done
}

func cleanupContextCostBarrier(process *contextCostBarrierProcess) {
	_ = process.ready.Close()
	_ = process.release.Close()
	cancelAndWaitContextCostProcess(process.process)
}

func runContextCostCommand(directory string, environment []string, stdin io.Reader, name string, args ...string) (string, int, error) {
	command := exec.Command(name, args...)
	command.Dir = directory
	command.Env = environment
	command.Stdin = stdin
	output, err := command.CombinedOutput()
	return string(output), contextCostExitCode(err), err
}

func contextCostExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode()
	}
	return -1
}

func runContextCostSetupCommand(t *testing.T, directory string, environment []string, name string, args ...string) string {
	t.Helper()
	if environment == nil {
		environment = os.Environ()
	}
	output, code, err := runContextCostCommand(directory, environment, nil, name, args...)
	if err != nil || code != 0 {
		t.Fatalf("setup command %s %v: code=%d err=%v\n%s", name, args, code, err, output)
	}
	return output
}

func writeContextCostSource(t *testing.T, runtimeName, path, toplevel string) contextCostSourceStats {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	buffered := bufio.NewWriterSize(file, 1024*1024)
	stats := contextCostSourceStats{IDs: contextCostInitialCalls}
	writeLine := func(line string) {
		written, writeErr := buffered.WriteString(line)
		if writeErr != nil {
			t.Fatal(writeErr)
		}
		stats.Bytes += int64(written)
		stats.Lines++
	}
	for index := 0; index < contextCostInitialCalls; index++ {
		writeLine(contextCostCallLine(runtimeName, contextResponseID(index), 120000, int64(index+1), toplevel))
	}
	if runtimeName == "codex" {
		padding := fmt.Sprintf(`{"type":"fixture_padding","padding":%q}`+"\n", strings.Repeat("x", 6000))
		for stats.Lines < 54471 {
			writeLine(padding)
		}
	} else {
		padding := fmt.Sprintf(`{"type":"fixture_padding","padding":%q}`+"\n", strings.Repeat("x", 12000))
		for stats.Lines < contextCostInitialCalls+4096 {
			writeLine(padding)
		}
	}
	if err := buffered.Flush(); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	stats.Bytes = info.Size()
	return stats
}

func appendContextCostCall(t *testing.T, runtimeName, path string, index int, toplevel string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(contextCostCallLine(runtimeName, contextResponseID(index), 120000, int64(index+1), toplevel)); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func contextCostCallLine(runtimeName, id string, tokens, ordinal int64, toplevel string) string {
	if runtimeName == "codex" {
		return fmt.Sprintf(`{"timestamp":"2026-09-13T11:00:00Z","type":"token_usage_record","ordinal":%d,"payload":{"response_id":%q,"usage":{"input_tokens":%d,"cached_input_tokens":0,"cache_write_input_tokens":0}}}`+"\n", ordinal, id, tokens)
	}
	return fmt.Sprintf(`{"type":"assistant","requestId":%q,"cwd":%q,"timestamp":"2026-09-13T11:00:00Z","message":{"usage":{"input_tokens":%d,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}},"ordinal":%d}`+"\n", id, toplevel, tokens, ordinal)
}

func contextCostMarkerLine(runtimeName string, ordinal int64) string {
	if runtimeName == "codex" {
		return fmt.Sprintf(`{"timestamp":"2026-09-13T11:00:01Z","type":"compacted","ordinal":%d}`+"\n", ordinal)
	}
	return fmt.Sprintf(`{"type":"system","subtype":"compact_boundary","timestamp":"2026-09-13T11:00:01Z","compactMetadata":{"trigger":"manual","preTokens":180001},"ordinal":%d}`+"\n", ordinal)
}

func replaceContextCostSource(t *testing.T, path string) {
	t.Helper()
	source, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	replacement := path + ".replacement"
	target, err := os.OpenFile(replacement, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		source.Close()
		t.Fatal(err)
	}
	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		source.Close()
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		target.Close()
		t.Fatal(err)
	}
	if err := target.Sync(); err != nil {
		target.Close()
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
}

func copyContextCostFile(t *testing.T, source, target string, mode os.FileMode) {
	t.Helper()
	input, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if err := testexec.Locked(func() error {
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			return err
		}
		if _, err := io.Copy(output, input); err != nil {
			_ = output.Close()
			return err
		}
		return output.Close()
	}); err != nil {
		t.Fatal(err)
	}
}

func readContextCostFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func contextCostCompleteLines(t *testing.T, path string) int {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return count
}

func contextCostClaudeSlug(path string) string {
	value := []byte(filepath.Clean(path))
	for index, character := range value {
		if (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') {
			continue
		}
		value[index] = '-'
	}
	return string(value)
}
