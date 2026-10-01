package launch

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

const (
	SeatWindowKey               = "launch.seat.window.tokens"
	BuildWindowKey              = "launch.build.window.tokens"
	DesignWindowKey             = "launch.design.window.tokens"
	ReadWindowKey               = "launch.read.window.tokens"
	BuildModelKey               = "launch.build.model"
	BuildEffortKey              = "launch.build.effort"
	DesignModelKey              = "launch.design.model"
	ReadModelKey                = "launch.read.model"
	CritiqueModelKey            = "launch.critique.model"
	BuildRuntimeKey             = "launch.build.runtime"
	CritiqueRuntimeKey          = "launch.critique.runtime"
	DesignRuntimeKey            = "launch.design.runtime"
	ReadRuntimeKey              = "launch.read.runtime"
	WaitCapKey                  = "launch.wait.cap.seconds"
	BriefCapKey                 = "launch.brief.admitted.tokens"
	BuildLinesCapKey            = "launch.build.max.changed.lines"
	ReadSplitLinesKey           = "launch.read.split.diff.lines"
	DesignBaselineTokensKey     = "launch.design.baseline.tokens"
	DesignBaselineRequestsKey   = "launch.design.baseline.requests"
	DesignBaselinePeakTokensKey = "launch.design.baseline.peak.tokens"
	ShippedSeatWindowKey        = "launch.seat.window.shipped"
	ShippedClaudeSettingsSource = "internal/runtimes/enforcement/claude-code-hooks.json"
	SeatRuntimeKey              = "launch.seat.runtime"
	SeatModelKey                = "launch.seat.model"
	SeatEffortKey               = "launch.seat.effort"
	LandingRuntimeKey           = "launch.landing.runtime"
	LandingModelKey             = "launch.landing.model"
	LandingEffortKey            = "launch.landing.effort"
	LandingDailyTokensKey       = "launch.landing.daily.tokens"
)

// SeatOwnerLineage is the owner lineage every steward-started seat main runs
// under, one per installation, so a successor seat succeeds a dead
// predecessor's lease and claim by the lease's own succession rule.
const SeatOwnerLineage = "steward-seat"

// SeatRuntimeOff is launch.seat.runtime's default: the steward starts no
// seat until an installation turns it on (Amendment 1).
const SeatRuntimeOff = config.SeatRuntimeOff

// LandingModelUnbound is launch.landing.model where the installation binds
// no model for the landing agent's runtime: a landing start there is
// refused, and every other setting still resolves.
const LandingModelUnbound = "unbound"

type Setting struct {
	Key, Value, Source     string
	ShippedDiffersFromConf *bool `json:"shippedDiffersFromConf,omitempty"`
}

type ShippedSeatWindow struct {
	Tokens          int64
	Source          string
	DiffersFromConf bool
}

type Settings struct {
	SeatWindow, BuildWindow, DesignWindow, ReadWindow int64
	BuildModel, BuildEffort, DesignModel, ReadModel   string
	CritiqueModel                                     string
	BuildRuntime, CritiqueRuntime, DesignRuntime      string
	ReadRuntime                                       string
	WaitCapSeconds, BriefCap, BuildLinesCap           int64
	ReadSplitLines                                    int64
	DesignBaselineTokens, DesignBaselineRequests      int64
	DesignBaselinePeakTokens                          int64
	SeatRuntime, SeatModel, SeatEffort                string
	LandingRuntime, LandingModel, LandingEffort       string
	Values                                            []Setting
}

// settingDefaults are the launch settings in their read order; each value
// is the compiled default (config.CompiledSettings), which lives once.
var settingDefaults = []Setting{
	{Key: SeatWindowKey, Value: config.MustDefault(SeatWindowKey), Source: "default"}, {Key: BuildWindowKey, Value: config.MustDefault(BuildWindowKey), Source: "default"},
	{Key: DesignWindowKey, Value: config.MustDefault(DesignWindowKey), Source: "default"}, {Key: ReadWindowKey, Value: config.MustDefault(ReadWindowKey), Source: "default"},
	{Key: BuildModelKey, Value: config.MustDefault(BuildModelKey), Source: "default"}, {Key: BuildEffortKey, Value: config.MustDefault(BuildEffortKey), Source: "default"},
	{Key: DesignModelKey, Value: config.MustDefault(DesignModelKey), Source: "default"}, {Key: ReadModelKey, Value: config.MustDefault(ReadModelKey), Source: "default"},
	{Key: WaitCapKey, Value: config.MustDefault(WaitCapKey), Source: "default"}, {Key: BriefCapKey, Value: config.MustDefault(BriefCapKey), Source: "default"},
	{Key: BuildLinesCapKey, Value: config.MustDefault(BuildLinesCapKey), Source: "default"}, {Key: ReadSplitLinesKey, Value: config.MustDefault(ReadSplitLinesKey), Source: "default"},
	{Key: DesignBaselineTokensKey, Value: config.MustDefault(DesignBaselineTokensKey), Source: "default"}, {Key: DesignBaselineRequestsKey, Value: config.MustDefault(DesignBaselineRequestsKey), Source: "default"},
	{Key: DesignBaselinePeakTokensKey, Value: config.MustDefault(DesignBaselinePeakTokensKey), Source: "default"},
	// The roster proper (R-123): every lane names its agent and its model,
	// appended after the index reads above so those hold. The critique lane
	// has a model of its own, because the author and the critic are
	// different models; and a lane names its runtime because a model name is
	// not an agent — more than one agent can serve the same model.
	{Key: CritiqueModelKey, Value: config.MustDefault(CritiqueModelKey), Source: "default"},
	{Key: BuildRuntimeKey, Value: config.MustDefault(BuildRuntimeKey), Source: "default"}, {Key: CritiqueRuntimeKey, Value: config.MustDefault(CritiqueRuntimeKey), Source: "default"},
	{Key: DesignRuntimeKey, Value: config.MustDefault(DesignRuntimeKey), Source: "default"}, {Key: ReadRuntimeKey, Value: config.MustDefault(ReadRuntimeKey), Source: "default"},
	// The seat a steward starts headless (seat-works-without-a-person
	// D-seat): its runtime, model and effort resolve as a lane's.
	{Key: SeatRuntimeKey, Value: config.MustDefault(SeatRuntimeKey), Source: "default"},
	{Key: SeatModelKey, Value: config.MustDefault(SeatModelKey), Source: "default"},
	{Key: SeatEffortKey, Value: config.MustDefault(SeatEffortKey), Source: "default"},
	// The landing agent (landing-lane-runtime-redesign D2): roster keys that
	// resolve as a lane's; its model follows the build lane's binding for its
	// runtime unless launch.landing.model names one for every runtime.
	{Key: LandingRuntimeKey, Value: config.MustDefault(LandingRuntimeKey), Source: "default"},
	{Key: LandingModelKey, Value: config.MustDefault(LandingModelKey), Source: "default"},
	{Key: LandingEffortKey, Value: config.MustDefault(LandingEffortKey), Source: "default"},
}

// LoadShippedSeatWindow reads the seat window the engine's shipped Claude
// settings (runtimes.ShippedEnforcement("claude")) impose.
func LoadShippedSeatWindow(data []byte, configured int64) (ShippedSeatWindow, error) {
	var source struct {
		AutoCompactWindow *int64 `json:"autoCompactWindow"`
	}
	if err := json.Unmarshal(data, &source); err != nil {
		return ShippedSeatWindow{}, fmt.Errorf("read shipped Claude settings: %w", err)
	}
	if source.AutoCompactWindow == nil {
		return ShippedSeatWindow{Source: "absent", DiffersFromConf: configured != 0}, nil
	}
	return ShippedSeatWindow{Tokens: *source.AutoCompactWindow, Source: ShippedClaudeSettingsSource, DiffersFromConf: *source.AutoCompactWindow != configured}, nil
}

// DefaultSettings are the compiled defaults on no host: every auto lane takes
// the first runtime metasystem.runtimes lists, since no PATH is searched.
func DefaultSettings() Settings {
	settings, _ := resolveSettings("", func(string) (string, bool) { return "", false })
	return settings
}

func ResolveSettings(confPath string, lookupEnv func(string) (string, bool)) (Settings, error) {
	return resolveSettings(confPath, lookupEnv)
}

// laneModelKeys are each lane's runtime-independent model key; the model a
// lane's resolved runtime binds is the same key with the runtime appended.
var laneModelKeys = map[string]string{BuildRuntimeKey: BuildModelKey, CritiqueRuntimeKey: CritiqueModelKey,
	DesignRuntimeKey: DesignModelKey, ReadRuntimeKey: ReadModelKey, SeatRuntimeKey: SeatModelKey,
	LandingRuntimeKey: LandingModelKey}

// laneModelOrder is the order the lanes' models resolve in.
var laneModelOrder = []string{BuildRuntimeKey, CritiqueRuntimeKey, DesignRuntimeKey, ReadRuntimeKey, SeatRuntimeKey, LandingRuntimeKey}

// boundModelPrefix is the key whose runtime-bound form a lane's empty model
// takes, when it is not the lane's own: the seat and the landing agent have
// no per-runtime model keys and run the build lane's model for their runtime
// (claude on Opus by the roster's default).
var boundModelPrefix = map[string]string{SeatModelKey: BuildModelKey, LandingModelKey: BuildModelKey}

func resolveSettings(confPath string, lookupEnv func(string) (string, bool)) (Settings, error) {
	resolve := func(key string) (string, string, error) {
		params := config.GetParams{Key: key, ConfPath: confPath, LookupEnv: lookupEnv}
		value, _, err := config.Get(params)
		if err != nil {
			return "", "", invalidSetting(key, err)
		}
		source, err := config.KeyOrigin(params)
		if err != nil {
			return "", "", invalidSetting(key, err)
		}
		return value, source, nil
	}
	resolved := map[string]Setting{}
	for _, definition := range settingDefaults {
		if _, model := laneModelKeyOf(definition.Key); model {
			continue
		}
		value, source, err := resolve(definition.Key)
		if err != nil {
			return Settings{}, err
		}
		if config.RuntimeSelectionKey(definition.Key) {
			if raw, _, rawErr := config.Get(config.GetParams{Key: definition.Key, ConfPath: confPath, LookupEnv: lookupEnv, KeepAuto: true}); rawErr == nil && raw == config.AutoRuntime {
				choice, choiceErr := config.ResolveAutoRuntime(confPath, lookupEnv)
				if choiceErr != nil {
					return Settings{}, invalidSetting(definition.Key, choiceErr)
				}
				source += "; " + choice.Describe()
			}
		}
		resolved[definition.Key] = Setting{Key: definition.Key, Value: value, Source: source}
	}
	// A lane's model follows the lane's resolved runtime unless the
	// runtime-independent key names one for every runtime. The lanes resolve
	// in a fixed order, the seat last, so a model missing for the build lane
	// is refused under the build lane's key even though the seat borrows it.
	for _, runtimeKey := range laneModelOrder {
		modelKey := laneModelKeys[runtimeKey]
		value, source, err := resolve(modelKey)
		if err != nil {
			return Settings{}, err
		}
		// A seat that is off runs no model: nothing is bound for it.
		if runtimeKey == SeatRuntimeKey && resolved[runtimeKey].Value == config.SeatRuntimeOff && strings.TrimSpace(value) == "" {
			resolved[modelKey] = Setting{Key: modelKey, Value: config.SeatRuntimeOff, Source: source + "; " + SeatRuntimeKey + "=" + config.SeatRuntimeOff}
			continue
		}
		if strings.TrimSpace(value) == "" {
			prefix := modelKey
			if other, ok := boundModelPrefix[modelKey]; ok {
				prefix = other
			}
			bound := prefix + "." + resolved[runtimeKey].Value
			boundValue, boundSource, boundErr := resolve(bound)
			// The landing agent is not every installation's: one whose
			// runtimes bind no model for it keeps its other settings, and a
			// landing start there is refused (LandingModelUnbound).
			if runtimeKey == LandingRuntimeKey && (boundErr != nil || strings.TrimSpace(boundValue) == "") {
				resolved[modelKey] = Setting{Key: modelKey, Value: LandingModelUnbound, Source: source + "; " + bound + " is not set here"}
				continue
			}
			if value, source, err = boundValue, boundSource, boundErr; err != nil {
				return Settings{}, err
			}
			source += " via " + bound
			if strings.TrimSpace(value) == "" {
				return Settings{}, coded("LAUNCH_SETTING_INVALID", "key="+modelKey,
					fmt.Errorf("no model is set for the %s lane; set %s or %s", resolved[runtimeKey].Value, modelKey, bound))
			}
		}
		resolved[modelKey] = Setting{Key: modelKey, Value: value, Source: source}
	}
	var result Settings
	for _, definition := range settingDefaults {
		setting := resolved[definition.Key]
		if strings.TrimSpace(setting.Value) == "" {
			return Settings{}, invalidSetting(definition.Key, errors.New("it is empty"))
		}
		result.Values = append(result.Values, setting)
	}
	number := func(index int) (int64, error) {
		value, err := strconv.ParseInt(result.Values[index].Value, 10, 64)
		if err != nil || value < 1 {
			return 0, invalidSetting(result.Values[index].Key, errors.New("it is not a positive number"))
		}
		return value, nil
	}
	// A window of 0 means NO CAP: the lane inherits whatever context window its
	// runtime gives that model, and neither adapter emits a cap argument. That
	// is the only way to express "do not impose a number", which matters
	// because the right number is per-model and this key is absolute tokens —
	// 400000 is generous on a 1000000-token Fable lane and impossible on Codex,
	// whose model_context_window is 258400. A negative stays invalid: that is a
	// bug, not an intent.
	windowNumber := func(index int) (int64, error) {
		value, err := strconv.ParseInt(result.Values[index].Value, 10, 64)
		if err != nil || value < 0 {
			return 0, invalidSetting(result.Values[index].Key, errors.New("it is not zero or a positive number"))
		}
		return value, nil
	}
	var err error
	targets := []*int64{&result.SeatWindow, &result.BuildWindow, &result.DesignWindow, &result.ReadWindow}
	for index := range targets {
		if *targets[index], err = windowNumber(index); err != nil {
			return Settings{}, err
		}
	}
	result.BuildModel, result.BuildEffort = result.Values[4].Value, result.Values[5].Value
	result.DesignModel, result.ReadModel = result.Values[6].Value, result.Values[7].Value
	for _, setting := range result.Values {
		switch setting.Key {
		case CritiqueModelKey:
			result.CritiqueModel = setting.Value
		case BuildRuntimeKey:
			result.BuildRuntime = setting.Value
		case CritiqueRuntimeKey:
			result.CritiqueRuntime = setting.Value
		case DesignRuntimeKey:
			result.DesignRuntime = setting.Value
		case ReadRuntimeKey:
			result.ReadRuntime = setting.Value
		case SeatRuntimeKey:
			result.SeatRuntime = setting.Value
		case SeatModelKey:
			result.SeatModel = setting.Value
		case SeatEffortKey:
			result.SeatEffort = setting.Value
		case LandingRuntimeKey:
			result.LandingRuntime = setting.Value
		case LandingModelKey:
			result.LandingModel = setting.Value
		case LandingEffortKey:
			result.LandingEffort = setting.Value
		}
	}
	targets = []*int64{&result.WaitCapSeconds, &result.BriefCap, &result.BuildLinesCap, &result.ReadSplitLines,
		&result.DesignBaselineTokens, &result.DesignBaselineRequests, &result.DesignBaselinePeakTokens}
	for offset := range targets {
		if *targets[offset], err = number(offset + 8); err != nil {
			return Settings{}, err
		}
	}
	return result, nil
}

func laneModelKeyOf(key string) (string, bool) {
	for runtimeKey, modelKey := range laneModelKeys {
		if key == modelKey {
			return runtimeKey, true
		}
	}
	return "", false
}

// launchRuntime is the agent a lane runs on, by the lane's own setting.
func (s Settings) launchRuntime(kind string) string {
	switch kind {
	case "build":
		return s.BuildRuntime
	case "critique":
		return s.CritiqueRuntime
	case "design":
		return s.DesignRuntime
	case "read":
		return s.ReadRuntime
	case "seat":
		return s.SeatRuntime
	case "landing":
		return s.LandingRuntime
	default:
		return ""
	}
}

func (s Settings) launchValues(kind string) (string, string, int64) {
	switch kind {
	case "build":
		return s.BuildModel, s.BuildEffort, s.BuildWindow
	case "critique":
		return s.CritiqueModel, s.BuildEffort, s.BuildWindow
	case "design":
		return s.DesignModel, s.BuildEffort, s.DesignWindow
	case "read":
		return s.ReadModel, s.BuildEffort, s.ReadWindow
	case "seat":
		return s.SeatModel, s.SeatEffort, s.SeatWindow
	case "landing":
		// The landing agent imposes no context window: 0 inherits the
		// runtime's own window for the model.
		return s.LandingModel, s.LandingEffort, 0
	default:
		return "", "", 0
	}
}

// invalidSetting is the refusal of a launch setting that cannot be used: the
// key and why, with the command that shows the settings.
func invalidSetting(key string, cause error) error {
	return coded("LAUNCH_SETTING_INVALID", "key="+key, fmt.Errorf("the setting %s cannot be used: %w; see metasystem settings show", key, cause))
}
