package goal

import (
	"fmt"
	"slices"
	"strings"
)

type DesignItem struct {
	Exit, DesignID, BodySHA256, Unit, Decision string
	Tests                                      []string
}

type DesignExit struct {
	Operation, DesignID, BodySHA256 string
	Units, Items                    []string
	Destination, OpenCommand        string
}

func (f *GoalFile) CheckDesignAcceptance(operation, design, body string) error {
	if f != nil {
		for i := len(f.DesignExits) - 1; i >= 0; i-- {
			exit := f.DesignExits[i]
			if exit.DesignID != design {
				continue
			}
			if exit.Operation != operation || exit.BodySHA256 != body || len(exit.Units) == 0 {
				break
			}
			var items []string
			for _, o := range f.ReviewObligations {
				if o.DesignItem == nil || o.DesignItem.Exit != operation {
					continue
				}
				item := o.DesignItem
				if item.DesignID != design || item.BodySHA256 != body || !slices.Contains(exit.Units, item.Unit) || item.Decision == "" || len(item.Tests) == 0 || o.Fixture == "" {
					return fmt.Errorf("design %s has an incomplete acceptance item", design)
				}
				items = append(items, o.Finding)
			}
			slices.Sort(items)
			expected := slices.Clone(exit.Items)
			slices.Sort(expected)
			if slices.Equal(items, expected) {
				return nil
			}
			break
		}
	}
	return fmt.Errorf("design %s lacks its accepted body or required items; resume its publication", design)
}

func (f *GoalFile) DesignCompletionProblem() (string, []string) {
	for _, o := range f.ReviewObligations {
		if o.State == "open" && (o.DesignItem != nil || o.Fixture != "") {
			return fmt.Sprintf("required design finding %s remains open", o.Finding), []string{"metasystem", "work", "review", f.Id, "--review", o.Chain, "--finding", o.Finding, "--implementation-chain", "CHAIN", "--artifact", o.Artifact, "--result", "RUN", "--critic", "ROOT"}
		}
	}
	return "", nil
}

func (f *GoalFile) DesignDestinationProblem(tree *TreeGoals) string {
	for _, exit := range f.DesignExits {
		if exit.Destination != "" && tree.Live[exit.Destination] == nil && tree.Done[exit.Destination] == nil && tree.Abandoned[exit.Destination] == nil {
			return "the split destination has not been opened; run " + strings.TrimSpace(exit.OpenCommand)
		}
	}
	return ""
}
