package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// helmCommit is one commit made at the helm, matched to a pre-commit yield by
// its tree.
type helmCommit struct{ sha, tree, subject, branch, where string }

// helmCatchUp is what return does after the signature is gone: the readback
// of the acts and commits made at the helm, the two questions when a person
// is at the terminal, what each answer runs, and the ledger fence and
// supervision recovery. Every part is best-effort; a failure is one line and
// never changes the exit status. lines holds what return already says; when
// the questions are asked they are printed first, so the person sees them.
func (inv *intentInvocation) helmCatchUp(seat helm.Seat, record helm.Record, since time.Time, readable bool, lines []string) (out []string, printed bool) {
	owners := inv.owners.helm.withDefaults()
	if owners.ask == nil {
		owners.ask = helmAsk(owners.stdin, inv.stdout)
	}
	yields, err := helmYieldsSince(seat, since)
	if err != nil {
		lines = append(lines, "acts at the helm: unavailable: "+err.Error())
	}
	commits, readback := helmReadback(owners, seat, since, yields)
	lines = append(lines, readback...)

	root := inv.helmRoot()
	candidate := helmGoalCandidate(inv, owners, root, yields)
	conclusion := fmt.Sprintf("landed at the helm by %s", record.By)
	if len(commits) > 0 {
		shas := make([]string, 0, len(commits))
		for _, commit := range commits {
			shas = append(shas, commit.sha[:min(7, len(commit.sha))])
		}
		conclusion += fmt.Sprintf(": %d commits %s", len(commits), strings.Join(shas, ", "))
	}
	patch, brief := "", ""
	if len(commits) > 0 {
		var problem string
		if patch, brief, problem = helmWriteReadInputs(owners, seat, record, commits); problem != "" {
			lines = append(lines, "the patch for a read: "+problem)
			patch, brief = "", ""
		}
	}

	interactive := readable && owners.stdinTerminal() && !inv.input.switched("json")
	if !interactive {
		named := candidate
		if named == "" {
			named = "G"
		}
		lines = append(lines, "every goal stays open; to conclude one: "+shellCommand([]string{"metasystem", "goal", "done", named, "--reason", conclusion, "--by", record.By}))
		if patch != "" {
			lines = append(lines, "to ask independent readers for feedback: "+shellCommand([]string{"metasystem", "work", "review", "--patch", patch, "--brief", brief}))
		}
	} else {
		for _, line := range lines {
			fmt.Fprintln(inv.stdout, line)
		}
		lines, printed = nil, true
		lines = append(lines, inv.helmAnswers(owners, seat, record, root, candidate, conclusion, patch, brief)...)
	}

	if root == "" {
		lines = append(lines, "the ledger hook: the installation cannot be found")
	} else if err := owners.fence(root); err != nil {
		lines = append(lines, "the ledger hook: "+err.Error())
	} else {
		lines = append(lines, "the ledger hook is enrolled")
	}
	if scope, _, problem := inv.selectProcessScope(); problem != nil {
		lines = append(lines, "supervision re-arms at the next turn end (the Stop hook arms it); it cannot be recovered from here: "+problem.Summary)
	} else {
		lines = append(lines, owners.recover(scope))
	}
	return lines, printed
}

// helmAnswers asks the two questions and runs what each answer names.
func (inv *intentInvocation) helmAnswers(owners helmOwners, seat helm.Seat, record helm.Record, root, candidate, conclusion, patch, brief string) []string {
	var lines []string
	id := candidate
	if candidate != "" {
		if answer, _ := owners.ask(fmt.Sprintf("Conclude goal %s with these commits? [y/N] ", candidate)); !helmYes(answer) {
			id = ""
		}
	} else if answer, _ := owners.ask("Conclude a goal with these commits? [goal id / Enter keeps every goal open] "); strings.TrimSpace(answer) != "" {
		id = strings.TrimSpace(answer)
	}
	if id != "" {
		if answer, _ := owners.ask(fmt.Sprintf("Conclusion [Enter: %s] ", conclusion)); strings.TrimSpace(answer) != "" {
			conclusion = strings.TrimSpace(answer)
		}
		proof, err := humanauthority.HelmProof(root, humanauthority.HelmGrant{By: record.By, Since: record.At, Checkout: seat.CommonDir}, owners.now())
		if err != nil {
			lines = append(lines, "goal done "+id+": "+err.Error()+"; the goal stays open")
		} else {
			lines = append(lines, helmActLine("goal done "+id, owners.done(inv, id, record.By, conclusion, proof)))
		}
	} else {
		lines = append(lines, "every goal stays open")
	}
	if patch == "" {
		return lines
	}
	if answer, _ := owners.ask("Ask independent readers for feedback on these commits? [y/N] "); helmYes(answer) {
		lines = append(lines, helmActLine("read", owners.read(inv, patch, brief)))
	}
	return lines
}

func helmYes(answer string) bool {
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

// helmActLine is one answer's own confirmation, or its reason in one line.
func helmActLine(act string, result intentResult) string {
	summary := strings.TrimSpace(result.Summary)
	if result.Outcome == intentConfirmed || result.Outcome == intentUnchanged {
		if summary == "" {
			summary = result.Outcome
		}
		return act + ": " + summary
	}
	return act + ": " + result.Outcome + ": " + summary
}

// helmRoot is the installation's state root, or "" when it cannot be found.
func (inv *intentInvocation) helmRoot() string {
	layout, err := inv.owners.resolver.ResolveLayout(inv.helmPath())
	if err != nil {
		return ""
	}
	root, err := inv.owners.resolver.RootForInstallation(layout.InstallationRoot)
	if err != nil {
		return ""
	}
	return root
}

// helmYieldsSince are the yields recorded since the take, as helmYieldCount
// counts them.
func helmYieldsSince(seat helm.Seat, since time.Time) ([]helm.Yield, error) {
	file, err := os.Open(seat.Yields)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var yields []helm.Yield
	for scanner := bufio.NewScanner(file); scanner.Scan(); {
		var yield helm.Yield
		if json.Unmarshal(scanner.Bytes(), &yield) == nil && !yield.At.Before(since) {
			yields = append(yields, yield)
		}
	}
	return yields, nil
}

// helmSubjectField is one name=value of a yield's subject; a value runs to
// the next " name=" of the known names.
func helmSubjectField(subject, name string) string {
	_, rest, found := strings.Cut(subject, name+"=")
	if !found {
		return ""
	}
	end := len(rest)
	for _, next := range []string{" branch=", " tree=", " class=", " cwd=", " verb="} {
		if index := strings.Index(rest, next); index >= 0 && index < end {
			end = index
		}
	}
	return rest[:end]
}

// helmReadback matches the pre-commit yields to the commits made since the
// take by tree id, and names the acts the person proof admitted.
func helmReadback(owners helmOwners, seat helm.Seat, since time.Time, yields []helm.Yield) ([]helmCommit, []string) {
	var branches []string
	trees := map[string][]string{}
	var acts []string
	for _, yield := range yields {
		switch yield.Boundary {
		case "pre-commit":
			branch, tree := helmSubjectField(yield.Subject, "branch"), helmSubjectField(yield.Subject, "tree")
			if !slices.Contains(branches, branch) {
				branches = append(branches, branch)
			}
			if !slices.Contains(trees[branch], tree) {
				trees[branch] = append(trees[branch], tree)
			}
		case "person-proof":
			acts = append(acts, fmt.Sprintf("%s (%s)", helmSubjectField(yield.Subject, "verb"), helmSubjectField(yield.Subject, "class")))
		}
	}
	var commits []helmCommit
	var lines []string
	for _, branch := range branches {
		short := strings.TrimPrefix(branch, "refs/heads/")
		log, err := owners.git(seat.Checkout, "log", "--format=%H%x1f%T%x1f%s", "--since="+since.UTC().Format(time.RFC3339), branch)
		if err != nil {
			lines = append(lines, "commits at the helm on "+short+": unavailable: "+firstLine(err.Error()))
			continue
		}
		upstream := "origin/" + short
		if named, err := owners.git(seat.Checkout, "rev-parse", "--abbrev-ref", "--symbolic-full-name", short+"@{upstream}"); err == nil && strings.TrimSpace(named) != "" {
			upstream = strings.TrimSpace(named)
		}
		var found []helmCommit
		matched := map[string]bool{}
		for _, line := range strings.Split(strings.TrimSpace(log), "\n") {
			fields := strings.SplitN(line, "\x1f", 3)
			if len(fields) != 3 || !slices.Contains(trees[branch], fields[1]) || matched[fields[1]] {
				continue
			}
			matched[fields[1]] = true
			commit := helmCommit{sha: fields[0], tree: fields[1], subject: fields[2], branch: branch, where: "not pushed"}
			if _, err := owners.git(seat.Checkout, "merge-base", "--is-ancestor", commit.sha, upstream); err == nil {
				commit.where = "on " + upstream
			}
			found = append([]helmCommit{commit}, found...)
		}
		var named []string
		for _, commit := range found {
			named = append(named, fmt.Sprintf("%s %s (%s)", commit.sha[:min(7, len(commit.sha))], commit.subject, commit.where))
		}
		if len(named) > 0 {
			lines = append(lines, "commits at the helm on "+short+": "+strings.Join(named, ", "))
		}
		if missing := len(trees[branch]) - len(found); missing > 0 {
			lines = append(lines, fmt.Sprintf("admitted at the helm on %s, no commit found: %d", short, missing))
		}
		commits = append(commits, found...)
	}
	if len(acts) > 0 {
		lines = append(lines, "acts at the helm: "+strings.Join(acts, ", "))
	}
	return commits, lines
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return line
}

// helmGoalCandidate is the goal a pre-commit yield's branch names
// (refs/heads/goal/<id>), else the one live goal the seat's holder claims.
func helmGoalCandidate(inv *intentInvocation, owners helmOwners, root string, yields []helm.Yield) string {
	for _, yield := range yields {
		if id, found := strings.CutPrefix(helmSubjectField(yield.Subject, "branch"), "refs/heads/goal/"); yield.Boundary == "pre-commit" && found && id != "" {
			return id
		}
	}
	if root == "" || inv.owners.dependencies.endpoint == nil || inv.owners.commandNow == nil {
		return ""
	}
	holder, err := owners.holder(root)
	if err != nil || holder.OwnerLineage == "" {
		return ""
	}
	machine, err := owners.machine(root)
	if err != nil {
		return ""
	}
	inv.stateRoot = root
	projection, _, problem := inv.projection()
	if problem != nil {
		return ""
	}
	var claimed []string
	for id, file := range projection.Tree.Live {
		if file.State == goal.StateClaimed && file.Claimed != nil && file.Claimed.Machine == machine && file.Claimed.Lineage == holder.OwnerLineage {
			claimed = append(claimed, id)
		}
	}
	if len(claimed) != 1 {
		return ""
	}
	return claimed[0]
}

// helmWriteReadInputs writes the patch of the helm commits and the brief a
// read requires beside it, under the seat's metasystem directory.
func helmWriteReadInputs(owners helmOwners, seat helm.Seat, record helm.Record, commits []helmCommit) (string, string, string) {
	stamp := strings.NewReplacer(":", "", "-", "").Replace(record.At)
	patch := filepath.Join(seat.Dir, "helm-"+stamp+".patch")
	brief := filepath.Join(seat.Dir, "helm-"+stamp+".brief.md")
	var diff strings.Builder
	var named []string
	for _, branch := range helmBranches(commits) {
		var oldest, newest helmCommit
		for _, commit := range commits {
			if commit.branch != branch {
				continue
			}
			if oldest.sha == "" {
				oldest = commit
			}
			newest = commit
			named = append(named, commit.sha[:min(7, len(commit.sha))]+" "+commit.subject)
		}
		base := oldest.sha + "^"
		if _, err := owners.git(seat.Checkout, "rev-parse", "--verify", "--quiet", base); err != nil {
			base = "4b825dc642cb6eb9a060e54bf8d69288fbee4904" // the empty tree: the oldest is a root commit
		}
		text, err := owners.git(seat.Checkout, "diff", base, newest.sha)
		if err != nil {
			return "", "", "unavailable: " + firstLine(err.Error())
		}
		diff.WriteString(text)
	}
	if err := os.MkdirAll(seat.Dir, 0o700); err != nil {
		return "", "", err.Error()
	}
	briefText := "Commits made at the helm by " + record.By + ": " + strings.Join(named, "; ") + "\n" +
		"Scope: the diff in the patch " + patch + "\n" +
		"Purpose: feedback only; no goal is bound\n"
	if err := os.WriteFile(patch, []byte(diff.String()), 0o600); err != nil {
		return "", "", err.Error()
	}
	if err := os.WriteFile(brief, []byte(briefText), 0o600); err != nil {
		return "", "", err.Error()
	}
	return patch, brief, ""
}

func helmBranches(commits []helmCommit) []string {
	var branches []string
	for _, commit := range commits {
		if !slices.Contains(branches, commit.branch) {
			branches = append(branches, commit.branch)
		}
	}
	return branches
}

// helmReturnDone is the answer yes to the conclusion question: goal done in
// this process, as runIntentDone calls its owner, with the person proof the
// holder's take was: the helm proof built from the removed signature.
func helmReturnDone(inv *intentInvocation, id, by, reason string, proof humanauthority.Proof) intentResult {
	if problem := inv.selectRoot(); problem != nil {
		return *problem
	}
	if !proof.ValidFor(inv.stateRoot) && proof.Helm != nil {
		if rebound, err := humanauthority.HelmProof(inv.stateRoot, *proof.Helm, proof.CheckedAt); err == nil {
			proof = rebound
		}
	}
	owned := *inv
	owned.owners.dependencies.proveHuman = func(string, int64, humanauthority.Reader, time.Time) (humanauthority.Proof, error) {
		return proof, nil
	}
	args := []string{"--root", inv.stateRoot, "--id", id, "--conclude", reason, "--by", by}
	return owned.ownerCall(inv.targets(id), func(dependencies syncRequestDependencies) int {
		code, _ := trySyncMutationWithCompletion("done", args, owned.owners.commandNow, dependencies, owned.owners.parkBranchCheck, owned.owners.completion)
		return code
	}, func() intentResult { return owned.afterGoalAct(id, "done") })
}

// helmReturnRead is the answer yes to the read question: the standalone read
// runIntentReviewDiagnostic starts, on the patch and brief return wrote,
// feedback only.
func helmReturnRead(inv *intentInvocation, patch, brief string) intentResult {
	if problem := inv.selectRoot(); problem != nil {
		return *problem
	}
	result, err := inv.unitRunner().StartRead(launch.ReadRequest{Directory: inv.cwd, Patch: patch, Brief: brief})
	return inv.diagnosticReadResult(result, err, patch)
}

// helmAsk asks on the invocation's own writer and reads one answer line from
// the standard input it was given.
func helmAsk(stdin io.Reader, stdout io.Writer) func(string) (string, bool) {
	input := bufio.NewReader(stdin)
	return func(prompt string) (string, bool) {
		fmt.Fprint(stdout, prompt)
		answer, err := input.ReadString('\n')
		return strings.TrimSpace(answer), err == nil
	}
}

// helmRecover is the recover path system start --if-down takes; its own
// report is kept to one line.
func helmRecover(scope processScope) string {
	var report bytes.Buffer
	if runUpWith([]string{"--metasystem-root", scope.Installation, "--repo", scope.Checkout, "--recover-only", "--if-down"}, stateroot.RepositoryTop, &report, &report) == 0 {
		return "supervision: recovered"
	}
	line := "supervision re-arms at the next turn end (the Stop hook arms it)"
	if last := strings.TrimSpace(report.String()); last != "" {
		lines := strings.Split(last, "\n")
		line += "; the recovery said: " + lines[len(lines)-1]
	}
	return line
}

// helmHolder is the seat's recorded lease holder.
func helmHolder(root string) (lease.CurrentHolderView, error) { return lease.CurrentHolder(root) }
