package seat

// A machine's work that runs as a launch of the host's launch lane rather than
// as a delegate job record: the seat session the steward starts (`claude -p`,
// launch kind seat) and the builds, reviews and proofs that seat launches in
// turn. Their records live in the host's launch store, not under the
// checkout's artifacts/agents/jobs, so presence that read the job records
// alone published every such machine as idle while it worked.
//
// Each running launch becomes one more job record of the machine whose work
// it is, and the chain, the Running column and this seat's own row are
// composed from the joined set exactly as before. Nothing here decides a
// launch's state: an ended launch is not work in hand, and one that has not
// reached running is a reservation.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// ReadWork is what this machine is doing, from both places its work is
// recorded: the checkout's delegate job records and the host's launches
// that run in this checkout or in one of its goals' sibling worktrees.
func ReadWork(root string) JobSet {
	store, err := launch.DefaultRoot()
	if err != nil {
		jobs := ReadJobs(root)
		jobs.Unreadable = append(jobs.Unreadable, "launch records: "+err.Error())
		return jobs
	}
	return readWork(root, store)
}

func readWork(root, store string) JobSet {
	jobs := ReadJobs(root)
	launched, problems := launchJobs(root, store)
	jobs.Records = append(jobs.Records, launched...)
	jobs.Unreadable = append(jobs.Unreadable, problems...)
	return jobs
}

// launchJobs reads the launch store once and keeps the launches in flight
// that are this checkout's.
//
// A launch is this machine's when it works in the checkout itself or in the
// sibling worktree `<checkout>-<goal>` of the goal it names, which is where
// `metasystem work` puts a goal's build. A directory created a moment before
// its record is written is skipped rather than reported unreadable.
func launchJobs(root, store string) ([]JobRecord, []string) {
	entries, err := os.ReadDir(store)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []string{"launch records: " + err.Error()}
	}
	top := checkoutTop(root)
	var goals map[string]string
	var records []JobRecord
	var problems []string
	reader := launch.Store{Root: store}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		record, err := reader.Read(entry.Name())
		if err != nil {
			if errors.Is(err, launch.ErrInvalidID) || errors.Is(err, os.ErrNotExist) {
				continue
			}
			problems = append(problems, "launch "+entry.Name()+": "+err.Error())
			continue
		}
		status := launchStatus(record.State)
		if status == "" || record.StartedAt == "" || !launchOf(top, record) {
			continue
		}
		goal := record.Goal
		if goal == "" && record.Kind == "seat" {
			if goals == nil {
				goals = seatGoals(root)
			}
			goal = goals[record.ID]
		}
		started := launchTime(record.StartedAt)
		job := JobRecord{
			Job: record.ID, Role: launchRole(record), Goal: goal, Round: record.Round,
			StartedAt: started, CreatedAt: started, Status: status,
		}
		if record.MaxRounds > 0 {
			limit := record.MaxRounds
			job.ReviewRoundLimit = &limit
		}
		records = append(records, job)
	}
	return records, problems
}

// launchTime is a launch's start in presence's own form: a launch stamps it
// to the microsecond, and presence times carry whole seconds, so an
// unconverted stamp would read as a job with no start and no minutes.
func launchTime(value string) string {
	at, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return FormatTime(at)
}

// launchStatus is a launch's state in the job vocabulary: running is being
// worked, starting is a reservation, and an ended launch is nothing in hand.
func launchStatus(state launch.State) string {
	switch state {
	case launch.Running:
		return RunningStatus
	case launch.Starting:
		return "pending"
	}
	return ""
}

// launchRole is the stage a launch is, in the words the Running column says.
// It follows the board's own projection of a launch (internal/launch
// publishCard): a build past its first round is a revise round, and a read
// is a review.
func launchRole(record launch.Record) string {
	switch record.Kind {
	case "seat":
		return "working"
	case "build":
		if record.Round > 1 {
			return "revise"
		}
		return "building"
	case "read", "critique":
		return "review"
	case "proof":
		return "unit proof"
	case "design":
		return "design"
	case "":
		return "launch"
	}
	return record.Kind
}

// launchOf reports whether a launch works in this checkout or in the
// sibling worktree of the goal it names.
func launchOf(top string, record launch.Record) bool {
	dir := cleanPath(record.WorkingDirectory)
	if dir == "" || top == "" {
		return false
	}
	if within(dir, top) {
		return true
	}
	return record.Goal != "" && within(dir, top+"-"+record.Goal)
}

func within(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

// checkoutTop is the folder that holds root's .git: the checkout a launch's
// working directory is measured against. Without one it is root itself.
func checkoutTop(root string) string {
	start := cleanPath(root)
	for dir := start; dir != ""; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	return start
}

func cleanPath(path string) string {
	if path == "" {
		return ""
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	// A directory that is gone is resolved through its parent, so it is
	// compared in the same form as a checkout that still exists.
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		return resolved
	}
	if parent, err := filepath.EvalSymlinks(filepath.Dir(absolute)); err == nil {
		return filepath.Join(parent, filepath.Base(absolute))
	}
	return absolute
}

// seatGoals is the goal each seat launch the steward started serves, from
// the steward's own seat records. A record that cannot be read names no
// goal, and its seat still reads as working.
func seatGoals(root string) map[string]string {
	goals := map[string]string{}
	dir := filepath.Join(root, "artifacts", "agents", "steward", "seats")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return goals
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var record struct {
			LaunchID string `json:"launchId"`
			Goal     string `json:"goal"`
		}
		if json.Unmarshal(data, &record) == nil && record.LaunchID != "" {
			goals[record.LaunchID] = record.Goal
		}
	}
	return goals
}
