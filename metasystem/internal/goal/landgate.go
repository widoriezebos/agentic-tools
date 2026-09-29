package goal

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// The landing gate (g1-s70, R-132-ui).
//
// A goal whose work is published is either small enough to land by itself or
// needs a person's word, and which one is two settings: the tier from which a
// person decides (landing.review.human-from-tier) and the grace time after
// which a smaller goal is eligible to land by itself (landing.review.
// auto-after). Every fact the gate reads is a line on the goal's history, so
// every seat — the holder that lands, the landing batch that publishes, the
// interface that shows the clock — reads the same thing after its sync:
//
//   - the human's word at the current branch tip: a clear-to-land verdict
//     (g1-s69) or a land-without-sitting decision with its reason (D4);
//   - a standing review sitting, which holds the goal at every tier: opened
//     by `goal review G --hold`, released by `--release` or by the same
//     human's verdict (D2);
//   - the clock, which starts at the Landing record and restarts at every
//     human act after it, and does not run while a hold stands (D3).
//
// The holder lands; nothing here lands anything. The seat that holds the claim
// reads LandingDue on its turn and runs the ordinary landing, which calls Gate
// again against the fresh ledger, and once the publication is confirmed it
// writes the landed line naming what let it land.

const (
	// LandWithoutSittingVerb is the human's decision to land without a sitting.
	LandWithoutSittingVerb = "land-without-sitting"
	// LandedVerb is the holder's line once a landing's publication is confirmed.
	LandedVerb = "landed"

	sittingOpenedPrefix   = "review-sitting opened "
	sittingReleasedPrefix = "review-sitting released "
	withoutSittingPrefix  = "landed-without-sitting "
	landedUnderPrefix     = "landed under "
	maxWithoutReasonRunes = 500
)

// The gate's refusal codes, one per missing fact; the register carries each.
const (
	GateWaitsForHuman = "LANDING_WAITS_FOR_HUMAN"
	GateHeldBySitting = "LANDING_HELD_BY_SITTING"
)

// GateSettings are the two settings as the layered resolution answered them.
type GateSettings struct {
	// HumanFromTier is the lowest tier that waits for a person.
	HumanFromTier uint8
	// AutoAfter is the grace time below the tier.
	AutoAfter time.Duration
	// AutoAfterText is AutoAfter as it is configured, for the history line.
	AutoAfterText string
}

// GateRefusal is why a landing is refused at the gate.
type GateRefusal struct {
	Code   string
	Reason string
}

func (r *GateRefusal) Error() string { return r.Code + ": " + r.Reason }

// Hold is one standing review sitting on a goal.
type Hold struct {
	By     string
	Record string
	At     string
}

// WithoutSitting is one land-without-sitting decision, read back.
type WithoutSitting struct {
	Opid   string
	At     string
	Tip    string
	By     string
	Reason string
}

// GateTier is the tier the gate reads: a tierless goal is read as tier 3, the
// migration's own conservative reading (tierBoxReviewRounds).
func GateTier(f *GoalFile) uint8 {
	if f == nil || f.Tier < 1 || f.Tier > 3 {
		return 3
	}
	return f.Tier
}

// WaitsForHuman says the goal is at or above the tier that waits for a person.
func (s GateSettings) WaitsForHuman(f *GoalFile) bool {
	return GateTier(f) >= s.HumanFromTier
}

// sittingReason reads a hold or release reason: the record and the human.
func sittingReason(reason string) (opened bool, record, by string, ok bool) {
	rest, isOpen := strings.CutPrefix(reason, sittingOpenedPrefix)
	if !isOpen {
		var isRelease bool
		if rest, isRelease = strings.CutPrefix(reason, sittingReleasedPrefix); !isRelease {
			return false, "", "", false
		}
	}
	fields, err := reasonFields(rest, "record", "by")
	if err != nil || !reviewRecordPath.MatchString(fields["record"]) || fields["by"] == "" {
		return false, "", "", false
	}
	return isOpen, fields["record"], fields["by"], true
}

// HoldsOf is every standing hold on a goal, oldest first: a sitting opened and
// not yet released by its human, by --release or by a verdict of the same
// human, which releases in the same line.
func HoldsOf(f *GoalFile) []Hold {
	if f == nil {
		return nil
	}
	var holds []Hold
	drop := func(by string) {
		holds = slices.DeleteFunc(holds, func(h Hold) bool { return h.By == by })
	}
	for _, h := range f.History {
		if h.Verb != reviewVerb {
			continue
		}
		if opened, record, by, ok := sittingReason(h.Reason); ok {
			drop(by)
			if opened {
				holds = append(holds, Hold{By: by, Record: record, At: h.At})
			}
			continue
		}
		if line, err := parseReviewReason(h.Reason); err == nil {
			drop(line.By)
		}
	}
	return holds
}

func parseWithoutSitting(reason string) (WithoutSitting, error) {
	rest, found := strings.CutPrefix(reason, withoutSittingPrefix)
	if !found {
		return WithoutSitting{}, fmt.Errorf("land-without-sitting history's reason opens with %q", strings.TrimSpace(withoutSittingPrefix))
	}
	head, why, found := strings.Cut(rest, " because=")
	if !found || strings.TrimSpace(why) == "" {
		return WithoutSitting{}, fmt.Errorf("land-without-sitting history carries the human's reason")
	}
	fields, err := reasonFields(head, "tip", "by")
	if err != nil {
		return WithoutSitting{}, fmt.Errorf("land-without-sitting history: %v", err)
	}
	if !reviewCommit.MatchString(fields["tip"]) || fields["by"] == "" {
		return WithoutSitting{}, fmt.Errorf("land-without-sitting history names the tip it was decided at and its human")
	}
	return WithoutSitting{Tip: fields["tip"], By: fields["by"], Reason: why}, nil
}

func (w WithoutSitting) reason() string {
	return withoutSittingPrefix + "tip=" + w.Tip + " by=" + w.By + " because=" + w.Reason
}

// validGateHistory is the grammar ParseHistoryLine holds the gate's lines to:
// a hold and a decision are a human's, the landed line is the holder's.
func validGateHistory(h HistoryLine) error {
	switch h.Verb {
	case LandWithoutSittingVerb:
		if !strings.HasPrefix(h.Actor, "human:") || len(h.Targets) != 1 {
			return fmt.Errorf("land-without-sitting history is a human's act on one goal")
		}
		_, err := parseWithoutSitting(h.Reason)
		return err
	case LandedVerb:
		if strings.HasPrefix(h.Actor, "human:") || len(h.Targets) != 1 {
			return fmt.Errorf("landed history is the holder's line on one goal")
		}
		if !strings.HasPrefix(h.Reason, landedUnderPrefix) {
			return fmt.Errorf("landed history names what it landed under")
		}
	}
	return nil
}

// word is the newest human word on the landing at hand: a verdict or a
// land-without-sitting decision, since the goal's standing Landing record (or
// over the whole history of a claim that never had one).
type word struct {
	kind    string // VerdictClearToLand, VerdictSendBack or LandWithoutSittingVerb
	tip     string
	by      string
	opid    string
	because string
	record  string
}

func since(f *GoalFile) int {
	if index := landingIndex(f); index >= 0 {
		return index + 1
	}
	return 0
}

func newestWord(f *GoalFile) *word {
	if f == nil {
		return nil
	}
	var newest *word
	for _, h := range f.History[since(f):] {
		switch h.Verb {
		case reviewVerb:
			if line, err := parseReviewReason(h.Reason); err == nil {
				newest = &word{kind: line.Verdict, tip: line.Tip, by: line.By, opid: h.Opid, record: line.Record}
			}
		case LandWithoutSittingVerb:
			if decided, err := parseWithoutSitting(h.Reason); err == nil {
				newest = &word{kind: LandWithoutSittingVerb, tip: decided.Tip, by: decided.By, opid: h.Opid, because: decided.Reason}
			}
		}
	}
	return newest
}

// Gate decides whether a goal's work at tip may be published now: refused
// under a standing hold at every tier; at or above the tier refused unless the
// newest human word on the landing is clear to land or land without a sitting,
// recorded against this tip; below the tier it proceeds. It answers the words
// the landed line carries: what the landing is under.
func Gate(f *GoalFile, tip string, s GateSettings) (string, error) {
	if f == nil {
		return "", &GateRefusal{Code: GateWaitsForHuman, Reason: "the goal is not live, so there is nothing its word could be read from"}
	}
	if holds := HoldsOf(f); len(holds) > 0 {
		return "", &GateRefusal{Code: GateHeldBySitting, Reason: fmt.Sprintf(
			"goal %s is held by %s's review sitting (%s); nothing lands while a sitting stands, whatever the tier; it lands once the sitting ends",
			f.Id, holds[0].By, holds[0].Record)}
	}
	tier := GateTier(f)
	if !s.WaitsForHuman(f) {
		return fmt.Sprintf("landing.review.auto-after=%s, tier %d below human-from-tier=%d", s.AutoAfterText, tier, s.HumanFromTier), nil
	}
	said := newestWord(f)
	missing := func(why string) error {
		return &GateRefusal{Code: GateWaitsForHuman, Reason: fmt.Sprintf(
			"goal %s is tier %d, at or above landing.review.human-from-tier=%d, and waits for a person: %s; hold a review sitting that ends clear to land, or land it without a sitting with a reason",
			f.Id, tier, s.HumanFromTier, why)}
	}
	switch {
	case said == nil:
		return "", missing("its history carries no clear-to-land verdict and no land-without-sitting decision")
	case said.kind == VerdictSendBack:
		return "", missing(fmt.Sprintf("its newest verdict is %s's send-back", said.by))
	case tip == "":
		return "", missing("this landing names no branch tip the word could be bound to")
	case said.tip != tip:
		return "", missing(fmt.Sprintf("%s's word was given at %s and the branch is now at %s; a moved tip needs the word again", said.by, short(said.tip), short(tip)))
	case said.kind == LandWithoutSittingVerb:
		return fmt.Sprintf("landed-without-sitting by=%s tip=%s, tier %d at or above human-from-tier=%d", said.by, short(tip), tier, s.HumanFromTier), nil
	}
	return fmt.Sprintf("reviewed verdict=clear-to-land by=%s tip=%s record=%s, tier %d at or above human-from-tier=%d", said.by, short(tip), said.record, tier, s.HumanFromTier), nil
}

// GateReading is what the card and the inbox say about one goal waiting to
// land, computed from its file and the settings alone; the interface never
// runs the clock.
type GateReading struct {
	Tier          uint8
	WaitsForHuman bool
	// ClockFrom is when the grace time started: the Landing record, or the
	// newest human act after it. Zero above the tier or under a hold.
	ClockFrom time.Time
	// AutoLandsAt is when a goal below the tier becomes eligible to land by
	// itself. Zero above the tier or under a hold.
	AutoLandsAt time.Time
	Eligible    bool
	HeldBy      []Hold
	// Word is the newest human word on the landing, as the card says it:
	// "clear-to-land", "send-back", "land-without-sitting", or "".
	Word   string
	WordBy string
	// WordTip is the tip the word was given at.
	WordTip string
	// Landed says the holder recorded the landing's confirmed publication.
	Landed bool
}

// ReadGate reads the gate for one goal at now.
func ReadGate(f *GoalFile, s GateSettings, now time.Time) GateReading {
	read := GateReading{Tier: GateTier(f), WaitsForHuman: s.WaitsForHuman(f), HeldBy: HoldsOf(f)}
	if f == nil {
		return read
	}
	if said := newestWord(f); said != nil {
		read.Word, read.WordBy, read.WordTip = said.kind, said.by, said.tip
	}
	for _, h := range f.History[since(f):] {
		if h.Verb == LandedVerb {
			read.Landed = true
		}
	}
	if read.WaitsForHuman || len(read.HeldBy) > 0 || f.Landing == nil {
		return read
	}
	start, err := time.Parse(time.RFC3339, f.Landing.At)
	if err != nil {
		return read
	}
	for _, h := range f.History[since(f):] {
		if !strings.HasPrefix(h.Actor, "human:") {
			continue
		}
		if at, err := time.Parse(time.RFC3339, h.At); err == nil && at.After(start) {
			start = at
		}
	}
	read.ClockFrom = start
	read.AutoLandsAt = start.Add(s.AutoAfter)
	read.Eligible = !now.Before(read.AutoLandsAt)
	return read
}

// LandingDue says the holder should land this goal on its turn now, and why:
// a goal waiting to land, not held, not sent back and not landed yet, that is
// eligible by the clock below the tier, or at or above it carries the human's
// word to land. The landing itself evaluates Gate against the fresh ledger and
// the branch tip; this only says it is worth trying. Key names the fact the
// due landing rests on, so a turn that has already been told of it is not told
// again.
func LandingDue(f *GoalFile, s GateSettings, now time.Time) (due bool, why, key string) {
	if f == nil || !f.IsLandingClaim() {
		return false, "", ""
	}
	read := ReadGate(f, s, now)
	if read.Landed || len(read.HeldBy) > 0 || read.Word == VerdictSendBack {
		return false, "", ""
	}
	if !read.WaitsForHuman {
		if !read.Eligible {
			return false, "", ""
		}
		return true, fmt.Sprintf("eligible under landing.review.auto-after=%s, tier %d below human-from-tier=%d", s.AutoAfterText, read.Tier, s.HumanFromTier),
			f.Id + "@" + read.ClockFrom.UTC().Format(time.RFC3339)
	}
	if read.Word == "" {
		return false, "", ""
	}
	said := newestWord(f)
	words := map[string]string{VerdictClearToLand: "cleared to land by " + said.by, LandWithoutSittingVerb: "to land without a sitting, by " + said.by}
	return true, words[said.kind] + " at " + short(said.tip), f.Id + "@" + said.opid
}

// LandWithoutSitting records one human's decision to land a goal at or above
// the tier without a sitting, bound to the branch tip it was decided at, with
// the reason the human wrote. It is a human act, from the enrolled terminal or
// a signed-in browser session; the reason is required.
func LandWithoutSitting(r VerbRequest, id, tip, because string, proof *humanauthority.Proof) (PublishResult, error) {
	if r.Actor.Human == "" {
		return PublishResult{}, fmt.Errorf("land without a sitting is a human act and names its human (--by)")
	}
	if proof == nil || !(proof.ValidFor(r.Endpoint.Root) || proof.SessionValidFor(r.Endpoint.Root)) {
		return PublishResult{}, fmt.Errorf("land without a sitting requires freshly observed enrolled-terminal human authority or a signed-in browser session")
	}
	if named := humanOfProof(r.Endpoint.Root, proof); named != "" && named != r.Actor.Human {
		return PublishResult{}, fmt.Errorf("the proof names %s and the act is attributed to %s; an act is recorded under the person who made it", named, r.Actor.Human)
	}
	decided, err := WithoutSittingLine(tip, r.Actor.Human, because)
	if err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, withoutSittingRequest(r, id, decided, proof))
}

// WithoutSittingLine is the decision a human's words make, or why they make
// none: the reason is required, one line, and the tip is a full commit.
func WithoutSittingLine(tip, by, because string) (WithoutSitting, error) {
	because = strings.TrimSpace(because)
	switch {
	case because == "":
		return WithoutSitting{}, fmt.Errorf("land without a sitting carries your reason: say why this goal needs no sitting")
	case strings.ContainsAny(because, "\r\n") || !utf8.ValidString(because) || utf8.RuneCountInString(because) > maxWithoutReasonRunes:
		return WithoutSitting{}, fmt.Errorf("the reason is one line of at most %d characters", maxWithoutReasonRunes)
	case !reviewCommit.MatchString(tip):
		return WithoutSitting{}, fmt.Errorf("the goal's branch has no tip to bind the decision to; a decision is recorded against the work it lets land")
	}
	return WithoutSitting{Tip: tip, By: by, Reason: because}, nil
}

func withoutSittingRequest(r VerbRequest, id string, decided WithoutSitting, proof *humanauthority.Proof) PublishRequest {
	reason := decided.reason()
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: LandWithoutSittingVerb, Targets: []string{id}, Args: intentArgs(r, map[string]string{"reason": reason})},
		Message: "goal land-without-sitting " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTreeFor(r.Endpoint, tip)
			if err != nil {
				return nil, err
			}
			f := t.Live[id]
			if f == nil {
				return nil, fmt.Errorf("goal %s is not live; a decision to land is recorded on a goal whose work waits to land", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.State != StateClaimed || f.Claimed == nil {
				return nil, fmt.Errorf("goal %s is not claimed (it is %s); a decision to land is recorded on a goal whose work waits to land", id, f.State)
			}
			if said := newestWord(f); said != nil && said.kind == LandWithoutSittingVerb && said.tip == decided.Tip {
				return nil, AlreadyHolds{Reason: "goal " + id + " already carries " + said.by + "'s decision to land without a sitting at " + short(decided.Tip)}
			}
			touch(f, r, LandWithoutSittingVerb, []string{id})
			event := &f.History[len(f.History)-1]
			event.Reason = reason
			recordSessionAuthority(event, proof)
			return ackDisplacements(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}}), nil
		},
		Validate: func(commit string) error { return validateCommitFor(r.Endpoint, commit) },
	}
}

// Sitting opens or releases one human's review sitting on a goal: the hold
// every seat reads, written before the room reports the sitting open, and
// released by every way the sitting ends. A release with no standing hold of
// this human's is a repeat that changes nothing.
func Sitting(r VerbRequest, id, record string, open bool, proof *humanauthority.Proof) (PublishResult, error) {
	if r.Actor.Human == "" {
		return PublishResult{}, fmt.Errorf("a review sitting is a human's and names its human (--by)")
	}
	if proof == nil || !(proof.ValidFor(r.Endpoint.Root) || proof.SessionValidFor(r.Endpoint.Root)) {
		return PublishResult{}, fmt.Errorf("a review sitting's hold requires freshly observed enrolled-terminal human authority or a signed-in browser session")
	}
	if named := humanOfProof(r.Endpoint.Root, proof); named != "" && named != r.Actor.Human {
		return PublishResult{}, fmt.Errorf("the proof names %s and the act is attributed to %s; an act is recorded under the person who made it", named, r.Actor.Human)
	}
	if !reviewRecordPath.MatchString(record) || strings.HasSuffix(record, ".brief.md") {
		return PublishResult{}, fmt.Errorf("%s is not a review record in its home; a review record is %s<name>.md", record, ReviewHome)
	}
	return Publish(r.Endpoint, sittingRequest(r, id, record, open, proof))
}

// SittingReason is the line a hold or a release writes.
func SittingReason(open bool, record, by string) string {
	if open {
		return sittingOpenedPrefix + "record=" + record + " by=" + by
	}
	return sittingReleasedPrefix + "record=" + record + " by=" + by
}

func sittingRequest(r VerbRequest, id, record string, open bool, proof *humanauthority.Proof) PublishRequest {
	reason := SittingReason(open, record, r.Actor.Human)
	word := map[bool]string{true: "hold", false: "release"}[open]
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: reviewVerb, Targets: []string{id}, Args: intentArgs(r, map[string]string{"reason": reason})},
		Message: "goal review " + id + " --" + word,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTreeFor(r.Endpoint, tip)
			if err != nil {
				return nil, err
			}
			f := t.Live[id]
			if f == nil {
				return nil, fmt.Errorf("goal %s is not live; a sitting holds a goal whose work waits to land", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			var mine *Hold
			for _, hold := range HoldsOf(f) {
				if hold.By == r.Actor.Human {
					mine = &hold
				}
			}
			switch {
			case open && mine != nil && mine.Record == record:
				return nil, AlreadyHolds{Reason: "goal " + id + " is already held by " + r.Actor.Human + "'s sitting " + record}
			case open && (f.State != StateClaimed || f.Claimed == nil):
				return nil, fmt.Errorf("goal %s is not claimed (it is %s); a sitting holds a goal whose work waits to land", id, f.State)
			case !open && mine == nil:
				return nil, AlreadyHolds{Reason: "goal " + id + " carries no standing sitting of " + r.Actor.Human + "'s"}
			}
			touch(f, r, reviewVerb, []string{id})
			event := &f.History[len(f.History)-1]
			event.Reason = reason
			recordSessionAuthority(event, proof)
			return ackDisplacements(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}}), nil
		},
		Validate: func(commit string) error { return validateCommitFor(r.Endpoint, commit) },
	}
}

// RecordLanded writes the holder's landed line once a landing's publication
// is confirmed, naming what it landed under. It is the claim holder's own
// act, once per Landing record.
func RecordLanded(r VerbRequest, id, under string) (PublishResult, error) {
	if r.Actor.Human != "" {
		return PublishResult{}, fmt.Errorf("the landed line is the seat's that holds the goal; it takes no --by")
	}
	if strings.TrimSpace(under) == "" || strings.ContainsAny(under, "\r\n") {
		return PublishResult{}, fmt.Errorf("the landed line names, on one line, what the landing was under")
	}
	return Publish(r.Endpoint, landedRequest(r, id, landedUnderPrefix+under))
}

func landedRequest(r VerbRequest, id, reason string) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: LandedVerb, Targets: []string{id}, Args: intentArgs(r, map[string]string{"reason": reason})},
		Message: "goal landed " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTreeFor(r.Endpoint, tip)
			if err != nil {
				return nil, err
			}
			f := t.Live[id]
			if f == nil {
				return nil, fmt.Errorf("goal %s is not live; its landing is recorded while it waits to be done", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.State != StateClaimed || f.Claimed == nil || !ownPair(f.Claimed, r.Actor) {
				return nil, fmt.Errorf("goal %s is not this seat's claim; its landing is recorded by the seat that holds it", id)
			}
			for _, h := range f.History[since(f):] {
				if h.Verb == LandedVerb {
					return nil, AlreadyHolds{Reason: "goal " + id + "'s landing is already recorded"}
				}
			}
			touch(f, r, LandedVerb, []string{id})
			f.History[len(f.History)-1].Reason = reason
			return ackDisplacements(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}}), nil
		},
		Validate: func(commit string) error { return validateCommitFor(r.Endpoint, commit) },
	}
}
