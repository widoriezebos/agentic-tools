package steward

// The narration is the system's running plain-English account of what
// it is doing — one sentence per tick, written for a person, kept
// beside the machine records it summarizes. It is strictly read-only
// observation: a narration failure never fails a tick, narration never
// decides anything, and the records remain the authority. The journey
// tells the story of what concluded; the narration tells the story of
// now.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
)

// narrationCapLines bounds the file: old ticks scroll away, because a
// running account that grows forever stops being readable and starts
// being a disk problem.
const narrationCapLines = 2000

// NarrationPath is the account's one location.
func NarrationPath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "narration.log")
}

// Narrate appends one tick's sentence. Best-effort by contract: every
// failure path returns silently, because the tick's real duties must
// never hang on the storyteller.
func Narrate(repoRoot string, result TickResult, cfg TickConfig) {
	line := narrationLine(repoRoot, result, cfg, time.Now())
	if line == "" {
		return
	}
	_ = appendNarrationLine(repoRoot, line)
}

// NarrateDigest records only the changes that matter on return. Unlike the
// scrolling tick narration, this concluded-story register is durable and a
// write failure fails the tick before its observation high-water advances.
func NarrateDigest(repoRoot string, previous Evidence, result TickResult, now time.Time) error {
	var entries []narratordigest.Entry
	commit := result.Evidence.Marks.HeadOid
	if commit != "" && commit != "no-head" && commit != previous.Marks.HeadOid {
		shown := commit
		if len(shown) > 12 {
			shown = shown[:12]
		}
		entries = append(entries, narratordigest.Entry{
			Kind: "highlight", Text: "A landing moved the repository storyline to commit " + shown + ".",
			SourceType: "commit", SourceID: commit,
		})
	}
	for _, stop := range result.GoalStops {
		source := stop.StopID
		if source == "" {
			source = fmt.Sprintf("breach-%s-r%d-%s", stop.GoalID, stop.Revision, strings.ToLower(stop.State))
		}
		entries = append(entries, narratordigest.Entry{
			Kind: "lowlight", Text: fmt.Sprintf("A budget breach-stop for goal %s revision %d reached %s.", stop.GoalID, stop.Revision, strings.ToLower(stop.State)),
			SourceType: "episode", SourceID: source,
		})
	}
	if result.Decision.Action == ActNotify {
		entries = append(entries, narratordigest.Entry{
			Kind: "lowlight", Text: "The steward escalated: " + result.Decision.Reason + ".",
			SourceType: "episode", SourceID: "escalation-" + string(result.Decision.Verdict) + "-" + result.Evidence.Marks.HeadOid,
		})
	}
	if result.Decision.Action == ActRevive {
		entries = append(entries, narratordigest.Entry{
			Kind: "highlight", Text: "The steward started a revival after proving the prior worker dead.",
			SourceType: "episode", SourceID: fmt.Sprintf("revival-%s-%d", result.Evidence.Marks.HeadOid, result.Evidence.DryRevivals),
		})
	}
	return narratordigest.Append(repoRoot, entries, now)
}

// NarrateHealthLine durably appends the typed one-line health verdict. Unlike
// the optional running account above, this line is a mandatory tick result: a
// tick cannot claim success when its health narration did not land.
func NarrateHealthLine(repoRoot, line string) error {
	if strings.TrimSpace(line) == "" {
		return fmt.Errorf("the health narration line is empty")
	}
	return appendNarrationLine(repoRoot, line)
}

func appendNarrationLine(repoRoot, line string) error {
	path := NarrationPath(repoRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	existing, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimRight(string(existing), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	lines = append(lines, line)
	if len(lines) > narrationCapLines {
		lines = lines[len(lines)-narrationCapLines:]
	}
	durable, err := atomicfile.WriteText(path, strings.Join(lines, "\n")+"\n", repoRoot)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("the narration was published with durability unknown")
	}
	return nil
}

// narrationLine composes the sentence: when, who, what the machine is
// doing, and anything a person would want to know about this tick —
// in the narrator's plain-English register, no identifiers a reader
// would have to look up.
func narrationLine(repoRoot string, result TickResult, cfg TickConfig, now time.Time) string {
	machine := "this machine"
	if name, err := goal.ResolveMachine(repoRoot); err == nil {
		machine = name
	}
	var doing string
	switch {
	case strings.HasPrefix(result.OpenWork, "claimed goal: "):
		doing = "working on " + strings.TrimPrefix(result.OpenWork, "claimed goal: ")
	case strings.HasPrefix(result.OpenWork, "current goal: "):
		doing = "working on " + strings.TrimPrefix(result.OpenWork, "current goal: ")
	case result.OpenWork == "goal-free declared":
		doing = "at rest — the all-clear is declared"
	case strings.Contains(result.OpenWork, "queued goals await"):
		doing = "idle while work waits in the queue (" + result.OpenWork + ")"
	case result.OpenWork == "":
		doing = "observing"
	default:
		doing = result.OpenWork
	}
	var notes []string
	if result.ProviderOutage {
		notes = append(notes, "the model provider is overloaded; local work continues; the clocks are paused")
	}
	if len(result.Reaped) > 0 {
		notes = append(notes, fmt.Sprintf("closed %d finished helper run(s)", len(result.Reaped)))
	}
	for _, stop := range result.GoalStops {
		if stop.State == "COMPLETE" {
			notes = append(notes, fmt.Sprintf("breach-stop completed for %s revision %d", stop.GoalID, stop.Revision))
		} else {
			notes = append(notes, fmt.Sprintf("breach-stop for %s revision %d is %s", stop.GoalID, stop.Revision, strings.ToLower(stop.State)))
		}
	}
	if result.Decision.Action == ActNotify {
		notes = append(notes, "flagged something for the operator: "+result.Decision.Reason)
	}
	if result.Decision.Action == ActRevive {
		notes = append(notes, "reviving stalled work: "+result.Decision.Reason)
	}
	notes = append(notes, noticingLines(noticings(result, cfg))...)
	sentence := now.Format("2006-01-02 15:04") + "  " + machine + " is " + doing
	if len(notes) > 0 {
		sentence += "; " + strings.Join(notes, "; ")
	}
	return sentence + "."
}

// noticings names drift the patience vocabulary can see building —
// BEFORE the steward acts on it. A reader of the account watches a
// stall approach instead of learning about it from the intervention;
// once the decision itself acts, its own note speaks and these stay
// quiet.
// A Noticing is one named building anomaly: the key deduplicates it
// on the human channel (one pending message per building condition,
// not one per tick), the line is its plain-English sentence.
type Noticing struct {
	Key  string
	Line string
}

func noticingLines(items []Noticing) []string {
	var out []string
	for _, n := range items {
		out = append(out, n.Line)
	}
	return out
}

// longOutageAfter is when a standing provider outage stops being
// weather to wait out and becomes something the operator should hear
// about — well inside the mark's own horizon, so the alert fires
// while the outage is still provably standing.
const longOutageAfter = 10 * time.Minute

func noticings(result TickResult, cfg TickConfig) []Noticing {
	cfg = cfg.withDefaults()
	var out []Noticing
	// The long-outage alert is provider-INDEPENDENT by construction:
	// it rides the durable notify queue, and it must not be silenced
	// by whatever this tick's decision was — a held revival is exactly
	// when the operator most needs to hear the outage is still on.
	if result.ProviderOutage {
		if since, err := time.Parse(time.RFC3339, result.Outage.Since); err == nil {
			if age := time.Since(since); age >= longOutageAfter {
				out = append(out, Noticing{
					Key: "provider-outage-standing",
					Line: fmt.Sprintf(
						"noticing: the model provider has been overloaded for %d minutes (%d failure(s) recorded); local work continues and the clocks stay paused — a long outage is worth a look at the provider's status",
						int(age.Minutes()), result.Outage.ConsecutiveFailures),
				})
			}
		}
	}
	if result.Decision.Action != ActNone {
		return out
	}
	age := result.Evidence.TicksSinceAdvance
	working := strings.HasPrefix(result.OpenWork, "claimed goal: ") || strings.HasPrefix(result.OpenWork, "current goal: ")
	if working && age >= (cfg.StaleTicks+1)/2 && age < cfg.StaleTicks {
		out = append(out, Noticing{
			Key: "stall-approaching",
			Line: fmt.Sprintf(
				"noticing: no visible progress for %d checks in a row — watching, not yet acting (the steward steps in at %d)",
				age, cfg.StaleTicks),
		})
	}
	if result.Evidence.DryRevivals > 0 && result.Evidence.DryRevivals < cfg.MaxRevivals {
		out = append(out, Noticing{
			Key: "revivals-building",
			Line: fmt.Sprintf(
				"noticing: %d revival(s) so far without real progress (the steward stops trying at %d and calls the operator)",
				result.Evidence.DryRevivals, cfg.MaxRevivals),
		})
	}
	return out
}

// ReachTheHuman queues each noticing on the delivery-gated channel,
// one pending message per noticing key: the phone hears that a stall
// is building exactly once while it builds, and again only if it
// resolves and later rebuilds (delivery clears the pending slot).
// Best-effort like the narration itself: the messenger never fails
// the tick.
func ReachTheHuman(repoRoot string, items []Noticing) {
	for _, n := range items {
		_ = QueueNotification(repoRoot, PendingNotification{
			Nonce:   "noticing-" + n.Key,
			Message: "narrator: " + n.Line,
		})
	}
}
