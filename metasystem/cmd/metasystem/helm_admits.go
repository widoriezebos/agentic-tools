package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// wireHelmAdmission wires the person proof's helm seam once, at startup.
func wireHelmAdmission() {
	humanauthority.AtHelm = (&helmAdmitter{}).admit
}

// helmAdmitter answers humanauthority.AtHelm: while the seat is at the helm,
// a caller the person proof refused is the holder's act when the verb runs in
// the seat's primary checkout, the act's root is that seat, and no machinery
// (supervision, an adapter supervisor, the steward) is anywhere above the
// caller. Zero-valued owners are the production readers.
type helmAdmitter struct {
	owners helmAdmitOwners

	// One process is one act: the first admission is recorded and printed,
	// every later proof on the same seat in this process reuses it.
	mu       sync.Mutex
	admitted map[string]humanauthority.HelmGrant
}

// helmAdmitOwners are the facts helmAdmitter reads; tests fake them as they
// fake the pre-commit guard's.
type helmAdmitOwners struct {
	cwd       func() (string, error)
	git       func(dir string, args ...string) landpath.GitResult
	classify  func(root string, pid int64) (string, error)
	machinery func(root string, pid int64) (bool, error)
	yield     func(root string, y helm.Yield)
	verb      func() string
	stderr    io.Writer
}

func (o helmAdmitOwners) withDefaults() helmAdmitOwners {
	if o.cwd == nil {
		o.cwd = os.Getwd
	}
	if o.git == nil {
		o.git = func(dir string, args ...string) landpath.GitResult {
			return landingPathGit(landpath.GitCall{Dir: dir, Args: args})
		}
	}
	if o.classify == nil {
		o.classify = func(root string, pid int64) (string, error) {
			classification, err := lease.ClassifyAt(root, root, pid)
			return classification.Class, err
		}
	}
	if o.machinery == nil {
		o.machinery = lease.MachineryAncestor
	}
	if o.yield == nil {
		o.yield = helm.RecordYield
	}
	if o.verb == nil {
		o.verb = func() string { return publicVerb(os.Args[1:]) }
	}
	if o.stderr == nil {
		o.stderr = os.Stderr
	}
	return o
}

// publicVerb is the command's words before its first flag, at most three:
// the object, the action and its target ("goal open g1-s76").
func publicVerb(args []string) string {
	var words []string
	for _, arg := range args {
		if len(words) == 3 || strings.HasPrefix(arg, "-") {
			break
		}
		words = append(words, arg)
	}
	return strings.Join(words, " ")
}

func (a *helmAdmitter) admit(root string, pid int64) (humanauthority.HelmGrant, bool) {
	o := a.owners.withDefaults()
	cwd, err := o.cwd()
	if err != nil {
		return humanauthority.HelmGrant{}, false
	}
	state := helm.Active(cwd)
	if !state.Active {
		return humanauthority.HelmGrant{}, false
	}
	seat, rootErr := helm.Locate(root)
	here, cwdErr := helm.Locate(cwd)
	if rootErr != nil || cwdErr != nil || seat.CommonDir != here.CommonDir {
		return humanauthority.HelmGrant{}, false
	}
	key := seat.CommonDir
	a.mu.Lock()
	defer a.mu.Unlock()
	if grant, ok := a.admitted[key]; ok {
		return grant, true
	}
	if machinery, err := o.machinery(root, pid); err != nil || machinery {
		return humanauthority.HelmGrant{}, false
	}
	top := o.git(cwd, "rev-parse", "--show-toplevel")
	workTree := strings.TrimRight(string(top.Stdout), "\n")
	if top.Code != 0 || workTree == "" {
		return humanauthority.HelmGrant{}, false
	}
	common, primary := landpath.PrimaryCheckout(func(args ...string) landpath.GitResult { return o.git(workTree, args...) }, workTree)
	if !primary {
		return humanauthority.HelmGrant{}, false
	}
	class, err := o.classify(root, pid)
	if err != nil || class == "" {
		class = "unavailable"
	}
	verb := o.verb()
	o.yield(cwd, helm.Yield{Boundary: "person-proof", Gate: "human-proof", Would: "refuse",
		Subject: fmt.Sprintf("verb=%s class=%s cwd=%s", verb, class, cwd)})
	who := state.By
	if state.Malformed != "" {
		who = "signature unreadable"
	}
	fmt.Fprintf(o.stderr, "HUMAN AT THE HELM (%s): the person proof yields to the helm for %s; caller %s; recorded in %s\n",
		who, verb, class, filepath.Join(common, "metasystem", "helm-yields.log"))
	grant := humanauthority.HelmGrant{By: state.By, Since: state.Record.At, Class: class, Checkout: common}
	if a.admitted == nil {
		a.admitted = map[string]humanauthority.HelmGrant{}
	}
	a.admitted[key] = grant
	return grant, true
}
