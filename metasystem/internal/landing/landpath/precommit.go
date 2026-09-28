package landpath

import (
	"fmt"
	"io"
	"regexp"
	"strings"
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
func Guard(owners GuardOwners, root, workTree string, stdout, stderr io.Writer) int {
	// Enrollment's execution probe: the fence proves it RUNS by answering the
	// nonce and exiting distinctly, so no downstream hook does real work.
	if owners.Probe != "" {
		fmt.Fprintf(stdout, "guard-probe-ack %s\n", owners.Probe)
		return GuardProbeStatus
	}
	git := func(args ...string) GitResult { return owners.Git(GitCall{Dir: workTree, Args: args}) }
	if git("rev-parse", "--show-toplevel").Code != 0 {
		return 0
	}
	class, err := owners.Classify(root, owners.CallerPID)
	if err != nil || class == "" {
		// Observe mode stays non-refusing, but an unavailable identity
		// decision is itself evidence: keep it.
		tree := strings.TrimSpace(string(git("write-tree").Stdout))
		if !fullTree.MatchString(tree) {
			tree = "unknown"
		}
		line := fmt.Sprintf("schemaVersion=1 boundary=pre-commit tree=%s verdict=would-refuse code=classifier-unavailable", tree)
		if appendErr := owners.AppendObservation(root, line); appendErr != nil {
			fmt.Fprintf(stderr, "pre-commit guard: classifier unavailable and %v\n", appendErr)
		}
	} else if class != "HUMAN" {
		// Human commits are sovereign; an agent commit must run under the
		// live landing path that minted the wrapper token.
		if !owners.WrapperToken(TokenPath(root), owners.CallerPID) {
			fmt.Fprintln(stderr, "pre-commit guard: an agent commit goes through metasystem work land; the live wrapper ancestry token is missing")
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
		fmt.Fprintln(stderr, "pre-commit guard: goal files change only through goal verbs; hand edits go through goal sync:")
		for _, path := range ledger {
			fmt.Fprintf(stderr, "  %s\n", path)
		}
		return 1
	}
	backups := false
	for _, path := range strings.Split(strings.TrimRight(string(git("diff", "--cached", "--name-only", "--diff-filter=AM").Stdout), "\n"), "\n") {
		if strings.HasSuffix(path, ".orig") {
			fmt.Fprintf(stderr, "pre-commit guard: refusing %s: patch backups are never tracked\n", path)
			backups = true
		}
	}
	if backups {
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
	if len(added) == 0 {
		return 0
	}
	fmt.Fprintln(stderr, "pre-commit guard: refusing to commit NEW plan file(s):")
	for _, path := range added {
		fmt.Fprintf(stderr, "  %s\n", path)
	}
	fmt.Fprintln(stderr, "A new plan in the staged set is how a peer session's file gets committed")
	fmt.Fprintln(stderr, "by accident (0b9ca1b). If this addition is deliberate, acknowledge it:")
	fmt.Fprintln(stderr, "  METASYSTEM_ALLOW_NEW_PLAN=1 git commit ...")
	return 1
}
