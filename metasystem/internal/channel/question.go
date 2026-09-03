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
	Text   string     `json:"text"`
	UserID string     `json:"userID"`
	Ref    MessageRef `json:"ref"`
	At     time.Time  `json:"at"`
	Step   int64      `json:"step"`
	ULID   string     `json:"ulid"`
	Opid   string     `json:"opid"`
	Phase  string     `json:"phase"`
}
type Question struct {
	ID             string      `json:"id"`
	Goal           string      `json:"goal"`
	Kind           string      `json:"kind"`
	Machine        string      `json:"machine"`
	OpenedAt       time.Time   `json:"openedAt"`
	Facts          []string    `json:"facts"`
	Options        []Option    `json:"options"`
	Recommendation string      `json:"recommendation"`
	Wants          string      `json:"wants"`
	Thread         *MessageRef `json:"thread"`
	State          string      `json:"state"`
	Undelivered    int         `json:"undelivered"`
	Answer         *Answer     `json:"answer"`
	Rejected       []Rejection `json:"rejected"`
	FactsDigest    string      `json:"factsDigest"`
}

type AskRequest struct {
	Context                                context.Context
	RepoRoot, Goal, Kind, Machine, Lineage string
	Facts                                  []string
	Options                                []Option
	Recommendation, Wants                  string
	Provider                               Provider
	Destination                            DestinationConfig
	Now                                    time.Time
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
	err = json.Unmarshal(b, &q)
	return q, err
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
	q := Question{ID: id, Goal: r.Goal, Kind: r.Kind, Machine: r.Machine, OpenedAt: r.Now.UTC(), Facts: r.Facts, Options: r.Options, Recommendation: r.Recommendation, Wants: r.Wants, State: "open", FactsDigest: digest}
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
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s\n", strings.ReplaceAll(q.Goal, "-", " "), q.Kind)
	for _, f := range q.Facts {
		fmt.Fprintf(&b, "- %s\n", f)
	}
	for _, o := range q.Options {
		fmt.Fprintf(&b, "%s: %s\n", o.Label, o.Consequence)
	}
	fmt.Fprintf(&b, "Recommendation: %s\n", q.Recommendation)
	if q.Wants != "" {
		fmt.Fprintf(&b, "Reply in this thread with this token verbatim, followed by your code:\n%s", q.Wants)
	} else {
		b.WriteString("Reply in this thread with your answer followed by your code")
	}
	return b.String()
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
