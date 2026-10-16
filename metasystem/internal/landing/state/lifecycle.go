package state

import (
	"slices"
	"time"
)

type LifecycleFacts struct {
	Owner, Premise, Word, Attempt, CurrentAttempt, Subject, Detail string
	At                                                             time.Time
	ProcessGone, Empty, Latest                                     bool
	Subjects                                                       []string
	Merge                                                          MergeFacts
}
type LifecycleRow struct {
	Kind, Retires, To string
	Due               func(LifecycleFacts, time.Time) (bool, string)
}

const SubmissionWindow = 24 * time.Hour
const ResultRetention = 14 * SubmissionWindow

// Lifecycle owns retirement decisions; readers supply facts from the records.
var Lifecycle = []LifecycleRow{
	{"loop-stop", Phrase("closes-when-its-subjects-landed-returned-or-were-superseded"), "closed", func(f LifecycleFacts, _ time.Time) (bool, string) {
		if f.Owner != Loop {
			return false, ""
		}
		if f.Attempt != "" && f.CurrentAttempt != "" && f.Attempt != f.CurrentAttempt {
			return true, "its attempt is no longer current"
		}
		return subjectsEnded(f.Subjects), "its subject " + f.Subject + " " + f.Detail
	}},
	{"person-stop", "never retires automatically", "", func(LifecycleFacts, time.Time) (bool, string) { return false, "" }},
	{"question", "withdraws when its premise is gone; a person's withdrawal remains a decision for the same stop identity", "withdrawn", func(f LifecycleFacts, _ time.Time) (bool, string) {
		return f.Premise == Gone, "its premise is gone; " + f.Detail
	}},
	{"fix", Phrase("closes-when-done-superseded-by-another-proof-or-its"), "closed", func(f LifecycleFacts, _ time.Time) (bool, string) {
		if f.Word == Abandoned {
			return subjectsEnded(f.Subjects), "its subject " + f.Subject + " " + f.Detail
		}
		owner := MergeOwnership(f.Merge).Word
		return f.Word == "done" || f.Attempt != "" && f.CurrentAttempt != "" && f.Attempt != f.CurrentAttempt || owner == Settled, "its repair ended or was superseded"
	}},
	{"submission", Phrase("leaves-the-views-after-the-day-window-when-landed"), "retired", func(f LifecycleFacts, now time.Time) (bool, string) {
		return f.Word == "superseded" || (f.Word == "returned" || f.Word == "landed") && !f.At.IsZero() && now.Sub(f.At) > SubmissionWindow, "its completed submission is outside the day window"
	}},
	{"result", "leaves the views after fourteen days unless it is the latest of its kind; results.jsonl retains history", "retired", func(f LifecycleFacts, now time.Time) (bool, string) {
		return !f.Latest && !f.At.IsZero() && now.Sub(f.At) > ResultRetention, "its result is outside retention"
	}},
	{"agent-step", "retires when its command process is gone", "retired", func(f LifecycleFacts, _ time.Time) (bool, string) {
		return f.ProcessGone, "its command process is gone"
	}},
	{"batch", "retires when closure is recorded", "retired", func(f LifecycleFacts, _ time.Time) (bool, string) {
		return f.Word == "closed", "its batch closure is recorded"
	}},
}

func subjectsEnded(subjects []string) bool {
	return len(subjects) > 0 && !slices.ContainsFunc(subjects, func(word string) bool {
		return !slices.Contains([]string{"landed", "returned", "superseded"}, word)
	})
}

func LifecycleDue(kind string, facts LifecycleFacts, now time.Time) (bool, string) {
	for _, row := range Lifecycle {
		if row.Kind == kind {
			return row.Due(facts, now)
		}
	}
	return false, ""
}
