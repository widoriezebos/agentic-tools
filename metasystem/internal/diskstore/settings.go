package diskstore

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// Settings are one checkout's disk-lifetime settings (3.13): every key read
// through config.Get with its compiled-in default, each validated. A
// settings file that cannot be read, or a value that does not validate, is
// an error: the caller treats that checkout's settings as Unknown and acts
// on nothing that depends on them (R24); no default stands in.
type Settings struct {
	ConfPath     string
	Values       map[string]string
	Sources      map[string]string
	EvidenceRoot config.EvidenceRoot
}

// LoadSettings reads every key of 3.13 for the checkout whose
// metasystem.conf is confPath. lookupEnv nil is os.LookupEnv.
func LoadSettings(confPath string, lookupEnv func(string) (string, bool)) (Settings, error) {
	settings := Settings{ConfPath: confPath, Values: map[string]string{}, Sources: map[string]string{}}
	for _, row := range config.DiskSettings() {
		if row.Key == config.EvidenceRootKey {
			root, err := config.ResolveEvidenceRoot(config.EvidenceRootParams{ConfPath: confPath, LookupEnv: lookupEnv})
			if err != nil {
				return Settings{}, fmt.Errorf("settings of %s: %w", confPath, err)
			}
			settings.EvidenceRoot = root
			settings.Values[row.Key], settings.Sources[row.Key] = root.Path, root.Origin
			continue
		}
		params := config.GetParams{Key: row.Key, ConfPath: confPath, Default: row.Default, DefaultSet: true, LookupEnv: lookupEnv}
		value, code, err := config.Get(params)
		if err != nil || code != 0 {
			if err == nil {
				err = fmt.Errorf("exit %d", code)
			}
			return Settings{}, fmt.Errorf("settings of %s unreadable at %s: %w", confPath, row.Key, err)
		}
		if err := config.ValidateDiskSetting(row.Key, value); err != nil {
			return Settings{}, fmt.Errorf("settings of %s: %w", confPath, err)
		}
		source, err := config.KeyOrigin(params)
		if err != nil {
			return Settings{}, fmt.Errorf("settings of %s: %w", confPath, err)
		}
		settings.Values[row.Key], settings.Sources[row.Key] = strings.TrimSpace(value), source
	}
	return settings, nil
}

// DefaultSettings are the compiled-in values alone, for a pass that has no
// checkout of its own and for tests.
func DefaultSettings() Settings {
	settings := Settings{Values: map[string]string{}, Sources: map[string]string{}}
	for _, row := range config.DiskSettings() {
		settings.Values[row.Key], settings.Sources[row.Key] = row.Default, "default"
	}
	return settings
}

// number reads a numeric key. Values come only from LoadSettings, which
// validated them, or from the compiled defaults; a key missing from a
// hand-built Settings reads as its compiled default.
func (s Settings) number(key string) int64 {
	value, err := strconv.ParseInt(s.Values[key], 10, 64)
	if err != nil {
		row, _ := config.DiskSettingFor(key)
		value, _ = strconv.ParseInt(row.Default, 10, 64)
	}
	return value
}

// Bytes reads a GiB or MiB key in bytes.
func (s Settings) Bytes(key string) int64 {
	row, _ := config.DiskSettingFor(key)
	switch row.Unit {
	case config.UnitGiB:
		return s.number(key) << 30
	case config.UnitMiB:
		return s.number(key) << 20
	}
	return 0
}

// Duration reads a days, hours or seconds key.
func (s Settings) Duration(key string) time.Duration {
	row, _ := config.DiskSettingFor(key)
	switch row.Unit {
	case config.UnitDays:
		return time.Duration(s.number(key)) * 24 * time.Hour
	case config.UnitHours:
		return time.Duration(s.number(key)) * time.Hour
	case config.UnitSec:
		return time.Duration(s.number(key)) * time.Second
	}
	return 0
}

// Count reads a count key.
func (s Settings) Count(key string) int { return int(s.number(key)) }

// Participant is one armed checkout's settings for the host resolution; Err
// set means they could not be read.
type Participant struct {
	Checkout string
	Settings Settings
	Err      error
}

// HostSettings are the host-scoped keys in force for one machine pass.
type HostSettings struct {
	Values map[string]string
	// Conflicts are the report's "settings conflict" lines.
	Conflicts []string
	// Unknown names each participant whose settings could not be read; when
	// it is not empty, every host-scoped key is Unknown for the pass and
	// Values is empty (DL4E-08): a value from the others never stands in.
	Unknown []string
}

// Known reports whether the host policy can be acted on this pass.
func (h HostSettings) Known() bool { return len(h.Unknown) == 0 && h.Values != nil }

// Duration and Bytes read a host key in force; the caller checks Known.
func (h HostSettings) settings() Settings {
	return Settings{Values: h.Values}
}
func (h HostSettings) Duration(key string) time.Duration { return h.settings().Duration(key) }
func (h HostSettings) Bytes(key string) int64            { return h.settings().Bytes(key) }
func (h HostSettings) Count(key string) int              { return h.settings().Count(key) }

// ResolveHost resolves every host-scoped key over the participants (3.12
// "Settings scope", DL4D-09): agreeing values stand; differing values
// resolve to the most conservative one named in the key's row, reported as
// a conflict; an unreadable participant makes the whole host policy
// Unknown. With no participant the compiled defaults stand.
func ResolveHost(participants []Participant) HostSettings {
	host := HostSettings{}
	for _, participant := range participants {
		if participant.Err != nil {
			host.Unknown = append(host.Unknown, fmt.Sprintf("host settings unknown: %s unreadable: %v; run metasystem settings check there", participant.Checkout, participant.Err))
		}
	}
	if len(host.Unknown) != 0 {
		return host
	}
	host.Values = map[string]string{}
	for _, row := range config.DiskSettings() {
		if row.Scope != config.ScopeHost {
			continue
		}
		if len(participants) == 0 {
			host.Values[row.Key] = row.Default
			continue
		}
		type seen struct {
			value     int64
			checkouts []string
		}
		values := map[int64]*seen{}
		for _, participant := range participants {
			value := participant.Settings.number(row.Key)
			if values[value] == nil {
				values[value] = &seen{value: value}
			}
			values[value].checkouts = append(values[value].checkouts, participant.Checkout)
		}
		var ordered []int64
		for value := range values {
			ordered = append(ordered, value)
		}
		sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
		chosen := ordered[len(ordered)-1]
		if row.Conservative == config.ConservativeSmallest {
			chosen = ordered[0]
		}
		host.Values[row.Key] = strconv.FormatInt(chosen, 10)
		if len(ordered) > 1 {
			var parts []string
			for _, value := range ordered {
				parts = append(parts, fmt.Sprintf("%d (%s)", value, strings.Join(values[value].checkouts, ", ")))
			}
			host.Conflicts = append(host.Conflicts, fmt.Sprintf("settings conflict: %s %s: %d in force (the %s)",
				row.Key, strings.Join(parts, ", "), chosen, row.Conservative))
		}
	}
	return host
}
