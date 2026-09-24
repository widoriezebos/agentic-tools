package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

const readSubjectMismatchRefusal = "SUBJECT_MISMATCH"

var reviewedTreeID = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

type ReadSubjectRequest struct {
	RepoRoot, Role, Reviews, Workspace, Design, DeclaredOutputs, RootJob string
}

type readSubjectFacts interface {
	CommitParent(repo, commit string) (string, error)
	CommitTree(repo, commit string) (string, error)
	CommitDiff(repo, parentExpression, commit string) ([]byte, error)
	WorkspaceHead(workspace string) (string, error)
	LiveWorkspaceTree(repoRoot, workspaceRoot string) (string, error)
}

type gitReadSubjectFacts struct{}

func (gitReadSubjectFacts) CommitParent(repo, commit string) (string, error) {
	return gitOutput(repo, "rev-parse", commit+"^")
}

func (gitReadSubjectFacts) CommitTree(repo, commit string) (string, error) {
	return gitOutput(repo, "rev-parse", commit+"^{tree}")
}

func (gitReadSubjectFacts) CommitDiff(repo, parentExpression, commit string) ([]byte, error) {
	return gitRawOutput(repo, "diff", "--binary", "--full-index", parentExpression, commit)
}

func (gitReadSubjectFacts) WorkspaceHead(workspace string) (string, error) {
	return gitOutput(workspace, "rev-parse", "HEAD")
}

func (gitReadSubjectFacts) LiveWorkspaceTree(repoRoot, workspaceRoot string) (string, error) {
	prefix, _ := projectInstallPrefix(repoRoot)
	return (gittree.Workspace{Dir: filepath.Join(workspaceRoot, prefix)}).Snapshot("HEAD")
}

// ComputeReadSubject returns present=false with no error when a live
// subject's reviewed round has no review.json or diff.patch yet. The fold's
// existing missing-diff refusal remains responsible for that incomplete read.
func ComputeReadSubject(req ReadSubjectRequest) (ReadSubject, bool, error) {
	return computeReadSubject(req, gitReadSubjectFacts{})
}

func computeReadSubject(req ReadSubjectRequest, facts readSubjectFacts) (ReadSubject, bool, error) {
	state := loadCritiqueState(req.RepoRoot)
	reviews, design, outputsFile, outputsDigest := req.Reviews, req.Design, req.DeclaredOutputs, ""
	if req.RootJob != "" {
		root, present := state.records[req.RootJob]
		if !present {
			return ReadSubject{}, false, fmt.Errorf("critic root record %s is unreadable", req.RootJob)
		}
		reviews = asString(root["reviews"])
		design = asString(root["design"])
		outputsFile = ""
		outputsDigest = asString(root["declaredOutputsDigest"])
	}

	switch req.Role {
	case "design-critic":
		subject, err := designReadSubject(facts, req.Workspace, design, outputsFile, outputsDigest)
		return subject, err == nil, err
	case "code-critic":
		if validCommitReview.MatchString(reviews) {
			subject, err := commitReadSubject(facts, req.RepoRoot, reviews)
			return subject, err == nil, err
		}
	case "warden":
	default:
		return ReadSubject{}, false, fmt.Errorf("role %s does not read a critique subject", req.Role)
	}
	if req.RootJob != "" {
		var present bool
		var err error
		reviews, present, err = latestCompletedImplementerSubjectMember(state, reviews)
		if err != nil || !present {
			return ReadSubject{}, present, err
		}
	}

	subject, workspaceRoot, present, err := liveReadSubject(state, reviews)
	if err != nil || !present {
		return subject, present, err
	}
	if err := checkLiveSubjectWorkspace(facts, req.RepoRoot, workspaceRoot, subject); err != nil {
		return ReadSubject{}, false, err
	}
	return subject, true, nil
}

func latestCompletedImplementerSubjectMember(state critiqueState, reviewedJob string) (string, bool, error) {
	reviewed, present := state.records[reviewedJob]
	if !present || asString(reviewed["role"]) != "implementer" {
		return "", false, fmt.Errorf("reviewed job %s is not an implementer record", reviewedJob)
	}
	implementerRoot := state.chainRoot(reviewedJob)
	if implementerRoot == "" {
		return "", false, fmt.Errorf("reviewed implementer job %s has no readable chain root", reviewedJob)
	}

	latestJob := ""
	latestRound := int64(0)
	tied := false
	for jobID, record := range state.records {
		if state.chainRoot(jobID) != implementerRoot || asString(record["role"]) != "implementer" || asString(record["status"]) != "completed" {
			continue
		}
		round, ok := numInt(record["round"])
		if !ok || round < 1 {
			return "", false, fmt.Errorf("reviewed implementer chain %s has a completed member with an invalid round", implementerRoot)
		}
		roundDir := filepath.Join(state.agents, implementerRoot, "rounds", fmt.Sprint(round))
		hasArtifacts := true
		for _, name := range []string{"review.json", "diff.patch"} {
			if _, err := os.Stat(filepath.Join(roundDir, name)); err != nil {
				if os.IsNotExist(err) {
					hasArtifacts = false
					break
				}
				return "", false, fmt.Errorf("reviewed implementer job %s has an unreadable %s: %v", jobID, name, err)
			}
		}
		if !hasArtifacts || round < latestRound {
			continue
		}
		if round == latestRound {
			tied = true
			continue
		}
		latestJob, latestRound, tied = jobID, round, false
	}
	if tied {
		return "", false, fmt.Errorf("reviewed implementer chain %s has more than one completed subject member at round %d", implementerRoot, latestRound)
	}
	return latestJob, latestJob != "", nil
}

func liveReadSubject(state critiqueState, reviewedJob string) (subject ReadSubject, workspaceRoot string, present bool, err error) {
	reviewed, ok := state.records[reviewedJob]
	if !ok || asString(reviewed["role"]) != "implementer" {
		return subject, "", false, fmt.Errorf("reviewed job %s is not an implementer record", reviewedJob)
	}
	round, ok := numInt(reviewed["round"])
	if !ok || round < 1 {
		return subject, "", false, fmt.Errorf("reviewed implementer job %s has an invalid round", reviewedJob)
	}
	implementerRoot := state.chainRoot(reviewedJob)
	if implementerRoot == "" {
		return subject, "", false, fmt.Errorf("reviewed implementer job %s has no readable chain root", reviewedJob)
	}
	roundDir := filepath.Join(state.agents, implementerRoot, "rounds", fmt.Sprint(round))
	reviewBytes, reviewErr := os.ReadFile(filepath.Join(roundDir, "review.json"))
	if reviewErr != nil {
		if os.IsNotExist(reviewErr) {
			return ReadSubject{}, "", false, nil
		}
		return subject, "", false, fmt.Errorf("reviewed implementer job %s has an unreadable review.json: %v", reviewedJob, reviewErr)
	}
	patch, patchErr := os.ReadFile(filepath.Join(roundDir, "diff.patch"))
	if patchErr != nil {
		if os.IsNotExist(patchErr) {
			return ReadSubject{}, "", false, nil
		}
		return subject, "", false, fmt.Errorf("reviewed implementer job %s has an unreadable diff.patch: %v", reviewedJob, patchErr)
	}
	var review struct {
		ReviewedTree string `json:"reviewedTree"`
	}
	if err := json.Unmarshal(reviewBytes, &review); err != nil {
		return subject, "", false, fmt.Errorf("reviewed implementer job %s has a malformed review.json: %v", reviewedJob, err)
	}
	if !reviewedTreeID.MatchString(review.ReviewedTree) {
		return subject, "", false, fmt.Errorf("reviewed implementer job %s has an invalid reviewedTree %q", reviewedJob, review.ReviewedTree)
	}
	digest := sha256.Sum256(patch)
	return ReadSubject{
		Kind:                SubjectLive,
		ImplementerRoot:     implementerRoot,
		ReviewedMember:      reviewedJob,
		ReviewedProjectTree: review.ReviewedTree,
		DiffDigest:          hex.EncodeToString(digest[:]),
	}, asString(reviewed["workspaceRoot"]), true, nil
}

func commitReadSubject(facts readSubjectFacts, repoRoot, reviews string) (ReadSubject, error) {
	commit := strings.TrimPrefix(reviews, "commit:")
	parent, err := facts.CommitParent(repoRoot, commit)
	if err != nil {
		return ReadSubject{}, fmt.Errorf("commit subject %s has no readable parent: %v", reviews, err)
	}
	tree, err := facts.CommitTree(repoRoot, commit)
	if err != nil {
		return ReadSubject{}, fmt.Errorf("commit subject %s has no readable tree: %v", reviews, err)
	}
	patch, err := facts.CommitDiff(repoRoot, commit+"^", commit)
	if err != nil {
		return ReadSubject{}, fmt.Errorf("commit subject %s has no readable parent diff: %v", reviews, err)
	}
	digest := sha256.Sum256(patch)
	return ReadSubject{
		Kind: SubjectCommit, Commit: commit, Parent: parent, Tree: tree,
		DiffDigest: hex.EncodeToString(digest[:]),
	}, nil
}

func designReadSubject(facts readSubjectFacts, workspace, design, outputsFile, recordedOutputsDigest string) (ReadSubject, error) {
	abs := design
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(workspace, design)
	}
	rel, err := filepath.Rel(workspace, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ReadSubject{}, fmt.Errorf("design path must be inside the reviewed workspace")
	}
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "metasystem/") {
		return ReadSubject{}, fmt.Errorf("design path must begin metasystem/")
	}
	content, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(rel)))
	if err != nil {
		return ReadSubject{}, fmt.Errorf("design path %s is unreadable: %v", rel, err)
	}
	outputsDigest := recordedOutputsDigest
	if outputsFile != "" {
		outputs, parseErr := ParseDeclaredOutputs(outputsFile)
		if parseErr != nil {
			return ReadSubject{}, parseErr
		}
		outputsDigest = digestDeclaredOutputs(outputs)
	}
	if outputsDigest == "" {
		return ReadSubject{}, fmt.Errorf("design subject has no declared outputs digest")
	}
	reviewedCommit, err := facts.WorkspaceHead(workspace)
	if err != nil {
		return ReadSubject{}, fmt.Errorf("design workspace has no readable HEAD: %v", err)
	}
	digest := sha256.Sum256(content)
	return ReadSubject{
		Kind: SubjectDesign, DesignPath: rel,
		ContentDigest:         hex.EncodeToString(digest[:]),
		DeclaredOutputsDigest: outputsDigest,
		ReviewedCommit:        reviewedCommit,
	}, nil
}

func checkLiveSubjectWorkspace(facts readSubjectFacts, repoRoot, workspaceRoot string, subject ReadSubject) error {
	observed := "unavailable"
	info, statErr := os.Stat(workspaceRoot)
	if statErr == nil && info.IsDir() {
		snapshot, snapshotErr := facts.LiveWorkspaceTree(repoRoot, workspaceRoot)
		if snapshotErr == nil {
			observed = snapshot
			if observed == subject.ReviewedProjectTree {
				return nil
			}
		}
	}
	return &OpError{
		Code:   11,
		Reason: readSubjectMismatchRefusal,
		Message: fmt.Sprintf(
			"reviewed job %s recorded tree %s but its workspace at %s has tree %s; next: dispatch.sh follow-up on the implementer, validate conformance --stage review --job <next implementer round>, dispatch the critic again",
			subject.ReviewedMember, subject.ReviewedProjectTree, workspaceRoot, observed),
	}
}

func WriteReadSubject(path string, subject ReadSubject) error {
	data, err := json.MarshalIndent(subject, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Chmod(path, 0o644)
}
