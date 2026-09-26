// Package uihome is where the interface keeps an account's own material:
// under the account's registry home, outside every checkout.
//
// # Why there is a directory outside the checkout at all
//
// The interface's ordinary per-human state — the visit marker — lives in the
// checkout's state root, which on both layouts lies inside the checkout. That
// is right for a marker and wrong for two things this interface now keeps: a
// human's notepad, and the Project Partner's transcript. The Partner's own
// permission owner grants native reads anywhere inside the checkout, and a
// critic is handed the repository as a read root, so either of those written
// there would be readable by a seat from the moment it was written.
//
// The master says the transcript is private sitting material in a protected
// server-local store outside the checkout, which examiners never read, and it
// keeps a human's personal working material out of the project's records until
// the working-material owner exists. The account's registry home is the one
// directory the kit already resolves that is outside every checkout, so it is
// where both go — and the builds that put them there prove the two grants do
// not reach them by reading the grants rather than by asserting it in a
// comment (internal/ui/stickies/grants_test.go, internal/ui/partner/grants_test.go).
//
// # Why it is one owner and not one per store
//
// The notepad and the transcript need the same three decisions: which home,
// refused how when the account has none, and which directory one workspace's
// files go in. Two copies of those would be two copies that eventually differ
// — and the one that differed would be the one that quietly wrote a private
// transcript into the checkout. So they are here, once, and each store names
// the part of the home it owns.
package uihome

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// Home is the account's registry home: the directory the kit already resolves
// for seat registration, outside every checkout.
//
// It is derived from the registry's own selected path rather than resolved a
// second time here, so that the fixture seam the registry offers — one
// environment variable naming a run-scoped home — moves every store under it.
// A test that redirected one and not the other would be a test writing into
// the real account's home, which is the one thing a test of these stores must
// never do.
//
// A selected path that is not absolute is refused rather than used. Where the
// account has no home directory the kit can read, the registry falls back to a
// relative `.metasystem/armed-checkouts.jsonl`, and a relative path resolves
// against the working directory — which for this server is the checkout it
// serves. The material would then land inside the one place this package
// exists to keep it out of, silently, on the machine least able to notice it.
// So such a build has no store at all, and the caller says so.
func Home() (string, error) {
	selected, err := registry.DefaultPath()
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(selected) {
		return "", fmt.Errorf(
			"the account's registry home resolved to %q, which is not an absolute path, so anything kept under it would land inside whatever directory this server was started in",
			selected)
	}
	return filepath.Dir(selected), nil
}

// Under is the directory one owner keeps one workspace's files in:
// <home>/ui/<owner>/<workspace key>.
//
// The directory is keyed by the workspace rather than named after it: a
// checkout path is not a path segment, two checkouts can share a last name,
// and a name derived by hand would collide or escape. So the key is the
// checkout's own last name, which is what a human recognises, with a short
// digest of the whole absolute path after it, which is what makes it that
// checkout and no other.
func Under(home, owner, checkout string) string {
	return filepath.Join(home, "ui", owner, workspaceKey(checkout))
}

// Anchor is the directory an atomic write may make durable up to: the one
// above the registry home, which pre-exists every run, because everything
// below it a store may create for itself.
func Anchor(home string) string { return filepath.Dir(home) }

// workspaceKey is the one directory segment a checkout is kept under.
func workspaceKey(checkout string) string {
	resolved := checkout
	if absolute, err := filepath.Abs(checkout); err == nil {
		resolved = absolute
	}
	resolved = filepath.Clean(resolved)
	digest := sha256.Sum256([]byte(resolved))
	name := filepath.Base(resolved)
	// A base that is not a name at all — the root, a relative dot — leaves the
	// digest to say which workspace this is on its own.
	if name == "." || name == string(filepath.Separator) || name == ".." {
		return hex.EncodeToString(digest[:6])
	}
	return safeName(name) + "-" + hex.EncodeToString(digest[:6])
}

// safeName is a checkout's last name with everything that is not a plain
// letter, digit, dot, dash or underscore replaced, so the segment is a name on
// every filesystem this runs on.
func safeName(name string) string {
	var built strings.Builder
	for _, letter := range name {
		switch {
		case letter >= 'a' && letter <= 'z',
			letter >= 'A' && letter <= 'Z',
			letter >= '0' && letter <= '9',
			letter == '.' || letter == '-' || letter == '_':
			built.WriteRune(letter)
		default:
			built.WriteByte('-')
		}
	}
	return built.String()
}
