package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

const (
	BatchPrepared = "prepared"
	BatchRunning  = "running"
	BatchClosed   = "closed"
	CodeBatch     = "LANE_BATCH_MEMBERSHIP"
)

// PolicyValue retains the effective selection policy and its source.
// The command layer supplies the repository's policy resolver.
type PolicyValue struct {
	Value    string    `json:"value"`
	Source   string    `json:"source"`
	Checkout string    `json:"checkout"`
	SetBy    string    `json:"set-by"`
	At       time.Time `json:"at,omitempty"`
}

// PolicyDecision holds automatic selection when a person must choose it.
func PolicyDecision(value PolicyValue, readErr error) error {
	if readErr != nil {
		return fmt.Errorf("the batch policy cannot be read; no selection was admitted: %w", readErr)
	}
	if value.Value == "person" {
		return &Refusal{Code: "LANE_BATCH_PERSON", Reason: "the batch policy requires a person's selection; the human-selection consumer is pending", Next: "keep the proposal pending until the human-selection consumer is available"}
	}
	return nil
}

// Batch pins a selection, independently of the queue's authoritative outcomes.
// Its original base and members survive a rebuild on newer main.
type Batch struct {
	ID            string      `json:"id"`
	Lane          lane.Record `json:"lane"`
	Base          string      `json:"base"`
	Members       []GoalSHA   `json:"members"`
	Selector      PolicyValue `json:"selector"`
	CreatedAt     string      `json:"created-at"`
	State         string      `json:"state"`
	ClosureReason string      `json:"closure-reason,omitempty"`
}

func batchPath(install string) string { return filepath.Join(Dir(install), "batch.json") }

// ReadBatch only observes the selection; it never creates or admits work.
func ReadBatch(install string) (*Batch, error) {
	data, err := os.ReadFile(batchPath(install))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var batch Batch
	if err := json.Unmarshal(data, &batch); err != nil {
		return nil, fmt.Errorf("read batch selection: %w", err)
	}
	if batch.ID == "" || batch.Base == "" || batch.Lane.Root == "" || batch.Lane.Install != install || batch.Lane.CustodyEpoch == 0 || !slices.Contains([]string{BatchPrepared, BatchRunning, BatchClosed}, batch.State) {
		return nil, errors.New("the batch selection has an unknown identity or state")
	}
	seen := map[string]bool{}
	for _, member := range batch.Members {
		if member.Goal == "" || member.SHA == "" || seen[member.Goal] {
			return nil, errors.New("the batch selection has an unknown or repeated member")
		}
		seen[member.Goal] = true
	}
	return &batch, nil
}

func writeBatch(install string, batch *Batch) error {
	data, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteFile(batchPath(install), append(data, '\n'), 0600, install)
	return err
}

func (s ProveSeams) batchPolicy() (PolicyValue, error) {
	if s.Policy != nil {
		return s.Policy("landing.batch")
	}
	return PolicyValue{Value: "auto", Source: "built-in", SetBy: "built-in"}, nil
}

func batchRefusal(reason string) error {
	return &Refusal{Code: CodeBatch, Reason: reason + "; no batch check or push was admitted", Next: "reassemble only the recorded selected commits on fetched main once its live operations have ended, then metasystem landing prove"}
}

func (s ProveSeams) batchLane(batch *Batch) error {
	if s.Lane == nil {
		return nil
	}
	current, err := s.Lane()
	if err != nil {
		return err
	}
	if current != batch.Lane {
		return batchRefusal("the lane registration changed after this selection")
	}
	return nil
}

func batchIdle(install string, s ProveSeams) (bool, error) {
	_, _, alive, err := ReadRunning(install, s)
	if err != nil || alive {
		return false, err
	}
	regen, err := ReadRunningRegeneration(install, s)
	if err != nil || regen != nil && regen.State == "running" {
		return false, err
	}
	if s.AgentRunning != nil {
		running, err := s.AgentRunning()
		if err != nil || running {
			return false, err
		}
	}
	return true, nil
}

// SelectBatch fetches before taking the queue lock. A running selection is
// reconciled against main and queue outcomes, never replaced with new arrivals.
func SelectBatch(install, checkout string, registered lane.Record, s ProveSeams) (selected *Batch, err error) {
	entries, err := batchEntries(install)
	if err != nil {
		return nil, err
	}
	existing, err := ReadBatch(install)
	if err != nil {
		return nil, err
	}
	if existing == nil && len(entries) == 0 {
		return nil, nil
	}
	// Fetching main cannot release a conflict or environment hold. When
	// no line is actionable, leave those holds for their owning resolver.
	if (existing == nil || existing.State == BatchClosed) && slices.ContainsFunc(entries, func(e Entry) bool { return e.State == StateWaiting && e.Held }) {
		eligible, err := pending(install, checkout, s)
		if err != nil {
			return nil, err
		}
		if len(eligible) == 0 {
			return existing, nil
		}
	}
	if err := s.fetchMain(checkout); err != nil {
		return nil, err
	}
	main, err := checkoutGit(checkout, s).main()
	if err != nil {
		return nil, err
	}
	err = withLock(install, func() error {
		batch, err := ReadBatch(install)
		if err != nil {
			return err
		}
		if batch != nil {
			if batch.Lane != registered {
				return batchRefusal("the selection belongs to another lane registration")
			}
			if err := s.batchLane(batch); err != nil {
				return err
			}
			idle, err := batchIdle(install, s)
			if err != nil {
				return err
			}
			if batch.State != BatchClosed {
				terminal, err := batchTerminal(install, checkout, main, batch, s)
				if err != nil {
					return err
				}
				if terminal && idle {
					batch.State, batch.ClosureReason = BatchClosed, "all selected members are accounted for on main or by queue outcomes"
					if err := writeBatch(install, batch); err != nil {
						return err
					}
				} else if batch.State == BatchRunning || !idle {
					selected = batch
					return nil
				}
			}
		}
		policy, err := s.batchPolicy()
		if err != nil {
			return PolicyDecision(policy, err)
		}
		if batch != nil && batch.State == BatchPrepared && batch.Selector.Value == policy.Value {
			selected = batch
		} else {
			selected, err = selectBatchLocked(install, checkout, main, registered, policy, s)
			if err != nil {
				return err
			}
		}
		if selected == nil {
			selected = batch
			return nil
		}
		return PolicyDecision(policy, nil)
	})
	return selected, err
}

func selectBatchLocked(install, checkout, main string, registered lane.Record, policy PolicyValue, s ProveSeams) (*Batch, error) {
	if _, err := batchEntries(install); err != nil {
		return nil, err
	}
	entries, err := pending(install, checkout, s)
	if err != nil {
		return nil, err
	}
	n := len(entries)
	if policy.Value != "auto" && policy.Value != "person" {
		cap := strings.TrimLeft(policy.Value, "0")
		available := strconv.Itoa(n)
		if cap == "" || strings.ContainsAny(cap, "+- \t\n") {
			return nil, errors.New("the batch cap must be a positive decimal integer")
		}
		for _, digit := range cap {
			if digit < '0' || digit > '9' {
				return nil, errors.New("the batch cap must be a positive decimal integer")
			}
		}
		// Narrow only when the cap is smaller than the available count.
		if len(cap) < len(available) || len(cap) == len(available) && cap < available {
			n, _ = strconv.Atoi(cap)
		}
	}
	if n == 0 {
		return nil, nil
	}
	batch := &Batch{ID: s.newID(), Lane: registered, Base: main, Selector: policy, CreatedAt: s.now().Format(time.RFC3339Nano), State: BatchPrepared, Members: []GoalSHA{}}
	for _, entry := range entries[:n] {
		batch.Members = append(batch.Members, GoalSHA{Goal: entry.Goal, SHA: entry.SHA})
	}
	if err := s.batchLane(batch); err != nil {
		return nil, err
	}
	return batch, writeBatch(install, batch)
}

// batchEntries keeps the current outcome of each goal and commit pair.
// A returned hand-in can be handed in again at the same commit.
func batchEntries(install string) ([]Entry, error) {
	entries, err := AreaEntries(install)
	if err != nil {
		return nil, err
	}
	seen := map[GoalSHA]bool{}
	latest := []Entry{}
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		key := GoalSHA{Goal: entry.Goal, SHA: entry.SHA}
		if !seen[key] {
			latest = append(latest, entry)
			seen[key] = true
		}
	}
	slices.Reverse(latest)
	return latest, nil
}

func batchTerminal(install, checkout, main string, batch *Batch, s ProveSeams) (bool, error) {
	entries, err := batchEntries(install)
	if err != nil {
		return false, err
	}
	for _, member := range batch.Members {
		landed, err := batchContains(checkout, main, member.SHA, s)
		if err != nil {
			return false, err
		}
		if landed {
			continue
		}
		index := slices.IndexFunc(entries, func(e Entry) bool { return e.Goal == member.Goal && e.SHA == member.SHA })
		if index < 0 {
			return false, batchRefusal("a selected hand-in is missing from the queue")
		}
		entry := entries[index]
		if entry.State == StateWaiting && !(entry.Held && len(entry.After) > 0) {
			return false, nil
		}
	}
	return true, nil
}

// batchContains treats missing Git objects as unknown, never as exclusion.
func batchContains(checkout, commit, sha string, s ProveSeams) (bool, error) {
	for _, id := range []string{commit, sha} {
		if _, err := s.git(checkout, "cat-file", "-e", id+"^{commit}"); err != nil {
			return false, fmt.Errorf("read batch commit %s: %w", Short(id), err)
		}
	}
	return checkoutGit(checkout, s).contains(commit, sha)
}

// CheckBatch validates real first-parent merges and newly contained queue
// commits. Gates may prove a prefix; a full proof or push accounts for all members.
func CheckBatch(install, checkout, commit, main string, prefix bool, s ProveSeams) (*Batch, error) {
	var batch *Batch
	err := withLock(install, func() error {
		var err error
		batch, err = checkBatchLocked(install, checkout, commit, main, prefix, false, s)
		return err
	})
	return batch, err
}

func checkBatchLocked(install, checkout, commit, main string, prefix, admit bool, s ProveSeams) (*Batch, error) {
	batch, err := ReadBatch(install)
	if err != nil {
		return nil, err
	}
	if batch == nil {
		if s.Lane != nil {
			entries, err := batchEntries(install)
			if err != nil {
				return nil, err
			}
			// A check with no hand-ins has no goal selection to account for.
			if len(entries) == 0 {
				return nil, nil
			}
			if main == "" {
				main, err = checkoutGit(checkout, s).main()
				if err != nil {
					return nil, err
				}
			}
			onMain, err := batchContains(checkout, main, commit, s)
			if err != nil {
				return nil, err
			}
			if onMain {
				return nil, nil
			}
			return nil, batchRefusal("this candidate has no selection; its keeper must prepare one before launch")
		}
		return nil, nil
	}
	if err := s.batchLane(batch); err != nil {
		return batch, err
	}
	if main == "" {
		main, err = checkoutGit(checkout, s).main()
		if err != nil {
			return batch, err
		}
	}
	if batch.State == BatchPrepared && admit {
		policy, err := s.batchPolicy()
		if err := PolicyDecision(policy, err); err != nil {
			return batch, err
		}
		if policy.Value != batch.Selector.Value {
			batch, err = selectBatchLocked(install, checkout, main, batch.Lane, policy, s)
			if err != nil {
				return batch, err
			}
			if batch == nil {
				return nil, batchRefusal("no eligible member remains")
			}
		}
	}
	entries, err := batchEntries(install)
	if err != nil {
		return batch, err
	}
	chain, err := s.git(checkout, "rev-list", "--first-parent", "--reverse", "--parents", batch.Base+".."+commit)
	if err != nil {
		return batch, err
	}
	anchored, err := batchContains(checkout, commit, batch.Base, s)
	if err != nil {
		return batch, err
	}
	if !anchored {
		return batch, batchRefusal("the candidate does not contain the batch's original main")
	}
	previous, next := batch.Base, 0
	for _, line := range strings.Split(strings.TrimSpace(chain), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) < 2 || len(fields) > 3 || fields[1] != previous {
			return batch, batchRefusal("the candidate's first-parent topology cannot be accounted for")
		}
		previous = fields[0]
		onMain, err := batchContains(checkout, main, fields[0], s)
		if err != nil {
			return batch, err
		}
		if onMain {
			continue
		}
		if len(fields) == 2 {
			reason := "commit " + Short(fields[0]) + " is outside fetched main and does not merge a selected member or main"
			var outside []string
			for _, entry := range entries {
				if !slices.Contains(batch.Members, GoalSHA{Goal: entry.Goal, SHA: entry.SHA}) {
					outside = append(outside, "goal "+entry.Goal)
				}
			}
			if len(outside) > 0 {
				reason += "; hand-ins outside the selection: " + strings.Join(outside, ", ")
			}
			return batch, batchRefusal(reason)
		}
		refresh, err := batchContains(checkout, main, fields[2], s)
		if err != nil {
			return batch, err
		}
		if refresh {
			continue
		}
		at := slices.IndexFunc(batch.Members, func(g GoalSHA) bool { return g.SHA == fields[2] })
		if at < next {
			return batch, batchRefusal("a merge's second parent is outside the ordered selection")
		}
		for ; next < at; next++ {
			accounted, err := memberAccounted(entries, batch.Members[next], checkout, main, s)
			if err != nil {
				return batch, err
			}
			if !accounted {
				return batch, batchRefusal("a merge skipped an unresolved selected member")
			}
		}
		next = at + 1
	}
	// An older entry of a goal is skipped only when that goal's newest entry
	// is a selected, waiting member (a fix-forward carries its old commit);
	// any other goal's older content in the candidate is refused.
	newest := map[string]int{}
	for i, entry := range entries {
		newest[entry.Goal] = i
	}
	selectedGoal := map[string]bool{}
	for goal, i := range newest {
		entry := entries[i]
		selectedGoal[goal] = entry.State == StateWaiting && !entry.Held && slices.Contains(batch.Members, GoalSHA{Goal: entry.Goal, SHA: entry.SHA})
	}
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if newest[entry.Goal] != i && selectedGoal[entry.Goal] {
			continue
		}
		before, err := batchContains(checkout, main, entry.SHA, s)
		if err != nil {
			return batch, err
		}
		if before {
			continue
		}
		inside, err := batchContains(checkout, commit, entry.SHA, s)
		if err != nil {
			return batch, err
		}
		if !inside {
			continue
		}
		selected := slices.Contains(batch.Members, GoalSHA{Goal: entry.Goal, SHA: entry.SHA})
		if !selected || entry.State != StateWaiting || entry.Held {
			return batch, batchRefusal("the candidate contains an unselected, superseded, returned or held hand-in: " + entry.Goal)
		}
	}
	for _, member := range batch.Members {
		accounted, err := memberAccounted(entries, member, checkout, main, s)
		if err != nil {
			return batch, err
		}
		inside, err := batchContains(checkout, commit, member.SHA, s)
		if err != nil {
			return batch, err
		}
		if accounted {
			onMain, err := batchContains(checkout, main, member.SHA, s)
			if err != nil {
				return batch, err
			}
			if inside && !onMain {
				return batch, batchRefusal("the candidate contains a superseded, returned or held selected hand-in: " + member.Goal)
			}
			continue
		}
		if !inside && !prefix {
			return batch, batchRefusal("the full candidate omits selected goal " + member.Goal)
		}
	}
	if batch.State == BatchClosed {
		terminal, err := batchTerminal(install, checkout, main, batch, s)
		if err != nil {
			return batch, err
		}
		if !terminal {
			return batch, batchRefusal("the selection is closed")
		}
	}
	if admit && batch.State == BatchPrepared {
		batch.State = BatchRunning
		err = writeBatch(install, batch)
	}
	return batch, err
}

func memberAccounted(entries []Entry, member GoalSHA, checkout, main string, s ProveSeams) (bool, error) {
	landed, err := batchContains(checkout, main, member.SHA, s)
	if err != nil || landed {
		return landed, err
	}
	index := slices.IndexFunc(entries, func(e Entry) bool { return e.Goal == member.Goal && e.SHA == member.SHA })
	if index < 0 {
		return false, batchRefusal("a selected hand-in is missing from the queue")
	}
	entry := entries[index]
	return entry.State == StateReturned || entry.State == StateSuperseded || entry.Held && len(entry.After) > 0, nil
}
