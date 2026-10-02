package channel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

type Option struct {
	Label       string `json:"label"`
	Consequence string `json:"consequence"`
}
type Rejection struct {
	Ref     MessageRef  `json:"ref"`
	Reason  string      `json:"reason"`
	At      time.Time   `json:"at"`
	Posted  bool        `json:"posted"`
	PostRef *MessageRef `json:"postRef"`
}
type Answer struct {
	Text         string     `json:"text"`
	UserID       string     `json:"userID"`
	Ref          MessageRef `json:"ref"`
	At           time.Time  `json:"at"`
	Step         int64      `json:"step"`
	ULID         string     `json:"ulid"`
	Opid         string     `json:"opid"`
	ApprovalULID string     `json:"approvalULID,omitempty"`
	Receipt      string     `json:"receipt,omitempty"`
	Phase        string     `json:"phase"`
}
type Question struct {
	ID   string `json:"id"`
	Goal string `json:"goal"`
	// About names what a question without a goal is about: lane or machine.
	About   string `json:"about,omitempty"`
	Kind    string `json:"kind"`
	Machine string `json:"machine"`
	// Lineage is the asking session's owner lineage; with Machine it names
	// the session whose turn may end while the question is open.
	Lineage        string       `json:"lineage,omitempty"`
	OpenedAt       time.Time    `json:"openedAt"`
	Facts          []string     `json:"facts"`
	Options        []Option     `json:"options"`
	Recommendation string       `json:"recommendation"`
	Wants          string       `json:"wants"`
	Budget         *goal.Budget `json:"budget,omitempty"`
	Thread         *MessageRef  `json:"thread"`
	State          string       `json:"state"`
	Undelivered    int          `json:"undelivered"`
	Answer         *Answer      `json:"answer"`
	Rejected       []Rejection  `json:"rejected"`
	FactsDigest    string       `json:"factsDigest"`
	LedgerCursor   string       `json:"ledgerCursor,omitempty"`
}

type AskRequest struct {
	Context                                       context.Context
	RepoRoot, Goal, About, Kind, Machine, Lineage string
	Facts                                         []string
	Options                                       []Option
	Recommendation, Wants                         string
	Budget                                        *goal.Budget
	Provider                                      Provider
	Destination                                   DestinationConfig
	Now                                           time.Time
	LedgerCursor                                  string
}

const (
	// This human-reading limit is intentionally independent of and smaller than provider transport limits.
	questionMessageRuneLimit = 1600
	questionFactLimit        = 4
)

func channelRoot(repo string) string { return filepath.Join(repo, "artifacts", "agents", "channel") }
func questionPath(repo, id string) string {
	return filepath.Join(channelRoot(repo), "questions", id+".json")
}
func writeJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return writeDurable(path, b)
}
func writeDurable(path string, b []byte) error {
	_, err := atomicfile.WriteFile(path, b, 0o600, "")
	return err
}
func ReadQuestion(repo, id string) (Question, error) {
	var q Question
	b, err := os.ReadFile(questionPath(repo, id))
	if err != nil {
		return q, err
	}
	return q, json.Unmarshal(b, &q)
}
func listQuestions(repo string) ([]Question, error) {
	paths, _ := filepath.Glob(filepath.Join(channelRoot(repo), "questions", "*.json"))
	sort.Strings(paths)
	out := []Question{}
	for _, p := range paths {
		var q Question
		b, e := os.ReadFile(p)
		if e != nil {
			return nil, e
		}
		if e = json.Unmarshal(b, &q); e != nil {
			return nil, e
		}
		out = append(out, q)
	}
	return out, nil
}

// WalkOpenQuestions is the tolerant local question walk shared by brain boot
// and the turn-end scanner. One unreadable file never hides the other open
// questions; its path is returned on the separate failure channel.
func WalkOpenQuestions(repo string) ([]Question, []string) {
	all, unreadable := WalkQuestions(repo)
	var questions []Question
	for _, question := range all {
		if question.State == "open" {
			questions = append(questions, question)
		}
	}
	return questions, unreadable
}

// WalkQuestions reads every question record, open or ended, newest first,
// with the same tolerance as WalkOpenQuestions.
func WalkQuestions(repo string) ([]Question, []string) {
	paths, globErr := filepath.Glob(filepath.Join(channelRoot(repo), "questions", "*.json"))
	if globErr != nil {
		return nil, []string{globErr.Error()}
	}
	var questions []Question
	var unreadable []string
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			unreadable = append(unreadable, path+": "+err.Error())
			continue
		}
		var question Question
		if err := json.Unmarshal(data, &question); err != nil || question.ID == "" || question.OpenedAt.IsZero() || question.State == "" {
			if err == nil {
				err = fmt.Errorf("missing required question fields")
			}
			unreadable = append(unreadable, path+": "+err.Error())
			continue
		}
		questions = append(questions, question)
	}
	sort.Slice(questions, func(i, j int) bool { return questions[i].OpenedAt.After(questions[j].OpenedAt) })
	return questions, unreadable
}

// GoalOpenQuestions is the open questions as the goal judgment reads them:
// the goal each names, what a goal-less one is about, and who asked. An
// unreadable record is left out; the question list names it.
func GoalOpenQuestions(repo string) []goal.OpenQuestion {
	open, _ := WalkOpenQuestions(repo)
	out := make([]goal.OpenQuestion, 0, len(open))
	for _, q := range open {
		out = append(out, goal.OpenQuestion{ID: q.ID, Goal: q.Goal, About: q.About, Machine: q.Machine, Lineage: q.Lineage})
	}
	return out
}

func validateQuestionBudget(q Question) error {
	if q.Kind == "carry" {
		if q.Budget != nil {
			return fmt.Errorf("a carry question cannot carry a proposed budget tuple")
		}
		if !goal.ValidCarryToken(q.Wants) {
			return fmt.Errorf("a carry question requires --wants with exactly carry workspace=<sha40> goal=<id> past=<name>")
		}
		return nil
	}
	if q.Kind == "budget-above-norm" {
		if q.Budget == nil {
			return fmt.Errorf("a budget-above-norm question requires a complete proposed budget tuple")
		}
		if err := q.Budget.Validate(); err != nil {
			return fmt.Errorf("a budget-above-norm question requires a complete valid proposed budget tuple: %v", err)
		}
		return nil
	}
	if q.Budget != nil {
		return fmt.Errorf("question kind %s cannot carry a proposed budget tuple", q.Kind)
	}
	return nil
}
func factsDigest(goalID, kind string, facts []string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00", goalID, kind)
	for _, f := range facts {
		h.Write([]byte(f))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func Ask(r AskRequest) (Question, error) {
	q, _, err := AskOrFind(r)
	return q, err
}

// AskOrFind is Ask that also says whether the question already stood. An
// open question for the same goal and kind with the same facts, options,
// recommendation, wanted token and proposed budget is the requested state
// already holding (R-129-ui): it is returned with existing set, and nothing
// is written, posted or published again. The same facts with any other
// parameter is a different question and is asked.
func AskOrFind(r AskRequest) (Question, bool, error) {
	if r.Now.IsZero() {
		r.Now = time.Now().UTC()
	}
	if r.About != "" {
		switch {
		case r.About != "lane" && r.About != "machine":
			return Question{}, false, fmt.Errorf("a question without a goal is about the lane or the machine, not %q", r.About)
		case r.Goal != "":
			return Question{}, false, fmt.Errorf("a question names a goal or says what it is about (--about), not both")
		case r.Kind != "other":
			return Question{}, false, fmt.Errorf("a %s question carries authority over a goal, so it cannot be asked --about %s", r.Kind, r.About)
		}
	}
	if (r.Goal == "" && r.About == "") || r.Kind == "" || len(r.Facts) == 0 {
		return Question{}, false, fmt.Errorf("ask requires goal, kind, and a fact")
	}
	switch r.Kind {
	case "budget-above-norm", "carry", "fork", "reserved-decision", "stop", "other":
	default:
		return Question{}, false, fmt.Errorf("unknown question kind %q", r.Kind)
	}
	digest := factsDigest(questionSubjectKey(r.Goal, r.About), r.Kind, r.Facts)
	existing, err := listQuestions(r.RepoRoot)
	if err != nil {
		return Question{}, false, err
	}
	for _, q := range existing {
		if q.State == "open" && q.Goal == r.Goal && q.About == r.About && q.Kind == r.Kind && q.FactsDigest == digest && sameAsk(q, r) {
			return q, true, nil
		}
	}
	id, err := goal.NewOperationULID()
	if err != nil {
		return Question{}, false, err
	}
	q := Question{ID: id, Goal: r.Goal, About: r.About, Kind: r.Kind, Machine: r.Machine, Lineage: r.Lineage, OpenedAt: r.Now.UTC(), Facts: r.Facts, Options: r.Options, Recommendation: r.Recommendation, Wants: r.Wants, Budget: r.Budget, State: "open", FactsDigest: digest, LedgerCursor: r.LedgerCursor}
	if err = validateQuestionBudget(q); err != nil {
		return Question{}, false, err
	}
	if err = writeJSON(questionPath(r.RepoRoot, id), q); err != nil {
		return Question{}, false, err
	}
	if r.Provider != nil {
		postContext := r.Context
		if postContext == nil {
			postContext = context.Background()
		}
		ref, postErr := r.Provider.Post(postContext, r.Destination, renderQuestion(q, answerCodeOffOrOn(r.RepoRoot)), nil)
		if postErr == nil {
			q.Thread = &ref
		} else {
			q.Undelivered++
		}
		if err = writeJSON(questionPath(r.RepoRoot, id), q); err != nil {
			return q, false, err
		}
	}
	// A question without a goal has no goal file to mark.
	if r.Lineage != "" && r.Goal != "" {
		ep, e := goal.ResolveEndpoint(r.RepoRoot)
		if e != nil {
			return q, false, e
		}
		ulid, e := goal.NewOperationULID()
		if e != nil {
			return q, false, e
		}
		published, e := goal.Asked(goal.VerbRequest{Endpoint: ep, Actor: goal.Actor{Machine: r.Machine, Lineage: r.Lineage}, Ulid: ulid, Now: r.Now}, r.Goal, id, r.Kind, r.Facts[0])
		if e != nil {
			return q, false, e
		}
		if published.Outcome != goal.OutcomeConfirmed {
			return q, false, fmt.Errorf("goal ask was not confirmed: %s", published.Detail)
		}
	}
	return q, false, nil
}

// sameAsk says whether an open question carries exactly the options,
// recommendation, wanted token and proposed budget a new request asks with.
func sameAsk(q Question, r AskRequest) bool {
	if q.Recommendation != r.Recommendation || q.Wants != r.Wants || !reflect.DeepEqual(q.Budget, r.Budget) {
		return false
	}
	if len(q.Options) != len(r.Options) {
		return false
	}
	for index := range q.Options {
		if q.Options[index] != r.Options[index] {
			return false
		}
	}
	return true
}

// contextBackground avoids making Ask's durable-write ordering dependent on a caller context.
type contextBackground struct{}

func (contextBackground) Deadline() (time.Time, bool) { return time.Time{}, false }
func (contextBackground) Done() <-chan struct{}       { return nil }
func (contextBackground) Err() error                  { return nil }
func (contextBackground) Value(any) any               { return nil }

// AnswerCodeOff reads channel.human.answer-code: "on" (the default) asks the
// human for a TOTP code with every answer; "off" (Wido 2026-10-02) takes the
// sender's channel user id alone as the proof that the answer is the human's
// word, with the same authority.
func AnswerCodeOff(root string) (bool, error) {
	value, code, err := config.Get(config.GetParams{Key: "channel.human.answer-code", ConfPath: filepath.Join(root, "metasystem.conf"), Default: "on", DefaultSet: true})
	if code != 0 {
		return false, err
	}
	switch strings.TrimSpace(value) {
	case "on":
		return false, nil
	case "off":
		return true, nil
	default:
		return false, fmt.Errorf("channel.human.answer-code must be on or off, not %q", value)
	}
}

// answerCodeOffOrOn is AnswerCodeOff for wording: an unreadable setting keeps
// asking for the code.
func answerCodeOffOrOn(root string) bool {
	off, err := AnswerCodeOff(root)
	return err == nil && off
}

// ReplyInstructions is how the human answers q: in its authenticated
// channel thread, never through a local command.
func ReplyInstructions(q Question) string {
	return replyInstructions(q, false)
}

// ReplyInstructionsAt is ReplyInstructions under the repository's
// channel.human.answer-code setting.
func ReplyInstructionsAt(root string, q Question) string {
	return replyInstructions(q, answerCodeOffOrOn(root))
}

func replyInstructions(q Question, codeOff bool) string {
	if codeOff {
		if q.Wants != "" {
			return "Reply in this thread with this token verbatim:\n" + q.Wants
		}
		return "Reply in this thread with your answer"
	}
	if q.Wants != "" {
		return "Reply in this thread with this token verbatim, followed by your code:\n" + q.Wants
	}
	return "Reply in this thread with your answer followed by your code"
}

func renderQuestion(q Question, codeOff bool) string {
	tail := replyInstructions(q, codeOff)

	full := renderQuestionParts(q, q.Facts, optionConsequences(q.Options), q.Recommendation, "", tail)
	if len([]rune(full)) <= questionMessageRuneLimit {
		return full
	}

	factCount := len(q.Facts)
	factLimit := questionFactLimit
	if q.Kind == "carry" {
		factLimit = 6
	}
	if factCount > factLimit {
		factCount = factLimit
	}
	noticeReserve := 0
	for dropped := 0; dropped <= len(q.Facts); dropped++ {
		if size := len([]rune(questionTrimNotice(questionDetailsHome(q), dropped))); size > noticeReserve {
			noticeReserve = size
		}
	}
	mandatory := len([]rune(questionHead(q))) + len([]rune(questionBudgetLine(q))) + len([]rune(tail)) + noticeReserve + len([]rune("Recommendation: \n"))
	for _, o := range q.Options {
		mandatory += len([]rune(o.Label + ": \n"))
	}
	for factCount > 0 && mandatory+factCount*len([]rune("- \n")) > questionMessageRuneLimit {
		factCount--
	}
	mandatory += factCount * len([]rune("- \n"))
	remaining := questionMessageRuneLimit - mandatory
	if remaining < 0 {
		remaining = 0
	}

	consequenceBudget := remaining / 2
	consequences := trimQuestionParts(optionConsequences(q.Options), consequenceBudget)
	remaining -= questionPartsRunes(consequences)
	recommendationBudget := remaining / 3
	recommendation := trimQuestionPart(q.Recommendation, recommendationBudget)
	remaining -= len([]rune(recommendation))
	facts := trimQuestionParts(q.Facts[:factCount], remaining)
	notice := questionTrimNotice(questionDetailsHome(q), len(q.Facts)-factCount)
	return renderQuestionParts(q, facts, consequences, recommendation, notice, tail)
}

func renderQuestionParts(q Question, facts, consequences []string, recommendation, notice, tail string) string {
	var b strings.Builder
	b.WriteString(questionHead(q))
	for _, f := range facts {
		fmt.Fprintf(&b, "- %s\n", f)
	}
	if notice != "" {
		b.WriteString(notice)
	}
	for i, o := range q.Options {
		fmt.Fprintf(&b, "%s: %s\n", o.Label, consequences[i])
	}
	b.WriteString(questionBudgetLine(q))
	fmt.Fprintf(&b, "Recommendation: %s\n", recommendation)
	b.WriteString(tail)
	return b.String()
}

func questionHead(q Question) string {
	return fmt.Sprintf("%s — %s\n", QuestionSubject(q), q.Kind)
}

// QuestionSubject is what a question is about, in words: its goal, or the
// landing lane or the machine for a question without one.
func QuestionSubject(q Question) string {
	switch {
	case q.Goal != "":
		return strings.ReplaceAll(q.Goal, "-", " ")
	case q.About == "lane":
		return "the landing lane on " + q.Machine
	default:
		return "the machine " + q.Machine
	}
}

// questionSubjectKey keeps a goal's question and a goal-less one apart in
// the facts digest.
func questionSubjectKey(goalID, about string) string {
	if goalID == "" {
		return "about:" + about
	}
	return goalID
}

func questionBudgetLine(q Question) string {
	if q.Budget != nil {
		return fmt.Sprintf("Proposed box: %s\n", renderProposedBox(*q.Budget))
	}
	return ""
}

// questionDetailsHome is where a trimmed question's full text is kept: its
// goal, or the question record for a question without one.
func questionDetailsHome(q Question) string {
	if q.Goal == "" {
		return "question " + q.ID
	}
	return "goal " + q.Goal
}

func questionTrimNotice(home string, dropped int) string {
	if dropped == 0 {
		return fmt.Sprintf("Long text was trimmed for channel length; full details are in %s.\n", home)
	}
	factWord := "facts"
	if dropped == 1 {
		factWord = "fact"
	}
	return fmt.Sprintf("Trimmed for channel length: dropped %d %s; full details are in %s.\n", dropped, factWord, home)
}

func optionConsequences(options []Option) []string {
	out := make([]string, len(options))
	for i, option := range options {
		out[i] = option.Consequence
	}
	return out
}

func trimQuestionParts(parts []string, budget int) []string {
	out := make([]string, len(parts))
	remaining := budget
	for i, part := range parts {
		share := 0
		if count := len(parts) - i; count > 0 {
			share = remaining / count
		}
		out[i] = trimQuestionPart(part, share)
		remaining -= len([]rune(out[i]))
	}
	return out
}

func trimQuestionPart(text string, budget int) string {
	runes := []rune(text)
	if len(runes) <= budget {
		return text
	}
	if budget <= 0 {
		return ""
	}
	return string(runes[:budget-1]) + "…"
}

func questionPartsRunes(parts []string) int {
	total := 0
	for _, part := range parts {
		total += len([]rune(part))
	}
	return total
}

func renderProposedBox(b goal.Budget) string {
	return fmt.Sprintf("%s, %d attempts, %d reserved minutes, %d active job, %d review rounds", b.ElapsedLimit, b.AttemptLimit, b.ReservedJobMinutesLimit, b.ActiveJobLimit, b.ReviewRoundLimit)
}

func Close(repo, id, because string, p Provider, d DestinationConfig) error {
	q, err := ReadQuestion(repo, id)
	if err != nil {
		return err
	}
	if q.State == "closed" {
		// Already closed (R-129-ui): nothing is posted or written again.
		return nil
	}
	if q.Thread != nil && p != nil {
		_, _ = p.Post(contextBackground{}, d, "closed: "+because, q.Thread)
	}
	q.State = "closed"
	if q.Answer != nil {
		q.Answer.Phase = "closed"
	}
	return writeJSON(questionPath(repo, id), q)
}
