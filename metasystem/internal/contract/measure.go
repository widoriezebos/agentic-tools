package contract

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// Measuring a candidate is the per-cycle reading the mission runner records: run
// the contract's gate over the frozen instruments to read each declared metric,
// classify those metrics against the prior cycle's reading under the contract's
// direction and per-metric noise floors, and run each guard against its floor.
// The verdict feeds the ledger and decides whether a cycle completed the gate.

// MeasureResult is one cycle's reading of a candidate: the gate metrics and how
// they moved relative to the prior measurement, the guard readings, the
// candidate commit measured, and whether the gate as a whole passed.
type MeasureResult struct {
	Classification string            `json:"classification"`
	Observed       string            `json:"observed"`
	CandidateSHA   string            `json:"candidateSha"`
	Metrics        map[string]string `json:"metrics"`
	Guards         map[string]string `json:"guards"`
	GatePassed     bool              `json:"gatePassed"`
}

// Measure runs a contract's gate and guards against the current
// candidate and classifies the gate metrics against a prior measurement. The
// previous map supplies the per-metric values a regression is judged against;
// when it is nil the sealed baseline is used, so the first cycle measures
// against the price the seal froze. The gate passes when it clears every
// threshold and every guard clears its floor.
func Measure(path string, previous map[string]string) (*MeasureResult, error) {
	doc, repo, projectRoot, err := contractLoad(path)
	if err != nil {
		return nil, err
	}
	return doc.measure(repo, projectRoot, previous)
}

func (d *contractDoc) measure(repo, projectRoot string, previous map[string]string) (*MeasureResult, error) {
	// The pins are resolved exactly ONCE per measurement: with concurrent
	// jobs allowed to land commits on the mission branch mid-cycle, a
	// second resolve could hand the guards a different commit than the
	// gate measured, and the ledger would record one CandidateSHA over
	// mixed readings — evidence that never existed on any single commit.
	candidateSHA, gateRef, err := d.resolvePins(repo)
	if err != nil {
		return nil, err
	}
	metrics, failures, err := d.runGate(repo, projectRoot, candidateSHA, gateRef)
	if err != nil {
		return nil, err
	}
	names := d.thresholdNames()
	prior, err := d.priorMetrics(previous, names)
	if err != nil {
		return nil, err
	}
	classification, err := d.classify(metrics, prior, names)
	if err != nil {
		return nil, err
	}
	guards, guardsPassed, err := d.runGuards(repo, projectRoot, candidateSHA, gateRef)
	if err != nil {
		return nil, err
	}
	observed := make([]string, len(names))
	for i, name := range names {
		observed[i] = name + "=" + metrics[name]
	}
	return &MeasureResult{
		Classification: classification,
		Observed:       strings.Join(observed, ","),
		CandidateSHA:   candidateSHA,
		Metrics:        metrics,
		Guards:         guards,
		GatePassed:     failures == 0 && guardsPassed,
	}, nil
}

// priorMetrics resolves the baseline each metric's movement is judged against:
// the caller-supplied prior when present, otherwise the sealed baseline.
func (d *contractDoc) priorMetrics(previous map[string]string, names []string) (map[string]float64, error) {
	prior := make(map[string]float64, len(names))
	for _, name := range names {
		raw, ok := previous[name]
		if !ok {
			raw, ok = d.sealed["sealed.baseline."+name]
			if !ok {
				return nil, stateErr("no prior measurement or sealed baseline for metric: %s", name)
			}
		}
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, stateErr("prior metric is not numeric: %s=%s", name, raw)
		}
		prior[name] = value
	}
	return prior, nil
}

// classify folds every declared metric's movement into one verdict. A metric
// counts as moved only past its noise floor, in the contract's improving
// direction. An unambiguous improvement is contract-improved; movement within
// the noise on every metric is unresolved; anything else — a regression, or a
// mix of gains and losses — is no-progress.
func (d *contractDoc) classify(metrics map[string]string, prior map[string]float64, names []string) (string, error) {
	direction := d.values["gate.direction"]
	improved, regressed, within := false, false, true
	for _, name := range names {
		current, err := strconv.ParseFloat(metrics[name], 64)
		if err != nil {
			return "", stateErr("gate metric is not numeric: %s=%s", name, metrics[name])
		}
		noise, err := strconv.ParseFloat(d.values["gate.noise-floor."+name], 64)
		if err != nil {
			return "", stateErr("gate noise floor is not numeric: %s", name)
		}
		directed := current - prior[name]
		if direction == "min" {
			directed = -directed
		}
		switch {
		case directed > noise:
			improved, within = true, false
		case directed < -noise:
			regressed, within = true, false
		}
	}
	switch {
	case improved && !regressed:
		return "contract-improved", nil
	case within:
		return "unresolved", nil
	default:
		return "no-progress", nil
	}
}

// runGuards measures each declared guard against the same restored candidate the
// gate saw and reports whether every guard cleared its floor. A guard whose
// command fails, times out, or omits its own metric is a measurement error, not
// a soft failure.
func (d *contractDoc) runGuards(repo, projectRoot, candidateSHA, gateRef string) (map[string]string, bool, error) {
	names := d.guardNames()
	guards := make(map[string]string, len(names))
	if len(names) == 0 {
		return guards, true, nil
	}
	// The guards get their OWN clean worktree at the same pinned SHAs the
	// gate measured. A shared live worktree is not equivalent: gate and
	// guard commands are arbitrary bash with cwd inside the worktree, so
	// the gate could mutate what the guards read and defeat the
	// frozen-instruments restore.
	commandRoot, cleanup, err := d.materializeCandidate(repo, projectRoot, candidateSHA, gateRef)
	if err != nil {
		return nil, false, err
	}
	defer cleanup()

	capMin, _ := strconv.Atoi(d.values["fence.job-cap-min"])
	passed := true
	for _, name := range names {
		metrics, code, timedOut, err := measureCommand(commandRoot, d.values["guard."+name+".command"], capMin)
		if err != nil {
			return nil, false, err
		}
		if timedOut {
			return nil, false, stateErr("guard %s measurement exceeded named fence.job-cap-min ceiling (%sm)", name, d.values["fence.job-cap-min"])
		}
		if code != 0 {
			return nil, false, stateErr("guard %s measurement failed with exit %d", name, code)
		}
		value, ok := metrics[name]
		if !ok {
			return nil, false, stateErr("guard %s output omitted metric=%s=<value>", name, name)
		}
		measured, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, false, stateErr("guard %s metric is not numeric: %s", name, value)
		}
		floor, err := strconv.ParseFloat(d.values["guard."+name+".floor"], 64)
		if err != nil {
			return nil, false, stateErr("guard %s floor is not numeric", name)
		}
		guards[name] = value
		if measured < floor {
			passed = false
		}
	}
	return guards, passed, nil
}

// guardNames returns the sorted names of every declared guard.
func (d *contractDoc) guardNames() []string {
	var names []string
	for key := range d.values {
		if m := contractGuardRe.FindStringSubmatch(key); m != nil && m[2] == "command" {
			names = append(names, m[1])
		}
	}
	sort.Strings(names)
	return names
}

// resolvePins reads the two commits one measurement is pinned to: the
// candidate at the tip of the sealed branch, and the frozen-instruments
// gate ref. Every worktree of the measurement materializes at these pins.
func (d *contractDoc) resolvePins(repo string) (candidateSHA, gateRef string, err error) {
	gateRef, err = contractGitTrim(repo, "rev-parse", d.values["gate.ref"]+"^{commit}")
	if err != nil {
		return "", "", err
	}
	branch, err := d.candidateBranch(repo)
	if err != nil {
		return "", "", err
	}
	candidateSHA, err = contractGitTrim(repo, "rev-parse", branch+"^{commit}")
	if err != nil {
		return "", "", err
	}
	return candidateSHA, gateRef, nil
}

// materializeCandidate builds a throwaway worktree at the pinned candidate
// with the gate and truth paths restored to their pinned gate-ref bytes —
// the reproducible state every measurement command runs in — and returns
// the directory to run in together with a cleanup to remove the worktree.
func (d *contractDoc) materializeCandidate(repo, projectRoot, candidateSHA, gateRef string) (string, func(), error) {
	restored, err := d.restoredPaths(repo, projectRoot, gateRef)
	if err != nil {
		return "", nil, err
	}
	scratch, err := os.MkdirTemp("", "mission-candidate.")
	if err != nil {
		return "", nil, err
	}
	worktree := filepath.Join(scratch, "candidate")
	cleanup := func() {
		gitTry(repo, "worktree", "remove", "--force", worktree)
		os.RemoveAll(scratch)
	}
	// The registry entry lands BEFORE the worktree exists: the wall's
	// worktree census admits only runner-recorded worktrees, and a crash
	// mid-measurement must leave a recorded artifact, never a private
	// carrier. FAIL CLOSED: an unrecordable worktree never materializes.
	if err := recordMeasureWorktree(projectRoot, worktree, candidateSHA, gateRef); err != nil {
		os.RemoveAll(scratch)
		return "", nil, stateErr("measurement cannot record its worktree: %v", err)
	}
	if _, err := gitOutput(repo, "worktree", "add", "--detach", "--quiet", worktree, candidateSHA); err != nil {
		cleanup()
		return "", nil, err
	}
	if _, err := gitOutput(worktree, append([]string{"checkout", "--quiet", gateRef, "--"}, restored...)...); err != nil {
		cleanup()
		return "", nil, err
	}
	rel, err := filepath.Rel(resolvePath(repo), resolvePath(projectRoot))
	if err != nil {
		cleanup()
		return "", nil, stateErr("metasystem project root is outside its git repository")
	}
	commandRoot := worktree
	if rel != "." {
		commandRoot = filepath.Join(worktree, rel)
	}
	return commandRoot, cleanup, nil
}

// recordMeasureWorktree registers a measurement worktree in the
// runner's registry (artifacts/agents/measure-worktrees.jsonl under the
// project root — the same file the wall's census reads), pruning
// entries whose worktrees are gone. Best-effort: measurement never
// fails over its own bookkeeping.
func recordMeasureWorktree(projectRoot, path, sha, gateRef string) error {
	dir := filepath.Join(projectRoot, "artifacts", "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	registry := filepath.Join(dir, "measure-worktrees.jsonl")
	// The read-prune-rewrite is SERIALIZED under an exclusive flock and
	// published atomically (WSS I11-11): two concurrent missions can
	// otherwise clobber each other's records, and a pending record — a
	// path registered fail-closed BEFORE its worktree exists — must
	// never be pruned by a peer in that window, so absence prunes only
	// past a grace no measurement setup can outlive.
	lock, err := os.OpenFile(registry+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	kept := []string{}
	if data, err := os.ReadFile(registry); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var doc map[string]any
			if json.Unmarshal([]byte(line), &doc) != nil {
				continue
			}
			prior, _ := doc["path"].(string)
			if _, err := os.Stat(prior); err == nil {
				kept = append(kept, line)
				continue
			}
			if recorded, ok := doc["recordedAt"].(string); ok {
				if at, perr := time.Parse(time.RFC3339, recorded); perr == nil && time.Since(at) < time.Hour {
					kept = append(kept, line)
				}
			}
		}
	}
	entry, err := json.Marshal(map[string]string{
		"path": path, "sha": sha, "gateRef": gateRef,
		"recordedAt": time.Now().UTC().Format(time.RFC3339)})
	if err != nil {
		return err
	}
	kept = append(kept, string(entry))
	tmp, err := os.CreateTemp(dir, "measure-worktrees.")
	if err != nil {
		return err
	}
	if _, err := tmp.WriteString(strings.Join(kept, "\n") + "\n"); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), registry)
}

// measureCommand runs one measurement command under the named per-job ceiling
// and returns the metric values it emitted, its exit code, and whether it hit
// the ceiling. Only a failure to run the command at all is returned as an error;
// a nonzero exit or a timeout is reported for the caller to interpret.
func measureCommand(commandRoot, command string, capMinutes int) (map[string]string, int, bool, error) {
	// --noprofile --norc: a login shell would read /etc/profile and the
	// user profile, either of which could re-export GIT_DIR, GIT_CONFIG*,
	// or replacement steering AFTER the scrub and before child git runs.
	cmd := exec.Command("bash", "--noprofile", "--norc", "-c", command)
	cmd.Dir = commandRoot
	// Gate and guard commands are arbitrary bash: the repository-steering
	// environment is stripped and any git THEY spawn inherits the
	// replacement pin through git's environment-config channel, so a
	// measurement can never read a foreign or replacement view while the
	// wall judged the real checkout.
	cmd.Env = gittree.ScrubbedEnviron(
		"GIT_CONFIG_COUNT=3",
		"GIT_CONFIG_KEY_0=core.useReplaceRefs", "GIT_CONFIG_VALUE_0=false",
		"GIT_CONFIG_KEY_1=gc.auto", "GIT_CONFIG_VALUE_1=0",
		"GIT_CONFIG_KEY_2=maintenance.auto", "GIT_CONFIG_VALUE_2=false")
	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output
	// Bounded with group-kill (B4), for the same reason as the contract
	// gate command: a context deadline alone leaves grandchildren holding
	// the output pipe past the ceiling.
	runErr := boundedexec.Run(cmd,
		boundedexec.FixedBound(time.Duration(capMinutes)*time.Minute, "fence.job-cap-min"),
		"measurement command")
	if errors.Is(runErr, boundedexec.ErrTimedOut) {
		return nil, 0, true, nil
	}
	code := 0
	if runErr != nil {
		code = 1
		var exit *exec.ExitError
		if errors.As(runErr, &exit) {
			code = exit.ExitCode()
		}
	}
	metrics := map[string]string{}
	for _, line := range strings.Split(output.String(), "\n") {
		if m := contractMetricRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			metrics[m[1]] = m[2]
		}
	}
	return metrics, code, false, nil
}

// MeasureCandidate runs the declared gate against an EXPLICIT commit —
// the mission's active candidate branch (issue #4, ledgerSemantics 3) —
// through the same materialization, cap ceiling, and declared-metric
// validation as the gate of record. It returns the reported metrics
// alone: classification, guards, and the best fold belong to the caller's
// gate-of-record measurement.
func MeasureCandidate(path, sha string) (map[string]string, error) {
	doc, repo, projectRoot, err := contractLoad(path)
	if err != nil {
		return nil, err
	}
	_, gateRef, err := doc.resolvePins(repo)
	if err != nil {
		return nil, err
	}
	metrics, _, err := doc.runGate(repo, projectRoot, sha, gateRef)
	return metrics, err
}
