package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

// landingFixActor authenticates the lane's current session or its descendant.
func landingFixActor(root string, caller int64) bool {
	holder, err := lease.CurrentHolder(root)
	if err != nil || holder.OwnerLineage != lane.AgentLineage {
		return false
	}
	seen := map[int64]bool{}
	for caller > 0 && !seen[caller] {
		seen[caller] = true
		who, err := lease.ClassifyVerbAt(root, root, caller)
		if err == nil && who.Class == lease.ClassMain && who.MainId == holder.MainId && who.Announcement != nil && who.Announcement.OwnerLineage == lane.AgentLineage {
			return true
		}
		parent, ok := identity.ParentPid(caller)
		if !ok {
			break
		}
		caller = parent
	}
	return false
}

func landingFixCommit(root, checkout string, caller int64) *landpath.LaneFixCommit {
	home, err := batchowner.LandingLaneHome()
	if err != nil {
		return nil
	}
	registered, present, err := lane.Read(home)
	if err != nil || !present || registered.Install != root || realpath.Resolve(registered.Root) != realpath.Resolve(checkout) || !landingFixActor(root, caller) {
		return nil
	}
	fix := landingFixCheckpoint(root, checkout, registered)
	active, readErr := plain.ActiveFix(root)
	if fix == nil || readErr != nil || active == nil || active.Commit != "" || active.Parent != fix.Commit || landingFixForRegistered(root, checkout, active.Goal, caller, registered) == nil {
		return nil
	}
	fix.Members, fix.Unit = []string{active.Goal}, fmt.Sprintf("lane-fix-%d", active.Round)
	seen := map[int64]bool{}
	for caller > 0 && !seen[caller] {
		seen[caller] = true
		argv, known := (identity.KernelProber{}).ReadArgv(caller)
		if known && len(argv) > 1 && filepath.Base(argv[0]) == "git" {
			if at := slices.Index(argv, "commit"); at >= 0 {
				fix.Message = landingFixMessage(checkout, argv[at+1:])
				return fix
			}
		}
		parent, ok := identity.ParentPid(caller)
		if !ok {
			break
		}
		caller = parent
	}
	return nil
}

func landingFixCheckpoint(root, checkout string, registered lane.Record) *landpath.LaneFixCommit {
	if realpath.Resolve(registered.Root) != realpath.Resolve(checkout) || registered.Install != root {
		return nil
	}
	batch, err := plain.ReadBatch(root)
	if err != nil || batch == nil || batch.State != plain.BatchRunning || batch.Lane != registered {
		return nil
	}
	running, recorded, _, err := plain.ReadRunning(root, plain.ProveSeams{})
	if err != nil || !recorded || running.Commit == "" || running.BatchID != batch.ID || !slices.Equal(running.BatchMembers, batch.Members) {
		return nil
	}
	fix := &landpath.LaneFixCommit{Commit: running.Commit}
	for _, member := range batch.Members {
		fix.Members = append(fix.Members, member.Goal)
	}
	return fix
}

// The guard reads the message from git's arguments; interactive or rewritten
// messages cannot establish a trailer before the pre-commit hook runs.
func landingFixMessage(checkout string, args []string) string {
	// Git must supply the complete message; edits and mixed sources stay fenced.
	var file string
	var literal bool
	for i := 0; i < len(args); i++ {
		key, value, inline := strings.Cut(args[i], "=")
		switch key {
		case "-F", "--file", "-m", "--message":
			if file != "" && (!literal || key != "-m" && key != "--message") {
				return ""
			}
			if !inline {
				i++
				if i == len(args) {
					return ""
				}
				value = args[i]
			}
			if file != "" {
				file += "\n\n" + value
			} else {
				file = value
			}
			literal = key == "-m" || key == "--message"
		case "--allow-empty", "--no-edit", "--quiet", "-q":
		default:
			return ""
		}
	}
	if literal {
		return file
	}
	if file == "" || file == "-" {
		return ""
	}
	if !filepath.IsAbs(file) {
		file = filepath.Join(checkout, file)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	return string(data)
}
