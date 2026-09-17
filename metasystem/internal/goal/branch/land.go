package branch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

const (
	LandPartialCode       = "GOAL_LAND_PARTIAL"
	UnitRereadCode        = "GOAL_UNIT_REREAD"
	LandUnprovenCode      = "GOAL_LAND_UNPROVEN"
	LandRetryCode         = "GOAL_LAND_RETRY"
	LandUncheckedCode     = "GOAL_LAND_UNCHECKED"
	LandBranchMovedCode   = "GOAL_LAND_BRANCH_MOVED"
	LandAuthorUnboundCode = "GOAL_LAND_AUTHOR_UNBOUND"
)

type LandHooks struct {
	AfterLandingRead func() error
}

type LandRequest struct {
	Repo, Remote, EndpointTip, BranchTip, GoalID string
	Out, TestReceipt, Through                    string
	Last, LandingReady                           bool
	GoalPage, ApprovedBy, Seat                   string
	CheckClaim                                   func() error
	PushTransport                                PushTransport
	Hooks                                        LandHooks
}

type LandResult struct {
	Endpoint, Candidate, Landing, Branch, Attempt, LastUnit string
	RetryIdentity                                           string
	ProofNumber                                             int
	FailingGroups                                           []string
}

type LandingProof struct {
	Number                       int
	Endpoint, Candidate, Landing string
	Attempt, Verdict, CanaryRun  string
	CanaryTip, Fix               string
	RetryIdentity                string
	Groups                       []string
}

func landingBranchRef(goalID string) string { return "refs/heads/landing/" + goalID }

func landingRecordPath(goalID string) string {
	return "metasystem/records/misc/" + goalID + "-landing.md"
}

func RenderLandingProof(proof LandingProof) string {
	groups := "-"
	if len(proof.Groups) != 0 {
		groups = strings.Join(proof.Groups, ",")
	}
	canary := "-"
	if proof.CanaryRun != "" {
		canary = proof.CanaryRun + "@" + proof.CanaryTip
	}
	fix := proof.Fix
	if fix == "" {
		fix = "-"
	}
	retry := proof.RetryIdentity
	if retry == "" {
		retry = "-"
	}
	return fmt.Sprintf("- Proof: n=%d endpoint=%s candidate=%s landing=%s retry=%s attempt=%s verdict=%s groups=%s canary=%s fix=%s",
		proof.Number, proof.Endpoint, proof.Candidate, proof.Landing, retry, proof.Attempt, proof.Verdict, groups, canary, fix)
}

func ParseLandingRecord(data []byte) ([]LandingProof, error) {
	var proofs []LandingProof
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- Proof:") {
			continue
		}
		fields := map[string]string{}
		for _, field := range strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "- Proof:"))) {
			key, value, ok := strings.Cut(field, "=")
			if !ok || key == "" || value == "" || fields[key] != "" {
				return nil, fmt.Errorf("landing record has a malformed proof line: %s", line)
			}
			fields[key] = value
		}
		n, err := strconv.Atoi(fields["n"])
		if err != nil || n < 1 || !hex40(fields["endpoint"]) || !hex40(fields["candidate"]) ||
			!hex40(fields["landing"]) || fields["attempt"] == "" || fields["verdict"] == "" {
			return nil, fmt.Errorf("landing record has an incomplete proof line: %s", line)
		}
		proof := LandingProof{Number: n, Endpoint: fields["endpoint"], Candidate: fields["candidate"],
			Landing: fields["landing"], Attempt: fields["attempt"], Verdict: fields["verdict"]}
		if fields["retry"] != "" && fields["retry"] != "-" {
			if !hex40(fields["retry"]) {
				return nil, fmt.Errorf("landing record has a malformed retry identity: %s", line)
			}
			proof.RetryIdentity = fields["retry"]
		}
		if groups := fields["groups"]; groups != "" && groups != "-" {
			proof.Groups = strings.Split(groups, ",")
		}
		if canary := fields["canary"]; canary != "" && canary != "-" {
			proof.CanaryRun, proof.CanaryTip, _ = strings.Cut(canary, "@")
			if proof.CanaryRun == "" || !hex40(proof.CanaryTip) {
				return nil, fmt.Errorf("landing record has a malformed canary: %s", line)
			}
		}
		if fields["fix"] != "" && fields["fix"] != "-" {
			proof.Fix = fields["fix"]
			if !hex40(proof.Fix) {
				return nil, fmt.Errorf("landing record has a malformed fix commit: %s", line)
			}
		}
		proofs = append(proofs, proof)
	}
	return proofs, nil
}

func readLandingRecord(repo, tip, goalID string) ([]LandingProof, error) {
	path := landingRecordPath(goalID)
	if _, err := gitOutput(repo, "cat-file", "-e", tip+":"+path); err != nil {
		return nil, nil
	}
	data, err := gitOutput(repo, "show", tip+":"+path)
	if err != nil {
		return nil, err
	}
	return ParseLandingRecord(data)
}

func goalPageNamesCommit(page, commit string) bool {
	want := "land through " + commit
	for _, line := range strings.Split(strings.ReplaceAll(page, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "- Next step: "+want || strings.HasPrefix(line, "- ") && strings.HasSuffix(line, " reason="+want) {
			return true
		}
	}
	return false
}

type landUnit struct {
	status UnitStatus
	folds  []Commit
}

func landingUnits(status Status, count int) []landUnit {
	groups := make([]landUnit, count)
	byCommit := map[string]int{}
	byUnit := map[string]int{}
	for i := range groups {
		groups[i].status = status.Units[i]
		byCommit[status.Units[i].Commit] = i
		byUnit[status.Units[i].Unit] = i
	}
	next := 0
	for _, commit := range status.Commits {
		if commit.Kind == Unit {
			if index, ok := byCommit[commit.ID]; ok && index == next {
				next++
			}
			continue
		}
		target := -1
		if commit.Kind == Read {
			target = byUnit[commit.Unit]
			if _, ok := byUnit[commit.Unit]; !ok {
				target = -1
			}
		} else if next < count {
			target = next
		} else if count > 0 {
			target = count - 1
		}
		if target >= 0 {
			groups[target].folds = append(groups[target].folds, commit)
		}
	}
	return groups
}

type landingIdentity struct{ Name, Email string }

func resolveLandingIdentity(repo, approvedBy string) (landingIdentity, error) {
	name := strings.TrimPrefix(approvedBy, "human:")
	if name == approvedBy || !validName(name) {
		return landingIdentity{}, operationRefusal(LandAuthorUnboundCode, "goal approver %q does not name a human", approvedBy)
	}
	out, err := gitOutput(repo, "config", "--get", "goal.human."+name)
	if err != nil {
		return landingIdentity{}, operationRefusal(LandAuthorUnboundCode, "goal approver %s has no goal.human.%s identity", approvedBy, name)
	}
	address, err := mail.ParseAddress(strings.TrimSpace(string(out)))
	if err != nil || address.Name == "" || address.Address == "" || strings.ContainsAny(address.Name+address.Address, "\r\n") {
		return landingIdentity{}, operationRefusal(LandAuthorUnboundCode, "goal.human.%s must be Name <email>", name)
	}
	return landingIdentity{Name: address.Name, Email: address.Address}, nil
}

func treeEntry(repo, tree, path string) (mode, blob string, present bool, err error) {
	out, err := gitOutput(repo, "ls-tree", "-z", tree, "--", path)
	if err != nil {
		return "", "", false, err
	}
	if len(out) == 0 {
		return "", "", false, nil
	}
	meta, _, ok := strings.Cut(strings.TrimSuffix(string(out), "\x00"), "\t")
	fields := strings.Fields(meta)
	if !ok || len(fields) != 3 || fields[1] != "blob" {
		return "", "", false, fmt.Errorf("tree entry for %s is malformed", path)
	}
	return fields[0], fields[2], true, nil
}

func verifyUnitPreimages(repo, tree string, unit UnitStatus) error {
	entries, err := RawEntries(repo, unit.Commit)
	if err != nil {
		return err
	}
	var changed []string
	for _, entry := range entries {
		if entry.Path == "metasystem/testing.json" {
			// Its three blob versions are validated by the contract merge driver.
			continue
		}
		mode, blob, present, err := treeEntry(repo, tree, entry.Path)
		if err != nil {
			return err
		}
		absentSource := strings.Trim(entry.SrcBlob, "0") == ""
		if absentSource && present || !absentSource && (!present || blob != entry.SrcBlob || mode != entry.SrcMode) {
			changed = append(changed, entry.Path)
		}
	}
	if len(changed) != 0 {
		return operationRefusal(UnitRereadCode, "unit %s no longer applies to the endpoint at paths %s", unit.Unit, strings.Join(changed, ", "))
	}
	return nil
}

func transitionDigest(repo, before, after string) (string, error) {
	raw, err := transitionRaw(repo, before, after)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func transitionRaw(repo, before, after string, pathspec ...string) ([]byte, error) {
	args := []string{"diff-tree", "-r", "-z", "--no-renames", "--full-index", before, after}
	return gitOutput(repo, append(args, pathspec...)...)
}

func transitionChangesTestingContract(repo, before, after string) (bool, error) {
	raw, err := transitionRaw(repo, before, after, "--", "metasystem/testing.json")
	return len(raw) != 0, err
}

func unitTransitionMatches(repo, before, after string, unit UnitStatus) (string, bool, error) {
	got, err := transitionDigest(repo, before, after)
	if err != nil || got == unit.Digest {
		return got, got == unit.Digest, err
	}
	originalDigest, err := transitionDigest(repo, unit.Commit+"^", unit.Commit)
	if err != nil || originalDigest != unit.Digest {
		return got, false, err
	}
	originalContract, err := transitionChangesTestingContract(repo, unit.Commit+"^", unit.Commit)
	if err != nil {
		return got, false, err
	}
	appliedContract, err := transitionChangesTestingContract(repo, before, after)
	if err != nil || !originalContract || !appliedContract {
		return got, false, err
	}
	withoutContract := []string{"--", ".", ":(exclude)metasystem/testing.json"}
	original, err := transitionRaw(repo, unit.Commit+"^", unit.Commit, withoutContract...)
	if err != nil {
		return got, false, err
	}
	applied, err := transitionRaw(repo, before, after, withoutContract...)
	return got, bytes.Equal(original, applied), err
}

func applyCommit(worktree, commit string) error {
	patch, err := gitOutput(worktree, "diff", "--binary", "--full-index", commit+"^", commit)
	if err != nil {
		return err
	}
	_, err = gitInput(worktree, patch, "apply", "--index", "--3way", "-")
	return err
}

func commitCoAuthors(repo, commit string) ([]string, error) {
	out, err := gitOutput(repo, "show", "-s", "--format=%B", commit)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Co-Authored-By: ") && strings.TrimSpace(strings.TrimPrefix(line, "Co-Authored-By: ")) != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

func landingMessage(repo string, group landUnit, goalID, seat string, last bool) (string, []string, error) {
	lines := []string{"Goal-Unit: " + goalID + "/" + group.status.Unit, "Goal-Digest: " + group.status.Digest,
		"Goal-Source: " + group.status.Commit}
	foldPaths := map[string]bool{}
	for _, fold := range group.folds {
		lines = append(lines, "Goal-Source: "+fold.ID)
		entries, err := RawEntries(repo, fold.ID)
		if err != nil {
			return "", nil, err
		}
		for _, entry := range entries {
			foldPaths[entry.Path] = true
		}
	}
	paths := make([]string, 0, len(foldPaths))
	for path := range foldPaths {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		lines = append(lines, "Goal-Fold: "+path)
	}
	if last {
		lines = append(lines, "Goal-Last: "+goalID)
	}
	lines = append(lines, "Landed-By: "+seat)
	coauthors, err := commitCoAuthors(repo, group.status.Commit)
	if err != nil {
		return "", nil, err
	}
	lines = append(lines, coauthors...)
	return "land goal " + goalID + " unit " + group.status.Unit + "\n\n" + strings.Join(lines, "\n") + "\n", paths, nil
}

func lastUnitName(status UnitStatus) string {
	if len(status.Units) != 0 {
		return status.Units[len(status.Units)-1]
	}
	return status.Unit
}

type landingReceiptEvidence struct {
	attempt, stamp string
	failingGroups  []string
}

func readLandingReceipt(repo, path, candidate string) (landingReceiptEvidence, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return landingReceiptEvidence{}, operationRefusal(LandUnprovenCode, "read landing receipt: %v", err)
	}
	var receipt landing.TestReceipt
	if err := json.Unmarshal(data, &receipt); err != nil || receipt.SchemaVersion != 3 {
		return landingReceiptEvidence{}, operationRefusal(LandUnprovenCode, "landing receipt is not a schema-3 receipt")
	}
	matched := receipt.Tree == candidate
	if !matched && hex40(receipt.Tree) {
		projected, projectErr := projectLandingWorkspace(repo, receipt.Tree)
		matched = projectErr == nil && projected == candidate
	}
	if !matched {
		return landingReceiptEvidence{}, operationRefusal(LandUnprovenCode, "receipt proves %s, not candidate workspace %s", receipt.Tree, candidate)
	}
	evidence := landingReceiptEvidence{}
	if receipt.Proof != nil {
		evidence.attempt = receipt.Proof.AttemptID
	}
	if evidence.attempt == "" && receipt.Testing != nil {
		evidence.attempt = receipt.Testing.AttemptID
	}
	if evidence.attempt == "" && len(receipt.AttemptIDs) == 1 {
		evidence.attempt = receipt.AttemptIDs[0]
	}
	if evidence.attempt == "" {
		return landingReceiptEvidence{}, operationRefusal(LandUnprovenCode, "landing receipt names no single proof attempt")
	}
	evidence.stamp = receipt.Time
	if _, err := time.Parse(time.RFC3339Nano, evidence.stamp); err != nil {
		return landingReceiptEvidence{}, operationRefusal(LandUnprovenCode, "landing receipt has no valid proof time")
	}
	if receipt.ExitStatus != 0 {
		if receipt.Testing == nil || len(receipt.Testing.Delivery.FailingGroups) == 0 {
			return landingReceiptEvidence{}, operationRefusal(LandUnprovenCode, "red landing receipt names no failing groups")
		}
		evidence.failingGroups = append([]string(nil), receipt.Testing.Delivery.FailingGroups...)
	}
	return evidence, nil
}

func appendReceiptRow(worktree, goalID, lastUnit, attempt, stamp string) error {
	path := filepath.Join(worktree, "metasystem", "memory", "receipts.log")
	parsed, _ := time.Parse(time.RFC3339Nano, stamp)
	rowTime := parsed.UTC()
	row := fmt.Sprintf("%d|%s|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|delegate=none|goal=%s|built_by=coordinator|last_unit=%s|proof=%s|critique_waived=none|waiver_stream=none|note=landing proof %s\n",
		rowTime.Unix(), rowTime.Format("2006-01-02T15:04:05Z"), goalID, lastUnit, attempt, attempt)
	handle, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, writeErr := handle.WriteString(row)
	closeErr := handle.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func redProofChecked(repo, branchTip, goalID string, proof LandingProof) bool {
	if proof.CanaryRun == "" || !hex40(proof.CanaryTip) || proof.Fix == "" {
		return false
	}
	fix, err := KindOf(repo, proof.Fix, goalID)
	if err != nil || fix.Kind != Unit {
		return false
	}
	if _, err := gitOutput(repo, "merge-base", "--is-ancestor", proof.Fix, proof.CanaryTip); err != nil {
		return false
	}
	if proof.CanaryTip == branchTip {
		return true
	}
	out, err := gitOutput(repo, "rev-list", "--first-parent", "--reverse", proof.CanaryTip+".."+branchTip)
	if err != nil {
		return false
	}
	for _, commit := range strings.Fields(string(out)) {
		kind, err := KindOf(repo, commit, goalID)
		if err != nil || kind.Kind == Unit {
			return false
		}
	}
	return true
}

func landingRetryIdentity(repo, tree, goalID string) (string, error) {
	return (gittree.Workspace{Dir: repo}).FilterTree(tree, []string{
		landingRecordPath(goalID), "metasystem/memory/receipts.log",
	})
}

func projectLandingWorkspace(repo, tree string) (string, error) {
	return landing.ProjectWorkspaceTree(filepath.Join(repo, "metasystem"), tree)
}

func writeLandArtifacts(out string, result LandResult, proof LandingProof, messages, diffs map[string][]byte) error {
	parent := filepath.Dir(out)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".land-prep-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	trunk := fmt.Sprintf("endpoint=%s\ncandidate=%s\nlanding=%s\nbranch=%s\n", result.Endpoint, result.Candidate, result.Landing, result.Branch)
	if err := os.WriteFile(filepath.Join(stage, "trunk"), []byte(trunk), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, "record-draft"), []byte(RenderLandingProof(proof)+"\n"), 0o644); err != nil {
		return err
	}
	for unit, message := range messages {
		if err := os.WriteFile(filepath.Join(stage, unit+".message"), message, 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(stage, unit+".patch"), diffs[unit], 0o644); err != nil {
			return err
		}
	}
	if _, err := os.Stat(out); err == nil {
		return fmt.Errorf("landing output %s already exists", out)
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(stage, out)
}

func requireLandOutputAbsent(out string) error {
	if _, err := os.Lstat(out); err == nil {
		return fmt.Errorf("landing output %s already exists", out)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func PrepareLanding(req LandRequest) (LandResult, error) {
	if req.PushTransport == nil {
		req.PushTransport = GitPushTransport{}
	}
	if !validName(req.GoalID) || req.Repo == "" || req.Remote == "" || req.Out == "" || req.TestReceipt == "" ||
		!hex40(req.EndpointTip) || !hex40(req.BranchTip) || req.Last == (req.Through != "") {
		return LandResult{}, fmt.Errorf("land-prep needs a goal, endpoint, branch tip, output, receipt, and exactly one of --last or --through")
	}
	if err := requireLandOutputAbsent(req.Out); err != nil {
		return LandResult{}, err
	}
	if err := checkClaim(req.CheckClaim); err != nil {
		return LandResult{}, err
	}
	identity, err := resolveLandingIdentity(req.Repo, req.ApprovedBy)
	if err != nil {
		return LandResult{}, err
	}
	if strings.TrimSpace(req.Seat) == "" || strings.ContainsAny(req.Seat, "\r\n") {
		return LandResult{}, fmt.Errorf("land-prep needs the landing seat")
	}
	status, err := InspectStatus(req.Repo, req.EndpointTip, req.BranchTip, req.GoalID)
	if err != nil {
		return LandResult{}, err
	}
	count := status.Prefix
	if count == 0 {
		return LandResult{}, operationRefusal(LandUnprovenCode, "goal %s has no land-ready unit", req.GoalID)
	}
	if req.Last && !req.LandingReady {
		return LandResult{}, operationRefusal(LandPartialCode, "--last requires goal land-ready on %s", req.GoalID)
	}
	if req.Last && status.Prefix != len(status.Units) {
		return LandResult{}, operationRefusal(LandPartialCode, "--last found a unit beyond the land-ready prefix on %s", req.GoalID)
	}
	if req.Through != "" {
		count = 0
		for i := 0; i < status.Prefix; i++ {
			if status.Units[i].Commit == req.Through {
				count = i + 1
				break
			}
		}
		if count == 0 {
			return LandResult{}, operationRefusal(LandPartialCode, "--through %s is outside the land-ready prefix", req.Through)
		}
		if !goalPageNamesCommit(req.GoalPage, req.Through) {
			return LandResult{}, operationRefusal(LandPartialCode, "goal page has no human word naming partial tip %s", req.Through)
		}
	}
	groups := landingUnits(status, count)
	remoteRef := landingBranchRef(req.GoalID)
	expected, expectedPresent, err := req.PushTransport.RemoteTip(req.Repo, req.Remote, remoteRef)
	if err != nil {
		return LandResult{}, err
	}
	if !expectedPresent {
		expected = ""
	}
	scratch, err := os.MkdirTemp("", "goal-land-prep-*")
	if err != nil {
		return LandResult{}, err
	}
	defer os.RemoveAll(scratch)
	worktree := filepath.Join(scratch, "worktree")
	if _, err := gitOutput(req.Repo, "worktree", "add", "--quiet", "--detach", worktree, req.EndpointTip); err != nil {
		return LandResult{}, err
	}
	defer func() { _, _ = gitOutput(req.Repo, "worktree", "remove", "--force", worktree) }()
	lastUnit := lastUnitName(groups[len(groups)-1].status)
	compose := func(receiptID, receiptStamp string) (map[string][]byte, map[string][]byte, error) {
		if _, err := gitOutput(worktree, "reset", "--hard", req.EndpointTip); err != nil {
			return nil, nil, err
		}
		messages, diffs := map[string][]byte{}, map[string][]byte{}
		for index, group := range groups {
			parentOut, err := gitOutput(worktree, "rev-parse", "HEAD^{commit}")
			if err != nil {
				return nil, nil, err
			}
			for _, fold := range group.folds {
				if err := applyCommit(worktree, fold.ID); err != nil {
					return nil, nil, operationRefusal(UnitRereadCode, "fold %s no longer applies to the endpoint: %v", fold.ID, err)
				}
			}
			treeOut, err := gitOutput(worktree, "write-tree")
			if err != nil {
				return nil, nil, err
			}
			if err := verifyUnitPreimages(req.Repo, strings.TrimSpace(string(treeOut)), group.status); err != nil {
				return nil, nil, err
			}
			if err := applyCommit(worktree, group.status.Commit); err != nil {
				return nil, nil, operationRefusal(UnitRereadCode, "unit %s no longer applies to the endpoint: %v", group.status.Unit, err)
			}
			appliedOut, err := gitOutput(worktree, "write-tree")
			if err != nil {
				return nil, nil, err
			}
			digest, matches, err := unitTransitionMatches(worktree, strings.TrimSpace(string(treeOut)), strings.TrimSpace(string(appliedOut)), group.status)
			if err != nil {
				return nil, nil, err
			}
			if !matches {
				return nil, nil, operationRefusal(UnitRereadCode, "unit %s applies with digest %s, not attested digest %s", group.status.Unit, digest, group.status.Digest)
			}
			message, _, err := landingMessage(req.Repo, group, req.GoalID, req.Seat, req.Last && index == len(groups)-1)
			if err != nil {
				return nil, nil, err
			}
			messages[group.status.Unit] = []byte(message)
			if err := appendReceiptRow(worktree, req.GoalID, lastUnit, receiptID, receiptStamp); err != nil {
				return nil, nil, err
			}
			if _, err := gitOutput(worktree, "add", "-A"); err != nil {
				return nil, nil, err
			}
			env := []string{"GIT_AUTHOR_NAME=" + identity.Name, "GIT_AUTHOR_EMAIL=" + identity.Email,
				"GIT_COMMITTER_NAME=" + identity.Name, "GIT_COMMITTER_EMAIL=" + identity.Email}
			if _, err := gitInputEnv(worktree, env, []byte(message), "commit", "--quiet", "-F", "-"); err != nil {
				return nil, nil, err
			}
			patch, err := gitOutput(worktree, "diff", "--binary", "--full-index", strings.TrimSpace(string(parentOut)), "HEAD")
			if err != nil {
				return nil, nil, err
			}
			diffs[group.status.Unit] = patch
		}
		return messages, diffs, nil
	}
	if _, _, err := compose("pending", time.Unix(0, 0).UTC().Format(time.RFC3339)); err != nil {
		return LandResult{}, err
	}
	candidateOut, err := gitOutput(worktree, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return LandResult{}, err
	}
	projected, err := projectLandingWorkspace(req.Repo, strings.TrimSpace(string(candidateOut)))
	if err != nil {
		return LandResult{}, err
	}
	evidence, err := readLandingReceipt(req.Repo, req.TestReceipt, projected)
	if err != nil {
		return LandResult{}, err
	}
	messages, diffs, err := compose(evidence.attempt, evidence.stamp)
	if err != nil {
		return LandResult{}, err
	}
	candidateOut, err = gitOutput(worktree, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return LandResult{}, err
	}
	landingOut, err := gitOutput(worktree, "rev-parse", "HEAD^{commit}")
	if err != nil {
		return LandResult{}, err
	}
	candidate, landingTip := strings.TrimSpace(string(candidateOut)), strings.TrimSpace(string(landingOut))
	finalProjected, err := projectLandingWorkspace(req.Repo, candidate)
	if err != nil || finalProjected != projected {
		return LandResult{}, operationRefusal(LandUnprovenCode, "receipt projection moved while binding the landing rows")
	}
	proofs, err := readLandingRecord(req.Repo, req.BranchTip, req.GoalID)
	if err != nil {
		return LandResult{}, err
	}
	candidateIdentity, err := landingRetryIdentity(req.Repo, candidate, req.GoalID)
	if err != nil {
		return LandResult{}, err
	}
	for _, proof := range proofs {
		proofIdentity := proof.RetryIdentity
		if proofIdentity == "" {
			proofIdentity, err = landingRetryIdentity(req.Repo, proof.Candidate, req.GoalID)
			if err != nil {
				return LandResult{}, err
			}
		}
		if proof.Verdict == "red" && proofIdentity == candidateIdentity {
			return LandResult{}, operationRefusal(LandRetryCode, "candidate %s was already recorded red in proof %d", candidate, proof.Number)
		}
	}
	if len(proofs) != 0 {
		last := proofs[len(proofs)-1]
		if last.Verdict == "red" && !redProofChecked(req.Repo, req.BranchTip, req.GoalID, last) {
			return LandResult{}, operationRefusal(LandUncheckedCode, "proof %d is red without a clean canary on the fixed branch tip", last.Number)
		}
	}
	result := LandResult{Endpoint: req.EndpointTip, Candidate: candidate, Landing: landingTip,
		Branch: "landing/" + req.GoalID, Attempt: evidence.attempt, LastUnit: lastUnit, ProofNumber: len(proofs) + 1,
		FailingGroups: append([]string(nil), evidence.failingGroups...), RetryIdentity: candidateIdentity}
	if len(evidence.failingGroups) != 0 {
		return result, nil
	}
	if req.Hooks.AfterLandingRead != nil {
		if err := req.Hooks.AfterLandingRead(); err != nil {
			return LandResult{}, err
		}
	}
	outcome, pushErr := req.PushTransport.Push(req.Repo, req.Remote, remoteRef, expected, landingTip)
	if outcome == CASRefused {
		return LandResult{}, operationRefusal(LandBranchMovedCode, "landing branch moved after %s was observed: %v", expected, pushErr)
	}
	if outcome == CASUnknown {
		observed, present, err := req.PushTransport.RemoteTip(req.Repo, req.Remote, remoteRef)
		if err != nil || !present || observed != landingTip {
			return LandResult{}, operationRefusal(LandBranchMovedCode, "landing branch outcome is unknown: %v", pushErr)
		}
	}
	draft := LandingProof{Number: result.ProofNumber, Endpoint: result.Endpoint, Candidate: result.Candidate,
		Landing: result.Landing, RetryIdentity: result.RetryIdentity, Attempt: result.Attempt, Verdict: "pending"}
	if err := writeLandArtifacts(req.Out, result, draft, messages, diffs); err != nil {
		return LandResult{}, err
	}
	return result, nil
}
