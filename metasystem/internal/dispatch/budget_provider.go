package dispatch

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

type clockDependency struct {
	at, until      time.Time
	runtime        string
	providerFailed bool
}

// A failed job remains dependent only when its retained provider evidence
// explains the failure; a runtime error alone does not establish an outage.
func jobProviderFailure(repoRoot, jobID string, record map[string]any) bool {
	if asString(record["status"]) != "failed" {
		return false
	}
	jobsDir := filepath.Join(repoRoot, "artifacts", "agents", "jobs")
	if _, _, hit := outage.ClassifyLogs(filepath.Join(jobsDir, jobID+".log")); hit {
		return true
	}
	rootJob, err := usage.RootJobID(jobsDir, jobID)
	if err != nil {
		return false
	}
	roundDir := filepath.Join(repoRoot, "artifacts", "agents", rootJob, "rounds", asString(record["round"]))
	for _, name := range []string{"claude-result.json", "raw.out"} {
		if _, _, hit := outage.ClassifyProviderResult(filepath.Join(roundDir, name)); hit {
			return true
		}
	}
	return false
}

// providerWaits follows retained work, never the checkout's current roster.
// Starting local work or another provider's work ends the previous dependency.
func providerWaits(home, goalID string, start, now time.Time, dependencies []clockDependency) (spans []outage.Span, err error) {
	store := launch.Store{}
	if home != "" {
		store.Root = filepath.Join(home, "launch")
	}
	records, err := store.List()
	if err != nil {
		return nil, fmt.Errorf("launch dependency history is unreadable: %w", err)
	}
	for _, record := range records {
		if record.Goal != goalID {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, record.StartedAt)
		if err != nil {
			return nil, fmt.Errorf("launch %s dependency time is unreadable", record.ID)
		}
		until, endErr := time.Parse(time.RFC3339Nano, record.FinishedAt)
		if record.State.Terminal() && (endErr != nil || until.Before(at)) {
			return nil, fmt.Errorf("launch %s execution end is unreadable", record.ID)
		}
		if !record.State.Terminal() {
			until = now
		}
		dependencies = append(dependencies, clockDependency{at: at, until: until, runtime: record.Adapter,
			providerFailed: record.State == launch.Failed && record.Cause == outage.ProviderLimit})
	}
	sort.Slice(dependencies, func(i, j int) bool { return dependencies[i].at.Before(dependencies[j].at) })
	if !slices.ContainsFunc(dependencies, func(d clockDependency) bool { return d.runtime != "local" && d.runtime != "plain-exec" }) {
		return nil, nil
	}
	providers, err := outage.ReadProviders(home)
	if err != nil {
		return nil, err
	}
	for i, dependency := range dependencies {
		end := now
		if i+1 < len(dependencies) {
			end = dependencies[i+1].at
			if end.Equal(dependency.at) && outage.Provider(dependency.runtime) != outage.Provider(dependencies[i+1].runtime) {
				return nil, fmt.Errorf("simultaneous work has an unknown provider dependency")
			}
		}
		// Execution's end releases the dependency unless the provider stopped it.
		if !dependency.providerFailed && dependency.until.Before(end) {
			end = dependency.until
		}
		from := dependency.at
		if from.Before(start) {
			from = start
		}
		if end.After(now) {
			end = now
		}
		for _, active := range dependencies {
			if !active.at.After(from) && active.until.After(from) && outage.Provider(active.runtime) != outage.Provider(dependency.runtime) {
				from = active.until
			}
		}
		if !end.After(from) {
			continue
		}
		waiting, err := providers.Waiting(dependency.runtime, from, end)
		if err != nil {
			return nil, err
		}
		spans = append(spans, waiting...)
	}
	return spans, nil
}
