package landing

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathclass"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// ReceiptLineParams names one prospective landing for the receipt-line
// check. CandidateTree is the whole-project staged tree, not the
// installation subtree the other landing judges read: the receipt ledger
// lives at the application root, which an adopted nested installation
// keeps outside its own subtree, so this check speaks repository paths
// inside and installation-relative paths in everything it reports.
type ReceiptLineParams struct {
	RepoRoot      string
	CandidateTree string
	Goal          string
	DirectFix     string
}

// ReceiptLineDecision is the check's answer. Outcome is pass, exempt or
// refused; Reason is its one-word cause; Detail is the sentence land.sh
// shows; Command is the one command that writes the missing line. Ledger
// and CodePaths are paths from the installation root, the paths a
// landing's caller names.
type ReceiptLineDecision struct {
	SchemaVersion int      `json:"schemaVersion"`
	Outcome       string   `json:"outcome"`
	Reason        string   `json:"reason"`
	Detail        string   `json:"detail"`
	Ledger        string   `json:"ledger"`
	Goal          string   `json:"goal,omitempty"`
	Command       string   `json:"command,omitempty"`
	CodePaths     []string `json:"codePaths,omitempty"`
}

const (
	ReceiptLineOutcomePass    = "pass"
	ReceiptLineOutcomeExempt  = "exempt"
	ReceiptLineOutcomeRefused = "refused"

	receiptLineListedPaths = 6
)

// ObserveReceiptLine decides whether a landing that changes code appends,
// in the same commit, the RECEIPT line that describes it to the receipt
// ledger (development/project-rules-local.md: bookkeeping-only commits
// hide the ratio of records to evidence). A landing that changes records
// or ledgers only, the receipt ledger alone, or an exact revert (whose
// tree must be the reverted commit's exact inverse) is exempt, as is a
// checkout that keeps no receipt ledger in git. A landing that removes
// the ledger is refused whatever else it carries, since the next landing
// would otherwise be exempt for the ledger's absence. With a goal named,
// the appended line must carry that goal; without one, any appended
// RECEIPT line satisfies the check.
func ObserveReceiptLine(params ReceiptLineParams) (ReceiptLineDecision, error) {
	workspace := gittree.Workspace{Dir: params.RepoRoot}
	location, err := locateReceiptLedger(workspace, params.RepoRoot)
	if err != nil {
		return ReceiptLineDecision{}, err
	}
	decision := ReceiptLineDecision{SchemaVersion: 1, Ledger: location.fromInstallation, Goal: params.Goal}
	baseTreeBytes, err := landingGit(params.RepoRoot, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return ReceiptLineDecision{}, err
	}
	baseTree := strings.TrimSpace(string(baseTreeBytes))
	baseLedger, present, err := workspace.FileAt(baseTree, location.repositoryPath)
	if err != nil {
		return ReceiptLineDecision{}, err
	}
	if !present {
		return receiptLineExempt(decision, "no-receipt-ledger",
			fmt.Sprintf("the checkout keeps no receipt ledger at %s", location.fromInstallation)), nil
	}
	changed, err := workspace.ChangedPaths(baseTree, params.CandidateTree)
	if err != nil {
		return ReceiptLineDecision{}, err
	}
	if len(changed) == 0 {
		return receiptLineExempt(decision, "no-changes", "the landing changes no path"), nil
	}
	candidateLedger, present, err := workspace.FileAt(params.CandidateTree, location.repositoryPath)
	if err != nil {
		return ReceiptLineDecision{}, err
	}
	if !present {
		decision.Outcome = ReceiptLineOutcomeRefused
		decision.Reason = "receipt-ledger-removed"
		decision.Detail = fmt.Sprintf("the landing removes the receipt ledger %s", location.fromInstallation)
		return decision, nil
	}
	if params.DirectFix == "exact-revert" {
		return receiptLineExempt(decision, "exact-revert",
			"an exact revert is the reverted commit's inverse tree and carries its receipt in the commit that follows"), nil
	}
	codePaths, err := receiptLineCodePaths(workspace, location, baseTree, changed)
	if err != nil {
		return ReceiptLineDecision{}, err
	}
	if len(codePaths) == 0 {
		if len(changed) == 1 && changed[0] == location.repositoryPath {
			return receiptLineExempt(decision, "receipt-only", "the landing appends to the receipt ledger alone"), nil
		}
		return receiptLineExempt(decision, "records-only", "the landing changes records only"), nil
	}
	for _, codePath := range codePaths {
		decision.CodePaths = append(decision.CodePaths, location.pathFromInstallation(codePath))
	}
	listed := receiptLineListed(decision.CodePaths)
	var otherGoals []string
	for _, line := range addedLines(baseLedger, candidateLedger) {
		fields := strings.Split(line, "|")
		if len(fields) < 3 || fields[2] != "RECEIPT" {
			continue
		}
		lineGoal := receiptLineGoal(fields)
		if params.Goal == "" || lineGoal == params.Goal {
			decision.Outcome = ReceiptLineOutcomePass
			decision.Reason = "receipt-line-appended"
			decision.Detail = fmt.Sprintf("the landing appends the RECEIPT line of epoch %s (goal %s) to %s",
				fields[0], receiptLineGoalWord(lineGoal), location.fromInstallation)
			return decision, nil
		}
		otherGoals = append(otherGoals, receiptLineGoalWord(lineGoal))
	}
	goalWord := "<goal id>"
	target := "RECEIPT line"
	if params.Goal != "" {
		goalWord = params.Goal
		target = "RECEIPT line for goal " + params.Goal
	}
	decision.Outcome = ReceiptLineOutcomeRefused
	decision.Reason = "receipt-line-missing"
	decision.Command = fmt.Sprintf(`scripts/receipt.sh add --type implement --outcome shipped --goal %s --built-by coordinator --note "<what landed and how it was verified>"`, goalWord)
	detail := fmt.Sprintf("the landing changes code (%s) but its staged %s appends no %s", listed, location.fromInstallation, target)
	if len(otherGoals) > 0 {
		detail += fmt.Sprintf("; the RECEIPT lines it appends name goal %s", strings.Join(otherGoals, ", "))
	}
	decision.Detail = detail + fmt.Sprintf("; write the line with %s and include %s in the landing", decision.Command, location.fromInstallation)
	return decision, nil
}

func receiptLineExempt(decision ReceiptLineDecision, reason, detail string) ReceiptLineDecision {
	decision.Outcome = ReceiptLineOutcomeExempt
	decision.Reason = reason
	decision.Detail = detail
	return decision
}

// receiptLedgerLocation holds the receipt ledger twice: as the repository
// path the trees are read by and as the path from the installation root
// that a landing's caller stages. top, installation and appRoot are
// resolved absolute directories, so the relations hold where the
// temporary directory is itself a link; appRelative is the application
// root's path under the repository top ("" when they coincide).
type receiptLedgerLocation struct {
	top              string
	installation     string
	appRoot          string
	appRelative      string
	repositoryPath   string
	fromInstallation string
}

func locateReceiptLedger(workspace gittree.Workspace, root string) (receiptLedgerLocation, error) {
	appRoot, err := stateroot.RootForInstallation(root)
	if err != nil {
		return receiptLedgerLocation{}, err
	}
	top, err := workspace.TopLevel()
	if err != nil {
		return receiptLedgerLocation{}, err
	}
	relativeRoot, err := stateroot.RelativeRoot(stateroot.Receipts)
	if err != nil {
		return receiptLedgerLocation{}, err
	}
	location := receiptLedgerLocation{top: resolvedPath(top), installation: resolvedPath(root), appRoot: resolvedPath(appRoot)}
	appRelative, err := filepath.Rel(location.top, location.appRoot)
	if err != nil || appRelative == ".." || strings.HasPrefix(appRelative, ".."+string(filepath.Separator)) {
		return receiptLedgerLocation{}, fmt.Errorf("receipt ledger root %s is outside repository %s", location.appRoot, location.top)
	}
	if appRelative != "." {
		location.appRelative = filepath.ToSlash(appRelative)
	}
	location.repositoryPath = path.Join(location.appRelative, relativeRoot, "receipts.log")
	location.fromInstallation = location.pathFromInstallation(location.repositoryPath)
	return location, nil
}

// pathFromInstallation rewrites a repository path as the path a caller in
// the installation root names: unchanged at the repository top, stripped
// of the installation prefix beneath it, and reached through ".." above it.
func (location receiptLedgerLocation) pathFromInstallation(repositoryPath string) string {
	relative, err := filepath.Rel(location.installation, filepath.Join(location.top, filepath.FromSlash(repositoryPath)))
	if err != nil {
		return repositoryPath
	}
	return filepath.ToSlash(relative)
}

// appStateRecord reports whether a repository path is application state
// kept outside the installation: a vendored adopted installation keeps its
// registers, plans and records at the application root, where the path
// class manifest (an installation-relative document) has no row for them
// and ownership alone would call them outside. Paths inside the
// installation are left to the manifest, which knows its own READMEs.
func (location receiptLedgerLocation) appStateRecord(repositoryPath string) bool {
	installationRelative, err := filepath.Rel(location.installation, filepath.Join(location.top, filepath.FromSlash(repositoryPath)))
	if err == nil && installationRelative != ".." && !strings.HasPrefix(installationRelative, ".."+string(filepath.Separator)) {
		return false
	}
	appRelative := repositoryPath
	if location.appRelative != "" {
		trimmed, inside := strings.CutPrefix(repositoryPath, location.appRelative+"/")
		if !inside {
			return false
		}
		appRelative = trimmed
	}
	for _, kind := range []stateroot.Kind{stateroot.Registers, stateroot.Records, stateroot.OpenWork} {
		relativeRoot, err := stateroot.RelativeRoot(kind)
		if err != nil || relativeRoot == "" {
			continue
		}
		if strings.HasPrefix(appRelative, relativeRoot+"/") {
			return true
		}
	}
	return false
}

func resolvedPath(value string) string {
	absolute, err := filepath.Abs(value)
	if err != nil {
		return filepath.Clean(value)
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		return resolved
	}
	return absolute
}

// receiptLineCodePaths keeps the changed repository paths that are neither
// records nor ledgers: the paths whose change is work a receipt describes.
// The manifest is read from the landing base, as every other landing judge
// reads it; application state outside the installation is a record by its
// state root.
func receiptLineCodePaths(workspace gittree.Workspace, location receiptLedgerLocation, baseTree string, changed []string) ([]string, error) {
	prefix, err := workspace.Prefix()
	if err != nil {
		return nil, err
	}
	manifestPath := path.Join(prefix, pathclass.ManifestPath)
	manifestBytes, present, err := workspace.FileAt(baseTree, manifestPath)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, fmt.Errorf("path class manifest %s is not in the landing base", manifestPath)
	}
	manifest, err := pathclass.Parse(manifestBytes)
	if err != nil {
		return nil, err
	}
	var code []string
	for _, changedPath := range changed {
		if location.appStateRecord(changedPath) {
			continue
		}
		ownership, mode, err := stateroot.OwnerForInstallation(location.installation, changedPath)
		if err != nil {
			return nil, fmt.Errorf("classify landing path %s: %w", changedPath, err)
		}
		resolution := manifest.ResolveRepositoryPath(pathclass.Mode(mode), ownership, prefix, changedPath)
		switch resolution.Class {
		case pathclass.Record, pathclass.Ledger:
		default:
			code = append(code, changedPath)
		}
	}
	sort.Strings(code)
	return code, nil
}

func receiptLineListed(paths []string) string {
	if len(paths) <= receiptLineListedPaths {
		return strings.Join(paths, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(paths[:receiptLineListedPaths], ", "), len(paths)-receiptLineListedPaths)
}

// addedLines returns the candidate ledger's lines that the base ledger does
// not carry, in candidate order. The receipt verbs append; a landing that
// also rewrote history is still judged by the lines it adds.
func addedLines(base, candidate []byte) []string {
	known := map[string]struct{}{}
	for _, line := range strings.Split(string(base), "\n") {
		known[strings.TrimSpace(line)] = struct{}{}
	}
	var added []string
	for _, line := range strings.Split(string(candidate), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, present := known[line]; present {
			continue
		}
		added = append(added, line)
	}
	return added
}

func receiptLineGoal(fields []string) string {
	for _, field := range fields[3:] {
		if value, ok := strings.CutPrefix(field, "goal="); ok {
			return value
		}
	}
	return ""
}

func receiptLineGoalWord(goalID string) string {
	if goalID == "" {
		return "none"
	}
	return goalID
}
