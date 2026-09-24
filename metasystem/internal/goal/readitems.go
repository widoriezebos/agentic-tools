package goal

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	ReadItemOpen     = "open"
	ReadItemFixed    = "fixed"
	ReadItemMoved    = "moved"
	ReadItemAccepted = "accepted"
)

// ReadItemBlock is the derived fix unit printed beside a goal's Next step.
type ReadItemBlock struct {
	Heading string     `json:"heading"`
	Items   []ReadItem `json:"items"`
}

func OpenReadItemBlocks(f *GoalFile) []ReadItemBlock {
	if f == nil {
		return nil
	}
	byRead := map[string][]ReadItem{}
	for _, item := range f.ReadItems {
		if item.State == ReadItemOpen {
			byRead[item.Read] = append(byRead[item.Read], item)
		}
	}
	labels := make([]string, 0, len(byRead))
	for label := range byRead {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	blocks := make([]ReadItemBlock, 0, len(labels))
	for _, label := range labels {
		items := byRead[label]
		blocks = append(blocks, ReadItemBlock{
			Heading: fmt.Sprintf("Open read items (fix unit %s): %d", label, len(items)),
			Items:   items,
		})
	}
	return blocks
}

func renderReadItem(b *strings.Builder, item ReadItem) {
	changed := item.ChangedAt
	if changed == "" {
		changed = "-"
	}
	fmt.Fprintf(b, "- ReadItem: id=%s read=%s state=%s addedAt=%s changedAt=%s closingReference=%s text=%s\n",
		item.ID, item.Read, item.State, item.AddedAt, changed, strconv.Quote(item.ClosingReference), strconv.Quote(item.Text))
}

func validateReadItem(item ReadItem) error {
	if !bareReviewID(item.Read) || strings.Contains(item.Read, "=") {
		return fmt.Errorf("read label must be one non-empty token")
	}
	prefix := item.Read + "-"
	n, err := strconv.ParseUint(strings.TrimPrefix(item.ID, prefix), 10, 64)
	if err != nil || n == 0 || !strings.HasPrefix(item.ID, prefix) {
		return fmt.Errorf("id must be %s<n>, with n starting at 1", prefix)
	}
	if strings.TrimSpace(item.Text) == "" || strings.ContainsAny(item.Text, "\r\n") {
		return fmt.Errorf("text must be one non-blank line")
	}
	if !utcStamp(item.AddedAt) {
		return fmt.Errorf("addedAt must be an RFC3339 UTC timestamp")
	}
	switch item.State {
	case ReadItemOpen:
		if item.ChangedAt != "" || item.ClosingReference != "" {
			return fmt.Errorf("open item cannot carry closing fields")
		}
	case ReadItemFixed, ReadItemMoved, ReadItemAccepted:
		if !utcStamp(item.ChangedAt) || strings.TrimSpace(item.ClosingReference) == "" || strings.ContainsAny(item.ClosingReference, "\r\n") {
			return fmt.Errorf("closed item needs a UTC changedAt and one-line closing reference")
		}
	default:
		return fmt.Errorf("state must be open, fixed, moved, or accepted")
	}
	return nil
}

func utcStamp(value string) bool {
	parsed, err := time.Parse(time.RFC3339, value)
	return err == nil && parsed.Location() == time.UTC && strings.HasSuffix(value, "Z")
}

func authorizeReadItemChange(f *GoalFile, r VerbRequest) (string, error) {
	if f.State == StateClaimed && !ownPair(f.Claimed, r.Actor) && r.Actor.Human == "" {
		return "", fmt.Errorf("goal %s is claimed by %s+%s; editing another's claimed goal is a human act", f.Id, f.Claimed.Machine, f.Claimed.Lineage)
	}
	if f.State == StateParked && r.Actor.Human == "" {
		return "", fmt.Errorf("goal %s is parked; editing a parked goal is a human act", f.Id)
	}
	if f.State == StateClaimed && f.Claimed != nil && !ownPair(f.Claimed, r.Actor) {
		return pairMarker(f.Claimed), nil
	}
	return "", nil
}

func nextReadItemID(items []ReadItem, label string) string {
	var largest uint64
	prefix := label + "-"
	for _, item := range items {
		if strings.HasPrefix(item.ID, prefix) {
			if n, err := strconv.ParseUint(strings.TrimPrefix(item.ID, prefix), 10, 64); err == nil && n > largest {
				largest = n
			}
		}
	}
	return fmt.Sprintf("%s%d", prefix, largest+1)
}

// AddReadItems records a read's non-breaking findings without rewriting NextStep.
func AddReadItems(r VerbRequest, id, label string, texts []string) (PublishResult, error) {
	if !bareReviewID(label) || strings.Contains(label, "=") {
		return PublishResult{}, fmt.Errorf("read label must be one non-empty token")
	}
	if len(texts) == 0 {
		return PublishResult{}, fmt.Errorf("read-items add needs at least one item")
	}
	for _, item := range texts {
		if strings.TrimSpace(item) == "" || strings.ContainsAny(item, "\r\n") {
			return PublishResult{}, fmt.Errorf("each read item must be one non-blank line")
		}
	}
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "read-items-add", Targets: []string{id}}, Message: "goal read-items add " + id,
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTreeFor(r.Endpoint, tip)
			if err != nil {
				return nil, err
			}
			file := tree.Live[id]
			if file == nil {
				return nil, fmt.Errorf("goal %s is not live", id)
			}
			if opidLanded(file, r) {
				return nil, AlreadyApplied{}
			}
			displaced, err := authorizeReadItemChange(file, r)
			if err != nil {
				return nil, err
			}
			added := 0
			for _, text := range texts {
				duplicate := false
				for _, existing := range file.ReadItems {
					if existing.Read == label && existing.Text == text {
						duplicate = true
						break
					}
				}
				if duplicate {
					continue
				}
				file.ReadItems = append(file.ReadItems, ReadItem{ID: nextReadItemID(file.ReadItems, label), Read: label, Text: text, State: ReadItemOpen, AddedAt: r.stamp()})
				added++
			}
			if added == 0 {
				return nil, NothingToDo{Reason: "all read items already tracked"}
			}
			touchDisplaced(file, r, "read-items-add", []string{id}, displaced)
			return ackDisplacements(tree, r, []Change{{Path: livePath(id), Content: RenderFile(file)}}), nil
		}, Validate: func(commit string) error { return validateCommitFor(r.Endpoint, commit) },
	})
}

// ReadItemClosure names exactly one terminal disposition.
type ReadItemClosure struct {
	Fixed    *string
	Moved    *string
	Accepted *string
}

func CloseReadItem(r VerbRequest, id, itemID string, closure ReadItemClosure) (PublishResult, error) {
	return closeReadItem(r, id, itemID, closure, resolveReadItemCodeCommit)
}

// CloseReadItemWithResolver keeps the ordinary close policy and publication
// while allowing a caller to supply the raw code commit lookup.
func CloseReadItemWithResolver(r VerbRequest, id, itemID string, closure ReadItemClosure, resolveCodeCommit func(root, ref string) (string, error)) (PublishResult, error) {
	return closeReadItem(r, id, itemID, closure, resolveCodeCommit)
}

func resolveReadItemCodeCommit(root, ref string) (string, error) {
	return gitIn(root, "rev-parse", "--verify", ref+"^{commit}")
}

func closeReadItem(r VerbRequest, id, itemID string, closure ReadItemClosure, resolveCodeCommit func(root, ref string) (raw string, err error)) (PublishResult, error) {
	ways := 0
	for _, present := range []bool{closure.Fixed != nil, closure.Moved != nil, closure.Accepted != nil} {
		if present {
			ways++
		}
	}
	if ways != 1 {
		return PublishResult{}, fmt.Errorf("read-items close needs exactly one of --fixed, --moved, or --accepted")
	}
	state, reference := "", ""
	switch {
	case closure.Fixed != nil:
		state, reference = ReadItemFixed, strings.TrimSpace(*closure.Fixed)
		commit, err := resolveCodeCommit(r.Endpoint.Root, reference)
		if err != nil || reference == "" {
			return PublishResult{}, fmt.Errorf("--fixed %q does not resolve to a commit object in this repository", reference)
		}
		reference = strings.TrimSpace(commit)
	case closure.Moved != nil:
		state, reference = ReadItemMoved, strings.TrimSpace(*closure.Moved)
		if reference == "" {
			return PublishResult{}, fmt.Errorf("--moved needs a goal id")
		}
	case closure.Accepted != nil:
		state, reference = ReadItemAccepted, strings.TrimSpace(*closure.Accepted)
		if reference == "" {
			return PublishResult{}, fmt.Errorf("--accepted needs a non-blank reason")
		}
	}
	requestTargets := []string{id}
	if state == ReadItemMoved {
		requestTargets = append(requestTargets, reference)
	}
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "read-items-close", Targets: requestTargets}, Message: "goal read-items close " + id,
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTreeFor(r.Endpoint, tip)
			if err != nil {
				return nil, err
			}
			file := tree.Live[id]
			if file == nil {
				return nil, fmt.Errorf("goal %s is not live", id)
			}
			if opidLanded(file, r) {
				return nil, AlreadyApplied{}
			}
			displaced, err := authorizeReadItemChange(file, r)
			if err != nil {
				return nil, err
			}
			index := -1
			for i := range file.ReadItems {
				if file.ReadItems[i].ID == itemID {
					index = i
					break
				}
			}
			if index < 0 {
				return nil, fmt.Errorf("goal %s has no read item %s", id, itemID)
			}
			if file.ReadItems[index].State != ReadItemOpen {
				return nil, fmt.Errorf("read item %s is already %s", itemID, file.ReadItems[index].State)
			}
			changes := []Change{}
			targets := []string{id}
			if state == ReadItemMoved {
				if reference == id {
					return nil, fmt.Errorf("read item %s cannot move to its own goal", itemID)
				}
				target := tree.Live[reference]
				if target == nil {
					return nil, fmt.Errorf("move target %s is not an open goal", reference)
				}
				targetDisplaced, authErr := authorizeReadItemChange(target, r)
				if authErr != nil {
					return nil, authErr
				}
				original := file.ReadItems[index]
				target.ReadItems = append(target.ReadItems, ReadItem{ID: nextReadItemID(target.ReadItems, original.Read), Read: original.Read, Text: original.Text + " (moved from " + id + ")", State: ReadItemOpen, AddedAt: r.stamp()})
				targets = append(targets, reference)
				touchDisplaced(target, r, "read-items-close", targets, targetDisplaced)
				changes = append(changes, Change{Path: livePath(reference), Content: RenderFile(target)})
			}
			file.ReadItems[index].State = state
			file.ReadItems[index].ClosingReference = reference
			file.ReadItems[index].ChangedAt = r.stamp()
			touchDisplaced(file, r, "read-items-close", targets, displaced)
			changes = append(changes, Change{Path: livePath(id), Content: RenderFile(file)})
			return ackDisplacements(tree, r, changes), nil
		}, Validate: func(commit string) error { return validateCommitFor(r.Endpoint, commit) },
	})
}

// DoneReadItemsOpenError is the typed terminal-transition refusal for unclosed findings.
type DoneReadItemsOpenError struct {
	Goal    string
	ItemIDs []string
}

func (e *DoneReadItemsOpenError) Error() string {
	remedies := make([]string, 0, len(e.ItemIDs))
	for _, itemID := range e.ItemIDs {
		command := fmt.Sprintf("metasystem goal read-items close --id %s --item %s", e.Goal, itemID)
		remedies = append(remedies, command+" --fixed <commit> | "+command+" --moved <goal-id> | "+command+` --accepted "<reason>"`)
	}
	return fmt.Sprintf("GOAL_DONE_READ_ITEMS_OPEN: goal %s has open read items %s; close each with one of:\n%s", e.Goal, strings.Join(e.ItemIDs, ", "), strings.Join(remedies, "\n"))
}

func refuseOpenReadItems(goalID string, file *GoalFile) error {
	var open []string
	if file != nil {
		for _, item := range file.ReadItems {
			if item.State == ReadItemOpen {
				open = append(open, item.ID)
			}
		}
	}
	if len(open) == 0 {
		return nil
	}
	return &DoneReadItemsOpenError{Goal: goalID, ItemIDs: open}
}

// ReadItemsForRetro returns the retro's ledger section from the accepted tree.
// A concluded record always reports zero open items; an impossible open item
// beside it is separately and loudly reported as a ledger defect.
func ReadItemsForRetro(root string) ([]string, bool, error) {
	return readItemsForRetro(Endpoint{Root: root})
}

func readItemsForRetro(e Endpoint) ([]string, bool, error) {
	tip, present, err := e.repository().Accepted()
	if err != nil || !present {
		return nil, present, err
	}
	tree, err := loadTreeFor(e, tip)
	if err != nil {
		return nil, true, err
	}
	return readItemRetroLines(tree), true, nil
}

func readItemRetroLines(tree *TreeGoals) []string {
	all := map[string]*GoalFile{}
	for id, file := range tree.Live {
		all[id] = file
	}
	for id, file := range tree.Done {
		all[id] = file
	}
	for id, file := range tree.Abandoned {
		all[id] = file
	}
	ids := make([]string, 0, len(all))
	for id, file := range all {
		if len(file.ReadItems) > 0 {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	var lines []string
	for _, id := range ids {
		file := all[id]
		open := 0
		for _, item := range file.ReadItems {
			if item.State == ReadItemOpen && file.State != StateDone {
				open++
			}
		}
		lines = append(lines, fmt.Sprintf("goal=%s state=%s open=%d", id, file.State, open))
		for _, item := range file.ReadItems {
			if item.State != ReadItemOpen {
				continue
			}
			if file.State == StateDone {
				lines = append(lines, fmt.Sprintf("LEDGER DEFECT goal=%s concluded with open read item read=%s id=%s text=%s", id, item.Read, item.ID, strconv.Quote(item.Text)))
				continue
			}
			lines = append(lines, fmt.Sprintf("item goal=%s read=%s id=%s text=%s", id, item.Read, item.ID, strconv.Quote(item.Text)))
		}
	}
	return lines
}
