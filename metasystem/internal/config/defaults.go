package config

import (
	"strconv"
	"strings"
)

// Setting is one configuration key the engine compiles a default for.
//
// Data the engine reads at runtime to decide its own behaviour is source
// code (Wido, 2026-09-28): every default lives here, once, and
// metasystem.conf holds only a project's overrides. Get, KeyOrigin, Keys,
// ConfValue and Validate read this table beneath the committed file, so an
// overrides-only file resolves exactly as the full shipped file did.
type Setting struct {
	Key string
	// Default is the compiled value; empty when Computed names the owner
	// that derives it.
	Default string
	// Meaning is what the key decides, for `metasystem settings`.
	Meaning string
	// ProofInput reports whether the resolved value is an input to what a
	// proof proves (the proof configuration digest). A machine- or
	// checkout-specific value is not: it must not change proof digests
	// between checkouts.
	ProofInput bool
	// Runtime, when set, is the runtime the default binds: it holds only
	// while that runtime is in the effective metasystem.runtimes, so an
	// installation without it inherits none of its roster.
	Runtime string
	// Computed names the owner that derives the default at runtime; such a
	// key has no static default here.
	Computed string
}

// compiledSettings is the one table of compiled-in configuration defaults:
// the rows below, then every disk-lifetime row of disksettings.go that has a
// default (those rows carry their unit, scope and conservative direction
// beside the default, so they are spelled there and registered here).
var compiledSettings = append(coreSettings, diskCompiledSettings()...)

// diskCompiledSettings registers the disk-lifetime rows (design
// engine-owns-disk-lifetimes 3.13) into the one table. A row without a
// default ("none") stays out: it has no value to show. None is a proof
// input: they bound what the machine keeps on disk, never what a proof
// proves.
func diskCompiledSettings() []Setting {
	var rows []Setting
	for _, row := range diskSettings {
		if row.Default == "" || row.Key == EvidenceRootKey {
			continue
		}
		rows = append(rows, Setting{Key: row.Key, Default: row.Default, ProofInput: false,
			Meaning: row.Meaning + " (" + string(row.Unit) + ")"})
	}
	return rows
}

var coreSettings = []Setting{
	// Installation shape.
	{Key: "metasystem.engine-delivery", Default: "source", ProofInput: true,
		Meaning: "how the engine ships (D17/D33): source, rebuilt by the target and by CI; declared, never inferred"},
	{Key: "metasystem.runtimes", Default: "claude,codex,devin", ProofInput: true,
		Meaning: "the runtimes this installation runs agents on; a runtime-bound default holds only while its runtime is listed"},
	{Key: "testing.contract", Default: "testing.json", ProofInput: true,
		Meaning: "the committed application testing contract, relative to the installation; delivery reads only this path"},
	{Key: "launch.contract", Default: "launch.json", ProofInput: true,
		Meaning: "the application launch contract the app verbs start, probe and stop; a project without the file has no app verbs"},
	{Key: TemplateModeKey, Default: "false", ProofInput: true,
		Meaning: "true only in the template repository's committed metasystem.conf (read there alone); adoption never ships it"},
	{Key: EvidenceRootKey, Computed: "config.ResolveEvidenceRoot", ProofInput: false,
		Meaning: "where durable evidence is mirrored; defaults to ~/metasystem-evidence/<checkout name>, outside the repository"},
	{Key: "proof.admission.top-level-max", Computed: "proofrun.ResolveAdmissionCap", ProofInput: false,
		Meaning: "host-wide cap on concurrent top-level proof attempts (ruling R-111-m1e); unset is max(1, cores/6), 0 disables it"},

	// The interface (ui.go). None is a proof input: the interface never
	// changes what a proof proves, and ui.listen is a machine's own port.
	{Key: UIListenKey, Default: "127.0.0.1:7878", ProofInput: false,
		Meaning: "the loopback address the interface server listens on; a machine sets its own in the .local file"},
	{Key: UISubjectKey, Default: "", ProofInput: false,
		Meaning: "the name the interface gives this workspace; empty names the template itself when self-hosted, and the checkout otherwise"},
	{Key: UISessionHoursKey, Default: "12", ProofInput: false,
		Meaning: "how many hours a signed-in browser session lasts"},
	{Key: UIHumanKey, Default: "", ProofInput: false,
		Meaning: "the handle this seat signs in as; empty, the sign-in sheet asks once"},
	{Key: UIPartnerRuntimeKey, Default: "", ProofInput: false,
		Meaning: "which agent answers as the Project Partner (claude, codex or devin); empty is a seat with no Partner"},
	{Key: UIPartnerModelKey, Default: "", ProofInput: false,
		Meaning: "the model the Partner's session runs; empty is the runtime's own default"},
	{Key: UIStoreWireMBKey, Default: "8", ProofInput: false,
		Meaning: "megabytes the Partner's wire journal may reach before rotation; 0 disables the bound"},
	{Key: UIStoreConversationMBKey, Default: "2", ProofInput: false,
		Meaning: "megabytes one transcript may reach before its oldest messages are trimmed; 0 disables the bound"},
	{Key: UIStoreConversationDaysKey, Default: "90", ProofInput: false,
		Meaning: "days the oldest message of a transcript may reach before it is trimmed; 0 disables the bound"},
	{Key: ReviewDesignToolCallsKey, Default: "30", ProofInput: false,
		Meaning: "the reader tool-call budget the interface's Send to critique prefills for a design review; the sheet may change it"},

	// Spend accounting (spend.go), alert-only; committed root only outside a
	// fixture root. Not a proof input: it meters, it never changes a result.
	{Key: SpendModeKey, Default: "alert", ProofInput: false,
		Meaning: "spend accounting mode; alert measures and alerts, never refuses"},
	{Key: SpendCurrencyKey, Default: "USD", ProofInput: false,
		Meaning: "the currency spend is priced in"},
	{Key: SpendZoneKey, Default: "UTC", ProofInput: false,
		Meaning: "the time zone a spend day is counted in"},
	{Key: SpendCeilingDayTokensKey, Default: "250000000", ProofInput: false,
		Meaning: "the daily token ceiling the spend meter alerts on"},
	{Key: SpendCeilingDayMoneyKey, Default: "750", ProofInput: false,
		Meaning: "the daily money ceiling the spend meter alerts on"},
	{Key: SpendCeilingGoalTokensKey, Default: "125000000", ProofInput: false,
		Meaning: "the per-goal token ceiling the spend meter alerts on"},
	{Key: SpendCeilingGoalMoneyKey, Default: "300", ProofInput: false,
		Meaning: "the per-goal money ceiling the spend meter alerts on"},

	// Model catalogue.
	{Key: "runtime.claude.maximal-models", Default: "claude-fable-5-1,claude-opus-5-5", ProofInput: true, Runtime: "claude",
		Meaning: "the Claude models admitted as maximal"},
	{Key: "runtime.claude.model-alias.claude-fable-5", Default: "claude-fable-5-1", ProofInput: true, Runtime: "claude",
		Meaning: "family pointer (R-71-m3): the source id means the target at resolution"},

	// Budget law (committed root only outside a fixture root).
	{Key: ElapsedGracePercentKey, Default: "50", ProofInput: true,
		Meaning: "admission closes at the goal limit; breach-stop waits this percentage longer"},
	{Key: SliceNormHoursKey, Default: "4", ProofInput: true,
		Meaning: "the ordinary per-job slice norm, in hours"},
	{Key: ReviewRoundMaxKey, Default: "3", ProofInput: true,
		Meaning: "the review-round ceiling per critique class (decision 06, R-42-m0); 0 removes it"},
	{Key: Tier1BudgetKey, Default: "1h/3/360m/1/0", ProofInput: true,
		Meaning: "the tier-1 goal budget box: elapsed/attempts/job-minutes/design-critiques/code-critiques"},
	{Key: Tier2BudgetKey, Default: "4h/6/720m/1/2", ProofInput: true,
		Meaning: "the tier-2 goal budget box"},
	{Key: Tier3BudgetKey, Default: "8h/10/1200m/1/3", ProofInput: true,
		Meaning: "the tier-3 goal budget box"},
	{Key: CarryOpenMaxKey, Default: "1", ProofInput: true,
		Meaning: "the most open carry words one seat may own at once"},

	// Steward, seat and context.
	{Key: LedgerAttentionStaleMinutesKey, Default: "30", ProofInput: true,
		Meaning: "minutes after which the ledger's attention reads as stale"},
	{Key: SeatPresenceStaleMinutesKey, Default: "30", ProofInput: true,
		Meaning: "a seat whose presence record is older than this many minutes (or three of its writer's ticks) reads as unreachable"},
	{Key: "steward.tick-patience-sec", Default: "120", ProofInput: true,
		Meaning: "seconds a steward tick may take before the resident runner is considered stuck"},
	{Key: "steward.stop-slow-sec", Default: "15", ProofInput: true,
		Meaning: "a Stop using this many seconds of its sixty-second budget is unhealthy"},
	// The steward's machine cache trimmer (disk-lifetimes A12). Not proof
	// inputs: they bound the machine's caches, never what a proof proves.
	{Key: DiskGoCacheCapGiBKey, Default: "30", ProofInput: false,
		Meaning: "GiB the engine's Go build cache is trimmed to by last use"},
	{Key: DiskDelegateGoCacheCapGiBKey, Default: "10", ProofInput: false,
		Meaning: "GiB the one machine delegate Go cache is trimmed to by last use"},
	{Key: DiskStaticcheckCacheCapGiBKey, Default: "2", ProofInput: false,
		Meaning: "GiB each staticcheck cache is trimmed to by last use"},
	{Key: DiskGoCacheKeepHoursKey, Default: "12", ProofInput: false,
		Meaning: "hours within which a used cache entry is never trimmed, whatever the cap"},
	{Key: DiskCacheTrimBudgetSecKey, Default: "10", ProofInput: false,
		Meaning: "seconds one trim pass over all caches may take; a cut pass resumes where it ended"},
	{Key: DiskCacheTrimPersonBudgetSecKey, Default: "300", ProofInput: false,
		Meaning: "seconds metasystem disk clean, run at a terminal, keeps trimming pass after pass until every cache is measured and within its cap; the steward's own pass keeps disk.cache-trim-budget-sec"},
	{Key: ContextCeilingTokensKey, Default: "250000", ProofInput: true,
		Meaning: "the context ceiling in tokens; the handoff trigger is the ceiling minus the margin (trigger 105000), and construction refuses a trigger above 106638"},
	{Key: ContextHandoffMarginTokensKey, Default: "145000", ProofInput: true,
		Meaning: "the handoff margin below the context ceiling, in tokens"},
	{Key: ContextToolGateModeKey, Default: "observe", ProofInput: true,
		Meaning: "the context tool gate: observe or deny"},
	{Key: "metasystem.governance.correlation-policy", Default: "C", ProofInput: true,
		Meaning: "the active correlation policy A, B or C; empty means no correlation policy may affect work"},
	{Key: "validation.weight-threshold", Default: "60", ProofInput: true,
		Meaning: "the change weight above which validation escalates"},
	{Key: "metasystem.counselor.brief-cadence-hours", Default: "24", ProofInput: true,
		Meaning: "hours between counselor briefs"},

	// Retro, refactor, watch and suite.
	{Key: "retro.max-receipts", Default: "25", ProofInput: true,
		Meaning: "receipts after which a retro is due"},
	{Key: "retro.max-age-days", Default: "30", ProofInput: true,
		Meaning: "days after which a retro is due"},
	{Key: "refactor.max-age-minutes", Default: "1440", ProofInput: true,
		Meaning: "the oldest a refactor baseline may be, in minutes"},
	{Key: "refactor.max-commits", Default: "40", ProofInput: true,
		Meaning: "the most commits a refactor baseline may be behind"},
	{Key: "watch.stale-min", Default: "20", ProofInput: true,
		Meaning: "minutes of silence after which a watched job reads as stale"},
	{Key: "watch.cap-min", Default: "180", ProofInput: true,
		Meaning: "the longest a watch waits, in minutes"},
	{Key: "watch.interval-sec", Default: "60", ProofInput: true,
		Meaning: "seconds between watch progress reports"},
	{Key: "suite.progress-silence-min", Default: "30", ProofInput: true,
		Meaning: "minutes a suite may go without progress before its watchdog acts"},
	{Key: "suite.section-cap-min", Default: "45", ProofInput: true,
		Meaning: "the longest one suite section may run, in minutes"},
	{Key: "suite.evidence-copy-timeout-sec", Default: "60", ProofInput: true,
		Meaning: "seconds allowed to copy a suite's evidence"},
	{Key: "suite.evidence-copy-max-mb", Default: "512", ProofInput: true,
		Meaning: "the most evidence copied from one suite, in megabytes"},
	{Key: "census.log-max-bytes", Default: "1048576", ProofInput: true,
		Meaning: "the census log's size bound in bytes"},
	{Key: "census.max-interval-share-percent", Default: "50", ProofInput: true,
		Meaning: "the share of its interval a census run may take, in percent"},

	// Dispatch.
	{Key: "dispatch.cap-min", Default: "120", ProofInput: true,
		Meaning: "the general dispatch reservation cap, in minutes"},
	{Key: "dispatch.cap-max", Default: "120", ProofInput: true,
		Meaning: "the bound on every reservation, so the goal pool stays a runaway guard (decision 14, R-58-m1)"},
	{Key: "dispatch.max-inline-input-kb", Default: "80", ProofInput: true,
		Meaning: "the most inline input one dispatch carries, in KiB (raised from 64 on 2026-09-20 on Wido's word)"},
	{Key: "capability.snapshot-max-age-days", Default: "30", ProofInput: true,
		Meaning: "the oldest a runtime capability snapshot may be, in days"},
	{Key: "landing.receipt-bound-min", Default: "40", ProofInput: true,
		Meaning: "the bound on a landing's tier-one receipt battery, in minutes"},
	{Key: LandingHumanFromTierKey, Default: "2",
		Meaning: "the lowest goal tier whose landing waits for a person's review or word (g1-s70); 4 lets every tier land by itself"},
	{Key: LandingAutoAfterKey, Default: "4h",
		Meaning: "how long a goal below that tier waits in Review before it is eligible to land by itself"},
	{Key: IntentReviewToolCallsKey, Default: "48", ProofInput: true,
		Meaning: "the independent read's tool calls for a public build whose brief names none"},

	// The delegate roster (Wido, 2026-09-26): every delegate runs on Claude.
	{Key: "role.default.runtime", Default: "claude", ProofInput: true, Runtime: "claude",
		Meaning: "the runtime a role runs on when it names none"},
	{Key: "role.default.model.claude", Default: "claude-opus-5-5", ProofInput: true, Runtime: "claude",
		Meaning: "the Claude model a role runs when it names none"},
	{Key: "role.design-critic.runtime", Default: "claude", ProofInput: true, Runtime: "claude",
		Meaning: "the design critic's runtime"},
	{Key: "role.design-critic.model.claude", Default: "claude-opus-5-5", ProofInput: true, Runtime: "claude",
		Meaning: "the design critic's Claude model"},
	{Key: "role.implementer.runtime", Default: "claude", ProofInput: true, Runtime: "claude",
		Meaning: "the implementer's runtime"},
	{Key: "role.implementer.model.claude", Default: "claude-opus-5-5", ProofInput: true, Runtime: "claude",
		Meaning: "the implementer's Claude model"},
	{Key: "mode.design.role.implementer.runtime", Default: "claude", ProofInput: true, Runtime: "claude",
		Meaning: "the implementer's runtime for design-bearing authoring"},
	{Key: "mode.design.role.implementer.model.claude", Default: "claude-fable-5-1", ProofInput: true, Runtime: "claude",
		Meaning: "the implementer's Claude model for design-bearing authoring"},
	{Key: "role.code-critic.runtime", Default: "claude", ProofInput: true, Runtime: "claude",
		Meaning: "the code critic's runtime, on a different effective model from the implementer"},
	{Key: "role.code-critic.model.claude", Default: "claude-fable-5-1", ProofInput: true, Runtime: "claude",
		Meaning: "the code critic's Claude model"},
	{Key: "role.verifier.runtime", Default: "main", ProofInput: true,
		Meaning: "the verifier runs in the main session"},
	{Key: "role.investigator.runtime", Default: "main", ProofInput: true,
		Meaning: "the investigator runs in the main session"},
	{Key: "dispatch.permissions.design-critic", Default: "critic", ProofInput: true,
		Meaning: "the design critic's permission profile"},
	{Key: "dispatch.permissions.code-critic", Default: "critic", ProofInput: true,
		Meaning: "the code critic's permission profile"},
	{Key: "dispatch.permissions.investigator", Default: "none", ProofInput: true,
		Meaning: "the investigator's permission profile"},
	{Key: "dispatch.permissions.implementer", Default: "workspace", ProofInput: true,
		Meaning: "the implementer's permission profile"},
	{Key: "dispatch.permissions.verifier", Default: "workspace", ProofInput: true,
		Meaning: "the verifier's permission profile"},
	{Key: "dispatch.transport.devin", Default: "acp", ProofInput: true, Runtime: "devin",
		Meaning: "the Devin transport (D82: ACP since 2026-08-24); legacy stays selectable"},

	// Launch lanes (R-123). No lane imposes a context window: 0 inherits the
	// runtime's own window for the model (Wido, 2026-09-20).
	{Key: "launch.seat.window.tokens", Default: "0", ProofInput: true,
		Meaning: "the seat's context window cap in tokens; 0 imposes none"},
	{Key: "launch.build.window.tokens", Default: "0", ProofInput: true,
		Meaning: "the build lane's context window cap in tokens; 0 imposes none"},
	{Key: "launch.design.window.tokens", Default: "0", ProofInput: true,
		Meaning: "the design lane's context window cap in tokens; 0 imposes none"},
	{Key: "launch.read.window.tokens", Default: "0", ProofInput: true,
		Meaning: "the read lane's context window cap in tokens; 0 imposes none"},
	{Key: "launch.build.runtime", Default: "claude", ProofInput: true,
		Meaning: "the build lane's runtime"},
	{Key: "launch.build.model", Default: "claude-opus-5-5", ProofInput: true,
		Meaning: "the build lane's model"},
	{Key: "launch.build.effort", Default: "xhigh", ProofInput: true,
		Meaning: "the build lane's effort, translated by the runtime adapter"},
	{Key: "launch.critique.runtime", Default: "claude", ProofInput: true,
		Meaning: "the critique lane's runtime"},
	{Key: "launch.critique.model", Default: "claude-opus-5-5", ProofInput: true,
		Meaning: "the critique lane's model, different from the author's"},
	{Key: "launch.design.runtime", Default: "claude", ProofInput: true,
		Meaning: "the design lane's runtime"},
	{Key: "launch.design.model", Default: "claude-fable-5-1", ProofInput: true,
		Meaning: "the design lane's model"},
	{Key: "launch.read.runtime", Default: "claude", ProofInput: true,
		Meaning: "the read lane's runtime"},
	{Key: "launch.read.model", Default: "claude-fable-5-1", ProofInput: true,
		Meaning: "the read lane's model"},
	{Key: "launch.wait.cap.seconds", Default: "240", ProofInput: true,
		Meaning: "the longest a launch waits, in seconds"},
	{Key: "launch.brief.admitted.tokens", Default: "120000", ProofInput: true,
		Meaning: "the largest brief admitted; no lane imposes a window, so it is sized against the SMALLEST window a lane runs in, Codex at model_context_window=258400: a full brief leaves about 138000 tokens for tool output and the diff"},
	{Key: "launch.build.max.changed.lines", Default: "1500", ProofInput: true,
		Meaning: "the most changed lines one build unit may carry"},
	{Key: "launch.read.split.diff.lines", Default: "1200", ProofInput: true,
		Meaning: "the diff size above which a read is split"},
	{Key: "launch.design.baseline.tokens", Default: "2432374", ProofInput: true,
		Meaning: "the design lane's measured token baseline"},
	{Key: "launch.design.baseline.requests", Default: "28", ProofInput: true,
		Meaning: "the design lane's measured request baseline"},
	{Key: "launch.design.baseline.peak.tokens", Default: "163000", ProofInput: true,
		Meaning: "the design lane's measured peak context baseline"},
}

var compiledSettingIndex = func() map[string]int {
	index := make(map[string]int, len(compiledSettings))
	for position, setting := range compiledSettings {
		index[setting.Key] = position
	}
	return index
}()

// CompiledSettings returns a copy of the compiled defaults table.
func CompiledSettings() []Setting {
	return append([]Setting(nil), compiledSettings...)
}

func compiledSetting(key string) (Setting, bool) {
	position, ok := compiledSettingIndex[key]
	if !ok {
		return Setting{}, false
	}
	return compiledSettings[position], true
}

// CompiledDefault returns key's static compiled default without judging its
// runtime binding. It is for readers that hold no configuration context.
func CompiledDefault(key string) (string, bool) {
	setting, ok := compiledSetting(key)
	if !ok || setting.Computed != "" {
		return "", false
	}
	return setting.Default, true
}

// MustDefault returns key's static compiled default and panics when the key
// has none: a reader that names a key the table does not hold is a defect.
func MustDefault(key string) string {
	value, ok := CompiledDefault(key)
	if !ok {
		panic("config: no compiled default for " + key)
	}
	return value
}

// uintDefault, int64Default and intDefault parse a numeric compiled default
// for the typed Default* values the owners export; a malformed table row is a
// defect caught at start.
func uintDefault(key string) uint64 {
	value, err := strconv.ParseUint(MustDefault(key), 10, 64)
	if err != nil {
		panic("config: compiled default " + key + " is not an unsigned integer")
	}
	return value
}

func int64Default(key string) int64 {
	value, err := strconv.ParseInt(MustDefault(key), 10, 64)
	if err != nil {
		panic("config: compiled default " + key + " is not an integer")
	}
	return value
}

func intDefault(key string) int {
	return int(int64Default(key))
}

// MustIntDefault returns key's compiled default as an int, for owners whose
// fallback is a number; the table is the one place the number lives.
func MustIntDefault(key string) int {
	return intDefault(key)
}

func floatDefault(key string) float64 {
	value, err := strconv.ParseFloat(MustDefault(key), 64)
	if err != nil {
		panic("config: compiled default " + key + " is not a number")
	}
	return value
}

// ProofInput reports whether key's resolved value is an input to the proof
// configuration digest. A key outside the table is, conservatively.
func ProofInput(key string) bool {
	setting, ok := compiledSetting(key)
	return !ok || setting.ProofInput
}

// selectedRuntimes reports whether runtime is in a metasystem.runtimes value.
func runtimeSelected(runtimes, runtime string) bool {
	for _, item := range strings.Split(runtimes, ",") {
		if strings.TrimSpace(item) == runtime {
			return true
		}
	}
	return false
}

// applicableDefault returns key's static default when it holds under the
// given runtime selection. runtimes is resolved only for a runtime-bound key.
func applicableDefault(key string, runtimes func() string) (string, bool) {
	setting, ok := compiledSetting(key)
	if !ok || setting.Computed != "" {
		return "", false
	}
	if setting.Runtime != "" && !runtimeSelected(runtimes(), setting.Runtime) {
		return "", false
	}
	return setting.Default, true
}

// effectiveRuntimes resolves metasystem.runtimes through Get's own layers.
func effectiveRuntimes(confPath string, lookupEnv func(string) (string, bool)) func() string {
	return func() string {
		value, _, err := Get(GetParams{Key: "metasystem.runtimes", ConfPath: confPath, LookupEnv: lookupEnv})
		if err != nil {
			return ""
		}
		return value
	}
}

// fileRuntimes resolves metasystem.runtimes from one file's content, or its
// compiled default: the committed layer a single-file reader sees.
func fileRuntimes(content string) func() string {
	return func() string {
		value, found := "", false
		parseSettings(content, func(_ int, key, val string, ok bool) {
			if ok && key == "metasystem.runtimes" {
				value, found = val, true
			}
		})
		if !found {
			value, _ = CompiledDefault("metasystem.runtimes")
		}
		return value
	}
}

// EffectiveCommittedContent renders a committed metasystem.conf's content
// with every applicable compiled default the file does not name appended,
// so a reader that parses the committed layer itself sees what the full
// shipped file showed.
func EffectiveCommittedContent(content string) string {
	present := map[string]bool{}
	parseSettings(content, func(_ int, key, _ string, ok bool) {
		if ok {
			present[key] = true
		}
	})
	runtimes := fileRuntimes(content)
	var builder strings.Builder
	builder.WriteString(content)
	if content != "" && !strings.HasSuffix(content, "\n") {
		builder.WriteString("\n")
	}
	for _, setting := range compiledSettings {
		if present[setting.Key] {
			continue
		}
		if value, ok := applicableDefault(setting.Key, runtimes); ok {
			builder.WriteString(setting.Key + "=" + value + "\n")
		}
	}
	return builder.String()
}
