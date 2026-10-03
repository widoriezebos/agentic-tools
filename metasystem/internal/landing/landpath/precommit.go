package landpath

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/shellquote"
)

// GuardOwners are what the pre-commit guard reads.
type GuardOwners struct {
	Git Git
	// Probe is the enrollment probe nonce (METASYSTEM_GUARD_PROBE), empty
	// outside a probe.
	Probe string
	// AllowNewPlan is the per-commit new-plan acknowledgment
	// (METASYSTEM_ALLOW_NEW_PLAN=1).
	AllowNewPlan bool
	// CallerPID is the guard's own process: classification and the wrapper
	// token's ancestry walk start from it.
	CallerPID int64
	// Classify returns the caller's lease class; an error or an empty class
	// is an unavailable identity decision.
	Classify func(root string, caller int64) (string, error)
	// WrapperToken reports whether caller runs under the live wrapper the
	// token names.
	WrapperToken func(token string, caller int64) bool
	// AppendObservation appends one line to the landing observation log and
	// reports why it could not.
	AppendObservation func(root, line string) error
	// Helm reads whether the work tree's seat is at the helm (helm.Active);
	// nil is no helm. HelmYield records one yield (helm.RecordYield).
	Helm      func(workTree string) helm.State
	HelmYield func(workTree string, y helm.Yield)
}

// GuardProbeStatus is the guard's distinct exit status under an enrollment
// probe; the hook chain must propagate it.
const GuardProbeStatus = 42

var (
	ledgerPath  = regexp.MustCompile(`(^|/)plans/(goals|channel)/`)
	newPlanPath = regexp.MustCompile(`(^|/)plans/[^/]+\.md$`)
	fullTree    = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
)

// Guard is the pre-commit guard body: the ledger fence, the patch-backup
// refusal, the agent-commit wrapper proof and the new-plan acknowledgment.
// root is the metasystem installation root; the working directory of git is
// the commit's work tree. It returns the hook's exit status.
//
// A refusal is the two lines of "Messages a Person Reads": what is wrong with
// this commit, and the one command that resolves it. A helm notice (the
// helm let a check through) is printed only when the commit then proceeds.
func Guard(owners GuardOwners, root, workTree string, stdout, stderr io.Writer) int {
	// Enrollment's execution probe: the fence proves it RUNS by answering the
	// nonce and exiting distinctly, so no downstream hook does real work.
	if owners.Probe != "" {
		fmt.Fprintf(stdout, "guard-probe-ack %s\n", owners.Probe)
		return GuardProbeStatus
	}
	var notices []string
	status := guard(owners, root, workTree, stderr, &notices)
	if status == 0 {
		for _, notice := range notices {
			fmt.Fprintln(stderr, notice)
		}
	}
	return status
}

// guardChecks names each check the helm may let through, as a person reads
// it.
var guardChecks = map[string]string{"wrapper-fence": "agent-commit check", "new-plan-acknowledgment": "new-plan check"}

// allowNewPlan is the variable that acknowledges a deliberate new plan.
const allowNewPlan = "METASYSTEM_ALLOW_NEW_PLAN"

func guard(owners GuardOwners, root, workTree string, stderr io.Writer, notices *[]string) int {
	git := func(args ...string) GitResult { return owners.Git(GitCall{Dir: workTree, Args: args}) }
	if git("rev-parse", "--show-toplevel").Code != 0 {
		return 0
	}
	class, err := owners.Classify(root, owners.CallerPID)
	// yield is consulted only where a gate would refuse: at the helm in the
	// seat's primary checkout the gate records one yield, prints one line
	// and lets the guard go on.
	admits := helmAdmission(owners, workTree, git)
	yield := func(gate string) bool {
		state, _, admitted := admits()
		if !admitted {
			return false
		}
		who := state.By
		if state.Malformed != "" {
			who = "signature unreadable"
		}
		if owners.HelmYield != nil {
			owners.HelmYield(workTree, helm.Yield{Boundary: "pre-commit", Gate: gate, Would: "refuse", Subject: helmSubject(git, class, err)})
		}
		actor := state.By + "'s"
		if state.Malformed != "" {
			actor = "the helm's"
		}
		*notices = append(*notices, fmt.Sprintf("HUMAN AT THE HELM (%s): this commit passed the %s as %s act", who, guardChecks[gate], actor))
		return true
	}
	if err != nil || class == "" {
		// Observe mode stays non-refusing, but an unavailable identity
		// decision is itself evidence: keep it.
		tree := strings.TrimSpace(string(git("write-tree").Stdout))
		if !fullTree.MatchString(tree) {
			tree = "unknown"
		}
		// A record of the observation log, never a person's message.
		if appendErr := owners.AppendObservation(root, fmt.Sprintf("schemaVersion=1 boundary=pre-commit tree=%s verdict=would-refuse code=classifier-unavailable", tree)); appendErr != nil {
			fmt.Fprintf(stderr, "the commit hook couldn't tell who is committing, nor log that (%v); the commit goes ahead\n", appendErr)
		}
	} else if class != "HUMAN" {
		// Human commits are sovereign; an agent commit that could damage
		// what the wrapper protects must run under the live landing path
		// that minted the wrapper token.
		if reason := wrapperFenced(git, root); reason != "" && !owners.WrapperToken(TokenPath(root), owners.CallerPID) && !yield("wrapper-fence") {
			fmt.Fprintf(stderr, "an agent commits here only through metasystem work land (%s), so the commit was refused\n", reason)
			fmt.Fprintln(stderr, "run: metasystem work land --message FILE --staged")
			return 1
		}
	}
	staged := strings.Fields(string(git("diff", "--cached", "--name-only").Stdout))
	// The goal ledger changes only through goal verbs, which publish through
	// plumbing that never runs this hook: a staged ledger path is a hand
	// edit. This runs before both acknowledgments below, which say nothing
	// about the ledger.
	var ledger []string
	for _, path := range staged {
		if ledgerPath.MatchString(path) {
			ledger = append(ledger, path)
		}
	}
	if len(ledger) > 0 {
		fmt.Fprintf(stderr, "goal files change only through goal commands, and this commit edits %s\n", firstPath(ledger))
		fmt.Fprintf(stderr, "run: git restore --staged %s  (then make the change through the goal commands)\n", strings.Join(quoted(ledger), " "))
		return 1
	}
	var backups []string
	for _, path := range strings.Split(strings.TrimRight(string(git("diff", "--cached", "--name-only", "--diff-filter=AM").Stdout), "\n"), "\n") {
		if strings.HasSuffix(path, ".orig") {
			backups = append(backups, path)
		}
	}
	if len(backups) > 0 {
		fmt.Fprintf(stderr, "%s is a patch backup (.orig), which is never committed\n", firstPath(backups))
		fmt.Fprintf(stderr, "run: git restore --staged %s  (then repeat the commit)\n", strings.Join(quoted(backups), " "))
		return 1
	}
	if owners.AllowNewPlan {
		return 0
	}
	// An unborn branch has no peer work to capture: the initial commit
	// stages the entire payload by design.
	if git("rev-parse", "--verify", "HEAD").Code != 0 {
		return 0
	}
	var added []string
	for _, line := range strings.Split(strings.TrimRight(string(git("diff", "--cached", "--name-status", "--diff-filter=A").Stdout), "\n"), "\n") {
		_, path, found := strings.Cut(line, "\t")
		if found && newPlanPath.MatchString(path) {
			added = append(added, strings.Fields(path)...)
		}
	}
	if len(added) == 0 || yield("new-plan-acknowledgment") {
		return 0
	}
	fmt.Fprintf(stderr, "this commit adds a new plan, %s, and a new plan is often another session's file taken along\n", firstPath(added))
	fmt.Fprintf(stderr, "run: %s=1 git commit ...  (if adding it is deliberate; else git restore --staged it)\n", allowNewPlan)
	return 1
}

// quoted is paths as shell words.
func quoted(paths []string) []string {
	words := make([]string, len(paths))
	for index, path := range paths {
		words[index] = shellquote.Token(path)
	}
	return words
}

// helmAdmission answers, once per guard run, whether the helm admits this
// commit: the work tree's seat is at the helm and the work tree is the seat's
// primary checkout (PrimaryCheckout).
func helmAdmission(owners GuardOwners, workTree string, git func(args ...string) GitResult) func() (helm.State, string, bool) {
	var (
		asked, admitted bool
		state           helm.State
		commonDir       string
	)
	return func() (helm.State, string, bool) {
		if asked {
			return state, commonDir, admitted
		}
		asked = true
		if owners.Helm == nil {
			return state, commonDir, false
		}
		if state = owners.Helm(workTree); !state.Active {
			return state, commonDir, false
		}
		commonDir, admitted = PrimaryCheckout(git, workTree)
		return state, commonDir, admitted
	}
}

// PrimaryCheckout reports whether workTree is its seat's primary checkout,
// and names the common dir when it is: the work tree's own .git entry, the
// effective git dir and the common dir resolve to one directory. A linked
// worktree's git dir lies under the common dir's worktrees/; steering GIT_DIR
// cannot change the on-disk entry. git runs in workTree. Any answer that
// fails is "not the primary checkout".
func PrimaryCheckout(git func(args ...string) GitResult, workTree string) (string, bool) {
	var dirs []string
	for _, args := range [][]string{
		{"rev-parse", "--path-format=absolute", "--resolve-git-dir", filepath.Join(workTree, ".git")},
		{"rev-parse", "--path-format=absolute", "--git-dir"},
		{"rev-parse", "--path-format=absolute", "--git-common-dir"},
	} {
		answer := git(args...)
		dir := strings.TrimRight(string(answer.Stdout), "\n")
		if answer.Code != 0 || dir == "" {
			return "", false
		}
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(workTree, dir)
		}
		dir, err := filepath.EvalSymlinks(dir)
		if err != nil || len(dirs) > 0 && dir != dirs[0] {
			return "", false
		}
		dirs = append(dirs, dir)
	}
	return dirs[0], true
}

// PrimaryInstallation maps the installation at root to the same
// installation in its primary checkout: a linked worktree belongs to its
// primary checkout, the work tree whose .git is the common dir. It names
// that checkout and the installation in it; for a primary checkout they are
// root's own. Both are "" when the common dir is not a checkout's .git (a
// bare repository's worktree). problem says, in plain words, why the
// repository's directories could not be mapped. git runs in root.
func PrimaryInstallation(git func(args ...string) GitResult, root string) (checkout, installation, problem string) {
	common := git("rev-parse", "--path-format=absolute", "--git-common-dir")
	top := git("rev-parse", "--show-toplevel")
	if common.Code != 0 || top.Code != 0 {
		return "", "", "the repository's directories cannot be read"
	}
	commonDir := strings.TrimRight(string(common.Stdout), "\n")
	if filepath.Base(commonDir) != ".git" {
		return "", "", ""
	}
	prefix, err := filepath.Rel(strings.TrimRight(string(top.Stdout), "\n"), root)
	if err != nil || !filepath.IsLocal(prefix) && prefix != "." {
		return "", "", "the installation lies outside its work tree"
	}
	checkout = filepath.Dir(commonDir)
	return checkout, filepath.Join(checkout, prefix), ""
}

// helmSubject names what a helm yield admitted: the branch HEAD names, the
// index tree and the caller's class.
func helmSubject(git func(args ...string) GitResult, class string, classErr error) string {
	branch := "unavailable"
	if head := git("symbolic-ref", "--quiet", "HEAD"); head.Code == 0 {
		branch = strings.TrimSpace(string(head.Stdout))
	}
	tree := strings.TrimSpace(string(git("write-tree").Stdout))
	if !fullTree.MatchString(tree) {
		tree = "unknown"
	}
	if classErr != nil || class == "" {
		class = "unavailable"
	}
	return fmt.Sprintf("branch=%s tree=%s class=%s", branch, tree, class)
}

// ledgerBranch is the dedicated single-machine ledger branch
// (internal/goal.LocalLedgerBranch), named here without importing the goal
// package into the landing path.
const ledgerBranch = "refs/heads/metasystem/goals"

// wrapperFenced says why an agent commit here needs the wrapper token, or ""
// when it cannot damage what the token protects. The token (D-6 of "One
// writer, safe readers", introduced with d2d33e9fb) keeps a checkout a seat
// holds to one writer, and keeps agent commits off the published line except
// through the landing path. A feature branch of a clone no seat holds is
// neither: its commits reach the line only by a later landing, which proves
// them again. Anything the guard cannot read stays fenced.
func wrapperFenced(git func(args ...string) GitResult, root string) string {
	head := git("symbolic-ref", "--quiet", "HEAD")
	branch := strings.TrimSpace(string(head.Stdout))
	if head.Code != 0 || !strings.HasPrefix(branch, "refs/heads/") {
		return "HEAD names no branch"
	}
	published := "refs/heads/main"
	config := git("config", "--get", "goal.sync-branch")
	switch value := strings.TrimSpace(string(config.Stdout)); {
	case config.Code == 1 && value == "":
	case config.Code == 0 && strings.HasPrefix(value, "refs/"):
		published = value
	default:
		return "goal.sync-branch cannot be read"
	}
	if branch == published || branch == ledgerBranch {
		return branch + " is the published line"
	}
	installations := []string{root}
	// A linked worktree belongs to its primary checkout: a seat holding
	// that checkout holds its worktrees too.
	_, primary, problem := PrimaryInstallation(git, root)
	if problem != "" {
		return problem
	}
	if primary != "" {
		installations = append(installations, primary)
	}
	for _, installation := range installations {
		// artifacts/agents/mains is where a seat's sessions announce and
		// its checkout lease lives.
		_, err := os.Stat(filepath.Join(installation, "artifacts", "agents", "mains"))
		if err == nil {
			return "a seat holds this checkout"
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "whether a seat holds this checkout cannot be read"
		}
	}
	return ""
}
