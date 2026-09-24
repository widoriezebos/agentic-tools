package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Resolution rules for the metasystem.conf settings file. A caller asks for one
// key and gets the single value the running system would actually use, chosen
// from the layered sources below.

var (
	confKeyPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`)
	modePattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	roleRuntimeKey  = regexp.MustCompile(`^role\.[a-z0-9-]+\.runtime$`)
	roleModelKey    = regexp.MustCompile(`^role\.[a-z0-9-]+\.model\.[a-z0-9-]+$`)
	digitsOnlyValue = regexp.MustCompile(`^[0-9]+$`)
)

const (
	BatchRootKey        = "landing.batch-root"
	BatchMaxWaitKey     = "landing.batch-max-wait"
	DefaultBatchMaxWait = 45 * time.Minute
)

type BatchLanding struct {
	Root    string
	MaxWait time.Duration
	now     func() time.Time
}

type gitRequest struct {
	Directory         string
	Args, Environment []string
}

type gitRunner func(gitRequest) ([]byte, error)

func runGit(request gitRequest) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", request.Directory}, request.Args...)...)
	command.Env = request.Environment
	return command.Output()
}

// NewBatchLanding binds already-resolved owner settings. It is used by the
// production supervisor, which runs inside the dedicated landing checkout and
// therefore cannot resolve that same checkout as if it were a seat.
func NewBatchLanding(root string, maxWait time.Duration, now func() time.Time) (BatchLanding, error) {
	if now == nil {
		return BatchLanding{}, fmt.Errorf("resolve batch landing: an injected clock is required")
	}
	if root == "" {
		return BatchLanding{}, fmt.Errorf("%s is required", BatchRootKey)
	}
	if maxWait < time.Minute || maxWait > 6*time.Hour {
		return BatchLanding{}, fmt.Errorf("%s must be a duration from 1m through 6h", BatchMaxWaitKey)
	}
	return BatchLanding{Root: resolvePath(root), MaxWait: maxWait, now: now}, nil
}

// ResolveExplicitBatchLanding validates an explicitly supplied landing root
// with the same existence and non-seat rules as configured resolution.
func ResolveExplicitBatchLanding(root, seatRoot string, maxWait time.Duration, now func() time.Time) (BatchLanding, error) {
	return resolveExplicitBatchLandingWithRunner(root, seatRoot, maxWait, now, runGit)
}

// ResolveExplicitBatchLandingWithRunner validates a landing checkout using a raw Git response source.
func ResolveExplicitBatchLandingWithRunner(root, seatRoot string, maxWait time.Duration, now func() time.Time, raw func(string, []string, []string) ([]byte, error)) (BatchLanding, error) {
	if raw == nil {
		return BatchLanding{}, fmt.Errorf("resolve %s: a Git runner is required", BatchRootKey)
	}
	return resolveExplicitBatchLandingWithRunner(root, seatRoot, maxWait, now, func(request gitRequest) ([]byte, error) {
		return raw(request.Directory, request.Args, request.Environment)
	})
}

func resolveExplicitBatchLandingWithRunner(root, seatRoot string, maxWait time.Duration, now func() time.Time, runner gitRunner) (BatchLanding, error) {
	validated, err := batchLandingRootWithRunner(root, seatRoot, runner)
	if err != nil {
		return BatchLanding{}, err
	}
	return NewBatchLanding(validated, maxWait, now)
}

func ResolveBatchLanding(confPath, seatRoot string, now func() time.Time) (BatchLanding, error) {
	return resolveBatchLandingWithRunner(confPath, seatRoot, now, runGit)
}

func resolveBatchLandingWithRunner(confPath, seatRoot string, now func() time.Time, runner gitRunner) (BatchLanding, error) {
	if now == nil {
		return BatchLanding{}, fmt.Errorf("resolve batch landing: an injected clock is required")
	}
	rawRoot, _, err := Get(GetParams{Key: BatchRootKey, ConfPath: confPath})
	if err != nil {
		return BatchLanding{}, fmt.Errorf("resolve %s: %w", BatchRootKey, err)
	}
	root, err := batchLandingRootWithRunner(rawRoot, seatRoot, runner)
	if err != nil {
		return BatchLanding{}, err
	}
	rawWait, _, err := Get(GetParams{Key: BatchMaxWaitKey, ConfPath: confPath, Default: DefaultBatchMaxWait.String(), DefaultSet: true})
	if err != nil {
		return BatchLanding{}, fmt.Errorf("resolve %s: %w", BatchMaxWaitKey, err)
	}
	wait, err := time.ParseDuration(rawWait)
	if err != nil || wait < time.Minute || wait > 6*time.Hour {
		return BatchLanding{}, fmt.Errorf("%s must be a duration from 1m through 6h, got %q", BatchMaxWaitKey, rawWait)
	}
	return BatchLanding{Root: root, MaxWait: wait, now: now}, nil
}

func batchLandingRootWithRunner(raw, seatRoot string, runner gitRunner) (string, error) {
	if !filepath.IsAbs(raw) {
		return "", fmt.Errorf("%s must be absolute, got %q", BatchRootKey, raw)
	}
	root := resolvePath(raw)
	if root == resolvePath(seatRoot) {
		return "", fmt.Errorf("%s must name a dedicated non-seat checkout", BatchRootKey)
	}
	if runner == nil {
		return "", fmt.Errorf("resolve %s: a Git runner is required", BatchRootKey)
	}
	top, err := runner(gitRequest{Directory: root, Args: []string{"rev-parse", "--show-toplevel"}, Environment: []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C"}})
	if err != nil || resolvePath(strings.TrimSpace(string(top))) != root {
		return "", fmt.Errorf("%s must name an existing checkout, got %q", BatchRootKey, raw)
	}
	if _, err := os.Lstat(filepath.Join(root, "artifacts", "agents", "brain.json")); err == nil {
		return "", fmt.Errorf("%s must name a dedicated non-seat checkout", BatchRootKey)
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect %s as a non-seat checkout: %w", BatchRootKey, err)
	}
	return root, nil
}

func (c BatchLanding) MaxWaitElapsed(oldestJoinedAt time.Time) bool {
	return !c.now().Before(oldestJoinedAt.Add(c.MaxWait))
}

// GetParams is one configuration lookup.
//
// The uncommitted metasystem.conf.local beside ConfPath holds values that must
// not ship to adopting projects (a developer's own evidence root, or the
// template repository's own settings); it wins over the committed file and
// loses to the environment and to an explicit flag.
type GetParams struct {
	Key        string // the setting name, e.g. role.implementer.runtime
	Mode       string // optional mode scope for role.<r>.runtime / role.<r>.model.<m> keys
	Role       string // reserved: overrides are scoped by mode only, so this does not change resolution
	Flag       string // an explicit override value
	FlagSet    bool   // whether Flag was supplied (an empty flag still wins)
	Default    string // fallback when no source holds the key
	DefaultSet bool   // whether Default was supplied
	ConfPath   string // path to metasystem.conf; its .local sibling is derived
	// LookupEnv resolves an environment variable, reporting whether it is set
	// (a set-but-empty variable still wins). Defaults to os.LookupEnv.
	LookupEnv func(string) (string, bool)
}

// Get resolves one key and returns the value with the process exit code the
// caller should use. The precedence, highest first: an explicit flag, the
// mechanically derived environment variable, the .local override (its
// mode-scoped role keys first, then its base keys), the
// mode-scoped key (for role runtime/model keys), the committed key, then the
// explicit default. Exit code 0 carries the value; 2 marks an invalid key or
// mode; 1 marks a missing value or a malformed source (duplicate key,
// unreadable file).
func Get(p GetParams) (value string, code int, err error) {
	lookupEnv := p.LookupEnv
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}

	if !confKeyPattern.MatchString(p.Key) {
		return "", 2, fmt.Errorf("invalid configuration key: %s", p.Key)
	}
	if p.Mode != "" && !modePattern.MatchString(p.Mode) {
		return "", 2, fmt.Errorf("invalid mode: %s", p.Mode)
	}

	if p.FlagSet {
		return p.Flag, 0, nil
	}

	if env, ok := lookupEnv(EnvName(p.Key)); ok {
		return env, 0, nil
	}

	localPath := p.ConfPath + ".local"
	if p.Mode != "" && (roleRuntimeKey.MatchString(p.Key) || roleModelKey.MatchString(p.Key)) && isFile(localPath) {
		// The local overlay outranks the committed file for MODE-scoped
		// role keys exactly as it does for base keys: a machine carrying
		// its model lanes by hand (R-25) writes mode.<m>.role.<r>.* into
		// .local, and skipping the overlay here silently dispatched the
		// wrong lane (found live: a design brief launched on the
		// implementation lane's runtime).
		v, found, err := ConfLookup(localPath, "mode."+p.Mode+"."+p.Key)
		if err != nil {
			return "", 1, err
		}
		if found {
			return v, 0, nil
		}
	}
	if isFile(localPath) {
		v, found, err := ConfLookup(localPath, p.Key)
		if err != nil {
			return "", 1, err
		}
		if found {
			return v, 0, nil
		}
	}

	if p.Mode != "" && (roleRuntimeKey.MatchString(p.Key) || roleModelKey.MatchString(p.Key)) {
		v, found, err := ConfLookup(p.ConfPath, "mode."+p.Mode+"."+p.Key)
		if err != nil {
			return "", 1, err
		}
		if found {
			return v, 0, nil
		}
	}

	v, found, err := ConfLookup(p.ConfPath, p.Key)
	if err != nil {
		return "", 1, err
	}
	if found {
		return v, 0, nil
	}

	if p.DefaultSet {
		return p.Default, 0, nil
	}
	return "", 1, fmt.Errorf("no value configured for %s", p.Key)
}

// ConfLookup reads exactly one setting from a metasystem.conf-format file with
// strict duplicate detection. A line is a setting when it is non-blank, not a
// comment, and contains '='; the key is the text left of the first '=', the
// value the text right of it, both trimmed. found is true when exactly one line
// names key; err is non-nil when the file cannot be read or key appears more
// than once; otherwise found is false and the key is simply absent.
func ConfLookup(path, key string) (value string, found bool, err error) {
	content, readErr := os.ReadFile(path)
	if readErr != nil {
		return "", false, fmt.Errorf("cannot read metasystem configuration: %s: %w", path, readErr)
	}
	// Strict HERE, deliberately: resolution verbs refuse an ambiguous
	// conf instead of silently picking a winner (ConfValue's hot-path
	// readers keep last-wins).
	var matches []string
	parseSettings(string(content), func(_ int, name, val string, ok bool) {
		if ok && name == key {
			matches = append(matches, val)
		}
	})
	if len(matches) > 1 {
		return "", false, fmt.Errorf("duplicate metasystem configuration key: %s", key)
	}
	if len(matches) == 1 {
		return matches[0], true, nil
	}
	return "", false, nil
}

// Keys enumerates the configured keys under prefix, in first-seen order: the
// committed file, then its .local sibling, then numeric-suffix members named
// only in the environment. Enumerating the real keys lets a caller walk a
// family (say model.tier.) without probing a fixed numeric range, so no
// arbitrary bound can hide a configured member. environ is a list of NAME=VALUE
// entries (os.Environ()).
func Keys(confPath, prefix string, environ []string) []string {
	seen := map[string]bool{}
	var keys []string
	add := func(key string) {
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}

	for _, path := range []string{confPath, confPath + ".local"} {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		parseSettings(string(content), func(_ int, key, _ string, ok bool) {
			if ok && strings.HasPrefix(key, prefix) {
				add(key)
			}
		})
	}

	// A key present only in the environment is still a real key. The family
	// shape <prefix><n> maps to <ENVPREFIX><n>, so an env-only member (e.g. a
	// tier) is not invisible to a caller enumerating the family.
	envPrefix := EnvName(prefix)
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(name, envPrefix) {
			continue
		}
		suffix := name[len(envPrefix):]
		if digitsOnlyValue.MatchString(suffix) {
			add(prefix + suffix)
		}
	}
	return keys
}

// EnvName is the environment-variable name a key reads from: METASYSTEM_ joined
// to the upper-cased key with dots and dashes turned into underscores, so
// refactor.max-age-minutes reads METASYSTEM_REFACTOR_MAX_AGE_MINUTES.
func EnvName(key string) string {
	upper := strings.ToUpper(key)
	upper = strings.NewReplacer(".", "_", "-", "_").Replace(upper)
	return "METASYSTEM_" + upper
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// KeyOrigin reports the source Get would take a key from — "env",
// "conf-local", "conf", or "default" — using the resolver's own precedence
// instead of a shadow probe (review script-orchestration-03: the shell copy
// re-derived the env mangling and ignored mode-scoped keys). A mode-scoped
// hit in the committed file reports "conf": the vocabulary names files, not
// key spellings.
func KeyOrigin(p GetParams) (string, error) {
	lookupEnv := p.LookupEnv
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	if !confKeyPattern.MatchString(p.Key) {
		return "", fmt.Errorf("invalid configuration key: %s", p.Key)
	}
	if p.Mode != "" && !modePattern.MatchString(p.Mode) {
		return "", fmt.Errorf("invalid mode: %s", p.Mode)
	}
	if _, ok := lookupEnv(EnvName(p.Key)); ok {
		return "env", nil
	}
	localPath := p.ConfPath + ".local"
	if isFile(localPath) {
		_, found, err := ConfLookup(localPath, p.Key)
		if err != nil {
			return "", err
		}
		if found {
			return "conf-local", nil
		}
	}
	if p.Mode != "" && (roleRuntimeKey.MatchString(p.Key) || roleModelKey.MatchString(p.Key)) {
		_, found, err := ConfLookup(p.ConfPath, "mode."+p.Mode+"."+p.Key)
		if err != nil {
			return "", err
		}
		if found {
			return "conf", nil
		}
	}
	_, found, err := ConfLookup(p.ConfPath, p.Key)
	if err != nil {
		return "", err
	}
	if found {
		return "conf", nil
	}
	return "default", nil
}
