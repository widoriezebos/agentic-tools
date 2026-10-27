package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	landingstate "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

type stewardReport struct {
	At      time.Time `json:"at"`
	Summary string    `json:"summary"`
}

func readStewardReport(install string) (r stewardReport, err error) {
	data, err := os.ReadFile(filepath.Join(Dir(install), "steward-report.json"))
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err == nil {
		err = json.Unmarshal(data, &r)
		if err == nil && (r.At.IsZero() || r.Summary == "") {
			err = errors.New("the lane report has no time or summary")
		}
	}
	return r, err
}

// StewardReport publishes evidence since the preceding report under the lane lock.
func StewardReport(install, checkout string, status Status, acts []Intervention, now time.Time, seams ProveSeams, store launch.Store) (summary string, err error) {
	err = withLock(install, func() error {
		previous, err := readStewardReport(install)
		if err != nil {
			return err
		}
		summary = previous.Summary
		if !previous.At.IsZero() && now.Sub(previous.At) < time.Hour {
			return nil
		}
		from := now.Add(-time.Hour)
		if !previous.At.IsZero() {
			from = previous.At
		}
		inHour := func(stamp string) bool {
			at, err := time.Parse(time.RFC3339Nano, stamp)
			return err == nil && at.After(from) && !at.After(now)
		}
		var out strings.Builder
		fmt.Fprintf(&out, "## Hour to %s\n\n### Quality\n", now.Local().Format("2006-01-02 15:04 MST"))
		reds, causes := 0, map[string]int{}
		for _, path := range []string{resultsPath(install), gatesPath(install)} {
			results, err := readLines[Result](path)
			if err != nil {
				return err
			}
			last := map[string]int{}
			for i, r := range results {
				if r.Attempt != "" {
					last[r.Attempt] = i
				}
			}
			for i, r := range results {
				if r.Result != Red || !inHour(r.At) || r.Attempt != "" && last[r.Attempt] != i {
					continue
				}
				cause := "unknown"
				if r.Cause != nil {
					switch r.Cause.Kind {
					case "own":
						cause = "goal"
					case "main", "flake":
						cause = r.Cause.Kind
					}
				}
				reds++
				causes[cause]++
				detail := cause
				if r.Cause != nil && cause == "goal" {
					detail += " " + r.Cause.Goal
				}
				fmt.Fprintf(&out, "- Red %s: cause %s; %s; %s; evidence %s\n", r.Attempt, detail, r.Reason, strings.Join(failingTests(r.Failed), ", "), r.Log)
			}
		}
		main, err := seams.git(checkout, "rev-parse", "--verify", "refs/remotes/origin/main^{commit}")
		if err != nil {
			return err
		}
		incidents, err := seams.incidents(install, checkout, main)
		if err != nil {
			return err
		}
		for _, i := range incidents {
			if inHour(i.Opened) {
				fmt.Fprintf(&out, "- Incident %s opened: %s %s\n", i.ID, i.TestUnit, i.TestName)
			}
			if i.Closed != nil && inHour(i.Closed.At) {
				fmt.Fprintf(&out, "- Incident %s closed: %s\n", i.ID, i.Closed.Why)
			}
		}
		for _, act := range acts {
			if act.Row == "confirmed-flake" && inHour(act.At) && strings.HasPrefix(act.What, "opened ") && !strings.HasPrefix(act.Outcome, "failed:") {
				fmt.Fprintf(&out, "- Flake goal %s; sightings %s\n", act.What, act.Why)
			}
		}
		if status.Health != nil {
			seenFix := map[string]bool{}
			for _, f := range status.Health.Flakes {
				if f.FixGoal == "" || seenFix[f.FixGoal] {
					continue
				}
				seenFix[f.FixGoal] = true
				file, err := readFlakeFixGoal(install, checkout, f.FixGoal, seams)
				if err != nil {
					fmt.Fprintf(&out, "- Flake fix goal %s cannot be read: %v\n", f.FixGoal, err)
					continue
				}
				if file.State != goal.StateDone && file.State != goal.StateAbandoned {
					fmt.Fprintf(&out, "- Flake fix goal %s still open: %s\n", f.FixGoal, file.State)
				}
			}
		}
		fmt.Fprintln(&out, "\n### Speed")
		landed, returned, oldest := 0, 0, 0.
		var landings []Entry
		for _, e := range status.Queue {
			end := now.Format(time.RFC3339Nano)
			if e.State == StateLanded {
				end = e.LandedAt
			} else if e.State == StateReturned {
				end = e.ReturnedAt
			}
			if e.State == StateSuperseded || e.State != StateWaiting && !inHour(end) {
				continue
			}
			if e.State == StateLanded {
				landed++
				landings = append(landings, e)
			} else if e.State == StateReturned {
				returned++
			}
			minutes, target := elapsedMinutes(e.At, end), "against 60 min"
			if minutes != nil && *minutes > 60 {
				target = "over 60 min"
			}
			fmt.Fprintf(&out, "- %s: %s, %s min; %s; %s\n", e.Goal, e.State, measured(minutes), target, e.Reason)
		}
		pushes, err := readLines[Pushed](pushesPath(install))
		if err != nil {
			return err
		}
		for _, p := range pushes {
			if inHour(p.At) {
				fmt.Fprintf(&out, "- Push %s at %s\n", p.Commit, lane.LocalText(p.At))
			}
		}
		if status.Health != nil {
			for _, row := range status.Health.Rows() {
				fmt.Fprintf(&out, "- Row %s: %s; %s\n", row.Row, row.Subject, row.Act)
			}
		}
		count := 0
		for _, a := range acts {
			if inHour(a.At) {
				count++
				fmt.Fprintf(&out, "- Row %s: %s; act %s; why %s; how %s; outcome %s\n", a.Row, a.Subject, a.What, a.Why, a.How, a.Outcome)
			}
		}
		fmt.Fprintln(&out, "\n### drift")
		events, skipped, err := landingstate.ReadEvents(Dir(install))
		if err != nil {
			fmt.Fprintf(&out, "- The lane shadow events cannot be read: %s\n", err)
		} else if skipped > 0 {
			fmt.Fprintf(&out, "- %s\n", landingstate.SkippedEvidence(skipped))
		}
		for _, event := range events {
			if event.Kind == "drift" && event.By == "keeper" && event.At.After(from) && !event.At.After(now) {
				differences := strings.Split(event.Evidence, "\n")
				for i, difference := range differences {
					field, values, _ := strings.Cut(difference, ": ")
					before, after, _ := strings.Cut(values, " → ")
					differences[i] = field + ": " + localShadowValue(before) + " → " + localShadowValue(after)
				}
				fmt.Fprintf(&out, "- %s: %s\n", event.At.Local().Format("15:04"), strings.Join(differences, "; "))
			}
		}
		fmt.Fprintln(&out, "\n### Tokens\n\nSession totals; a landing session may be shared across goals.")
		records, err := store.List()
		if err != nil {
			return err
		}
		if len(landings) == 0 {
			fmt.Fprintln(&out, "no landing this hour")
		}
		for _, e := range landings {
			criticFrom := ""
			for _, old := range status.Queue {
				distance := elapsedMinutes(old.LandedAt, e.LandedAt)
				if old.Goal == e.Goal && old.State == StateLanded && distance != nil && *distance > 0 && (criticFrom == "" || elapsedMinutes(criticFrom, old.LandedAt) != nil) {
					criticFrom = old.LandedAt
				}
			}
			for _, kind := range []string{"landing", "critics"} {
				found := false
				var total int64
				var sessions, missing []string
				for _, r := range records {
					match := r.Kind == "read" || r.Kind == "critique"
					if kind == "landing" {
						match = r.Kind == launch.LandingKind && r.WorkingDirectory == checkout && elapsedMinutes(r.StartedAt, e.LandedAt) != nil && (r.FinishedAt == "" || elapsedMinutes(e.At, r.FinishedAt) != nil)
					} else {
						sincePrevious := elapsedMinutes(criticFrom, r.StartedAt)
						match = match && r.Goal == e.Goal && elapsedMinutes(r.StartedAt, e.LandedAt) != nil && (criticFrom == "" || sincePrevious != nil && *sincePrevious > 0)
					}
					if !match {
						continue
					}
					if root := r.AdapterData["fenceRoot"]; len(root) > 0 {
						var recorded string
						if err := json.Unmarshal(root, &recorded); err != nil {
							return err
						}
						match = match && (recorded == "" || filepath.Clean(recorded) == filepath.Clean(install))
					}
					if !match {
						continue
					}
					found = true
					dir, err := store.StateDir(r.ID)
					if err != nil {
						return err
					}
					if r.Measurement.UsageRead {
						total += r.Measurement.TotalTokens()
						sessions = append(sessions, fmt.Sprintf("  - session %s: %d tokens", r.ID, r.Measurement.TotalTokens()))
					} else {
						missing = append(missing, filepath.Join(dir, "record.json")+" measurement.usageRead")
					}
				}
				if !found {
					dir, err := store.StateDir("missing")
					if err != nil {
						return err
					}
					window := "first landing (no lower bound)"
					if criticFrom != "" {
						window = lane.LocalText(criticFrom)
					}
					fmt.Fprintf(&out, "- %s %s: unmeasured: no session record in %s; window %s to %s\n", e.Goal, kind, filepath.Dir(dir), window, lane.LocalText(e.LandedAt))
				} else {
					value := fmt.Sprintf("%d tokens", total)
					if len(missing) > 0 {
						value = "unmeasured: " + strings.Join(missing, ", ")
						if len(sessions) > 0 {
							value += fmt.Sprintf("; measured sessions %d tokens", total)
						}
					}
					fmt.Fprintf(&out, "- %s %s: %s\n", e.Goal, kind, value)
					for _, session := range sessions {
						fmt.Fprintln(&out, session)
					}
				}
			}
		}
		overdueGoals, overdueRecords := 0, 0
		if status.Health != nil {
			for _, row := range status.Health.Rows() {
				if row.Row == "overdue-goal" {
					overdueGoals++
					if minutes, err := strconv.ParseFloat(row.Minutes, 64); err == nil {
						oldest = max(oldest, minutes)
					}
				}
				if row.Row == "overdue-record" {
					overdueRecords++
				}
			}
		}
		summary = fmt.Sprintf(landingstate.Phrase("hour-to-s-d-landed-d-returned-d-red"), now.Local().Format("15:04"), landed, returned, reds, causes["goal"], causes["main"], causes["flake"], causes["unknown"], oldest, count, overdueGoals, overdueRecords)
		fmt.Fprintf(&out, "\n%s\n\n", summary)
		path := filepath.Join(Dir(install), "steward-report.md")
		old, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := atomicfile.WriteVolatile(path, out.String()+string(old)); err != nil {
			return err
		}
		data, err := json.Marshal(stewardReport{At: now.UTC(), Summary: summary})
		if err != nil {
			return err
		}
		return atomicfile.WriteVolatileFile(filepath.Join(Dir(install), "steward-report.json"), data, 0644)
	})
	return summary, err
}
