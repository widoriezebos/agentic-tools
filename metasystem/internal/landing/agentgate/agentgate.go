// Package agentgate is the landing agent's tool gate (goal
// landing-lane-runtime-redesign, design section 3): a fail-closed allowlist
// the runtime's PreToolUse hook applies to every call a session of lineage
// landing-agent makes. A call is allowed only when it matches an entry of
// lane-agent-tools.json and every rule attached to that entry; a gate that
// cannot decide (unreadable payload, unresolvable checkout, Git failure)
// denies.
//
// The gate is prevention against the agent's own tool use only. It does not
// stand between candidate code run by a proof and the network, and it cannot
// hold when the runtime kills the hook at its timeout; those are the
// residual risks the design accepts under D6/D8 (section 9).
package agentgate

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

//go:embed lane-agent-tools.json
var allowlistJSON []byte

const (
	// Lineage is the owner lineage the landing launch gives its sessions.
	Lineage = "landing-agent"
	// LineageEnv carries a session's owner lineage to its hooks.
	LineageEnv = "METASYSTEM_OWNER_LINEAGE"
)

// Governs reports whether the calling session is the landing agent, whose
// every tool call this gate decides.
func Governs(lookup func(string) (string, bool)) bool {
	if lookup == nil {
		return false
	}
	value, _ := lookup(LineageEnv)
	return value == Lineage
}

type allowlist struct {
	Tools     map[string]string `json:"tools"`
	Verbs     [][]string        `json:"verbs"`
	GitRead   []string          `json:"gitRead"`
	GitLane   []string          `json:"gitLane"`
	Shell     []string          `json:"shell"`
	Protected struct {
		Segments    []string `json:"segments"`
		Names       []string `json:"names"`
		Pairs       []string `json:"pairs"`
		ShallowDirs []string `json:"shallowDirs"`
	} `json:"protected"`
}

func load() (allowlist, error) {
	var list allowlist
	if err := json.Unmarshal(allowlistJSON, &list); err != nil {
		return allowlist{}, fmt.Errorf("the landing agent's allowlist is unreadable: %w", err)
	}
	if len(list.Tools) == 0 || len(list.Verbs) == 0 {
		return allowlist{}, errors.New("the landing agent's allowlist is empty")
	}
	return list, nil
}

// Entry is one allowlist entry: a tool, a metasystem verb, a git
// subcommand (read-only or lane-branch), or a read-only shell command.
type Entry struct{ Kind, Name string }

// Entries lists every allowlist entry, in a stable order.
func Entries() ([]Entry, error) {
	list, err := load()
	if err != nil {
		return nil, err
	}
	var entries []Entry
	tools := make([]string, 0, len(list.Tools))
	for tool := range list.Tools {
		tools = append(tools, tool)
	}
	sort.Strings(tools)
	for _, tool := range tools {
		entries = append(entries, Entry{"tool", tool})
	}
	for _, verb := range list.Verbs {
		entries = append(entries, Entry{"verb", strings.Join(verb, " ")})
	}
	for _, sub := range list.GitRead {
		entries = append(entries, Entry{"git", sub})
	}
	for _, sub := range list.GitLane {
		entries = append(entries, Entry{"git-lane", sub})
	}
	for _, command := range list.Shell {
		entries = append(entries, Entry{"shell", command})
	}
	return entries, nil
}

// Request is one PreToolUse call to decide.
type Request struct {
	// Payload is the runtime's hook input, as read from stdin.
	Payload []byte
	// Installation is the hook's working directory, inside the lane
	// checkout; the checkout is its Git toplevel.
	Installation string
}

// Decision is allow, or deny with a two-line reason: the plain situation,
// then the one command to run.
type Decision struct {
	Allow  bool
	Reason string
}

func allow() Decision { return Decision{Allow: true} }

func deny(situation, command string) Decision {
	return Decision{Reason: situation + "\nrun: " + command}
}

const (
	stopCommand   = `metasystem landing stop --reason "WHY THIS CALL IS NEEDED"`
	statusCommand = "metasystem landing status"
	// The kernel's publish and prove verbs arrive with units K-b and K-c;
	// until then a denial names the lane's status, which says what is next.
	publishCommand = statusCommand
	proveCommand   = statusCommand
)

func notListed(what string) Decision {
	return deny("the landing agent may not "+what+": only the listed landing verbs, read-only calls and lane/* branch work are open to it",
		stopCommand)
}

// Undecided is the denial of a call the gate could not decide.
func Undecided(err error) Decision { return undecided(err) }

func undecided(err error) Decision {
	return deny("the landing tool gate could not decide ("+oneLine(err.Error())+"), so this call is denied", statusCommand)
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

// Response is the runtime's answer: the deny object for a denial, nothing
// for an allowed call (the runtime's own flow continues).
func Response(decision Decision) []byte {
	if decision.Allow {
		return nil
	}
	var payload struct {
		HookSpecificOutput struct {
			HookEventName            string `json:"hookEventName"`
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	payload.HookSpecificOutput.HookEventName = "PreToolUse"
	payload.HookSpecificOutput.PermissionDecision = "deny"
	payload.HookSpecificOutput.PermissionDecisionReason = decision.Reason
	encoded, _ := json.Marshal(payload)
	return encoded
}

type hookPayload struct {
	Cwd   string          `json:"cwd"`
	Tool  string          `json:"tool_name"`
	Input json.RawMessage `json:"tool_input"`
}

// gate is one decision's resolved context.
type gate struct {
	list     allowlist
	checkout string // the lane checkout's physical Git toplevel
	cwd      string // the session's physical working directory
	module   string // the hook's physical installation directory
}

// Decide decides one call. It never returns an error: every failure is a
// denial.
func Decide(request Request) Decision {
	list, err := load()
	if err != nil {
		return undecided(err)
	}
	var call hookPayload
	if len(request.Payload) == 0 {
		return undecided(errors.New("the hook received no call"))
	}
	if err := json.Unmarshal(request.Payload, &call); err != nil {
		return undecided(fmt.Errorf("the call is not readable: %w", err))
	}
	if request.Installation == "" {
		return undecided(errors.New("the hook has no working directory"))
	}
	module, err := filepath.EvalSymlinks(request.Installation)
	if err != nil {
		return undecided(err)
	}
	top, err := runGit(module, "rev-parse", "--show-toplevel")
	if err != nil {
		return undecided(fmt.Errorf("the hook is not inside a Git checkout: %w", err))
	}
	checkout, err := filepath.EvalSymlinks(top)
	if err != nil {
		return undecided(err)
	}
	g := gate{list: list, checkout: checkout, module: module, cwd: checkout}
	if call.Cwd != "" {
		cwd, err := filepath.EvalSymlinks(call.Cwd)
		if err != nil {
			return undecided(fmt.Errorf("the session's working directory does not resolve: %w", err))
		}
		g.cwd = cwd
	}
	kind, listed := list.Tools[call.Tool]
	if !listed {
		return notListed("use the " + call.Tool + " tool")
	}
	switch kind {
	case "read":
		return allow()
	case "edit":
		return g.edit(call)
	case "command":
		return g.bash(call)
	default:
		return undecided(fmt.Errorf("tool kind %q is unknown", kind))
	}
}

func (g gate) edit(call hookPayload) Decision {
	var input struct {
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
	}
	if err := json.Unmarshal(call.Input, &input); err != nil {
		return undecided(fmt.Errorf("the edit is not readable: %w", err))
	}
	path := input.FilePath
	if path == "" {
		path = input.NotebookPath
	}
	if path == "" {
		return undecided(errors.New("the edit names no file"))
	}
	if !filepath.IsAbs(path) {
		return deny("the landing agent edits files by absolute path inside the lane checkout, and "+path+" is relative", stopCommand)
	}
	resolved, err := resolveExisting(filepath.Clean(path))
	if err != nil {
		return undecided(err)
	}
	return g.pathInside(resolved, "edit "+path)
}

// resolveExisting resolves symbolic links along the longest existing prefix
// of path, so a link inside the checkout cannot carry an edit outside it.
func resolveExisting(path string) (string, error) {
	rest := ""
	current := path
	for {
		if _, err := os.Lstat(current); err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			return filepath.Join(resolved, rest), nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return path, nil
		}
		rest = filepath.Join(filepath.Base(current), rest)
		current = parent
	}
}

// pathInside allows a resolved path inside the checkout that no protected
// rule names.
func (g gate) pathInside(resolved, what string) Decision {
	relative, err := filepath.Rel(g.checkout, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return deny("the landing agent may only "+what+" inside the lane checkout "+g.checkout, stopCommand)
	}
	if g.protected(filepath.ToSlash(relative)) {
		return deny("the landing agent may not "+what+": Git internals, runtime settings, the engine, local configuration and goals are protected", stopCommand)
	}
	return allow()
}

func (g gate) protected(relative string) bool {
	segments := strings.Split(relative, "/")
	for index, segment := range segments {
		for _, name := range g.list.Protected.Segments {
			if segment == name {
				return true
			}
		}
		for _, name := range g.list.Protected.ShallowDirs {
			if segment == name && index <= 1 && index < len(segments)-1 {
				return true
			}
		}
	}
	base := segments[len(segments)-1]
	for _, name := range g.list.Protected.Names {
		if base == name {
			return true
		}
	}
	for _, pair := range g.list.Protected.Pairs {
		if relative == pair || strings.HasPrefix(relative, pair+"/") || strings.Contains(relative, "/"+pair+"/") || strings.HasSuffix(relative, "/"+pair) {
			return true
		}
	}
	return false
}

func (g gate) bash(call hookPayload) Decision {
	var input struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(call.Input, &input); err != nil {
		return undecided(fmt.Errorf("the command is not readable: %w", err))
	}
	if strings.TrimSpace(input.Command) == "" {
		return undecided(errors.New("the call names no command"))
	}
	if inside := g.pathInside(g.cwd, "run commands"); !inside.Allow {
		return inside
	}
	top, err := runGit(g.cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return undecided(fmt.Errorf("the session's working directory is not in the lane checkout: %w", err))
	}
	if resolved, err := filepath.EvalSymlinks(top); err != nil || resolved != g.checkout {
		return deny("the landing agent runs commands only in the lane checkout "+g.checkout, stopCommand)
	}
	commands, err := splitCommands(input.Command)
	if err != nil {
		return deny("the landing agent's commands are plain words joined by &&, ||, ; or |, and this one "+err.Error(), stopCommand)
	}
	for _, words := range commands {
		if decision := g.command(words); !decision.Allow {
			return decision
		}
	}
	return allow()
}

func (g gate) command(words []string) Decision {
	for _, word := range words {
		if word == "--no-verify" {
			return deny("the landing agent never skips hooks (--no-verify)", stopCommand)
		}
	}
	name := words[0]
	switch {
	case name == "git":
		return g.git(words[1:])
	case g.isEngine(name):
		return g.verb(words[1:])
	case name == "cd":
		return deny("the landing agent works from the lane checkout and never changes directory", stopCommand)
	case name == "go" || name == "make" || strings.HasSuffix(name, "devgate"):
		return deny("the landing agent never builds or runs tests itself; proofs run through the kernel", proveCommand)
	}
	for _, shell := range g.list.Shell {
		if name == shell {
			if name == "sort" {
				for _, word := range words[1:] {
					if strings.HasPrefix(word, "-o") || strings.HasPrefix(word, "--output") {
						return notListed("write a file with sort")
					}
				}
			}
			return allow()
		}
	}
	return notListed("run " + name)
}

// isEngine accepts the engine on PATH, or the lane's own bin/metasystem by
// path; bin/ is protected from edits, so no planted engine passes.
func (g gate) isEngine(word string) bool {
	if word == "metasystem" {
		return true
	}
	if filepath.Base(word) != "metasystem" || filepath.Base(filepath.Dir(word)) != "bin" {
		return false
	}
	path := word
	if !filepath.IsAbs(path) {
		path = filepath.Join(g.cwd, path)
	}
	relative, err := filepath.Rel(g.checkout, filepath.Clean(path))
	return err == nil && !strings.HasPrefix(relative, "..") && strings.Count(filepath.ToSlash(relative), "/") <= 2
}

func (g gate) verb(args []string) Decision {
	for _, word := range args {
		if word == "--force" || strings.HasPrefix(word, "--force=") {
			return deny("the landing agent never forces a verb; a person decides overrides", stopCommand)
		}
	}
	for _, verb := range g.list.Verbs {
		if len(args) >= len(verb) && equalWords(args[:len(verb)], verb) {
			return allow()
		}
	}
	shown := strings.Join(args, " ")
	if len(args) > 2 {
		shown = strings.Join(args[:2], " ")
	}
	if len(args) > 0 && (args[0] == "test" || args[0] == "internal") {
		return deny("the landing agent never runs tests itself (metasystem "+shown+"); proofs run through the kernel", proveCommand)
	}
	return notListed("run metasystem " + shown)
}

func equalWords(a, b []string) bool {
	for index := range b {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

var laneBranch = regexp.MustCompile(`^lane/[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$`)

func isLaneBranch(name string) bool {
	return laneBranch.MatchString(name) && !strings.Contains(name, "..") && !strings.HasSuffix(name, ".lock")
}

// readDenied are options that make a read-only git subcommand write a file
// or run a program.
var readDenied = []string{"--output", "-O", "--open-files-in-pager", "--ext-diff", "--exec", "--upload-pack"}

func (g gate) git(args []string) Decision {
	if len(args) > 0 && args[0] == "--no-pager" {
		args = args[1:]
	}
	if len(args) == 0 {
		return notListed("run git without a subcommand")
	}
	sub, rest := args[0], args[1:]
	if strings.HasPrefix(sub, "-") {
		return notListed("pass git a global option (" + sub + ")")
	}
	if sub == "push" {
		return deny("the landing agent never pushes; main moves only through the kernel's publication", publishCommand)
	}
	for _, read := range g.list.GitRead {
		if sub == read {
			return g.gitRead(sub, rest)
		}
	}
	for _, lane := range g.list.GitLane {
		if sub == lane {
			return g.gitLane(sub, rest)
		}
	}
	return notListed("run git " + sub)
}

func (g gate) gitRead(sub string, args []string) Decision {
	for _, word := range args {
		for _, prefix := range readDenied {
			if word == prefix || strings.HasPrefix(word, prefix+"=") || (prefix == "-O" && strings.HasPrefix(word, "-O")) {
				return notListed("run git " + sub + " " + word)
			}
		}
	}
	switch sub {
	case "branch":
		listing := false
		for index := 0; index < len(args); index++ {
			switch word := args[index]; {
			case word == "--list" || word == "-l":
				listing = true
			case word == "-a" || word == "--all" || word == "-r" || word == "--remotes" || word == "-v" || word == "-vv" ||
				word == "--show-current" || strings.HasPrefix(word, "--format=") || strings.HasPrefix(word, "--sort="):
			case word == "--contains" || word == "--merged" || word == "--no-merged" || word == "--points-at":
				index++
			case strings.HasPrefix(word, "-"):
				return notListed("change branches with git branch " + word)
			default:
				if !listing {
					return notListed("create a branch with git branch; lane/* branches come from git checkout -b")
				}
			}
		}
	case "reflog":
		if len(args) > 0 && !strings.HasPrefix(args[0], "-") && args[0] != "show" {
			return notListed("run git reflog " + args[0])
		}
	}
	return allow()
}

// laneFlags are each lane subcommand's admitted options; a value option
// consumes the next word.
var laneFlags = map[string]struct{ plain, value []string }{
	"fetch":       {plain: []string{"-q", "--quiet", "--prune", "-p", "--no-tags"}},
	"add":         {plain: []string{"-u", "--update", "-A", "--all", "--"}},
	"cherry-pick": {plain: []string{"-x", "--continue", "--abort", "--skip", "--quit", "--allow-empty", "--keep-redundant-commits", "-n", "--no-commit", "--empty=drop", "--empty=keep", "--empty=stop"}, value: []string{"-m", "--mainline"}},
	"rebase":      {plain: []string{"--continue", "--abort", "--skip", "--quit", "-q", "--quiet", "--keep-empty"}, value: []string{"--onto"}},
	"commit":      {plain: []string{"--amend", "--no-edit", "--allow-empty", "-a", "--all", "-q", "--quiet"}, value: []string{"-m", "--message", "-F", "--file", "--trailer"}},
	"checkout":    {plain: []string{"-b", "-B", "--ours", "--theirs", "-q", "--quiet", "--"}},
}

func (g gate) gitLane(sub string, args []string) Decision {
	flags := laneFlags[sub]
	var positional []string
	afterSeparator := false
	for index := 0; index < len(args); index++ {
		word := args[index]
		if afterSeparator || !strings.HasPrefix(word, "-") {
			positional = append(positional, word)
			continue
		}
		if word == "--" {
			afterSeparator = true
		}
		if contains(flags.plain, word) {
			continue
		}
		if contains(flags.value, word) {
			index++
			continue
		}
		if name, _, found := strings.Cut(word, "="); found && contains(flags.value, name) {
			continue
		}
		return notListed("run git " + sub + " " + word)
	}
	switch sub {
	case "fetch":
		return g.fetch(positional)
	case "checkout":
		return g.gitCheckout(args)
	}
	branch, err := g.currentBranch()
	if err != nil {
		return undecided(err)
	}
	if !isLaneBranch(branch) {
		return deny("the landing agent changes only lane/* branches, and the lane checkout is on "+describeBranch(branch),
			"git checkout lane/BATCH")
	}
	switch sub {
	case "add":
		for _, path := range positional {
			if decision := g.relativePath(path, "stage "+path); !decision.Allow {
				return decision
			}
		}
	case "rebase":
		if len(positional) > 2 {
			return notListed("pass git rebase more than an upstream and a branch")
		}
		if len(positional) == 2 && !isLaneBranch(positional[1]) {
			return deny("the landing agent rebases only lane/* branches, not "+positional[1], "git checkout lane/BATCH")
		}
	case "commit":
		if len(positional) != 0 {
			return notListed("commit named paths; stage them with git add first")
		}
	}
	return allow()
}

func describeBranch(branch string) string {
	if branch == "" {
		return "a detached head"
	}
	return branch
}

func contains(list []string, word string) bool {
	for _, item := range list {
		if item == word {
			return true
		}
	}
	return false
}

func (g gate) fetch(positional []string) Decision {
	for index, word := range positional {
		if strings.Contains(word, "::") || strings.Contains(word, "upload-pack") {
			return notListed("fetch through a command transport")
		}
		if index == 0 {
			continue
		}
		_, destination, found := strings.Cut(strings.TrimPrefix(word, "+"), ":")
		if !found {
			continue
		}
		if !strings.HasPrefix(destination, "refs/remotes/") && !isLaneBranch(strings.TrimPrefix(destination, "refs/heads/")) {
			return deny("the landing agent fetches only into remote-tracking refs and lane/* branches, not "+destination, "git fetch origin")
		}
	}
	return allow()
}

// gitCheckout admits switching to or creating a lane/* branch, and restoring
// paths on a lane branch (conflict resolution).
func (g gate) gitCheckout(args []string) Decision {
	separator := -1
	for index, word := range args {
		if word == "--" {
			separator = index
			break
		}
	}
	if separator >= 0 {
		branch, err := g.currentBranch()
		if err != nil {
			return undecided(err)
		}
		if !isLaneBranch(branch) {
			return deny("the landing agent restores files only on a lane/* branch, and the lane checkout is on "+describeBranch(branch), "git checkout lane/BATCH")
		}
		if len(args[separator+1:]) == 0 {
			return notListed("run git checkout -- without paths")
		}
		for _, path := range args[separator+1:] {
			if decision := g.relativePath(path, "restore "+path); !decision.Allow {
				return decision
			}
		}
		return allow()
	}
	var positional []string
	create := false
	for _, word := range args {
		switch word {
		case "-b", "-B":
			create = true
		case "-q", "--quiet":
		default:
			if strings.HasPrefix(word, "-") {
				return notListed("run git checkout " + word)
			}
			positional = append(positional, word)
		}
	}
	if len(positional) == 0 || !isLaneBranch(positional[0]) || (!create && len(positional) != 1) || len(positional) > 2 {
		return deny("the landing agent checks out only lane/* branches", "git checkout lane/BATCH")
	}
	return allow()
}

// relativePath checks a path a git command names relative to the session's
// working directory.
func (g gate) relativePath(path, what string) Decision {
	if path == "." || path == "" {
		return g.pathInside(g.cwd, what)
	}
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(g.cwd, full)
	}
	resolved, err := resolveExisting(filepath.Clean(full))
	if err != nil {
		return undecided(err)
	}
	return g.pathInside(resolved, what)
}

// currentBranch is the checkout's branch, or the branch a rebase in progress
// is rewriting; empty for a detached head.
func (g gate) currentBranch() (string, error) {
	if branch, err := runGit(g.checkout, "symbolic-ref", "--quiet", "--short", "HEAD"); err == nil {
		return branch, nil
	}
	for _, state := range []string{"rebase-merge/head-name", "rebase-apply/head-name"} {
		path, err := runGit(g.checkout, "rev-parse", "--path-format=absolute", "--git-path", state)
		if err != nil {
			return "", err
		}
		if data, err := os.ReadFile(path); err == nil {
			return strings.TrimPrefix(strings.TrimSpace(string(data)), "refs/heads/"), nil
		}
	}
	return "", nil
}

// runGit runs git in dir with the caller's Git steering removed, and returns
// its trimmed output.
var runGit = func(dir string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = scrubbedEnviron()
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(output)), nil
}

func scrubbedEnviron() []string {
	var kept []string
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "GIT_") {
			continue
		}
		kept = append(kept, entry)
	}
	return kept
}
