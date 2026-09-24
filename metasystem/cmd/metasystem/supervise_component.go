package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	dispatchpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
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
func runSuperviseComponent(args []string) (code int) {
	flags := flag.NewFlagSet("supervise component", flag.ContinueOnError)
	component := flags.String("component", "", "watcher | reaper | landing-owner")
	repo := pathFlag(flags, "repo", "", "checkout root the component operates on")
	metasystemRoot := flags.String("metasystem-root", "", "installation root containing config and runtime adapters")
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
	ignoreTerm := flags.Bool("ignore-term", false, "ignore TERM (fixture-only)")
	slowStop := flags.Int("slow-stop", 0, "delay orderly signal exit by this many seconds (fixture-only)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *component == "" || *tag == "" || *heartbeat == "" {
		fmt.Fprintln(os.Stderr, "supervise component: --component, --tag, --heartbeat required")
		return 2
	}
	if *component == "watcher" || *component == "reaper" || *component == "landing-owner" {
		if *repo == "" {
			fmt.Fprintln(os.Stderr, "supervise component: --repo is required for the "+*component)
			return 2
		}
		if *generation < 1 {
			fmt.Fprintln(os.Stderr, "supervise component: --generation is required for the "+*component)
			return 2
		}
	}
	if *scope == "" {
		*scope = *repo
	}
	if *metasystemRoot == "" {
		*metasystemRoot = *repo
	}
	if *slowStop < 0 {
		fmt.Fprintln(os.Stderr, "supervise component: --slow-stop must be non-negative")
		return 2
	}
	if (*crashOnStart || *ignoreTerm || *slowStop > 0) && !fixtureauth.FixtureModeRoot(*metasystemRoot) {
		fmt.Fprintln(os.Stderr, "supervise component: crash and signal-control flags are fixture-only")
		return 1
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
	wake := make(chan os.Signal, 1)
	signal.Notify(wake, syscall.SIGUSR1)
	defer signal.Stop(wake)
	if *ignoreTerm {
		signal.Ignore(syscall.SIGTERM)
		signal.Notify(stop, syscall.SIGINT)
	} else {
		signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	}

	if *capMin < 1 {
		*capMin = *intervalSec
	}
	beat := heartbeatWriter(*heartbeat, *component, self, *tag, *intervalSec, *capMin)

	var work func() error
	switch *component {
	case "watcher":
		release, pass, ok := setupWatcher(*metasystemRoot, *repo, *scope, self, *tag, *generation, *intervalSec)
		if !ok {
			return 1
		}
		defer release()
		work = pass
	case "reaper":
		reaperPass := setupReaper(*repo, *metasystemRoot)
		work = func() error {
			reaperPass()
			return nil
		}
	case "landing-owner":
		release, pass, ok := setupLandingOwner(*metasystemRoot, landingOwnerCheckoutRoot(*repo, *scope))
		if !ok {
			return 1
		}
		defer func() {
			if releaseCode := reportLandingOwnerRelease(release); releaseCode != 0 {
				code = 1
			}
		}()
		work = landingOwnerReportedPass(*repo, pass)
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
			if *slowStop > 0 {
				time.Sleep(time.Duration(*slowStop) * time.Second)
			}
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
		case <-wake:
			beat()
			if err := work(); err != nil {
				fmt.Fprintln(os.Stderr, "supervise component:", err)
			}
		}
	}
}

// landingOwnerCheckoutRoot picks the checkout root the batch landing owner
// operates on. On a repository that nests the module the steward launches every
// component with --repo naming the installation (…/checkout/metasystem) and
// --scope naming the git toplevel (…/checkout). The batch landing root is the
// toplevel: config.ResolveBatchLanding refuses any value whose
// `git rev-parse --show-toplevel` is not itself, and the batch store, the
// control root and the owner inputs are all derived from the toplevel by
// batch.ModuleRoot. Comparing the configured root against the installation
// instead made the owner's activation test fail on every tick, silently, so
// batches were joined and never sealed. --scope defaults to --repo, so a flat
// checkout is unchanged.
func landingOwnerCheckoutRoot(repo, scope string) string {
	if scope == "" {
		return repo
	}
	return scope
}

func setupLandingOwner(metasystemRoot, repo string) (release func() error, pass func() error, ok bool) {
	return setupLandingOwnerWithCadence(metasystemRoot, repo, newBatchOwnerCadence())
}

func setupLandingOwnerWithCadence(metasystemRoot, repo string, cadence *batchOwnerCadence) (release func() error, pass func() error, ok bool) {
	return setupLandingOwnerWithInputs(metasystemRoot, repo, cadence, resolveProductionBatchOwnerInputs)
}

func setupLandingOwnerWithInputs(metasystemRoot, repo string, cadence *batchOwnerCadence, resolveInputs func(string) (productionBatchOwnerInputs, error)) (release func() error, pass func() error, ok bool) {
	var activePass func() error
	var held *batchOwnerLease
	var announced *batchOwnerLease
	var settings config.BatchLanding
	var inputs productionBatchOwnerInputs
	clock := cadenceProductionClock
	release = func() error {
		cadence.stop()
		if announced != nil {
			return announced.retire()
		}
		return nil
	}
	pass = func() error {
		if activePass != nil {
			return activePass()
		}
		conf := filepath.Join(metasystemRoot, "metasystem.conf")
		rawRoot, _, err := config.Get(config.GetParams{Key: config.BatchRootKey, ConfPath: conf, Default: "", DefaultSet: true})
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if rawRoot == "" {
			return nil
		}
		resolved, err := resolvePathFlag(rawRoot)
		if err != nil {
			return err
		}
		if resolved != filepath.Clean(repo) {
			return nil
		}
		rawWait, _, err := config.Get(config.GetParams{Key: config.BatchMaxWaitKey, ConfPath: conf,
			Default: config.DefaultBatchMaxWait.String(), DefaultSet: true})
		if err != nil {
			return err
		}
		wait, err := time.ParseDuration(strings.TrimSpace(rawWait))
		if err != nil {
			return fmt.Errorf("invalid batch wait: %w", err)
		}
		settings, err = config.NewBatchLanding(repo, wait, clock)
		if err != nil {
			return err
		}
		inputs, err = resolveInputs(repo)
		if err != nil {
			return err
		}
		if held == nil {
			acquired, err := acquireBatchOwnerForComponent(repo)
			if acquired.announced {
				announced = &acquired
			}
			if err != nil {
				return err
			}
			held = &acquired
			announced = held
		}
		owner, err := batchOwnerConstruct(settings, *held, inputs, clock)
		if err != nil {
			return err
		}
		activePass = func() error {
			if err := batchOwnerRequire(*held); err != nil {
				activePass = nil
				held = nil
				return err
			}
			runBatchOwnerPass(owner, *held, repo, clock, cadence)
			return nil
		}
		return activePass()
	}
	return release, pass, true
}

func landingOwnerErrorPath(repo string) string {
	return filepath.Join(supervise.SupervisionDir(repo), "landing-owner.last-error")
}

func writeLandingOwnerError(repo string, passErr error) (bool, error) {
	path := landingOwnerErrorPath(repo)
	if passErr == nil {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
		return false, nil
	}
	want := passErr.Error() + "\n"
	if current, err := os.ReadFile(path); err == nil && string(current) == want {
		return false, nil
	}
	durable, err := atomicfile.WriteText(path, want, repo)
	if err != nil {
		return false, err
	}
	if !durable {
		return true, fmt.Errorf("landing owner error record durability is unknown")
	}
	return true, nil
}

func landingOwnerReportedPass(repo string, pass func() error) func() error {
	return func() error {
		passErr := pass()
		_, recordErr := writeLandingOwnerError(repo, passErr)
		return errors.Join(passErr, recordErr)
	}
}

func reportLandingOwnerRelease(release func() error) int {
	if err := release(); err != nil {
		fmt.Fprintln(os.Stderr, "supervise component landing-owner release:", err)
		return 1
	}
	return 0
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
func setupWatcher(metasystemRoot, repo, scope string, self identity.Ref, tag string, generation, intervalSec int) (release func(), pass func() error, ok bool) {
	supervisionDir := supervise.SupervisionDir(repo)
	lock := &supervise.CensusWriterLock{
		Dir: supervisionDir, Self: self, Tag: tag, Prober: identity.KernelProber{},
	}
	if err := lock.Claim(); err != nil {
		fmt.Fprintln(os.Stderr, "supervise component watcher:", err)
		return nil, nil, false
	}

	cfg := watcherConfig(metasystemRoot, repo, scope, supervisionDir, intervalSec)
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
	store := dispatchpkg.NewConcludingRunStore(repo, nil)
	return runPassWithStore(repo, self, store)
}

func runPassWithStore(repo string, self identity.Ref, store *run.Store) error {
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
func setupReaper(repo, metasystemRoot string) func() {
	cfg := supervise.ReaperConfig{
		Repo:      repo,
		JobsDir:   supervise.JobsDir(repo),
		Now:       func() time.Time { return time.Now().UTC() },
		Custodian: kernelCustodian(metasystemRoot),
		ReturnComplete: func(role, file string) bool {
			return len(validate.ReturnCompleteRole(repo, role, file)) == 0
		},
		Apply: recordCASApplier(repo),
		Emit:  func(line string) { fmt.Fprintln(os.Stderr, line) },
	}
	return func() {
		if err := cfg.ReaperPass(); err != nil {
			fmt.Fprintln(os.Stderr, "supervise component reaper:", err)
		}
		// Proof attempts are reconciled on the same tick: a dead launcher
		// commits no terminal, and nothing else on the checkout read the
		// attempt records (goal hung-proof-attempts-end-at-their-deadline).
		if _, err := proofrun.ReconcileAttempts(metasystemRoot, proofrun.ReconcileOptions{
			Emit: func(line string) { fmt.Fprintln(os.Stderr, "supervise component reaper:", line) },
		}); err != nil {
			fmt.Fprintln(os.Stderr, "supervise component reaper: proof attempts:", err)
		}
	}
}

// kernelCustodian binds the shared kernel custodian discipline
// (identity.Custodian) as the reaper's custody prover: one implementation,
// so the standing reaper and the mission runner's drain reap can never
// disagree about one record's custodian. The fixture authority is
// root-checked; a refused construction refuses fixtures.
func kernelCustodian(metasystemRoot string) func(pid, start int64, tag string) identity.Liveness {
	authorization, err := fixtureauth.New(metasystemRoot)
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
