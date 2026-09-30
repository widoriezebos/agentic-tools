package lane

import (
	"fmt"
)

// Operation is a lane operation the gate admits or refuses.
type Operation string

// The gated operations (design r10 K2), and a seat's join, which the unset
// fence refuses but a pause does not: a stopped lane holds seats, whose
// joins wait in it.
const (
	OpJoin     Operation = "join"
	OpBegin    Operation = "begin"
	OpProve    Operation = "prove"
	OpPublish  Operation = "publish"
	OpAdvance  Operation = "advance"
	OpValidate Operation = "validate"
	OpReturn   Operation = "return"
)

// CodeUnknownOperation is an operation the gate does not know: refused,
// because the gate fails closed.
const CodeUnknownOperation = "LANDING_LANE_UNKNOWN_OPERATION"

// Authority is who asks for an operation: the lane's agent (or a seat), or
// a person proven at an enrolled terminal.
type Authority string

const (
	AuthorityAgent  Authority = "agent"
	AuthorityPerson Authority = "person"
)

func knownOperation(op Operation) bool {
	switch op {
	case OpJoin, OpBegin, OpProve, OpPublish, OpAdvance, OpValidate, OpReturn:
		return true
	}
	return false
}

// Gate is the pause that holds (design r10 K2). Under the host lane flock,
// immediately before op acts, it reads the lane record, the unset fence and
// the pause, and runs start there when they admit op:
//
//   - no registered lane, or a record that cannot be read: refused;
//   - an unset under way (its journal present, readable or not): only a
//     person's cleanup return is admitted, joins are refused;
//   - a pause (present, readable or not): only a person's cleanup return
//     and a seat's join are admitted;
//   - an operation the gate does not know: refused.
//
// start only begins the work (starts a child, writes one record) and never
// waits for it, and it must not take the lane flock itself; nil admits
// without starting anything.
func Gate(home string, op Operation, authority Authority, start func(Record) error) error {
	if !knownOperation(op) {
		return &Refusal{Code: CodeUnknownOperation, Message: fmt.Sprintf("the landing lane does not know the operation %q, so nothing was started", op),
			Fix: "run one of the landing verbs: metasystem landing status lists them"}
	}
	return withLock(home, func() error {
		record, ok, err := Read(home)
		if err != nil {
			return err
		}
		if !ok {
			return &Refusal{Code: CodeNotRegistered, Message: "no landing lane is registered on this computer, so nothing was started",
				Fix: "a person registers the landing checkout: metasystem landing set PATH"}
		}
		cleanup := authority == AuthorityPerson && op == OpReturn
		if journal, fenced, _ := ReadUnset(home); fenced && !cleanup {
			return unsettingRefusal(journal)
		}
		if pause, paused := ReadPause(home); paused && !cleanup && op != OpJoin {
			return pausedRefusal(pause, op)
		}
		if start == nil {
			return nil
		}
		return start(record)
	})
}

func pausedRefusal(pause Pause, op Operation) *Refusal {
	return &Refusal{Code: CodePaused,
		Message: fmt.Sprintf("the landing lane is stopped by %s, so %s was not started", pausedBy(pause), op),
		Fix:     "a person resumes it: metasystem landing start",
		Argv:    []string{"metasystem", "landing", "start"}}
}

func pausedBy(pause Pause) string {
	if pause.By == "" {
		return "a person"
	}
	return pause.By
}

func unsettingRefusal(journal UnsetJournal) *Refusal {
	by := journal.By
	if by == "" {
		by = "a person"
	}
	return &Refusal{Code: CodeUnsetting,
		Message: "this computer's landing lane is being unset by " + by + ", so it takes no new work; nothing was done",
		Fix:     "the person finishes it with metasystem landing unset; after that each seat lands its own work",
		Argv:    []string{"metasystem", "landing", "unset"}}
}
