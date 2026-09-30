package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

var launchExecutable = os.Executable
var launchLookupEnv = os.LookupEnv

// shippedClaudeSettings is the Claude hook settings this engine ships; the
// seat window it imposes is reported beside the configured one.
var shippedClaudeSettings = func() ([]byte, error) { return runtimes.ShippedEnforcement("claude") }

func newLaunchManager() *launch.Manager {
	prober := identity.KernelProber{}
	processes := launch.OSProcesses{Prober: prober}
	home, _ := os.UserHomeDir()
	executable, executableErr := launchExecutable()
	confPath := filepath.Join(filepath.Dir(executable), "..", "metasystem.conf")
	settings, settingsErr := launch.ResolveSettings(confPath, launchLookupEnv)
	// An unreadable disk setting reads as its compiled default.
	disk, _ := diskstore.LoadSettings(confPath, launchLookupEnv)
	if executableErr != nil {
		settingsErr = executableErr
	}
	scanner := launch.KernelProcessScanner{Prober: prober}
	codex := launch.CodexExec{Binary: "codex", SessionsRoot: filepath.Join(home, ".codex", "sessions"), Now: time.Now, Scanner: scanner}
	// Claude Code keeps its session transcripts under CLAUDE_CONFIG_DIR when
	// that is set, else under ~/.claude; the measurement reads the same place.
	claudeConfig := filepath.Join(home, ".claude")
	if configured := os.Getenv("CLAUDE_CONFIG_DIR"); configured != "" {
		claudeConfig = configured
	}
	claude := launch.ClaudeHeadless{Binary: "claude", ProjectsRoot: filepath.Join(claudeConfig, "projects"), Scanner: scanner}
	stateRoot, _ := launch.DefaultRoot()
	devin := launch.DevinPrint{Binary: "devin", StateRoot: stateRoot, Scanner: scanner}
	return &launch.Manager{Store: launch.Store{}, Adapters: map[string]launch.Adapter{"codex-exec": codex, "claude-headless": claude, "devin-print": devin, "plain-exec": launch.PlainExec{}},
		Processes: processes, Signaler: processes, Prober: prober, Supervisor: launch.OSSupervisorStarter{Prober: prober}, Now: time.Now,
		Sleep: time.Sleep, Grace: 2 * time.Second, Poll: 50 * time.Millisecond, StartCap: launch.DefaultWaitTimeout,
		Settings: settings, SettingsError: settingsErr, CompressAbove: disk.Bytes(config.DiskCompressAboveKey), Seat: launchSeat(executable, executableErr, goal.ResolveMachine)}
}

// launchSeat is the engine installation's seat on the host board, resolved
// once at the manager's one constructor: the installation that holds this
// engine and its enrolled nickname. Without a nickname the launches write no
// card (D14, R24).
func launchSeat(executable string, executableErr error, resolve func(string) (string, error)) board.Seat {
	if executableErr != nil {
		return board.Seat{}
	}
	installation := launchSeatInstallation(executable)
	machine, err := resolve(installation)
	if err != nil {
		return board.Seat{}
	}
	return board.Seat{Machine: machine, Installation: installation}
}

// launchSeatInstallation is the installation that holds an engine: the
// nearest ancestor carrying metasystem.conf. The engine runs from
// <installation>/bin or, enrolled, from its pin under
// <installation>/artifacts/agents/steward/engine-pins, so the parent of its
// folder is the installation only in the first case. Without such an
// ancestor it stays the parent of the engine's folder.
func launchSeatInstallation(executable string) string {
	folder := realpath.Resolve(filepath.Dir(executable))
	for candidate := folder; ; candidate = filepath.Dir(candidate) {
		if info, err := os.Stat(filepath.Join(candidate, "metasystem.conf")); err == nil && !info.IsDir() {
			return candidate
		}
		if filepath.Dir(candidate) == candidate {
			return filepath.Dir(folder)
		}
	}
}

var launchManager = newLaunchManager

func runLaunchSupervise(args []string, stdout, stderr io.Writer) int {
	id, ok := launchID(args, "supervise", stdout, stderr)
	if !ok {
		return 2
	}
	record, err := launchManager().Supervise(id)
	if err != nil {
		fmt.Fprintln(stderr, "launch supervise:", err)
		return 1
	}
	if record.ExitCode != nil {
		return *record.ExitCode
	}
	return 0
}

func launchID(args []string, verb string, stdout, stderr io.Writer) (string, bool) {
	flags := newFlagSet("launch "+verb, stdout, stderr)
	id := flags.String("id", "", "launch id")
	if flags.Parse(args) != nil || !requireFlags(flags, stderr, "id") || *id == "" || flags.NArg() != 0 {
		if verb == "status" || verb == "cancel" {
			writeLaunchRecordUsage(stderr, verb)
		} else {
			fmt.Fprintf(stderr, "usage: metasystem launch %s --id <id>\n", verb)
		}
		return "", false
	}
	return *id, true
}
func writeLaunchRecordUsage(w io.Writer, verb string) {
	fmt.Fprintf(w, "usage: metasystem launch %s --id <id>\n", verb)
	fmt.Fprintln(w, "Launch records belong to the current user under ~/.metasystem/launch; they are not selected by repository.")
	fmt.Fprintln(w, "--root is not a launch flag. Use --id to select a launch record.")
}
func runLaunchReport(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("launch report", stdout, stderr)
	id := flags.String("id", "", "launch id")
	goal := flags.String("goal", "", "goal id")
	sinceText := flags.String("since", "", "include activity at or after this RFC3339 instant")
	asJSON := flags.Bool("json", false, "print structured report")
	if flags.Parse(args) != nil || flags.NArg() != 0 || (*id != "" && (*goal != "" || *sinceText != "")) {
		fmt.Fprintln(stderr, "usage: metasystem work status [G | j1:ID] --history [--since RFC3339] [--json]")
		return 2
	}
	var since time.Time
	if *sinceText != "" {
		var err error
		since, err = time.Parse(time.RFC3339, *sinceText)
		if err != nil {
			fmt.Fprintf(stderr, "launch report: invalid --since %q: must be RFC3339\n", *sinceText)
			return 2
		}
	}
	manager := launchManager()
	if *id != "" {
		record, err := manager.Status(*id)
		if err != nil {
			fmt.Fprintln(stderr, "launch report:", err)
			return 1
		}
		if *asJSON {
			writeJSONLine(stdout, stderr, record)
			return 0
		}
		fmt.Fprintf(stdout, "%s declared-lines=%d read-mode=%s read-package=%s changed-lines=%d verdict-counts=%t\n", launchReport(record), record.DeclaredLines, record.ReadMode, record.ReadPackage, record.ChangedLines, verdictCounts(record))
		return 0
	}
	report, err := manager.Report(*goal, since)
	if err != nil {
		fmt.Fprintln(stderr, "launch report:", err)
		return 1
	}
	if *asJSON {
		writeJSONLine(stdout, stderr, report)
		return 0
	}
	for _, line := range report.Lines() {
		fmt.Fprintln(stdout, line)
	}
	return 0
}
func verdictCounts(record launch.Record) bool {
	return record.VerdictIsCounting()
}
func launchReport(record launch.Record) string {
	root, _ := launch.DefaultRoot()
	return launch.RecordLine(record, root)
}
