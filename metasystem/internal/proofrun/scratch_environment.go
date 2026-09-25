package proofrun

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// ScratchEnvironmentPolicy names the managed group layout. Changing a
// generated path, variable or token bumps it, which changes every managed
// group's environment identity.
const ScratchEnvironmentPolicy = "scratch-environment/v1"

const (
	scratchEnvironmentDir = ".environment"
	scratchGoEnvFile      = "env"
	scratchGoEnvMaxBytes  = 1 << 20
	goEnvOff              = "off"
	// goEnvDefault: the caller's GOENV is unset or empty, so each group's
	// default config file is resolved from its own pre-managed HOME and
	// XDG_CONFIG_HOME (defaultGoEnvPaths) rather than one run-wide snapshot.
	goEnvDefault = "default"
)

// ScratchEnvironment is the launcher-prepared description of one run's
// managed group environments. It travels inside the authenticated request;
// only the paths it names are normalized in identity, never a value an
// environment variable claims for itself.
type ScratchEnvironment struct {
	Policy string `json:"policy"`
	Run    string `json:"run"`
	Root   string `json:"root"`
	// GoEnv is "file" when the inherited Go env file was copied to
	// <root>/goenv/env, or "off" when the caller ran with GOENV=off.
	GoEnv       string                    `json:"goEnv"`
	GoEnvDigest string                    `json:"goEnvDigest,omitempty"`
	Groups      []ScratchEnvironmentGroup `json:"groups"`
}

type ScratchEnvironmentValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ScratchEnvironmentGroup names a prepared group; DeclaredGoEnvDigest binds
// the contents of a GOENV file the group itself declares.
type ScratchEnvironmentGroup struct {
	ID                  string `json:"id"`
	DeclaredGoEnvDigest string `json:"declaredGoEnvDigest,omitempty"`
	// DefaultGoEnv records a declared empty GOENV, which go resolves to the
	// default config file: "managed" when that location now lies in the group's
	// tree and holds a snapshot of the pre-managed default file, "external"
	// when a declared HOME or XDG_CONFIG_HOME keeps it at the user's own file.
	DefaultGoEnv       string `json:"defaultGoEnv,omitempty"`
	DefaultGoEnvDigest string `json:"defaultGoEnvDigest,omitempty"`
	// Dependencies are the module-store locations this group's Go would have
	// used before its HOME was managed; they stay literal in identity.
	Dependencies []ScratchEnvironmentValue `json:"dependencies,omitempty"`
}

// declarable names keep a group-declared value literally, outside cleanup.
var scratchDeclarable = []string{"HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR", "GOENV"}

// runnerOwned cache and temp locations always point into the run root.
var scratchRunnerOwned = []string{"XDG_CACHE_HOME", "TMPDIR", "TMP", "TEMP", "GOTMPDIR", "GOCACHE", "STATICCHECK_CACHE"}

func (e *ScratchEnvironment) groupDir(group string) string {
	return filepath.Join(e.Root, "groups", filepath.FromSlash(group), scratchEnvironmentDir)
}

func (e *ScratchEnvironment) goEnvPath() string {
	return filepath.Join(e.Root, "goenv", scratchGoEnvFile)
}

func (e *ScratchEnvironment) group(id string) (ScratchEnvironmentGroup, bool) {
	for _, group := range e.Groups {
		if group.ID == id {
			return group, true
		}
	}
	return ScratchEnvironmentGroup{}, false
}

// managedValues are the exact generated values for one group. GOENV is
// present only for the file mode; GOENV=off is a literal value.
func (e *ScratchEnvironment) managedValues(group string) map[string]string {
	dir := e.groupDir(group)
	home := filepath.Join(dir, "home")
	values := map[string]string{
		"HOME":              home,
		"XDG_CONFIG_HOME":   filepath.Join(home, ".config"),
		"XDG_CACHE_HOME":    filepath.Join(home, ".cache"),
		"XDG_DATA_HOME":     filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME":    filepath.Join(home, ".local", "state"),
		"XDG_RUNTIME_DIR":   filepath.Join(dir, "runtime"),
		"TMPDIR":            filepath.Join(dir, "tmp"),
		"TMP":               filepath.Join(dir, "tmp"),
		"TEMP":              filepath.Join(dir, "tmp"),
		"GOTMPDIR":          filepath.Join(dir, "tmp"),
		"GOCACHE":           filepath.Join(e.Root, "gocache"),
		"STATICCHECK_CACHE": filepath.Join(e.Root, "staticcheck"),
	}
	if e.GoEnv == "file" {
		values["GOENV"] = e.goEnvPath()
	}
	return values
}

func (e *ScratchEnvironment) groupDirectories(group string) []string {
	values := e.managedValues(group)
	dirs := []string{e.groupDir(group)}
	for _, name := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR", "TMPDIR"} {
		dirs = append(dirs, values[name])
	}
	return dirs
}

// overlayScratchEnvironment points a group's home, config, cache and temp
// variables at its run-private tree. A group-declared HOME, config or GOENV
// wins and stays literal; cache and temp variables are runner-owned.
func overlayScratchEnvironment(environment []string, descriptor *ScratchEnvironment, group testpolicy.Group) []string {
	values := descriptor.managedValues(group.ID)
	overlay := map[string]string{}
	for _, name := range scratchRunnerOwned {
		overlay[name] = values[name]
	}
	for _, name := range scratchDeclarable {
		if _, declared := group.Env[name]; declared {
			continue
		}
		if name == "GOENV" && descriptor.GoEnv == goEnvOff {
			overlay[name] = goEnvOff
		} else if value, ok := values[name]; ok {
			overlay[name] = value
		}
	}
	prepared, _ := descriptor.group(group.ID)
	for _, dependency := range prepared.Dependencies {
		overlay[dependency.Name] = dependency.Value
	}
	return overlayTestEnvironment(environment, overlay)
}

// digestScratchGroupEnvironment is the group environment identity. Without a
// descriptor it is the legacy digest. With one, only a value equal to the
// descriptor's generated path for this group digests as a stable token, and
// the policy version and Go env contents join the identity.
func digestScratchGroupEnvironment(request TestRunRequest, group testpolicy.Group, environment []string) string {
	descriptor := request.ScratchEnvironment
	if descriptor == nil {
		return digestGroupEnvironment(group, environment)
	}
	values := descriptor.managedValues(group.ID)
	normalized := make([]string, 0, len(environment)+3)
	for _, entry := range environment {
		name, value, _ := strings.Cut(entry, "=")
		if generated, ok := values[name]; ok && value == generated {
			entry = name + "=managed:" + name + ":v1"
		}
		normalized = append(normalized, entry)
	}
	// '#' cannot begin an environment name, so these entries cannot be forged
	// by a contract declaration.
	normalized = append(normalized, "#policy="+descriptor.Policy)
	if lookupEnvironment(environment, "GOENV") == values["GOENV"] && descriptor.GoEnv == "file" {
		normalized = append(normalized, "#goenv-sha256="+descriptor.GoEnvDigest)
	}
	if prepared, ok := descriptor.group(group.ID); ok && prepared.DeclaredGoEnvDigest != "" {
		normalized = append(normalized, "#goenv-declared-sha256="+prepared.DeclaredGoEnvDigest)
	}
	if prepared, ok := descriptor.group(group.ID); ok && prepared.DefaultGoEnv != "" {
		normalized = append(normalized, "#goenv-default-sha256="+prepared.DefaultGoEnvDigest)
	}
	return digestGroupEnvironment(group, normalized)
}

func lookupEnvironment(environment []string, name string) string {
	value := ""
	for _, entry := range environment {
		if key, found, ok := strings.Cut(entry, "="); ok && key == name {
			value = found
		}
	}
	return value
}

func lookupEnvironmentSet(environment []string, name string) (string, bool) {
	value, set := "", false
	for _, entry := range environment {
		if key, found, ok := strings.Cut(entry, "="); ok && key == name {
			value, set = found, true
		}
	}
	return value, set
}

// PrepareScratchEnvironment creates every selected group's managed tree and
// snapshots the inherited Go env file into the run root, then attaches the
// descriptor to request. It runs before metadata and before any test, so a
// failure here launches nothing. Env file contents never leave the root.
func PrepareScratchEnvironment(request *TestRunRequest, run *ScratchRun) error {
	if request == nil || run == nil {
		return errors.New("scratch environment: request and run are required")
	}
	if request.ScratchEnvironment != nil {
		return errors.New("scratch environment: request already prepared")
	}
	descriptor := &ScratchEnvironment{Policy: ScratchEnvironmentPolicy, Run: run.ID(), Root: run.Root()}
	if err := validateScratchRoot(descriptor, run); err != nil {
		return err
	}
	source, off, err := goEnvFileFor(request.Environment, runtime.GOOS)
	if err != nil {
		return err
	}
	contents := []byte{}
	if off {
		descriptor.GoEnv = goEnvOff
	} else if inherited := lookupEnvironment(request.Environment, "GOENV"); inherited == "" {
		descriptor.GoEnv = goEnvDefault
	} else {
		descriptor.GoEnv = "file"
		if source != "" {
			if contents, err = readGoEnvFile(source, false); err != nil {
				return fmt.Errorf("scratch environment: inherited Go env file: %w", err)
			}
		}
		if err := writeScratchFile(descriptor.goEnvPath(), contents); err != nil {
			return fmt.Errorf("scratch environment: Go env snapshot: %w", err)
		}
		descriptor.GoEnvDigest = digestBytes(contents)
	}
	groups := map[string]testpolicy.Group{}
	for _, group := range request.Contract.Groups {
		groups[group.ID] = group
	}
	ids := append([]string(nil), request.Plan.SelectedGroups...)
	sort.Strings(ids)
	for index, id := range ids {
		if index > 0 && ids[index-1] == id {
			continue
		}
		group, ok := groups[id]
		if !ok {
			return fmt.Errorf("scratch environment: selected testing group %s is absent", id)
		}
		prepared := ScratchEnvironmentGroup{ID: id}
		goEnv := contents
		declared, isDeclared := group.Env["GOENV"]
		if isDeclared || descriptor.GoEnv != "file" {
			goEnv = nil
		}
		// A declared empty GOENV, like an unset inherited one, is go's default
		// resolution against this group's own pre-managed HOME/XDG_CONFIG_HOME.
		if isDeclared || descriptor.GoEnv == goEnvDefault {
			if declared == "" {
				pre, post, managed, err := descriptor.defaultGoEnvPaths(request.Environment, group)
				if err != nil {
					return fmt.Errorf("scratch environment: group %s default Go env: %w", id, err)
				}
				if post != "" {
					if goEnv, err = readGoEnvFile(pre, false); err != nil {
						return fmt.Errorf("scratch environment: group %s default Go env file: %w", id, err)
					}
					prepared.DefaultGoEnv, prepared.DefaultGoEnvDigest = "external", digestBytes(goEnv)
					if managed {
						prepared.DefaultGoEnv = "managed"
					}
				}
			} else if declared != goEnvOff {
				if goEnv, err = readDeclaredGoEnv(declared); err != nil {
					return fmt.Errorf("scratch environment: group %s GOENV: %w", id, err)
				}
				prepared.DeclaredGoEnvDigest = digestBytes(goEnv)
			}
		}
		prepared.Dependencies = effectiveDependencyStores(request.Environment, group.Env, goEnv)
		for _, dir := range descriptor.groupDirectories(id) {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return fmt.Errorf("scratch environment: group %s: %w", id, err)
			}
		}
		if prepared.DefaultGoEnv == "managed" {
			_, post, _, _ := descriptor.defaultGoEnvPaths(request.Environment, group)
			if err := os.MkdirAll(filepath.Dir(post), 0o700); err != nil {
				return fmt.Errorf("scratch environment: group %s default Go env: %w", id, err)
			}
			if err := writeScratchFile(post, goEnv); err != nil {
				return fmt.Errorf("scratch environment: group %s default Go env snapshot: %w", id, err)
			}
		}
		descriptor.Groups = append(descriptor.Groups, prepared)
	}
	request.ScratchEnvironment = descriptor
	return nil
}

// ValidateScratchEnvironment checks, after the worker reopened its scratch
// run, that the descriptor names exactly that run's generated paths and that
// the Go env snapshot still has its prepared digest. It never re-snapshots.
// A legacy request with neither run nor descriptor passes; a scratch run
// without a descriptor does not fall back to the ambient environment.
func ValidateScratchEnvironment(request TestRunRequest, run *ScratchRun) error {
	descriptor := request.ScratchEnvironment
	switch {
	case descriptor == nil && run == nil:
		return nil
	case descriptor == nil:
		return errors.New("scratch environment: managed request has no environment descriptor")
	case run == nil:
		return errors.New("scratch environment: descriptor without a scratch run")
	}
	if descriptor.Policy != ScratchEnvironmentPolicy {
		return fmt.Errorf("scratch environment: unsupported policy %q", descriptor.Policy)
	}
	if descriptor.Run != run.ID() || descriptor.Root != run.Root() {
		return errors.New("scratch environment: descriptor does not name this scratch run")
	}
	if err := validateScratchRoot(descriptor, run); err != nil {
		return err
	}
	selected := map[string]bool{}
	for _, id := range request.Plan.SelectedGroups {
		selected[id] = true
	}
	groups := map[string]testpolicy.Group{}
	for _, group := range request.Contract.Groups {
		groups[group.ID] = group
	}
	seen := map[string]bool{}
	for _, prepared := range descriptor.Groups {
		if seen[prepared.ID] || !selected[prepared.ID] {
			return fmt.Errorf("scratch environment: unexpected group %q", prepared.ID)
		}
		seen[prepared.ID] = true
		for _, dir := range descriptor.groupDirectories(prepared.ID) {
			if err := requireScratchDirectory(descriptor.Root, dir); err != nil {
				return fmt.Errorf("scratch environment: group %s: %w", prepared.ID, err)
			}
		}
	}
	for id := range selected {
		if !seen[id] {
			return fmt.Errorf("scratch environment: selected group %s was not prepared", id)
		}
	}
	switch descriptor.GoEnv {
	case goEnvOff:
		if descriptor.GoEnvDigest != "" {
			return errors.New("scratch environment: GOENV=off carries a snapshot digest")
		}
	case goEnvDefault:
		if descriptor.GoEnvDigest != "" {
			return errors.New("scratch environment: default GOENV carries a run snapshot digest")
		}
	case "file":
		if err := requireScratchDirectory(descriptor.Root, filepath.Dir(descriptor.goEnvPath())); err != nil {
			return fmt.Errorf("scratch environment: Go env snapshot: %w", err)
		}
		contents, err := readGoEnvFile(descriptor.goEnvPath(), true)
		if err != nil {
			return fmt.Errorf("scratch environment: Go env snapshot: %w", err)
		}
		if digestBytes(contents) != descriptor.GoEnvDigest {
			return errors.New("scratch environment: Go env snapshot changed after preparation")
		}
	default:
		return fmt.Errorf("scratch environment: unknown GOENV mode %q", descriptor.GoEnv)
	}
	// External files are read only after every generated path checked out.
	for _, prepared := range descriptor.Groups {
		declared, ok := groups[prepared.ID].Env["GOENV"]
		if err := validateDefaultGoEnv(descriptor, request.Environment, groups[prepared.ID], prepared, declared == "" && (ok || descriptor.GoEnv == goEnvDefault)); err != nil {
			return err
		}
		want := ok && declared != goEnvOff && declared != ""
		if want != (prepared.DeclaredGoEnvDigest != "") {
			return fmt.Errorf("scratch environment: group %s declared GOENV does not match preparation", prepared.ID)
		}
		if !want {
			continue
		}
		contents, err := readDeclaredGoEnv(declared)
		if err != nil || digestBytes(contents) != prepared.DeclaredGoEnvDigest {
			return fmt.Errorf("scratch environment: group %s declared Go env file changed after preparation", prepared.ID)
		}
	}
	return nil
}

func validateScratchRoot(descriptor *ScratchEnvironment, run *ScratchRun) error {
	if !filepath.IsAbs(descriptor.Root) || filepath.Clean(descriptor.Root) != descriptor.Root {
		return errors.New("scratch environment: run root is not a clean absolute path")
	}
	resolved, err := filepath.EvalSymlinks(run.Root())
	if err != nil || resolved != descriptor.Root {
		return errors.New("scratch environment: run root is not its own resolved path")
	}
	return nil
}

// requireScratchDirectory accepts a real directory beneath root reached
// without a symlink at any component below root.
func requireScratchDirectory(root, dir string) error {
	relative, err := filepath.Rel(root, dir)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %s escapes the run root", dir)
	}
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("path %s is not a directory", current)
		}
	}
	return nil
}

// goEnvFileFor resolves the Go env file the caller's environment would read
// on goos, without launching go: GOENV if set, else os.UserConfigDir's rule
// evaluated over the given environment rather than this process's.
func goEnvFileFor(environment []string, goos string) (string, bool, error) {
	if value, set := lookupEnvironmentSet(environment, "GOENV"); set && value != "" {
		if value == goEnvOff {
			return "", true, nil
		}
		if !filepath.IsAbs(value) {
			return "", false, errors.New("scratch environment: inherited GOENV is not absolute")
		}
		return value, false, nil
	}
	home := lookupEnvironment(environment, "HOME")
	switch goos {
	case "darwin", "ios":
		if home == "" {
			return "", false, nil
		}
		return filepath.Join(home, "Library", "Application Support", "go", "env"), false, nil
	default:
		if config := lookupEnvironment(environment, "XDG_CONFIG_HOME"); filepath.IsAbs(config) {
			return filepath.Join(config, "go", "env"), false, nil
		}
		if home == "" {
			return "", false, nil
		}
		return filepath.Join(home, ".config", "go", "env"), false, nil
	}
}

// readGoEnvFile returns an absent file as empty, the way go reads it. The
// owned snapshot is read noFollow, so a replaced final component never
// redirects validation to bytes outside the root.
func readGoEnvFile(path string, noFollow bool) ([]byte, error) {
	flags := os.O_RDONLY
	if noFollow {
		flags |= syscall.O_NOFOLLOW
	}
	file, err := os.OpenFile(path, flags, 0)
	if errors.Is(err, fs.ErrNotExist) && !noFollow {
		return []byte{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	contents, err := io.ReadAll(io.LimitReader(file, scratchGoEnvMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(contents) > scratchGoEnvMaxBytes {
		return nil, errors.New("larger than 1 MiB")
	}
	return contents, nil
}

// readDeclaredGoEnv reads a group-declared external Go env file; that file
// belongs to the user, so it is followed like go itself would.
func readDeclaredGoEnv(path string) ([]byte, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("declared GOENV is not absolute")
	}
	return readGoEnvFile(path, false)
}

func writeScratchFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, err = file.Write(contents)
	return errors.Join(err, file.Close())
}

// effectiveDependencyStores keeps one group's Go module store where its Go
// would have found it before HOME was managed: a group-declared GOPATH or
// GOMODCACHE stays as declared, else the value it inherits, else what its
// effective Go env file sets travels with that file, else the default GOPATH
// derived from its effective (declared, else inherited) HOME.
func effectiveDependencyStores(base []string, declared map[string]string, goEnv []byte) []ScratchEnvironmentValue {
	effective := func(name string) string {
		if value, ok := declared[name]; ok {
			return value
		}
		return lookupEnvironment(base, name)
	}
	fileSets := func(name string) bool {
		for _, line := range bytes.Split(goEnv, []byte("\n")) {
			key, value, ok := strings.Cut(strings.TrimSpace(string(line)), "=")
			if ok && strings.TrimSpace(key) == name && strings.TrimSpace(value) != "" {
				return true
			}
		}
		return false
	}
	var result []ScratchEnvironmentValue
	// A declared empty GOPATH is go's default resolution, so it is derived
	// like an unset one, from the group's pre-managed HOME, never left to the
	// managed HOME.
	if value, ok := declared["GOPATH"]; !ok || value == "" {
		if value := effective("GOPATH"); value != "" {
			result = append(result, ScratchEnvironmentValue{Name: "GOPATH", Value: value})
		} else if home := effective("HOME"); !fileSets("GOPATH") && filepath.IsAbs(home) {
			result = append(result, ScratchEnvironmentValue{Name: "GOPATH", Value: filepath.Join(home, "go")})
		}
	}
	if _, ok := declared["GOMODCACHE"]; !ok {
		if value := effective("GOMODCACHE"); value != "" {
			result = append(result, ScratchEnvironmentValue{Name: "GOMODCACHE", Value: value})
		}
	}
	return result
}

// scratchDiscoveryEnvironment is the within-run Go discovery key view: a
// value equal to any prepared group's generated path for its name becomes a
// token, and the Go env snapshot digest joins. Declared and inherited values,
// including declared config paths, stay literal.
func scratchDiscoveryEnvironment(descriptor *ScratchEnvironment, environment []string) []string {
	if descriptor == nil {
		return environment
	}
	generated := map[string]bool{}
	for _, group := range descriptor.Groups {
		for name, value := range descriptor.managedValues(group.ID) {
			generated[name+"="+value] = true
		}
	}
	result := make([]string, 0, len(environment)+2)
	for _, entry := range environment {
		if generated[entry] {
			name, _, _ := strings.Cut(entry, "=")
			entry = name + "=managed:" + name + ":v1"
			if name == "GOENV" {
				result = append(result, "#goenv-sha256="+descriptor.GoEnvDigest)
			}
		}
		result = append(result, entry)
	}
	for _, group := range descriptor.Groups {
		if group.DeclaredGoEnvDigest != "" {
			result = append(result, "#goenv-declared-sha256="+group.ID+"="+group.DeclaredGoEnvDigest)
		}
		if group.DefaultGoEnv != "" {
			result = append(result, "#goenv-default-sha256="+group.ID+"="+group.DefaultGoEnvDigest)
		}
	}
	return append(result, "#policy="+descriptor.Policy)
}

// defaultGoEnvPaths resolves, for a group declaring GOENV="", the default Go
// env file before management (declared, else inherited HOME/XDG_CONFIG_HOME)
// and the one go will read under the managed overlay. managed reports that
// the latter lies in the group's generated tree.
func (e *ScratchEnvironment) defaultGoEnvPaths(base []string, group testpolicy.Group) (string, string, bool, error) {
	values := e.managedValues(group.ID)
	var before, after []string
	for _, name := range []string{"HOME", "XDG_CONFIG_HOME"} {
		value, declared := group.Env[name]
		if !declared {
			before = append(before, name+"="+lookupEnvironment(base, name))
			value = values[name]
		} else {
			before = append(before, name+"="+value)
		}
		after = append(after, name+"="+value)
	}
	pre, _, err := goEnvFileFor(before, runtime.GOOS)
	if err != nil {
		return "", "", false, err
	}
	post, _, err := goEnvFileFor(after, runtime.GOOS)
	if err != nil {
		return "", "", false, err
	}
	managed := strings.HasPrefix(post, e.groupDir(group.ID)+string(filepath.Separator))
	if post != "" && !managed && post != pre {
		return "", "", false, errors.New("default Go env location differs under the managed overlay")
	}
	return pre, post, managed, nil
}

// validateDefaultGoEnv checks a declared empty GOENV's default file: the
// managed snapshot no-follow beneath real generated directories, or the
// user's own external file as go would read it.
func validateDefaultGoEnv(descriptor *ScratchEnvironment, base []string, group testpolicy.Group, prepared ScratchEnvironmentGroup, declaredEmpty bool) error {
	if !declaredEmpty {
		if prepared.DefaultGoEnv != "" {
			return fmt.Errorf("scratch environment: group %s default Go env does not match preparation", prepared.ID)
		}
		return nil
	}
	_, post, managed, err := descriptor.defaultGoEnvPaths(base, group)
	want := ""
	if err == nil && post != "" {
		want = "external"
		if managed {
			want = "managed"
		}
	}
	if err != nil || want != prepared.DefaultGoEnv {
		return fmt.Errorf("scratch environment: group %s default Go env does not match preparation", prepared.ID)
	}
	if want == "" {
		return nil
	}
	if managed {
		if err := requireScratchDirectory(descriptor.Root, filepath.Dir(post)); err != nil {
			return fmt.Errorf("scratch environment: group %s default Go env: %w", prepared.ID, err)
		}
	}
	contents, err := readGoEnvFile(post, managed)
	if err != nil || digestBytes(contents) != prepared.DefaultGoEnvDigest {
		return fmt.Errorf("scratch environment: group %s default Go env file changed after preparation", prepared.ID)
	}
	return nil
}
