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
	class := "cheap"
	for _, entry := range entries {
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
		if err != nil || tier == 0 || tier > 3 || int(tier) >= threshold {
			class = "full"
		}
	}
	return entries, class, nil
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
