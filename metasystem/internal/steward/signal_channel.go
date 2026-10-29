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
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	channelphase "github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
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

// noticeBreachStops asks the person, once per stopped goal revision, whether
// the goal gets one more working day: a COMPLETE report with its box. The
// stop itself is unchanged and nothing resumes the goal by itself; with no
// channel configured the tick report is all. The channel duty's poll posts
// the question in the same pass, and again after a failed post. It also says
// whether it attempted a notice: a pass with nothing to ask attempts none.
func noticeBreachStops(repoRoot string, reports []BreachStopReport, now time.Time) (bool, error) {
	var problems []error
	attempted := false
	for _, report := range reports {
		if report.State != "COMPLETE" || report.GoalID == "" {
			continue
		}
		if !loadSignalChannel(repoRoot).configured {
			return false, nil
		}
		attempted = true
		if report.Budget == nil {
			problems = append(problems, fmt.Errorf("goal %s stopped with no box to ask about", report.GoalID))
			continue
		}
		_, _, err := channel.AskOrFind(breachStopAsk(repoRoot, report, now))
		problems = append(problems, err)
	}
	return attempted, errors.Join(problems...)
}

// breachStopAsk is the stopped goal's question: the box it used, and one
// more working day counted when the person chooses.
func breachStopAsk(repoRoot string, report BreachStopReport, now time.Time) channel.AskRequest {
	used := "its " + report.Budget.ElapsedLimit + " box"
	if report.Elapsed > 0 {
		used = goalbudget.FormatWorkingDuration(report.Elapsed) + " of " + used
	}
	return channel.AskRequest{RepoRoot: repoRoot, Goal: report.GoalID, Kind: "stop", Contract: true, Now: now, Recommendation: "One more day",
		Decision: &channel.Decision{Kind: "breach-stop", Subject: report.GoalID, State: fmt.Sprint(report.Revision)},
		Facts: []string{fmt.Sprintf("stopped at revision %d: %s", report.Revision, report.Reason),
			"undo after One more day: metasystem goal budget " + report.GoalID + " " + goalbudget.FormatBox(*report.Budget)},
		Plain: &channel.Plain{Decision: fmt.Sprintf("%s: it used %s and stopped at %s.", strings.ReplaceAll(report.GoalID, "-", " "), used, now.Local().Format("15:04")),
			IfNothing: "it stays stopped; nothing resumes it."},
		Options: []channel.Option{
			{Label: "One more day", Consequence: "its time box becomes the time used so far plus one working day (8 hours), counted when you choose; the other limits stay. Undo: goal budget back to today's box (it stops again).",
				Act: []string{"metasystem", "goal", "budget", report.GoalID, "one-more-day"}},
			{Label: "Leave it stopped", Consequence: "nothing runs and nothing is spent; this question stays open, so you can still choose One more day."},
		}}
}

// reportBreachStopNotices is the tick's best-effort question pass: the stop
// already happened and its report stands whatever the question does. A pass
// that asked is the breach-stop-notice duty's outcome; one with nothing to ask
// leaves the last outcome standing.
func reportBreachStopNotices(repoRoot string, reports []BreachStopReport, now time.Time) {
	if attempted, err := noticeBreachStops(repoRoot, reports, now); attempted {
		recordDuty(repoRoot, "breach-stop-notice", err, now)
	}
}
