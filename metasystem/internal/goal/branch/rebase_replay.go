package branch

import (
	"bytes"
	"maps"
	"path/filepath"
	"strings"
)

// replayWithoutCheckout computes objects only. Every uncertainty returns to
// the scratch rebase, which owns conflict handling and regeneration.
func replayWithoutCheckout(req RebaseRequest, local string, d rebaseDependencies) (string, bool) {
	git := d.git
	config, err := git(req.Repo, "config", "--null", "--list", "--show-scope")
	if err != nil || !replayConfigEquivalent(config) {
		return "", false
	}
	input, ok := replayCommits(req.Repo, req.EndpointTip, local, git)
	if !ok || len(input) == 0 {
		return "", false
	}
	for _, c := range input {
		if c.signed {
			return "", false
		}
	}
	if !replayAttributesUnchanged(req.Repo, req.EndpointTip, local, git) {
		return "", false
	}
	// Top-anchored pathspecs include attributes outside a nested installation.
	attrs := []string{"--", ":(top).gitattributes", ":(top,glob)**/.gitattributes"}
	args := append([]string{"diff", "--quiet", req.EndpointTip}, attrs...)
	if _, err := git(req.Repo, args...); err != nil {
		return "", false
	}
	args = append([]string{"ls-files", "--others", "-z"}, attrs...)
	if out, err := git(req.Repo, args...); err != nil || len(out) != 0 {
		return "", false
	}

	// A ref name lets replay print the proposed update. The captured object
	// range above and the old object below protect against a moved ref.
	out, err := git(req.Repo, "-c", "replay.refAction=print", "replay", "--onto", req.EndpointTip, req.EndpointTip+".."+goalBranchRef(req.GoalID))
	fields := strings.Fields(string(out))
	if err != nil || len(fields) != 4 || fields[0] != "update" || fields[1] != goalBranchRef(req.GoalID) || !hex40(fields[2]) || fields[3] != local {
		return "", false
	}
	next := fields[2]
	output, ok := replayCommits(req.Repo, req.EndpointTip, next, git)
	if !ok || len(output) != len(input) {
		return "", false
	}
	endpointTree, err := git(req.Repo, "rev-parse", req.EndpointTip+"^{tree}")
	if err != nil || !hex40(strings.TrimSpace(string(endpointTree))) {
		return "", false
	}
	parent, parentTree := req.EndpointTip, strings.TrimSpace(string(endpointTree))
	for i, c := range output {
		if c.parent != parent || c.author != input[i].author || !bytes.Equal(c.message, input[i].message) {
			return "", false
		}
		if c.tree == parentTree {
			// Git versions differ on commits that become empty. Keep their
			// preservation on the scratch path even when replay retained one.
			if _, err := git(req.Repo, "diff-tree", "--quiet", input[i].parent, input[i].id); err != nil {
				return "", false
			}
		}
		parent, parentTree = c.id, c.tree
	}
	if !replayAttributesUnchanged(req.Repo, req.EndpointTip, next, git) {
		return "", false
	}
	if _, err := d.repository.facts.Range(req.Repo, req.EndpointTip, next, req.GoalID); err != nil {
		return "", false
	}
	return next, true
}

type replayCommit struct {
	id, parent, author, tree string
	message                  []byte
	signed                   bool
}

// Read the unfiltered sequence: ValidateRange intentionally omits some empty
// builds and landed reads, whereas rebase's --empty=keep preserves them.
func replayCommits(repo, endpoint, tip string, git func(string, ...string) ([]byte, error)) ([]replayCommit, bool) {
	out, err := git(repo, "rev-list", "--reverse", "--topo-order", endpoint+".."+tip)
	if err != nil {
		return nil, false
	}
	var commits []replayCommit
	for _, id := range strings.Fields(string(out)) {
		if !hex40(id) {
			return nil, false
		}
		raw, err := git(repo, "cat-file", "commit", id)
		header, message, found := bytes.Cut(raw, []byte("\n\n"))
		if err != nil || !found {
			return nil, false
		}
		c := replayCommit{id: id, message: message}
		parents, authors, trees := 0, 0, 0
		for _, line := range strings.Split(string(header), "\n") {
			if strings.HasPrefix(line, "tree ") {
				trees++
				c.tree = strings.TrimPrefix(line, "tree ")
			}
			if strings.HasPrefix(line, "parent ") {
				parents++
				c.parent = strings.TrimPrefix(line, "parent ")
			}
			if strings.HasPrefix(line, "author ") {
				authors++
				c.author = line
			}
			if strings.HasPrefix(line, "gpgsig ") || strings.HasPrefix(line, "gpgsig-sha256 ") {
				c.signed = true
			}
		}
		if parents != 1 || authors != 1 || trees != 1 || !hex40(c.parent) || !hex40(c.tree) {
			return nil, false
		}
		if len(commits) > 0 && c.parent != commits[len(commits)-1].id {
			return nil, false
		}
		commits = append(commits, c)
	}
	return commits, true
}

func replayAttributesUnchanged(repo, endpoint, tip string, git func(string, ...string) ([]byte, error)) bool {
	// Inspect every input commit, including attribute edits later reverted.
	out, err := git(repo, "log", "--format=", "--name-only", "-z", "--no-renames", "--full-history", endpoint+".."+tip, "--", ":(top).gitattributes", ":(top,glob)**/.gitattributes")
	return err == nil && len(out) == 0
}

func replayConfigEquivalent(raw []byte) bool {
	// A fresh detached worktree reads the shared scopes but has no worktree
	// overrides. Compare last-value-wins effective merge settings. Includes
	// are expanded by Git; conditional includes cannot be evaluated for a
	// scratch path that does not yet exist, so their presence means fallback.
	fields := strings.Split(string(raw), "\x00")
	if len(fields) < 3 || fields[len(fields)-1] != "" || len(fields)%2 != 1 {
		return false
	}
	caller, scratch := map[string]string{}, map[string]string{}
	for i := 0; i < len(fields)-1; i += 2 {
		scope := fields[i]
		key, value, _ := strings.Cut(fields[i+1], "\n")
		if strings.HasPrefix(strings.ToLower(key), "includeif.") {
			return false
		}
		if !strings.HasPrefix(key, "merge.") && key != "core.attributesfile" {
			continue
		}
		switch scope {
		case "system", "global", "local", "worktree", "command":
		default:
			return false
		}
		// Relative attribute files resolve from the invocation's directory.
		if key == "core.attributesfile" && value != "" && !filepath.IsAbs(value) {
			return false
		}
		caller[key] = value
		if scope != "worktree" {
			scratch[key] = value
		}
	}
	return maps.Equal(caller, scratch)
}
