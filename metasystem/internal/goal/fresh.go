package goal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
)

// Observation describes this invocation's read, independently of commit age.
type Observation struct {
	Tip        string    `json:"tip,omitempty"`
	ObservedAt time.Time `json:"observedAt"`
	Outcome    string    `json:"outcome"`
	Cause      string    `json:"cause,omitempty"`
}

// ContextRepository binds every repository read and ref effect to cancellation.
// Injected repositories must implement it to participate in fresh decisions.
type ContextRepository interface {
	WithContext(context.Context) Repository
}

// FreshProjection makes one bounded acceptance attempt and loads its immutable
// tip. Transport reserves time for reaping; ref cleanup has its own short bound.
func FreshProjection(ctx context.Context, e Endpoint, clock func() (time.Time, error)) (p Projection, observed Observation, err error) {
	return freshProjection(ctx, e, clock, freshProjectionTiming{now: time.Now, withTimeout: context.WithTimeout})
}

type freshProjectionTiming struct {
	now         func() time.Time
	withTimeout func(context.Context, time.Duration) (context.Context, context.CancelFunc)
}

func freshProjection(ctx context.Context, e Endpoint, clock func() (time.Time, error), timing freshProjectionTiming) (p Projection, observed Observation, err error) {
	ctx, cancel := timing.withTimeout(ctx, defaultFreshProjectionTimeout)
	defer cancel()
	deadline, _ := ctx.Deadline()
	source := e
	bind := func(c context.Context, bound time.Time) (Repository, error) {
		if source.Repository == nil {
			return gitRepository{endpoint: source, ctx: c, totalDeadline: bound}, nil
		}
		repository, ok := source.Repository.(ContextRepository)
		if !ok {
			return nil, errors.New("the ledger repository does not support cancellation")
		}
		return repository.WithContext(c), nil
	}
	defer func() {
		if err != nil {
			p = Projection{}
			observed.Outcome = "unavailable"
			observed.Cause = err.Error()
		}
	}()
	reader, err := bind(ctx, deadline)
	if err != nil {
		return p, observed, err
	}
	release := func(opid string) error {
		cleanupCtx, cleanupCancel := timing.withTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cleanupCancel()
		cleanupDeadline, _ := cleanupCtx.Deadline()
		cleanup, bindErr := bind(cleanupCtx, cleanupDeadline)
		if bindErr != nil {
			return bindErr
		}
		return cleanup.Release(opid)
	}
	allowance := max(0, deadline.Sub(timing.now()))
	transport := min(defaultFreshFetchProcessTimeout, allowance*3/4)
	transportCtx, transportCancel := timing.withTimeout(ctx, transport)
	defer transportCancel()
	capture, err := bind(transportCtx, deadline)
	if err != nil {
		return p, observed, err
	}
	e.Repository = reader
	advanced, err := fetchAdvance(e, capture.Capture, release)
	if err != nil {
		return p, observed, err
	}
	if err = ctx.Err(); err != nil {
		return p, observed, err
	}
	// An unchanged accepted tip still needs whole-tree validation for an
	// explicit fresh decision; display and hook readers keep their own rules.
	if !advanced.Advanced {
		if err = validateCommitFor(e, advanced.Tip); err != nil {
			return p, observed, err
		}
	}
	p, err = projectTipClock(e, advanced.Tip, clock)
	if err != nil {
		return p, observed, err
	}
	if err = ctx.Err(); err != nil {
		return p, observed, err
	}
	for i, banner := range p.Banners {
		if age, _, stale := strings.Cut(banner, "; goal list --fetch "); stale {
			p.Banners[i] = strings.Replace(age, "accepted tree", "accepted commit", 1) + "; freshly observed by this command"
		}
	}
	observed = Observation{Tip: advanced.Tip, ObservedAt: p.Horizon.Now, Outcome: "fresh"}
	return p, observed, nil
}

func (g gitRepository) run(cmd *exec.Cmd) error {
	if g.ctx == nil {
		return cmd.Run()
	}
	remaining := max(0, time.Until(g.totalDeadline))
	cmd.WaitDelay = remaining
	return boundedexec.RunContext(g.ctx, cmd, boundedexec.Bound{Limit: remaining, TotalDeadline: g.totalDeadline}, "ledger read")
}

func (g gitRepository) git(args ...string) (string, error) {
	if g.ctx == nil {
		return gitIn(g.endpoint.Root, args...)
	}
	cmd := commandWithEnvironment(g.endpoint.commandEnv, "git", append([]string{"-C", g.endpoint.Root, "-c", "core.logAllRefUpdates=false"}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := g.run(cmd); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return "", &gitError{ExitError: exit, stderr: strings.TrimSpace(stderr.String()), args: args}
		}
		return "", fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func gitExitCode(err error) int {
	var exit *gitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return -1
}

func (g gitRepository) capture(opid string) (string, error) {
	if g.endpoint.LocalMode() {
		if tip, err := g.git("rev-parse", "--verify", LocalLedgerBranch); err == nil {
			return strings.TrimSpace(tip), nil
		}
		tip, err := g.git("rev-parse", "--verify", "HEAD")
		return strings.TrimSpace(tip), err
	}
	if _, err := g.git("fetch", "--no-tags", "--refmap=", g.endpoint.Remote, "+"+g.endpoint.Branch+":"+fetchRefFor(opid)); err != nil {
		return "", err
	}
	tip, err := g.git("rev-parse", "--verify", fetchRefFor(opid))
	return strings.TrimSpace(tip), err
}

func (g gitRepository) deleteRefs(refs ...string) error {
	if len(refs) == 0 {
		return nil
	}
	cmd := commandWithEnvironment(g.endpoint.commandEnv, "git", "-C", g.endpoint.Root, "-c", "core.logAllRefUpdates=false", "update-ref", "--stdin")
	var input strings.Builder
	for _, ref := range refs {
		fmt.Fprintf(&input, "delete %s\n", ref)
	}
	cmd.Stdin = strings.NewReader(input.String())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := g.run(cmd); err != nil {
		return fmt.Errorf("delete temporary ledger refs: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
