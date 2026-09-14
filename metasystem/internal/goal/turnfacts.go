package goal

import (
	"fmt"
	"sort"
	"strings"
)

// TurnVerdictFacts is the uncut, immutable input to Stop presentation. It is
// assembled while the turn verdict lock still owns the judgment and is never
// recovered by parsing the bounded display.
type TurnVerdictFacts struct {
	SchemaVersion int                `json:"schemaVersion"`
	Identity      TurnFactsIdentity  `json:"identity"`
	Verdict       Verdict            `json:"verdict"`
	FullDisplay   string             `json:"fullDisplay"`
	Scan          ScanResult         `json:"scan"`
	Work          TurnWorkFacts      `json:"work"`
	Ownership     TurnOwnershipFacts `json:"ownership"`
	Actions       []TurnAction       `json:"actions"`
	Refusal       TurnRefusalFacts   `json:"refusal"`
}

type TurnFactsIdentity struct {
	Installation string `json:"installation"`
	Session      string `json:"session"`
	MainId       string `json:"mainId"`
	ObservedAt   string `json:"observedAt"`
}

type TurnWorkFacts struct {
	ReadSucceeded   bool               `json:"readSucceeded"`
	Claimed         []GoalFacts        `json:"claimed"`
	Landing         []GoalFacts        `json:"landing"`
	Claimable       []GoalFacts        `json:"claimable"`
	Selected        *GoalFacts         `json:"selected"`
	Selection       string             `json:"selection"`
	Refused         []AdmissionRefusal `json:"refused"`
	InFlight        []string           `json:"inFlight"`
	NonTerminalJobs []string           `json:"nonTerminalJobs"`
	Queued          int                `json:"queued"`
	GoalFree        bool               `json:"goalFree"`
}

type TurnOwnershipFacts struct {
	State    string `json:"state"`
	GoalId   string `json:"goalId"`
	Evidence string `json:"evidence"`
}

type TurnAction struct {
	Kind              string `json:"kind"`
	TargetId          string `json:"targetId"`
	Instruction       string `json:"instruction"`
	Command           string `json:"command"`
	Owner             string `json:"owner"`
	Restriction       string `json:"restriction"`
	HumanRequired     bool   `json:"humanRequired"`
	SupervisionRepair bool   `json:"supervisionRepair"`
}

type TurnRefusalFacts struct {
	Class             string              `json:"class"`
	CauseCode         string              `json:"causeCode"`
	Component         string              `json:"component"`
	Detail            string              `json:"detail"`
	Remedy            string              `json:"remedy"`
	BlockSource       string              `json:"blockSource"`
	Occurrence        int                 `json:"occurrence"`
	CountSpent        bool                `json:"countSpent"`
	IdleRefusal       bool                `json:"idleRefusal"`
	HumanStopConsumed bool                `json:"humanStopConsumed"`
	HumanRequired     bool                `json:"humanRequired"`
	SupervisionRepair bool                `json:"supervisionRepair"`
	Escalation        TurnEscalationFacts `json:"escalation"`
}

type TurnEscalationFacts struct {
	IntentId          string `json:"intentId"`
	IncidentId        string `json:"incidentId"`
	AlarmDetail       string `json:"alarmDetail"`
	Detail            string `json:"detail"`
	IntentPrepared    bool   `json:"intentPrepared"`
	HumanRequired     bool   `json:"humanRequired"`
	SupervisionRepair bool   `json:"supervisionRepair"`
}

func freezeTurnVerdictFacts(root, sessionID, mainID string, scan ScanResult, work ClaimableBudgetedWork, workRead bool, workErr error, verdict Verdict, fullDisplay string, session *sessionState, options TurnVerdictOptions, humanStopConsumed bool) *TurnVerdictFacts {
	scan = cloneScanForFreeze(scan)
	classifyOwnership(&scan, mainID)
	facts := &TurnVerdictFacts{
		SchemaVersion: 1,
		Identity:      TurnFactsIdentity{Installation: root, Session: sessionID, MainId: mainID, ObservedAt: sNowISO(root, session)},
		Verdict:       verdict, FullDisplay: fullDisplay, Scan: scan,
		Work: freezeWork(work, workRead, workErr), Actions: []TurnAction{},
		Refusal: TurnRefusalFacts{
			Class: verdict.Class, CauseCode: verdict.CauseCode, Component: verdict.Component,
			Detail: fullDisplay, Occurrence: session.IdleBlocks, CountSpent: verdict.CountSpent,
			IdleRefusal: verdict.IdleRefusal, HumanStopConsumed: humanStopConsumed,
			SupervisionRepair: verdict.Class == "infrastructure",
		},
	}
	if verdict.BlockSource != nil {
		facts.Refusal.BlockSource = *verdict.BlockSource
	}
	facts.Ownership = freezeOwnership(facts.Work, options)
	facts.Actions = append(facts.Actions, workActions(root, facts.Work, options)...)
	facts.Actions = append(facts.Actions, scanActions(root, scan)...)
	if verdict.escalation != nil {
		facts.Refusal.Escalation = *verdict.escalation
	}
	return facts
}

func cloneScanForFreeze(scan ScanResult) ScanResult {
	scan.Open = append([]Item{}, scan.Open...)
	scan.TemplateUnfilled = append([]Item{}, scan.TemplateUnfilled...)
	scan.OpenWorkWarnings = append([]string{}, scan.OpenWorkWarnings...)
	scan.WaitingOnHuman = append([]Item{}, scan.WaitingOnHuman...)
	scan.StalePlans = append([]Item{}, scan.StalePlans...)
	scan.Busy = append([]Item{}, scan.Busy...)
	scan.Questions = append([]Item{}, scan.Questions...)
	scan.Drafts = append([]Item{}, scan.Drafts...)
	scan.Unreadable = append([]string{}, scan.Unreadable...)
	scan.Jobs = append([]JobFact{}, scan.Jobs...)
	scan.Runs = append([]RunFact{}, scan.Runs...)
	scan.RunUnreadable = append([]string{}, scan.RunUnreadable...)
	return nonnilScan(scan)
}

// sNowISO uses the same already-frozen timestamp that touched this session.
func sNowISO(_ string, session *sessionState) string { return session.LastTouched }

func nonnilScan(scan ScanResult) ScanResult {
	if scan.Open == nil {
		scan.Open = []Item{}
	}
	if scan.TemplateUnfilled == nil {
		scan.TemplateUnfilled = []Item{}
	}
	if scan.OpenWorkWarnings == nil {
		scan.OpenWorkWarnings = []string{}
	}
	if scan.WaitingOnHuman == nil {
		scan.WaitingOnHuman = []Item{}
	}
	if scan.StalePlans == nil {
		scan.StalePlans = []Item{}
	}
	if scan.Busy == nil {
		scan.Busy = []Item{}
	}
	if scan.Questions == nil {
		scan.Questions = []Item{}
	}
	if scan.Drafts == nil {
		scan.Drafts = []Item{}
	}
	if scan.Unreadable == nil {
		scan.Unreadable = []string{}
	}
	if scan.Jobs == nil {
		scan.Jobs = []JobFact{}
	}
	if scan.Runs == nil {
		scan.Runs = []RunFact{}
	}
	if scan.RunUnreadable == nil {
		scan.RunUnreadable = []string{}
	}
	return scan
}

func classifyOwnership(scan *ScanResult, mainID string) {
	for index := range scan.Jobs {
		scan.Jobs[index].Ownership = ownershipForMain(scan.Jobs[index].MainId, mainID)
	}
	for index := range scan.Runs {
		scan.Runs[index].Ownership = ownershipForMain(scan.Runs[index].MainId, mainID)
	}
}

func ownershipForMain(owner, mainID string) string {
	if owner == "" || mainID == "" {
		return "unknown"
	}
	if owner == mainID {
		return "owned"
	}
	return "other"
}

func freezeWork(work ClaimableBudgetedWork, workRead bool, workErr error) TurnWorkFacts {
	result := TurnWorkFacts{
		ReadSucceeded: workRead && workErr == nil, Claimed: []GoalFacts{}, Landing: []GoalFacts{}, Claimable: []GoalFacts{},
		Selection: "none", Refused: append([]AdmissionRefusal{}, work.Refused...),
		InFlight: append([]string{}, work.InFlight...), NonTerminalJobs: append([]string{}, work.NonTerminalJobs...),
		Queued: work.Queued, GoalFree: work.GoalFree,
	}
	if !workRead || workErr != nil {
		result.Selection = "unknown"
		return result
	}
	appendGoals := func(ids []string) []GoalFacts {
		out := make([]GoalFacts, 0, len(ids))
		for _, id := range ids {
			fact := work.GoalFacts[id]
			if fact.Id == "" {
				fact.Id = id
			}
			out = append(out, fact)
		}
		return out
	}
	result.Claimed = appendGoals(work.Claimed)
	result.Landing = appendGoals(work.Landing)
	result.Claimable = appendGoals(work.Claimable)
	if len(result.Claimed) > 0 {
		selected := result.Claimed[0]
		result.Selected, result.Selection = &selected, "held"
	} else if len(result.Claimable) > 0 {
		selected := result.Claimable[0]
		result.Selected, result.Selection = &selected, "claimable"
	}
	return result
}

func freezeOwnership(work TurnWorkFacts, options TurnVerdictOptions) TurnOwnershipFacts {
	if !work.ReadSucceeded || options.SeatActorProblem != "" {
		return TurnOwnershipFacts{State: "unknown", Evidence: options.SeatActorProblem}
	}
	if len(work.Claimed) == 0 {
		return TurnOwnershipFacts{State: "none", Evidence: "the fresh goal judgment found no held working claim"}
	}
	return TurnOwnershipFacts{State: "owned", GoalId: work.Claimed[0].Id,
		Evidence: fmt.Sprintf("the fresh goal judgment joined the held claim to %s", options.SeatActor.historyActor())}
}

func workActions(root string, work TurnWorkFacts, options TurnVerdictOptions) []TurnAction {
	if work.Selected == nil {
		return nil
	}
	selected := *work.Selected
	machine := options.SeatActor.Machine
	if machine == "" {
		machine, _ = ResolveMachine(root)
	}
	if work.Selection == "held" {
		owner := options.SeatActor.historyActor()
		if owner == "" {
			owner = machine
		}
		return []TurnAction{{Kind: "continue-goal", TargetId: selected.Id,
			Instruction: selected.NextStep, Owner: owner}}
	}
	if machine == "" {
		return []TurnAction{{Kind: "claim-goal", TargetId: selected.Id,
			Instruction: "Recover the seat machine identity, then fetch and claim the ready goal and follow its next step: " + selected.NextStep,
			Owner:       "seat"}}
	}
	return []TurnAction{{Kind: "claim-goal", TargetId: selected.Id,
		Instruction: "Fetch and claim the ready goal, then follow its next step: " + selected.NextStep,
		Command:     "metasystem goal next --machine " + machine + " --fetch", Owner: machine}}
}

func scanActions(root string, scan ScanResult) []TurnAction {
	var actions []TurnAction
	for _, item := range scan.Jobs {
		if item.Ownership == "owned" && (item.Status == "pending" || item.Status == "running") && !item.WaiterLive {
			actions = append(actions, TurnAction{Kind: "watch-job", TargetId: item.Id,
				Instruction: "watch the owned delegate job before ending the turn",
				Command:     "metasystem job watch --root " + shellArgument(root) + " --job " + shellArgument(item.Id) + " --caller-pid $$", Owner: "seat"})
		}
	}
	for _, item := range scan.Runs {
		if item.Ownership == "owned" && (item.Status == "launching" || item.Status == "running" || item.Status == "draining") && !item.WaiterLive {
			actions = append(actions, TurnAction{Kind: "watch-run", TargetId: item.Id,
				Instruction: "watch the owned run before ending the turn",
				Command:     "metasystem run watch --id " + shellArgument(item.Id) + " --root " + shellArgument(root), Owner: "seat"})
		}
	}
	for _, item := range append(append([]Item{}, scan.Questions...), scan.WaitingOnHuman...) {
		actions = append(actions, TurnAction{Kind: item.Kind, TargetId: item.Id, Instruction: item.RequestedAction,
			Owner: "human", Restriction: "human decision required", HumanRequired: true})
	}
	for _, item := range scan.Open {
		actions = append(actions, TurnAction{Kind: "open-plan", TargetId: item.Id, Instruction: item.RequestedAction,
			Owner: item.OwnerMainId})
	}
	sort.SliceStable(actions, func(i, j int) bool { return actions[i].Kind+actions[i].TargetId < actions[j].Kind+actions[j].TargetId })
	return actions
}

func shellArgument(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
