package act

import (
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

// Reviewed is a verdict as the room performs it (g1-s69 D1, D2, §6): the review
// record by path, the verdict, and for a send-back the correction brief the
// human read and edited and, once the holder has asked, the work item the
// human named. A repeat that names the work carries no brief: the one the
// first send-back published is the brief.
type Reviewed struct {
	Record  string
	Verdict string
	Brief   string
	Work    string
	// Tip and Revision are the version the person decided on and the saved
	// revision of the record they decided on (review-findings-read-as-decisions
	// RF-02): the act publishes the record only while it still names that
	// version and still reads as that revision.
	Tip      string
	Revision string
	// Branch is the goal's branch as the server read it at the moment of the
	// press: the version that would land (fix round 1, F-3). A verdict on an
	// older version is refused, as the landing gate would refuse it.
	Branch string
}

// Recorded is the history line the act wrote, as the room says it.
type Recorded struct {
	Verdict string `json:"verdict"`
	Tip     string `json:"tip"`
	Record  string `json:"record"`
	By      string `json:"by"`
	Brief   string `json:"brief,omitempty"`
	Work    string `json:"work,omitempty"`
	Line    string `json:"line"`
}

// Review publishes goal review for one goal under this hand: the verdict line
// on the goal's history, with the review record and a send-back's brief
// published beside it in the ledger's own commit. The record is resolved in
// its home and read from the checkout as it now stands, which is the record
// the human has just recorded the Outcome into; every rule about what it must
// say is the engine's, and reaches the page in its words.
func (a Authority) Review(id string, asked Reviewed) (Recorded, error) {
	if strings.TrimSpace(id) == "" {
		return Recorded{}, refuse(KindRequest, "no-goal", "a verdict names one goal waiting to land")
	}
	if goal.VerdictWords(asked.Verdict) == "" {
		return Recorded{}, refuse(KindRequest, "verdict",
			"a verdict is clear-to-land or send-back; without one, nothing is recorded on the goal")
	}
	path, content, err := goal.ResolveReviewRecord(a.root, asked.Record)
	if err != nil {
		return Recorded{}, refuse(KindRequest, "record", err.Error())
	}
	if refusal := Unseen(asked, content); refusal != nil {
		return Recorded{}, refusal
	}
	request, done, err := a.request(id, "goal review")
	if err != nil {
		return Recorded{}, err
	}
	defer done()
	reviewing := goal.ReviewAct{Record: path, Content: content, Verdict: asked.Verdict, Brief: []byte(asked.Brief), Work: asked.Work}
	if asked.Verdict == goal.VerdictSendBack && strings.TrimSpace(asked.Brief) == "" && asked.Work != "" {
		published, readErr := goal.ReadPublished(request.Endpoint, goal.BriefPathFor(path))
		if readErr != nil {
			return Recorded{}, refuse(KindEngine, "no-brief", "naming the work repeats a send-back, and its brief is not published: "+readErr.Error())
		}
		reviewing.Brief = published
	}
	line, err := reviewing.Line(id, request.Actor.Human)
	if err != nil {
		return Recorded{}, refuse(KindRequest, "record", err.Error())
	}
	result, publishErr := goal.Review(request, id, reviewing, &a.proof)
	if err := a.settle(request, result, publishErr, "goal review"); err != nil {
		return Recorded{}, err
	}
	return Recorded{Verdict: line.Verdict, Tip: line.Tip, Record: line.Record, By: line.By, Brief: line.Brief, Work: line.Work, Line: line.Reason()}, nil
}

// Unseen is why a verdict is not published on the record as it now stands, or
// nil. Where the verdict names the version or the saved record the person
// decided on — the review room always does — the record read here must still
// be what it names (RF-02): another room can retip or write the same record
// between the person's press and this read, and publishing then would put
// their word on something they did not see. A verdict that names neither, as
// the board card's named work and an applied proposal do, is held to the
// record as it stands, as before (fix round 2, R-142-m1e). And a clear to land
// must be about the version that would land, the goal's branch as read at the
// press, because the landing gate refuses a word given at another version
// (fix round 1, F-3); a branch that could not be read is left to that gate.
func Unseen(asked Reviewed, content []byte) *Refusal {
	seen, revision := strings.TrimSpace(asked.Tip), strings.TrimSpace(asked.Revision)
	head := goal.ReadReviewRecord(content).Tip
	if seen != "" && head != seen {
		return refuse(KindRequest, "version-changed",
			"the review now names another version than the one you decided on; nothing was recorded")
	}
	if revision != "" && project.RevisionOf(content) != revision {
		return refuse(KindRequest, "version-changed",
			"the review changed after you decided, in another room; nothing was recorded, so decide again")
	}
	if branch := strings.TrimSpace(asked.Branch); asked.Verdict == goal.VerdictClearToLand && branch != "" && branch != head {
		return refuse(KindRequest, "version-moved",
			"a newer version of this goal exists; nothing was recorded: review the current version")
	}
	return nil
}
