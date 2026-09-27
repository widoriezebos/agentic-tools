package delegation

import (
	"errors"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

func asOpError(err error, target **dispatch.OpError) bool { return errors.As(err, target) }

// route is dispatch.sh's command router: the legacy grammar fence, then one
// command or internal callback.
func (s *session) route(argv []string) error {
	if !s.env.DelegateInternal {
		act := "dispatch"
		if len(argv) > 0 && !strings.HasPrefix(argv[0], "--") {
			act = argv[0]
		}
		switch act {
		case "dispatch", "follow-up", "cancel", "close", "reap":
			fenced, err := s.brainFenceOutcome(act)
			if err != nil {
				return err
			}
			if fenced != "" {
				s.println(fenced)
				return exitWith(2)
			}
		}
		first := "dispatch"
		if len(argv) > 0 {
			first = argv[0]
		}
		if first == "dispatch" || first == "follow-up" || first == "cancel" || strings.HasPrefix(first, "--") {
			s.println(`{"outcome":"REFUSED-REQUEST","headline":"refused","detail":"the legacy dispatch authority grammar was removed; use metasystem internal delegate"}`)
			return exitWith(2)
		}
	}
	command := ""
	if len(argv) > 0 {
		command = argv[0]
	}
	args := argv
	if strings.HasPrefix(command, "--") {
		command = "dispatch"
	} else if len(args) > 0 {
		args = args[1:]
	}
	switch command {
	case "__engine-skew-preflight":
		if len(args) > 1 {
			return exitWith(2)
		}
		stamp := ""
		if len(args) == 1 {
			stamp = args[0]
		}
		return s.engineSkewPreflight(stamp)
	case "dispatch":
		return s.dispatchJob(args)
	case "watch":
		return s.watchJob(args)
	case "follow-up":
		return s.followUp(args)
	case "status":
		return s.statusJob(args)
	case "cancel":
		return s.cancelJob(args)
	case "close":
		return s.closeChain(args)
	case "reap":
		return s.reapJobs(args)
	case "-h", "--help":
		s.eprintln(usageText)
		return nil
	}
	if strings.HasPrefix(command, "__") {
		return s.callback(command, args)
	}
	return s.usageExit()
}

// brainFenceOutcome is brain_fence_outcome: the typed BRAIN_REFUSED line when
// the act is fenced, empty when it is not.
func (s *session) brainFenceOutcome(act string) (string, error) {
	detail := s.brainFence(act)
	if detail == "" {
		return "", nil
	}
	return encodeObject(map[string]string{"outcome": "BRAIN_REFUSED", "headline": "refused", "detail": detail}), nil
}

// fenceBrain is the guard at the head of dispatch, follow-up, cancel, close
// and reap: a fenced act prints and records its typed refusal and exits 2.
func (s *session) fenceBrain(act string) error {
	outcome, err := s.brainFenceOutcome(act)
	if err != nil {
		return err
	}
	if outcome == "" {
		return nil
	}
	s.recordOutcomeRaw(outcome)
	s.println(outcome)
	return exitWith(2)
}

// encodeObject is `json object k=v ...`: a compact object of strings,
// HTML characters unescaped.
func encodeObject(fields map[string]string) string {
	pairs := make([]string, 0, len(fields))
	for key, value := range fields {
		pairs = append(pairs, key+"="+value)
	}
	line, _ := jsonObject(pairs)
	return line
}
