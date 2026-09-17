package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func goalBranchGit(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = gittree.ScrubbedEnviron()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func runGoalBranch(args []string) int {
	if len(args) == 0 || args[0] != "check" {
		fmt.Fprintln(os.Stderr, "goal branch needs the check verb")
		return 2
	}
	flags := flag.NewFlagSet("goal branch check", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	goalID := flags.String("goal", "", "goal id")
	tipFlag := flags.String("tip", "", "branch tip commit")
	noFetch := flags.Bool("no-fetch", false, "use existing remote-tracking refs")
	if flags.Parse(args[1:]) != nil || *goalID == "" || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "goal branch check needs --goal")
		return 2
	}
	endpoint, err := goal.ResolveEndpoint(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if endpoint.Branch != "refs/heads/main" {
		fmt.Fprintf(os.Stderr, "GOAL_BRANCH_ENDPOINT_UNSUPPORTED: endpoint %s is not refs/heads/main\n", endpoint.Branch)
		return 1
	}
	if !*noFetch {
		if _, err = goalBranchGit(*root, "fetch", "origin"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	endpointTip, err := goalBranchGit(*root, "rev-parse", "--verify", "refs/remotes/origin/main^{commit}")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	tipName := *tipFlag
	if tipName == "" {
		tipName = "refs/remotes/origin/goal/" + *goalID
	}
	tip, err := goalBranchGit(*root, "rev-parse", "--verify", "-q", tipName+"^{commit}")
	if err != nil {
		var exit *exec.ExitError
		if *tipFlag == "" && errors.As(err, &exit) && exit.ExitCode() == 1 {
			fmt.Println("no branch")
			return 0
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	commits, err := branch.ValidateRange(*root, endpointTip, tip, *goalID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, commit := range commits {
		fmt.Printf("%.12s %s %s %s\n", commit.ID, commit.Kind, commit.Unit, commit.Digest)
	}
	return 0
}
