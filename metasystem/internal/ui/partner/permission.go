package partner

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/acp"
)

// The permission point: the backstop above the runtime's own read-only
// configuration.
//
// ACP does not require every tool execution to pass through a permission
// request, and a request that does arrive may carry nothing but a tool-call id
// — classification fields are optional and rawInput has no common schema. So
// "could this write?" is not decidable from the wire, and this point does not
// try: it admits exactly one shape, a request whose kind is a read and whose
// every named location resolves inside the checkout, and refuses everything
// else, including everything it cannot read at all.
//
// A refusal is never silent. It becomes an activity line in the conversation,
// naming what was asked for, so a human watching the drawer sees the moment
// the Partner tried to leave its fence.

// permissionRequest is the part of RequestPermissionRequest this point judges.
// Everything else on the wire is journalled by the connection and ignored
// here.
type permissionRequest struct {
	SessionID string `json:"sessionId"`
	ToolCall  struct {
		ToolCallID string `json:"toolCallId"`
		Title      string `json:"title"`
		Kind       string `json:"kind"`
		Locations  []struct {
			Path string `json:"path"`
		} `json:"locations"`
	} `json:"toolCall"`
	Options []acp.PermissionOption `json:"options"`
}

// decision is what the point decided about one request, and the sentence the
// conversation shows for it.
type decision struct {
	answer   acp.PermissionAnswer
	allowed  bool
	activity string
}

// judge decides one permission request against one checkout.
//
// The envelope is fixed and is not configuration: this Partner reads the
// checkout, writes nothing, executes nothing and reaches no network, so there
// is no value a seat could set that would widen it.
func judge(params json.RawMessage, sessionID, checkout string) decision {
	var request permissionRequest
	if err := json.Unmarshal(params, &request); err != nil {
		return refusal(nil, "a permission request this client could not read")
	}
	if request.SessionID != sessionID {
		return refusal(request.Options, "a permission request for another session")
	}
	what := describe(request.ToolCall.Title, request.ToolCall.Kind)
	if request.ToolCall.Kind != "read" {
		return refusal(request.Options, what)
	}
	// The roots the decision is made against are canonical, and so are the
	// paths, or a symbolic link would be a way out of the comparison.
	root := resolve(checkout)
	paths := make([]string, 0, len(request.ToolCall.Locations))
	for _, location := range request.ToolCall.Locations {
		resolved, ok := inside(location.Path, root)
		if !ok {
			return refusal(request.Options, what)
		}
		paths = append(paths, resolved)
	}
	if len(paths) == 0 {
		// A read that names no path is a read of something this client cannot
		// see, which is exactly what "cannot classify" means.
		return refusal(request.Options, what)
	}
	envelope := acp.Envelope{ReadRoots: []string{root}, Network: "deny", Approvals: "deny", Tools: "read-only"}
	verdict := acp.Decide([]acp.Effect{{Class: acp.EffectRead, Paths: paths}}, envelope)
	if verdict != acp.VerdictAllow {
		return refusal(request.Options, what)
	}
	answer := acp.MapVerdict(acp.VerdictAllow, request.Options)
	if answer.Outcome != "selected" {
		// The server offered no single allow_once option, so there is nothing
		// to select; a guess here would be an approval nobody granted.
		return refusal(request.Options, what)
	}
	return decision{answer: answer, allowed: true,
		activity: "Allowed a read inside the checkout: " + what}
}

// refusal is the one refusing answer, with the sentence the drawer shows.
func refusal(options []acp.PermissionOption, what string) decision {
	return decision{answer: acp.StrictAnswer(options), allowed: false,
		activity: "Refused: the Partner reads this checkout and does nothing else — " + what}
}

// describe names what was asked for, from the two fields a request usually
// carries. Neither is evidence of anything; both are what a human reads.
func describe(title, kind string) string {
	title = strings.TrimSpace(title)
	kind = strings.TrimSpace(kind)
	switch {
	case title != "" && kind != "":
		return fmt.Sprintf("%s (%s)", title, kind)
	case title != "":
		return title
	case kind != "":
		return kind
	default:
		return "an unnamed tool call"
	}
}

// inside resolves one path and reports whether it lies within the checkout.
// A relative path is resolved against the checkout, because that is the
// session's own working directory; symbolic links are followed on both sides
// so that a link out of the checkout cannot be read as a path inside it. The
// root reaching here is already canonical.
func inside(path, root string) (string, bool) {
	if strings.TrimSpace(path) == "" {
		return "", false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path = resolve(path)
	if path == root {
		return path, true
	}
	if strings.HasPrefix(path, root+string(filepath.Separator)) {
		return path, true
	}
	return "", false
}

// resolve canonicalises a path whether or not it exists: it follows symbolic
// links as far as the filesystem goes and keeps the rest verbatim. A path that
// is not there yet still has to be judged — a request to read a file that has
// not been created is still a request — and the ancestor that does exist is
// where a link out of the checkout would be.
func resolve(path string) string {
	path = filepath.Clean(path)
	rest := ""
	for {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			if rest == "" {
				return resolved
			}
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return filepath.Join(path, rest)
		}
		rest = filepath.Join(filepath.Base(path), rest)
		path = parent
	}
}
