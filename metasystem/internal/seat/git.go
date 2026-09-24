package seat

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
)

// TickNamespace is the clone's canonical local copy of the fleet's presence,
// fetched once per tick by the seat-presence component. Every reader has a
// namespace of its own, because two concurrent fetches into ONE shared ref
// collide on that ref's lock exactly as the ledger's per-operation fetch refs
// exist to prevent.
const TickNamespace = "refs/metasystem/presence-copy"

// FetchNamespacePrefix is where `seat fleet --fetch` puts its own throwaway
// copy, one namespace per read, deleted before the verb exits. A stale one
// left by an interrupted read is harmless and collected by the next run.
const FetchNamespacePrefix = "refs/metasystem/presence-fetch"

// TransportBudget is the deadline every presence fetch and push runs under:
// the ledger fetch's own budget (internal/steward/ledgerattention.go:81), so
// a hung remote ends as a failed attempt and never as a hung tick.
const TransportBudget = 60 * time.Second

// subNamespace names the local sub-namespace one remote namespace lands in.
// Readers do not need to know which rung a machine publishes on: the fetch
// carries both remote namespaces, and Join picks the newest record.
func subNamespace(namespace, remote string) string {
	if remote == BranchNamespace {
		return namespace + "/heads"
	}
	return namespace + "/metasystem"
}

// Git is the presence transport: bounded git, on an owned process group.
type Git struct {
	// Root is the checkout every git invocation runs in.
	Root string
	// Remote is the ledger remote; Local is goal.sync-remote == "local",
	// in which case nothing is fetched or pushed and the local ref is
	// updated in place, so a single-machine seat still sees itself.
	Remote string
	Local  bool
	// Budget bounds every invocation; zero takes TransportBudget.
	Budget time.Duration
}

func (g Git) budget() boundedexec.Bound {
	limit := g.Budget
	if limit <= 0 {
		limit = TransportBudget
	}
	return boundedexec.FixedBound(limit, "the ledger fetch budget")
}

// run executes one git invocation under the bound, on its own process group.
func (g Git) run(what string, stdin []byte, args ...string) (string, string, error) {
	full := append([]string{"-C", g.Root, "-c", "core.logAllRefUpdates=false"}, args...)
	command := exec.Command("git", full...)
	// A credential prompt would outlive every bound this package sets.
	command.Env = append(command.Environ(), "GIT_TERMINAL_PROMPT=0")
	if stdin != nil {
		command.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := boundedexec.Run(command, g.budget(), what)
	if err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("%s: %w (%s)", what, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), stderr.String(), nil
}

// Fetch carries both remote presence namespaces into two sub-namespaces of
// the reader's own. --refmap= keeps them out of the remote-tracking refs,
// --atomic makes a multi-ref update all or nothing so a failed fetch leaves
// no half-updated namespace, and --prune with these refspecs removes only
// namespace refs whose remote ref is gone, so an operator's deletion reaches
// every reader. In LocalMode nothing is fetched and nothing is pruned.
func (g Git) Fetch(namespace string) error {
	if g.Local {
		return nil
	}
	args := []string{"fetch", "--no-tags", "--refmap=", "--atomic", "--prune", g.Remote,
		"+" + MetasystemNamespace + "/*:" + subNamespace(namespace, MetasystemNamespace) + "/*",
		"+" + BranchNamespace + "/*:" + subNamespace(namespace, BranchNamespace) + "/*",
	}
	_, _, err := g.run("presence fetch", nil, args...)
	return err
}

// Read returns the joined presence one namespace holds. In LocalMode it
// reads the publishing refs themselves, because nothing was fetched.
func (g Git) Read(namespace string) (Copy, error) {
	prefixes := []string{subNamespace(namespace, MetasystemNamespace), subNamespace(namespace, BranchNamespace)}
	if g.Local {
		prefixes = []string{MetasystemNamespace, BranchNamespace}
	}
	copies := make([]Copy, 0, len(prefixes))
	for _, prefix := range prefixes {
		one, err := g.readPrefix(prefix)
		if err != nil {
			return Copy{}, err
		}
		copies = append(copies, one)
	}
	return Join(copies...), nil
}

// Tips reports the commit each machine's ref holds under one remote
// namespace as this clone last read it — the parent a fast-forward push
// needs.
func (g Git) Tips(namespace, remote string) (map[string]string, error) {
	prefix := subNamespace(namespace, remote)
	if g.Local {
		prefix = remote
	}
	out, _, err := g.run("presence ref list", nil, "for-each-ref", "--format=%(refname)%09%(objectname)", prefix)
	if err != nil {
		return nil, err
	}
	tips := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		name, object, found := strings.Cut(strings.TrimSpace(line), "\t")
		if !found || name == "" {
			continue
		}
		tips[name[strings.LastIndex(name, "/")+1:]] = object
	}
	return tips, nil
}

func (g Git) readPrefix(prefix string) (Copy, error) {
	out, _, err := g.run("presence ref list", nil, "for-each-ref", "--format=%(refname)", prefix)
	if err != nil {
		return Copy{}, err
	}
	copied := Copy{Records: map[string]Record{}, Malformed: map[string]string{}}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		ref := strings.TrimSpace(line)
		if ref == "" {
			continue
		}
		machine := ref[strings.LastIndex(ref, "/")+1:]
		blob, _, readErr := g.run("presence read", nil, "cat-file", "blob", ref+":presence.json")
		if readErr != nil {
			copied.Malformed[machine] = "SEAT_PRESENCE_MALFORMED: the presence ref carries no readable presence.json"
			continue
		}
		record, parseErr := ParseRecord([]byte(blob))
		if parseErr != nil {
			copied.Malformed[machine] = parseErr.Error()
			continue
		}
		if record.Machine != machine {
			copied.Malformed[machine] = fmt.Sprintf("SEAT_PRESENCE_MALFORMED: the record on %s names machine %q", ref, record.Machine)
			continue
		}
		copied.Records[machine] = record
	}
	return copied, nil
}

// Publish writes the record as the whole tree of a fresh commit and moves
// ref to it: force and parentless when parent is empty, a child of parent
// and fast-forward otherwise. In LocalMode the ref is updated locally and
// nothing is pushed.
func (g Git) Publish(ref, message string, file []byte, parent string) (string, error) {
	blob, _, err := g.run("presence blob", file, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", err
	}
	tree, _, err := g.run("presence tree", []byte("100644 blob "+strings.TrimSpace(blob)+"\tpresence.json\n"), "mktree")
	if err != nil {
		return "", err
	}
	args := []string{"commit-tree", strings.TrimSpace(tree), "-m", message}
	if parent != "" {
		args = append(args, "-p", parent)
	}
	commit, _, err := g.run("presence commit", nil, args...)
	if err != nil {
		return "", err
	}
	object := strings.TrimSpace(commit)
	if g.Local {
		if _, _, err := g.run("presence ref update", nil, "update-ref", ref, object); err != nil {
			return "", err
		}
		return object, nil
	}
	spec := object + ":" + ref
	if parent == "" {
		spec = "+" + spec
	}
	stdout, stderr, err := g.run("presence push", nil, "push", g.Remote, spec)
	if err != nil {
		if errors.Is(err, boundedexec.ErrTimedOut) {
			return "", err
		}
		if refusalNamesRef(ref, stdout+stderr) {
			return "", &RefRefused{Ref: ref, Detail: firstLine(stdout + stderr)}
		}
		return "", err
	}
	return object, nil
}

// DeleteNamespace removes every ref under one namespace, which is how
// `seat fleet --fetch` cleans up the copy it made.
func (g Git) DeleteNamespace(namespace string) error {
	out, _, err := g.run("presence ref list", nil, "for-each-ref", "--format=%(refname)", namespace)
	if err != nil {
		return err
	}
	var refs []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if ref := strings.TrimSpace(line); ref != "" {
			refs = append(refs, ref)
		}
	}
	if len(refs) == 0 {
		return nil
	}
	sort.Strings(refs)
	var batch strings.Builder
	for _, ref := range refs {
		batch.WriteString("delete " + ref + "\n")
	}
	_, _, err = g.run("presence ref delete", []byte(batch.String()), "update-ref", "--stdin")
	return err
}

func firstLine(output string) string {
	trimmed := strings.TrimSpace(output)
	if index := strings.IndexByte(trimmed, '\n'); index >= 0 {
		return strings.TrimSpace(trimmed[:index])
	}
	return trimmed
}
