// Package hostcapacity composes the current host facts without keeping an inventory.
package hostcapacity

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
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
	Usage       Usage            `json:"usage"`
}

// Build identifies a reservation or running build in the global launch store.
type Build struct {
	ID               string `json:"id"`
	Goal             string `json:"goal"`
	Runtime          string `json:"runtime"`
	State            string `json:"state"`
	WorkingDirectory string `json:"workingDirectory"`
}

// ProviderCoverage names the shared provider evidence that is not yet available.
type ProviderCoverage struct {
	Available bool          `json:"available"`
	Detail    string        `json:"detail"`
	Marks     []outage.Mark `json:"marks,omitempty"`
	Recovery  []Recovery    `json:"recovery,omitempty"`
}

type Recovery struct {
	Provider string `json:"provider"`
	Episode  string `json:"episode"`
	Interval string `json:"interval"`
	Due      string `json:"due"`
}

// Sources allows each caller to supply its readers; nil uses the real sources.
type Sources struct {
	Load         func(time.Time) hostload.Sample
	Registration func(string) (lane.Record, bool, error)
	RegistryPath string
	Usage        func(time.Time) (Usage, error)
}

// Read samples the host and reconciles global launches between two registration reads.
// Changed or unreadable ownership keeps facts visible but ownership unknown.
func Read(home string, manager interface{ CapacityBuilds() ([]Build, error) }, now time.Time, sources Sources) Snapshot {
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
	if sources.Usage != nil {
		s.Usage, err = sources.Usage(now)
		if err != nil {
			s.Usage.Problems = append(s.Usage.Problems, err.Error())
		}

		path := sources.RegistryPath
		if path == "" {
			path, err = registry.DefaultPath()
		}
		seats, registryErr := registry.HostCheckouts(path)
		for i := range s.Usage.Sessions {
			session := &s.Usage.Sessions[i]
			known := s.OwnerKnown && (session.WorkingDirectory == owner.Root || strings.HasPrefix(session.WorkingDirectory, owner.Root+string(filepath.Separator)))
			for _, seat := range seats {
				known = known || session.WorkingDirectory == seat.Path || strings.HasPrefix(session.WorkingDirectory, seat.Path+string(filepath.Separator))
			}
			if !known || registryErr != nil {
				session.Problems = append(session.Problems, "working directory registration is unknown")
			}
		}
	}
	if !s.Load.Available {
		s.Errors = append(s.Errors, "host load: "+s.Load.Detail)
	}
	records, err := manager.CapacityBuilds()
	s.BuildsKnown = err == nil
	if err != nil {
		s.Errors = append(s.Errors, "builds: "+err.Error())
	}
	goals := map[string]bool{}
	for _, build := range records {
		s.Builds = append(s.Builds, build)
		if build.Goal != "" {
			goals[build.Goal] = true
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
		for provider, condition := range providers.Current {
			if condition.Mark.ConsecutiveFailures > 0 {
				s.Providers.Marks = append(s.Providers.Marks, condition.Mark)
			}
			intervals := append([]outage.Interval{}, condition.Intervals...)
			if condition.Mark.ConsecutiveFailures > 0 {
				intervals = append(intervals, outage.Interval{Since: condition.Mark.Since, RecoveryAfter: condition.Mark.RecoveryAfter})
			}
			for _, interval := range intervals {
				dueText := "pending success"
				if due, err := interval.RecoveryDue(); err != nil {
					dueText = "unknown: " + err.Error()
				} else if !due.IsZero() {
					dueText = due.UTC().Format(time.RFC3339Nano)
				}
				s.Providers.Recovery = append(s.Providers.Recovery, Recovery{provider, interval.Since, interval.RecoveryAfter, dueText})
			}
		}
		sort.Slice(s.Providers.Recovery, func(i, j int) bool {
			a, b := s.Providers.Recovery[i], s.Providers.Recovery[j]
			return a.Provider+a.Episode < b.Provider+b.Episode
		})
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
	if sources.Usage != nil && !s.OwnerKnown {
		s.Usage.Problems = append(s.Usage.Problems, "host ownership changed or is unknown")
	}
	return s
}
