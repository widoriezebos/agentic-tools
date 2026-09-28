// Package adopt installs the metasystem into a fresh application repository:
// the public `system adopt` action. It composes the owners that already decide
// each part (runtime registry, configuration tailoring, landing rulings, goal
// genesis, host registration, the structural audit and the ledger fence) and
// owns only what adoption itself decides: which template bytes ship, how a
// target is recognized, and what may never be overwritten.
//
// Nothing below the command layer imports this package.
package adopt

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/audit"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostsetup"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ledgerfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// The exit codes adoption has always used: 0 adopted or already adopted, 1
// refused, 2 a usage or environment error.
const (
	CodeRefused = 1
	CodeUsage   = 2
)

// PayloadAllow is the payload allowlist: what is not named here does not
// ship. The paper, the development ledgers and the roster stay home because
// nothing ships by default. cmd/, internal/, go.mod
// and go.sum are the engine source: the payload ships source and the target
// rebuilds, and the engine's data (the agent protocol, the landing and path
// policy, the runtime hook settings and this workflow) is compiled into it.
// The coverage floors and the parallel ratchet (testing-parallel-ratchet.json)
// are the template's own development test policy and stay home.
var PayloadAllow = []string{
	".gitattributes", ".gitignore", "AGENTS.md", "CLAUDE.md", "cmd", "docs", "go.mod", "go.sum",
	"internal", "memory", "metasystem.conf", "optional-skills", "plans", "records",
	"skills", "testing.json", "wow.md",
}

// githubActionsWorkflow is the runtime-neutral CI enforcement adoption
// installs at workflowPath.
//
//go:embed github-actions-metasystem.yml
var githubActionsWorkflow []byte

// ForeignAssets are the file-shaped instruction assets adoption can detect.
// Any of them in a target means the repository is not fresh.
var ForeignAssets = []string{
	"AGENTS.md", "CLAUDE.md", "GEMINI.md", "wow.md", ".cursorrules", ".cursor/rules",
	".github/copilot-instructions.md", ".windsurfrules", ".claude", ".devin", ".agents", "skills",
}

// MarkerPrefix is the adoption record line in docs/project-rules.md.
const MarkerPrefix = "- Adopted from template SHA:"

// Placeholder is the marker's value before adoption fills it.
const Placeholder = "<template sha>"

const workflowPath = ".github/workflows/metasystem.yml"

// Options is one adoption.
type Options struct {
	// Source is the template installation adopted from.
	Source string
	// Target is the application directory adopted into; it is created when
	// missing.
	Target string
	// Runtimes is the comma-separated runtime selection, or "none".
	Runtimes string
	// Enable names optional skills moved into skills/.
	Enable []string
	// CopySkills copies skill trees instead of linking them.
	CopySkills bool
	Stdout     io.Writer
	Stderr     io.Writer
	Deps       Deps
}

// Deps are the effects adoption delegates. Zero values select the real ones,
// except Build and Genesis, which the command layer supplies.
type Deps struct {
	// Git runs git in dir. With scrub, git's steering variables are removed
	// so a probe of the target can never answer for another repository.
	Git func(dir string, scrub bool, args ...string) ([]byte, error)
	// LookPath finds a command on PATH.
	LookPath func(string) (string, error)
	// Build builds the template's engine at <source>/bin/metasystem.
	Build func(source string) error
	// EngineStamp is the build stamp of the engine running this adoption.
	EngineStamp string
	// Genesis writes the target's goal baseline through the goal owner's
	// genesis path, classified against the target.
	Genesis func(target string) error
	// Now is the clock for the goal-free declaration.
	Now func() time.Time
	// LookupEnv answers the environment the target's evidence root resolves
	// under; nil is os.LookupEnv.
	LookupEnv func(string) (string, bool)
}

// Refusal is a stop before or during adoption. Remedy names the way forward:
// a refusal is never a dead end.
type Refusal struct {
	Code    int
	Message string
	Detail  []string
	Remedy  string
	// Argv is a command that achieves the intent the right way, when one
	// exists.
	Argv []string
}

func (r *Refusal) Error() string { return r.Message }

func refuse(code int, message, remedy string, argv ...string) *Refusal {
	return &Refusal{Code: code, Message: message, Remedy: remedy, Argv: argv}
}

// Result is a finished adoption.
type Result struct {
	// Already is true when the target was this template's healthy
	// installation at the same commit and nothing was written.
	Already bool
	SHA     string
	Target  string
	// Notes are facts found on the way that the person should know.
	Notes []string
	// Installed lists the registration paths runtime setup wrote.
	Installed []string
}

func (o *Options) defaults() {
	if o.Stdout == nil {
		o.Stdout = io.Discard
	}
	if o.Stderr == nil {
		o.Stderr = io.Discard
	}
	if o.Deps.Git == nil {
		o.Deps.Git = RunGit
	}
	if o.Deps.LookPath == nil {
		o.Deps.LookPath = exec.LookPath
	}
	if o.Deps.Now == nil {
		o.Deps.Now = time.Now
	}
	if o.Runtimes == "" {
		o.Runtimes = runtimes.AdoptionDefault()
	}
}

// RunGit is the real git adapter.
func RunGit(dir string, scrub bool, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if scrub {
		command.Env = ledgerfence.EnvironWithoutGitSteering()
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			return out, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return out, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, detail)
	}
	return out, nil
}

// SelectRuntimes validates a runtime selection before anything is touched:
// a typo must not leave a partially adopted target behind.
func SelectRuntimes(csv string) ([]string, *Refusal) {
	adoptable := runtimes.Adoptable()
	usage := func(message string) *Refusal {
		return refuse(CodeUsage, message, "choose from "+strings.Join(adoptable, ", ")+", or none",
			"metasystem", "system", "adopt", "TARGET", "--runtimes", strings.Join(adoptable, ","))
	}
	if strings.TrimSpace(csv) == "" {
		return nil, usage("--runtimes cannot be empty")
	}
	selected := strings.Split(csv, ",")
	if csv == "none" {
		return selected, nil
	}
	seen := map[string]bool{}
	for _, name := range selected {
		if name == "none" {
			return nil, usage("--runtimes none cannot be combined with other runtimes")
		}
		known := false
		for _, candidate := range adoptable {
			known = known || candidate == name
		}
		if !known {
			return nil, usage("unknown or non-adoptable runtime: " + name)
		}
		if seen[name] {
			return nil, usage("--runtimes contains a duplicate runtime")
		}
		seen[name] = true
	}
	return selected, nil
}

// Adopt performs one adoption. Every refusal before the payload is copied
// leaves the target exactly as it was (apart from creating a missing target
// directory).
func Adopt(options Options) (Result, error) {
	options.defaults()
	d := options.Deps
	selected, refusal := SelectRuntimes(options.Runtimes)
	if refusal != nil {
		return Result{}, refusal
	}
	// The engine is always rebuilt from the template source, never copied
	// on trust, so a machine without Go refuses here, before any write.
	if _, err := d.LookPath("go"); err != nil {
		return Result{}, refuse(CodeRefused, "adoption requires the Go toolchain: the engine is always rebuilt from the template source",
			"install Go, then run the same command again")
	}
	if missing := up.MissingProductionCommands(d.LookPath); len(missing) > 0 {
		r := refuse(CodeRefused, "adoption refused: this host is missing production commands", "install the named commands, then run the same command again")
		for _, command := range missing {
			r.Detail = append(r.Detail, fmt.Sprintf("%s (package: %s)", command.Name, command.Package))
		}
		return Result{}, r
	}
	if err := os.MkdirAll(options.Target, 0o755); err != nil {
		return Result{}, refuse(CodeUsage, fmt.Sprintf("cannot create the target %s: %v", options.Target, err), "name a directory you can write")
	}
	target, err := filepath.EvalSymlinks(options.Target)
	if err == nil {
		target, err = filepath.Abs(target)
	}
	if err != nil {
		return Result{}, refuse(CodeUsage, fmt.Sprintf("cannot resolve the target %s: %v", options.Target, err), "name an existing directory")
	}
	source := options.Source

	// Source provenance: the payload is exported from the tracked HEAD, so
	// ignored or untracked content cannot ride along, and a dirty worktree
	// is refused so the recorded SHA identifies the payload exactly.
	if _, err := d.Git(source, false, "rev-parse", "--is-inside-work-tree"); err != nil {
		return Result{}, refuse(CodeUsage, "the template source is not a git checkout: "+source, "adopt from a git checkout of the template")
	}
	status, err := d.Git(source, false, "status", "--porcelain", "--", ".")
	if err != nil {
		return Result{}, refuse(CodeUsage, fmt.Sprintf("cannot read the template's status: %v", err), "repair the template checkout")
	}
	if strings.TrimSpace(string(status)) != "" {
		return Result{}, refuse(CodeRefused, "the template worktree is dirty; the recorded SHA would not identify the copied payload",
			"commit or stash the template's changes, then run the same command again", "git", "-C", source, "status")
	}
	sha, err := gitLine(d, source, false, "rev-parse", "HEAD")
	if err != nil {
		return Result{}, refuse(CodeUsage, fmt.Sprintf("cannot read the template's HEAD: %v", err), "repair the template checkout")
	}
	prefixOut, err := d.Git(source, false, "rev-parse", "--show-prefix")
	if err != nil {
		return Result{}, refuse(CodeUsage, fmt.Sprintf("cannot read the template's prefix: %v", err), "repair the template checkout")
	}
	prefix := strings.TrimRight(string(prefixOut), "\n")

	already, refusal := recognize(target, sha)
	if refusal != nil {
		return Result{}, refusal
	}
	if already {
		return Result{Already: true, SHA: sha, Target: target}, nil
	}
	result := Result{SHA: sha, Target: target}
	result.Notes = append(result.Notes, "only file-shaped instruction assets are detectable; confirm by hand that no agent-directed prose, prompt directories, or agent-encoding hooks or CI exist before treating this repository as fresh")

	if refusal := hookPreflight(d, target); refusal != nil {
		return Result{}, refusal
	}

	if d.Build == nil {
		return Result{}, errors.New("adopt: no engine build supplied")
	}
	if err := d.Build(source); err != nil {
		return Result{}, refuse(CodeRefused, fmt.Sprintf("could not build the metasystem engine for adoption: %v", err),
			"repair the template's build, then run the same command again", "go", "-C", source, "run", "./cmd/devgate", "build")
	}
	// The owners below run in this process; they must be the template's
	// own code, or the payload would be tailored by another version's rules.
	if d.EngineStamp != sha {
		return Result{}, refuse(CodeRefused,
			fmt.Sprintf("this engine was built from %s, not from the template's HEAD %s; adopting with it could tailor the payload by other rules", displayStamp(d.EngineStamp), sha),
			"the template's engine is now rebuilt; run the adoption with it",
			append([]string{filepath.Join(source, "bin", "metasystem"), "system", "adopt", target, "--repo", source}, adoptFlags(options)...)...)
	}

	stage, err := os.MkdirTemp("", "metasystem-adopt-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(stage)
	if refusal := stagePayload(d, options, source, prefix, stage, target); refusal != nil {
		return Result{}, refusal
	}
	conf := filepath.Join(stage, "metasystem.conf")
	contract := filepath.Join(stage, "testing.json")
	for _, required := range []string{conf, contract} {
		if !regularFile(required) {
			return Result{}, refuse(CodeRefused, "the payload is missing "+filepath.Base(required), "restore it in the template, commit, then run the same command again")
		}
	}
	// The selected-runtime list is durable state; no unselected runtime's
	// model placeholder or mode override may reach the adopted repository.
	if err := validate.TailorConf(conf, selected); err != nil {
		return Result{}, refuse(CodeRefused, fmt.Sprintf("could not tailor metasystem.conf: %v", err), "repair the template's metasystem.conf")
	}
	incomplete, err := testpolicy.IncompleteTemplate()
	if err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(contract, incomplete, 0o644); err != nil {
		return Result{}, err
	}

	if collisions := collide(stage, target); len(collisions) > 0 {
		r := refuse(CodeRefused, fmt.Sprintf("the target already contains %d differing payload path(s)", len(collisions)),
			"resolve them, or follow docs/metasystem-reconciliation.md for a repository that already has content")
		for _, path := range collisions {
			r.Detail = append(r.Detail, "collision: "+path)
		}
		return Result{}, r
	}
	if err := copyPayload(stage, target); err != nil {
		return Result{}, err
	}
	if !regularFile(filepath.Join(target, "metasystem.conf")) || !regularFile(filepath.Join(target, "cmd", "metasystem", "main.go")) {
		return Result{}, refuse(CodeRefused, "the adopted payload is missing metasystem.conf or the engine source", "restore the template's payload, commit, then run the same command again")
	}
	if err := copyFile(filepath.Join(source, "bin", "metasystem"), filepath.Join(target, "bin", "metasystem"), 0o755); err != nil {
		return Result{}, err
	}

	// The target inherits the adopting machine's enrolled nickname: goal
	// actions publish an actor, and hostnames are never published.
	nickname, _ := gitLine(d, source, false, "config", "--get", "metasystem.goal.machine")
	if nickname == "" {
		return Result{}, refuse(CodeRefused, "no machine nickname is enrolled on this machine, and hostnames are never published",
			"enroll a nickname once, then run the same command again", "git", "config", "--global", "metasystem.goal.machine", "NICKNAME")
	}
	targetIsGit := false
	if _, err := d.Git(target, true, "rev-parse", "--git-dir"); err == nil {
		targetIsGit = true
		if _, err := d.Git(target, true, "config", "metasystem.goal.machine", nickname); err != nil {
			return Result{}, err
		}
		result.Notes = append(result.Notes, seedLandingRef(d, target)...)
	}

	store := &goal.Store{Root: target}
	if !(store.BaselinePresent() && store.BaselineMatches()) {
		if d.Genesis == nil {
			return Result{}, errors.New("adopt: no goal genesis supplied")
		}
		if err := d.Genesis(target); err != nil {
			return Result{}, refuse(CodeRefused, fmt.Sprintf("the goal baseline genesis failed in the target: %v", err),
				"fix the cause above, then run the same command again")
		}
	}

	if err := os.MkdirAll(filepath.Join(target, "artifacts"), 0o755); err != nil {
		return Result{}, err
	}
	if err := appendMissingLines(filepath.Join(target, ".gitignore"), []string{"artifacts/"}); err != nil {
		return Result{}, err
	}
	if err := recordSHA(filepath.Join(target, "docs", "project-rules.md"), sha); err != nil {
		return Result{}, err
	}

	// Runtime registrations are planned and published by the same owner
	// that reconciles existing installations, so foreign settings survive
	// and a retry converges.
	registered, err := hostsetup.Setup(hostsetup.Options{RepositoryPath: target, Runtimes: selected, CopySkills: options.CopySkills})
	if err != nil {
		return Result{}, refuse(CodeRefused, fmt.Sprintf("runtime registration failed in the target: %v", err), "fix the cause above, then run the same command again")
	}
	for _, changed := range registered.Changed {
		result.Installed = append(result.Installed, filepath.Join(registered.Layout.RepositoryRoot, filepath.FromSlash(changed)))
	}

	// Runtime-neutral enforcement.
	if err := installFile(filepath.Join(target, filepath.FromSlash(workflowPath)), githubActionsWorkflow, 0o644); err != nil {
		return Result{}, err
	}

	// Structural check now; the placeholder check waits for the facts.
	audited, err := audit.AuditMetasystem(target, audit.AuditOptions{AllowPlaceholders: true})
	if err != nil || len(audited.Violations) > 0 {
		r := refuse(CodeRefused, "the structural audit failed in the adopted target", "fix the violations named below, then run the same command again")
		if err != nil {
			r.Detail = append(r.Detail, err.Error())
		}
		r.Detail = append(r.Detail, audited.Violations...)
		return Result{}, r
	}

	// The ledger fence: the guard runs first, a project's own hook is kept
	// as pre-commit.local behind it. Genesis already enrolled it in a git
	// target; this makes the enrollment explicit for the report.
	if targetIsGit {
		if err := ledgerfence.Ensure(target); err != nil {
			return Result{}, refuse(CodeRefused, fmt.Sprintf("the pre-commit guard could not be enrolled: %v", err),
				"compose or enroll the hook by hand, then run the same command again")
		}
		if regularFile(filepath.Join(hookDirectory(d, target), "pre-commit.local")) {
			result.Notes = append(result.Notes, "the target's own pre-commit hook runs after the guard, as pre-commit.local")
		}
	} else {
		result.Notes = append(result.Notes, "the target is not a git repository; the pre-commit guard is enrolled by the first goal action after git init")
	}
	result.Notes = append(result.Notes, evidenceRootNote(target, d.LookupEnv))
	return result, nil
}

// evidenceRootNote is the one line adoption says about the target's evidence
// root; it never requires one, so a refusal is said, not raised.
func evidenceRootNote(target string, lookup func(string) (string, bool)) string {
	resolved, err := config.ResolveEvidenceRoot(config.EvidenceRootParams{ConfPath: filepath.Join(target, "metasystem.conf"), LookupEnv: lookup})
	if err != nil {
		return "evidence root: " + err.Error()
	}
	return resolved.Line()
}

func displayStamp(stamp string) string {
	if stamp == "" {
		return "an unstamped build"
	}
	return stamp
}

func adoptFlags(options Options) []string {
	flags := []string{"--runtimes", options.Runtimes}
	for _, skill := range options.Enable {
		flags = append(flags, "--enable", skill)
	}
	if options.CopySkills {
		flags = append(flags, "--copy-skills")
	}
	return flags
}

func gitLine(d Deps, dir string, scrub bool, args ...string) (string, error) {
	out, err := d.Git(dir, scrub, args...)
	return strings.TrimRight(string(out), "\n"), err
}

// recognize classifies the target: this template's healthy installation at
// the same commit is a no-op; an installation at another commit goes to the
// upgrade path; any other detectable instruction asset goes to
// reconciliation.
func recognize(target, sha string) (bool, *Refusal) {
	rules := filepath.Join(target, "docs", "project-rules.md")
	if regularFile(filepath.Join(target, "wow.md")) && regularFile(rules) {
		if line, found := markerLine(rules); found {
			if strings.Contains(line, Placeholder) || strings.Contains(line, sha) {
				if regularFile(filepath.Join(target, filepath.FromSlash(workflowPath))) && executable(filepath.Join(target, "bin", "metasystem")) {
					audited, err := audit.AuditMetasystem(target, audit.AuditOptions{AllowPlaceholders: true})
					if err == nil && len(audited.Violations) == 0 {
						return true, nil
					}
				}
				return false, refuse(CodeRefused, "the target carries this template's marker but is not a complete healthy installation (missing workflow, engine, or failing structural audit)",
					"finish it by hand per docs/project-adaptation.md, or adopt into a clean target")
			}
			return false, refuse(CodeRefused, "the target carries an installation at another template SHA",
				"follow the upgrade path in docs/metasystem-reconciliation.md")
		}
	}
	for _, asset := range ForeignAssets {
		if _, err := os.Lstat(filepath.Join(target, filepath.FromSlash(asset))); err == nil {
			return false, refuse(CodeRefused, fmt.Sprintf("the target contains an existing instruction asset (%s)", asset),
				"follow docs/metasystem-reconciliation.md to merge the metasystem into a repository that already instructs agents")
		}
	}
	if _, err := os.Lstat(filepath.Join(target, filepath.FromSlash(workflowPath))); err == nil {
		return false, refuse(CodeRefused, "the target already has "+workflowPath,
			"follow docs/metasystem-reconciliation.md to merge the metasystem into a repository that already has it")
	}
	return false, nil
}

func markerLine(rules string) (string, bool) {
	data, err := os.ReadFile(rules)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, MarkerPrefix) {
			return line, true
		}
	}
	return "", false
}

// hookPreflight proves enrollment is feasible before any target write:
// adoption's genesis enrolls the guard, and that enrollment refuses a target
// carrying both pre-commit and pre-commit.local without the guard. This gate
// must not execute foreign hook code, so it reads the composer's marker; the
// behavioral probe runs at enrollment.
func hookPreflight(d Deps, target string) *Refusal {
	out, err := d.Git(target, true, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		if strings.Contains(err.Error(), "not a git repository") {
			return nil
		}
		// A malformed configuration in a valid repository fails the same
		// way; adopting through it would strand a half-adoption later.
		return refuse(CodeRefused, fmt.Sprintf("the target's repository shape cannot be proven: %v", err), "repair the target repository's git configuration")
	}
	_ = out
	hooks := hookDirectory(d, target)
	if hooks == "" {
		return nil
	}
	main, mainErr := os.ReadFile(filepath.Join(hooks, "pre-commit"))
	_, localErr := os.Lstat(filepath.Join(hooks, "pre-commit.local"))
	if mainErr == nil && localErr == nil &&
		!(bytes.Contains(main, []byte("git rev-parse --show-toplevel")) &&
			(bytes.Contains(main, []byte("internal pre-commit")) || bytes.Contains(main, []byte("pre-commit-guard.sh")))) {
		return refuse(CodeRefused, "the target carries both pre-commit and pre-commit.local and neither enrolls the guard",
			"compose them by hand (the guard first, then your hook), then run the same command again")
	}
	return nil
}

func hookDirectory(d Deps, target string) string {
	hooks, err := gitLine(d, target, true, "rev-parse", "--path-format=absolute", "--git-path", "hooks")
	if err != nil {
		return ""
	}
	return hooks
}

// seedLandingRef records the landing ref automatic re-arm follows: a preset
// ref is kept; otherwise the branch's upstream when it is a remote branch.
func seedLandingRef(d Deps, target string) []string {
	if existing, err := gitLine(d, target, true, "config", "--local", "--no-includes", "--get", "metasystem.steward.landing-ref"); err == nil && existing != "" {
		return []string{"adoption: landing ref was kept: metasystem.steward.landing-ref=" + existing}
	}
	branch, err := gitLine(d, target, true, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || branch == "" {
		return []string{"adoption: landing ref was not seeded: the target checkout is detached; automatic machine re-arm remains disabled until the key is configured"}
	}
	upstream, err := gitLine(d, target, true, "rev-parse", "--symbolic-full-name", "@{upstream}")
	if err != nil || upstream == "" {
		return []string{"adoption: landing ref was not seeded: target branch " + branch + " has no upstream; automatic machine re-arm remains disabled until the key is configured"}
	}
	if rest, ok := strings.CutPrefix(upstream, "refs/remotes/"); ok && strings.Contains(rest, "/") {
		if _, err := d.Git(target, true, "config", "--local", "metasystem.steward.landing-ref", upstream); err != nil {
			return []string{"adoption: landing ref was not seeded: " + err.Error()}
		}
		return nil
	}
	return []string{"adoption: landing ref was not seeded: target branch " + branch + " has upstream " + upstream + ", not refs/remotes/<remote>/<branch>"}
}

// stagePayload exports the template's tracked HEAD into stage and shapes it
// into what an adopted repository starts with.
func stagePayload(d Deps, options Options, source, prefix, stage, target string) *Refusal {
	var archive []byte
	var err error
	// A tree path after the colon resolves relative to the current
	// directory, so HEAD:<prefix> is archived from the toplevel, where the
	// prefix means what it says.
	if prefix != "" {
		top, topErr := gitLine(d, source, false, "rev-parse", "--show-toplevel")
		if topErr != nil {
			return refuse(CodeUsage, fmt.Sprintf("cannot resolve the template's toplevel: %v", topErr), "repair the template checkout")
		}
		archive, err = d.Git(top, false, "archive", "HEAD:"+strings.TrimSuffix(prefix, "/"))
	} else {
		archive, err = d.Git(source, false, "archive", "HEAD")
	}
	if err != nil {
		return refuse(CodeRefused, fmt.Sprintf("cannot export the template payload: %v", err), "repair the template checkout")
	}
	allowed := map[string]bool{}
	for _, name := range PayloadAllow {
		allowed[name] = true
	}
	if err := extract(archive, stage, func(name string) bool {
		first, _, _ := strings.Cut(name, "/")
		return allowed[first]
	}); err != nil {
		return refuse(CodeRefused, fmt.Sprintf("cannot unpack the template payload: %v", err), "repair the template checkout")
	}

	// The brain's role packet lives under records/; keep it without
	// shipping the template's history.
	kept := map[string][]byte{}
	for _, rel := range []string{"records/misc/fleet-coordinator-brain-role-packet.md"} {
		path := filepath.Join(stage, filepath.FromSlash(rel))
		if !regularFile(path) {
			return refuse(CodeRefused, "the payload is missing "+rel, "restore it in the template, commit, then run the same command again")
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return refuse(CodeRefused, readErr.Error(), "repair the template checkout")
		}
		kept[rel] = data
	}
	if err := os.RemoveAll(filepath.Join(stage, "records")); err != nil {
		return refuse(CodeRefused, err.Error(), "retry")
	}
	if refusal := dropTemplateProjectState(stage); refusal != nil {
		return refusal
	}
	// The landing owner selects required authority and preserves the
	// application's own rulings.
	rulings, err := landing.AdoptionRulings(stage, target)
	if err != nil {
		return refuse(CodeRefused, fmt.Sprintf("could not prepare the adopted landing authority: %v", err), "repair the target's memory/rulings.md or the template's landing classes")
	}
	if err := os.RemoveAll(filepath.Join(stage, "memory")); err != nil {
		return refuse(CodeRefused, err.Error(), "retry")
	}
	// plans/ ships its README and fresh goal ledgers; memory/ is rebuilt
	// with fresh living registers: an adopted project starts with its own
	// history.
	entries, err := os.ReadDir(filepath.Join(stage, "plans"))
	if err != nil && !os.IsNotExist(err) {
		return refuse(CodeRefused, err.Error(), "repair the template checkout")
	}
	for _, entry := range entries {
		if entry.Name() != "README.md" {
			if err := os.RemoveAll(filepath.Join(stage, "plans", entry.Name())); err != nil {
				return refuse(CodeRefused, err.Error(), "retry")
			}
		}
	}
	files := map[string][]byte{
		"memory/README.md":             []byte(memoryReadme),
		"memory/rulings.md":            rulings,
		"memory/instruction-ledger.md": []byte(instructionLedger),
		"memory/known-issues.md":       []byte(knownIssues),
		"records/README.md":            []byte(recordsReadme),
		"plans/goals.md":               []byte(GoalFreeLedger(options.Deps.Now())),
	}
	for rel, data := range kept {
		files[rel] = data
	}
	for rel, data := range files {
		if err := writeFile(filepath.Join(stage, filepath.FromSlash(rel)), data, 0o644); err != nil {
			return refuse(CodeRefused, err.Error(), "retry")
		}
	}
	// records/goals/ is the concluded-goal parser's directory; an empty
	// directory needs no placeholder.
	if err := os.MkdirAll(filepath.Join(stage, "records", "goals"), 0o755); err != nil {
		return refuse(CodeRefused, err.Error(), "retry")
	}
	for _, skill := range options.Enable {
		from := filepath.Join(stage, "optional-skills", skill)
		if info, statErr := os.Stat(from); skill == "" || strings.ContainsAny(skill, `/\`) || statErr != nil || !info.IsDir() {
			return refuse(CodeUsage, "unknown optional skill: "+skill, "name one of the template's optional-skills/ directories")
		}
		if err := os.Rename(from, filepath.Join(stage, "skills", skill)); err != nil {
			return refuse(CodeRefused, err.Error(), "retry")
		}
	}
	if err := os.RemoveAll(filepath.Join(stage, "optional-skills")); err != nil {
		return refuse(CodeRefused, err.Error(), "retry")
	}
	return nil
}

// templateProjectDocs are the template repository's own project state under
// docs/: its project homes (the engine reads docs/intent, docs/doctrine and
// docs/decisions as the application's own) and its history. An application
// starts with none of them and writes its own.
var templateProjectDocs = []string{"docs/intent", "docs/doctrine", "docs/decisions", "docs/journey.md", "docs/reviews"}

// stopMoveDocs is the Stop-surface protocol's directory: its README ships,
// the template's declarations for its own goals do not.
const stopMoveDocs = "docs/stop-decision-moves"

// dropTemplateProjectState removes the template's own project state from the
// staged payload, so docs/ ships documentation alone.
func dropTemplateProjectState(stage string) *Refusal {
	for _, rel := range templateProjectDocs {
		if err := os.RemoveAll(filepath.Join(stage, filepath.FromSlash(rel))); err != nil {
			return refuse(CodeRefused, err.Error(), "retry")
		}
	}
	entries, err := os.ReadDir(filepath.Join(stage, filepath.FromSlash(stopMoveDocs)))
	if err != nil && !os.IsNotExist(err) {
		return refuse(CodeRefused, err.Error(), "repair the template checkout")
	}
	for _, entry := range entries {
		if entry.Name() != "README.md" {
			if err := os.RemoveAll(filepath.Join(stage, filepath.FromSlash(stopMoveDocs), entry.Name())); err != nil {
				return refuse(CodeRefused, err.Error(), "retry")
			}
		}
	}
	return nil
}

// GoalFreeLedger is the seeded goal ledger. Its declaration's scan digest
// reproduces goal.ScanDigest's rule over the seeded plans/ set (README.md
// alone): a live example goal would parse as real work, so the ledger
// declares itself goal-free.
func GoalFreeLedger(now time.Time) string {
	digest := sha256.Sum256([]byte("README.md"))
	return "# Goals\n\n## Goal-free: declared " + now.UTC().Format("2006-01-02T15:04:05Z") + " by human over " + hex.EncodeToString(digest[:]) + "\n"
}

const memoryReadme = `# Memory

This tree holds the living registers — records that accrete and are never finished (rulings, known issues, flakes, receipts, notes); static explanation belongs in docs/ and concluded history belongs in records/.
`

const recordsReadme = `# Records

This tree holds concluded history — critique rounds, dispositions, facts, finished designs, concluded goals under records/goals/; append-only for agents and humans (the goal engine alone mutates records/goals/ under its ledger rules); live intent belongs in plans/ and living registers in memory/.
`

const instructionLedger = "# Instruction Ledger\n\n" +
	"Standing ledger of instruction changes adopted by retros (`skills/retro/SKILL.md`). Rows enter as `ADOPTED` with `Review by` naming the next retro; that retro replaces the status with a verdict: `KEPT`, `KEPT-UNPROVEN`, `AMENDED`, or `REVERTED`. Two consecutive `KEPT-UNPROVEN` verdicts revert by default.\n\n" +
	"| Id | Retro | Change | Owner doc | Evidence pattern | Expected effect | Review by | Status |\n" +
	"| --- | --- | --- | --- | --- | --- | --- | --- |\n"

const knownIssues = `# Known Issues

Accepted defects and risks, each with its consequence and the condition that reopens it. A row here is a decision, not a backlog item: it says this project knows and accepts the issue until the stated condition changes.

| Id | Date | Issue | Consequence | Reopen when | Status |
| --- | --- | --- | --- | --- | --- |
`

// extract unpacks a tar archive's regular files and directories under dir,
// keeping file modes. Entries that keep returns false for are skipped.
func extract(archive []byte, dir string, keep func(string) bool) error {
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(header.Name, "/")
		if name == "" || name == "pax_global_header" || !keep(name) {
			continue
		}
		if !filepath.IsLocal(filepath.FromSlash(name)) {
			return fmt.Errorf("archive entry escapes the payload: %s", header.Name)
		}
		path := filepath.Join(dir, filepath.FromSlash(name))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			data, err := io.ReadAll(reader)
			if err != nil {
				return err
			}
			if err := writeFile(path, data, fs.FileMode(header.Mode).Perm()); err != nil {
				return err
			}
		}
		// Symbolic links never ship: only regular files are copied.
	}
}

// payloadFiles lists stage's regular files, slash-separated and sorted.
func payloadFiles(stage string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(stage, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			rel, relErr := filepath.Rel(stage, path)
			if relErr != nil {
				return relErr
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

// seedOnce are payload paths that become the project's own state after the
// first adoption: a re-adoption neither collides on them nor rewrites them.
func seedOnce(rel, target string) bool {
	switch rel {
	case "plans/goals.md", "plans/goals-accepted.json":
		_, err := os.Stat(filepath.Join(target, "plans", "goals.md"))
		return err == nil
	}
	return false
}

func mergedLineFile(rel string) bool { return rel == ".gitattributes" || rel == ".gitignore" }

// collide names every payload path that exists in the target with different
// content: adoption never overwrites and never skips silently.
func collide(stage, target string) []string {
	files, err := payloadFiles(stage)
	if err != nil {
		return []string{"(the payload cannot be listed: " + err.Error() + ")"}
	}
	var collisions []string
	for _, rel := range files {
		if mergedLineFile(rel) || rel == "memory/rulings.md" || seedOnce(rel, target) {
			continue
		}
		existing := filepath.Join(target, filepath.FromSlash(rel))
		if _, err := os.Lstat(existing); err != nil {
			continue
		}
		staged, stagedErr := os.ReadFile(filepath.Join(stage, filepath.FromSlash(rel)))
		present, presentErr := os.ReadFile(existing)
		if stagedErr != nil || presentErr != nil || !bytes.Equal(staged, present) {
			collisions = append(collisions, rel)
		}
	}
	return collisions
}

func copyPayload(stage, target string) error {
	files, err := payloadFiles(stage)
	if err != nil {
		return err
	}
	for _, rel := range files {
		from := filepath.Join(stage, filepath.FromSlash(rel))
		if mergedLineFile(rel) {
			data, err := os.ReadFile(from)
			if err != nil {
				return err
			}
			if err := appendMissingLines(filepath.Join(target, rel), strings.Split(string(data), "\n")); err != nil {
				return err
			}
			continue
		}
		if seedOnce(rel, target) {
			continue
		}
		to := filepath.Join(target, filepath.FromSlash(rel))
		if _, err := os.Lstat(to); os.IsNotExist(err) {
			// A new path takes the staged file itself; the stage is
			// discarded afterwards, and a rename across devices falls
			// back to a copy.
			if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
				return err
			}
			if os.Rename(from, to) == nil {
				continue
			}
		}
		info, err := os.Stat(from)
		if err != nil {
			return err
		}
		if err := copyFile(from, to, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

// appendMissingLines appends each non-empty line path lacks, creating path.
func appendMissingLines(path string, lines []string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	present := map[string]bool{}
	for _, line := range strings.Split(string(existing), "\n") {
		present[line] = true
	}
	var add bytes.Buffer
	for _, line := range lines {
		if line != "" && !present[line] {
			present[line] = true
			add.WriteString(line + "\n")
		}
	}
	if add.Len() == 0 {
		if os.IsNotExist(err) {
			return writeFile(path, nil, 0o644)
		}
		return nil
	}
	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n")) {
		existing = append(existing, '\n')
	}
	return writeFile(path, append(existing, add.Bytes()...), 0o644)
}

// recordSHA fills the adoption marker's placeholder with the template SHA.
func recordSHA(rules, sha string) error {
	data, err := os.ReadFile(rules)
	if err != nil {
		return err
	}
	updated := bytes.Replace(data, []byte(Placeholder), []byte(sha), 1)
	if bytes.Equal(updated, data) {
		return nil
	}
	return writeFile(rules, updated, 0o644)
}

// copyFile writes from's bytes to to with mode, leaving identical bytes
// untouched so a re-adoption changes nothing.
func copyFile(from, to string, mode fs.FileMode) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return installFile(to, data, mode)
}

// installFile publishes data at to with mode, leaving an identical file as
// it is.
func installFile(to string, data []byte, mode fs.FileMode) error {
	if present, err := os.ReadFile(to); err == nil && bytes.Equal(present, data) {
		if info, statErr := os.Stat(to); statErr == nil && info.Mode().Perm() == mode {
			return nil
		}
		return os.Chmod(to, mode)
	}
	return writeFile(to, data, mode)
}

// writeFile writes through a sibling and renames it over path, so a
// half-written file never stands; the mode is set explicitly because the
// umask filters the creation mode.
//
// Forks are excluded while the descriptor is open: an executable written
// while another goroutine forks can stay open in the child and then refuse
// to run (text file busy).
func writeFile(path string, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	return atomicfile.WriteVolatileFile(path, data, mode)
}

func regularFile(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

func executable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0
}
