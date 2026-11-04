package steward

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// RoleClaimQuota reports ready priority-1 work the one-claim rule holds back.
const RoleClaimQuota HealthRole = "claim-quota"

// checkClaimQuotaWith asks the claim's own quota query about every ready
// priority-1 goal for this machine. Without the lane reader (every preview)
// it shows the verdict the steward tick recorded.
func checkClaimQuotaWith(repoRoot, runRoot string, ledger *healthLedger) RoleVerdict {
	if ledger.laneState == nil {
		if record, err := loadHealthRecord(HealthRecordPath(repoRoot)); err == nil {
			for _, role := range record.Verdict.Roles {
				if role.Role == RoleClaimQuota {
					return role
				}
			}
		}
		return roleAlive(RoleClaimQuota, "the steward tick observes the claim quota")
	}
	unreadable := func(err error) RoleVerdict {
		return roleUnknown(RoleClaimQuota, "the claim quota cannot be read: "+err.Error(), "", RemedyFact{Cause: CauseUnreadable})
	}
	if !ledger.read().newWorld {
		return roleAlive(RoleClaimQuota, "the bootstrap ledger has no claims")
	}
	if ledger.endpointErr != nil {
		return unreadable(ledger.endpointErr)
	}
	if ledger.projectionErr != nil {
		return unreadable(ledger.projectionErr)
	}
	machine, err := goal.ResolveMachine(repoRoot)
	if err != nil {
		return unreadable(err)
	}
	next, err := goal.Next(ledger.projection, machine)
	if err != nil {
		return unreadable(err)
	}
	tree := ledger.projection.Tree
	var laneErr error
	laneState := func(id, main string) (string, error) {
		state, err := ledger.laneState(repoRoot, id, main)
		laneErr = errors.Join(laneErr, err)
		return state, err
	}
	var parts, pairs []string
	var facts []RemedyFact
	for _, id := range next.Ready {
		if tree.Live[id].Priority != 1 {
			continue
		}
		block := goal.ClaimQuotaBlocker(tree, id, machine, ledger.projection.Tip, laneState)
		if block.Held == "" {
			continue
		}
		facts = append(facts, RemedyFact{Cause: CauseClaimQuota, Goal: block.Held, Ready: id, Command: block.Command})
		parts = append(parts, id+" (priority 1) is ready but "+block.Held+" holds this machine's one claim; "+block.Held+" "+
			claimActivity(runRoot, block.Held)+"; next: "+block.Command+", then metasystem goal claim "+id)
		pairs = append(pairs, id+">"+block.Held)
	}
	if laneErr != nil {
		return unreadable(laneErr)
	}
	if len(facts) == 0 {
		return roleAlive(RoleClaimQuota, "no ready priority-1 goal waits behind this machine's claim")
	}
	sort.Strings(pairs)
	verdict := roleDead(RoleClaimQuota, strings.Join(parts, "; "), "", facts...)
	verdict.Cause = strings.Join(pairs, " ")
	return verdict
}

// claimActivity says whether the holding claim is working or waiting.
func claimActivity(runRoot, held string) string {
	jobs, err := goal.NonTerminalGoalJobs(runRoot, held)
	if err != nil {
		return "its jobs can't be read"
	}
	if len(jobs) > 0 {
		return "has a running job"
	}
	// Questions are run state, kept under the installation beside the jobs.
	for _, question := range channel.GoalOpenQuestions(runRoot) {
		if question.Goal == held {
			return "waits on an open question"
		}
	}
	return "has no job and no open question"
}

type claimQuotaOccurrence struct {
	Held  string `json:"held"`
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

func claimQuotaOccurrencePath(root string) string {
	return filepath.Join(filepath.Dir(askedReceipts(root)), "claim-quota.jsonl")
}

func readClaimQuotaOccurrences(root string) (map[string]string, error) {
	data, err := os.ReadFile(claimQuotaOccurrencePath(root))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	open := map[string]string{}
	for _, line := range strings.FieldsFunc(string(data), func(r rune) bool { return r == '\n' }) {
		var entry claimQuotaOccurrence
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, err
		}
		if entry.End != "" {
			delete(open, entry.Held)
		} else {
			open[entry.Held] = entry.Start
		}
	}
	return open, nil
}

func appendClaimQuotaOccurrence(root string, entry claimQuotaOccurrence) error {
	path := claimQuotaOccurrencePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	err = json.NewEncoder(file).Encode(entry)
	return errors.Join(err, file.Close())
}

// askClaimQuotaHolders keeps one ask per holding goal's observed blocking
// interval. Unknown or stopped observations leave those intervals untouched.
func askClaimQuotaHolders(health HealthVerdict, repoRoot, home string, machine func(string) (string, error), now time.Time) error {
	if health.Stopped {
		return nil
	}
	groups := map[string][]RemedyFact{}
	observed := false
	for _, role := range health.Roles {
		if role.Role != RoleClaimQuota {
			continue
		}
		if role.Status != HealthAlive && role.Status != HealthDead {
			return nil
		}
		observed = true
		for _, fact := range role.RemedyFacts {
			if fact.Cause == CauseClaimQuota {
				groups[fact.Goal] = append(groups[fact.Goal], fact)
			}
		}
	}
	if !observed {
		return nil
	}
	open, err := readClaimQuotaOccurrences(repoRoot)
	if err != nil {
		return err
	}
	name := ""
	if len(groups) > 0 {
		name, err = machine(repoRoot)
		if err != nil {
			return err
		}
		ledger := newHealthLedger(repoRoot, now).read()
		if ledger.endpointErr != nil || ledger.projectionErr != nil {
			return errors.Join(ledger.endpointErr, ledger.projectionErr)
		}
		if !ledger.newWorld {
			return nil
		}
		for held := range groups {
			file := ledger.projection.Tree.Live[held]
			if file == nil || file.State != goal.StateClaimed || file.Claimed == nil || file.Claimed.Machine != name || file.Claimed.HandedOver != (goal.HandedOver{}) {
				delete(groups, held)
			}
		}
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	for held := range open {
		if len(groups[held]) == 0 {
			if err := appendClaimQuotaOccurrence(repoRoot, claimQuotaOccurrence{Held: held, End: stamp}); err != nil {
				return err
			}
			delete(open, held)
		}
	}
	if len(groups) == 0 {
		return nil
	}
	if home == "" {
		home, err = board.Home()
		if err != nil {
			return err
		}
	}
	var asks []error
	for held, facts := range groups {
		start := open[held]
		if start == "" {
			start = stamp
			if err := appendClaimQuotaOccurrence(repoRoot, claimQuotaOccurrence{Held: held, Start: start}); err != nil {
				return err
			}
		}
		noun, verb := "ready goals", "wait"
		if len(facts) == 1 {
			noun, verb = "ready goal", "waits"
		}
		first := facts[0]
		text := fmt.Sprintf("%d %s at priority 1 %s for this machine's one claim, which your goal %s holds; the first is %s. Next: %s, then metasystem goal claim %s. Reply in this thread to keep the claim; nothing is asked again while this blocking lasts.", len(facts), noun, verb, held, first.Ready, first.Command, first.Ready)
		asks = append(asks, askOnce(askedReceipts(repoRoot), home, board.Sender{Machine: name}, board.Address{Goal: held},
			"claim-quota\n"+held+"\n"+start, text, now))
	}
	return errors.Join(asks...)
}
