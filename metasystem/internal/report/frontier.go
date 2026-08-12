package report

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The measured-improvement frontier: record stores the best-known committed
// state for an improvement goal; challenge admits only candidates beating
// the frontier by more than the persisted noise floor; status prints the
// file with the effective direction. The noise floor, direction, and
// measurement window persist in the file so a forgotten flag can never
// silently reinterpret a recorded frontier.

var frontierNumericRe = regexp.MustCompile(`^-?[0-9]+([.][0-9]+)?$`)

// FrontierError carries the shell contract's exit code beside the message.
type FrontierError struct {
	Code    int
	Message string
}

func (e *FrontierError) Error() string { return e.Message }

func frontierFail(code int, format string, args ...any) *FrontierError {
	return &FrontierError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// FrontierOptions carries one invocation's inputs after flag parsing; empty
// strings mean the flag was absent so the env/stored/default ladder applies.
type FrontierOptions struct {
	File      string
	Score     string
	Eval      string
	Artifact  string
	MinDelta  string
	MaxAge    string
	Direction string
	Force     bool
	Env       func(string) string // environment lookup, injectable for tests
	Now       func() time.Time
	Repo      string // git working directory for record's checks
}

func (o *FrontierOptions) env(key string) string {
	if o.Env == nil {
		return os.Getenv(key)
	}
	return o.Env(key)
}

func (o *FrontierOptions) now() time.Time {
	if o.Now == nil {
		return time.Now()
	}
	return o.Now()
}

func frontierReadFields(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fields := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		if _, seen := fields[key]; !seen {
			fields[key] = value
		}
	}
	return fields, nil
}

func (o *FrontierOptions) resolveMinDelta(stored string) (float64, *FrontierError) {
	candidate := o.MinDelta
	if candidate == "" {
		candidate = o.env("METASYSTEM_FRONTIER_MIN_DELTA")
	}
	if candidate == "" {
		candidate = stored
	}
	if candidate == "" {
		candidate = "0"
	}
	if !frontierNumericRe.MatchString(candidate) {
		return 0, frontierFail(2, "invalid noise floor: %s", candidate)
	}
	value, _ := strconv.ParseFloat(candidate, 64)
	return value, nil
}

func (o *FrontierOptions) resolveWindow(stored string) (string, *FrontierError) {
	candidate := o.MaxAge
	if candidate == "" {
		candidate = o.env("METASYSTEM_FRONTIER_MAX_AGE_MINUTES")
	}
	if candidate == "" {
		candidate = stored
	}
	if candidate != "" && !regexp.MustCompile(`^[0-9]+$`).MatchString(candidate) {
		return "", frontierFail(2, "invalid measurement window: %s", candidate)
	}
	return candidate, nil
}

func (o *FrontierOptions) resolveDirection(stored string) (string, *FrontierError) {
	candidate := o.Direction
	if candidate == "" {
		candidate = o.env("METASYSTEM_FRONTIER_DIRECTION")
	}
	if candidate == "" {
		candidate = stored
	}
	if candidate == "" {
		candidate = "max"
	}
	if candidate != "max" && candidate != "min" {
		return "", frontierFail(2, "invalid direction: %s (max or min)", candidate)
	}
	return candidate, nil
}

func (o *FrontierOptions) frontierExpired(window string, fields map[string]string, file string) (bool, *FrontierError) {
	if window == "" {
		return false, nil
	}
	epoch := fields["recorded_epoch"]
	if !regexp.MustCompile(`^[0-9]+$`).MatchString(epoch) {
		return false, frontierFail(2, "frontier file is malformed: %s", file)
	}
	recorded, _ := strconv.ParseInt(epoch, 10, 64)
	ageMinutes := (o.now().UTC().Unix() - recorded) / 60
	limit, _ := strconv.ParseInt(window, 10, 64)
	return ageMinutes > limit, nil
}

func frontierBeats(candidate, incumbent, delta float64, direction string) bool {
	if direction == "min" {
		return candidate < incumbent-delta
	}
	return candidate > incumbent+delta
}

func frontierGit(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// FrontierRecord implements `frontier record`. Returned lines print to
// stdout; a *FrontierError carries the exit code and stderr text.
func FrontierRecord(opts FrontierOptions) ([]string, *FrontierError) {
	if !frontierNumericRe.MatchString(opts.Score) {
		return nil, frontierFail(2, "record requires a numeric --score")
	}
	if opts.Eval == "" {
		return nil, frontierFail(2, "record requires --eval with the evaluation command that produced the score")
	}
	if _, err := frontierGit(opts.Repo, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, frontierFail(2, "not inside a git repository")
	}
	dirty, err := frontierGit(opts.Repo, "status", "--porcelain")
	if err != nil || dirty != "" {
		return nil, frontierFail(1, "worktree is dirty; a frontier must be an exact committed state")
	}
	var storedDelta, storedWindow, storedDirection string
	fields := map[string]string{}
	fileExists := false
	if read, readErr := frontierReadFields(opts.File); readErr == nil {
		fields, fileExists = read, true
		storedDelta = fields["min_delta"]
		storedWindow = fields["max_age_minutes"]
		storedDirection = fields["direction"]
	}
	minDelta, ferr := opts.resolveMinDelta(storedDelta)
	if ferr != nil {
		return nil, ferr
	}
	window, ferr := opts.resolveWindow(storedWindow)
	if ferr != nil {
		return nil, ferr
	}
	direction, ferr := opts.resolveDirection(storedDirection)
	if ferr != nil {
		return nil, ferr
	}
	score, _ := strconv.ParseFloat(opts.Score, 64)
	if fileExists && !opts.Force {
		effectiveStored := storedDirection
		if effectiveStored == "" {
			effectiveStored = "max"
		}
		if direction != effectiveStored {
			return nil, frontierFail(1, "direction %s differs from the recorded frontier's %s; a direction change re-baselines the frontier, so use --force and record the reason in the owning plan", direction, effectiveStored)
		}
		expired, ferr := opts.frontierExpired(window, fields, opts.File)
		if ferr != nil {
			return nil, ferr
		}
		if expired {
			return nil, frontierFail(1, "recorded frontier is older than its measurement window; the environment may have shifted. Re-baseline with --force and record the reason in the owning plan")
		}
		old := fields["score"]
		if !frontierNumericRe.MatchString(old) {
			return nil, frontierFail(2, "frontier file is malformed: %s", opts.File)
		}
		oldScore, _ := strconv.ParseFloat(old, 64)
		if !frontierBeats(score, oldScore, minDelta, direction) {
			return nil, frontierFail(1, "score %s does not beat the recorded frontier %s by more than %s (direction %s); use challenge first, or --force only to re-baseline after an evaluation change",
				opts.Score, old, formatFloat(minDelta), direction)
		}
	}
	sha, err := frontierGit(opts.Repo, "rev-parse", "HEAD")
	if err != nil {
		return nil, frontierFail(2, "not inside a git repository")
	}
	shortSHA, _ := frontierGit(opts.Repo, "rev-parse", "--short", "HEAD")
	if err := os.MkdirAll(filepath.Dir(opts.File), 0o755); err != nil {
		return nil, frontierFail(2, "cannot create frontier directory: %v", err)
	}
	content := fmt.Sprintf("sha=%s\nrecorded_epoch=%d\nscore=%s\nmin_delta=%s\ndirection=%s\nmax_age_minutes=%s\neval=%s\nartifact=%s\n",
		sha, opts.now().UTC().Unix(), opts.Score, formatFloat(minDelta), direction, window, opts.Eval, opts.Artifact)
	if err := os.WriteFile(opts.File, []byte(content), 0o644); err != nil {
		return nil, frontierFail(2, "cannot write frontier file: %v", err)
	}
	return []string{
		fmt.Sprintf("frontier recorded: score %s at %s", opts.Score, shortSHA),
		fmt.Sprintf("commit %s with the frontier checkpoint", opts.File),
	}, nil
}

// FrontierChallenge implements `frontier challenge`.
func FrontierChallenge(opts FrontierOptions) ([]string, *FrontierError) {
	if !frontierNumericRe.MatchString(opts.Score) {
		return nil, frontierFail(2, "challenge requires a numeric --score")
	}
	if opts.Direction != "" {
		return nil, frontierFail(2, "challenge uses only the persisted direction; change it with record --force, never at comparison time")
	}
	fields, err := frontierReadFields(opts.File)
	if err != nil {
		return nil, frontierFail(1, "no frontier recorded at %s; record the baseline first", opts.File)
	}
	old := fields["score"]
	if !frontierNumericRe.MatchString(old) {
		return nil, frontierFail(2, "frontier file is malformed: %s", opts.File)
	}
	window, ferr := opts.resolveWindow(fields["max_age_minutes"])
	if ferr != nil {
		return nil, ferr
	}
	expired, ferr := opts.frontierExpired(window, fields, opts.File)
	if ferr != nil {
		return nil, ferr
	}
	if expired {
		return nil, frontierFail(1, "frontier expired: recorded score %s is outside its measurement window; the environment may have shifted. Re-baseline with record --force before comparing candidates", old)
	}
	minDelta, ferr := opts.resolveMinDelta(fields["min_delta"])
	if ferr != nil {
		return nil, ferr
	}
	direction := "max"
	if stored, present := fields["direction"]; present {
		if stored != "max" && stored != "min" {
			return nil, frontierFail(2, "frontier file has a malformed direction: '%s'", stored)
		}
		direction = stored
	}
	score, _ := strconv.ParseFloat(opts.Score, 64)
	oldScore, _ := strconv.ParseFloat(old, 64)
	if frontierBeats(score, oldScore, minDelta, direction) {
		return []string{fmt.Sprintf("new frontier: %s beats %s by more than %s (direction %s); preserve this exact state before iterating",
			opts.Score, old, formatFloat(minDelta), direction)}, nil
	}
	if frontierBeats(score, oldScore, 0, direction) {
		return nil, frontierFail(1, "within noise floor: %s improves on %s but not by more than %s; treat as noise",
			opts.Score, old, formatFloat(minDelta))
	}
	return nil, frontierFail(1, "does not beat frontier: %s against %s (direction %s)", opts.Score, old, direction)
}

// FrontierStatus implements `frontier status`.
func FrontierStatus(opts FrontierOptions) ([]string, *FrontierError) {
	data, err := os.ReadFile(opts.File)
	if err != nil {
		return []string{fmt.Sprintf("no frontier recorded at %s", opts.File)}, nil
	}
	lines := []string{strings.TrimRight(string(data), "\n")}
	if !regexp.MustCompile(`(?m)^direction=.`).Match(data) {
		lines = append(lines, "direction=max")
	}
	return lines, nil
}

// formatFloat prints a float the way the shell echoed its inputs: no
// trailing zeros, no scientific notation for ordinary magnitudes.
func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
