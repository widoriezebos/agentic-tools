package channel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

type ReportConfig struct {
	RepoRoot, Machine string
	Now, WindowStart  time.Time
	Undelivered       int
	OldestUndelivered time.Time
}

func ComposeReport(c ReportConfig) (string, error) {
	text, _, err := ComposeStatusReport(c)
	return text, err
}

// ComposeStatusReport returns the rendered post and the goal named by its
// execution-approval line, when that line fits in the post.
func ComposeStatusReport(c ReportConfig) (string, string, error) {
	if c.Now.IsZero() {
		c.Now = time.Now().UTC()
	}
	if c.WindowStart.IsZero() {
		c.WindowStart = LoadStatusState(c.RepoRoot).LastPost
		if c.WindowStart.IsZero() {
			c.WindowStart = c.Now.Add(-4 * time.Hour)
		}
	}
	needs, next := []string{}, []string{}
	var delivered []string
	approvalGoal := ""
	features := map[string]string{}
	questions, _ := listQuestions(c.RepoRoot)
	questionGoals := map[string]bool{}
	for _, q := range questions {
		if q.State != "open" {
			continue
		}
		questionGoals[q.Goal] = true
		needs = append(needs, "Needs you: "+featureName(q.Goal)+" — "+questionRequest(q))
	}
	ep, err := goal.ResolveEndpoint(c.RepoRoot)
	if err == nil {
		if p, e := goal.Project(ep, false, c.Now); e == nil {
			for id := range p.Tree.Live {
				features[id] = featureName(id)
			}
			for id := range p.Tree.Done {
				features[id] = featureName(id)
			}
			frontier := goal.Next(p, c.Machine)
			if id := markedNextGoal(p, c.Machine); id != "" && !questionGoals[id] {
				needs = append(needs, approvalRequestLine(id))
				approvalGoal = id
			}
			for _, id := range frontier.Ready {
				next = append(next, "Next up: "+featureName(id))
				if len(next) == 2 {
					break
				}
			}
		}
	}
	sort.Strings(needs)
	delivered = landingLines(c, features)

	lineLimit := 12
	if c.Undelivered > 0 {
		lineLimit--
	}
	lines := []string{fmt.Sprintf("%s status %sZ", c.Machine, c.Now.UTC().Format("2006-01-02 15:04"))}
	for _, part := range [][]string{needs, delivered, next} {
		for _, line := range part {
			if len(lines) == lineLimit {
				break
			}
			lines = append(lines, line)
		}
	}
	if c.Undelivered > 0 {
		age := int(c.Now.Sub(c.OldestUndelivered).Minutes())
		if age < 0 {
			age = 0
		}
		lines = append(lines, fmt.Sprintf("Undelivered: %d channel messages, oldest %d min", c.Undelivered, age))
	}
	text := strings.Join(lines, "\n")
	if approvalGoal != "" && !strings.Contains(text, approvalRequestLine(approvalGoal)) {
		approvalGoal = ""
	}
	return text, approvalGoal, nil
}

func markedNextGoal(p goal.Projection, machine string) string {
	marked := ""
	for id, f := range p.Tree.Live {
		if f.State != goal.StateQueued || f.Pinned != machine || !goal.MatchesLabels(f.Labels, []string{"next"}) {
			continue
		}
		if marked != "" {
			return ""
		}
		marked = id
	}
	return marked
}

func approvalRequestLine(id string) string {
	return "Needs you: " + featureName(id) + " — Reply in this thread with this token verbatim, followed by your code: start " + id
}

func featureName(id string) string {
	return strings.ReplaceAll(id, "-", " ")
}

func questionRequest(q Question) string {
	switch q.Kind {
	case "budget-above-norm":
		return "approve the requested budget raise."
	case "fork":
		return "decide whether to fork it."
	case "stop":
		return "decide whether to resume it."
	}
	if len(q.Facts) > 0 {
		return oneSentence(q.Facts[0])
	}
	return "give your decision."
}

func oneSentence(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if i := strings.IndexAny(s, ".!?"); i >= 0 {
		s = s[:i+1]
	}
	if s == "" {
		return "give your decision."
	}
	if !strings.ContainsAny(s[len(s)-1:], ".!?") {
		s += "."
	}
	return s
}

func landingLines(c ReportConfig, features map[string]string) []string {
	cmd := exec.Command("git", "-C", c.RepoRoot, "log", "origin/main", "--since="+c.WindowStart.UTC().Format(time.RFC3339), "--format=%s%x00%(trailers:key=Goal-Item,valueonly)")
	cmd.Env = reportGitEnv()
	b, err := cmd.Output()
	if err != nil {
		return nil
	}
	subjects := map[string]string{}
	for _, row := range strings.Split(string(b), "\n") {
		parts := strings.Split(row, "\x00")
		if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
			id := strings.TrimSpace(parts[1])
			if _, ok := features[id]; ok {
				if subjects[id] == "" {
					subjects[id] = parts[0]
				}
			}
		}
	}
	ids := make([]string, 0, len(subjects))
	for id := range subjects {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, fmt.Sprintf("Delivered: %s — %s", features[id], subjects[id]))
	}
	return out
}

func reportGitEnv() []string {
	drop := map[string]bool{
		"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_COMMON_DIR": true, "GIT_INDEX_FILE": true,
		"GIT_CEILING_DIRECTORIES": true, "GIT_OBJECT_DIRECTORY": true, "GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
		"GIT_CONFIG": true, "GIT_CONFIG_PARAMETERS": true, "GIT_CONFIG_COUNT": true, "GIT_CONFIG_GLOBAL": true,
		"GIT_CONFIG_SYSTEM": true, "GIT_CONFIG_NOSYSTEM": true, "GIT_GRAFT_FILE": true, "GIT_SHALLOW_FILE": true,
		"GIT_REPLACE_REF_BASE": true,
	}
	out := []string{}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !drop[name] && !strings.HasPrefix(name, "GIT_CONFIG_KEY_") && !strings.HasPrefix(name, "GIT_CONFIG_VALUE_") {
			out = append(out, entry)
		}
	}
	return out
}

func Digest(text string) string {
	first, body, hasBody := strings.Cut(text, "\n")
	if strings.Contains(first, " status ") && strings.HasSuffix(first, "Z") {
		text = ""
		if hasBody {
			text = body
		}
	}
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

type StatusState struct {
	LastPost      time.Time  `json:"lastPost"`
	ContentDigest string     `json:"contentDigest"`
	Ref           MessageRef `json:"ref"`
	GoalID        string     `json:"goalId,omitempty"`
}

func LoadStatusState(repo string) StatusState {
	var s StatusState
	b, err := os.ReadFile(filepath.Join(channelRoot(repo), "status.json"))
	if err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}
func SaveStatusState(repo string, s StatusState) error {
	return writeJSON(filepath.Join(channelRoot(repo), "status.json"), s)
}

func ShouldPost(s StatusState, now time.Time, interval time.Duration, text string, force bool) bool {
	return force || (now.Sub(s.LastPost) >= interval && s.ContentDigest != Digest(text))
}
