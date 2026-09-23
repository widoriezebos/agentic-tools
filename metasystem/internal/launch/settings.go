package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	WaitCapKey                  = "launch.wait.cap.seconds"
	BriefCapKey                 = "launch.brief.admitted.tokens"
	BuildLinesCapKey            = "launch.build.max.changed.lines"
	ReadSplitLinesKey           = "launch.read.split.diff.lines"
	DesignBaselineTokensKey     = "launch.design.baseline.tokens"
	DesignBaselineRequestsKey   = "launch.design.baseline.requests"
	DesignBaselinePeakTokensKey = "launch.design.baseline.peak.tokens"
	ShippedSeatWindowKey        = "launch.seat.window.shipped"
	ShippedClaudeSettingsSource = "scripts/enforcement/claude-code-hooks.json"
)

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
	WaitCapSeconds, BriefCap, BuildLinesCap           int64
	ReadSplitLines                                    int64
	DesignBaselineTokens, DesignBaselineRequests      int64
	DesignBaselinePeakTokens                          int64
	Values                                            []Setting
}

var settingDefaults = []Setting{
	{Key: SeatWindowKey, Value: "0", Source: "default"}, {Key: BuildWindowKey, Value: "0", Source: "default"},
	{Key: DesignWindowKey, Value: "0", Source: "default"}, {Key: ReadWindowKey, Value: "0", Source: "default"},
	{Key: BuildModelKey, Value: "gpt-6-sol", Source: "default"}, {Key: BuildEffortKey, Value: "xhigh", Source: "default"},
	{Key: DesignModelKey, Value: "claude-fable-5-1", Source: "default"}, {Key: ReadModelKey, Value: "claude-opus-5-5[1m]", Source: "default"},
	{Key: WaitCapKey, Value: "240", Source: "default"}, {Key: BriefCapKey, Value: "120000", Source: "default"},
	{Key: BuildLinesCapKey, Value: "1500", Source: "default"}, {Key: ReadSplitLinesKey, Value: "1200", Source: "default"},
	{Key: DesignBaselineTokensKey, Value: "2432374", Source: "default"}, {Key: DesignBaselineRequestsKey, Value: "28", Source: "default"},
	{Key: DesignBaselinePeakTokensKey, Value: "163000", Source: "default"},
}

func LoadShippedSeatWindow(moduleRoot string, configured int64) (ShippedSeatWindow, error) {
	path := filepath.Join(moduleRoot, filepath.FromSlash(ShippedClaudeSettingsSource))
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ShippedSeatWindow{Source: "absent", DiffersFromConf: configured != 0}, nil
	}
	if err != nil {
		return ShippedSeatWindow{}, err
	}
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

func DefaultSettings() Settings {
	settings, _ := resolveSettings("", func(string) (string, bool) { return "", false }, false)
	return settings
}

func ResolveSettings(confPath string, lookupEnv func(string) (string, bool)) (Settings, error) {
	return resolveSettings(confPath, lookupEnv, true)
}

func resolveSettings(confPath string, lookupEnv func(string) (string, bool), useConf bool) (Settings, error) {
	var result Settings
	for _, definition := range settingDefaults {
		value, source := definition.Value, "default"
		if useConf {
			params := config.GetParams{Key: definition.Key, ConfPath: confPath, Default: definition.Value, DefaultSet: true, LookupEnv: lookupEnv}
			resolved, _, err := config.Get(params)
			if err != nil {
				return Settings{}, fmt.Errorf("LAUNCH_SETTING_INVALID key=%s: %w", definition.Key, err)
			}
			value = resolved
			source, err = config.KeyOrigin(params)
			if err != nil {
				return Settings{}, fmt.Errorf("LAUNCH_SETTING_INVALID key=%s: %w", definition.Key, err)
			}
		}
		if strings.TrimSpace(value) == "" {
			return Settings{}, fmt.Errorf("LAUNCH_SETTING_INVALID key=%s", definition.Key)
		}
		result.Values = append(result.Values, Setting{Key: definition.Key, Value: value, Source: source})
	}
	number := func(index int) (int64, error) {
		value, err := strconv.ParseInt(result.Values[index].Value, 10, 64)
		if err != nil || value < 1 {
			return 0, fmt.Errorf("LAUNCH_SETTING_INVALID key=%s", result.Values[index].Key)
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
			return 0, fmt.Errorf("LAUNCH_SETTING_INVALID key=%s", result.Values[index].Key)
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
	targets = []*int64{&result.WaitCapSeconds, &result.BriefCap, &result.BuildLinesCap, &result.ReadSplitLines,
		&result.DesignBaselineTokens, &result.DesignBaselineRequests, &result.DesignBaselinePeakTokens}
	for offset := range targets {
		if *targets[offset], err = number(offset + 8); err != nil {
			return Settings{}, err
		}
	}
	return result, nil
}

func (s Settings) launchValues(kind string) (string, string, int64) {
	switch kind {
	case "build", "critique":
		return s.BuildModel, s.BuildEffort, s.BuildWindow
	case "design":
		return s.DesignModel, s.BuildEffort, s.DesignWindow
	case "read":
		return s.ReadModel, s.BuildEffort, s.ReadWindow
	default:
		return "", "", 0
	}
}
