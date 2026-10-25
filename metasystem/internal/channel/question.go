package channel

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

type Option struct {
	Label       string `json:"label"`
	Consequence string `json:"consequence"`
	// Act is the public argv the option runs, without --by.
	Act []string `json:"act,omitempty"`
}

// Attempt is one executed choice of a contract question's option; Message
// is the inbox record that chose it.
type Attempt struct {
	At      time.Time `json:"at"`
	Label   string    `json:"label"`
	Outcome string    `json:"outcome"`
	Message string    `json:"message"`
}
type Rejection struct {
	Ref     MessageRef  `json:"ref"`
	Reason  string      `json:"reason"`
	At      time.Time   `json:"at"`
	Posted  bool        `json:"posted"`
	PostRef *MessageRef `json:"postRef"`
}
type Answer struct {
	Machine      string     `json:"machine,omitempty"`
	Lineage      string     `json:"lineage,omitempty"`
	Provider     string     `json:"provider,omitempty"`
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

// Decision identifies the machine state that needs a person.
type Decision struct{ Kind, Subject, State string }

type Question struct {
	// Text is the post, stored before its first attempt; every attempt posts it.
	Text     string           `json:"text,omitempty"`
	Plain    *Plain           `json:"plain,omitempty"`
	Recovery *RecoverySubject `json:"recovery,omitempty"`
	ID       string           `json:"id"`
	Goal     string           `json:"goal"`
	// About names what a question without a goal is about: lane or machine.
	About   string `json:"about,omitempty"`
	Kind    string `json:"kind"`
	Machine string `json:"machine"`
	// Lineage is the asking session's owner lineage; with Machine it names
	// the session whose turn may end while the question is open.
	Lineage        string            `json:"lineage,omitempty"`
	OpenedAt       time.Time         `json:"openedAt"`
	Facts          []string          `json:"facts"`
	Options        []Option          `json:"options"`
	Recommendation string            `json:"recommendation"`
	Wants          string            `json:"wants"`
	Budget         *goal.Budget      `json:"budget,omitempty"`
	Thread         *MessageRef       `json:"thread"`
	State          string            `json:"state"`
	ClosedBecause  string            `json:"closedBecause,omitempty"`
	Undelivered    int               `json:"undelivered"`
	Answer         *Answer           `json:"answer"`
	Rejected       []Rejection       `json:"rejected"`
	FactsDigest    string            `json:"factsDigest"`
	LedgerCursor   string            `json:"ledgerCursor,omitempty"`
	UnitStop       *UnitStopQuestion `json:"unitStop,omitempty"`
	ProcessAct     string            `json:"processAct,omitempty"`
	Decision       *Decision         `json:"decision,omitempty"`
	RetiredAt      time.Time         `json:"retiredAt,omitzero"`
	Contract       bool              `json:"contract,omitempty"`
	Attempts       []Attempt         `json:"attempts,omitempty"`
}

type AskRequest struct {
	Plain                                         *Plain
	Decision                                      *Decision
	Recovery                                      *RecoverySubject
	ProcessAct                                    string
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
	UnitStop                                      *UnitStopQuestion
	Contract                                      bool
}

// A post's first line is bounded for a phone.
const plainDecisionRunes = 400

// RecoverySubject binds a request to the host registration and the exact failed seat launch.
type RecoverySubject struct {
	Host, Seat, Launch, Provider, Episode, Due string
}

// Plain is a question's words for a person (design D3): what he decides,
// the act that goes on, and what happens if he does nothing.
type Plain struct {
	Decision  string `json:"decision"`
	Act       string `json:"act,omitempty"`
	IfNothing string `json:"ifNothing,omitempty"`
}

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
	// The folder is listed rather than globbed: a glob reads a folder it
	// can't list as one with no questions.
	dir := filepath.Join(channelRoot(repo), "questions")
	listed, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, []string{dir + ": " + err.Error()}
	}
	var paths []string
	for _, entry := range listed {
		if strings.HasSuffix(entry.Name(), ".json") {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
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
	if err := validateContract(r); err != nil {
		return Question{}, false, err
	}
	if r.UnitStop != nil {
		return askUnitStop(r)
	}
	return askOrFind(r)
}

func askOrFind(r AskRequest) (Question, bool, error) {
	if r.Decision != nil {
		if r.About != "machine" || r.Decision.Kind == "" || r.Decision.Subject == "" || r.Decision.State == "" {
			return Question{}, false, fmt.Errorf("a decision requires a machine question and a kind, subject, and state")
		}
	}
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
	var existing []Question
	var err error
	if r.About == "lane" && r.Lineage == "landing-agent" {
		existing, _ = WalkQuestions(r.RepoRoot)
	} else {
		existing, err = listQuestions(r.RepoRoot)
	}
	if err != nil {
		return Question{}, false, err
	}
	for _, q := range existing {
		if r.Decision != nil && q.Decision != nil && *r.Decision == *q.Decision && q.RetiredAt.IsZero() {
			return q, true, nil
		}
		if r.Recovery != nil && q.Recovery != nil && *r.Recovery == *q.Recovery {
			return q, true, nil
		}
		if r.UnitStop != nil && q.UnitStop != nil && q.Goal == r.Goal && q.UnitStop.sameKey(*r.UnitStop) {
			return q, true, nil
		}
		if r.Decision == nil && q.Decision == nil && r.UnitStop == nil && q.UnitStop == nil && q.State == "open" && q.Goal == r.Goal && q.About == r.About && q.Kind == r.Kind && q.FactsDigest == digest && sameAsk(q, r) {
			return q, true, nil
		}
	}
	id, err := goal.NewOperationULID()
	if err != nil {
		return Question{}, false, err
	}
	q := Question{Plain: r.Plain, Decision: r.Decision, Recovery: r.Recovery, ProcessAct: r.ProcessAct, ID: id, Goal: r.Goal, About: r.About, Kind: r.Kind, Machine: r.Machine, Lineage: r.Lineage, OpenedAt: r.Now.UTC(), Facts: r.Facts, Options: r.Options, Recommendation: r.Recommendation, Wants: r.Wants, Budget: r.Budget, State: "open", FactsDigest: digest, LedgerCursor: r.LedgerCursor, UnitStop: r.UnitStop, Contract: r.Contract}
	if err = validateQuestionBudget(q); err != nil {
		return Question{}, false, err
	}
	q.Text = renderQuestion(r.RepoRoot, q)
	if err = writeJSON(questionPath(r.RepoRoot, id), q); err != nil {
		return Question{}, false, err
	}
	if r.Provider != nil {
		postContext := r.Context
		if postContext == nil {
			postContext = context.Background()
		}
		ref, postErr := r.Provider.Post(postContext, r.Destination, q.Text, nil)
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
	if r.Lineage != "" && r.Goal != "" && r.ProcessAct == "" {
		ep, e := project.PublicationEndpoint(r.RepoRoot)
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
	if q.ProcessAct != r.ProcessAct || q.Recommendation != r.Recommendation || q.Wants != r.Wants || !reflect.DeepEqual(q.Budget, r.Budget) {
		return false
	}
	if len(q.Options) != len(r.Options) {
		return false
	}
	for index := range q.Options {
		if !reflect.DeepEqual(q.Options[index], r.Options[index]) {
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
	if q.Contract {
		return answerLine(q, codeOff)
	}
	if q.Recovery != nil {
		return "Run " + q.Wants + "; only a matching successful seat recovery closes this question. A text answer leaves it open."
	}
	if q.ProcessAct != "" {
		return "Run " + q.Wants + "; applying this exact process act closes the question."
	}
	if q.UnitStop != nil {
		return "Run " + q.UnitStop.Needs + "; a successful matching act or unit closure closes this question. A text answer leaves it open."
	}
	if command := LaneStopCommand(q); command != "" {
		return q.Facts[0] + "; a later lane record closes this question."
	}
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

// renderPlain keeps the decision, act and consequences together in the post.
func renderPlain(q Question) string {
	var plain Plain
	if q.Plain != nil {
		plain = *q.Plain
	}
	subject := QuestionSubject(q)
	if q.Goal != "" {
		subject = "Goal " + q.Goal
	}
	lines := []string{plainSentence(trimQuestionPart(cmp.Or(plain.Decision, subject+" needs your answer"), plainDecisionRunes), true)}
	switch {
	case plain.Act != "":
		lines = append(lines, "To go on: "+plain.Act)
	case ActCommand(q) != "":
		lines = append(lines, "To go on: "+ActCommand(q))
	case q.ProcessAct != "":
		lines = append(lines, "To go on: "+q.Wants)
	default:
		lines = append(lines, "To answer: metasystem question answer "+q.ID)
	}
	lines = append(lines, "If you do nothing: "+plainSentence(cmp.Or(plain.IfNothing, "this question stays open"), false))
	for _, option := range q.Options {
		lines = append(lines, "- "+option.Label+": "+plainSentence(option.Consequence, false))
	}
	return strings.Join(lines, "\n")
}

// unplainWords are what a person cannot read on his phone: a commit or run
// id, a local path, a process number, a seat generation.
var unplainWords = regexp.MustCompile(`[0-9a-fA-F]{10}|/Users/|\bpid\b|\bgeneration [0-9]+`)

// validateContract refuses a contract question a person cannot read on one
// screen, saying what to shorten or remove.
func validateContract(r AskRequest) error {
	if !r.Contract {
		return nil
	}
	if r.Plain == nil || strings.TrimSpace(r.Plain.Decision) == "" || strings.TrimSpace(r.Plain.IfNothing) == "" {
		return fmt.Errorf("a question for a person says what he decides and what happens if he does nothing")
	}
	if len([]rune(r.Plain.Decision)) > 280 {
		return fmt.Errorf("shorten the decision to 280 characters")
	}
	words, labels := []string{r.Plain.Decision, r.Plain.IfNothing}, map[string]bool{}
	for _, option := range r.Options {
		switch {
		case strings.TrimSpace(option.Consequence) == "":
			return fmt.Errorf("say what option %q does, what it costs and how it is undone", option.Label)
		case len([]rune(option.Consequence)) > 200:
			return fmt.Errorf("shorten what option %q does to 200 characters", option.Label)
		case labels[strings.ToLower(option.Label)]:
			return fmt.Errorf("offer option %q once", option.Label)
		}
		labels[strings.ToLower(option.Label)] = true
		words = append(words, option.Consequence)
	}
	if len(r.Options) == 0 || r.Recommendation != "" && !slices.ContainsFunc(r.Options, func(o Option) bool { return o.Label == r.Recommendation }) {
		return fmt.Errorf("offer at least one option and recommend one of them by its label")
	}
	for _, text := range words {
		if found := unplainWords.FindString(text); found != "" {
			return fmt.Errorf("remove %q from the question's words: a person reads no ids, paths or process numbers", found)
		}
	}
	return nil
}

// renderQuestion is the post: a contract question in the contract's order,
// any other as it always was.
func renderQuestion(repo string, q Question) string {
	if !q.Contract {
		return renderPlain(q)
	}
	lines := []string{plainSentence(q.Plain.Decision, true), ""}
	for _, option := range shownOptions(q) {
		line := option.Label + " — " + plainSentence(option.Consequence, false)
		if option.Label == q.Recommendation {
			line = "★ " + line
		}
		lines = append(lines, line)
	}
	return strings.Join(append(lines, "", "If you do nothing: "+plainSentence(q.Plain.IfNothing, false), answerLine(q, answerCodeOffOrOn(repo))), "\n")
}

// shownOptions are the options in the order shown: the recommended first.
func shownOptions(q Question) []Option {
	var first, rest []Option
	for _, option := range q.Options {
		if option.Label == q.Recommendation {
			first = append(first, option)
		} else {
			rest = append(rest, option)
		}
	}
	return append(first, rest...)
}

// answerLine is how a contract question is answered, generated from its
// labels in the order shown.
func answerLine(q Question, codeOff bool) string {
	var labels []string
	for _, option := range shownOptions(q) {
		labels = append(labels, option.Label)
	}
	if codeOff {
		return "Reply: " + strings.Join(labels, " / ")
	}
	return "Reply with the word and your code: " + strings.Join(labels, " / ")
}

func plainSentence(s string, capital bool) string {
	runes := []rune(strings.Join(strings.Fields(s), " "))
	if len(runes) == 0 {
		return ""
	}
	if capital {
		runes[0] = unicode.ToUpper(runes[0])
	}
	if last := runes[len(runes)-1]; last != '.' && last != '?' && last != '!' {
		runes = append(runes, '.')
	}
	return string(runes)
}

func ensureText(repo string, q *Question) error {
	if q.Text == "" {
		q.Text = renderQuestion(repo, *q)
		return writeJSON(questionPath(repo, q.ID), q)
	}
	return nil
}

// LaneStopCommand recognizes the lane's own stop question in its existing facts.
func LaneStopCommand(q Question) string {
	if q.About == "lane" && q.Goal == "" && q.Lineage == "landing-agent" && len(q.Facts) == 3 && (strings.HasPrefix(q.Facts[1], "lane stop: ") || strings.HasPrefix(q.Facts[1], "lane policy: ")) {
		var record struct {
			Required []string `json:"required"`
		}
		if _, data, ok := strings.Cut(q.Facts[1], ": "); ok && json.Unmarshal([]byte(data), &record) == nil && len(record.Required) > 0 {
			return strings.Join(record.Required, " ")
		}
		return q.Facts[0]
	}
	return ""
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

func ProposedBox(b goal.Budget) string {
	return fmt.Sprintf("%s, %d attempts, %d reserved minutes, %d active job, %d review rounds", b.ElapsedLimit, b.AttemptLimit, b.ReservedJobMinutesLimit, b.ActiveJobLimit, b.ReviewRoundLimit)
}

func Close(repo, id, because string, p Provider, d DestinationConfig, successfulAct ...string) error {
	q, err := ReadQuestion(repo, id)
	if err != nil {
		return err
	}
	if q.State == "closed" {
		// Already closed (R-129-ui): nothing is posted or written again.
		return nil
	}
	if q.Thread != nil && p != nil {
		_, _ = p.Post(contextBackground{}, d, "Closed: "+plainSentence(because, false), q.Thread)
	}
	q.State = "closed"
	if q.UnitStop != nil && len(successfulAct) > 0 {
		q.UnitStop.ClosedBy = successfulAct[0]
	}
	if LaneStopCommand(q) != "" || q.Recovery != nil {
		q.ClosedBecause = because
		if q.Recovery != nil {
			q.Facts = append(q.Facts, because)
		}
	}
	if q.Answer != nil && q.Answer.Phase != "acted" {
		q.Answer.Phase = "closed"
	}
	return writeJSON(questionPath(repo, id), q)
}

// AnswerSummary describes the reply and any act that still belongs to the person.
func AnswerSummary(q Question) string {
	if q.Answer == nil {
		return ""
	}
	verb := "answered"
	act := ActCommand(q)
	if act != "" && q.State != "closed" {
		verb = "replied"
	}
	text := fmt.Sprintf("%s '%s' at %s", verb, q.Answer.Text, q.Answer.At.Local().Format("15:04"))
	if verb == "replied" {
		if LaneStopCommand(q) != "" {
			act = q.Facts[0]
		}
		text += "; the act is still needed: " + act
	}
	return text
}
