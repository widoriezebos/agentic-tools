package partner

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/acp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
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
		// Name is the tool's own name where the runtime carries one. An MCP
		// tool has no checkout path and no read kind, so this is the only
		// field that can say which tool it is.
		Name string `json:"name"`
		// Meta is where Claude's adapter puts the same name again, beside the
		// server that serves it.
		Meta struct {
			ClaudeCode struct {
				ToolName  string `json:"toolName"`
				MCPServer struct {
					Name string `json:"name"`
				} `json:"mcpServer"`
			} `json:"claudeCode"`
		} `json:"_meta"`
		Locations []struct {
			Path string `json:"path"`
		} `json:"locations"`
	} `json:"toolCall"`
	Options []acp.PermissionOption `json:"options"`
}

// The one named exception, and how a runtime spells it.
//
// The interface's own tool server is handed to the session at session/new, and
// its calls arrive here looking like nothing this point could classify: kind
// "other", no location, and a title that is the tool's name. That is not a
// runtime misbehaving — it is what an application tool looks like — so the
// point names the exception rather than widening the rule around it: a call
// whose tool name is this server's, followed by one of its operations, is
// admitted, and everything else is refused exactly as before.
//
// The separators below are every way a runtime has been seen to join a server
// to a tool. Claude presents mcp__metasystem__board; the prefix is stripped
// first so that the server's own name is what is matched, never a substring of
// somebody else's.
var toolSeparators = []string{"__", ".", "/", ":", "-"}

var mcpPrefixes = []string{"mcp__", "mcp.", "mcp:", "mcp/"}

// toolOperation answers which of this server's operations a request names, out
// of every field a runtime might carry the name in.
func toolOperation(names ...string) (string, bool) {
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		for _, prefix := range mcpPrefixes {
			if strings.HasPrefix(name, prefix) {
				name = name[len(prefix):]
				break
			}
		}
		for _, separator := range toolSeparators {
			prefix := uitools.ServerName + separator
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			operation := name[len(prefix):]
			if uitools.Names(operation) {
				return operation, true
			}
		}
	}
	return "", false
}

// admitted is what the conversation says about one admitted call to this
// server. All but two of its operations are readings; the two that are not read
// nothing at all and prepare something for a human to decide about, and a line
// calling either a read would be this point saying something happened that did
// not.
func admitted(operation string) string {
	switch operation {
	case uitools.OpSuggest:
		return "Allowed the interface's own tools to prepare a suggestion: nothing was read and nothing was written"
	case uitools.OpDeposit:
		return "Allowed the interface's own tools to prepare a deposit: nothing was read and nothing was written"
	}
	return "Allowed a read through the interface's own tools: " + operation
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
	// The named exception, before the classification, because the exception is
	// exactly the shape the classification cannot read.
	if operation, named := toolOperation(
		request.ToolCall.Name,
		request.ToolCall.Meta.ClaudeCode.ToolName,
		request.ToolCall.Title,
	); named {
		answer := acp.MapVerdict(acp.VerdictAllow, request.Options)
		if answer.Outcome != "selected" {
			return refusal(request.Options, what)
		}
		return decision{answer: answer, allowed: true, activity: admitted(operation)}
	}
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
