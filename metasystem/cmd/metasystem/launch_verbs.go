package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var launchExecutable = os.Executable
var launchLookupEnv = os.LookupEnv

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
	codex := launch.CodexExec{Binary: "codex", SessionsRoot: filepath.Join(home, ".codex", "sessions"), CommonTemplate: filepath.Join(filepath.Dir(executable), "..", "scripts", "agents", "templates", "design-common.md"), Now: time.Now, Scanner: scanner}
	claude := launch.ClaudeHeadless{Binary: "claude", ProjectsRoot: filepath.Join(home, ".claude", "projects"), Scanner: scanner}
	return &launch.Manager{Store: launch.Store{}, Adapters: map[string]launch.Adapter{"codex-exec": codex, "claude-headless": claude},
		Processes: processes, Signaler: processes, Prober: prober, Supervisor: launch.OSSupervisorStarter{Prober: prober}, Now: time.Now,
		Sleep: time.Sleep, Grace: 2 * time.Second, Poll: 50 * time.Millisecond, StartCap: launch.DefaultWaitTimeout,
		Settings: settings, SettingsError: settingsErr}
}

var launchManager = newLaunchManager

func runLaunchStart(args []string) int {
	flags := flag.NewFlagSet("launch start", flag.ContinueOnError)
	kind := flags.String("kind", "", "launch kind")
	brief := flags.String("brief", "", "brief file")
	directory := flags.String("dir", ".", "working directory")
	goal := flags.String("goal", "", "goal id")
	tag := flags.String("tag", "", "launch tag")
	model := flags.String("model", "", "model")
	effort := flags.String("effort", "", "reasoning effort")
	resume := flags.String("resume-session", "", "Claude session id")
	page := flags.String("page", "", "page file")
	unitsPage := flags.String("units-page", "", "page containing the units table")
	diffFile := flags.String("diff-file", "", "diff file for a read")
	readPackage := flags.String("package", "", "directory selected from a split diff")
	wide := flags.Bool("wide", false, "use the wide read window")
	var inputs, outputs, units multiFlag
	flags.Var(&inputs, "input", "additional input file (repeatable)")
	flags.Var(&outputs, "output", "output file to copy into launch state (repeatable)")
	flags.Var(&units, "unit", "unit name from the selected units table (repeatable)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *kind == "" || *brief == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem launch start --kind <build|design|read|critique> --brief <file> [--dir <directory>] [--goal <id>] [--tag <tag>] [--model <model>] [--effort <effort>] [--resume-session <id>] [--page <file>] [--input <file>]... [--output <file>]... [--units-page <file>] [--unit <name>]... [--diff-file <file> [--package <directory>|--wide]]")
		return 2
	}
	data := map[string]json.RawMessage{}
	if *resume != "" {
		data["resumeSession"], _ = json.Marshal(*resume)
	}
	record, err := launchManager().Start(launch.StartSpec{Kind: *kind, Brief: *brief, WorkingDirectory: *directory,
		Goal: *goal, Tag: *tag, Page: *page, Model: *model, Effort: *effort, Inputs: inputs, Outputs: outputs,
		UnitsPage: *unitsPage, Units: units, DiffFile: *diffFile, Package: *readPackage, Wide: *wide, AdapterData: data})
	if record.ID != "" {
		fmt.Println(launchReport(record))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "launch start:", err)
		return 1
	}
	return 0
}
func runLaunchRoundTask(args []string) int {
	flags := flag.NewFlagSet("launch round-task", flag.ContinueOnError)
	tag := flags.String("tag", "", "launch tag")
	round := flags.Int("round", 0, "task number")
	previous := flags.String("previous", "", "previous task file")
	out := flags.String("out", "", "output file")
	constraints := flags.String("constraints", "0", "constraint count")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *tag == "" || (*round != 2 && *round != 3) || *previous == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem launch round-task --tag <tag> --round <2|3> --previous <file> --out <file> [--constraints <n>]")
		return 2
	}
	data, err := os.ReadFile(*previous)
	if err != nil {
		fmt.Fprintln(os.Stderr, "launch round-task:", err)
		return 1
	}
	result, err := launch.RoundTask(*tag, *round, data, *constraints)
	if err != nil {
		fmt.Fprintln(os.Stderr, "launch round-task:", err)
		return 1
	}
	if err := os.WriteFile(*out, result, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "launch round-task:", err)
		return 1
	}
	return 0
}
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
func runLaunchWait(args []string) int {
	return launchWaitWith(launchManager(), args, os.Stdout, os.Stderr)
}
func launchWaitWith(manager *launch.Manager, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("launch wait", flag.ContinueOnError)
	flags.SetOutput(stderr)
	id := flags.String("id", "", "launch id")
	timeout := flags.Duration("timeout", time.Duration(1<<63-1), "maximum wait")
	if flags.Parse(args) != nil || *id == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: metasystem launch wait --id <id> [--timeout <duration>] (one call waits at most launch.wait.cap.seconds)")
		return 2
	}
	cap, err := manager.WaitCap()
	if err != nil {
		fmt.Fprintln(stderr, "launch wait:", err)
		return 1
	}
	effectiveWait := *timeout
	if effectiveWait > cap {
		effectiveWait = cap
		record, statusErr := manager.Status(*id)
		if statusErr != nil {
			fmt.Fprintln(stderr, "launch wait:", statusErr)
			return 1
		}
		if record.State.Terminal() {
			fmt.Fprintln(stdout, launchReport(record))
			return 0
		}
		fmt.Fprintf(stderr, "launch wait: --timeout %s exceeds launch.wait.cap.seconds=%d; waiting %s\n", *timeout, cap/time.Second, cap)
	}
	record, terminal, err := manager.Wait(*id, *timeout)
	if err != nil {
		fmt.Fprintln(stderr, "launch wait:", err)
		return 1
	}
	if !terminal {
		fmt.Fprintf(stderr, "launch wait: %s still %s after %s; run launch wait again or launch status\n", *id, record.State, effectiveWait)
		return 3
	}
	fmt.Fprintln(stdout, launchReport(record))
	return 0
}
func runLaunchStatus(args []string) int {
	return launchRecordVerb(args, "status", (*launch.Manager).Status)
}
func runLaunchList(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem launch list")
		return 2
	}
	records, err := launchManager().List()
	if err != nil {
		fmt.Fprintln(os.Stderr, "launch list:", err)
		return 1
	}
	for _, record := range records {
		fmt.Println(launchReport(record))
	}
	return 0
}
func runLaunchCancel(args []string) int {
	return launchRecordVerb(args, "cancel", (*launch.Manager).Cancel)
}
func launchID(args []string, verb string) (string, bool) {
	flags := flag.NewFlagSet("launch "+verb, flag.ContinueOnError)
	id := flags.String("id", "", "launch id")
	if flags.Parse(args) != nil || *id == "" || flags.NArg() != 0 {
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
func launchRecordVerb(args []string, verb string, action func(*launch.Manager, string) (launch.Record, error)) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		writeLaunchRecordUsage(os.Stdout, verb)
		return 0
	}
	id, ok := launchID(args, verb)
	if !ok {
		return 2
	}
	record, err := action(launchManager(), id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "launch %s: %v\n", verb, err)
		return 1
	}
	fmt.Println(launchReport(record))
	return 0
}
func runLaunchCensus(args []string) int {
	flags := flag.NewFlagSet("launch census", flag.ContinueOnError)
	reap := flags.Bool("reap", false, "terminate idle plugin brokers")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem launch census [--reap]")
		return 2
	}
	lines, err := launchManager().Census(*reap)
	if err != nil {
		fmt.Fprintln(os.Stderr, "launch census:", err)
		return 1
	}
	for _, line := range lines {
		fmt.Println(line)
	}
	return 0
}
func runLaunchSettings(args []string) int {
	flags := flag.NewFlagSet("launch settings", flag.ContinueOnError)
	asJSON := flags.Bool("json", false, "print structured settings")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem launch settings [--json]")
		return 2
	}
	executable, err := launchExecutable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "launch settings:", err)
		return 1
	}
	confPath := filepath.Join(filepath.Dir(executable), "..", "metasystem.conf")
	settings, err := launch.ResolveSettings(confPath, launchLookupEnv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "launch settings:", err)
		return 1
	}
	values := append([]launch.Setting{}, settings.Values...)
	shipped, err := launch.LoadShippedSeatWindow(filepath.Dir(confPath), settings.SeatWindow)
	if err != nil {
		fmt.Fprintln(os.Stderr, "launch settings:", err)
		return 1
	}
	values = append(values, launch.Setting{Key: launch.ShippedSeatWindowKey, Value: strconv.FormatInt(shipped.Tokens, 10), Source: shipped.Source, ShippedDiffersFromConf: &shipped.DiffersFromConf})
	for _, definition := range []struct {
		key      string
		fallback int64
	}{{config.ContextCeilingTokensKey, config.DefaultContextCeilingTokens}, {config.ContextHandoffMarginTokensKey, config.DefaultContextHandoffMarginTokens}} {
		params := config.GetParams{Key: definition.key, ConfPath: confPath, Default: strconv.FormatInt(definition.fallback, 10), DefaultSet: true, LookupEnv: launchLookupEnv}
		value, _, getErr := config.Get(params)
		if getErr != nil {
			fmt.Fprintln(os.Stderr, "launch settings:", getErr)
			return 1
		}
		source, originErr := config.KeyOrigin(params)
		if originErr != nil {
			fmt.Fprintln(os.Stderr, "launch settings:", originErr)
			return 1
		}
		values = append(values, launch.Setting{Key: definition.key, Value: value, Source: source})
	}
	if *asJSON {
		printJSON(values)
		return 0
	}
	for _, value := range values {
		suffix := ""
		if value.ShippedDiffersFromConf != nil && *value.ShippedDiffersFromConf {
			suffix = " shipped-differs-from-conf"
		}
		fmt.Printf("%s=%s source=%s%s\n", value.Key, value.Value, value.Source, suffix)
	}
	return 0
}
func runLaunchReport(args []string) int {
	flags := flag.NewFlagSet("launch report", flag.ContinueOnError)
	id := flags.String("id", "", "launch id")
	goal := flags.String("goal", "", "goal id")
	sinceText := flags.String("since", "", "include activity at or after this RFC3339 instant")
	asJSON := flags.Bool("json", false, "print structured report")
	if flags.Parse(args) != nil || flags.NArg() != 0 || (*id != "" && (*goal != "" || *sinceText != "")) {
		fmt.Fprintln(os.Stderr, "usage: metasystem launch report [--id <id>|--goal <goal> [--since <RFC3339>]] [--json]")
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
