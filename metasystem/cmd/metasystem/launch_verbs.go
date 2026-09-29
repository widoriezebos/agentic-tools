package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
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
	return &launch.Manager{Store: launch.Store{}, Adapters: map[string]launch.Adapter{"codex-exec": codex, "claude-headless": claude, "plain-exec": launch.PlainExec{}},
		Processes: processes, Signaler: processes, Prober: prober, Supervisor: launch.OSSupervisorStarter{Prober: prober}, Now: time.Now,
		Sleep: time.Sleep, Grace: 2 * time.Second, Poll: 50 * time.Millisecond, StartCap: launch.DefaultWaitTimeout,
		Settings: settings, SettingsError: settingsErr}
}

var launchManager = newLaunchManager

func runLaunchSupervise(args []string) int {
	id, ok := launchID(args, "supervise")
	if !ok {
		return 2
	}
	record, err := launchManager().Supervise(id)
	if err != nil {
		fmt.Fprintln(os.Stderr, "launch supervise:", err)
		return 1
	}
	if record.ExitCode != nil {
		return *record.ExitCode
	}
	return 0
}

func launchID(args []string, verb string) (string, bool) {
	flags := newFlagSet("launch " + verb)
	id := flags.String("id", "", "launch id")
	if flags.Parse(args) != nil || !requireFlags(flags, nil, "id") || *id == "" || flags.NArg() != 0 {
		if verb == "status" || verb == "cancel" {
			writeLaunchRecordUsage(os.Stderr, verb)
		} else {
			fmt.Fprintf(os.Stderr, "usage: metasystem launch %s --id <id>\n", verb)
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
func runLaunchReport(args []string) int {
	flags := newFlagSet("launch report")
	id := flags.String("id", "", "launch id")
	goal := flags.String("goal", "", "goal id")
	sinceText := flags.String("since", "", "include activity at or after this RFC3339 instant")
	asJSON := flags.Bool("json", false, "print structured report")
	if flags.Parse(args) != nil || flags.NArg() != 0 || (*id != "" && (*goal != "" || *sinceText != "")) {
		fmt.Fprintln(os.Stderr, "usage: metasystem work status [G | j1:ID] --history [--since RFC3339] [--json]")
		return 2
	}
	var since time.Time
	if *sinceText != "" {
		var err error
		since, err = time.Parse(time.RFC3339, *sinceText)
		if err != nil {
			fmt.Fprintf(os.Stderr, "launch report: invalid --since %q: must be RFC3339\n", *sinceText)
			return 2
		}
	}
	manager := launchManager()
	if *id != "" {
		record, err := manager.Status(*id)
		if err != nil {
			fmt.Fprintln(os.Stderr, "launch report:", err)
			return 1
		}
		if *asJSON {
			printJSON(record)
			return 0
		}
		fmt.Printf("%s declared-lines=%d read-mode=%s read-package=%s changed-lines=%d verdict-counts=%t\n", launchReport(record), record.DeclaredLines, record.ReadMode, record.ReadPackage, record.ChangedLines, verdictCounts(record))
		return 0
	}
	report, err := manager.Report(*goal, since)
	if err != nil {
		fmt.Fprintln(os.Stderr, "launch report:", err)
		return 1
	}
	if *asJSON {
		printJSON(report)
		return 0
	}
	for _, line := range report.Lines() {
		fmt.Println(line)
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
