package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
)

// stagedLandingOptions are the options of work land's staged form: a change
// a person or agent made by hand, landed with its message and declarations.
var stagedLandingOptions = []string{"message", "staged", "path", "chain", "recertification", "test-receipt",
	"direct-fix", "revert-of", "root-job", "tests", "allow-new-plan", "skip-transport", "local"}

// stagedLandingFlags are those options as work land declares them.
var stagedLandingFlags = []intentFlag{
	{name: "message", value: "FILE", usage: "land a hand-made change with this commit message (- reads it from standard input)"},
	{name: "staged", usage: "with --message: land exactly the staged set"},
	{name: "path", value: "PATH", repeat: true, usage: "with --message: stage and land these paths (relative to the installation)"},
	{name: "chain", value: "J", advanced: true, usage: "with --message: the reviewed implementation chain the change lands"},
	{name: "recertification", value: "RECORD", advanced: true, usage: "with --chain: the canonical recertification record"},
	{name: "test-receipt", value: "PATH", advanced: true, usage: "with --chain or --direct-fix tier-1: the receipt for this exact candidate"},
	{name: "direct-fix", value: "CLASS", advanced: true, usage: "with --message: register-carriage, exact-revert or tier-1"},
	{name: "revert-of", value: "COMMIT", advanced: true, usage: "with --direct-fix exact-revert: the commit reverted"},
	{name: "root-job", value: "J", advanced: true, usage: "with --direct-fix tier-1: the root implementer job"},
	{name: "tests", value: "COMMAND", advanced: true, usage: "with --direct-fix tier-1: the legacy test command a receipt is made from"},
	{name: "allow-new-plan", advanced: true, usage: "with --message: the landing deliberately adds a new plan file"},
	{name: "skip-transport", advanced: true, usage: "with --message: do not mirror origin to the transport remote"},
	{name: "local", advanced: true, usage: "with --message --staged: commit the staged set through the commit boundary only; nothing is fetched or pushed"},
}

// runIntentLandStaged lands a hand-made change through the landing path:
// stage (or take the staged set), commit through the commit boundary, rebase
// onto origin, prove, push and mirror.
func runIntentLandStaged(inv *intentInvocation) int {
	refuse := func(summary string) int {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: summary + "; nothing was done",
			Decision: "metasystem work land [G] --message FILE (--staged | --path P...)"})
	}
	if len(inv.input.args) > 1 {
		return refuse("work land --message takes at most one goal")
	}
	for _, other := range append([]string{"through", "queue-only", "lineage", "using-exception"}, exceptionOptions...) {
		if inv.input.has(other) {
			return refuse("--" + other + " belongs to landing a goal's reviewed work, not a hand-made change")
		}
	}
	request := landpath.LandRequest{
		MessageFile: inv.input.text("message"), StagedOnly: inv.input.switched("staged"),
		Pathspecs: inv.input.values["path"], Chain: inv.input.text("chain"), Recertification: inv.input.text("recertification"),
		TestReceipt: inv.input.text("test-receipt"), DirectFix: inv.input.text("direct-fix"), RevertOf: inv.input.text("revert-of"),
		RootJob: inv.input.text("root-job"), Tests: inv.input.text("tests"),
		AllowNewPlan: inv.input.switched("allow-new-plan"), SkipTransport: inv.input.switched("skip-transport"),
		// The seat's lineage is read once at this entry's boundary and named
		// on the request; nothing below reads the environment for it.
		OwnerLineage: os.Getenv("METASYSTEM_OWNER_LINEAGE"),
	}
	if len(inv.input.args) == 1 {
		ref, problem := inv.resolveWorkRef(inv.input.args[0], []string{refGoal})
		if problem != nil {
			return inv.render(*problem)
		}
		request.Goal, request.GoalSet = ref.id, true
	}
	if request.MessageFile == "-" {
		message, err := io.ReadAll(os.Stdin)
		if err != nil {
			return refuse("the commit message cannot be read from standard input: " + err.Error())
		}
		request.Message = message
	} else if !filepath.IsAbs(request.MessageFile) {
		request.MessageFile = filepath.Join(inv.cwd, request.MessageFile)
	}
	path := inv.cwd
	if inv.input.has("repo") {
		path = inv.input.text("repo")
		if !filepath.IsAbs(path) {
			path = filepath.Join(inv.cwd, path)
		}
	}
	layout, err := inv.owners.resolver.ResolveLayout(path)
	if err != nil {
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary:  notAnInstallation(path, err),
			Decision: "run this inside the repository, or name it with --repo PATH"})
	}
	request.Root = layout.InstallationRoot
	owners := landingPathOwners()
	if request.GoalSet && !inv.input.switched("local") {
		// A hand-made change landed in a goal's name meets the goal's gate
		// (g1-s70 D2); --local publishes nothing.
		// The gate's facts are lines on the synced ledger; an installation
		// with none lands through the landing path's own held check as it
		// did before.
		stateRoot, rootErr := inv.owners.resolver.RootForInstallation(layout.InstallationRoot)
		inv.layout, inv.stateRoot = layout, stateRoot
		if rootErr == nil && converted(stateRoot) {
			if refused := inv.admitLanding([]intentTarget{{Kind: "goal", ID: request.Goal}}, request.Goal, inv.intentBranchTip(request.Goal)); refused != nil {
				return inv.render(*refused)
			}
			// Admission is not the last word: the same gate is read again
			// immediately before each push.
			owners.LandingGate = inv.pushGate()
			// The goal's workspaces this landing ends: recorded before each
			// push, released once it succeeded (disk-lifetimes Part B 3.6).
			owners.RecordRelease = func(commit, branch string) error { return inv.recordStagedRelease(request.Goal, commit, branch) }
			owners.ReleaseLanded = func(commit string) { inv.releaseStagedLanding(request.Goal, commit) }
		}
	}
	stdout, stderr := inv.stdout, inv.stderr
	var captured bytes.Buffer
	if inv.input.switched("json") {
		stdout, stderr = &captured, &captured
	}
	targets := []intentTarget{}
	if request.GoalSet {
		targets = append(targets, intentTarget{Kind: "goal", ID: request.Goal})
	}
	// With a landing lane on this host the change joins it (U11b); the hand
	// path stays for a checkout with no lane, for --local (it publishes
	// nothing) and for a recertified chain, whose proof binds a frozen origin
	// target a batch would move.
	if !inv.input.switched("local") && request.Recertification == "" {
		inv.layout = layout
		landingRoot, configured, refused := inv.landingBatchRoot(targets)
		if refused != nil {
			return inv.render(*refused)
		}
		if configured {
			result := inv.landChange(request, owners, landingRoot, targets, stdout, stderr)
			if inv.input.switched("json") {
				if data, ok := result.Data.(map[string]any); ok {
					data["output"] = captured.String()
				}
			}
			return inv.render(result)
		}
	}
	var status int
	if inv.input.switched("local") {
		status = landStagedLocally(request, stdout, stderr)
	} else {
		status = inv.delivery().runLandPath(owners, request, stdout, stderr)
	}
	data := map[string]any{"exitCode": status}
	if inv.input.switched("json") {
		data["output"] = captured.String()
	}
	if status != 0 {
		return inv.render(intentResult{Outcome: intentRefused, code: status, Targets: targets, Data: data,
			Summary: fmt.Sprintf("the landing stopped with exit %d; the lines above name the cause", status)})
	}
	head := strings.TrimSpace(string(landingPathGit(landpath.GitCall{Dir: request.Root, Args: []string{"rev-parse", "HEAD"}}).Stdout))
	data["commit"] = head
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: data,
		Summary: fmt.Sprintf("landed %s", head)})
}

// landStagedLocally commits the staged set through the commit boundary alone:
// the former commit.sh without --push, for a caller that publishes itself.
func landStagedLocally(request landpath.LandRequest, stdout, stderr io.Writer) int {
	if !request.StagedOnly || len(request.Pathspecs) > 0 || request.Tests != "" || request.SkipTransport {
		fmt.Fprintln(stderr, "land refused: --local commits exactly the staged set: use --staged, without --path, --tests or --skip-transport")
		return 2
	}
	message := request.MessageFile
	if message == "-" {
		file, done, err := diskstore.ScratchFile("metasystem-local-commit-message-*")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer done()
		_, writeErr := file.Write(request.Message)
		if closeErr := file.Close(); writeErr != nil || closeErr != nil {
			fmt.Fprintln(stderr, "land refused: the commit message cannot be written")
			return 1
		}
		message = file.Name()
	}
	return landpath.Commit(landingPathOwners(), landpath.CommitRequest{Root: request.Root, Chain: request.Chain,
		DirectFix: request.DirectFix, RevertOf: request.RevertOf, Goal: request.Goal, GoalSet: request.GoalSet,
		RootJob: request.RootJob, TestReceipt: request.TestReceipt, Recertification: request.Recertification,
		MessageFile: message, OwnerLineage: request.OwnerLineage, AllowNewPlan: request.AllowNewPlan}, stdout, stderr)
}
