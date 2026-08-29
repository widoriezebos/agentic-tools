package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func runProofRunLaunch(args []string) int {
	flags := flag.NewFlagSet("proof-run launch", flag.ContinueOnError)
	suite := flags.String("suite", "", "suite name")
	root := flags.String("root", "", "metasystem root")
	conf := flags.String("conf", "", "metasystem configuration")
	progress := flags.String("progress", "", "append-only progress JSONL")
	logPath := flags.String("log", "", "suite output log")
	banner := flags.String("banner", "", "one-line cost banner")
	selector := flags.String("selector", "", "validation section selector")
	selected := flags.String("selected", "", "single selected section")
	var tmpPaths repeatedFlag
	flags.Var(&tmpPaths, "tmp", "temporary evidence path (repeatable)")
	silenceMS := flags.Int64("silence-ms", 0, "fixture override for output-silence milliseconds")
	sectionCapMS := flags.Int64("section-cap-ms", 0, "fixture override for section-cap milliseconds")
	evidenceTimeoutMS := flags.Int64("evidence-timeout-ms", 0, "fixture override for evidence-copy milliseconds")
	evidenceMaxBytes := flags.Int64("evidence-max-bytes", 0, "fixture override for evidence-copy bytes")
	pollMS := flags.Int64("poll-ms", 1000, "watchdog poll milliseconds")
	termGraceMS := flags.Int64("term-grace-ms", 5000, "TERM grace milliseconds")
	killGraceMS := flags.Int64("kill-grace-ms", 1000, "KILL observation milliseconds")
	if flags.Parse(args) != nil {
		return 2
	}
	command := flags.Args()
	if len(command) == 0 || *root == "" || *conf == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem proof-run launch --suite S --root R --conf F --progress P --log L --banner B [--selector F] -- COMMAND...")
		return 2
	}
	limits, err := resolveProofRunLimits(*conf)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run launch:", err)
		return 1
	}
	if *silenceMS > 0 {
		limits.silence = time.Duration(*silenceMS) * time.Millisecond
	}
	if *sectionCapMS > 0 {
		limits.sectionCap = time.Duration(*sectionCapMS) * time.Millisecond
	}
	if *evidenceTimeoutMS > 0 {
		limits.evidenceTimeout = time.Duration(*evidenceTimeoutMS) * time.Millisecond
	}
	if *evidenceMaxBytes > 0 {
		limits.evidenceMax = *evidenceMaxBytes
	}
	expected, repeated, err := selectedSections(*selector, *selected)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run launch:", err)
		return 1
	}
	return proofrun.LaunchSuite(proofrun.LaunchOptions{
		Suite: *suite, Root: *root, ConfPath: *conf, ProgressPath: *progress, LogPath: *logPath,
		TmpPaths: tmpPaths, Banner: *banner, ExpectedSections: expected, TwiceConsulted: repeated,
		Silence: limits.silence, SectionCap: limits.sectionCap,
		EvidenceTimeout: limits.evidenceTimeout, EvidenceMax: limits.evidenceMax,
		Poll:      time.Duration(*pollMS) * time.Millisecond,
		TermGrace: time.Duration(*termGraceMS) * time.Millisecond,
		KillGrace: time.Duration(*killGraceMS) * time.Millisecond,
		Command:   command, Output: os.Stdout, ErrorOutput: os.Stderr,
	})
}

type proofRunLimits struct {
	silence         time.Duration
	sectionCap      time.Duration
	evidenceTimeout time.Duration
	evidenceMax     int64
}

func proofRunConfigProblems(confPath string) ([]string, error) {
	var problems []string
	for _, knob := range []struct {
		name    string
		minimum int
		maximum int
	}{
		{"suite.progress-silence-min", 1, 600},
		{"suite.section-cap-min", 1, 600},
		{"suite.evidence-copy-timeout-sec", 1, 600},
		{"suite.evidence-copy-max-mb", 1, 10240},
	} {
		raw, found, err := config.ConfLookup(confPath, knob.name)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < knob.minimum || parsed > knob.maximum {
			problems = append(problems, fmt.Sprintf("%s must be an integer from %d through %d, got %q", knob.name, knob.minimum, knob.maximum, raw))
		}
	}
	return problems, nil
}

func resolveProofRunLimits(confPath string) (proofRunLimits, error) {
	read := func(key string, def, minimum, maximum int) (int, error) {
		value, _, err := config.Get(config.GetParams{
			Key: key, Default: strconv.Itoa(def), DefaultSet: true, ConfPath: confPath,
		})
		if err != nil {
			return 0, err
		}
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < minimum || parsed > maximum {
			return 0, fmt.Errorf("%s must be an integer from %d through %d", key, minimum, maximum)
		}
		return parsed, nil
	}
	silence, err := read("suite.progress-silence-min", 30, 1, 600)
	if err != nil {
		return proofRunLimits{}, err
	}
	section, err := read("suite.section-cap-min", 45, 1, 600)
	if err != nil {
		return proofRunLimits{}, err
	}
	evidenceTimeout, err := read("suite.evidence-copy-timeout-sec", 60, 1, 600)
	if err != nil {
		return proofRunLimits{}, err
	}
	evidenceMB, err := read("suite.evidence-copy-max-mb", 512, 1, 10240)
	if err != nil {
		return proofRunLimits{}, err
	}
	return proofRunLimits{
		silence:         time.Duration(silence) * time.Minute,
		sectionCap:      time.Duration(section) * time.Minute,
		evidenceTimeout: time.Duration(evidenceTimeout) * time.Second,
		evidenceMax:     int64(evidenceMB) * 1024 * 1024,
	}, nil
}

func selectedSections(selector, selected string) ([]string, map[string]bool, error) {
	if selector == "" {
		if selected != "" {
			return []string{selected}, map[string]bool{}, nil
		}
		return nil, map[string]bool{}, nil
	}
	command := exec.Command("bash", selector, "list")
	output, err := command.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("list validation selector: %w", err)
	}
	var sections []string
	known := map[string]bool{}
	for _, line := range bytes.Split(bytes.TrimSpace(output), []byte{'\n'}) {
		id, _, found := bytes.Cut(line, []byte{'\t'})
		if !found || len(id) == 0 {
			return nil, nil, fmt.Errorf("selector emitted invalid row %q", line)
		}
		section := string(id)
		sections = append(sections, section)
		known[section] = true
	}

	command = exec.Command("bash", selector, "twice")
	output, err = command.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("read twice-consulted validation sections: %w", err)
	}
	declaredTwice := map[string]bool{}
	trimmed := bytes.TrimSpace(output)
	if len(trimmed) > 0 {
		for _, line := range bytes.Split(trimmed, []byte{'\n'}) {
			section := string(line)
			if !known[section] {
				return nil, nil, fmt.Errorf("twice-consulted section %q is absent from the selector", section)
			}
			declaredTwice[section] = true
		}
	}
	if selected == "" {
		return sections, declaredTwice, nil
	}
	// An enumeration run drives ONE call site, so even a
	// declared-twice section is consulted exactly once here — the
	// double consult exists only across a full run's two sites
	// (found live: 'expected 2 of each' broke every single-section
	// run of engine-delivery-contract).
	return []string{selected}, map[string]bool{}, nil
}

func runProofRunWatchdog(args []string) int {
	flags := flag.NewFlagSet("proof-run watchdog", flag.ContinueOnError)
	suite := flags.String("suite", "", "suite name")
	root := flags.String("root", "", "metasystem root")
	conf := flags.String("conf", "", "metasystem configuration")
	progress := flags.String("progress", "", "progress JSONL")
	done := flags.String("done", "", "launcher done file")
	suitePID := flags.Int64("suite-pid", 0, "suite process identifier")
	suiteStarted := flags.Int64("suite-started-at", 0, "suite start epoch second")
	suiteTicks := flags.Int64("suite-start-ticks", 0, "suite boot-relative start ticks")
	suiteBoot := flags.String("suite-boot-id", "", "suite boot identity")
	silenceMS := flags.Int64("silence-ms", 0, "output-silence milliseconds")
	sectionCapMS := flags.Int64("section-cap-ms", 0, "section-cap milliseconds")
	evidenceTimeoutMS := flags.Int64("evidence-timeout-ms", 0, "evidence timeout milliseconds")
	evidenceMax := flags.Int64("evidence-max-bytes", 0, "evidence byte cap")
	pollMS := flags.Int64("poll-ms", 1000, "poll milliseconds")
	termGraceMS := flags.Int64("term-grace-ms", 5000, "TERM grace milliseconds")
	killGraceMS := flags.Int64("kill-grace-ms", 1000, "KILL observation milliseconds")
	var logs repeatedFlag
	flags.Var(&logs, "log", "watched output log (repeatable)")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *conf == "" {
		return 2
	}
	// Re-read the layered configuration in the sibling process immediately
	// before watchdog startup. A conf.local or environment change between
	// launcher resolution and spawn can therefore only refuse, never install
	// an unlawful operational window. Fixture duration overrides remain the
	// passed values after this effective-value validation.
	if _, err := resolveProofRunLimits(*conf); err != nil {
		fmt.Fprintln(os.Stderr, "proof-run watchdog:", err)
		return 1
	}
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run watchdog:", err)
		return 1
	}
	err = proofrun.RunWatchdog(proofrun.WatchdogOptions{
		Suite: *suite, Root: *root, ProgressPath: *progress, DonePath: *done, LogPaths: logs,
		SuiteIdentity:   identity.Ref{Pid: *suitePID, StartedAtSec: *suiteStarted, StartTicks: *suiteTicks, BootID: *suiteBoot},
		Silence:         time.Duration(*silenceMS) * time.Millisecond,
		SectionCap:      time.Duration(*sectionCapMS) * time.Millisecond,
		EvidenceTimeout: time.Duration(*evidenceTimeoutMS) * time.Millisecond,
		EvidenceMax:     *evidenceMax, Poll: time.Duration(*pollMS) * time.Millisecond,
		TermGrace:  time.Duration(*termGraceMS) * time.Millisecond,
		KillGrace:  time.Duration(*killGraceMS) * time.Millisecond,
		Executable: executable, Output: os.Stdout, ErrorOutput: os.Stderr,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "suite watchdog:", err)
		return 1
	}
	return 0
}

func runProofRunPreserve(args []string) int {
	flags := flag.NewFlagSet("proof-run preserve", flag.ContinueOnError)
	destination := flags.String("destination", "", "evidence destination")
	maxBytes := flags.Int64("max-bytes", 0, "evidence byte cap")
	var sources repeatedFlag
	flags.Var(&sources, "source", "evidence source (repeatable)")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	result, err := proofrun.PreserveEvidence(*destination, sources, *maxBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run preserve:", err)
		return 1
	}
	fmt.Printf("copied %d bytes; dropped %d paths; copy errors %d\n", result.CopiedBytes, len(result.Dropped), len(result.Errors))
	for _, dropped := range result.Dropped {
		fmt.Printf("DROPPED %s\n", dropped)
	}
	for _, copyError := range result.Errors {
		fmt.Printf("ERROR %s\n", copyError)
	}
	return 0
}

func runProofRunAssert(args []string) int {
	flags := flag.NewFlagSet("proof-run assert", flag.ContinueOnError)
	progress := flags.String("progress", "", "progress JSONL")
	suite := flags.String("suite", "", "suite name")
	selector := flags.String("selector", "", "section selector")
	selected := flags.String("selected", "", "single selected section")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	expected, repeated, err := selectedSections(*selector, *selected)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run assert:", err)
		return 1
	}
	run, err := proofrun.ReadLatestProgressRun(*progress)
	if err == nil {
		err = proofrun.AssertSectionProgress(run, *suite, expected, repeated)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "proof-run assert:", err)
		return 1
	}
	return 0
}

func runProofRunBanner(args []string) int {
	flags := flag.NewFlagSet("proof-run banner", flag.ContinueOnError)
	suite := flags.String("suite", "", "suite name")
	root := flags.String("root", "", "metasystem root")
	progress := flags.String("progress", "", "progress JSONL path")
	logPath := flags.String("log", "", "suite log path")
	if flags.Parse(args) != nil {
		return 2
	}
	if *suite == "" || *root == "" || *progress == "" || *logPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem proof-run banner --suite S --root R --progress P --log L")
		return 2
	}
	state := proofRunWitnessState(*root)
	duration := "minutes"
	if state == "unarmed" {
		duration = "full-gate"
	}
	fmt.Printf("suite-cost suite=%s witness=%s duration=%s heartbeat=%s logs=%s\n",
		*suite, state, duration, proofRunDisplayPath(*root, *progress), proofRunDisplayPath(*root, *logPath))
	return 0
}

func runProofRunHeartbeat(args []string) int {
	flags := flag.NewFlagSet("proof-run heartbeat", flag.ContinueOnError)
	root := flags.String("root", "", "watched root")
	if flags.Parse(args) != nil || *root == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem proof-run heartbeat --root R")
		return 2
	}
	heartbeat, ok := deepestSuiteHeartbeat(*root, time.Now())
	if !ok {
		return 1
	}
	fmt.Println(heartbeat)
	return 0
}

func deepestSuiteHeartbeat(root string, now time.Time) (string, bool) {
	progress := filepath.Join(root, "artifacts", "agents", "supervision", "suite-progress.jsonl")
	run, err := proofrun.ReadLatestProgressRun(progress)
	if err != nil {
		return "", false
	}
	return proofrun.DeepestLiveHeartbeat(run, now)
}

func startSuiteProgressPrinter(root string, interval time.Duration, output io.Writer) func() {
	if root == "" || output == nil {
		return func() {}
	}
	if interval <= 0 {
		interval = 2 * time.Second
	}
	last := ""
	printChanged := func() {
		heartbeat, ok := deepestSuiteHeartbeat(root, time.Now())
		if ok && heartbeat != last {
			fmt.Fprintln(output, heartbeat)
			last = heartbeat
		}
	}
	printChanged()
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				printChanged()
			case <-stop:
				return
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}

func proofRunDisplayPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return path
}

func proofRunWitnessState(root string) string {
	if os.Getenv("METASYSTEM_GATE_WITNESS") != "" {
		if proofRunWitnessUsable(root) {
			if export := os.Getenv("METASYSTEM_GATE_WITNESS_EXPORT"); export != "" {
				if info, err := os.Stat(export); err == nil && info.IsDir() {
					return "frozen"
				}
				return "unarmed"
			}
			return "armed"
		}
		return "unarmed"
	}
	if proofRunFrozenWillRun(root) {
		return "frozen"
	}
	return "unarmed"
}

func proofRunWitnessUsable(root string) bool {
	script := filepath.Join(root, "scripts", "agents", "go-gate.sh")
	command := exec.Command("bash", script, "--witness-check-only")
	command.Dir = root
	command.Env = proofRunEnvironment("METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE", "ENGINE")
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	return command.Run() == nil
}

func proofRunEnvironment(name, value string) []string {
	prefix := name + "="
	environment := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, prefix) {
			environment = append(environment, entry)
		}
	}
	return append(environment, prefix+value)
}

func proofRunFrozenWillRun(root string) bool {
	if os.Getenv("METASYSTEM_COVERAGE_RATCHET_SEED") == "1" ||
		os.Getenv("METASYSTEM_GATE_FORCE") == "1" ||
		os.Getenv("METASYSTEM_DELIVERY_CONTRACT") == "1" || proofRunAlternateGoInputs() {
		return false
	}
	dirty, proved := proofRunEngineDirty(root)
	return proved && dirty
}

func proofRunAlternateGoInputs() bool {
	fields := strings.Fields(os.Getenv("GOFLAGS"))
	for _, field := range fields {
		if field == "-modfile" || strings.HasPrefix(field, "-modfile=") ||
			field == "-overlay" || strings.HasPrefix(field, "-overlay=") {
			return true
		}
	}
	return false
}

func proofRunEngineDirty(root string) (bool, bool) {
	prefixBytes, err := exec.Command("git", "-C", root, "rev-parse", "--show-prefix").Output()
	if err != nil {
		return false, false
	}
	gitRootBytes, err := exec.Command("git", "-C", root, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return false, false
	}
	policy, err := behaviorsurface.Load()
	if err != nil {
		return false, false
	}
	prefix := strings.TrimSpace(string(prefixBytes))
	gitRoot := strings.TrimSpace(string(gitRootBytes))
	commands := [][]string{
		{"diff", "--no-renames", "--name-only", "-z", "HEAD", "--"},
		{"ls-files", "--others", "--exclude-standard", "--full-name", "-z"},
		{"ls-files", "--others", "-i", "--exclude-standard", "--full-name", "-z"},
	}
	for _, arguments := range commands {
		output, err := exec.Command("git", append([]string{"-C", gitRoot}, arguments...)...).Output()
		if err != nil {
			return false, false
		}
		for _, path := range bytes.Split(output, []byte{0}) {
			if len(path) == 0 {
				continue
			}
			included, err := policy.Includes(behaviorsurface.Engine, string(path), prefix)
			if err != nil {
				return false, false
			}
			if included {
				return true, true
			}
		}
	}
	return false, true
}
