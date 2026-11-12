package steward

import (
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
)

type FollowRefusal struct {
	Commit, Reason, Command string
	Waiting                 bool
}

const FollowDeploymentUnavailable = "deployment-unavailable"

func TellFollow(root, machine string, refusal *FollowRefusal, now time.Time) (refused bool, err error) {
	if refusal != nil && refusal.Waiting {
		return false, nil
	}
	questions, unreadable := channel.WalkQuestions(root)
	if len(unreadable) > 0 {
		return false, fmt.Errorf("question store: %s", strings.Join(unreadable, "; "))
	}
	because := "the seat runs the deployed engine"
	if refusal != nil {
		because = "the deployed engine changed"
	}
	for _, q := range questions {
		if q.Machine == machine && q.Decision != nil && q.Decision.Kind == "engine-follow" && q.RetiredAt.IsZero() && (refusal == nil || q.Decision.State != refusal.Commit) {
			if err := channel.Retire(root, q.ID, because, now); err != nil {
				return false, err
			}
		}
	}
	if refusal == nil {
		return false, nil
	}
	decision := fmt.Sprintf("Seat %s could not install the deployed engine %.12s: %s", machine, refusal.Commit, refusal.Reason)
	if refusal.Commit == FollowDeploymentUnavailable {
		decision = fmt.Sprintf("Seat %s cannot read this project's deployed engine: %s", machine, refusal.Reason)
	}
	_, _, err = channel.AskOrFind(channel.AskRequest{RepoRoot: root, About: "machine", Kind: "other", Machine: machine, Now: now, Wants: refusal.Command,
		Decision: &channel.Decision{Kind: "engine-follow", Subject: machine, State: refusal.Commit}, Facts: []string{refusal.Commit},
		Plain: &channel.Plain{Decision: decision, IfNothing: "the seat keeps its current engine and follows by itself once the cause clears; this question then closes"}})
	return err == nil, err
}
