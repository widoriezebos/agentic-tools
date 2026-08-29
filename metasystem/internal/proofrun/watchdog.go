package proofrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type WatchdogOptions struct {
	Suite           string
	Root            string
	ProgressPath    string
	DonePath        string
	LogPaths        []string
	SuiteIdentity   identity.Ref
	Silence         time.Duration
	SectionCap      time.Duration
	EvidenceTimeout time.Duration
	EvidenceMax     int64
	Poll            time.Duration
	TermGrace       time.Duration
	KillGrace       time.Duration
	Executable      string
	Output          *os.File
	ErrorOutput     *os.File
	Prober          identity.Prober
	Signal          func(int, syscall.Signal) error
	Shutdown        func() error
}

func RunWatchdog(options WatchdogOptions) error {
	if err := validateWatchdogOptions(options); err != nil {
		return err
	}
	lastBytes := totalLogBytes(options.LogPaths)
	lastGrowth := time.Now()
	ticker := time.NewTicker(options.Poll)
	defer ticker.Stop()
	for {
		if suiteDone(options.DonePath) {
			return nil
		}
		now := time.Now()
		bytes := totalLogBytes(options.LogPaths)
		if bytes > lastBytes {
			lastBytes = bytes
			lastGrowth = now
		}
		run, readErr := ReadLatestProgressRun(options.ProgressPath)
		section, sectionStarted := "startup", time.Time{}
		if readErr == nil {
			section, sectionStarted = CurrentSection(run, options.Suite)
		}
		reason := ""
		if now.Sub(lastGrowth) > options.Silence {
			reason = fmt.Sprintf("no output grew for %s", options.Silence)
		} else if !sectionStarted.IsZero() && now.Sub(sectionStarted) > options.SectionCap {
			reason = fmt.Sprintf("section exceeded its %s cap while still producing output", options.SectionCap)
		}
		if reason != "" {
			return stopStalledSuite(options, section, reason, run)
		}
		<-ticker.C
	}
}

func validateWatchdogOptions(options WatchdogOptions) error {
	if options.Suite == "" || options.Root == "" || options.ProgressPath == "" || options.DonePath == "" || len(options.LogPaths) == 0 {
		return errors.New("watchdog requires suite, root, progress, done, and log paths")
	}
	if options.SuiteIdentity.Pid < 1 || options.SuiteIdentity.StartedAtSec < 1 {
		return errors.New("watchdog requires an exact suite identity")
	}
	if options.Silence <= 0 || options.SectionCap <= 0 || options.EvidenceTimeout <= 0 || options.EvidenceMax < 1 {
		return errors.New("watchdog bounds must be positive")
	}
	if options.Poll <= 0 || options.TermGrace <= 0 || options.KillGrace <= 0 {
		return errors.New("watchdog polling and signal grace periods must be positive")
	}
	return nil
}

func totalLogBytes(paths []string) int64 {
	var total int64
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
	}
	return total
}

func stopStalledSuite(options WatchdogOptions, section, reason string, run ProgressRun) error {
	// Completion wins every race until the first kill-capable action. The
	// launcher may publish done after this watchdog observed the final quiet
	// interval but before the suite wait and output drain become visible here.
	if suiteDone(options.DonePath) {
		return nil
	}
	evidenceDir := filepath.Join(options.Root, "artifacts", "agents", "suite-failures",
		time.Now().UTC().Format("20060102T150405Z")+"-watchdog-"+strconv.FormatInt(options.SuiteIdentity.Pid, 10))
	sources := append([]string{}, run.Header.TmpPaths...)
	sources = append(sources, run.Header.LogPaths...)
	if len(sources) == 0 {
		sources = append(sources, options.LogPaths...)
	}
	evidenceNote := preserveWithBound(options, evidenceDir, sources)
	if suiteDone(options.DonePath) {
		return nil
	}

	prober := options.Prober
	if prober == nil {
		prober = identity.KernelProber{}
	}
	if state := identity.AliveRef(prober, options.SuiteIdentity); state != identity.Alive {
		return fmt.Errorf("suite stalled in section %s (%s); evidence: %s; kill refused because suite pid %d no longer has its recorded start identity (%s)",
			section, reason, evidenceNote, options.SuiteIdentity.Pid, state)
	}
	if suiteDone(options.DonePath) {
		return nil
	}
	if options.Shutdown != nil {
		if err := options.Shutdown(); err != nil {
			fmt.Fprintf(options.ErrorOutput, "suite watchdog: supervision shutdown was incomplete: %v\n", err)
		}
	} else if err := shutdownSupervision(options); err != nil {
		fmt.Fprintf(options.ErrorOutput, "suite watchdog: supervision shutdown was incomplete: %v\n", err)
	}
	if err := signalSuiteGroup(options, prober); err != nil {
		fmt.Fprintf(options.ErrorOutput, "suite watchdog: suite process-group cleanup was incomplete: %v\n", err)
	}
	if err := sweepExecutionGuard(options, prober); err != nil {
		fmt.Fprintf(options.ErrorOutput, "suite watchdog: execution-guard sweep was incomplete: %v\n", err)
	}
	return fmt.Errorf("suite stalled in section %s (%s); evidence preserved before kill at %s (%s)", section, reason, evidenceDir, evidenceNote)
}

func suiteDone(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func preserveWithBound(options WatchdogOptions, destination string, sources []string) string {
	ctx, cancel := context.WithTimeout(context.Background(), options.EvidenceTimeout)
	defer cancel()
	args := []string{"proof-run", "preserve", "--destination", destination, "--max-bytes", strconv.FormatInt(options.EvidenceMax, 10)}
	for _, source := range sources {
		args = append(args, "--source", source)
	}
	command := exec.CommandContext(ctx, options.Executable, args...)
	output, err := command.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		note := fmt.Sprintf("DROPPED evidence copy exceeded its %s timeout; partial evidence retained", options.EvidenceTimeout)
		_ = AppendEvidenceNote(destination, note)
		fmt.Fprintln(options.ErrorOutput, "suite watchdog:", note)
		return note
	}
	if err != nil {
		note := fmt.Sprintf("DROPPED evidence copy failed: %v: %s", err, string(output))
		_ = AppendEvidenceNote(destination, note)
		fmt.Fprintln(options.ErrorOutput, "suite watchdog:", note)
		return note
	}
	note := string(output)
	if note == "" {
		note = "bounded copy completed"
	}
	return note
}

func shutdownSupervision(options WatchdogOptions) error {
	script := filepath.Join(options.Root, "scripts", "agents", "arm-supervision.sh")
	if _, err := os.Stat(script); err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, script, "--repo", options.Root, "--shutdown")
	command.Stdout = options.Output
	command.Stderr = options.ErrorOutput
	if err := command.Run(); err != nil {
		return err
	}
	return nil
}

func signalSuiteGroup(options WatchdogOptions, prober identity.Prober) error {
	pgid := -int(options.SuiteIdentity.Pid)
	if err := signalAuthenticated(options, prober, options.SuiteIdentity, pgid, syscall.SIGCONT, "suite process group"); err != nil {
		return fmt.Errorf("continue process group: %w", err)
	}
	if err := signalAuthenticated(options, prober, options.SuiteIdentity, pgid, syscall.SIGTERM, "suite process group"); err != nil {
		return fmt.Errorf("terminate process group: %w", err)
	}
	waitForGroup(options.SuiteIdentity.Pid, options.TermGrace)
	if err := signalAuthenticated(options, prober, options.SuiteIdentity, pgid, syscall.SIGKILL, "suite process group"); err != nil {
		return fmt.Errorf("kill process group: %w", err)
	}
	waitForGroup(options.SuiteIdentity.Pid, options.KillGrace)
	return nil
}

func signalAuthenticated(options WatchdogOptions, prober identity.Prober, ref identity.Ref, target int, signalValue syscall.Signal, label string) error {
	if state := identity.AliveRef(prober, ref); state != identity.Alive {
		return fmt.Errorf("%s signal refused for %s because pid %d no longer has its recorded start identity (%s)", signalValue, label, ref.Pid, state)
	}
	signal := options.Signal
	if signal == nil {
		signal = syscall.Kill
	}
	if err := signal(target, signalValue); err != nil && err != syscall.ESRCH {
		return err
	}
	return nil
}

func waitForGroup(pgid int64, duration time.Duration) {
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(-int(pgid), 0); err == syscall.ESRCH {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

type executionGuardMember struct {
	Pid          int64 `json:"pid"`
	PidStartedAt int64 `json:"pidStartedAt"`
}

type executionGuardRecord struct {
	Members []executionGuardMember `json:"members"`
}

func sweepExecutionGuard(options WatchdogOptions, prober identity.Prober) error {
	path := filepath.Join(options.Root, "artifacts", "agents", "supervision", "gate-runs", "checkout-execution.lock.d", "owner.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var record executionGuardRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return fmt.Errorf("read execution-guard members: %w", err)
	}
	var failures []error
	for _, member := range record.Members {
		if member.Pid == options.SuiteIdentity.Pid && member.PidStartedAt == options.SuiteIdentity.StartedAtSec {
			continue
		}
		ref := identity.Ref{Pid: member.Pid, StartedAtSec: member.PidStartedAt}
		if identity.AliveRef(prober, ref) != identity.Alive {
			continue
		}
		if err := stopGuardMember(options, ref, prober); err != nil {
			failures = append(failures, fmt.Errorf("execution-guard member %d: %w", member.Pid, err))
		}
	}
	return errors.Join(failures...)
}

func stopGuardMember(options WatchdogOptions, ref identity.Ref, prober identity.Prober) error {
	target := int(ref.Pid)
	if pgid, err := syscall.Getpgid(target); err == nil && pgid == target {
		target = -target
	}
	if err := signalAuthenticated(options, prober, ref, target, syscall.SIGCONT, "execution-guard member"); err != nil {
		return fmt.Errorf("continue execution-guard member %d: %w", ref.Pid, err)
	}
	if err := signalAuthenticated(options, prober, ref, target, syscall.SIGTERM, "execution-guard member"); err != nil {
		return fmt.Errorf("terminate execution-guard member %d: %w", ref.Pid, err)
	}
	deadline := time.Now().Add(options.TermGrace)
	for time.Now().Before(deadline) && identity.AliveRef(prober, ref) == identity.Alive {
		time.Sleep(20 * time.Millisecond)
	}
	if err := signalAuthenticated(options, prober, ref, target, syscall.SIGKILL, "execution-guard member"); err != nil {
		return fmt.Errorf("kill execution-guard member %d: %w", ref.Pid, err)
	}
	return nil
}
