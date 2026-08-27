package goal

// The canonical projection: every read — list, next, the
// open-work verdict — comes from the ACCEPTED ref's tree, never the
// checkout. Offline reads work from the last accepted tree with a
// staleness banner past the threshold; the sync mode is a durable
// identity (the root record's word against the clone's config,
// mismatch refused by name); single-machine mode banners itself and
// names backlog-local-promotion as the only exit.

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// Projection is one read of the accepted world.
type Projection struct {
	Tip             string
	Tree            *TreeGoals
	Banners         []string
	AppetiteBanners []AppetiteBanner
}

// AppetiteBand is the computed checkpoint state of one standing claim.
type AppetiteBand string

const (
	BandWithin         AppetiteBand = "WITHIN-BAND"
	BandBreachEscalate AppetiteBand = "BREACH-ESCALATE"
	BandBreachStop     AppetiteBand = "BREACH-STOP"
)

// AppetiteBanner retains the goal and claimant coordinates beside the human
// text so callers can filter without parsing a sentence.
type AppetiteBanner struct {
	GoalId    string
	Machine   string
	Lineage   string
	Band      AppetiteBand
	Text      string
	Elapsed   time.Duration
	Appetite  time.Duration
	Remaining time.Duration
}

// CurrentAppetiteBanners is the read-only surface shared by commands and
// hooks. A legacy checkout has no synced claim events and is silently empty.
func CurrentAppetiteBanners(root string, now time.Time) ([]AppetiteBanner, error) {
	if !NewWorld(root) {
		return nil, nil
	}
	endpoint, err := ResolveEndpoint(root)
	if err != nil {
		return nil, err
	}
	projection, err := Project(endpoint, false, now)
	if err != nil {
		return nil, err
	}
	return projection.AppetiteBanners, nil
}

// StaleThreshold is how old an accepted tree may grow before the
// projection banners its staleness.
const StaleThreshold = 30 * time.Minute

// Project reads the accepted tree. With fetchFirst, the read-side
// validator runs before the read (the --fetch flag); otherwise the
// read is offline-capable and banners staleness.
func Project(e Endpoint, fetchFirst bool, now time.Time) (Projection, error) {
	if fetchFirst {
		if _, err := FetchAdvance(e); err != nil {
			return Projection{}, err
		}
	}
	tipOut, err := goalGit(e.Root, nil, "rev-parse", "--verify", "--quiet", AcceptedRef)
	if err != nil {
		return Projection{}, fmt.Errorf("no accepted tree; the first fetch or the migration bootstraps it")
	}
	tip := strings.TrimSpace(tipOut)
	tree, err := loadTree(e.Root, tip)
	if err != nil {
		return Projection{}, err
	}
	p := Projection{Tip: tip, Tree: tree}

	// The durable sync-mode identity: the root record's word against
	// the clone's config. A local-mode ledger with a remote config is
	// the forbidden promotion; a remote-mode ledger pointed at local
	// is a split-brain risk. Both refuse by name.
	if tree.Root != nil {
		recordMode := tree.Root.SyncMode
		configLocal := e.LocalMode()
		if recordMode == SyncLocal && !configLocal {
			return Projection{}, fmt.Errorf("sync-mode mismatch refused: the ledger is committed local, the config says remote %q — promotion is the backlog-local-promotion goal, not a config flip", e.Remote)
		}
		if recordMode == SyncRemote && configLocal {
			return Projection{}, fmt.Errorf("sync-mode mismatch refused: the ledger is committed remote, the config says local — a split brain is not a mode")
		}
		if recordMode == SyncLocal {
			p.Banners = append(p.Banners, "single-machine mode: multi-machine guarantees are void here; joining a fleet is the backlog-local-promotion goal")
		}
	}

	// Appetite checkpoints are computed from the claim-time snapshot and
	// authority-bearing history. Current prose alone is never authority.
	grace := config.AppetiteOverrunGracePercent(filepath.Join(e.Root, "metasystem.conf"))
	for _, id := range sortedGoalIds(tree.Live) {
		f := tree.Live[id]
		if f.State != StateClaimed || f.Claimed == nil {
			continue
		}
		appetite, declared := effectiveAppetite(e, tip, f)
		if !declared {
			continue
		}
		claimedAt, tErr := time.Parse(time.RFC3339, f.Claimed.At)
		if tErr != nil {
			continue
		}
		remaining, _ := latestRemainingAfterClaim(f)
		age := now.Sub(claimedAt)
		band := EvaluateAppetiteBand(age, appetite, remaining, grace)
		if band != BandWithin {
			banner := appetiteBanner(id, f.Claimed, band, age, appetite, remaining, grace)
			p.AppetiteBanners = append(p.AppetiteBanners, banner)
			p.Banners = append(p.Banners, banner.Text)
		}
	}

	// Staleness: the accepted COMMIT's age is the tree's age.
	if ageOut, err := goalGit(e.Root, nil, "log", "-1", "--format=%ct", tip); err == nil {
		if seconds := strings.TrimSpace(ageOut); seconds != "" {
			var epoch int64
			if _, scanErr := fmt.Sscanf(seconds, "%d", &epoch); scanErr == nil {
				age := now.Sub(time.Unix(epoch, 0))
				if age > StaleThreshold {
					p.Banners = append(p.Banners, fmt.Sprintf("the accepted tree is %s old; goal list --fetch validates and advances it", age.Round(time.Minute)))
				}
			}
		}
	}
	return p, nil
}

// ParseAppetite reads the appetite convention: the next step OPENS
// with "Appetite: <duration>" where the duration's first token is
// machine-comparable — 4h, 1d (a day is eight working hours), 30m.
// Prose after the token is welcome; prose INSTEAD of a token means
// no enforceable appetite is declared.
func ParseAppetite(nextStep string) (time.Duration, bool) {
	const prefix = "Appetite:"
	trimmed := strings.TrimSpace(nextStep)
	if !strings.HasPrefix(trimmed, prefix) {
		return 0, false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	token := rest
	if sp := strings.IndexAny(rest, " \t.,;"); sp >= 0 {
		token = rest[:sp]
	}
	return ParseWorkingDuration(token)
}

// ParseWorkingDuration accepts one or more positive integer day, hour, and
// minute segments. A working day is eight hours; seconds and fractions are not
// part of the ledger grammar.
func ParseWorkingDuration(value string) (time.Duration, bool) {
	if value == "" {
		return 0, false
	}
	var total time.Duration
	for i := 0; i < len(value); {
		start := i
		for i < len(value) && value[i] >= '0' && value[i] <= '9' {
			i++
		}
		if start == i || i >= len(value) {
			return 0, false
		}
		n, err := strconv.ParseInt(value[start:i], 10, 64)
		if err != nil || n <= 0 {
			return 0, false
		}
		var unit time.Duration
		switch value[i] {
		case 'm':
			unit = time.Minute
		case 'h':
			unit = time.Hour
		case 'd':
			unit = 8 * time.Hour
		default:
			return 0, false
		}
		if time.Duration(n) > (time.Duration(1<<63-1)-total)/unit {
			return 0, false
		}
		total += time.Duration(n) * unit
		i++
	}
	return total, total > 0
}

// FormatWorkingDuration is the canonical token stored in claim and estimate
// events. It preserves the working-day convention and never emits seconds.
func FormatWorkingDuration(value time.Duration) string {
	minutes := int64(value / time.Minute)
	if minutes <= 0 {
		return ""
	}
	days := minutes / (8 * 60)
	minutes %= 8 * 60
	hours := minutes / 60
	minutes %= 60
	var b strings.Builder
	if days > 0 {
		fmt.Fprintf(&b, "%dd", days)
	}
	if hours > 0 {
		fmt.Fprintf(&b, "%dh", hours)
	}
	if minutes > 0 {
		fmt.Fprintf(&b, "%dm", minutes)
	}
	return b.String()
}

// EvaluateAppetiteBand owns the checkpoint math. Forecasts add a STOP path;
// they never subtract the measured elapsed-time path.
func EvaluateAppetiteBand(elapsed, appetite, remaining time.Duration, gracePercent int) AppetiteBand {
	if appetite <= 0 || elapsed < 0 {
		return BandWithin
	}
	// Build the grace term without multiplying a Duration by up to 200;
	// accepted durations can approach the int64 ceiling. Saturating preserves
	// the intended comparison instead of wrapping a very large appetite.
	grace := time.Duration(gracePercent)
	extra := (appetite/100)*grace + (appetite%100)*grace/100
	maxDuration := time.Duration(1<<63 - 1)
	limit := maxDuration
	if extra <= maxDuration-appetite {
		limit = appetite + extra
	}
	// Subtract from the already-proven limit instead of adding remaining to
	// elapsed; the latter can overflow for a large honest estimate.
	if elapsed > limit || (remaining > 0 && remaining > limit-elapsed) {
		return BandBreachStop
	}
	if elapsed > appetite {
		return BandBreachEscalate
	}
	return BandWithin
}

func appetiteBanner(id string, claim *ClaimRecord, band AppetiteBand, elapsed, appetite, remaining time.Duration, grace int) AppetiteBanner {
	used := FormatWorkingDuration(elapsed.Round(time.Minute))
	if used == "" {
		used = "under 1m"
	}
	text := ""
	switch band {
	case BandBreachEscalate:
		text = fmt.Sprintf("BREACH-ESCALATE: goal %s has used %s against its %s claim appetite (%s+%s); Wido decides whether to continue with an adjusted appetite or abandon it",
			id, used, FormatWorkingDuration(appetite), claim.Machine, claim.Lineage)
	case BandBreachStop:
		reason := fmt.Sprintf("%s elapsed", used)
		if remaining > 0 {
			reason += fmt.Sprintf(" plus a %s remaining estimate", FormatWorkingDuration(remaining))
		}
		text = fmt.Sprintf("BREACH-STOP: goal %s has %s against its %s appetite and %d percent grace band (%s+%s); new dispatch rounds wait for Wido's word or a goal estimate showing the work within-band",
			id, reason, FormatWorkingDuration(appetite), grace, claim.Machine, claim.Lineage)
	}
	return AppetiteBanner{GoalId: id, Machine: claim.Machine, Lineage: claim.Lineage,
		Band: band, Text: text, Elapsed: elapsed, Appetite: appetite, Remaining: remaining}
}

func claimHistoryIndex(f *GoalFile) int {
	if f.Claimed == nil {
		return -1
	}
	for i := len(f.History) - 1; i >= 0; i-- {
		if f.History[i].At == f.Claimed.At && f.History[i].Verb != "edit" && f.History[i].Verb != "estimate" {
			return i
		}
	}
	return -1
}

func latestRemainingAfterClaim(f *GoalFile) (time.Duration, bool) {
	for i := len(f.History) - 1; i > claimHistoryIndex(f); i-- {
		h := f.History[i]
		if h.Verb == "estimate" && h.Remaining != "" {
			return ParseWorkingDuration(h.Remaining)
		}
	}
	return 0, false
}

// effectiveAppetite starts from the claim snapshot, then recognizes only the
// latest human-attributed edit after that claim. The accepted Git revision
// carrying that event supplies the Next-step bytes to re-parse; later claimant
// edits therefore cannot borrow the human actor's authority.
func effectiveAppetite(e Endpoint, tip string, f *GoalFile) (time.Duration, bool) {
	base, ok := ParseWorkingDuration(f.Claimed.Appetite)
	if !ok {
		return 0, false
	}
	claimIndex := claimHistoryIndex(f)
	var humanEdit *HistoryLine
	for i := len(f.History) - 1; i > claimIndex; i-- {
		h := &f.History[i]
		if h.Verb == "edit" && strings.HasPrefix(h.Actor, "human:") {
			humanEdit = h
			break
		}
	}
	if humanEdit == nil {
		return base, true
	}
	out, err := goalGit(e.Root, nil, "rev-list", tip, "--", livePath(f.Id))
	if err != nil {
		return base, true
	}
	for _, commit := range strings.Fields(out) {
		data, showErr := goalGit(e.Root, nil, "show", commit+":"+livePath(f.Id))
		if showErr != nil {
			continue
		}
		revision, problems := ParseFile([]byte(data))
		if len(problems) > 0 || len(revision.History) == 0 {
			continue
		}
		last := revision.History[len(revision.History)-1]
		if last.Opid != humanEdit.Opid {
			continue
		}
		if raised, declared := ParseAppetite(revision.NextStep); declared {
			return raised, true
		}
		return base, true
	}
	return base, true
}

// NextVerdict is the frontier read the dispatcher and the steward
// consume: claimed-first, then the queued frontier, then blocked.
type NextVerdict struct {
	Claimed []string // this machine's claimed goals, sorted
	Ready   []string // queued with every blocker done
	Blocked []string // queued behind an open blocker
}

// Next computes the frontier for one machine from a projection.
func Next(p Projection, machine string, requiredLabels ...string) NextVerdict {
	v := NextVerdict{}
	t := p.Tree
	// An arc claims as one unit, so one member pinned elsewhere makes
	// EVERY member unclaimable on this machine — the whole arc leaves
	// this machine's frontier, not just the pinned member.
	foreignPinnedArc := map[string]bool{}
	for _, f := range t.Live {
		if f.Arc != "" && f.Pinned != "" && f.Pinned != machine {
			foreignPinnedArc[f.Arc] = true
		}
	}
	for _, id := range sortedGoalIds(t.Live) {
		f := t.Live[id]
		switch f.State {
		case StateClaimed:
			if f.Claimed != nil && f.Claimed.Machine == machine {
				v.Claimed = append(v.Claimed, id)
			}
		case StateQueued:
			if !MatchesLabels(f.Labels, requiredLabels) {
				continue
			}
			// A goal pinned to another machine — or any member of an
			// arc with such a member — is invisible to this machine's
			// frontier: it can never claim it, so reporting it ready
			// would hide genuinely claimable work behind it.
			if f.Pinned != "" && f.Pinned != machine {
				continue
			}
			if f.Arc != "" && foreignPinnedArc[f.Arc] {
				continue
			}
			ready := true
			for _, dep := range f.Blocked {
				if depState(t, dep) != StateDone {
					ready = false
					break
				}
			}
			if ready {
				v.Ready = append(v.Ready, id)
			} else {
				v.Blocked = append(v.Blocked, id)
			}
		}
	}
	return v
}
