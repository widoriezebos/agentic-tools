package state

type Decision struct{ Word, Fact string }
type Ownership = Decision
type Admission = Decision
type Who = Decision
type Premise = Decision
type Owner = Decision

const Abandoned, Owned, Settled, Unknown, Person, AgentActor, Loop, Current, Gone = "abandoned", "owned", "settled", "unknown", "person", "agent", "loop", "current", "gone"

type MergeFacts struct {
	Present, JobAlive, WritersSettled, MainMoved, Closed, Merged bool
	Job, State, Checkpoint, Head, Tip, MergeTip, Unreadable      string
}

// MergeOwnership uses the first matching row:
// unreadable, absent or mismatched record -> unknown (unreadable/record/checkpoint/tip).
// closed record or merged tip -> settled (closed/merged).
// live job, unsettled writers or otherwise-current resolution -> owned (job/writers/resolution).
// settled abandoned record or resolving after a code move -> abandoned (state/main).
func MergeOwnership(f MergeFacts) Ownership {
	switch {
	case f.Unreadable != "":
		return Ownership{Unknown, f.Unreadable}
	case !f.Present:
		return Ownership{Unknown, "record"}
	case f.Checkpoint == "" || f.Checkpoint != f.Head:
		return Ownership{Unknown, "checkpoint"}
	case f.Tip == "" || f.Tip != f.MergeTip:
		return Ownership{Unknown, "tip"}
	case f.Closed:
		return Ownership{Settled, "closed"}
	case f.Merged:
		return Ownership{Settled, "merged"}
	case f.JobAlive:
		return Ownership{Owned, "job"}
	case !f.WritersSettled:
		return Ownership{Owned, "writers"}
	case f.State == "abandoned":
		return Ownership{Abandoned, "state"}
	case f.State == "resolving" && f.MainMoved:
		return Ownership{Abandoned, "main"}
	default:
		return Ownership{Owned, "resolution"}
	}
}

type AdmissionFacts struct {
	PersonSeal, CapReached, Selected, Held, Eligible, Selecting bool
	Policy, EntryState, HoldReason, Unreadable                  string
}

// Admissibility: unreadable facts or absent admission policy/eligibility -> unknown (read/policy/hand-in).
// seal, cap, person/nonautomatic policy, selected, nonwaiting or held -> not admissible (recorded reason).
// other selection or eligible automatic entry -> admissible (policy).
func Admissibility(f AdmissionFacts) Admission {
	switch {
	case f.Unreadable != "":
		return Admission{Unknown, f.Unreadable}
	case f.PersonSeal:
		return Admission{"not admissible", "sealed by a person"}
	case f.Selecting && f.Policy == "person":
		return Admission{"not admissible", "the batch policy requires a person's selection"}
	case f.Selecting:
		return Admission{"admissible", "selection policy permits it"}
	case f.CapReached:
		return Admission{"not admissible", "batch cap is one"}
	case f.Policy == "":
		return Admission{Unknown, "the admission policy is unrecorded"}
	case f.Policy != "auto":
		return Admission{"not admissible", "the batch does not use automatic policy"}
	case f.Selected:
		return Admission{"not admissible", "the goal is already selected in this batch"}
	case !f.Eligible:
		return Admission{Unknown, "the hand-in does not establish eligibility"}
	case f.EntryState != "waiting":
		return Admission{"not admissible", "the goal is " + f.EntryState}
	case f.Held:
		return Admission{"not admissible", "the goal is held: " + f.HoldReason}
	default:
		return Admission{"admissible", "eligible under automatic admission"}
	}
}

type ActorFacts struct {
	Provenance, Enrolled bool
	Lineage, Unreadable  string
}

// Actor: unreadable enrollment/lineage -> unknown (unreadable fact).
// recorded act or enrolled terminal -> person (provenance/enrollment).
// otherwise -> agent (lineage, or absent person for legacy acts).
func Actor(f ActorFacts) Who {
	switch {
	case f.Unreadable != "":
		return Who{Unknown, f.Unreadable}
	case f.Provenance:
		return Who{Person, "provenance"}
	case f.Enrolled:
		return Who{Person, "enrollment"}
	case f.Lineage != "":
		return Who{AgentActor, "lineage"}
	default:
		return Who{AgentActor, "absent person"}
	}
}

type PremiseFacts struct {
	Policy, Satisfied, LaneAgent, CheckoutOffMain, Contains bool
	Branch, Main, ExpectedMain, Unreadable                  string
}

// QuestionPremise: unreadable fact, other lineage or no comparable premise -> unknown (fact/lineage/premise).
// satisfied policy, restored main checkout or contained old main -> gone (policy/checkout/main).
// unsatisfied policy, unchanged main or checkout still off main -> current (policy/main/checkout).
func QuestionPremise(f PremiseFacts) Premise {
	switch {
	case f.Unreadable != "":
		return Premise{Unknown, f.Unreadable}
	case f.Policy && f.Satisfied:
		return Premise{Gone, "policy effect"}
	case f.Policy:
		return Premise{Current, "policy"}
	case !f.LaneAgent:
		return Premise{Unknown, "lineage"}
	case f.CheckoutOffMain && f.Branch == "main":
		return Premise{Gone, "checkout"}
	case f.ExpectedMain != "" && f.Main == f.ExpectedMain:
		return Premise{Current, "main"}
	case f.ExpectedMain != "" && f.Contains:
		return Premise{Gone, "main"}
	case f.CheckoutOffMain:
		return Premise{Current, "checkout"}
	default:
		return Premise{Unknown, "premise"}
	}
}

type StopFacts struct {
	Paused, StoppedBy, PersonProof, Recorded bool
	Loop                                     string
}

// StopOwner's decision table:
// pause, stopped_by, proof.person or another owner's stop record -> person (first matching field).
// no stop record or lane-proof/lane-gate/lane-return -> loop (loop).
func StopOwner(f StopFacts) Owner {
	switch {
	case f.Paused:
		return Owner{Person, "pause"}
	case f.StoppedBy:
		return Owner{Person, "stopped_by"}
	case f.PersonProof:
		return Owner{Person, "proof.person"}
	case !f.Recorded && f.Loop == "" || f.Loop == "lane-proof" || f.Loop == "lane-gate" || f.Loop == "lane-return":
		return Owner{Loop, "loop"}
	default:
		return Owner{Person, "loop"}
	}
}
