package goal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/retrodebt"
)

const (
	RatifierHuman = "human"
	RatifierMain  = "main"
)

// MemberDraft is one parsed, ratifiable member definition. Authority-owned
// fields are deliberately absent: Split computes them from the parent.
type MemberDraft struct {
	ID       string
	Intent   string
	NextStep string
	Blocked  []string
	Labels   []string
}

// SplitRatification binds the authenticated ratifier to the canonical member
// draft. Human ratification uses By; main ratification uses lease coordinates.
type SplitRatification struct {
	Tier        string
	By          string
	MainID      string
	ClaimEpoch  int64
	DraftSHA256 string
}

func (r SplitRatification) Validate() error {
	if !hexDigest(r.DraftSHA256) {
		return fmt.Errorf("draftSha256 is not a lowercase sha256 digest")
	}
	switch r.Tier {
	case RatifierHuman:
		if r.By == "" || r.MainID != "" || r.ClaimEpoch != 0 {
			return fmt.Errorf("tier=human requires by and forbids mainId and claimEpoch")
		}
	case RatifierMain:
		if r.By != "" || r.MainID == "" || r.ClaimEpoch < 1 {
			return fmt.Errorf("tier=main requires mainId and a positive claimEpoch and forbids by")
		}
	default:
		return fmt.Errorf("tier %q is not human|main", r.Tier)
	}
	return nil
}

// ParseMemberDraft parses the closed split-draft grammar. It intentionally
// does not accept computed GoalFile fields.
func ParseMemberDraft(data []byte, parentID string) ([]MemberDraft, error) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "# split "+parentID {
		return nil, fmt.Errorf("split draft heading must be exactly # split %s", parentID)
	}
	var members []MemberDraft
	var current *MemberDraft
	seenFields := map[string]bool{}
	finish := func() error {
		if current == nil {
			return nil
		}
		if current.Intent == "" || current.NextStep == "" {
			return fmt.Errorf("member %s needs one-line Intent and Next step fields", current.ID)
		}
		canonical, err := canonicalLabels(current.Labels)
		if err != nil {
			return fmt.Errorf("member %s: %w", current.ID, err)
		}
		current.Labels = canonical
		current.Blocked = sortedUnique(current.Blocked)
		members = append(members, *current)
		return nil
	}
	for lineNo, raw := range lines[1:] {
		line := strings.TrimRight(raw, " \t")
		switch {
		case strings.TrimSpace(line) == "":
			continue
		case strings.HasPrefix(line, "## member "):
			if err := finish(); err != nil {
				return nil, err
			}
			id := strings.TrimSpace(strings.TrimPrefix(line, "## member "))
			if !validId(id) {
				return nil, fmt.Errorf("split draft line %d has invalid member id %q", lineNo+2, id)
			}
			current = &MemberDraft{ID: id}
			seenFields = map[string]bool{}
		case strings.HasPrefix(line, "- "):
			if current == nil {
				return nil, fmt.Errorf("split draft line %d has a field before its member heading", lineNo+2)
			}
			key, value, ok := strings.Cut(strings.TrimPrefix(line, "- "), ":")
			if !ok {
				return nil, fmt.Errorf("split draft line %d is not a field", lineNo+2)
			}
			if seenFields[key] {
				return nil, fmt.Errorf("member %s repeats field %s", current.ID, key)
			}
			seenFields[key] = true
			value = strings.TrimSpace(value)
			switch key {
			case "Intent":
				current.Intent = value
			case "Next step":
				current.NextStep = value
			case "BlockedBy":
				current.Blocked = draftCommaValues(value)
			case "Labels":
				current.Labels = draftCommaValues(value)
			default:
				return nil, fmt.Errorf("member %s has unknown or computed field %q", current.ID, key)
			}
		default:
			return nil, fmt.Errorf("split draft line %d is outside the closed grammar: %q", lineNo+2, line)
		}
	}
	if err := finish(); err != nil {
		return nil, err
	}
	if len(members) < 2 {
		return nil, fmt.Errorf("a split needs at least two members; one member is a rename")
	}
	seenIDs := map[string]bool{}
	for _, member := range members {
		if seenIDs[member.ID] {
			return nil, fmt.Errorf("split draft repeats member id %s", member.ID)
		}
		seenIDs[member.ID] = true
	}
	return members, nil
}

func draftCommaValues(value string) []string {
	if value == "" || value == "-" {
		return nil
	}
	parts := strings.Split(value, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// RenderMemberDraft is the canonical serialization used by the ratification
// digest and by recovery. Member ordering is lexical by id.
func RenderMemberDraft(parentID string, members []MemberDraft) []byte {
	canonical := append([]MemberDraft(nil), members...)
	sort.Slice(canonical, func(i, j int) bool { return canonical[i].ID < canonical[j].ID })
	var b strings.Builder
	fmt.Fprintf(&b, "# split %s\n", parentID)
	for _, member := range canonical {
		fmt.Fprintf(&b, "\n## member %s\n", member.ID)
		fmt.Fprintf(&b, "- Intent: %s\n", member.Intent)
		fmt.Fprintf(&b, "- Next step: %s\n", member.NextStep)
		if len(member.Blocked) > 0 {
			fmt.Fprintf(&b, "- BlockedBy: %s\n", strings.Join(sortedUnique(member.Blocked), ", "))
		}
		if len(member.Labels) > 0 {
			fmt.Fprintf(&b, "- Labels: %s\n", strings.Join(sortedUnique(member.Labels), ", "))
		}
	}
	return []byte(b.String())
}

// SplitDraftSHA256 returns the digest bound into a split ratification.
func SplitDraftSHA256(parentID string, members []MemberDraft) string {
	sum := sha256.Sum256(RenderMemberDraft(parentID, members))
	return hex.EncodeToString(sum[:])
}

func sortedUnique(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		if value != "" {
			set[value] = true
		}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// Split atomically replaces a parent with a member arc and the permanent
// decomposition record.
func Split(r VerbRequest, parentID string, members []MemberDraft, ratification SplitRatification, proof *humanauthority.Proof) (PublishResult, error) {
	req, err := splitRequest(r, parentID, members, ratification, proof)
	if err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, req)
}

func splitRequest(r VerbRequest, parentID string, members []MemberDraft, ratification SplitRatification, proof *humanauthority.Proof) (PublishRequest, error) {
	parsed, err := ParseMemberDraft(RenderMemberDraft(parentID, members), parentID)
	if err != nil {
		return PublishRequest{}, err
	}
	canonicalMembers := string(RenderMemberDraft(parentID, parsed))
	args := intentArgs(r, map[string]string{
		"members": canonicalMembers, "ratifierTier": ratification.Tier,
		"ratifierMainId": ratification.MainID, "ratifierClaimEpoch": strconv.FormatInt(ratification.ClaimEpoch, 10),
		"draftSha256": ratification.DraftSHA256,
	})
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "split", Targets: []string{parentID}, Args: args},
		Message: "goal split " + parentID,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			if archived := t.Done[parentID]; archived != nil {
				if opidLanded(archived, r) {
					return nil, AlreadyApplied{}
				}
				return nil, fmt.Errorf("goal %s is in the archive; there is nothing to split", parentID)
			}
			parent := t.Live[parentID]
			if parent == nil {
				return nil, fmt.Errorf("goal %s does not exist", parentID)
			}
			if parent.Sliced != nil {
				return nil, fmt.Errorf("GOAL_SPLIT_REFUSED: goal %s recorded its first slice (machine %s, revision %d, %s); split is a before-slicing act and slicing has begun — conclude the goal and open successor goals instead", parentID, parent.Sliced.Machine, parent.Sliced.Revision, parent.Sliced.At)
			}
			if _, retired := rootDecomposed(t.Root, parentID); retired {
				return nil, fmt.Errorf("goal %s is registered as decomposed and cannot split again", parentID)
			}
			if err := validateSplitParent(parent, r); err != nil {
				return nil, err
			}
			if err := validateSplitRatification(r, parent, parsed, ratification, proof); err != nil {
				return nil, err
			}
			if err := validateSplitMembers(t, parentID, parsed); err != nil {
				return nil, err
			}

			memberIDs := make([]string, 0, len(parsed))
			for _, member := range parsed {
				memberIDs = append(memberIDs, member.ID)
			}
			sort.Strings(memberIDs)
			targets := append([]string{parentID}, memberIDs...)
			changes := make([]Change, 0, len(parsed)+4)
			for _, draft := range parsed {
				labels, _ := canonicalLabels(append(append([]string{}, parent.Labels...), draft.Labels...))
				member := &GoalFile{
					Id: draft.ID, State: StateQueued, Intent: draft.Intent, Origin: parent.Origin,
					NextStep: draft.NextStep, OpenedAt: r.stamp(), Blocked: sortedUnique(append(append([]string{}, parent.Blocked...), draft.Blocked...)),
					Labels: labels, Arc: parentID, Pinned: parent.Pinned,
				}
				touch(member, r, "split", targets)
				changes = append(changes, Change{Path: livePath(member.Id), Content: RenderFile(member)})
			}
			for _, id := range sortedGoalIds(t.Live) {
				if id == parentID || !containsString(t.Live[id].Blocked, parentID) {
					continue
				}
				dependent := t.Live[id]
				rewritten := make([]string, 0, len(dependent.Blocked)+len(memberIDs))
				for _, blocker := range dependent.Blocked {
					if blocker != parentID {
						rewritten = append(rewritten, blocker)
					}
				}
				dependent.Blocked = sortedUnique(append(rewritten, memberIDs...))
				touch(dependent, r, "split", targets)
				changes = append(changes, Change{Path: livePath(id), Content: RenderFile(dependent)})
			}

			parent.State = StateDone
			parent.Conclude = "decomposed into arc " + parentID + ": " + goalPointers(memberIDs)
			parent.Blocked = nil
			parent.Parked = nil
			if err := clearClaimBinding(parent); err != nil {
				return nil, err
			}
			parent.Ratified = &SplitRatification{Tier: ratification.Tier, By: ratification.By, MainID: ratification.MainID, ClaimEpoch: ratification.ClaimEpoch, DraftSHA256: ratification.DraftSHA256}
			touch(parent, r, "split", targets)
			changes = append(changes, Change{Path: livePath(parentID), Delete: true}, Change{Path: donePath(parentID), Content: RenderFile(parent)})

			t.Root.Free = nil
			t.Root.Decomposed = append(t.Root.Decomposed, DecomposedEntry{Id: parentID, Opid: r.opid(), At: r.stamp(), OldArc: parent.Arc})
			t.Root.Revision++
			t.Root.History = append(t.Root.History, HistoryLine{At: r.stamp(), Opid: r.opid(), Verb: "split", Actor: r.Actor.historyActor(), Targets: targets, Keep: -1})
			changes = append(changes, Change{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)})
			return ackDisplacements(t, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
		AfterConfirmed: func(tip string) error {
			return raiseSplitOldArcDebt(r.Endpoint, tip, parentID, r.opid(), r.Now)
		},
	}, nil
}

func validateSplitParent(parent *GoalFile, r VerbRequest) error {
	switch parent.State {
	case StateQueued, StateApproved:
		return nil
	case StateClaimed:
		if ownPair(parent.Claimed, r.Actor) {
			return nil
		}
		return fmt.Errorf("goal %s is claimed by %s+%s; whether its slicing has started is that machine's job-record truth — park or steal it first, then split", parent.Id, parent.Claimed.Machine, parent.Claimed.Lineage)
	case StateParked:
		if r.Actor.Human == "" {
			return fmt.Errorf("goal %s is parked; splitting it is a human act", parent.Id)
		}
		return nil
	default:
		return fmt.Errorf("goal %s is %s and cannot split", parent.Id, parent.State)
	}
}

func validateSplitRatification(r VerbRequest, parent *GoalFile, members []MemberDraft, ratification SplitRatification, proof *humanauthority.Proof) error {
	if err := ratification.Validate(); err != nil {
		return fmt.Errorf("SPLIT_RATIFY_REFUSED: %w", err)
	}
	wantDigest := SplitDraftSHA256(parent.Id, members)
	if ratification.DraftSHA256 != wantDigest {
		return fmt.Errorf("SPLIT_RATIFY_REFUSED: the ratified draft digest %.8s does not match the member definitions being published (%.8s); re-run goal split with the ratified draft", ratification.DraftSHA256, wantDigest)
	}
	if ratification.Tier == RatifierHuman {
		if r.Actor.Human == "" || ratification.By != r.Actor.Human || proof == nil || !proof.ValidFor(r.Endpoint.Root) {
			return fmt.Errorf("SPLIT_RATIFY_REFUSED: a human ratification requires --by and fresh enrolled-terminal proof")
		}
		return nil
	}
	if parent.Origin != OriginMain {
		return fmt.Errorf("SPLIT_RATIFY_REFUSED: goal %s is human-origin; its split draft requires enrolled-human ratification", parent.Id)
	}
	return nil
}

func validateSplitMembers(t *TreeGoals, parentID string, members []MemberDraft) error {
	memberSet := map[string]bool{}
	for _, member := range members {
		if memberSet[member.ID] {
			return fmt.Errorf("split draft repeats member id %s", member.ID)
		}
		memberSet[member.ID] = true
		if t.Live[member.ID] != nil || t.Done[member.ID] != nil {
			return fmt.Errorf("split member id %s collides with an existing goal", member.ID)
		}
		if retired, ok := rootDecomposed(t.Root, member.ID); ok {
			return fmt.Errorf("split member id %s is retired by decomposition %s", member.ID, retired.Opid)
		}
	}
	var arcUsers []string
	for id, goal := range t.Live {
		if id != parentID && goal.Arc == parentID {
			arcUsers = append(arcUsers, id)
		}
	}
	for id, goal := range t.Done {
		if id != parentID && goal.Arc == parentID {
			arcUsers = append(arcUsers, id)
		}
	}
	if len(arcUsers) > 0 {
		sort.Strings(arcUsers)
		return fmt.Errorf("arc %s is already in use by %s; a split arc must be born empty", parentID, strings.Join(arcUsers, ", "))
	}
	for _, member := range members {
		for _, blocker := range member.Blocked {
			if !memberSet[blocker] && t.Live[blocker] == nil && t.Done[blocker] == nil {
				return fmt.Errorf("split member %s BlockedBy names a goal that does not exist: %s", member.ID, blocker)
			}
		}
	}
	return nil
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func goalPointers(ids []string) string {
	pointers := make([]string, len(ids))
	for i, id := range ids {
		pointers[i] = "goal:" + id
	}
	return strings.Join(pointers, ", ")
}

func raiseSplitOldArcDebt(e Endpoint, tip, parentID, opid string, now time.Time) error {
	tree, err := loadTree(e.Root, tip)
	if err != nil {
		return fmt.Errorf("classify split's old-arc retro debt: %w", err)
	}
	parent := tree.Done[parentID]
	oldArc := ""
	if parent != nil {
		oldArc = parent.Arc
	} else {
		decomposed, found := rootDecomposed(tree.Root, parentID)
		if !found || decomposed.Opid != opid {
			return fmt.Errorf("classify split's old-arc retro debt: archived parent %s is absent and its decomposition registry entry does not authenticate split %s", parentID, opid)
		}
		oldArc = decomposed.OldArc
	}
	if oldArc == "" {
		return nil
	}
	for _, live := range tree.Live {
		if live.Arc == oldArc {
			return nil
		}
	}
	if _, err := retrodebt.Raise(e.Root, retrodebt.KindArc, oldArc+":"+opid, now); err != nil {
		return fmt.Errorf("raise split's old-arc retro debt: %w", err)
	}
	return nil
}
