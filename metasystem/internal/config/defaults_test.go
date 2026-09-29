package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// An overrides-only metasystem.conf: every reader answers a key it does not
// name with the compiled-in default, from source "default", and lists it.
func TestCompiledDefaultsAnswerEveryReader(t *testing.T) {
	t.Parallel()
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, "# overrides only\n")
	for key, want := range map[string]string{
		"watch.stale-min": "20", "suite.section-cap-min": "45", "metasystem.runtimes": "claude,codex,devin",
		"testing.contract": "testing.json", "dispatch.max-inline-input-kb": "80", "dispatch.transport.devin": "acp",
		"launch.build.model.claude": "claude-opus-5-5", "role.default.model.claude": "claude-opus-5-5",
	} {
		params := GetParams{Key: key, ConfPath: conf, LookupEnv: noEnv}
		if value, code, err := Get(params); err != nil || code != 0 || value != want {
			t.Fatalf("Get(%s) = %q, %d, %v; want %q", key, value, code, err, want)
		}
		if origin, err := KeyOrigin(params); err != nil || origin != "default" {
			t.Fatalf("KeyOrigin(%s) = %q, %v; want default", key, origin, err)
		}
		if got := ConfValue(conf, key, "caller-default"); got != want {
			t.Fatalf("ConfValue(%s) = %q; want %q", key, got, want)
		}
		if keys := Keys(conf, key, nil); !slices.Contains(keys, key) {
			t.Fatalf("Keys(%s) = %v; want it listed", key, keys)
		}
	}
	// A compiled default wins over a caller's fallback: each default lives once.
	if value, _, err := Get(GetParams{Key: "watch.cap-min", ConfPath: conf, Default: "7", DefaultSet: true, LookupEnv: noEnv}); err != nil || value != "180" {
		t.Fatalf("Get(watch.cap-min) with a caller default = %q, %v; want the compiled 180", value, err)
	}
	// An override in the file wins and names its source.
	putFile(t, conf, "watch.stale-min=5\n")
	params := GetParams{Key: "watch.stale-min", ConfPath: conf, LookupEnv: noEnv}
	if value, _, _ := Get(params); value != "5" {
		t.Fatalf("an override lost to the default: %q", value)
	}
	if origin, _ := KeyOrigin(params); origin != "conf" {
		t.Fatalf("an override's source = %q; want conf", origin)
	}
}

// A default that binds a runtime holds only while that runtime is selected,
// so an installation without Claude inherits no Claude roster, and an auto
// runtime resolves among the selected runtimes alone.
func TestRuntimeBoundDefaultsFollowTheSelectedRuntimes(t *testing.T) {
	t.Parallel()
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, "metasystem.runtimes=codex\n")
	for _, key := range []string{"role.default.model.claude", "runtime.claude.maximal-models", "dispatch.transport.devin"} {
		if value, code, err := Get(GetParams{Key: key, ConfPath: conf, LookupEnv: noEnv}); err == nil || code != 1 {
			t.Fatalf("Get(%s) without its runtime = %q, %d, %v; want no value", key, value, code, err)
		}
		if keys := Keys(conf, key, nil); slices.Contains(keys, key) {
			t.Fatalf("Keys lists %s without its runtime", key)
		}
		if got := ConfValue(conf, key, ""); got != "" {
			t.Fatalf("ConfValue(%s) without its runtime = %q", key, got)
		}
	}
	if value, _, err := Get(GetParams{Key: "role.default.runtime", ConfPath: conf, LookupEnv: noEnv}); err != nil || value != "codex" {
		t.Fatalf("role.default.runtime with only Codex selected = %q, %v; want codex", value, err)
	}
	if value, _, err := Get(GetParams{Key: "role.verifier.runtime", ConfPath: conf, LookupEnv: noEnv}); err != nil || value != "main" {
		t.Fatalf("a runtime-neutral default did not hold: %q, %v", value, err)
	}
	// The environment's runtime selection is the one that counts.
	env := mapEnv(map[string]string{EnvName("metasystem.runtimes"): "claude"})
	if value, _, err := Get(GetParams{Key: "role.default.runtime", ConfPath: conf, LookupEnv: env}); err != nil || value != "claude" {
		t.Fatalf("role.default.runtime with Claude selected by the environment = %q, %v", value, err)
	}
}

// A mode-scoped default outranks a base key of the committed layer, as the
// same line in the shipped file did.
func TestModeScopedDefaultsKeepTheirPrecedence(t *testing.T) {
	t.Parallel()
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, "role.implementer.model.claude=base-override\n")
	params := GetParams{Key: "role.implementer.model.claude", Mode: "design", ConfPath: conf, LookupEnv: noEnv}
	if value, _, err := Get(params); err != nil || value != "claude-fable-5-1" {
		t.Fatalf("design-mode implementer model = %q, %v; want the compiled mode default", value, err)
	}
	if origin, _ := KeyOrigin(params); origin != "default" {
		t.Fatalf("design-mode implementer model source = %q; want default", origin)
	}
	params.Mode = ""
	if value, _, _ := Get(params); value != "base-override" {
		t.Fatalf("base implementer model = %q; want the override", value)
	}
}

// Validation reads the effective configuration: an overrides-only file is
// complete, and the compiled defaults are themselves valid.
func TestValidateAcceptsAnOverridesOnlyConfiguration(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	conf := filepath.Join(repo, "metasystem.conf")
	putFile(t, conf, "# overrides only\n")
	contract, err := os.ReadFile(filepath.Join("..", "..", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(repo, "testing.json"), string(contract))
	for _, dir := range []string{".claude/agents", ".claude/skills", ".codex", ".agents/skills", ".devin"} {
		if err := os.MkdirAll(filepath.Join(repo, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	_, problems, err := Validate(conf, repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, problem := range problems {
		if strings.Contains(problem, "is required") || strings.Contains(problem, "has no model") {
			t.Fatalf("an overrides-only configuration was refused: %v", problems)
		}
	}
}

// The table is well formed: every key valid and unique, every row explained.
func TestCompiledSettingsTableIsWellFormed(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, setting := range CompiledSettings() {
		if !confKeyPattern.MatchString(setting.Key) || seen[setting.Key] {
			t.Fatalf("setting key %q is malformed or repeated", setting.Key)
		}
		seen[setting.Key] = true
		if strings.TrimSpace(setting.Meaning) == "" {
			t.Fatalf("setting %s has no meaning", setting.Key)
		}
	}
	if ProofInput(EvidenceRootKey) {
		t.Fatal("the evidence root, a per-checkout path, is a proof input")
	}
	if !ProofInput("suite.section-cap-min") || !ProofInput("an.unknown-key") {
		t.Fatal("a proof control or an unknown key is not a proof input")
	}
}

// The shipped metasystem.conf holds overrides only: no active line repeats a
// compiled default, so each default lives once, in Go.
func TestShippedConfigurationHoldsNoCompiledDefault(t *testing.T) {
	t.Parallel()
	content, err := os.ReadFile(filepath.Join("..", "..", "metasystem.conf"))
	if err != nil {
		t.Fatal(err)
	}
	parseSettings(string(content), func(line int, key, value string, ok bool) {
		if !ok {
			return
		}
		if setting, found := compiledSetting(key); found && setting.Computed == "" && setting.Default == value {
			t.Errorf("metasystem.conf:%d repeats the compiled default %s=%s", line, key, value)
		}
	})
}

// The only compiled mode-scoped role defaults are the design author's: its
// auto runtime and its model on each runtime.
func TestCompiledModeRoleDefaultsAreTheDesignAuthor(t *testing.T) {
	t.Parallel()
	var got []string
	for _, setting := range CompiledSettings() {
		if strings.HasPrefix(setting.Key, "mode.") {
			got = append(got, setting.Key+"="+setting.Default)
		}
	}
	want := []string{"mode.design.role.implementer.runtime=auto", "mode.design.role.implementer.model.claude=claude-fable-5-1",
		"mode.design.role.implementer.model.codex=gpt-6-astra", "mode.design.role.implementer.model.devin=claude-opus-5-5-xhigh"}
	if !slices.Equal(got, want) {
		t.Fatalf("compiled mode role defaults %q, want %q", got, want)
	}
}

// Template mode reads the committed file alone: the uncommitted .local file
// cannot make a checkout the template (and no environment variable is read).
func TestTemplateModeReadsOnlyTheCommittedKey(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	conf := filepath.Join(root, "metasystem.conf")
	putFile(t, conf, "# overrides only\n")
	putFile(t, conf+".local", TemplateModeKey+"=true\n")
	if TemplateMode(root) {
		t.Fatal("a .local value made the checkout the template")
	}
	putFile(t, conf, TemplateModeKey+"=true\n")
	if !TemplateMode(root) {
		t.Fatal("the committed key did not declare the template")
	}
	if value, ok := CompiledDefault(TemplateModeKey); !ok || value != "false" {
		t.Fatalf("compiled template default = %q", value)
	}
}

// The disk-lifetime settings and the cache trimmer's seven keys have their
// defaults in the one compiled table: an overrides-only file answers each
// from source "default", lists it, and none is a proof input (they meter
// the machine's disk, never what a proof proves).
func TestDiskDefaultsAreInTheOneCompiledTable(t *testing.T) {
	t.Parallel()
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, "# overrides only\n")
	want := map[string]string{
		DiskGoCacheCapGiBKey: "30", DiskDelegateGoCacheCapGiBKey: "10", DiskStaticcheckCacheCapGiBKey: "2",
		DiskGoCacheKeepHoursKey: "12", DiskCacheTrimBudgetSecKey: "10", DiskCacheTrimPersonBudgetSecKey: "300",
		DiskCacheMinKeepMinutesKey: "120",
	}
	for _, row := range DiskSettings() {
		if row.Default != "" {
			want[row.Key] = row.Default
		}
	}
	if len(want) != 33 {
		t.Fatalf("disk keys with a default = %d, want 33 (26 of 3.13 and the trimmer's seven)", len(want))
	}
	for key, value := range want {
		if compiled, ok := CompiledDefault(key); !ok || compiled != value {
			t.Errorf("compiled default of %s = %q, %v; want %q", key, compiled, ok, value)
		}
		params := GetParams{Key: key, ConfPath: conf, LookupEnv: noEnv}
		if got, code, err := Get(params); err != nil || code != 0 || got != value {
			t.Errorf("Get(%s) = %q, %d, %v; want %q", key, got, code, err, value)
		}
		if origin, err := KeyOrigin(params); err != nil || origin != "default" {
			t.Errorf("KeyOrigin(%s) = %q, %v; want default", key, origin, err)
		}
		if keys := Keys(conf, key, nil); !slices.Contains(keys, key) {
			t.Errorf("Keys(%s) does not list it", key)
		}
		if ProofInput(key) {
			t.Errorf("%s is a proof input", key)
		}
	}
	trim, err := CacheTrimSettings(conf)
	if err != nil || trim.EngineGoCapBytes != 30<<30 || trim.Budget.Seconds() != 10 || trim.PersonBudget.Seconds() != 300 || trim.MinKeep.Minutes() != 120 {
		t.Fatalf("CacheTrimSettings over an overrides-only file = %+v, %v", trim, err)
	}
}

// No configuration file (an empty path) is the compiled defaults under the
// environment: Get and KeyOrigin answer every key Keys lists, as they did
// before the defaults were compiled in, when Keys listed nothing for it.
func TestNoConfigurationFileResolvesTheCompiledDefaults(t *testing.T) {
	t.Parallel()
	env := mapEnv(map[string]string{EnvName("watch.stale-min"): "7"})
	for _, key := range Keys("", "", nil) {
		if key == EvidenceRootKey {
			continue // resolved from HOME by its owner, which noEnv leaves unset
		}
		if _, code, err := Get(GetParams{Key: key, LookupEnv: noEnv}); err != nil || code != 0 {
			t.Fatalf("Get(%s) without a configuration file = %d, %v", key, code, err)
		}
	}
	if value, _, err := Get(GetParams{Key: "suite.section-cap-min", LookupEnv: noEnv}); err != nil || value != "45" {
		t.Fatalf("a compiled default without a file = %q, %v", value, err)
	}
	if value, _, err := Get(GetParams{Key: "watch.stale-min", LookupEnv: env}); err != nil || value != "7" {
		t.Fatalf("the environment without a file = %q, %v", value, err)
	}
	if origin, err := KeyOrigin(GetParams{Key: "watch.stale-min", LookupEnv: noEnv}); err != nil || origin != "default" {
		t.Fatalf("KeyOrigin without a file = %q, %v", origin, err)
	}
}

// TestBoardAndPipelineSettingsHaveCompiledDefaults (R19, R22, R24; U10b-1's
// five rows): the pipeline keys and the max wait resolve to the table's
// defaults with no configuration file; a stage list naming a stage twice,
// missing one of the pre-join order or naming landing, and a non-integer
// -min or -n, are refused by validate with the key named; the wait cap is
// not a key.
func TestBoardAndPipelineSettingsHaveCompiledDefaults(t *testing.T) {
	t.Parallel()
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, "# overrides only\n")
	for key, want := range map[string]string{
		BatchMaxWaitKey: "10m", PipelineStallMinKey: "20", PipelineProofCostKey: "40m",
		PipelineStageDefaultsKey: "build=10m,revise=10m,unit-proof=5m,review=18m,judgement=5m,land-ready=3m", PipelineHistoryNKey: "8",
		BoardKeepHoursKey: "24", BoardPollSecKey: "5", BoardMailboxKeepDaysKey: "7", BoardHandoverLockWaitSecKey: "30",
	} {
		if value, _, err := Get(GetParams{Key: key, ConfPath: conf, LookupEnv: noEnv}); err != nil || value != want {
			t.Fatalf("Get(%s) = %q, %v; want %q", key, value, err, want)
		}
		if ProofInput(key) {
			t.Fatalf("%s decides when a proof starts, never what it proves; it is no proof input", key)
		}
	}
	if DefaultBatchMaxWait != 10*time.Minute {
		t.Fatalf("DefaultBatchMaxWait = %v; the table's 10m", DefaultBatchMaxWait)
	}
	settings := DefaultPipelineSettings()
	if settings.Stall != 20*time.Minute || settings.ProofCost != 40*time.Minute || settings.HistoryN != 8 ||
		settings.StageDefaults["review"] != 18*time.Minute || settings.StageDefaults["judgement"] != 5*time.Minute || len(settings.StageDefaults) != 6 {
		t.Fatalf("compiled pipeline settings = %+v", settings)
	}
	if _, ok := CompiledDefault("landing.pipeline-wait-cap"); ok {
		t.Fatal("landing.pipeline-wait-cap is not a key: the bound is the measured proof cost")
	}
	for _, row := range []struct{ setting, want string }{
		{PipelineStageDefaultsKey + "=build=10m,build=5m,revise=10m,unit-proof=5m,review=18m,judgement=5m,land-ready=3m\n", "names build twice"},
		{PipelineStageDefaultsKey + "=build=10m,revise=10m,unit-proof=5m,review=18m,land-ready=3m\n", "does not name judgement"},
		{PipelineStageDefaultsKey + "=build=10m,revise=10m,unit-proof=5m,review=18m,judgement=5m,land-ready=3m,landing=2m\n", "landing"},
		{PipelineStallMinKey + "=20m\n", PipelineStallMinKey},
		{PipelineHistoryNKey + "=eight\n", PipelineHistoryNKey},
		{PipelineProofCostKey + "=forty\n", PipelineProofCostKey},
		{BoardMailboxKeepDaysKey + "=7d\n", BoardMailboxKeepDaysKey},
	} {
		if problems := validateRepo(t, validConf+row.setting); !hasProblem(problems, row.want) {
			t.Fatalf("Validate accepted %q: %v", row.setting, problems)
		}
	}
	if problems := validateRepo(t, validConf); len(problems) != 0 {
		t.Fatalf("the compiled max wait alone is no max wait without a root: %v", problems)
	}
}
