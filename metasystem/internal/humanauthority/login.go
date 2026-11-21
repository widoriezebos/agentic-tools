package humanauthority

import (
	"errors"
	"runtime"
)

// LoginEvidence retains the linked terminal walk when a login rule admitted a
// withheld executable. Enrollment decisions use fresh process observations.
type LoginEvidence struct {
	Nodes []Node `json:"nodes,omitempty"`
}

// loginBranch tracks only the immediate SSH links and the requirement that a
// hidden account session be the highest process belonging to that account.
type loginBranch struct {
	account                                  uint32
	child, hiddenChild, monitor, aboveHidden bool
}

const linuxLogin = runtime.GOOS == "linux"

const sshSessionName = "sshd-session"

// observe returns the evidence required for a withheld executable, or the
// missing fact. The caller supplies only nodes strictly above the leader.
func (b *loginBranch) observe(n Node, hidden, end bool) (rule, missing string) {
	child := b.account != 0 && n.OwnerUID == b.account && n.KernelName == sshSessionName && !end
	monitor := b.child && n.OwnerUID == 0 && n.Ref.PID != 1 && n.KernelName == sshSessionName
	switch {
	case b.hiddenChild && !monitor:
		return "", "the login session has no immediate root-owned sshd-session parent"
	case b.aboveHidden && n.OwnerUID == b.account:
		return "", "another process owned by this account is above the hidden login session"
	case (b.aboveHidden || hidden) && end && (n.Ref.PID != 1 || n.OwnerUID != 0):
		return "", "the login chain does not reach process 1"
	case hidden:
		switch {
		case end && n.Ref.PID == 1 && n.OwnerUID == 0:
			rule = "pid-one"
		case child:
			rule = "ssh-session"
		case monitor:
			rule = "ssh-monitor"
		case b.monitor && !end && n.Ref.PID != 1 && n.OwnerUID == 0 && n.KernelName == "sshd":
			rule = "ssh-listener"
		case n.OwnerUID == b.account:
			return "", "the hidden account process lacks the required sshd-session name or parent"
		default:
			return "", "the hidden process is neither the system's first process nor a root SSH daemon for this login"
		}
	}
	b.child, b.hiddenChild, b.monitor = child, child && hidden, monitor
	b.aboveHidden = b.aboveHidden || hidden && rule != "pid-one"
	return rule, ""
}

const guardrail = "This check catches known agents in the terminal's process chain, but it cannot tell who sent the command."

// TerminalCheck is the limit of the Linux login observation as shown to its caller.
func TerminalCheck() string {
	if linuxLogin {
		return guardrail
	}
	return ""
}

// Method describes only what this generation observed on its own terminal.
func (e Enrollment) Method() string {
	if e.Login == nil {
		return "Enrollment method unrecorded."
	}
	for _, n := range e.Login.Nodes {
		if n.ExecutableWithheld == "ssh-session" || n.ExecutableWithheld == "ssh-monitor" || n.ExecutableWithheld == "ssh-listener" {
			return "Enrolled at an ssh login."
		}
	}
	if len(e.Login.Nodes) > 0 {
		return "Enrolled at a login whose system ancestors cannot be inspected."
	}
	return "Every program in the terminal's process chain was readable."
}

// loginFailure preserves the scope of a failed full terminal walk for its
// caller. A walk to an already enrolled shell has a different remedy.
type loginFailure struct{ error }

func (e loginFailure) Unwrap() error { return e.error }

func failedLogin(err error) bool {
	var login loginFailure
	return errors.As(err, &login)
}

// LoginRefusal names the observation that failed and where to retry the login.
func LoginRefusal(err error) (reason, remedy string) {
	outcome, agent := OutcomeOf(err)
	switch outcome {
	case OutcomeAgent:
		if agent != "" {
			return "an agent started this shell (" + agent + ")", "use your own login shell"
		}
		return "an agent started this shell", "use your own login shell"
	case OutcomeTerminalMissing:
		return "the terminal's session could not be reached", "use an interactive login with a terminal"
	}
	var unreadable *processReadRefusal
	if errors.As(err, &unreadable) {
		return unreadable.reason, "use your own interactive login shell"
	}
	if reason, known := outcomePlainWords[outcome]; known {
		return reason, "use your own interactive login shell"
	}
	return err.Error(), "use your own interactive login shell"
}
