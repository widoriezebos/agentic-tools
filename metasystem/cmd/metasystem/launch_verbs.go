package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func launchManager() *launch.Manager {
	prober := identity.KernelProber{}
	processes := launch.OSProcesses{Prober: prober}
	home, _ := os.UserHomeDir()
	executable, _ := os.Executable()
	scanner := launch.KernelProcessScanner{Prober: prober}
	codex := launch.CodexExec{Binary: "codex", Model: "gpt-5.6-sol", Effort: "xhigh", SessionsRoot: filepath.Join(home, ".codex", "sessions"), CommonTemplate: filepath.Join(filepath.Dir(executable), "..", "scripts", "agents", "templates", "design-common.md"), Now: time.Now, Scanner: scanner}
	claude := launch.ClaudeHeadless{Binary: "claude", ProjectsRoot: filepath.Join(home, ".claude", "projects"), Scanner: scanner}
	return &launch.Manager{Store: launch.Store{}, Adapters: map[string]launch.Adapter{"codex-exec": codex, "claude-headless": claude},
		Processes: processes, Prober: prober, Supervisor: launch.OSSupervisorStarter{Prober: prober}, Now: time.Now,
		Sleep: time.Sleep, Grace: 2 * time.Second, Poll: 50 * time.Millisecond, StartCap: launch.DefaultWaitTimeout}
}
func runLaunchStart(args []string) int {
	flags := flag.NewFlagSet("launch start", flag.ContinueOnError)
	kind := flags.String("kind", "", "launch kind")
	brief := flags.String("brief", "", "brief file")
	directory := flags.String("dir", ".", "working directory")
	goal := flags.String("goal", "", "goal id")
	tag := flags.String("tag", "", "launch tag")
	model := flags.String("model", "", "model")
	effort := flags.String("effort", "xhigh", "reasoning effort")
	resume := flags.String("resume-session", "", "Claude session id")
	page := flags.String("page", "", "page file")
	var inputs, outputs multiFlag
	flags.Var(&inputs, "input", "additional input file (repeatable)")
	flags.Var(&outputs, "output", "output file to copy into launch state (repeatable)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *kind == "" || *brief == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem launch start --kind <build|design|read|critique> --brief <file> [--dir <directory>] [--goal <id>] [--tag <tag>] [--model <model>] [--resume-session <id>] [--page <file>] [--input <file>]... [--output <file>]...")
		return 2
	}
	data := map[string]json.RawMessage{}
	if *model != "" {
		data["model"], _ = json.Marshal(*model)
	}
	data["effort"], _ = json.Marshal(*effort)
	if *resume != "" {
		data["resumeSession"], _ = json.Marshal(*resume)
	}
	record, err := launchManager().Start(launch.StartSpec{Kind: *kind, Brief: *brief, WorkingDirectory: *directory,
		Goal: *goal, Tag: *tag, Page: *page, Inputs: inputs, Outputs: outputs, AdapterData: data})
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
	flags := flag.NewFlagSet("launch wait", flag.ContinueOnError)
	id := flags.String("id", "", "launch id")
	timeout := flags.Duration("timeout", launch.DefaultWaitTimeout, "maximum wait")
	if flags.Parse(args) != nil || *id == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem launch wait --id <id> [--timeout <duration>]")
		return 2
	}
	record, terminal, err := launchManager().Wait(*id, *timeout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "launch wait:", err)
		return 1
	}
	if !terminal {
		return 3
	}
	fmt.Println(launchReport(record))
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
		fmt.Fprintf(os.Stderr, "usage: metasystem launch %s --id <id>\n", verb)
		return "", false
	}
	return *id, true
}
func launchRecordVerb(args []string, verb string, action func(*launch.Manager, string) (launch.Record, error)) int {
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
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem launch census")
		return 2
	}
	lines, err := launchManager().Census()
	if err != nil {
		fmt.Fprintln(os.Stderr, "launch census:", err)
		return 1
	}
	for _, line := range lines {
		fmt.Println(line)
	}
	return 0
}
func launchReport(record launch.Record) string {
	root, _ := launch.DefaultRoot()
	exit := "-"
	if record.ExitCode != nil {
		exit = strconv.Itoa(*record.ExitCode)
	}
	verdict := strings.ReplaceAll(record.Measurement.Verdict, "\n", " ")
	page := fmt.Sprintf("page-lines=%d page-words=%d", record.Measurement.PageLines, record.Measurement.PageWords)
	if record.Measurement.PageMissing {
		page = "page=missing"
	}
	return fmt.Sprintf("id=%s state=%s directory=%s exit=%s result-lines=%d result-words=%d result-tail=%q calls=%d turns=%d compactions=%d peak-context=%d calls-above-200k=%d %s material=%d verdict=%q",
		record.ID, record.State, filepath.Join(root, record.ID), exit, record.Measurement.ResultLines, record.Measurement.ResultWords,
		record.Measurement.ResultTail, record.Measurement.Calls, record.Measurement.Turns, record.Measurement.Compactions, record.Measurement.PeakContext,
		record.Measurement.CallsAbove200, page, record.Measurement.MaterialCount, verdict)
}
