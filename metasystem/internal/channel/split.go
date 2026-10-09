package channel

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// PostSplitApproval asks for approval of held children once per committed
// transaction. The caller supplies the confirmed tree, never a draft.
func PostSplitApproval(ctx context.Context, e goal.Endpoint, tip, parent, transaction string, children []string, p Provider, d DestinationConfig, now time.Time) error {
	children = slices.Clone(children)
	slices.Sort(children)
	notice := LandedNotice{SHA: "split:" + transaction + ":" + strings.Join(children, ","), SplitParent: parent, SplitChildren: children, At: now.UTC()}
	projection, err := goal.ProjectAtEndpoint(e, tip, now)
	if err != nil {
		return err
	}
	text, err := splitApprovalText(projection.Tree, notice)
	if err != nil || text == "" {
		return err
	}
	notice.Text = text
	var retryErr error
	if p != nil {
		retryErr = RetrySplitApproval(ctx, e, p, d, now)
	}
	return withLandedState(e.Root, func(s *LandedState) error {
		if s.known(notice.SHA) {
			return retryErr
		}
		var err error
		if p == nil {
			err = ErrUnconfigured("no approval channel configured")
		} else {
			_, err = p.Post(ctx, d, text, nil)
		}
		if err != nil {
			notice.Error = err.Error()
			s.Pending = append(s.Pending, notice)
			return errors.Join(retryErr, fmt.Errorf("approval request not sent: %w\n%s", err, text))
		}
		s.Posted = append(s.Posted, notice.SHA)
		return retryErr
	})
}

// RetrySplitApproval shares the channel's bounded retry, refreshing the
// approval and hold from the accepted ledger before offering a message.
func RetrySplitApproval(ctx context.Context, e goal.Endpoint, p Provider, d DestinationConfig, now time.Time) error {
	if !slices.ContainsFunc(LoadLandedState(e.Root).Pending, func(n LandedNotice) bool { return n.SplitParent != "" }) {
		return nil
	}
	return withLandedState(e.Root, func(s *LandedState) error {
		projection, err := goal.Project(e, true, now)
		return retryPending(ctx, s, p, d, func(n LandedNotice) (string, error) {
			if err != nil {
				return "", err
			}
			return splitApprovalText(projection.Tree, n)
		})
	})
}

func splitApprovalText(tree *goal.TreeGoals, notice LandedNotice) (string, error) {
	parent := tree.Live[notice.SplitParent]
	if parent == nil || (parent.Split != nil && parent.State != goal.StateSplit) {
		return "", nil
	}
	if parent.Split != nil && notice.SHA != "split:"+parent.Split.Transaction+":"+strings.Join(parent.Split.Children, ",") {
		return "", nil
	}
	var children, commands, details []string
	for _, id := range notice.SplitChildren {
		child := tree.Live[id]
		if child == nil {
			if _, archived := tree.Archived(id); archived {
				continue
			}
			return "", fmt.Errorf("approval request cannot read child %s", id)
		}
		if child.Approved != nil || child.State == goal.StateParked || !slices.Contains(child.Blocked, parent.Id) {
			continue
		}
		if parent.Split != nil && (child.SplitFrom != parent.Id || !slices.Contains(parent.Split.Children, id)) {
			return "", errors.New("approval request cannot confirm the child and source hold")
		}
		children = append(children, id)
		commands = append(commands, "metasystem goal approve "+id)
		remaining := slices.DeleteFunc(slices.Clone(child.Blocked), func(id string) bool { return id == parent.Id })
		detail := "After approval, deliberately release the source hold with metasystem goal unblock " + id + " --on " + parent.Id + "."
		if len(remaining) != 0 {
			detail += " Remaining prerequisites: " + strings.Join(remaining, ", ") + "."
		}
		details = append(details, detail)
	}
	if len(children) == 0 {
		return "", nil
	}
	return fmt.Sprintf("Work from goal %s needs your approval: %s are unapproved and held by %s.\n%s\nApproval alone keeps the source hold. Supply each child's ordinary risk and budget information before approving.\n%s", parent.Id, strings.Join(children, ", "), parent.Id, strings.Join(commands, "; "), strings.Join(details, "\n")), nil
}
