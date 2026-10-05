package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

type correctedBriefRepository struct{ root, unit, goal string }

func (r correctedBriefRepository) Range(_, _, _, _ string) ([]branch.Commit, error) {
	return []branch.Commit{{ID: r.unit, Kind: branch.Unit, Unit: "u1", Units: []string{"u1"}}}, nil
}
func (r correctedBriefRepository) Subject(_, _ string) (branch.AttestationSubject, error) {
	return branch.AttestationSubject{Commit: r.unit, Parent: strings.Repeat("a", 40), Tree: strings.Repeat("d", 40), PatchDigest: strings.Repeat("e", 64), UnitDigest: strings.Repeat("f", 64)}, nil
}
func (r correctedBriefRepository) CommonDir(_ string) (string, error) {
	return filepath.Join(r.root, ".git"), nil
}
func (r correctedBriefRepository) Entries(_, _ string) ([]branch.Entry, error) {
	return nil, errors.New("no plan in fixture")
}
func (r correctedBriefRepository) Detached(_, _ string) (string, func() error, error) {
	return r.root, func() error { return nil }, nil
}

func TestReviewOfABuiltUnitTakesACorrectedBrief(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	original := bed.brief("build.md", "Working Mode: implement\n\nBuild the unit.\n")
	code, built, _ := bed.work(append([]string{"work", "build", bed.id, "--work", "u1", "--brief", original, "--lines", "10"}, workCheck...)...)
	if code != 0 || built.Outcome != intentConfirmed {
		t.Fatalf("build: code=%d %+v", code, built)
	}
	run := resultData(t, built)["run"].(string)
	runner := &launch.UnitRunner{Root: bed.unitRoot}
	unit := strings.Repeat("b", 40)
	if err := runner.ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		body, err := os.ReadFile(review.BuildBrief)
		if err != nil || review.BuildBriefSHA256 != fmt.Sprintf("%x", sha256.Sum256(body)) {
			t.Fatalf("build digest=%s err=%v", review.BuildBriefSHA256, err)
		}
		return retain(launch.UnitSubject{Round: review.Round.Number, ExpectedParent: review.Head, ResultDigest: launch.UnitResultDigest(review.Result), DiffDigest: review.DiffDigest, Commit: unit, Tip: unit, Published: unit})
	}); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(bed.unitRoot, run, "run.json")
	before := mustRead(t, recordPath)
	launches := bed.starter.launched()
	corrected := bed.brief("corrected.md", "Working Mode: implement\n\nReview `docs/absent.md`.\n")
	owners := bed.workOwners()
	owners.work.inspectRead = func(_, _, _ string) (branch.BranchReadResult, error) { return branch.BranchReadResult{}, nil }
	owners.connection.endpointTip = func(_ string, _ goal.Endpoint) (string, error) { return "", nil }
	repository := correctedBriefRepository{root: bed.worktree, unit: unit, goal: bed.id}
	delegates := 0
	var composed string
	owners.delivery = &intentDeliveryOwners{branchRead: func(args []string) (branch.BranchReadResult, int, error) {
		value := func(name string) string {
			for i, arg := range args {
				if arg == name && i+1 < len(args) {
					return args[i+1]
				}
			}
			return ""
		}
		if slices.Contains(args, "--join") {
			t.Fatal("a request with --brief joined an existing review")
		}
		if value("--brief") != filepath.Join(bed.root(), corrected) {
			t.Fatalf("supplied brief=%q", value("--brief"))
		}
		result, err := branch.RunBranchRead(branch.BranchReadRequest{Repo: repository.root, Remote: "origin", EndpointTip: strings.Repeat("a", 40), BranchTip: unit, GoalID: bed.id, UnitCommit: unit, Repository: repository,
			BriefPath: value("--brief"), BuildBriefSHA256: value("--build-brief-sha256"), CheckClaim: func() error { return nil }, Gate: func(string) (string, error) { return "green", nil },
			Delegate: func(brief, _, _, _, _ string) (string, error) {
				composed = string(mustRead(t, brief))
				if !strings.Contains(composed, "# Corrected implementation brief (given at review)") || strings.Contains(composed, "# Supplied accepted implementation brief (frozen at dispatch)") {
					t.Fatalf("corrected brief was frozen:\n%s", composed)
				}
				git := func(_ string, args ...string) (string, error) {
					switch strings.Join(args, " ") {
					case "rev-parse --show-prefix":
						return "", nil
					case "rev-parse --verify HEAD^{commit}":
						return "head", nil
					case "rev-parse --verify --end-of-options " + unit + "^{commit}":
						return unit, nil
					case "ls-tree -d --name-only head", "ls-tree -d --name-only " + unit:
						return "docs", nil
					}
					return "", errors.New("path is absent")
				}
				if _, err := dispatchcore.ReadReviewBriefAdmission(brief, repository.root, repository.root, repository.root, "commit:"+unit, git); err != nil {
					return "", &branch.ReadNeverLaunchedError{Err: err}
				}
				delegates++
				return "critic-corrected", nil
			},
		})
		if err != nil {
			return result, 1, err
		}
		return result, 0, nil
	}}
	code, refused := bed.runJSON(owners, "work", "review", bed.id, "--work", "u1", "--brief", corrected)
	if code == 0 || refused.Outcome != intentRefused || !strings.Contains(resultWords(refused), "docs/absent.md") || delegates != 0 {
		t.Fatalf("missing corrected citation: code=%d %+v delegates=%d", code, refused, delegates)
	}
	if err := os.WriteFile(filepath.Join(bed.root(), corrected), []byte("Working Mode: implement\n\nReview the corrected requirements.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, reviewed := bed.runJSON(owners, "work", "review", bed.id, "--work", "u1", "--brief", corrected)
	if reviewed.Outcome != intentInProgress || delegates != 1 || !strings.Contains(composed, "Review the corrected requirements.") {
		t.Fatalf("corrected review: code=%d %+v delegates=%d", code, reviewed, delegates)
	}
	if !slices.Equal(launches, bed.starter.launched()) || string(before) != string(mustRead(t, recordPath)) {
		t.Fatal("review changed the unit run or started another builder round")
	}
	var read map[string]any
	data := mustRead(t, filepath.Join(repository.root, ".git", "metasystem", "goal-reads", bed.id, unit+".json"))
	if err := json.Unmarshal(data, &read); err != nil {
		t.Fatal(err)
	}
	if read["briefInputPath"] != filepath.Join(bed.root(), corrected) || read["briefInputSha256"] != fmt.Sprintf("%x", sha256.Sum256(mustRead(t, filepath.Join(bed.root(), corrected)))) {
		t.Fatalf("read did not retain corrected input: %s", data)
	}
}
