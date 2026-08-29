package proofrun

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type LaunchOptions struct {
	Suite              string
	Root               string
	ConfPath           string
	ProgressPath       string
	LogPath            string
	TmpPaths           []string
	Banner             string
	ExpectedSections   []string
	TwiceConsulted     map[string]bool
	Silence            time.Duration
	SectionCap         time.Duration
	EvidenceTimeout    time.Duration
	EvidenceMax        int64
	Poll               time.Duration
	TermGrace          time.Duration
	KillGrace          time.Duration
	WatchdogExecutable string
	Command            []string
	Output             io.Writer
	ErrorOutput        io.Writer
}

func LaunchSuite(options LaunchOptions) int {
	if err := validateLaunchOptions(options); err != nil {
		fmt.Fprintln(options.ErrorOutput, "suite launcher:", err)
		return 2
	}
	if err := os.MkdirAll(filepath.Dir(options.LogPath), 0o700); err != nil {
		fmt.Fprintln(options.ErrorOutput, "suite launcher:", err)
		return 1
	}
	log, err := os.OpenFile(options.LogPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintln(options.ErrorOutput, "suite launcher:", err)
		return 1
	}
	defer log.Close()
	combined := &lockedWriter{writers: []io.Writer{options.Output, log}}
	combinedErr := &lockedWriter{writers: []io.Writer{options.ErrorOutput, log}}
	if err := AppendProgressHeader(options.ProgressPath, ProgressHeader{TmpPaths: options.TmpPaths, LogPaths: []string{options.LogPath}}); err != nil {
		fmt.Fprintln(combinedErr, "suite launcher:", err)
		return 1
	}
	fmt.Fprintln(combined, options.Banner)

	suite := exec.Command(options.Command[0], options.Command[1:]...)
	suite.Dir = options.Root
	suite.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	suiteOut, err := suite.StdoutPipe()
	if err != nil {
		fmt.Fprintln(combinedErr, "suite launcher:", err)
		return 1
	}
	suiteErr, err := suite.StderrPipe()
	if err != nil {
		fmt.Fprintln(combinedErr, "suite launcher:", err)
		return 1
	}
	if err := suite.Start(); err != nil {
		fmt.Fprintln(combinedErr, "suite launcher:", err)
		return 1
	}
	suiteExact, state, probeErr := (identity.KernelProber{}).Probe(int64(suite.Process.Pid))
	if probeErr != nil || state != identity.Alive {
		_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
		_ = suite.Wait()
		fmt.Fprintf(combinedErr, "suite launcher: cannot record exact suite identity: %v (%s)\n", probeErr, state)
		return 1
	}

	var copies sync.WaitGroup
	copies.Add(2)
	go copyStream(&copies, combined, suiteOut)
	go copyStream(&copies, combinedErr, suiteErr)
	donePath := options.LogPath + ".done"
	_ = os.Remove(donePath)
	watchdog := watchdogCommand(options, suiteExact.Ref(), donePath)
	watchdog.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	watchdogOut, err := watchdog.StdoutPipe()
	if err != nil {
		_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
		_ = suite.Wait()
		fmt.Fprintln(combinedErr, "suite launcher:", err)
		return 1
	}
	watchdogErr, err := watchdog.StderrPipe()
	if err != nil {
		_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
		_ = suite.Wait()
		fmt.Fprintln(combinedErr, "suite launcher:", err)
		return 1
	}
	if err := watchdog.Start(); err != nil {
		_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
		_ = suite.Wait()
		fmt.Fprintln(combinedErr, "suite launcher: start sibling watchdog:", err)
		return 1
	}
	copies.Add(2)
	go copyStream(&copies, combined, watchdogOut)
	go copyStream(&copies, combinedErr, watchdogErr)

	suiteErrWait := suite.Wait()
	if err := touchDone(donePath); err != nil {
		fmt.Fprintln(combinedErr, "suite launcher: write watchdog done file:", err)
	}
	watchdogErrWait := watchdog.Wait()
	copies.Wait()
	defer os.Remove(donePath)

	result := exitStatus(suiteErrWait)
	if watchdogErrWait != nil {
		result = 1
	}
	if err := assertBanner(options.LogPath, options.Banner); err != nil {
		fmt.Fprintln(combinedErr, "suite launcher:", err)
		result = 1
	}
	if len(options.ExpectedSections) > 0 {
		run, err := ReadLatestProgressRun(options.ProgressPath)
		if err == nil {
			err = AssertSectionProgress(run, options.Suite, options.ExpectedSections, options.TwiceConsulted)
		}
		if err != nil {
			fmt.Fprintln(combinedErr, "suite launcher:", err)
			result = 1
		}
	}
	return result
}

func validateLaunchOptions(options LaunchOptions) error {
	if options.Suite == "" || options.Root == "" || options.ConfPath == "" || options.ProgressPath == "" || options.LogPath == "" || options.Banner == "" {
		return errors.New("suite, root, configuration, progress, log, and banner are required")
	}
	if options.Silence <= 0 || options.SectionCap <= 0 || options.EvidenceTimeout <= 0 || options.EvidenceMax < 1 {
		return errors.New("suite limits must be positive")
	}
	if options.Poll <= 0 || options.TermGrace <= 0 || options.KillGrace <= 0 {
		return errors.New("watchdog polling and signal grace periods must be positive")
	}
	if len(options.Command) == 0 {
		return errors.New("suite command is required")
	}
	if options.Output == nil {
		options.Output = io.Discard
	}
	if options.ErrorOutput == nil {
		options.ErrorOutput = io.Discard
	}
	return nil
}

type lockedWriter struct {
	mu      sync.Mutex
	writers []io.Writer
}

func (w *lockedWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, writer := range w.writers {
		if writer == nil {
			continue
		}
		if _, err := writer.Write(data); err != nil {
			return 0, err
		}
	}
	return len(data), nil
}

func copyStream(group *sync.WaitGroup, destination io.Writer, source io.Reader) {
	defer group.Done()
	_, _ = io.Copy(destination, source)
}

func watchdogCommand(options LaunchOptions, ref identity.Ref, donePath string) *exec.Cmd {
	executable := options.WatchdogExecutable
	if executable == "" {
		executable, _ = os.Executable()
	}
	args := []string{
		"proof-run", "watchdog",
		"--suite", options.Suite,
		"--root", options.Root,
		"--conf", options.ConfPath,
		"--progress", options.ProgressPath,
		"--done", donePath,
		"--suite-pid", strconv.FormatInt(ref.Pid, 10),
		"--suite-started-at", strconv.FormatInt(ref.StartedAtSec, 10),
		"--suite-start-ticks", strconv.FormatInt(ref.StartTicks, 10),
		"--suite-boot-id", ref.BootID,
		"--silence-ms", strconv.FormatInt(options.Silence.Milliseconds(), 10),
		"--section-cap-ms", strconv.FormatInt(options.SectionCap.Milliseconds(), 10),
		"--evidence-timeout-ms", strconv.FormatInt(options.EvidenceTimeout.Milliseconds(), 10),
		"--evidence-max-bytes", strconv.FormatInt(options.EvidenceMax, 10),
		"--poll-ms", strconv.FormatInt(options.Poll.Milliseconds(), 10),
		"--term-grace-ms", strconv.FormatInt(options.TermGrace.Milliseconds(), 10),
		"--kill-grace-ms", strconv.FormatInt(options.KillGrace.Milliseconds(), 10),
	}
	for _, path := range []string{options.LogPath} {
		args = append(args, "--log", path)
	}
	return exec.Command(executable, args...)
}

func touchDone(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	return file.Close()
}

func exitStatus(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if code := exitErr.ExitCode(); code >= 0 {
			return code
		}
	}
	return 1
}

func assertBanner(path, banner string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("read cost banner log: %w", err)
	}
	defer file.Close()
	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if scanner.Text() == banner {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("cost banner appeared %d times in the suite log; expected exactly once", count)
	}
	return nil
}

func ReadSelectorSections(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var sections []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		id, _, found := strings.Cut(line, "\t")
		if !found || id == "" {
			return nil, fmt.Errorf("selector row is invalid: %q", line)
		}
		sections = append(sections, id)
	}
	return sections, scanner.Err()
}
