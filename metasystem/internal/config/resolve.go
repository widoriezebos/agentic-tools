package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
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
	BatchRootKey             = "landing.batch-root"
	BatchMaxWaitKey          = "landing.batch-max-wait"
	PipelineStallMinKey      = "landing.pipeline-stall-min"
	PipelineProofCostKey     = "landing.pipeline-proof-cost"
	PipelineStageDefaultsKey = "landing.pipeline-stage-defaults"
	PipelineHistoryNKey      = "landing.pipeline-history-n"
)

// DefaultBatchMaxWait is the compiled max wait; the table holds the number.
var DefaultBatchMaxWait = durationDefault(BatchMaxWaitKey)

// PipelineStages are the stages of the pre-join order, each of which the
// stage defaults name exactly once.
var PipelineStages = []string{"build", "revise", "unit-proof", "review", "judgement", "land-ready"}

// PipelineSettings decide when a batch starts from the host board (D14,
// R22): the stall bound, the separate-proof cost and the stage estimates
// used until the lane has measured its own, and the median's window.
type PipelineSettings struct {
	Stall         time.Duration
	ProofCost     time.Duration
	StageDefaults map[string]time.Duration
	HistoryN      int
}

type BatchLanding struct {
	Root     string
	MaxWait  time.Duration
	Pipeline PipelineSettings
	now      func() time.Time
}

func durationDefault(key string) time.Duration {
	value, err := time.ParseDuration(MustDefault(key))
	if err != nil {
		panic("config: compiled default " + key + " is not a duration")
	}
	return value
}

// DefaultPipelineSettings are the compiled pipeline settings.
func DefaultPipelineSettings() PipelineSettings {
	stages, err := ParseStageDefaults(MustDefault(PipelineStageDefaultsKey))
	if err != nil {
		panic("config: compiled " + PipelineStageDefaultsKey + ": " + err.Error())
	}
	return PipelineSettings{Stall: time.Duration(intDefault(PipelineStallMinKey)) * time.Minute,
		ProofCost: durationDefault(PipelineProofCostKey), StageDefaults: stages, HistoryN: intDefault(PipelineHistoryNKey)}
}

// ParseStageDefaults reads a stage list: every stage of the pre-join order
// named exactly once with a positive duration, and nothing else.
func ParseStageDefaults(raw string) (map[string]time.Duration, error) {
	stages := map[string]time.Duration{}
	known := map[string]bool{}
	for _, stage := range PipelineStages {
		known[stage] = true
	}
	for _, item := range strings.Split(raw, ",") {
		name, value, ok := strings.Cut(strings.TrimSpace(item), "=")
		if !ok || !known[name] {
			return nil, fmt.Errorf("%s names %q, which is not a stage of the pre-join order (%s)", PipelineStageDefaultsKey, name, strings.Join(PipelineStages, ", "))
		}
		if _, twice := stages[name]; twice {
			return nil, fmt.Errorf("%s names %s twice", PipelineStageDefaultsKey, name)
		}
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return nil, fmt.Errorf("%s gives %s the duration %q; want a positive duration", PipelineStageDefaultsKey, name, value)
		}
		stages[name] = duration
	}
	for _, stage := range PipelineStages {
		if _, ok := stages[stage]; !ok {
			return nil, fmt.Errorf("%s does not name %s", PipelineStageDefaultsKey, stage)
		}
	}
	return stages, nil
}

// resolvePipelineSettings reads the pipeline keys through the one resolver.
func resolvePipelineSettings(confPath string) (PipelineSettings, error) {
	get := func(key string) (string, error) {
		value, _, err := Get(GetParams{Key: key, ConfPath: confPath})
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", key, err)
		}
		return value, nil
	}
	settings := PipelineSettings{}
	raw, err := get(PipelineStallMinKey)
	if err != nil {
		return settings, err
	}
	minutes, err := strconv.Atoi(raw)
	if err != nil || minutes < 1 {
		return settings, fmt.Errorf("%s must be a positive integer, got %q", PipelineStallMinKey, raw)
	}
	settings.Stall = time.Duration(minutes) * time.Minute
	if raw, err = get(PipelineProofCostKey); err != nil {
		return settings, err
	}
	if settings.ProofCost, err = time.ParseDuration(raw); err != nil || settings.ProofCost <= 0 {
		return settings, fmt.Errorf("%s must be a positive duration, got %q", PipelineProofCostKey, raw)
	}
	if raw, err = get(PipelineStageDefaultsKey); err != nil {
		return settings, err
	}
	if settings.StageDefaults, err = ParseStageDefaults(raw); err != nil {
		return settings, err
	}
	if raw, err = get(PipelineHistoryNKey); err != nil {
		return settings, err
	}
	if settings.HistoryN, err = strconv.Atoi(raw); err != nil || settings.HistoryN < 1 {
		return settings, fmt.Errorf("%s must be a positive integer, got %q", PipelineHistoryNKey, raw)
	}
	return settings, nil
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
	return BatchLanding{Root: realpath.Resolve(root), MaxWait: maxWait, Pipeline: DefaultPipelineSettings(), now: now}, nil
}

// WithPipeline binds pipeline settings the caller resolved from its own
// configuration file.
func (landing BatchLanding) WithPipeline(confPath string) (BatchLanding, error) {
	settings, err := resolvePipelineSettings(confPath)
	if err != nil {
		return landing, err
	}
	landing.Pipeline = settings
	return landing, nil
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
	pipeline, err := resolvePipelineSettings(confPath)
	if err != nil {
		return BatchLanding{}, err
	}
	return BatchLanding{Root: root, MaxWait: wait, Pipeline: pipeline, now: now}, nil
}

func batchLandingRootWithRunner(raw, seatRoot string, runner gitRunner) (string, error) {
	if !filepath.IsAbs(raw) {
		return "", fmt.Errorf("%s must be absolute, got %q", BatchRootKey, raw)
	}
	root := realpath.Resolve(raw)
	if root == realpath.Resolve(seatRoot) {
		return "", fmt.Errorf("%s must name a dedicated non-seat checkout", BatchRootKey)
	}
	if runner == nil {
		return "", fmt.Errorf("resolve %s: a Git runner is required", BatchRootKey)
	}
	top, err := runner(gitRequest{Directory: root, Args: []string{"rev-parse", "--show-toplevel"}, Environment: []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C"}})
	if err != nil || realpath.Resolve(strings.TrimSpace(string(top))) != root {
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
	Policy *PolicyContext // current checkout and explicit registry/helm readers

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
	// KeepAuto returns a runtime-selection key's configured `auto` as it is,
	// instead of the runtime it resolves to on this environment's PATH.
	KeepAuto bool
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
	if PolicyScope(p.Key) != "" {
		resolved, problem := ResolvePolicy(p)
		if problem != nil {
			return "", 1, problem
		}
		return resolved.Value, 0, nil
	}
	value, code, err = getLayered(p)
	if err != nil || code != 0 || p.KeepAuto || value != AutoRuntime || !RuntimeSelectionKey(p.Key) {
		return value, code, err
	}
	lookupEnv := p.LookupEnv
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}
	choice, err := ResolveAutoRuntime(p.ConfPath, lookupEnv)
	if err != nil {
		return "", 1, err
	}
	return choice.Runtime, 0, nil
}

func getLayered(p GetParams) (value string, code int, err error) {
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
	if CommittedOnly(p.Key) {
		value, found, err := CommittedLookup(p.ConfPath, p.Key)
		if err != nil {
			return "", 1, err
		}
		if !found {
			return "", 1, fmt.Errorf("%w for %s in metasystem.conf", ErrNoValue, p.Key)
		}
		if err := SettingValueProblem(p.Key, value); err != nil {
			return "", 1, err
		}
		return value, 0, nil
	}
	// The evidence root has one owner and a compiled-in default; a general
	// reader answers what the owner resolves, never "no value".
	if p.Key == EvidenceRootKey && !p.FlagSet {
		root, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: p.ConfPath, LookupEnv: lookupEnv})
		if err != nil {
			return "", 1, err
		}
		return root.Path, 0, nil
	}

	if p.FlagSet {
		return p.Flag, 0, nil
	}

	if env, ok := lookupEnv(EnvName(p.Key)); ok {
		return env, 0, nil
	}

	// An empty ConfPath is no configuration file: the compiled defaults
	// under the environment and the flag, nothing read from disk.
	hasFile := p.ConfPath != ""
	localPath := p.ConfPath + ".local"
	if hasFile && p.Mode != "" && (roleRuntimeKey.MatchString(p.Key) || roleModelKey.MatchString(p.Key)) && isFile(localPath) {
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
	if hasFile && isFile(localPath) {
		v, found, err := ConfLookup(localPath, p.Key)
		if err != nil {
			return "", 1, err
		}
		if found {
			return v, 0, nil
		}
	}

	// The committed layer is the file over the compiled defaults
	// (defaults.go): a mode-scoped key, file then default, outranks the
	// base key, file then default, as the same lines in the shipped file did.
	runtimes := effectiveRuntimes(p.ConfPath, lookupEnv)
	if p.Mode != "" && (roleRuntimeKey.MatchString(p.Key) || roleModelKey.MatchString(p.Key)) {
		if hasFile {
			v, found, err := ConfLookup(p.ConfPath, "mode."+p.Mode+"."+p.Key)
			if err != nil {
				return "", 1, err
			}
			if found {
				return v, 0, nil
			}
		}
		if v, ok := applicableDefault("mode."+p.Mode+"."+p.Key, runtimes); ok {
			return v, 0, nil
		}
	}

	if hasFile {
		v, found, err := ConfLookup(p.ConfPath, p.Key)
		if err != nil {
			return "", 1, err
		}
		if found {
			return v, 0, nil
		}
	}
	// A compiled default outranks a caller's fallback: each default lives
	// once, in the table.
	if v, ok := applicableDefault(p.Key, runtimes); ok {
		return v, 0, nil
	}

	if p.DefaultSet {
		return p.Default, 0, nil
	}
	return "", 1, fmt.Errorf("%w for %s", ErrNoValue, p.Key)
}

// ErrNoValue is Get's refusal of a key that is set nowhere and has no
// default: a caller that treats an unset key as empty decides on it with
// errors.Is, never on the words.
var ErrNoValue = errors.New("no value configured")

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
	return confContentLookup(string(content), key)
}

func confContentLookup(content, key string) (value string, found bool, err error) {
	// Strict HERE, deliberately: resolution verbs refuse an ambiguous
	// conf instead of silently picking a winner (ConfValue's hot-path
	// readers keep last-wins).
	var matches []string
	parseSettings(content, func(_ int, name, val string, ok bool) {
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

// CommittedLookup reads one key of the committed layer: the file's own line,
// else the key's compiled default when it holds under the file's runtime
// selection (defaults.go). It is strict on duplicates and on an unreadable
// file, as ConfLookup is.
func CommittedLookup(confPath, key string) (value string, found bool, err error) {
	if confPath == "" {
		return CommittedContentLookup("", key)
	}
	content, readErr := os.ReadFile(confPath)
	if readErr != nil {
		return "", false, fmt.Errorf("cannot read metasystem configuration: %s: %w", confPath, readErr)
	}
	return CommittedContentLookup(string(content), key)
}

// CommittedContentLookup reads a committed configuration's content with the
// same duplicate detection and applicable defaults as CommittedLookup.
func CommittedContentLookup(content, key string) (value string, found bool, err error) {
	value, found, err = confContentLookup(content, key)
	if err != nil || found {
		return value, found, err
	}
	if compiled, ok := applicableDefault(key, fileRuntimes(content)); ok {
		return compiled, true, nil
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
	// The evidence root always has a value (its compiled-in default), so it
	// is always a configured key.
	if strings.HasPrefix(EvidenceRootKey, prefix) {
		add(EvidenceRootKey)
	}
	// Every applicable compiled default is a configured key too.
	lookup := func(name string) (string, bool) {
		for index := len(environ) - 1; index >= 0; index-- {
			if key, value, ok := strings.Cut(environ[index], "="); ok && key == name {
				return value, true
			}
		}
		return "", false
	}
	runtimes := effectiveRuntimes(confPath, lookup)
	for _, setting := range compiledSettings {
		if !strings.HasPrefix(setting.Key, prefix) {
			continue
		}
		if _, ok := applicableDefault(setting.Key, runtimes); ok {
			add(setting.Key)
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
	if PolicyScope(p.Key) != "" {
		resolved, err := ResolvePolicy(p)
		return resolved.Source, err
	}
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
	if CommittedOnly(p.Key) {
		source := "default"
		if p.ConfPath != "" {
			_, found, err := ConfLookup(p.ConfPath, p.Key)
			if err != nil {
				return "", err
			}
			if found {
				source = "conf"
			}
		}
		if _, set := lookupEnv(EnvName(p.Key)); set {
			source += "; environment value ignored"
		}
		if p.ConfPath != "" && isFile(p.ConfPath+".local") {
			_, found, err := ConfLookup(p.ConfPath+".local", p.Key)
			if found || err != nil {
				source += "; local value ignored"
			}
		}
		return source, nil
	}
	if p.Key == EvidenceRootKey {
		root, err := ResolveEvidenceRoot(EvidenceRootParams{ConfPath: p.ConfPath, LookupEnv: lookupEnv})
		if err != nil {
			return "", err
		}
		return root.Origin, nil
	}
	if _, ok := lookupEnv(EnvName(p.Key)); ok {
		return "env", nil
	}
	if p.ConfPath == "" {
		// No configuration file: what the environment does not set is
		// the compiled default.
		return "default", nil
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
	runtimes := effectiveRuntimes(p.ConfPath, lookupEnv)
	if p.Mode != "" && (roleRuntimeKey.MatchString(p.Key) || roleModelKey.MatchString(p.Key)) {
		_, found, err := ConfLookup(p.ConfPath, "mode."+p.Mode+"."+p.Key)
		if err != nil {
			return "", err
		}
		if found {
			return "conf", nil
		}
		if _, ok := applicableDefault("mode."+p.Mode+"."+p.Key, runtimes); ok {
			return "default", nil
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
