package plain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func selectDepthClass(install, checkout string, entries []Entry, seams ProveSeams) ([]Entry, string, error) {
	raw, _, err := config.Get(config.GetParams{Key: "landing.full-from-tier", ConfPath: filepath.Join(install, "metasystem.conf")})
	if errors.Is(err, os.ErrNotExist) {
		raw, _ = config.CompiledDefault("landing.full-from-tier")
		err = nil
	}
	threshold, parseErr := strconv.Atoi(raw)
	if err != nil || parseErr != nil || threshold < 1 || threshold > 3 {
		return nil, "", fmt.Errorf("landing.full-from-tier must be a tier from 1 to 3: %q", raw)
	}
	full := make([]bool, len(entries))
	parent := make([]int, len(entries))
	var root func(int) int
	root = func(i int) int {
		if parent[i] != i {
			parent[i] = root(parent[i])
		}
		return parent[i]
	}
	for i, entry := range entries {
		parent[i] = i
		var tier uint8
		if seams.GoalTier != nil {
			tier, err = seams.GoalTier(install, entry.Goal)
		} else {
			var data []byte
			data, err = os.ReadFile(filepath.Join(install, "plans", "goals", entry.Goal+".md"))
			if err == nil {
				file, problems := goal.ParseFile(data)
				if len(problems) == 0 && file.Id == entry.Goal {
					tier = file.Tier
				}
			}
		}
		full[i] = err != nil || tier == 0 || tier > 3 || int(tier) >= threshold
	}
	mixed := false
	for _, value := range full {
		mixed = mixed || value != full[0]
	}
	if !mixed {
		class := "cheap"
		if full[0] {
			class = "full"
		}
		return entries, class, nil
	}
	queued, err := batchEntries(install)
	if err != nil {
		return nil, "", err
	}
	main, err := checkoutGit(checkout, seams).main()
	if err != nil {
		return nil, "", err
	}
	handIns := map[string][]string{}
	for _, entry := range append(queued, entries...) {
		landed, err := batchContains(checkout, main, entry.SHA, seams)
		if err != nil {
			return nil, "", err
		}
		if landed {
			continue
		}
		handIns[entry.Goal] = append(handIns[entry.Goal], entry.SHA)
	}
	// A connected ancestry group cannot be split: a waiting tip can carry
	// another goal's unlanded superseded hand-in as well as its current one.
	for i := range entries {
		for j := 0; j < i; j++ {
			related := false
			for _, pair := range [][2]int{{i, j}, {j, i}} {
				for _, sha := range handIns[entries[pair[1]].Goal] {
					related, err = batchContains(checkout, entries[pair[0]].SHA, sha, seams)
					if err == nil && !related {
						related, err = batchContains(checkout, sha, entries[pair[0]].SHA, seams)
					}
					if err != nil {
						return nil, "", err
					}
					if related {
						break
					}
				}
				if related {
					break
				}
			}
			if related {
				parent[root(i)] = root(j)
			}
		}
	}
	groupFull := map[int]bool{}
	for i := range entries {
		groupFull[root(i)] = groupFull[root(i)] || full[i]
	}
	wantFull := true
	for i := range entries {
		if !groupFull[root(i)] {
			wantFull = false
		}
	}
	selected := []Entry{}
	for i, entry := range entries {
		if groupFull[root(i)] == wantFull {
			selected = append(selected, entry)
		}
	}
	class := "cheap"
	if wantFull {
		class = "full"
	}
	return selected, class, nil
}

func overdueBatch(install, batchID string, seams ProveSeams) (string, time.Time) {
	if batchID == "" {
		return "", time.Time{}
	}
	due, err := fullProofDue(install, seams.now())
	if err != nil {
		return "full proof clock cannot be read: " + err.Error(), seams.now()
	}
	if !due {
		return "", time.Time{}
	}
	last, every, _ := fullProofClock(install)
	since := last.Add(every)
	if last.IsZero() {
		since = seams.now()
		if batch, err := ReadBatch(install); err == nil && batch != nil && batch.ID == batchID {
			if at, err := time.Parse(time.RFC3339Nano, batch.CreatedAt); err == nil {
				since = at
			}
		}
	}
	if push, ok, _ := LastPush(install); ok {
		if proof, ok, _ := ResultFor(install, push.Tree); ok && proof.Scope == "scoped" {
			if at, err := time.Parse(time.RFC3339, proof.FullAt); err == nil && at.Add(time.Hour).Before(since) {
				since = at.Add(time.Hour)
			}
		}
	}
	return "full proof overdue since " + since.Local().Format("15:04"), since
}

func batchGreenReusable(install, batchID string, result Result, seams ProveSeams) bool {
	if !result.reusableGreen(seams.now()) {
		return false
	}
	if seams.Gate {
		return true
	}
	if reason, since := overdueBatch(install, batchID, seams); reason != "" {
		at, err := time.Parse(time.RFC3339, result.FullAt)
		return err == nil && !at.Before(since.Truncate(time.Second)) && result.Scope == "full" && result.FullTree != ""
	}
	return true
}
