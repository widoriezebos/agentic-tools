package goal

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

type DesignItem struct {
	Read, Finding, Passage, Checklist, Requirement, Class string
	Evidence                                              readsubject.Finding
	Exit, DesignID, BodySHA256, Unit, Decision            string
	Tests                                                 []string
}

type DesignExit struct {
	Obligations                                     []ReviewObligation
	Operation, DesignID, BodySHA256                 string
	Units, Items                                    []string
	Destination, OpenCommand                        string
	DestinationBrief                                string
	DestinationBriefPath                            string
	TransferUnits                                   []string
	TransferObligations                             []ReviewObligation
	AuthorAttempt                                   int
	AuthorPrior                                     string
	Root, ExaminedSHA256, DispositionsSHA256, State string
	Round                                           int64
	Revision                                        uint64
	Expected, Page, Dispositions                    string
	Who, Ruling, Reason, Impact, At                 string
}

// OpenDesignDestination opens only the scope retained by a committed split.
// The destination waits for the source and still needs a person's approval.
func OpenDesignDestination(r VerbRequest, source, operation string) (PublishResult, error) {
	req, err := designDestinationRequest(r, source, operation)
	if err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, req)
}

func designDestinationRequest(r VerbRequest, source, operation string) (PublishRequest, error) {
	projection, err := Project(r.Endpoint, false, r.Now)
	if err != nil {
		return PublishRequest{}, err
	}
	f := projection.Tree.Live[source]
	if f == nil {
		return PublishRequest{}, fmt.Errorf("split source %s is not live", source)
	}
	var exit *DesignExit
	for i := range f.DesignExits {
		if f.DesignExits[i].Operation == operation {
			exit = &f.DesignExits[i]
		}
	}
	if exit == nil || exit.State != "committed" || exit.Destination == "" || exit.DestinationBrief == "" || exit.DestinationBriefPath == "" || f.Origin != OriginHuman || f.Approved == nil {
		return PublishRequest{}, fmt.Errorf("the follow-up needs a committed split of a person-created, approved source")
	}
	req, err := openRequest(r, exit.Destination, "Reshape transferred design scope from "+source, OriginMain, "Reshape scope in "+exit.DestinationBriefPath, nil, []string{source}, f.Tier, nil, f.Risk, "", false, false, nil)
	if err != nil {
		return PublishRequest{}, err
	}
	req.Intent = Intent{Verb: "design-destination", Targets: []string{exit.Destination}, Args: intentArgs(r, map[string]string{"source": source, "operation": operation})}
	mutate := req.Mutate
	req.Mutate = func(tip string) ([]Change, error) {
		tree, err := loadTreeFor(r.Endpoint, tip)
		if err != nil {
			return nil, err
		}
		current := tree.Live[source]
		if current == nil || current.Origin != OriginHuman || current.Approved == nil || !slices.ContainsFunc(current.DesignExits, func(e DesignExit) bool { return reflect.DeepEqual(e, *exit) }) {
			return nil, fmt.Errorf("the committed split changed; reconcile before opening its destination")
		}
		return mutate(tip)
	}
	return req, nil
}

// PublishDesignExit commits acceptance before any document can expose it.
func PublishDesignExit(r VerbRequest, id string, exit DesignExit, admit func() error) (PublishResult, error) {
	exit.State = "committed"
	return Publish(r.Endpoint, PublishRequest{Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "design-exit", Targets: []string{id}}, Message: "Publish design acceptance for " + id,
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTreeFor(r.Endpoint, tip)
			if err != nil {
				return nil, err
			}
			f := tree.Live[id]
			if f == nil {
				return nil, fmt.Errorf("goal %s is not live", id)
			}
			for _, existing := range f.DesignExits {
				if existing.Operation == exit.Operation {
					exit.Revision = existing.Revision
					if !reflect.DeepEqual(existing, exit) {
						return nil, fmt.Errorf("design exit %s conflicts with its committed acceptance", exit.Operation)
					}
					return nil, AlreadyHolds{Reason: "the design acceptance is committed"}
				}
			}
			if exit.Who != "" {
				if r.Authority == nil || r.Authority.Helm != nil || !r.Authority.EnrolledTerminalFor(r.Endpoint.Root) {
					return nil, fmt.Errorf("only the person at their own terminal can rule on a design")
				}
				if exit.Who != r.Actor.Human {
					return nil, fmt.Errorf("the design ruling names another person")
				}
			} else if f.State != StateClaimed || !ownPair(f.Claimed, r.Actor) || f.Approved == nil || f.StopFence != nil {
				return nil, fmt.Errorf("goal %s changed or is not held by this session; acceptance remains pending", id)
			}
			if err := admit(); err != nil {
				return nil, err
			}
			exit.Revision = f.Revision
			for _, item := range exit.Obligations {
				if item.DesignItem == nil || item.DesignItem.Exit != exit.Operation || item.DesignItem.BodySHA256 != exit.BodySHA256 || !slices.Contains(exit.Items, item.Finding) {
					return nil, fmt.Errorf("the prepared acceptance item is incomplete")
				}
				f.ReviewObligations = append(f.ReviewObligations, item)
			}
			f.DesignExits = append(f.DesignExits, exit)
			touch(f, r, "design-exit", []string{id})
			return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
		}, Validate: func(commit string) error { return validateCommitFor(r.Endpoint, commit) }})
}

func (f *GoalFile) CheckDesignAcceptance(operation, design, body string) error {
	if f != nil {
		for i := len(f.DesignExits) - 1; i >= 0; i-- {
			exit := f.DesignExits[i]
			if exit.DesignID != design {
				continue
			}
			if exit.Operation != operation || exit.BodySHA256 != body || len(exit.Units) == 0 || exit.State != "" && exit.State != "committed" {
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
