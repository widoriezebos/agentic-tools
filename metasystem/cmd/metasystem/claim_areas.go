package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// claimAreaReaders observes accepted design bytes and this host's newest queue
// for every publication attempt. Local main is independent of the ledger tip.
func (inv *intentInvocation) claimAreaReaders() goal.ClaimAreaReaders {
	return goal.ClaimAreaReaders{Design: func(id, tip string) goal.AreaSnapshot {
		paths, problem := inv.acceptedDesignPaths(id)
		if problem != nil {
			return goal.AreaSnapshot{Warnings: []string{"areas unknown for goal " + id + ": " + problem.Summary}}
		}
		var snapshot goal.AreaSnapshot
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				return goal.AreaSnapshot{Warnings: []string{"areas unknown for goal " + id + " in design " + path + ": " + err.Error()}}
			}
			designID := ""
			for _, line := range strings.Split(string(data), "\n") {
				if value, ok := strings.CutPrefix(line, "- Id:"); ok {
					designID = strings.TrimSpace(value)
					break
				}
			}
			areas, known, err := launch.DeclaredAreas(string(data))
			source := fmt.Sprintf("%s@%x", designID, sha256.Sum256(data))
			if err != nil || !known {
				return goal.AreaSnapshot{Source: source, Warnings: []string{"areas unknown for goal " + id + " in design " + path}}
			}
			snapshot.Known = true
			snapshot.Areas = append(snapshot.Areas, areas...)
			if snapshot.Source != "" {
				snapshot.Source += ";"
			}
			snapshot.Source += source
		}
		snapshot.Areas, _ = launch.NormalizeAreas(snapshot.Areas)
		if !snapshot.Known {
			snapshot.Warnings = []string{"areas unknown for goal " + id + ": no accepted design declares areas"}
		}
		return snapshot
	}, Queue: func() ([]goal.ClaimAreaEntry, []string) {
		root, configured, problem := inv.laneCheck(nil)
		if problem != nil {
			return nil, []string{"areas unknown for landing queue: " + problem.Summary}
		}
		if !configured {
			return nil, nil
		}
		install, err := inv.laneInstallOf(root)
		if err != nil {
			return nil, []string{"areas unknown for landing queue at " + root + ": " + err.Error()}
		}
		queuePath := filepath.Join(plain.Dir(install), "queue.jsonl")
		entries, err := plain.AreaEntries(install)
		if err != nil {
			return nil, []string{"areas unknown for landing queue " + queuePath + ": " + err.Error()}
		}
		main := ""
		if read := inv.delivery().claimMain; read != nil {
			main, err = read(inv.layout.GitRoot)
		} else {
			main, err = branch.ScrubbedGit(inv.layout.GitRoot, "rev-parse", "--verify", "refs/heads/main")
		}
		// A missing or unreadable main cannot prove containment; keep exclusions.
		if err != nil {
			main = ""
		}
		newest := map[string]plain.Entry{}
		code := map[string]plain.Entry{}
		for _, entry := range entries {
			newest[entry.Goal] = entry
			if !entry.Records {
				code[entry.Goal] = entry
			}
		}
		var out []goal.ClaimAreaEntry
		ids := make([]string, 0, len(newest))
		for id := range newest {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		for _, id := range ids {
			entry := newest[id]
			snapshot := entry.AreaSnapshot
			state := entry.State
			contained := false
			// Records never hide the code's landing or the newest queue state.
			work := entry
			if codeEntry, ok := code[id]; ok {
				work = codeEntry
			}
			if main != "" {
				if read := inv.delivery().laneContains; read != nil {
					contained, _ = read(work.SHA, main)
				} else {
					contained, _ = plain.ContainedIn(inv.layout.InstallationRoot.Path(), main)(work.SHA)
				}
			}
			if contained {
				state = plain.StateLanded
			} else if state == plain.StateLanded {
				state = plain.StateWaiting
			}
			out = append(out, goal.ClaimAreaEntry{Goal: id, State: state, Snapshot: snapshot})
		}
		return out, nil
	}}
}
