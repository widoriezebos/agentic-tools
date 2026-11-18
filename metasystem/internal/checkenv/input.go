// Package checkenv owns the stable environment admitted to a proof.
package checkenv

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

const Version = 1

type Entry struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Source string `json:"source"`
}
type Input struct {
	Version int      `json:"version"`
	Entries []Entry  `json:"entries"`
	Dropped []string `json:"dropped,omitempty"`
}

// Supported names are explicit: a prefix is not evidence of toolchain input.
var hostNames = strings.Fields(`PATH HOME TMPDIR TMP TEMP LANG LANGUAGE LC_ALL LC_CTYPE LC_COLLATE LC_MESSAGES LC_MONETARY LC_NUMERIC LC_TIME TZ SYSTEMROOT JAVA_HOME STATICCHECK_CACHE XDG_CONFIG_HOME
 GODEBUG GO111MODULE GOARCH GOOS GOAMD64 GOARM GOARM64 GO386 GOMIPS GOMIPS64 GOPPC64 GORISCV64 GOWASM GOCACHE GOCACHEPROG GOMODCACHE GOTMPDIR GOPATH GOROOT GOFLAGS GOWORK GOENV GOTOOLCHAIN GOEXPERIMENT GOPROXY GOPRIVATE GONOPROXY GONOSUMDB GOSUMDB GOVCS GOAUTH GOTELEMETRY CGO_ENABLED CGO_CFLAGS CGO_CPPFLAGS CGO_CXXFLAGS CGO_FFLAGS CGO_LDFLAGS CGO_CFLAGS_ALLOW CGO_CFLAGS_DISALLOW CGO_CPPFLAGS_ALLOW CGO_CPPFLAGS_DISALLOW CGO_CXXFLAGS_ALLOW CGO_CXXFLAGS_DISALLOW CGO_FFLAGS_ALLOW CGO_FFLAGS_DISALLOW CGO_LDFLAGS_ALLOW CGO_LDFLAGS_DISALLOW CC CXX FC AR PKG_CONFIG SDKROOT DEVELOPER_DIR MACOSX_DEPLOYMENT_TARGET CPATH C_INCLUDE_PATH CPLUS_INCLUDE_PATH OBJC_INCLUDE_PATH LIBRARY_PATH HTTP_PROXY HTTPS_PROXY ALL_PROXY NO_PROXY http_proxy https_proxy all_proxy no_proxy`)

func HostName(name string) bool { return slices.Contains(hostNames, name) }

var validName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func hasURLUserInformation(value string) bool {
	for _, element := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '|' || unicode.IsSpace(r) }) {
		if prefix, rest, ok := strings.Cut(element, "://"); ok && !strings.Contains(prefix, "/") {
			element = rest
		}
		authority, _, _ := strings.Cut(element, "/")
		if strings.Contains(authority, "@") {
			return true
		}
	}
	return false
}

func validate(name, value string) error {
	if !validName.MatchString(name) || strings.ContainsRune(value, 0) {
		return fmt.Errorf("unsupported environment entry name: %s", name)
	}
	if name == "GOAUTH" && value != "off" && value != "netrc" {
		return fmt.Errorf("unsupported environment input: %s", name)
	}
	if (strings.HasPrefix(name, "GO") || strings.Contains(strings.ToUpper(name), "PROXY")) && hasURLUserInformation(value) {
		return fmt.Errorf("unsupported environment input: %s", name)
	}
	return nil
}

// Admit drops ambient input; caller input must instead classify custom names.
// Later assignments win, including deliberately empty values.
func Admit(environment, declared []string, ambient bool) (*Input, error) {
	input := &Input{Version: Version, Entries: []Entry{}}
	entries := map[string]Entry{}
	dropped := map[string]bool{}
	for _, group := range []struct {
		values   []string
		declared bool
	}{{environment, false}, {declared, true}} {
		for _, assignment := range group.values {
			name, value, ok := strings.Cut(assignment, "=")
			if ambient && !group.declared && !HostName(name) {
				dropped[name] = true
				continue
			}
			if !ok || !validName.MatchString(name) {
				return nil, fmt.Errorf("unsupported environment entry name: %s", name)
			}
			source := "host"
			if group.declared {
				source = "declared"
			} else if !HostName(name) {
				return nil, fmt.Errorf("environment input %s must move to declaredEnv or be removed", name)
			}
			if err := validate(name, value); err != nil {
				return nil, err
			}
			entries[name] = Entry{name, value, source}
		}
	}
	for _, entry := range entries {
		input.Entries = append(input.Entries, entry)
	}
	slices.SortFunc(input.Entries, func(a, b Entry) int { return strings.Compare(a.Name, b.Name) })
	for name := range dropped {
		input.Dropped = append(input.Dropped, name)
	}
	slices.Sort(input.Dropped)
	return input, nil
}

func (input *Input) Validate() error {
	if input == nil || input.Version != Version {
		return fmt.Errorf("unsupported environment policy version")
	}
	last := ""
	for _, entry := range input.Entries {
		if entry.Name <= last {
			return fmt.Errorf("environment entries must have unique sorted names")
		}
		if err := validate(entry.Name, entry.Value); err != nil {
			return err
		}
		if entry.Source != "declared" && (entry.Source != "host" || !HostName(entry.Name)) {
			return fmt.Errorf("unsupported environment provenance: %s", entry.Name)
		}
		last = entry.Name
	}
	for _, name := range input.Dropped {
		if strings.ContainsAny(name, "=\x00") {
			return fmt.Errorf("unsupported environment diagnostic name")
		}
	}
	return nil
}
func (input *Input) Environment() []string {
	result := []string{}
	if input != nil {
		for _, entry := range input.Entries {
			result = append(result, entry.Name+"="+entry.Value)
		}
	}
	return result
}

// Canonical returns an independent representation with explicit empty entries
// and sorted diagnostic names. Callers validate admitted records first.
func (input *Input) Canonical() *Input {
	if input == nil {
		return nil
	}
	result := *input
	result.Entries = append([]Entry{}, input.Entries...)
	result.Dropped = slices.Clone(input.Dropped)
	slices.Sort(result.Dropped)
	result.Dropped = slices.Compact(result.Dropped)
	return &result
}
func (input *Input) Stable() *Input {
	result := input.Canonical()
	if result != nil {
		result.Dropped = nil
	}
	return result
}

// Resolve never consults the parent's PATH. Relative PATH entries are resolved
// in the child's working directory, just as the child would resolve them.
func Resolve(program, directory string, environment []string) (string, error) {
	if filepath.IsAbs(program) {
		return program, nil
	}
	if strings.ContainsRune(program, filepath.Separator) {
		return filepath.Join(directory, program), nil
	}
	path, found := "", false
	for _, entry := range environment {
		if value, ok := strings.CutPrefix(entry, "PATH="); ok {
			path, found = value, true
		}
	}
	if !found {
		return "", fmt.Errorf("proof executable requires approved PATH")
	}
	for _, root := range filepath.SplitList(path) {
		if !filepath.IsAbs(root) {
			root = filepath.Join(directory, root)
		}
		candidate := filepath.Join(root, program)
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("proof executable is unavailable in approved PATH")
}
