package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	dispatchpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// runSuperviseComponent runs one supervised component — the watcher or the
// reaper — launched by the owner. Every interval it rewrites its heartbeat (the
// owner reads liveness by heartbeat freshness) AND does its real work:
//
//   - watcher: runs the repository process census and publishes the verdict to
//     last-census.json, holding the census-writer lock for its whole life so at
//     most one census stream exists per supervision directory.
//   - reaper: sweeps the job records, applying the process-lost and budget-cap
//     terminal verdicts.
//
// A transient error in either job is logged and the loop continues — a
// component must not die on a bad scan or an unreadable record; the owner tears
// it down deliberately by signal, or replaces it when its heartbeat goes stale.
func runSuperviseComponent(args []string) int {
	flags := flag.NewFlagSet("supervise component", flag.ContinueOnError)
	component := flags.String("component", "", "watcher | reaper")
	repo := flags.String("repo", "", "checkout root the component operates on")
	scope := flags.String("scope", "", "census scope (git toplevel); defaults to --repo")
	tag := flags.String("tag", "", "component instance tag")
	heartbeat := flags.String("heartbeat", "", "heartbeat file path")
	intervalSec := flags.Int("interval", 60, "heartbeat/work interval seconds")
	generation := flags.Int("generation", 0, "supervision generation this component serves")
	capMin := flags.Int("cap-min", 0, "loaded watcher cap ceiling for the heartbeat attestation (defaults to the interval when unset)")
	// crashOnStart reproduces the pure crash-loop shape (D-2): a
	// component that dies on startup WITHOUT ever beating, so it never
	// reads Healthy and the breaker advances monotonically to N. A
	// component that beat even once would reset the breaker (the
	// four-fail-then-reset pattern), which is a different, non-terminal
	// shape. Fixture-only.
	crashOnStart := flags.Bool("crash-on-start", false, "exit immediately without beating (fixture-only)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *component == "" || *tag == "" || *heartbeat == "" {
		fmt.Fprintln(os.Stderr, "supervise component: --component, --tag, --heartbeat required")
		return 2
	}
	if *component == "watcher" || *component == "reaper" {
		if *repo == "" {
			fmt.Fprintln(os.Stderr, "supervise component: --repo is required for the "+*component)
			return 2
		}
		if *generation < 1 {
			fmt.Fprintln(os.Stderr, "supervise component: --generation is required for the "+*component)
			return 2
		}
	}
	if *crashOnStart {
		fmt.Fprintln(os.Stderr, "supervise component: crash-on-start (fixture)")
		return 1
	}

	// This process's own identity: the census-writer lock publishes it, and it
	// lets a heartbeat name the exact process it beats for.
	self := identity.Ref{Pid: int64(os.Getpid())}
	if exact, state, err := (identity.KernelProber{}).Probe(self.Pid); err == nil && state == identity.Alive {
		self = exact.Ref()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	if *capMin < 1 {
		*capMin = *intervalSec
	}
	beat := heartbeatWriter(*heartbeat, *component, self, *tag, *intervalSec, *capMin)

	if *scope == "" {
		*scope = *repo
	}
	var work func() error
	switch *component {
	case "watcher":
		release, pass, ok := setupWatcher(*repo, *scope, self, *tag, *generation, *intervalSec)
		if !ok {
			return 1
		}
		defer release()
		work = pass
	case "reaper":
		reaperPass := setupReaper(*repo)
		work = func() error {
			reaperPass()
			return nil
		}
	default:
		// An unknown component still beats, so a mislabelled owner launch is
		// observable rather than a silent no-op.
		work = func() error { return nil }
	}

	// A definitively absent checkout root means the supervised thing is gone:
	// exit rather than beat. A heartbeat or verdict write would re-create the
	// supervision tree (atomic writes make parent directories), resurrecting
	// a deleted checkout and turning the owner's purpose-gone into a
	// spurious superseded.
	rootGone := func() bool {
		if *repo == "" {
			return false
		}
		_, statErr := os.Stat(*repo)
		return errors.Is(statErr, os.ErrNotExist)
	}

	if rootGone() {
		fmt.Fprintln(os.Stderr, "supervise component: checkout root is gone; exiting")
		return 0
	}
	beat() // beat once immediately so the owner sees liveness fast
	if err := work(); err != nil {
		fmt.Fprintln(os.Stderr, "supervise component:", err)
	} // and produce a first verdict/sweep without waiting a full interval
	ticker := time.NewTicker(time.Duration(*intervalSec) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return 0
		case <-ticker.C:
			if rootGone() {
				fmt.Fprintln(os.Stderr, "supervise component: checkout root is gone; exiting")
				return 0
			}
			beat()
			if err := work(); err != nil {
				fmt.Fprintln(os.Stderr, "supervise component:", err)
			}
		}
	}
}

// heartbeatWriter returns a never-failing closure that rewrites the component's
// heartbeat with a fresh observedAtEpoch. A write error is swallowed so a full
// disk cannot crash a component (the stale heartbeat makes the owner replace it).
func heartbeatWriter(path, component string, self identity.Ref, tag string, intervalSec, capMin int) func() {
	return func() {
		_ = supervise.WriteHeartbeat(path, component, self, tag, intervalSec, capMin)
	}
}

// setupWatcher claims the census-writer lock and returns the release closure
// and the per-interval census pass. It returns ok=false (and logs) when a live
// writer already owns the lock — the owner then sees this watcher fail and, once
// the incumbent stops, relaunches one that can claim it.
func setupWatcher(repo, scope string, self identity.Ref, tag string, generation, intervalSec int) (release func(), pass func() error, ok bool) {
	supervisionDir := supervise.SupervisionDir(repo)
	lock := &supervise.CensusWriterLock{
		Dir: supervisionDir, Self: self, Tag: tag, Prober: identity.KernelProber{},
	}
	if err := lock.Claim(); err != nil {
		fmt.Fprintln(os.Stderr, "supervise component watcher:", err)
		return nil, nil, false
	}

	cfg := watcherConfig(repo, scope, supervisionDir, intervalSec)
	pass = func() error {
		attempt, err := steward.BeginComponentAttempt(repo, "repo-watcher", generation, self, time.Now())
		if err != nil {
			return fmt.Errorf("record watcher attempt: %w", err)
		}
		var passErrors []error
		if err := cfg.WatcherPass(); err != nil {
			passErrors = append(passErrors, err)
		} else if err := requireSuccessfulWatcherCensus(supervisionDir, generation); err != nil {
			passErrors = append(passErrors, err)
		}
		// The run pass (monitor facility, MON-06/07): assess every
		// non-terminal run record, then attest the pass — the attestation
		// is written ONLY when every assessment succeeded, and it names
		// this watcher's identity plus the full lifecycle triples it
		// scanned, so a one-shot invocation can never impersonate the
		// standing watcher and a reused id can never be blessed unseen.
		if err := runPass(repo, self); err != nil {
			passErrors = append(passErrors, err)
		}
		repair, err := steward.RepairEnrolledRunner(repo)
		if err != nil {
			passErrors = append(passErrors, fmt.Errorf("repair enrolled steward: %w", err))
		}
		if len(passErrors) > 0 {
			joined := errors.Join(passErrors...)
			_, _ = steward.CompleteComponentAttempt(repo, "repo-watcher", generation, attempt.AttemptSeq,
				steward.ComponentError, "PASS_FAILED", joined.Error(), time.Now())
			return joined
		}
		evidence := fmt.Sprintf("census, run assessment, and steward check completed (%s)", repair.Status)
		if _, err := steward.CompleteComponentAttempt(repo, "repo-watcher", generation, attempt.AttemptSeq,
			steward.ComponentOK, "PASS_COMPLETE", evidence, time.Now()); err != nil {
			return fmt.Errorf("record watcher completion: %w", err)
		}
		return nil
	}
	return lock.Release, pass, true
}

func requireSuccessfulWatcherCensus(supervisionDir string, generation int) error {
	data, err := os.ReadFile(filepath.Join(supervisionDir, "last-census.json"))
	if err != nil {
		return fmt.Errorf("read watcher census completion: %w", err)
	}
	var verdict struct {
		Verdict    string `json:"verdict"`
		Generation *int64 `json:"generation"`
	}
	if err := json.Unmarshal(data, &verdict); err != nil {
		return fmt.Errorf("read watcher census completion: %w", err)
	}
	if verdict.Verdict != "SUCCESS" {
		return fmt.Errorf("watcher census completed with verdict %s", verdict.Verdict)
	}
	if verdict.Generation == nil || *verdict.Generation != int64(generation) {
		return fmt.Errorf("watcher census does not belong to supervision generation %d", generation)
	}
	return nil
}

// runPass assesses runs and writes the attestation on full success.
func runPass(repo string, self identity.Ref) error {
	store := &run.Store{Root: repo}
	records, unreadable := store.List()
	type scanned struct {
		Id          string `json:"id"`
		Generation  int    `json:"generation"`
		LaunchNonce string `json:"launchNonce"`
	}
	var scannedRuns []scanned
	clean := len(unreadable) == 0
	for _, record := range records {
		if run.Terminal(record.Status) {
			continue
		}
		if _, err := store.Assess(record.RunId); err != nil {
			fmt.Fprintln(os.Stderr, "supervise component watcher run pass:", err)
			clean = false
			continue
		}
		scannedRuns = append(scannedRuns, scanned{record.RunId, record.Generation, record.LaunchNonce})
	}
	for _, line := range unreadable {
		fmt.Fprintln(os.Stderr, "supervise component watcher run pass:", line)
	}
	if !clean {
		return fmt.Errorf("run assessment did not complete")
	}
	if scannedRuns == nil {
		scannedRuns = []scanned{}
	}
	attestation := map[string]any{
		"completedAt":  time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"watcherPid":   self.Pid,
		"watcherStart": self.StartedAtSec,
		"scannedRuns":  scannedRuns,
	}
	data, err := json.MarshalIndent(attestation, "", " ")
	if err != nil {
		return err
	}
	path := filepath.Join(supervise.SupervisionDir(repo), "runs-pass.json")
	durable, err := atomicfile.WriteText(path, string(append(data, '\n')), repo)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("run assessment attestation was published with durability unknown")
	}
	return nil
}

// setupReaper returns the per-interval job sweep, proving custody liveness
// against the live kernel. Verdicts land through the locked job-record
// compare-and-swap owner: a completion that arrives after the sweep's read
// wins, and the stale verdict is void.
func setupReaper(repo string) func() {
	cfg := supervise.ReaperConfig{
		Repo:      repo,
		JobsDir:   supervise.JobsDir(repo),
		Now:       func() time.Time { return time.Now().UTC() },
		Custodian: kernelCustodian(repo),
		Apply:     recordCASApplier(repo),
		Emit:      func(line string) { fmt.Fprintln(os.Stderr, line) },
	}
	return func() {
		if err := cfg.ReaperPass(); err != nil {
			fmt.Fprintln(os.Stderr, "supervise component reaper:", err)
		}
	}
}

// kernelCustodian binds the shared kernel custodian discipline
// (identity.Custodian) as the reaper's custody prover: one implementation,
// so the standing reaper and the mission runner's drain reap can never
// disagree about one record's custodian. The fixture authority is
// root-checked; a refused construction refuses fixtures.
func kernelCustodian(repo string) func(pid, start int64, tag string) identity.Liveness {
	authorization, err := fixtureauth.New(repo)
	if err != nil {
		// A leaked fixture makes every custody verdict Unknown — which
		// authorizes nothing.
		return func(int64, int64, string) identity.Liveness { return identity.Unknown }
	}
	return func(pid, start int64, tag string) identity.Liveness {
		return identity.Custodian(pid, start, tag, authorization.Identity())
	}
}

// recordCASApplier binds the dispatch record owner as the reaper's verdict
// applier: the patch lands only if the record still carries the expected
// status. A lost compare (the record moved on, e.g. a completion beat the
// verdict) reports applied=false with no error — exactly the void-verdict
// contract the reaper documents. The wiring lives here because dispatch
// imports supervise, so supervise cannot import dispatch back.
func recordCASApplier(repo string) func(job, expect, target string, patch map[string]any) (bool, error) {
	return func(job, expect, target string, patch map[string]any) (bool, error) {
		encoded, err := json.Marshal(patch)
		if err != nil {
			return false, err
		}
		// The suffix must NOT be .json: this temp file lives in the
		// jobs directory, and a concurrent classification strict-reads
		// every jobs/*.json — a half-written patch would refuse the
		// whole classification as a corrupt job record.
		patchFile, err := os.CreateTemp(supervise.JobsDir(repo), "reap-patch-*.tmp")
		if err != nil {
			return false, err
		}
		defer os.Remove(patchFile.Name())
		if _, err := patchFile.Write(encoded); err != nil {
			patchFile.Close()
			return false, err
		}
		if err := patchFile.Close(); err != nil {
			return false, err
		}
		observed, err := dispatchpkg.RecordCAS(repo, job, expect, target, patchFile.Name())
		if observed != "" {
			return false, nil // lost compare: the record moved on, verdict void
		}
		if err != nil {
			return false, err
		}
		return true, nil
	}
}
