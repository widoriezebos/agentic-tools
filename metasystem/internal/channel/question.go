package channel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	ID             string       `json:"id"`
	Goal           string       `json:"goal"`
	Kind           string       `json:"kind"`
	Machine        string       `json:"machine"`
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
}

type AskRequest struct {
	Context                                context.Context
	RepoRoot, Goal, Kind, Machine, Lineage string
	Facts                                  []string
	Options                                []Option
	Recommendation, Wants                  string
	Budget                                 *goal.Budget
	Provider                               Provider
	Destination                            DestinationConfig
	Now                                    time.Time
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".channel-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(b); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
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

func validateQuestionBudget(q Question) error {
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
	if r.Now.IsZero() {
		r.Now = time.Now().UTC()
	}
	if r.Goal == "" || r.Kind == "" || len(r.Facts) == 0 {
		return Question{}, fmt.Errorf("ask requires goal, kind, and a fact")
	}
	switch r.Kind {
	case "budget-above-norm", "fork", "reserved-decision", "stop", "other":
	default:
		return Question{}, fmt.Errorf("unknown question kind %q", r.Kind)
	}
	digest := factsDigest(r.Goal, r.Kind, r.Facts)
	existing, err := listQuestions(r.RepoRoot)
	if err != nil {
		return Question{}, err
	}
	for _, q := range existing {
		if q.State == "open" && q.Goal == r.Goal && q.Kind == r.Kind && q.FactsDigest == digest {
			return q, nil
		}
	}
	id, err := goal.NewOperationULID()
	if err != nil {
		return Question{}, err
	}
	q := Question{ID: id, Goal: r.Goal, Kind: r.Kind, Machine: r.Machine, OpenedAt: r.Now.UTC(), Facts: r.Facts, Options: r.Options, Recommendation: r.Recommendation, Wants: r.Wants, Budget: r.Budget, State: "open", FactsDigest: digest}
	if err = validateQuestionBudget(q); err != nil {
		return Question{}, err
	}
	if err = writeJSON(questionPath(r.RepoRoot, id), q); err != nil {
		return Question{}, err
	}
	if r.Provider != nil {
		postContext := r.Context
		if postContext == nil {
			postContext = context.Background()
		}
		ref, postErr := r.Provider.Post(postContext, r.Destination, renderQuestion(q), nil)
		if postErr == nil {
			q.Thread = &ref
		} else {
			q.Undelivered++
		}
		if err = writeJSON(questionPath(r.RepoRoot, id), q); err != nil {
			return q, err
		}
	}
	if r.Lineage != "" {
		ep, e := goal.ResolveEndpoint(r.RepoRoot)
		if e != nil {
			return q, e
		}
		ulid, e := goal.NewOperationULID()
		if e != nil {
			return q, e
		}
		published, e := goal.Asked(goal.VerbRequest{Endpoint: ep, Actor: goal.Actor{Machine: r.Machine, Lineage: r.Lineage}, Ulid: ulid, Now: r.Now}, r.Goal, id, r.Kind, r.Facts[0])
		if e != nil {
			return q, e
		}
		if published.Outcome != goal.OutcomeConfirmed {
			return q, fmt.Errorf("goal ask was not confirmed: %s", published.Detail)
		}
	}
	return q, nil
}

// contextBackground avoids making Ask's durable-write ordering dependent on a caller context.
type contextBackground struct{}

func (contextBackground) Deadline() (time.Time, bool) { return time.Time{}, false }
func (contextBackground) Done() <-chan struct{}       { return nil }
func (contextBackground) Err() error                  { return nil }
func (contextBackground) Value(any) any               { return nil }
func renderQuestion(q Question) string {
	tail := "Reply in this thread with your answer followed by your code"
	if q.Wants != "" {
		tail = "Reply in this thread with this token verbatim, followed by your code:\n" + q.Wants
	}

	full := renderQuestionParts(q, q.Facts, optionConsequences(q.Options), q.Recommendation, "", tail)
	if len([]rune(full)) <= questionMessageRuneLimit {
		return full
	}

	factCount := len(q.Facts)
	if factCount > questionFactLimit {
		factCount = questionFactLimit
	}
	noticeReserve := 0
	for dropped := 0; dropped <= len(q.Facts); dropped++ {
		if size := len([]rune(questionTrimNotice(q.Goal, dropped))); size > noticeReserve {
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
	notice := questionTrimNotice(q.Goal, len(q.Facts)-factCount)
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
	return fmt.Sprintf("%s — %s\n", strings.ReplaceAll(q.Goal, "-", " "), q.Kind)
}

func questionBudgetLine(q Question) string {
	if q.Budget != nil {
		return fmt.Sprintf("Proposed box: %s\n", renderProposedBox(*q.Budget))
	}
	return ""
}

func questionTrimNotice(goalID string, dropped int) string {
	if dropped == 0 {
		return fmt.Sprintf("Long text was trimmed for channel length; full details are in goal %s.\n", goalID)
	}
	factWord := "facts"
	if dropped == 1 {
		factWord = "fact"
	}
	return fmt.Sprintf("Trimmed for channel length: dropped %d %s; full details are in goal %s.\n", dropped, factWord, goalID)
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
	if q.Thread != nil && p != nil {
		_, _ = p.Post(contextBackground{}, d, "closed: "+because, q.Thread)
	}
	q.State = "closed"
	if q.Answer != nil {
		q.Answer.Phase = "closed"
	}
	return writeJSON(questionPath(repo, id), q)
}
