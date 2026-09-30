package act

import (
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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
