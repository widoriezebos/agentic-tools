package landing

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/receipt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// A carried goal delivery lands a goal's composed candidate under a person's
// recorded exception (plans/intent-carried-delivery-design.md). The carry
// word records only the candidate's projected workspace, so the delivery
// retains the code endpoint it was composed on beside it: the patch that
// reaches the main checkout is always projected endpoint -> projected
// candidate, never the current main against an old candidate, which would
// quietly undo product work that landed since.

// CarriedSubject is the retained identity of one composed candidate.
type CarriedSubject struct {
	Goal              string `json:"goal"`
	Endpoint          string `json:"endpoint"`
	EndpointWorkspace string `json:"endpointWorkspace"`
	Workspace         string `json:"workspace"`
}

var carriedObjectID = regexp.MustCompile(`^[0-9a-f]{40}$`)

func (s CarriedSubject) valid() bool {
	return s.Goal != "" && carriedObjectID.MatchString(s.Endpoint) &&
		carriedObjectID.MatchString(s.EndpointWorkspace) && carriedObjectID.MatchString(s.Workspace)
}

// CarriedRefusal is a staging refusal with its cause; the checkout's bytes,
// index and refs are as they were before the step that refused.
type CarriedRefusal struct {
	Code   string
	Detail string
}

func (r *CarriedRefusal) Error() string { return r.Code + ": " + r.Detail }

func carriedRefuse(code, format string, args ...any) error {
	return &CarriedRefusal{Code: code, Detail: fmt.Sprintf(format, args...)}
}

// RetainCarriedSubject freezes a composed subject in dir, keyed by its
// endpoint and workspace. Retaining the same subject again is a no-op.
func RetainCarriedSubject(dir string, subject CarriedSubject) error {
	if !subject.valid() {
		return fmt.Errorf("carried subject is incomplete: %+v", subject)
	}
	return writeCarriedSubject(filepath.Join(dir, "subject-"+subject.Endpoint+"-"+subject.Workspace+".json"), subject)
}

// BindCarriedSubject binds a recorded exception to its retained subject. A
// binding is written once; a different subject for the same exception is a
// refusal, never a rebinding.
func BindCarriedSubject(dir, opid string, subject CarriedSubject) error {
	path := filepath.Join(dir, "exception-"+opid+".json")
	if existing, present, err := readCarriedSubject(path); err != nil {
		return err
	} else if present {
		if existing != subject {
			return carriedRefuse("carried-subject-ambiguous", "exception %s is bound to endpoint %s workspace %s, not endpoint %s workspace %s",
				opid, existing.Endpoint, existing.Workspace, subject.Endpoint, subject.Workspace)
		}
		return nil
	}
	return writeCarriedSubject(path, subject)
}

// BoundCarriedSubject reads the subject bound to a recorded exception.
func BoundCarriedSubject(dir, opid string) (CarriedSubject, bool, error) {
	return readCarriedSubject(filepath.Join(dir, "exception-"+opid+".json"))
}

// RetainedCarriedSubject is the one retained subject for a workspace: a
// recorded exception whose response was lost is adopted only when exactly
// one composition produced its workspace.
func RetainedCarriedSubject(dir, goalID, workspace string) (CarriedSubject, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "subject-*-"+workspace+".json"))
	if err != nil {
		return CarriedSubject{}, err
	}
	var found []CarriedSubject
	for _, path := range matches {
		subject, present, err := readCarriedSubject(path)
		if err != nil {
			return CarriedSubject{}, err
		}
		if present && subject.Goal == goalID && subject.Workspace == workspace {
			found = append(found, subject)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return CarriedSubject{}, carriedRefuse("carried-subject-missing", "no retained composition of goal %s has workspace %s", goalID, workspace)
	default:
		endpoints := []string{}
		for _, subject := range found {
			endpoints = append(endpoints, subject.Endpoint)
		}
		sort.Strings(endpoints)
		return CarriedSubject{}, carriedRefuse("carried-subject-ambiguous", "workspace %s of goal %s was composed on endpoints %s", workspace, goalID, strings.Join(endpoints, ", "))
	}
}

func writeCarriedSubject(path string, subject CarriedSubject) error {
	if existing, present, err := readCarriedSubject(path); err != nil {
		return err
	} else if present {
		if existing != subject {
			return carriedRefuse("carried-subject-ambiguous", "%s already holds a different subject", path)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(subject, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(path, string(data)+"\n", filepath.Dir(path))
	return err
}

func readCarriedSubject(path string) (CarriedSubject, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return CarriedSubject{}, false, nil
	}
	if err != nil {
		return CarriedSubject{}, false, err
	}
	var subject CarriedSubject
	if err := json.Unmarshal(data, &subject); err != nil || !subject.valid() {
		return CarriedSubject{}, false, fmt.Errorf("%s is not a carried subject: %v", path, err)
	}
	return subject, true, nil
}

// CarriedAdvance brings a main checkout that is merely behind the fetched
// code origin up to it through the existing Advance owner, which takes the
// checkout mutation lock itself; the caller must not hold it. The fetched
// origin's product must still be the subject's endpoint product: movement
// after composition needs a person's explicit replacement, and local
// commits the origin lacks are never rebased here.
func CarriedAdvance(root, upstream string, subject CarriedSubject) error {
	workspace := gittree.Workspace{Dir: root}
	if err := carriedOnMain(workspace); err != nil {
		return err
	}
	fetched, err := workspace.ResolveCommit(upstream)
	if err != nil {
		return err
	}
	if err := carriedOriginProduct(root, workspace, fetched, subject); err != nil {
		return err
	}
	head, _, err := workspace.HeadCommit()
	if err != nil {
		return err
	}
	if head == fetched {
		return nil
	}
	headTree, err := workspace.ResolveRef(head + "^{tree}")
	if err != nil {
		return err
	}
	if product, err := ProjectWorkspaceTree(root, headTree); err != nil {
		return err
	} else if product == subject.EndpointWorkspace {
		// Only shared coordination state moved: the carried transaction
		// carries the staged candidate forward over it itself.
		return nil
	}
	behind, err := workspace.IsAncestor(head, fetched)
	if err != nil {
		return err
	}
	if !behind {
		return carriedRefuse("carried-local-diverged", "local main %s has commits origin main %s does not; publish or remove them by hand", head, fetched)
	}
	var stdout, stderr bytes.Buffer
	if err := Advance(root, upstream, &stdout, &stderr); err != nil {
		return fmt.Errorf("main is behind origin and could not be advanced: %w", errors.Join(err, errors.New(strings.TrimSpace(stderr.String()))))
	}
	return nil
}

func carriedOnMain(workspace gittree.Workspace) error {
	branch, detached, err := workspace.SymbolicHead()
	if err != nil {
		return err
	}
	if detached || branch != "refs/heads/main" {
		return carriedRefuse("carried-not-on-main", "the main checkout is on %q, not main", branch)
	}
	return nil
}

func carriedOriginProduct(root string, workspace gittree.Workspace, fetched string, subject CarriedSubject) error {
	tree, err := workspace.ResolveRef(fetched + "^{tree}")
	if err != nil {
		return err
	}
	product, err := ProjectWorkspaceTree(root, tree)
	if err != nil {
		return err
	}
	if product != subject.EndpointWorkspace {
		return carriedRefuse("carried-origin-moved", "origin main %s has product %s, not the endpoint %s the exception's candidate was composed on",
			fetched, product, subject.Endpoint)
	}
	return nil
}

// CarriedStage is the staging request: the main installation, the fetched
// code origin ref, the exception's subject and its identity for the receipt.
type CarriedStage struct {
	Root      string
	Upstream  string
	Subject   CarriedSubject
	Exception string
	Opid      string
	Now       func() time.Time
}

// CarriedStaged reports what staging did: Applied when this run applied the
// candidate (false when the exact candidate was already staged) and Receipt
// as the receipt-line step's resolution.
type CarriedStaged struct {
	Applied bool
	Receipt string
	Tree    string
}

// StageCarriedCandidate stages the subject's product change and its RECEIPT
// line into the main checkout under the checkout mutation lock. It applies
// the projected-endpoint -> projected-candidate patch with a plain
// git apply --index --binary at the repository top, after a private-index
// preflight, and never resets, cleans or three-way merges: a refusal leaves
// bytes, index and refs as they were. Repeating it for an already staged
// candidate is a no-op.
func StageCarriedCandidate(request CarriedStage) (CarriedStaged, error) {
	return stageCarriedCandidate(request, carriedApplyIndex)
}

func stageCarriedCandidate(request CarriedStage, apply func(top string, patch []byte) error) (CarriedStaged, error) {
	root, subject := request.Root, request.Subject
	if !subject.valid() {
		return CarriedStaged{}, fmt.Errorf("carried subject is incomplete: %+v", subject)
	}
	release, err := lease.LockBounded(lease.LockPath(root), "carried candidate staging")
	if err != nil {
		return CarriedStaged{}, carriedRefuse("carried-checkout-locked", "%v", err)
	}
	defer release()
	workspace := gittree.Workspace{Dir: root}
	if err := carriedOnMain(workspace); err != nil {
		return CarriedStaged{}, err
	}
	top, err := workspace.TopLevel()
	if err != nil {
		return CarriedStaged{}, err
	}
	topWorkspace := gittree.Workspace{Dir: top}
	head, unborn, err := workspace.HeadCommit()
	if err != nil {
		return CarriedStaged{}, err
	}
	if unborn {
		return CarriedStaged{}, carriedRefuse("carried-not-on-main", "main is unborn")
	}
	fetched, err := workspace.ResolveCommit(request.Upstream)
	if err != nil {
		return CarriedStaged{}, err
	}
	if err := carriedOriginProduct(root, workspace, fetched, subject); err != nil {
		return CarriedStaged{}, err
	}
	if contained, err := workspace.IsAncestor(head, fetched); err != nil {
		return CarriedStaged{}, err
	} else if !contained {
		return CarriedStaged{}, carriedRefuse("carried-local-diverged", "local main %s has commits origin main %s does not", head, fetched)
	}
	headTree, err := workspace.ResolveRef(head + "^{tree}")
	if err != nil {
		return CarriedStaged{}, err
	}
	if product, err := ProjectWorkspaceTree(root, headTree); err != nil {
		return CarriedStaged{}, err
	} else if product != subject.EndpointWorkspace {
		return CarriedStaged{}, carriedRefuse("carried-main-behind", "main %s has product %s, not the candidate's endpoint product; advance main to origin first", head, product)
	}
	patch, err := topWorkspace.Diff(subject.EndpointWorkspace, subject.Workspace)
	if err != nil {
		return CarriedStaged{}, err
	}
	expected, err := topWorkspace.Apply(headTree, patch)
	if err != nil {
		return CarriedStaged{}, carriedRefuse("carried-patch-conflict", "the candidate does not apply to main %s: %v", head, err)
	}
	if product, err := ProjectWorkspaceTree(root, expected); err != nil {
		return CarriedStaged{}, err
	} else if product != subject.Workspace {
		return CarriedStaged{}, carriedRefuse("carried-patch-conflict", "main %s with the candidate applied has product %s, not the exception's workspace %s", head, product, subject.Workspace)
	}
	ledger, err := deliveryReceiptLedger(root)
	if err != nil {
		return CarriedStaged{}, err
	}
	posture, err := workspace.TopStagedPosture()
	if err != nil {
		return CarriedStaged{}, err
	}
	if len(posture.Unmerged) != 0 {
		return CarriedStaged{}, carriedRefuse("carried-index-unmerged", "the index has unmerged entries: %s", strings.Join(posture.Unmerged, "; "))
	}
	staged := CarriedStaged{}
	switch {
	case posture.Tree == headTree:
		if err := carriedDriftClean(root, true); err != nil {
			return CarriedStaged{}, err
		}
		if err := apply(top, patch); err != nil {
			return CarriedStaged{}, carriedRefuse("carried-patch-conflict", "%v", err)
		}
		staged.Applied = true
	default:
		matches, err := carriedIndexIsCandidate(topWorkspace, expected, posture.Tree, ledger)
		if err != nil {
			return CarriedStaged{}, err
		}
		if !matches {
			return CarriedStaged{}, carriedRefuse("carried-index-not-candidate", "the index %s is neither main %s nor the exception's candidate %s", posture.Tree, headTree, expected)
		}
	}
	if err := carriedDriftClean(root, false); err != nil {
		return CarriedStaged{}, err
	}
	if after, err := workspace.TopStagedPosture(); err != nil {
		return CarriedStaged{}, err
	} else if matches, err := carriedIndexIsCandidate(topWorkspace, expected, after.Tree, ledger); err != nil || !matches || len(after.Unmerged) != 0 {
		return CarriedStaged{}, errors.Join(err, carriedRefuse("carried-index-not-candidate", "the staged index %s is not the candidate %s", after.Tree, expected))
	}
	resolution, tree, err := carriedReceiptLine(request, workspace, top, ledger)
	if err != nil {
		return CarriedStaged{}, err
	}
	staged.Receipt, staged.Tree = resolution, tree
	return staged, nil
}

func carriedApplyIndex(top string, patch []byte) error {
	command := exec.Command("git", "-C", top, "apply", "--index", "--binary", "-")
	command.Env = gittree.ScrubbedEnviron("LC_ALL=C")
	command.Stdin = bytes.NewReader(patch)
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Run(); err != nil {
		return fmt.Errorf("git apply --index --binary: %w: %s", err, strings.TrimSpace(output.String()))
	}
	return nil
}

// carriedDriftClean refuses unrelated staged, unstaged and untracked paths,
// keeping append-shaped register drift the landing rules already tolerate.
func carriedDriftClean(root string, requireEmptyIndex bool) error {
	drift, _, err := WorktreeDrift(root, requireEmptyIndex)
	if err != nil {
		return err
	}
	lines := []string{}
	for _, entry := range drift {
		lines = append(lines, entry.Kind+" "+entry.Path)
	}
	// The delivery commits the whole Git top level, so an untracked path
	// beside the installation is the checkout's product too: WorktreeDrift
	// leaves it to its caller, and the carried commit would refuse it after
	// the candidate was staged.
	workspace := gittree.Workspace{Dir: root}
	prefix, err := workspace.Prefix()
	if err != nil {
		return err
	}
	if prefix != "" {
		status, err := workspace.Status()
		if err != nil {
			return err
		}
		for _, entry := range status {
			if entry.Index == '?' && entry.Worktree == '?' && !strings.HasPrefix(entry.Path, prefix) {
				lines = append(lines, "untracked "+entry.Path)
			}
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return carriedRefuse("carried-checkout-dirty", "the main checkout has changes the candidate does not own: %s", strings.Join(lines, ", "))
}

// carriedIndexIsCandidate accepts the staged candidate itself or the
// candidate with an append to the receipt ledger staged beside it.
func carriedIndexIsCandidate(top gittree.Workspace, expected, index string, ledger carriedLedger) (bool, error) {
	if index == expected {
		return true, nil
	}
	changed, err := top.ChangedPaths(expected, index)
	if err != nil {
		return false, err
	}
	if len(changed) != 1 || changed[0] != ledger.repositoryPath {
		return false, nil
	}
	before, _, err := top.FileAt(expected, ledger.repositoryPath)
	if err != nil {
		return false, err
	}
	after, present, err := top.FileAt(index, ledger.repositoryPath)
	if err != nil || !present {
		return false, err
	}
	return carriedAppendShaped(before, after), nil
}

func carriedAppendShaped(before, after []byte) bool {
	if len(after) <= len(before) || after[len(after)-1] != '\n' || !bytes.Equal(after[:len(before)], before) {
		return false
	}
	return len(before) == 0 || before[len(before)-1] == '\n'
}

type carriedLedger struct {
	file           string
	repositoryPath string
}

// deliveryReceiptLedger resolves the tracked receipt ledger the receipt-line
// owner judges, as an absolute file and a repository path.
func deliveryReceiptLedger(root string) (carriedLedger, error) {
	location, err := locateReceiptLedger(gittree.Workspace{Dir: root}, root, stateroot.RootForInstallation)
	if err != nil {
		return carriedLedger{}, err
	}
	return carriedLedger{file: filepath.Join(location.top, filepath.FromSlash(location.repositoryPath)), repositoryPath: location.repositoryPath}, nil
}

// carriedReceiptLine completes the receipt-line obligation under the
// staging lock: the owner's pass or exemption stands; a missing line is
// first looked for in an append already in the ledger (a crash after the
// append must not write it twice), and only then written by receipt.Add
// into the exact tracked ledger the owner named.
func carriedReceiptLine(request CarriedStage, workspace gittree.Workspace, top string, ledger carriedLedger) (string, string, error) {
	observe := func() (ReceiptLineDecision, string, error) {
		posture, err := workspace.TopStagedPosture()
		if err != nil {
			return ReceiptLineDecision{}, "", err
		}
		decision, err := ObserveReceiptLine(ReceiptLineParams{RepoRoot: request.Root, CandidateTree: posture.Tree, Goal: request.Subject.Goal})
		return decision, posture.Tree, err
	}
	decision, tree, err := observe()
	if err != nil {
		return "", "", err
	}
	if decision.Outcome != ReceiptLineOutcomeRefused {
		return decision.Outcome + ":" + decision.Reason, tree, nil
	}
	if decision.Reason != "receipt-line-missing" {
		return "", "", carriedRefuse("carried-receipt-refused", "%s", decision.Detail)
	}
	if filepath.Join(request.Root, filepath.FromSlash(decision.Ledger)) != ledger.file && !sameCarriedFile(filepath.Join(request.Root, filepath.FromSlash(decision.Ledger)), ledger.file) {
		return "", "", fmt.Errorf("the receipt-line owner names ledger %s, not %s", decision.Ledger, ledger.file)
	}
	appended, err := carriedLedgerAppended(workspace, top, ledger)
	if err != nil {
		return "", "", err
	}
	if appended {
		if err := carriedStageLedger(top, ledger); err != nil {
			return "", "", err
		}
		if decision, tree, err = observe(); err != nil {
			return "", "", err
		}
		if decision.Outcome == ReceiptLineOutcomePass {
			return "pass:staged-existing", tree, nil
		}
	}
	options := receipt.Options{Root: request.Root, File: ledger.file, Type: "implement", Outcome: "reworked",
		Skills: "none", Verify: "skipped", Corrections: "0", StopLoss: "no", Goal: request.Subject.Goal,
		Note: fmt.Sprintf("prepared for carried delivery under exception %s code %s; not yet landed", request.Opid, request.Exception),
		Now:  request.Now}
	if added := receipt.Add(options); added.Code != 0 {
		return "", "", fmt.Errorf("receipt add refused: %s", strings.Join(append(added.Out, added.Err...), "; "))
	}
	if err := carriedStageLedger(top, ledger); err != nil {
		return "", "", err
	}
	decision, tree, err = observe()
	if err != nil {
		return "", "", err
	}
	if decision.Outcome != ReceiptLineOutcomePass {
		return "", "", carriedRefuse("carried-receipt-refused", "%s", decision.Detail)
	}
	return "pass:appended", tree, nil
}

func sameCarriedFile(left, right string) bool {
	a, errA := filepath.EvalSymlinks(filepath.Dir(left))
	b, errB := filepath.EvalSymlinks(filepath.Dir(right))
	return errA == nil && errB == nil && a == b && filepath.Base(left) == filepath.Base(right)
}

func carriedLedgerAppended(workspace gittree.Workspace, top string, ledger carriedLedger) (bool, error) {
	posture, err := workspace.TopStagedPosture()
	if err != nil {
		return false, err
	}
	staged, _, err := (gittree.Workspace{Dir: top}).FileAt(posture.Tree, ledger.repositoryPath)
	if err != nil {
		return false, err
	}
	current, err := os.ReadFile(ledger.file)
	if err != nil {
		return false, err
	}
	return carriedAppendShaped(staged, current), nil
}

func carriedStageLedger(top string, ledger carriedLedger) error {
	_, err := landingGit(top, "add", "--", ledger.repositoryPath)
	return err
}
