package httpd

// The gate's reading of a landing word fetches the goal branch from origin
// first (SOL-S70-04 round 2): a checkout that has not fetched since the branch
// moved at origin still reads the move, a goal without a landing word fetches
// nothing, and a fetch that fails reads the local ref as before.

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/decisions"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/review"
)

func gitLine(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// staleCheckout is a clone whose origin/goal/landing names the commit it last
// fetched, while the bare origin's goal/landing has moved one commit on.
func staleCheckout(t *testing.T) (checkout, fetched, moved string) {
	t.Helper()
	parent := t.TempDir()
	origin, work := filepath.Join(parent, "origin.git"), filepath.Join(parent, "work")
	checkout = filepath.Join(parent, "checkout")
	runGit(t, parent, "init", "--bare", "-q", "-b", "main", origin)
	runGit(t, parent, "init", "-q", "-b", "main", work)
	runGit(t, work, "commit", "-q", "--allow-empty", "-m", "base")
	runGit(t, work, "remote", "add", "origin", origin)
	runGit(t, work, "push", "-q", "origin", "main")
	runGit(t, work, "checkout", "-q", "-b", "goal/landing")
	runGit(t, work, "commit", "-q", "--allow-empty", "-m", "reviewed")
	runGit(t, work, "push", "-q", "origin", "goal/landing")
	runGit(t, parent, "clone", "-q", origin, checkout)
	fetched = gitLine(t, checkout, "rev-parse", "origin/goal/landing")
	runGit(t, work, "commit", "-q", "--allow-empty", "-m", "moved on")
	runGit(t, work, "push", "-q", "origin", "goal/landing")
	moved = gitLine(t, work, "rev-parse", "HEAD")
	if fetched == moved || gitLine(t, checkout, "rev-parse", "origin/goal/landing") != fetched {
		t.Fatalf("the checkout is not stale: fetched %s, moved %s", fetched, moved)
	}
	return checkout, fetched, moved
}

// countedFetches is the checkout's Git, counting the branches it fetches.
type countedFetches struct {
	review.Git
	fetched *[]string
}

func (c countedFetches) FetchBranch(branch string) error {
	*c.fetched = append(*c.fetched, branch)
	return c.Git.FetchBranch(branch)
}

func clearedAtTip(tip string) goal.HistoryLine {
	return goal.HistoryLine{At: "2026-09-28T08:00:00Z", Opid: "01J5X0000000000000000000W1-mac-ui-1a2b3c4d", Verb: "review", Actor: "human:Wido",
		Reason: "reviewed verdict=clear-to-land tip=" + tip + " record=plans/reviews/review-of-landing.md by=Wido"}
}

// gateReading is the landing row's word on the board and whether the inbox
// asks for the word again.
func gateReading(t *testing.T, label string, git review.Git, lines ...goal.HistoryLine) (*struct {
	Moved     bool
	BranchTip string
}, bool) {
	t.Helper()
	ledger := &gateLedger{lines: lines}
	info := Info{Project: func() (project.Pane, error) { return project.Pane{}, nil }, Review: &review.Owner{Git: git}}
	ledger.wire(&info)
	served := New(info, loopback(), testBundle())
	var board backlogPayload
	testutil.Require(t, label+": the board", json.Unmarshal(get(t, served, backlogPath, nil).Body.Bytes(), &board), nil)
	var word *struct {
		Moved     bool
		BranchTip string
	}
	for _, row := range board.Rows {
		if row.ID == "landing" && row.Gate != nil && row.Gate.Reviewed != nil {
			word = &struct {
				Moved     bool
				BranchTip string
			}{row.Gate.Reviewed.Moved, row.Gate.Reviewed.BranchTip}
		}
	}
	var page decisions.Page
	testutil.Require(t, label+": the inbox", json.Unmarshal(get(t, served, decisionsPath, nil).Body.Bytes(), &page), nil)
	asked := false
	for _, need := range page.NeedsYou {
		if need.Kind == decisions.KindLanding && need.ID == "landing" && need.Act == decisions.ActLandWithoutSitting {
			asked = true
		}
	}
	return word, asked
}

func TestAWordIsReadAgainstTheBranchAtOriginNotTheCheckoutsLastFetch(t *testing.T) {
	t.Parallel()
	checkout, fetched, moved := staleCheckout(t)
	word, asked := gateReading(t, "stale", gittree.Workspace{Dir: checkout}, clearedAtTip(fetched))
	if word == nil || !word.Moved || word.BranchTip != moved {
		t.Fatalf("the board reads the checkout's last fetch, not origin's tip %s: %+v", moved, word)
	}
	testutil.Expect(t, "the inbox asks for the word again", asked, true)
}

func TestAGoalWithoutALandingWordFetchesNothing(t *testing.T) {
	t.Parallel()
	checkout, fetched, _ := staleCheckout(t)
	var branches []string
	git := countedFetches{Git: gittree.Workspace{Dir: checkout}, fetched: &branches}
	gateReading(t, "no word", git)
	sentBack := clearedAtTip(fetched)
	sentBack.Reason = strings.Replace(sentBack.Reason, "clear-to-land", "send-back", 1) + " brief=" + goal.BriefPathFor("plans/reviews/review-of-landing.md")
	gateReading(t, "sent back", git, sentBack)
	testutil.Expect(t, "no word and a send-back fetch nothing", branches, []string(nil))

	gateReading(t, "cleared", git, clearedAtTip(fetched))
	// One board read and one inbox read, each fetching the one branch once.
	testutil.Expect(t, "a landing word fetches its branch once per reading", branches, []string{"goal/landing", "goal/landing"})
}

func TestAFailedFetchLeavesTheReadingAsItWas(t *testing.T) {
	t.Parallel()
	checkout, fetched, _ := staleCheckout(t)
	runGit(t, checkout, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))
	word, asked := gateReading(t, "unfetchable", gittree.Workspace{Dir: checkout}, clearedAtTip(fetched))
	if word == nil || word.Moved || word.BranchTip != "" {
		t.Fatalf("a failed fetch changed the reading of the local ref: %+v", word)
	}
	testutil.Expect(t, "the word stands and nothing is asked", asked, false)
}
