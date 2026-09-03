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
	if c.Now.IsZero() {
		c.Now = time.Now().UTC()
	}
	if c.WindowStart.IsZero() {
		c.WindowStart = c.Now.Add(-4 * time.Hour)
	}
	lines := []string{fmt.Sprintf("%s status %sZ", c.Machine, c.Now.UTC().Format("2006-01-02 15:04"))}
	under, planned := []string{}, []string{}
	running := runningJobs(c)
	landingFeatures := map[string]string{}
	ep, err := goal.ResolveEndpoint(c.RepoRoot)
	if err == nil {
		if p, e := goal.Project(ep, false, c.Now); e == nil {
			all := make([]*goal.GoalFile, 0, len(p.Tree.Live)+len(p.Tree.Done))
			for _, f := range p.Tree.Live {
				all = append(all, f)
			}
			for _, f := range p.Tree.Done {
				all = append(all, f)
			}
			for _, f := range all {
				claimed := f.Claimed != nil && f.Claimed.Machine == c.Machine
				for _, h := range f.History {
					if strings.HasPrefix(h.Actor, c.Machine+"+") && (h.Verb == "claim" || h.Verb == "steal") {
						claimed = true
					}
				}
				if claimed {
					landingFeatures[f.Id] = strings.ReplaceAll(f.Id, "-", " ") + " — " + firstSentence(f.Intent, 120)
				}
			}
			for id, f := range p.Tree.Live {
				feature := strings.ReplaceAll(id, "-", " ") + " — " + firstSentence(f.Intent, 120)
				if f.State == goal.StateClaimed && f.Claimed != nil && f.Claimed.Machine == c.Machine {
					line := "Under way: " + feature + "; " + firstSentence(f.NextStep, 120)
					if job := running[id]; job != "" {
						line += "; " + job
					}
					under = append(under, line)
				} else if f.State == goal.StateQueued && (f.Pinned == "" || f.Pinned == c.Machine) {
					readiness := "ready"
					if f.Budget == nil {
						readiness = "needs budget"
					}
					if len(f.Blocked) > 0 {
						readiness = "blocked by " + strings.ReplaceAll(f.Blocked[0], "-", " ")
					}
					planned = append(planned, "Planned: "+feature+" (queued, "+readiness+")")
				}
			}
			sort.Strings(under)
			sort.Strings(planned)
		}
	}
	landed := landingLines(c, landingFeatures)
	lines = append(lines, capLines(landed, 12)...)
	lines = append(lines, capLines(under, 12)...)
	lines = append(lines, capLines(planned, 12)...)
	if spend := spendLine(c); spend != "" {
		lines = append(lines, spend)
	}
	if c.Undelivered > 0 {
		age := int(c.Now.Sub(c.OldestUndelivered).Minutes())
		if age < 0 {
			age = 0
		}
		lines = append(lines, fmt.Sprintf("Undelivered: %d channel messages, oldest %d min", c.Undelivered, age))
	}
	text := strings.Join(lines, "\n")
	if len(text) > 3500 {
		text = text[:3488] + "\n(+more)"
	}
	return text, nil
}

func runningJobs(c ReportConfig) map[string]string {
	paths, _ := filepath.Glob(filepath.Join(c.RepoRoot, "artifacts", "agents", "jobs", "*.json"))
	out := map[string]string{}
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var v map[string]any
		if json.Unmarshal(b, &v) != nil || v["status"] != "running" {
			continue
		}
		goalID, _ := v["goalId"].(string)
		role, _ := v["role"].(string)
		started, _ := v["startedAt"].(string)
		at, err := time.Parse(time.RFC3339, started)
		if goalID == "" || role == "" || err != nil {
			continue
		}
		minutes := int(c.Now.Sub(at).Minutes())
		if minutes < 0 {
			minutes = 0
		}
		out[goalID] = fmt.Sprintf("job %s running %d min", role, minutes)
	}
	return out
}
func firstSentence(s string, n int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, ".\n"); i >= 0 {
		s = s[:i+1]
	}
	if len(s) > n {
		s = s[:n]
	}
	return s
}
func capLines(lines []string, n int) []string {
	if len(lines) <= n {
		return lines
	}
	extra := len(lines) - n
	return append(lines[:n], fmt.Sprintf("(+%d more)", extra))
}
func landingLines(c ReportConfig, features map[string]string) []string {
	cmd := exec.Command("git", "-C", c.RepoRoot, "log", "origin/main", "--since="+c.WindowStart.UTC().Format(time.RFC3339), "--format=%s%x00%(trailers:key=Goal-Item,valueonly)")
	b, err := cmd.Output()
	if err != nil {
		return nil
	}
	counts := map[string]int{}
	subjects := map[string]string{}
	for _, row := range strings.Split(string(b), "\n") {
		parts := strings.Split(row, "\x00")
		if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
			id := strings.TrimSpace(parts[1])
			if _, ok := features[id]; ok {
				counts[id]++
				if subjects[id] == "" {
					subjects[id] = parts[0]
				}
			}
		}
	}
	ids := make([]string, 0, len(counts))
	for id := range counts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, fmt.Sprintf("Landed since %s: %s — %s (%d landings)", c.WindowStart.UTC().Format(time.RFC3339), features[id], subjects[id], counts[id]))
	}
	return out
}
func spendLine(c ReportConfig) string {
	paths, _ := filepath.Glob(filepath.Join(c.RepoRoot, "artifacts", "agents", "jobs", "*.json"))
	jobs := 0
	units := map[string]float64{}
	for _, p := range paths {
		b, e := os.ReadFile(p)
		if e != nil {
			continue
		}
		var v map[string]any
		if json.Unmarshal(b, &v) != nil {
			continue
		}
		started, _ := v["startedAt"].(string)
		t, _ := time.Parse(time.RFC3339, started)
		if !sameUTCDate(t, c.Now) {
			continue
		}
		jobs++
		runtime, _ := v["runtime"].(string)
		u, _ := v["usage"].(map[string]any)
		for _, key := range []string{"inputTokens", "outputTokens", "reasoningTokens", "providerUnits"} {
			if n, ok := u[key].(float64); ok {
				units[runtime] += n
			}
		}
	}
	if jobs == 0 {
		return ""
	}
	keys := make([]string, 0, len(units))
	for k := range units {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := []string{}
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %.0f units", k, units[k]))
	}
	return fmt.Sprintf("Spend today: %d jobs; %s", jobs, strings.Join(parts, "; "))
}
func sameUTCDate(a, b time.Time) bool {
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}
func Digest(text string) string { h := sha256.Sum256([]byte(text)); return hex.EncodeToString(h[:]) }

type StatusState struct {
	LastPost      time.Time  `json:"lastPost"`
	ContentDigest string     `json:"contentDigest"`
	Ref           MessageRef `json:"ref"`
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
