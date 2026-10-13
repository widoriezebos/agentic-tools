package state

import "testing"

func TestMergeOwnership(t *testing.T) {
	t.Parallel()
	base := MergeFacts{Present: true, Job: "job", State: "resolving", WritersSettled: true, Checkpoint: "base", Head: "base", Tip: "tip", MergeTip: "tip"}
	for _, tc := range []struct {
		name   string
		change func(*MergeFacts)
		want   Ownership
	}{
		{"unreadable", func(f *MergeFacts) { f.Unreadable = "job unreadable" }, Ownership{"unknown", "job unreadable"}},
		{"absent", func(f *MergeFacts) { f.Present = false; f.MainMoved = true }, Ownership{"unknown", "record"}},
		{"another checkpoint", func(f *MergeFacts) { f.Checkpoint = "other"; f.State = "abandoned" }, Ownership{"unknown", "checkpoint"}},
		{"missing checkpoint", func(f *MergeFacts) { f.Checkpoint = ""; f.Head = "" }, Ownership{"unknown", "checkpoint"}},
		{"another merge", func(f *MergeFacts) { f.Tip = "other"; f.State = "abandoned" }, Ownership{"unknown", "tip"}},
		{"missing tip", func(f *MergeFacts) { f.Tip = ""; f.MergeTip = "" }, Ownership{"unknown", "tip"}},
		{"closed", func(f *MergeFacts) { f.Closed = true }, Ownership{"settled", "closed"}},
		{"merged", func(f *MergeFacts) { f.Merged = true }, Ownership{"settled", "merged"}},
		{"live job despite main move", func(f *MergeFacts) { f.JobAlive = true; f.MainMoved = true }, Ownership{"owned", "job"}},
		{"active writers despite abandoned record", func(f *MergeFacts) { f.WritersSettled = false; f.State = "abandoned" }, Ownership{"owned", "writers"}},
		{"abandoned", func(f *MergeFacts) { f.State = "abandoned"; f.Job = "" }, Ownership{"abandoned", "state"}},
		{"ended job and code move", func(f *MergeFacts) { f.MainMoved = true }, Ownership{"abandoned", "main"}},
		{"main moved with unrecorded job", func(f *MergeFacts) { f.MainMoved = true; f.Job = ""; f.WritersSettled = false }, Ownership{"owned", "writers"}},
		{"current resolution", func(f *MergeFacts) {}, Ownership{"owned", "resolution"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facts := base
			tc.change(&facts)
			if got := MergeOwnership(facts); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestAdmissibility(t *testing.T) {
	t.Parallel()
	base := AdmissionFacts{Policy: "auto", Eligible: true, EntryState: "waiting"}
	for _, tc := range []struct {
		name   string
		change func(*AdmissionFacts)
		want   Admission
	}{
		{"unreadable", func(f *AdmissionFacts) { f.Unreadable = "queue unreadable" }, Admission{"unknown", "queue unreadable"}},
		{"person overrides cap", func(f *AdmissionFacts) { f.PersonSeal = true; f.CapReached = true }, Admission{"not admissible", "sealed by a person"}},
		{"person selection required", func(f *AdmissionFacts) { f.Selecting = true; f.Policy = "person" }, Admission{"not admissible", "the batch policy requires a person's selection"}},
		{"numeric selection", func(f *AdmissionFacts) { f.Selecting = true; f.Policy = "2" }, Admission{"admissible", "selection policy permits it"}},
		{"selection already sealed", func(f *AdmissionFacts) { f.Selecting = true; f.PersonSeal = true }, Admission{"not admissible", "sealed by a person"}},
		{"cap", func(f *AdmissionFacts) { f.CapReached = true }, Admission{"not admissible", "batch cap is one"}},
		{"policy absent", func(f *AdmissionFacts) { f.Policy = "" }, Admission{"unknown", "the admission policy is unrecorded"}},
		{"nonautomatic", func(f *AdmissionFacts) { f.Policy = "2" }, Admission{"not admissible", "the batch does not use automatic policy"}},
		{"selected", func(f *AdmissionFacts) { f.Selected = true }, Admission{"not admissible", "the goal is already selected in this batch"}},
		{"eligibility absent", func(f *AdmissionFacts) { f.Eligible = false }, Admission{"unknown", "the hand-in does not establish eligibility"}},
		{"returned", func(f *AdmissionFacts) { f.EntryState = "returned" }, Admission{"not admissible", "the goal is returned"}},
		{"landed", func(f *AdmissionFacts) { f.EntryState = "landed" }, Admission{"not admissible", "the goal is landed"}},
		{"held", func(f *AdmissionFacts) { f.Held = true; f.HoldReason = "incident" }, Admission{"not admissible", "the goal is held: incident"}},
		{"eligible", func(f *AdmissionFacts) {}, Admission{"admissible", "eligible under automatic admission"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facts := base
			tc.change(&facts)
			if got := Admissibility(facts); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestActor(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		facts ActorFacts
		want  Who
	}{
		{"unreadable enrollment", ActorFacts{Unreadable: "enrollment", Provenance: true}, Who{"unknown", "enrollment"}},
		{"recorded person", ActorFacts{Provenance: true, Lineage: "agent"}, Who{"person", "provenance"}},
		{"enrolled terminal", ActorFacts{Enrolled: true}, Who{"person", "enrollment"}},
		{"agent lineage", ActorFacts{Lineage: "landing-agent"}, Who{"agent", "lineage"}},
		{"legacy automatic return", ActorFacts{}, Who{"agent", "absent person"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Actor(tc.facts); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestQuestionPremise(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		facts PremiseFacts
		want  Premise
	}{
		{"unreadable", PremiseFacts{Unreadable: "main", Policy: true, Satisfied: true}, Premise{"unknown", "main"}},
		{"policy satisfied", PremiseFacts{Policy: true, Satisfied: true}, Premise{"gone", "policy effect"}},
		{"policy waiting", PremiseFacts{Policy: true}, Premise{"current", "policy"}},
		{"other lineage", PremiseFacts{ExpectedMain: "old", Contains: true}, Premise{"unknown", "lineage"}},
		{"checkout restored before main changed", PremiseFacts{LaneAgent: true, CheckoutOffMain: true, Branch: "main", ExpectedMain: "old", Main: "old"}, Premise{"gone", "checkout"}},
		{"main unchanged", PremiseFacts{LaneAgent: true, ExpectedMain: "old", Main: "old", CheckoutOffMain: true}, Premise{"current", "main"}},
		{"main advanced", PremiseFacts{LaneAgent: true, ExpectedMain: "old", Main: "new", Contains: true}, Premise{"gone", "main"}},
		{"checkout still off main", PremiseFacts{LaneAgent: true, CheckoutOffMain: true, Branch: "goal"}, Premise{"current", "checkout"}},
		{"main unrelated", PremiseFacts{LaneAgent: true, ExpectedMain: "old", Main: "other"}, Premise{"unknown", "premise"}},
		{"no explicit premise", PremiseFacts{LaneAgent: true}, Premise{"unknown", "premise"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := QuestionPremise(tc.facts); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestStopOwner(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		facts StopFacts
		want  Owner
	}{
		{"pause protects loop", StopFacts{Paused: true, Loop: "lane-proof"}, Owner{"person", "pause"}},
		{"recorded stop", StopFacts{StoppedBy: true}, Owner{"person", "stopped_by"}},
		{"person proof", StopFacts{PersonProof: true}, Owner{"person", "proof.person"}},
		{"unbounded steward", StopFacts{}, Owner{"loop", "loop"}},
		{"stop with unrecorded loop", StopFacts{Recorded: true}, Owner{"person", "loop"}},
		{"proof loop", StopFacts{Loop: "lane-proof"}, Owner{"loop", "loop"}},
		{"gate loop", StopFacts{Loop: "lane-gate"}, Owner{"loop", "loop"}},
		{"return loop", StopFacts{Loop: "lane-return"}, Owner{"loop", "loop"}},
		{"another owner's loop", StopFacts{Loop: "unit-round"}, Owner{"person", "loop"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := StopOwner(tc.facts); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
