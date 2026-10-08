package steward

import "strings"

// Remedy names the act that changes a health condition, or who clears it
// when no command can do so. Rendering grants no authority to the reader.
type Remedy struct {
	Act    []string
	Plain  string
	Clears string
}

// Render chooses the public act for the reader's audience.
func (r Remedy) Render(audience string, commandAudience func(string, string) string) ([]string, string) {
	act := r.Act
	if len(act) == 3 && act[1] == "system" && act[2] == "start" && audience == "agent" {
		act = []string{"metasystem", "session", "start"}
	}
	if len(act) == 3 && act[1] == "session" && act[2] == "start" && audience != "agent" {
		return nil, "nothing to do: the claiming session refreshes its stop capability at its next metasystem session start"
	}
	if len(act) == 4 && act[1] == "goal" && act[2] == "release" && audience != "agent" {
		act = []string{"metasystem", "goal", "claim", act[3], "--take-over", "--reason", "TEXT"}
	}
	if len(act) >= 3 && commandAudience != nil && audience == "agent" && commandAudience(act[1], act[2]) == "human" {
		plain := "a person runs: " + strings.Join(act, " ")
		if r.Plain != "" {
			plain = r.Plain + "; " + plain
		}
		return nil, plain
	}
	return act, r.Plain
}

// PublicRemedy selects fresh typed facts before a role's general remedy.
// An empty result leaves the roles without table entries to their renderer.
func (v RoleVerdict) PublicRemedy(audience string, commandAudience func(string, string) string) ([]string, string) {
	if len(v.RemedyFacts) > 0 {
		return remedyFor(v.Role, v.RemedyFacts[0]).Render(audience, commandAudience)
	}
	if v.Role == RoleTrunkRed && !strings.Contains(v.Reason, "cadence") {
		return nil, ""
	}
	return remedyFor(v.Role, RemedyFact{}).Render(audience, commandAudience)
}

func remedyFor(role HealthRole, fact RemedyFact) Remedy {
	switch fact.Cause {
	case CauseBudgetMissing, CauseBudgetMalformed:
		return Remedy{Act: []string{"metasystem", "goal", "budget", fact.Goal, "BOX"}, Clears: "a person gives the goal a structured budget"}
	case CauseBudgetBreach:
		return Remedy{Act: []string{"metasystem", "goal", "budget", fact.Goal, "BOX"}, Plain: "goal " + fact.Goal + " is over its box; its stop runs by itself, and a person may give it a larger box", Clears: "a person enlarges the goal's budget"}
	case CauseBudgetUnknown:
		return Remedy{Plain: "a person repairs record " + fact.Record + ", which cannot be read as a budget, then runs metasystem system check", Clears: "a person repairs the unreadable budget record"}
	case CauseBreachStopOpen:
		return Remedy{Plain: "goal " + fact.Goal + "'s budget stop " + fact.Stop + " completes by itself on the steward's next pass; nothing needs doing", Clears: "the steward completes the budget stop on its next pass"}
	case CauseBreachStopUnresolved:
		return Remedy{Plain: "a person inspects budget stop " + fact.Stop + " of goal " + fact.Goal + " and its job records; new work stays fenced until it resolves", Clears: "a person resolves the budget stop's unreadable job records"}
	case CauseEpochMismatch:
		return Remedy{Act: []string{"metasystem", "session", "start"}, Clears: "the session refreshes its stop capability when started"}
	case CauseForeignLineage:
		return Remedy{Act: []string{"metasystem", "goal", "release", fact.Goal}, Plain: "the session that claimed goal " + fact.Goal + " releases it, or a person takes it over: metasystem goal claim " + fact.Goal + " --take-over --reason TEXT", Clears: "the claiming session releases the goal or a person takes it over"}
	case CauseStopCapabilityMissing:
		return Remedy{Plain: "a person repairs goal " + fact.Goal + "'s record (" + fact.Record + "), which has no stop capability", Clears: "a person repairs the goal's missing stop capability"}
	case CauseJobProcessDead:
		return Remedy{Act: []string{"metasystem", "work", "stop", "j2:" + fact.Job}, Clears: "the job's cancellation records it as ended"}
	case CauseTrunkRedUnowned:
		return Remedy{Act: []string{"metasystem", "incident", "claim", fact.Incident, "--goal", "G"}, Clears: "claiming the incident records its owner"}
	case "":
	default:
		return Remedy{Plain: "a person changes what the reason above names; no metasystem command does it", Clears: "a person repairs the condition the reason names"}
	}
	switch role {
	case RoleStewardRunner, RoleSupervisionOwner, RoleRepoWatcher, RoleNarratorFreshness, RoleCensusFreshness, RoleHookFreshness, RoleSessionMain:
		return Remedy{Act: []string{"metasystem", "system", "start"}, Clears: "starting the checkout restores its process roles"}
	case RoleTrunkRed:
		return Remedy{Plain: "nothing to do: the armed landing lane records the cadence at its next validation; metasystem system start arms it", Clears: "the armed landing lane records the cadence at its next validation"}
	}
	return Remedy{}
}
