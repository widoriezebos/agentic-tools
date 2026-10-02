package steward

// Steward signals to the channel (design blocked-agent-asks-the-human,
// Decision 4). Each named signal is a two-line notice, line 1 what happened
// and line 2 the one command, posted once per episode through the
// configured channel provider at the signal's own producer. With no channel
// configured each producer keeps its current local path. There is no
// routing here: each producer names its own notice and calls this one post.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	channelphase "github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// channelPostTimeout bounds one channel post, as notifyTimeout bounds one
// local delivery: a hung provider is a failed attempt, never a wedged tick.
const channelPostTimeout = 15 * time.Second

// signalChannel is the configured channel as one signal's producer reads
// it: configured is false only when no channel adapter is set; a configured
// channel that cannot be loaded is configured, and its post fails with the
// load's error so the episode records a failed transport.
type signalChannel struct {
	configured bool
	loaded     channelphase.Loaded
	loadErr    error
}

func loadSignalChannel(repoRoot string) signalChannel {
	loaded, err := channelphase.Load(repoRoot, false)
	if loaded.Provider == nil && err == nil {
		return signalChannel{}
	}
	return signalChannel{configured: true, loaded: loaded, loadErr: err}
}

// transport posts notice to the channel in place of the episode's local
// message: the notice is the channel's plain two-line text.
func (c signalChannel) transport(notice string) func(string, string) error {
	return func(string, string) error {
		if c.loadErr != nil {
			return c.loadErr
		}
		if c.loaded.Provider == nil {
			return errors.New("the channel is configured but has no provider")
		}
		ctx, cancel := context.WithTimeout(context.Background(), channelPostTimeout)
		defer cancel()
		_, err := c.loaded.Provider.Post(ctx, c.loaded.Destination, notice, nil)
		return err
	}
}

// signalTransport is a producer's transport for one notice: the channel's
// post when a channel is configured, else local (the current path; nil
// when that path submits nothing).
func signalTransport(repoRoot, notice string, local func(string, string) error) func(string, string) error {
	if channel := loadSignalChannel(repoRoot); channel.configured {
		return channel.transport(notice)
	}
	return local
}

func signalDigest(parts ...string) string {
	key := ""
	for _, part := range parts {
		key += part + "\n"
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// seatIdleNotice is the idle seat's channel notice.
func seatIdleNotice(incident SeatIdleIncident) string {
	waiting := "while work is ready"
	if incident.GoalID != "" {
		waiting = "while goal " + incident.GoalID + " is ready"
	}
	return fmt.Sprintf("Seat %s has stopped %d times %s.\nmetasystem status", incident.SessionID, incident.Refusal, waiting)
}

// spendNotice is a spend crossing's channel notice.
func spendNotice(crossing SpendCrossing) string {
	spendValue, limitValue := fmt.Sprintf("%.0f", crossing.Spend), fmt.Sprintf("%.0f", crossing.Limit)
	if crossing.Ceiling == "money" {
		spendValue, limitValue = fmt.Sprintf("%.2f", crossing.Spend), fmt.Sprintf("%.2f", crossing.Limit)
	}
	return fmt.Sprintf("Spend for %s on %s crossed %d times its %s ceiling (%s of %s); nothing is refused.\nmetasystem settings set spend.ceiling.%s.%s NEW-LIMIT",
		crossing.ScopeID, crossing.Machine, crossing.Multiple, crossing.Ceiling, spendValue, limitValue, crossing.Scope, crossing.Ceiling)
}

// breachStopAlertOwner owns the breach-stop notice episodes: one per
// stopped goal revision.
const breachStopAlertOwner = "breach-stop"

func breachStopNotice(goalID string) string {
	return fmt.Sprintf("Goal %s was stopped: it spent its budget.\nmetasystem goal resume %s", goalID, goalID)
}

// noticeBreachStops opens one notice episode per stopped goal revision: a
// COMPLETE report, keyed by goal and revision, which every report carries
// (the first has no stop id). The stop itself is unchanged and nothing
// resumes the goal; with no channel configured the tick report is all.
func noticeBreachStops(repoRoot string, reports []BreachStopReport, now time.Time) error {
	var stopped []BreachStopReport
	for _, report := range reports {
		if report.State == "COMPLETE" && report.GoalID != "" {
			stopped = append(stopped, report)
		}
	}
	if len(stopped) == 0 {
		return nil
	}
	channel := loadSignalChannel(repoRoot)
	if !channel.configured {
		return nil
	}
	held, err := lockAlerts(repoRoot, lock.Exclusive)
	if err != nil {
		return err
	}
	defer unlockAlerts(held)
	episodes, err := loadAlertEpisodesUnlocked(repoRoot)
	if err != nil {
		return err
	}
	for _, report := range stopped {
		digest := signalDigest(breachStopAlertOwner, report.GoalID, fmt.Sprint(report.Revision))
		episode := (*AlertEpisode)(nil)
		for index := range episodes {
			if episodes[index].Owner == breachStopAlertOwner && episodes[index].Digest == digest {
				episode = &episodes[index]
				break
			}
		}
		if episode == nil {
			episodes = append(episodes, AlertEpisode{
				Schema: 1, EpisodeID: nextEpisodeID(digest, episodes), Digest: digest,
				Owner: breachStopAlertOwner, ScopeID: report.GoalID, Message: breachStopNotice(report.GoalID),
				OpenedAt: now.UTC(), Attempts: []AlertAttempt{}, TransportResult: TransportPending,
			})
			episode = &episodes[len(episodes)-1]
			if err := saveAlertEpisode(repoRoot, *episode); err != nil {
				return err
			}
		}
		if episode.Cleared || episode.TransportResult == TransportSubmitted {
			continue
		}
		if err := submitEpisode(repoRoot, episode, now, channel.transport(episode.Message)); err != nil {
			return err
		}
	}
	return nil
}

// reportBreachStopNotices is the tick's best-effort notice pass: the stop
// already happened and its report stands whatever the notice does. A
// transport failure is kept on the episode and retried on the next report;
// a store failure is said on the runner's error stream and retried too.
func reportBreachStopNotices(repoRoot string, reports []BreachStopReport, now time.Time) {
	if err := noticeBreachStops(repoRoot, reports, now); err != nil {
		fmt.Fprintf(os.Stderr, "breach-stop notice not recorded: %v\n", err)
	}
}
