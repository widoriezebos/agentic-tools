package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
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
// mechanically derived environment variable, the .local override, the
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
	var matches []string
	for _, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		name, val, _ := strings.Cut(line, "=")
		if strings.TrimSpace(name) == key {
			matches = append(matches, strings.TrimSpace(val))
		}
	}
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
		for _, raw := range strings.Split(string(content), "\n") {
			line := strings.TrimSpace(raw)
			if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
				continue
			}
			key := strings.TrimSpace(strings.SplitN(line, "=", 2)[0])
			if strings.HasPrefix(key, prefix) {
				add(key)
			}
		}
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
