package plain

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
)

type Clock struct {
	HandInAt     map[string]string `json:"hand_in_at"`
	MergeMinutes *float64          `json:"merge_minutes"`
	PushMinutes  *float64          `json:"push_minutes"`
	MergeAt      string            `json:"merge_at"`
	PushAt       string            `json:"push_at"`
	TotalMinutes *float64          `json:"total_minutes"`
	Proofs       []ProofClock      `json:"proofs"`
	FixRounds    []FixClock        `json:"fix_rounds"`
	Unknown      []string          `json:"unknown,omitempty"`
}

type ProofClock struct {
	Result
	Shards  *int             `json:"shards"`
	Longest *PackageTiming   `json:"longest,omitempty"`
	Steps   map[string]int64 `json:"steps_ms"`
}

type FixClock struct {
	Goal       string   `json:"goal"`
	ReturnedAt string   `json:"returned_at"`
	HandInAt   string   `json:"hand_in_at"`
	Minutes    *float64 `json:"minutes"`
}

type PackageTiming struct {
	Unit   string `json:"unit"`
	Shard  int    `json:"shard"`
	Status string `json:"status"`
	MS     int64  `json:"ms"`
}

func elapsedMinutes(start, end string) *float64 {
	a, ae := time.Parse(time.RFC3339, start)
	b, be := time.Parse(time.RFC3339, end)
	if ae != nil || be != nil || b.Before(a) {
		return nil
	}
	minutes := b.Sub(a).Minutes()
	return &minutes
}

func timedResult(r Result, since string, now time.Time) Result {
	r.StartedAt, r.At = since, now.UTC().Format(time.RFC3339)
	r.Minutes = elapsedMinutes(r.StartedAt, r.At)
	return r
}

func PushedClock(install string, member GoalSHA) *Clock {
	pushes, _ := readLines[Pushed](pushesPath(install))
	for i := len(pushes) - 1; i >= 0; i-- {
		if slices.Contains(pushes[i].BatchMembers, member) {
			return pushes[i].Clock
		}
	}
	return nil
}

// An episode retains hand-ins and returns after each member's last landing.
func landingClock(install, checkout string, batch *Batch, now time.Time, seams ProveSeams) *Clock {
	c := &Clock{HandInAt: map[string]string{}, PushAt: now.UTC().Format(time.RFC3339), Proofs: []ProofClock{}, FixRounds: []FixClock{}}
	members := []GoalSHA{}
	if batch != nil {
		members = batch.Members
	} else {
		members, _ = goalsInCommit(install, checkout, "HEAD", seams.git)
	}
	entries, queueDamage, queueErr := countedLines[Line](queuePath(install))
	pushes, pushDamage, pushErr := countedLines[Pushed](pushesPath(install))
	contains := func(sha string) (bool, error) { return checkoutGit(checkout, seams).contains("origin/main", sha) }
	history, priorErr := landedBeforeAgain(entriesOf(entries), time.Time{}, contains)
	history, landingErr := Landed(history, contains)
	history, timesErr := LandingTimes(history, pushes, time.Time{}, checkoutGit(checkout, seams).brought)
	if queueErr != nil || queueDamage > 0 || pushErr != nil || pushDamage > 0 || priorErr != nil || landingErr != nil || timesErr != nil {
		c.Unknown = append(c.Unknown, "hand-in history")
	}
	first := ""
	for _, member := range members {
		last := ""
		for _, e := range history {
			if e.Goal == member.Goal && !e.Records && e.LandedAt > last {
				last = e.LandedAt
			}
		}
		pendingReturn := ""
		for _, e := range history {
			if e.Goal != member.Goal || e.Records && e.SHA != member.SHA || elapsedMinutes(last, e.At) == nil && last != "" || e.At == last {
				continue
			}
			if c.HandInAt[e.Goal] == "" {
				c.HandInAt[e.Goal] = e.At
				if first == "" || e.At < first {
					first = e.At
				}
			}
			if pendingReturn != "" {
				c.FixRounds = append(c.FixRounds, FixClock{e.Goal, pendingReturn, e.At, elapsedMinutes(pendingReturn, e.At)})
			}
			pendingReturn = e.ReturnedAt
		}
	}
	c.MergeAt, _ = seams.git(checkout, "show", "-s", "--format=%cI", "HEAD")
	c.TotalMinutes, c.MergeMinutes = elapsedMinutes(first, c.PushAt), elapsedMinutes(first, c.MergeAt)
	if len(c.Unknown) > 0 {
		c.TotalMinutes, c.MergeMinutes = nil, nil
	}
	gates, gateDamage, gateErr := countedLines[Result](gatesPath(install))
	results, damaged, err := countedLines[Result](resultsPath(install))
	results = append(gates, results...)
	if err != nil || damaged > 0 || gateErr != nil || gateDamage > 0 {
		c.Unknown = append(c.Unknown, "proof history")
	}
	seen := map[string]bool{}
	for _, r := range slices.Backward(results) {
		if seen[r.Attempt] || r.Trunk || !subsetGoals(r.Goals, members) || strings.HasPrefix(r.Reason, "inherits green from tree ") {
			continue
		}
		if r.StartedAt != "" && elapsedMinutes(first, r.StartedAt) == nil || elapsedMinutes(first, r.At) == nil || elapsedMinutes(r.At, c.PushAt) == nil {
			continue
		}
		seen[r.Attempt] = true
		p := ProofClock{Result: r}
		var record scopeRecord
		if r.Scope == "gate" {
			c.Proofs = append(c.Proofs, p)
			continue
		}
		data, readErr := os.ReadFile(scopePath(install, r.Attempt))
		if readErr == nil && json.Unmarshal(data, &record) == nil && record.Packages != nil {
			shards := len(record.Packages)
			p.Shards = &shards
			for _, pkg := range record.Packages {
				if p.Longest == nil || pkg.MS > p.Longest.MS {
					p.Longest = &pkg
				}
			}
			p.Steps = record.Durations
		} else {
			c.Unknown = append(c.Unknown, "scope of "+r.Attempt)
		}
		c.Proofs = append(c.Proofs, p)
	}
	slices.SortStableFunc(c.Proofs, func(a, b ProofClock) int { return strings.Compare(a.StartedAt, b.StartedAt) })
	if len(c.Proofs) > 0 {
		c.PushMinutes = elapsedMinutes(c.Proofs[len(c.Proofs)-1].At, c.PushAt)
	}
	return c
}

func clockMinutes(minutes *float64) string {
	if minutes == nil {
		return "unknown"
	}
	return fmt.Sprintf("%.1f", *minutes)
}

func (c *Clock) Words() string {
	if c == nil {
		return "landing duration unknown"
	}
	parts := []string{"hand-in to merge " + clockMinutes(c.MergeMinutes)}
	cheap := "cheap gate not run"
	for _, p := range c.Proofs {
		if p.Scope == "gate" {
			cheap = "cheap gate " + clockMinutes(p.Minutes) + " min"
		}
	}
	parts = append(parts, cheap)
	for _, p := range c.Proofs {
		if p.Scope == "gate" {
			continue
		}
		words := p.Scope + " proof " + clockMinutes(p.Minutes) + " (static " + clockStep(p.Steps, "fast-static-build") + ", shards unknown"
		if p.Shards != nil {
			words = strings.TrimSuffix(words, "shards unknown") + fmt.Sprintf("%d shards", *p.Shards)
		}
		if p.Longest != nil {
			words += fmt.Sprintf(", longest %s shard %d at %.1f", p.Longest.Unit, p.Longest.Shard, float64(p.Longest.MS)/60000)
		}
		parts = append(parts, words+")")
	}
	for _, f := range c.FixRounds {
		parts = append(parts, "fix round "+clockMinutes(f.Minutes))
	}
	parts = append(parts, "push "+clockMinutes(c.PushMinutes))
	if len(c.Unknown) > 0 {
		parts = append(parts, "unknown: "+strings.Join(c.Unknown, ", "))
	}
	return "landing took " + clockMinutes(c.TotalMinutes) + " min: " + strings.Join(parts, ", ")
}

func clockStep(steps map[string]int64, name string) string {
	ms, known := steps[name]
	if !known {
		return "unknown"
	}
	return fmt.Sprintf("%.1f", float64(ms)/60000)
}
