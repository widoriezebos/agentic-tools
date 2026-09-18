package launch

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

const (
	SeatWindowKey     = "launch.seat.window.tokens"
	BuildWindowKey    = "launch.build.window.tokens"
	DesignWindowKey   = "launch.design.window.tokens"
	ReadWindowKey     = "launch.read.window.tokens"
	BuildModelKey     = "launch.build.model"
	BuildEffortKey    = "launch.build.effort"
	DesignModelKey    = "launch.design.model"
	ReadModelKey      = "launch.read.model"
	WaitCapKey        = "launch.wait.cap.seconds"
	BriefCapKey       = "launch.brief.admitted.tokens"
	BuildLinesCapKey  = "launch.build.max.changed.lines"
	ReadSplitLinesKey = "launch.read.split.diff.lines"
)

type Setting struct {
	Key, Value, Source string
}

type Settings struct {
	SeatWindow, BuildWindow, DesignWindow, ReadWindow int64
	BuildModel, BuildEffort, DesignModel, ReadModel   string
	WaitCapSeconds, BriefCap, BuildLinesCap           int64
	ReadSplitLines                                    int64
	Values                                            []Setting
}

var settingDefaults = []Setting{
	{SeatWindowKey, "200000", "default"}, {BuildWindowKey, "200000", "default"},
	{DesignWindowKey, "200000", "default"}, {ReadWindowKey, "400000", "default"},
	{BuildModelKey, "gpt-5.6-sol", "default"}, {BuildEffortKey, "xhigh", "default"},
	{DesignModelKey, "claude-fable-5-1", "default"}, {ReadModelKey, "claude-opus-5", "default"},
	{WaitCapKey, "240", "default"}, {BriefCapKey, "120000", "default"},
	{BuildLinesCapKey, "1500", "default"}, {ReadSplitLinesKey, "1200", "default"},
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
		result.Values = append(result.Values, Setting{definition.Key, value, source})
	}
	number := func(index int) (int64, error) {
		value, err := strconv.ParseInt(result.Values[index].Value, 10, 64)
		if err != nil || value < 1 {
			return 0, fmt.Errorf("LAUNCH_SETTING_INVALID key=%s", result.Values[index].Key)
		}
		return value, nil
	}
	var err error
	targets := []*int64{&result.SeatWindow, &result.BuildWindow, &result.DesignWindow, &result.ReadWindow}
	for index := range targets {
		if *targets[index], err = number(index); err != nil {
			return Settings{}, err
		}
	}
	result.BuildModel, result.BuildEffort = result.Values[4].Value, result.Values[5].Value
	result.DesignModel, result.ReadModel = result.Values[6].Value, result.Values[7].Value
	targets = []*int64{&result.WaitCapSeconds, &result.BriefCap, &result.BuildLinesCap, &result.ReadSplitLines}
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
