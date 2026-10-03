package httpd

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// GET /api/fleet/proof-logs/<attempt> (fleet-panel-ux step 2, 2a.3): one
// proof's log, found by the attempt in this computer's lane records, served
// only from the lane's proofs folder. The client never names a path.

// proofLane is a lane installation with two proof records: a red result
// whose log is written, and a running proof whose log is being written. It
// answers the installation and the folder the logs lie in.
func proofLane(t *testing.T, results ...string) (string, string) {
	t.Helper()
	install := t.TempDir()
	proofs := filepath.Join(plain.Dir(install), "proofs")
	testutil.Require(t, "proofs folder", os.MkdirAll(proofs, 0o755), nil)
	lines := append([]string{
		`{"tree":"t1","commit":"c1","result":"red","log":"` + filepath.Join(proofs, "a1.log") + `","at":"2026-10-03T08:00:00Z","attempt":"a1","reason":"the proof command exited 1"}`,
	}, results...)
	testutil.Require(t, "results", os.WriteFile(filepath.Join(plain.Dir(install), "results.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644), nil)
	testutil.Require(t, "running", os.WriteFile(filepath.Join(plain.Dir(install), "running.json"),
		[]byte(`{"attempt":"a2","tree":"t2","commit":"c2","log":"`+filepath.Join(proofs, "a2.log")+`","since":"2026-10-03T08:05:00Z","pid":1}`), 0o644), nil)
	testutil.Require(t, "a1.log", os.WriteFile(filepath.Join(proofs, "a1.log"), []byte("go test ./...\nFAIL internal/x\n\nlanding prove: the proof command exited 1\n"), 0o644), nil)
	testutil.Require(t, "a2.log", os.WriteFile(filepath.Join(proofs, "a2.log"), []byte("go test ./...\n"), 0o644), nil)
	return install, proofs
}

// servedProofLogs is a server whose board serves the proof logs of install
// through plain.ProofLog, as the engine's seam does.
func servedProofLogs(install string) http.Handler {
	return New(Info{Board: &BoardSource{ProofLog: func(attempt string) (string, error) { return plain.ProofLog(install, attempt) }}}, loopback(), testBundle())
}

func TestProofLogServesARecordedAttempt(t *testing.T) {
	t.Parallel()
	install, _ := proofLane(t)
	served := servedProofLogs(install)

	response := request(t, served, http.MethodGet, proofLogPrefix+"a1", "127.0.0.1:7878", map[string]string{"Sec-Fetch-Site": "same-origin"})
	want := "go test ./...\nFAIL internal/x\n\nlanding prove: the proof command exited 1\n"
	testutil.Expect(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "content type", response.Header().Get("Content-Type"), "text/plain; charset=utf-8")
	testutil.Expect(t, "length", response.Header().Get("Content-Length"), strconv.Itoa(len(want)))
	testutil.Expect(t, "the whole log", response.Body.String(), want)

	// A running proof's log is served as far as it is written.
	running := request(t, served, http.MethodGet, proofLogPrefix+"a2", "127.0.0.1:7878", nil)
	testutil.Expect(t, "running status", running.Code, http.StatusOK)
	testutil.Expect(t, "running log so far", running.Body.String(), "go test ./...\n")
}

func TestProofLogRefusesWhatNoRecordNamesOrLiesOutsideTheFolder(t *testing.T) {
	t.Parallel()
	outside := filepath.Join(t.TempDir(), "x.log")
	testutil.Require(t, "outside log", os.WriteFile(outside, []byte("not a proof log\n"), 0o644), nil)
	install, _ := proofLane(t,
		`{"tree":"t3","commit":"c3","result":"red","log":"`+outside+`","at":"2026-10-03T08:10:00Z","attempt":"a3"}`)
	served := servedProofLogs(install)
	for path, words := range map[string]string{
		proofLogPrefix + "unknown":            "no lane record names attempt unknown",
		proofLogPrefix + "../results.jsonl":   "an attempt is one word of letters, digits",
		proofLogPrefix + "..%2Fresults.jsonl": "an attempt is one word of letters, digits",
		proofLogPrefix + "..":                 "an attempt is one word of letters, digits",
		proofLogPrefix:                        "an attempt is one word of letters, digits",
		proofLogPrefix + "a3":                 "the log of attempt a3 is not in the lane's own log folder",
	} {
		response := request(t, served, http.MethodGet, path, "127.0.0.1:7878", nil)
		testutil.Expect(t, path+" status", response.Code, http.StatusNotFound)
		testutil.Expect(t, path+" in words", strings.Contains(response.Body.String(), words), true)
		testutil.Expect(t, path+" serves no file", strings.Contains(response.Body.String(), "not a proof log") || strings.Contains(response.Body.String(), `"tree"`), false)
	}
}

// A log a record names in the proofs folder that is no longer there is a 404
// that says so; a record the server cannot read is a 500 in words, never a
// "no such log"; and a build with no reader says that.
func TestProofLogSaysAGoneLogAndARecordItCannotRead(t *testing.T) {
	t.Parallel()
	install, proofs := proofLane(t)
	testutil.Require(t, "remove a1.log", os.Remove(filepath.Join(proofs, "a1.log")), nil)
	gone := request(t, servedProofLogs(install), http.MethodGet, proofLogPrefix+"a1", "127.0.0.1:7878", nil)
	testutil.Expect(t, "gone status", gone.Code, http.StatusNotFound)
	testutil.Expect(t, "gone in words", strings.Contains(gone.Body.String(), "the log of attempt a1 is gone"), true)

	broken := New(Info{Board: &BoardSource{ProofLog: func(string) (string, error) {
		return "", errors.New("the proof results can't be read: permission denied")
	}}}, loopback(), testBundle())
	unread := request(t, broken, http.MethodGet, proofLogPrefix+"a1", "127.0.0.1:7878", nil)
	testutil.Expect(t, "unread status", unread.Code, http.StatusInternalServerError)
	testutil.Expect(t, "unread in words", strings.Contains(unread.Body.String(), "the log of attempt a1 can't be read: the proof results can't be read: permission denied"), true)

	for name, info := range map[string]Info{"no board": {}, "no reader": {Board: &BoardSource{}}} {
		none := request(t, New(info, loopback(), testBundle()), http.MethodGet, proofLogPrefix+"a1", "127.0.0.1:7878", nil)
		testutil.Expect(t, name+" status", none.Code, http.StatusInternalServerError)
		testutil.Expect(t, name+" in words", strings.Contains(none.Body.String(), "this engine was built without a landing-log reader"), true)
	}
}

func TestProofLogTakesGetAndHeadOnly(t *testing.T) {
	t.Parallel()
	install, _ := proofLane(t)
	served := servedProofLogs(install)

	head := request(t, served, http.MethodHead, proofLogPrefix+"a1", "127.0.0.1:7878", nil)
	testutil.Expect(t, "head status", head.Code, http.StatusOK)
	testutil.Expect(t, "head length", head.Header().Get("Content-Length") != "", true)
	testutil.Expect(t, "head body", head.Body.String(), "")

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		refused := request(t, served, method, proofLogPrefix+"a1", "127.0.0.1:7878", nil)
		testutil.Expect(t, method+" status", refused.Code, http.StatusMethodNotAllowed)
		testutil.Expect(t, method+" allow", refused.Header().Get("Allow"), "GET, HEAD")
	}
	// The read policy every read takes: the allowed host, the same site, one origin.
	testutil.Expect(t, "foreign host", request(t, served, http.MethodGet, proofLogPrefix+"a1", "attacker.invalid", nil).Code, http.StatusForbidden)
	testutil.Expect(t, "cross-site fetch", request(t, served, http.MethodGet, proofLogPrefix+"a1", "127.0.0.1:7878",
		map[string]string{"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Mode": "cors"}).Code, http.StatusForbidden)
	testutil.Expect(t, "foreign origin", request(t, served, http.MethodGet, proofLogPrefix+"a1", "127.0.0.1:7878",
		map[string]string{"Origin": "http://attacker.invalid"}).Code, http.StatusForbidden)
}
