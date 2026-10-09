package steward

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"path/filepath"
	"time"
)

func ReconcileRecoveryRequests(self string, cfg TickConfig, census WorkerCensus) error {
	providers, err := outage.ReadProviders(cfg.ProviderHome)
	if err != nil || !lane.OwnsLane(self, providers.Owner) || cfg.Seat == nil {
		return err
	}
	path := cfg.RecoveryRegistry
	if path == "" {
		path, err = registry.DefaultPath()
	}
	if err != nil {
		return err
	}
	seats, err := registry.HostCheckouts(path)
	if err != nil {
		return err
	}
	var obligations error
	for _, seat := range seats {
		root, err := goal.ResolveStateRoot(seat.Path)
		if err == nil {
			held, lockErr := AcquireArbitration(root)
			err = lockErr
			if err == nil {
				err = reconcileSeatRecovery(root, cfg, census, providers, seat.Armed)
				held.Release()
			}
		}
		if err != nil {
			obligations = errors.Join(obligations, fmt.Errorf("seat %s: %w", seat.Path, err))
		}
	}
	return obligations
}
func reconcileSeatRecovery(root string, cfg TickConfig, census WorkerCensus, providers outage.Providers, armed bool) error {
	fresh, err := outage.ReadProviders(cfg.ProviderHome)
	if err != nil {
		return err
	}
	if fresh.Owner != providers.Owner {
		return fmt.Errorf("host registration changed during recovery observation")
	}
	host, _ := json.Marshal(fresh.Owner)
	questions, unreadable := channel.WalkQuestions(root)
	if len(unreadable) > 0 {
		return fmt.Errorf("recovery questions unreadable: %v", unreadable)
	}
	records, err := readSeatRecords(root)
	if err != nil {
		return err
	}
	for _, q := range questions {
		if q.Recovery == nil {
			continue
		}
		if q.Recovery.Host != string(host) {
			if err := channel.Close(root, q.ID, "host registration changed; this request is stale", nil, channel.DestinationConfig{}); err != nil {
				return err
			}
			if len(records) > 0 && records[len(records)-1].LaunchID == q.Recovery.Launch {
				return nil
			}
			continue
		}
		if q.State != "open" {
			continue
		}
		for _, r := range records {
			if r.RecoveryOf == nil || r.RecoveryOf.LaunchID != q.Recovery.Launch || r.RecoveryOf.Provider != q.Recovery.Provider || r.RecoveryOf.Episode != q.Recovery.Episode || q.Recovery.Seat != root {
				continue
			}
			if _, err := confirmSeatStart(r, cfg.Seat); err != nil {
				continue
			}
			if err := channel.Close(root, q.ID, "matching recovery launch "+r.LaunchID, nil, channel.DestinationConfig{}); err != nil {
				return err
			}
		}
	}
	if !armed || len(records) == 0 || helm.Active(root).Active {
		return nil
	}
	driver, _, err := config.Get(config.GetParams{Key: "seat.driver", ConfPath: filepath.Join(root, "metasystem.conf")})
	if err != nil || driver == "person" {
		return err
	}
	closed, _, err := stopfence.Closed(root)
	if err != nil || closed {
		return err
	}
	last := records[len(records)-1]
	if last.Outcome != SeatProviderLimit || last.ReapedAt == "" {
		return nil
	}
	state, err := cfg.Seat.SeatLaunch(last.LaunchID)
	if err != nil || !state.Found || !state.Terminal {
		return fmt.Errorf("failed launch %s has no confirmed terminal observation: %v", last.LaunchID, err)
	}
	provider := outage.Provider(state.Runtime)
	condition := fresh.Current[provider]
	if condition.Mark.ConsecutiveFailures > 0 {
		return nil
	}
	workers, err := census.Workers(root)
	if err != nil {
		return err
	}
	if !workers.CensusComplete || workers.Live > 0 || workers.Untracked > 0 || workers.Unprovable > 0 {
		return nil
	}
	work, err := goal.ReadClaimableBudgetedWork(root, cfg.now())
	if err != nil {
		return err
	}
	file, owned := work.OwnedClaim(last.Goal)
	if !owned || claimLineage(file) != SeatLineage || file.Approved == nil || approvalOpid(file) != last.ApprovalOpid {
		return nil
	}
	if busy, _, _ := seatBusyReader(cfg.WorkStateRoot)(root, work, cfg.now()); busy {
		return nil
	}
	for _, interval := range condition.Intervals {
		ended, since, until := seatTime(state.FinishedAt), seatTime(interval.Since), seatTime(interval.Until)
		if ended.IsZero() || since.IsZero() || until.IsZero() {
			return fmt.Errorf("provider episode or failed launch time is unreadable")
		}
		if ended.Before(since) || ended.After(until) {
			continue
		}
		due, err := interval.RecoveryDue()
		if err != nil {
			return err
		}
		if due.IsZero() || cfg.now().Before(due) {
			continue
		}
		command := fmt.Sprintf("metasystem machine revive %q --after %q", last.Machine, last.LaunchID)
		_, _, err = channel.AskOrFind(channel.AskRequest{RepoRoot: root, About: "machine", Kind: "other", Machine: last.Machine, Now: cfg.now(), Wants: command,
			Recovery: &channel.RecoverySubject{Host: string(host), Seat: root, Launch: last.LaunchID, Provider: provider, Episode: interval.Since, Due: due.UTC().Format(time.RFC3339Nano)},
			Facts:    []string{command, fmt.Sprintf("Seat %s still holds approved work after provider %s recovered.", last.Machine, provider), "Failed launch: " + last.LaunchID + "; provider episode: " + interval.Since, "Recovery grace: " + interval.RecoveryAfter + "; due: " + due.Local().Format(time.RFC3339)}})
		return err
	}
	return nil
}
