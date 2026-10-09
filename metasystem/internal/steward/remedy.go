package steward

import "strings"

// Remedy names the act that changes a health condition, or who clears it
// when no command can do so. Rendering grants no authority to the reader.
type Remedy struct {
	Act           []string
	Plain         string
	Clears        string
	HumanRequired bool
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
	if len(act) >= 3 && audience == "agent" && (r.HumanRequired || commandAudience != nil && commandAudience(act[1], act[2]) == "human") {
		plain := "a person runs: " + strings.Join(act, " ")
		if r.Plain != "" {
			plain = r.Plain + "; " + plain
		}
		return nil, plain
	}
	return act, r.Plain
}

// PublicRemedy selects fresh typed facts before a role's general remedy.
// Every role is rendered here, including evidence whose cause is unknown.
func (v RoleVerdict) PublicRemedy(audience string, commandAudience func(string, string) string, stopped ...bool) ([]string, string) {
	if v.FailureEscalation == AutoHealEnded && (v.Role == RoleLedgerAttention || v.Role == RoleCapabilitySnapshots) {
		return nil, v.Remedy
	}
	fact := RemedyFact{}
	if len(v.RemedyFacts) > 0 {
		fact = v.RemedyFacts[0]
	}
	act, plain := remedyFor(v.Role, fact).Render(audience, commandAudience)
	if len(stopped) > 0 && stopped[0] && len(act) == 3 && act[1] == "session" && act[2] == "start" {
		return nil, "nothing to do: the claiming session refreshes its stop capability at its next admitted metasystem session start"
	}
	return act, plain
}

func remedyFor(role HealthRole, fact RemedyFact) Remedy {
	switch fact.Cause {
	case CauseBudgetMissing, CauseBudgetMalformed:
		return Remedy{Act: []string{"metasystem", "goal", "budget", fact.Goal, "BOX"}, Clears: "a person gives the goal a structured budget", HumanRequired: true}
	case CauseBudgetBreach:
		return Remedy{Act: []string{"metasystem", "goal", "budget", fact.Goal, "BOX"}, Plain: "goal " + fact.Goal + " is over its box; its stop runs by itself, and a person may give it a larger box", Clears: "a person enlarges the goal's budget", HumanRequired: true}
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
	case CauseStopFenceClosed:
		return Remedy{Plain: "a person runs: " + fact.Command, Clears: "a person reopens the checkout's process creation fence"}
	case CauseUnreadable:
		return Remedy{Plain: "a person repairs the unreadable evidence the reason above names, then runs metasystem system check", Clears: "a person repairs the unreadable evidence"}
	case CauseSettingsInvalid:
		return Remedy{Plain: "a person corrects the setting the reason above names in metasystem.conf, then runs metasystem system check", Clears: "a person corrects the invalid setting"}
	case CauseObservationPending:
		return Remedy{Plain: "nothing to do: the evidence producer completes its pending pass; the next observation reads that completion", Clears: "the evidence producer completes its pending pass"}
	case CauseClockRegressed:
		return Remedy{Plain: "nothing to do: the next observation clears when the machine's clock catches up with the recorded evidence; a person corrects an inaccurate clock", Clears: "the next observation uses a clock at or after the recorded evidence"}
	case "", CauseUnavailable:
	default:
		return Remedy{Plain: "a person changes what the reason above names; no metasystem command does it", Clears: "a person repairs the condition the reason names"}
	}
	if processHealthRole(role) {
		return Remedy{Act: []string{"metasystem", "system", "start"}, Clears: "starting the checkout restores its process roles"}
	}
	switch role {
	case RoleLedgerAttention:
		return Remedy{Plain: "nothing to do: the armed steward examines the move on its next tick", Clears: "the armed steward examines the move on its next tick"}
	case RoleTrunkRed:
		return Remedy{Plain: "nothing to do: the armed landing lane records the cadence at its next validation; metasystem system start arms it", Clears: "the armed landing lane records the cadence at its next validation"}
	case RoleCapabilitySnapshots:
		return Remedy{Plain: "nothing to do: the steward tick probes each runtime on PATH with a missing or stale snapshot and records a fresh snapshot on its next pass", Clears: "the steward tick records a fresh capability snapshot on its next pass"}
	case RoleStopHookDuration:
		return Remedy{Plain: "a person fixes the expensive hook under goal stop-hook-health-cost; the next hook turn records its duration", Clears: "a person fixes the hook before its next measured turn"}
	case RoleProofAttempts:
		return Remedy{Plain: "nothing to do: the job reaper reconciles a dead launcher's attempt on its next pass; metasystem system start arms it", Clears: "the job reaper reconciles the attempt on its next pass"}
	case RoleProofAdmission:
		if fact.Command != "" {
			return Remedy{Plain: "a person follows the lease owner's instruction: " + fact.Command, Clears: "the lease's admission owner settles the reported lease"}
		}
		return Remedy{Plain: "nothing to do: a waiting heavy proof reclaims a dead lease on its next admission pass", Clears: "the next admission pass reclaims the dead lease"}
	case RoleSpendFence:
		return Remedy{Plain: "a person raises the spend ceiling in metasystem.conf", Clears: "a person adjusts the spend ceiling"}
	case RoleRetroDebt:
		return Remedy{Plain: "a person runs the retro and records its receipt", Clears: "a person records the retro receipt"}
	case RoleGovernedObligations:
		return Remedy{Plain: "a person reduces, redesigns, retires, or extends the named attempt with a fresh complete tuple and obligation revision", Clears: "a person revises the governed attempt"}
	case RoleClaimedGoalDelivery:
		return Remedy{Plain: "the claiming session resolves the named delivery failure and delivers the goal; the next observation reads its receipt", Clears: "the claiming session delivers the goal"}
	case RoleContext:
		return Remedy{Plain: "a person restores the named session context evidence; the next observation reads it", Clears: "a person restores the session context evidence"}
	case RoleSeatPresence:
		return Remedy{Plain: "a person repairs the ledger remote or gives each checkout its own nickname, as the reason above names; the next steward tick republishes its presence", Clears: "a person repairs publication before the next steward tick"}
	case RoleDisk:
		return Remedy{Plain: "a person frees space or corrects the disk setting the reason above names; the next disk pass records the result", Clears: "a person repairs the disk condition before the next disk pass"}
	}
	return Remedy{Plain: "a person changes what the reason above names; no metasystem command does it", Clears: "a person repairs the condition the reason names"}
}

func processHealthRole(role HealthRole) bool {
	switch role {
	case RoleStewardRunner, RoleSupervisionOwner, RoleRepoWatcher, RoleNarratorFreshness, RoleCensusFreshness, RoleHookFreshness, RoleSessionMain:
		return true
	}
	return false
}

// WithAlertRemedies uses the newest health episode of the current finding,
// including a suppressed episode whose clear can retry another exhaustion.
func (v HealthVerdict) WithAlertRemedies(root string) HealthVerdict {
	episodes, _ := AlertEpisodes(root)
	var newest AlertEpisode
	for _, episode := range episodes {
		if episode.Digest == v.FindingDigest && episode.Owner == "" && (newest.EpisodeID == "" || !episode.OpenedAt.Before(newest.OpenedAt)) {
			newest = episode
		}
	}
	v.Roles = append([]RoleVerdict(nil), v.Roles...)
	for i, role := range v.Roles {
		if role.FailureEscalation == AutoHealEnded && (role.Role == RoleLedgerAttention || role.Role == RoleCapabilitySnapshots) {
			v.Roles[i].Remedy = healthClearRemedy(role.Role, newest.EpisodeID, role.Reason).Plain
		}
	}
	return v
}

func healthClearRemedy(role HealthRole, episode, reason string) Remedy {
	plain := "a person makes the health alert store readable, then clears its newest health alert to retry healing"
	if episode != "" {
		plain = "metasystem alert clear here/" + episode
	}
	if role == RoleCapabilitySnapshots {
		plain = "run the affected runtime's login (for Claude: claude auth login), then " + plain
	} else if strings.Contains(reason, "--accept-remote-history") {
		plain = "a person runs metasystem goal sync --accept-remote-history --by NAME, then " + plain
	}
	return Remedy{Plain: plain, Clears: "clearing the health alert retries the steward's ended healing"}
}
