// Package hostcapacity composes the current host facts without keeping an inventory.
package hostcapacity

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

// Snapshot is a read-time observation, never permission to start work.
type Snapshot struct {
	At          string           `json:"at"`
	Owner       lane.Record      `json:"owner"`
	OwnerKnown  bool             `json:"ownerKnown"`
	Load        hostload.Sample  `json:"load"`
	Builds      []Build          `json:"builds"`
	BuildsKnown bool             `json:"buildsKnown"`
	Goals       []string         `json:"goals"`
	Providers   ProviderCoverage `json:"providers"`
	Errors      []string         `json:"errors"`
}

// Build identifies a reservation or running build in the global launch store.
type Build struct {
	ID               string       `json:"id"`
	Goal             string       `json:"goal"`
	Runtime          string       `json:"runtime"`
	State            launch.State `json:"state"`
	WorkingDirectory string       `json:"workingDirectory"`
}

// ProviderCoverage names the shared provider evidence that is not yet available.
type ProviderCoverage struct {
	Available bool          `json:"available"`
	Detail    string        `json:"detail"`
	Marks     []outage.Mark `json:"marks,omitempty"`
}

// Sources allows each caller to supply its readers; nil uses the real sources.
type Sources struct {
	Load         func(time.Time) hostload.Sample
	Registration func(string) (lane.Record, bool, error)
}

// Read samples the host and reconciles global launches between two registration reads.
// Changed or unreadable ownership keeps facts visible but ownership unknown.
func Read(home string, manager *launch.Manager, now time.Time, sources Sources) Snapshot {
	if sources.Load == nil {
		sources.Load = hostload.Read
	}
	if sources.Registration == nil {
		sources.Registration = lane.Read
	}
	s := Snapshot{At: now.UTC().Format(time.RFC3339Nano), Builds: []Build{}, Goals: []string{}, Errors: []string{},
		Providers: ProviderCoverage{Detail: "shared provider coverage is unavailable"}}
	owner, registered, err := sources.Registration(home)
	s.Owner, s.OwnerKnown = owner, registered && err == nil
	if err != nil {
		s.Errors = append(s.Errors, "registration: "+err.Error())
	}
	if err == nil && !registered {
		s.Errors = append(s.Errors, "no host owner is registered; a person registers one with metasystem landing set PATH")
	}
	s.Load = sources.Load(now)
	if !s.Load.Available {
		s.Errors = append(s.Errors, "host load: "+s.Load.Detail)
	}
	records, err := manager.List()
	s.BuildsKnown = err == nil
	if err != nil {
		s.Errors = append(s.Errors, "builds: "+err.Error())
	}
	goals := map[string]bool{}
	for _, record := range records {
		if record.Kind == "build" && (record.State == launch.Starting || record.State == launch.Running) {
			s.Builds = append(s.Builds, Build{ID: record.ID, Goal: record.Goal, Runtime: record.Adapter, State: record.State, WorkingDirectory: record.WorkingDirectory})
			if record.Goal != "" {
				goals[record.Goal] = true
			}
		}
	}
	for goal := range goals {
		s.Goals = append(s.Goals, goal)
	}
	sort.Strings(s.Goals)
	providers, providerErr := outage.ReadProviders(home)
	if providerErr != nil {
		s.Providers.Detail = "provider state is unknown: " + providerErr.Error()
		s.Errors = append(s.Errors, s.Providers.Detail)
	} else if providers.Owner != owner {
		s.Providers.Detail = "provider ownership changed; run metasystem machine list again"
	} else if len(providers.Current) > 0 {
		s.Providers.Available, s.Providers.Detail = true, "current shared provider observations"
		for _, condition := range providers.Current {
			if condition.Mark.ConsecutiveFailures > 0 {
				s.Providers.Marks = append(s.Providers.Marks, condition.Mark)
			}
		}
		sort.Slice(s.Providers.Marks, func(i, j int) bool { return s.Providers.Marks[i].Provider < s.Providers.Marks[j].Provider })
	} else if _, err := os.Stat(filepath.Join(owner.Install, "artifacts", "agents", "outage.json")); err == nil {
		s.Providers.Detail = "legacy provider hint has unknown provider identity"
	}
	after, stillRegistered, err := sources.Registration(home)
	if err != nil || registered != stillRegistered || owner != after {
		s.OwnerKnown = false
		s.Providers = ProviderCoverage{Detail: "provider ownership changed or is unknown"}
		if err != nil {
			s.Errors = append(s.Errors, "registration re-read: "+err.Error())
		} else {
			s.Errors = append(s.Errors, "host ownership changed during the reading; run metasystem machine list again")
		}
	}
	return s
}
