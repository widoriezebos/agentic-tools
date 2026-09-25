package batch

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// protectedTestWorkspace serves only the tree reads used by the protected Go test checks.
func protectedTestWorkspace(t *testing.T, root string, trees map[string]map[string][]byte, paths []string, headTree string) gittree.Workspace {
	t.Helper()
	allowedPaths := make(map[string]bool, len(paths))
	for _, path := range paths {
		allowedPaths[path] = true
	}
	blobs := map[string][]byte{}
	entries := map[string]map[string]string{}
	for tree, files := range trees {
		entries[tree] = map[string]string{}
		for path, content := range files {
			copyOfContent := bytes.Clone(content)
			hash := sha1.New()
			fmt.Fprintf(hash, "blob %d\x00", len(copyOfContent))
			hash.Write(copyOfContent)
			oid := hex.EncodeToString(hash.Sum(nil))
			entries[tree][path] = oid
			blobs[oid] = copyOfContent
		}
	}
	prefix := append([]string{"-C", root}, []string{
		"-c", "core.fileMode=true",
		"-c", "diff.noprefix=false",
		"-c", "diff.mnemonicPrefix=false",
		"-c", "apply.ignoreWhitespace=no",
		"-c", "core.logAllRefUpdates=false",
		"-c", "core.useReplaceRefs=false",
		"-c", "gc.auto=0",
		"-c", "maintenance.auto=false",
	}...)
	return gittree.Workspace{Dir: root, RawSource: func(request gittree.RawRequest) gittree.RawResult {
		t.Helper()
		if request.Dir != root || request.Stdin != nil || !slices.Equal(request.Env, gittree.ScrubbedEnviron()) ||
			len(request.Args) < len(prefix) || !slices.Equal(request.Args[:len(prefix)], prefix) {
			t.Fatalf("unexpected protected-test raw request: %+v", request)
		}
		args := request.Args[len(prefix):]
		if request.Operation != "git "+strings.Join(args, " ") {
			t.Fatalf("protected-test operation = %q, args = %q", request.Operation, args)
		}
		switch {
		case headTree != "" && slices.Equal(args, []string{"rev-parse", "HEAD^{tree}"}):
			return gittree.RawResult{Stdout: []byte(headTree + "\n")}
		case headTree != "" && slices.Equal(args, []string{"rev-parse", "--show-prefix"}):
			return gittree.RawResult{}
		case len(args) == 8 && slices.Equal(args[:5], []string{"--literal-pathspecs", "ls-tree", "-r", "-z", "--full-tree"}) && args[6] == "--":
			files, knownTree := entries[args[5]]
			query := args[7]
			if !knownTree || !allowedPaths[query] {
				t.Fatalf("unexpected protected-test tree/path: tree=%q path=%q", args[5], query)
			}
			var names []string
			for path := range files {
				if path == query || strings.HasPrefix(path, query+"/") {
					names = append(names, path)
				}
			}
			sort.Strings(names)
			var raw bytes.Buffer
			for _, path := range names {
				fmt.Fprintf(&raw, "100644 blob %s\t%s\x00", files[path], path)
			}
			return gittree.RawResult{Stdout: raw.Bytes()}
		case len(args) == 3 && args[0] == "cat-file" && args[1] == "blob":
			content, knownBlob := blobs[args[2]]
			if !knownBlob {
				t.Fatalf("unexpected protected-test blob: %q", args[2])
			}
			return gittree.RawResult{Stdout: bytes.Clone(content)}
		default:
			t.Fatalf("unexpected protected-test raw command: %q", args)
		}
		return gittree.RawResult{}
	}}
}
