package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
)

type serveProber func(int64) (identity.Exact, identity.Liveness, error)

func (p serveProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) { return p(pid) }

// serveOptions keeps the state root apart from the checkout and the
// installation, so the state a server writes can only be found under the root.
func serveOptions(t *testing.T) Options {
	t.Helper()
	home := t.TempDir()
	roots := Roots{
		Checkout:     filepath.Join(home, "checkout"),
		Installation: filepath.Join(home, "checkout", "metasystem"),
		StateRoot:    filepath.Join(home, "state"),
	}
	return Options{
		Roots: roots, Listen: "127.0.0.1:0", EngineBuild: "dev-test",
		Now:        func() time.Time { return time.Date(2026, 9, 21, 10, 30, 0, 0, time.FixedZone("test", 3600)) },
		After:      func(time.Duration) <-chan time.Time { return nil },
		DigestFunc: func() (string, error) { return "sha256:serving", nil },
		Prober: serveProber(func(pid int64) (identity.Exact, identity.Liveness, error) {
			exact := identity.Exact{Pid: pid, StartedAt: time.Unix(123456, 123000)}
			if runtime.GOOS == "linux" {
				exact.StartTicks, exact.BootID = 123, "test-boot"
			}
			return exact, identity.Alive, nil
		}),
		NewHandler: func(bound net.Addr, rec Record) http.Handler {
			return httpd.New(httpd.Info{Checkout: rec.Checkout, StartedAt: rec.StartedAt, EngineBuild: rec.EngineBuild, ExecutableDigest: rec.ExecutableDigest}, bound, nil)
		},
	}
}

func runTestServer(t *testing.T, o Options) (string, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan string, 1)
	done := make(chan struct{})
	var serveErr error
	o.Ready = func(address string) { ready <- address }
	go func() {
		serveErr = Serve(ctx, o)
		close(done)
	}()
	stop := func() { cancel(); <-done }
	t.Cleanup(func() {
		stop()
		testutil.Expect(t, "server shutdown error", serveErr, nil)
	})
	select {
	case address := <-ready:
		return address, stop
	case <-done:
		testutil.Require(t, "server became ready", serveErr, nil)
		return "", stop
	}
}

func TestServePublishesBoundRecordAndShutsDown(t *testing.T) {
	t.Parallel()
	o := serveOptions(t)
	address, stop := runTestServer(t, o)
	rec, err := readRecord(o.Roots.StateRoot)
	testutil.Require(t, "published record read", err, nil)
	testutil.Expect(t, "record bound address", rec.Address, address)
	_, port, err := net.SplitHostPort(address)
	testutil.Require(t, "bound address shape", err, nil)
	testutil.Expect(t, "kernel assigned a port", port != "0", true)
	testutil.Expect(t, "record checkout", rec.Checkout, o.Roots.Checkout)
	testutil.Expect(t, "record installation", rec.Installation, o.Roots.Installation)
	testutil.Expect(t, "record time is UTC", rec.StartedAt, "2026-09-21T09:30:00Z")
	testutil.Expect(t, "record build", rec.EngineBuild, o.EngineBuild)
	testutil.Expect(t, "record digest", rec.ExecutableDigest, "sha256:serving")
	testutil.Expect(t, "record schema", rec.SchemaVersion, 1)
	ref, err := identity.ParseRef(rec.Process)
	testutil.Require(t, "record identity parses", err, nil)
	testutil.Expect(t, "record pid", ref.Pid, int64(os.Getpid()))

	transport := &http.Transport{}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport}
	response, err := client.Get("http://" + address + "/-/health")
	testutil.Require(t, "health request", err, nil)
	defer response.Body.Close()
	testutil.Expect(t, "health status", response.StatusCode, http.StatusOK)
	var health map[string]string
	testutil.Require(t, "health JSON", json.NewDecoder(response.Body).Decode(&health), nil)
	testutil.Expect(t, "health payload", health, map[string]string{
		"status": "ok", "checkout": o.Roots.Checkout, "startedAt": rec.StartedAt, "engineBuild": o.EngineBuild, "executableDigest": rec.ExecutableDigest, "bundleDigest": "",
	})
	for name, value := range map[string]string{
		"X-Content-Type-Options": "nosniff", "X-Frame-Options": "DENY", "Referrer-Policy": "same-origin", "Cross-Origin-Resource-Policy": "same-origin", "Cache-Control": "no-store",
	} {
		testutil.Expect(t, "health header "+name, response.Header.Get(name), value)
	}
	head, err := client.Head("http://" + address + "/-/health")
	testutil.Require(t, "HEAD request", err, nil)
	body, err := io.ReadAll(head.Body)
	_ = head.Body.Close()
	testutil.Require(t, "HEAD body read", err, nil)
	testutil.Expect(t, "HEAD status", head.StatusCode, http.StatusOK)
	testutil.Expect(t, "HEAD has no body", len(body), 0)
	request, err := http.NewRequest(http.MethodGet, "http://"+address+"/-/health", nil)
	testutil.Require(t, "foreign Host request build", err, nil)
	request.Host = "foreign.invalid"
	refused, err := client.Do(request)
	testutil.Require(t, "foreign Host request", err, nil)
	_ = refused.Body.Close()
	testutil.Expect(t, "foreign Host status", refused.StatusCode, http.StatusForbidden)
	stop()
	_, err = os.Stat(recordPath(o.Roots.StateRoot))
	testutil.Expect(t, "record removed after shutdown", errors.Is(err, os.ErrNotExist), true)
	f, won, err := probeLock(o.Roots.StateRoot)
	testutil.Require(t, "post-shutdown lock probe", err, nil)
	testutil.Require(t, "post-shutdown lock available", won, true)
	releaseLock(f)
}

func TestServeRemovesLeftoverRecordAndRotatesLog(t *testing.T) {
	t.Parallel()
	o := serveOptions(t)
	testutil.Require(t, "create leftover state directory", os.MkdirAll(Dir(o.Roots.StateRoot), 0o755), nil)
	testutil.Require(t, "write unreadable leftover", os.WriteFile(recordPath(o.Roots.StateRoot), []byte("truncated"), 0o644), nil)
	logPath := filepath.Join(Dir(o.Roots.StateRoot), "server.log")
	oldLog := strings.Repeat("x", (1<<20)+1)
	testutil.Require(t, "write oversized log", os.WriteFile(logPath, []byte(oldLog), 0o644), nil)
	appendLog, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0)
	testutil.Require(t, "open existing append descriptor", err, nil)
	defer appendLog.Close()
	digest := o.DigestFunc
	o.DigestFunc = func() (string, error) {
		if _, err := os.Stat(recordPath(o.Roots.StateRoot)); !errors.Is(err, os.ErrNotExist) {
			return "", errors.New("leftover record was not removed before hashing")
		}
		return digest()
	}
	_, stop := runTestServer(t, o)
	backup, err := os.ReadFile(logPath + ".1")
	testutil.Require(t, "read rotated log", err, nil)
	testutil.Expect(t, "rotated bytes preserved", string(backup), oldLog)
	_, err = appendLog.WriteString("new log\n")
	testutil.Require(t, "append descriptor remains usable", err, nil)
	current, err := os.ReadFile(logPath)
	testutil.Require(t, "read current log", err, nil)
	testutil.Expect(t, "log truncated in place", string(current), "new log\n")
	stop()
}

func TestServeRefusesSecondServerWithoutChangingLog(t *testing.T) {
	t.Parallel()
	o := serveOptions(t)
	address, _ := runTestServer(t, o)
	logPath := filepath.Join(Dir(o.Roots.StateRoot), "server.log")
	content := strings.Repeat("x", (1<<20)+1)
	testutil.Require(t, "write active log", os.WriteFile(logPath, []byte(content), 0o644), nil)
	o.After = func(time.Duration) <-chan time.Time { panic("fast refusal must not wait for the lock") }
	err := Serve(context.Background(), o)
	var running *AlreadyRunningError
	testutil.Require(t, "second server refusal type", errors.As(err, &running), true)
	testutil.Expect(t, "second server address", running.Address, address)
	current, err := os.ReadFile(logPath)
	testutil.Require(t, "active log read", err, nil)
	testutil.Expect(t, "active log unchanged", string(current), content)
	_, err = os.Stat(logPath + ".1")
	testutil.Expect(t, "active log not rotated", errors.Is(err, os.ErrNotExist), true)
}

func TestServeLockTimeoutHasUnknownAddress(t *testing.T) {
	t.Parallel()
	o := serveOptions(t)
	testutil.Require(t, "create locked directory", os.MkdirAll(Dir(o.Roots.StateRoot), 0o755), nil)
	f, won, done, err := waitLock(o.Roots.StateRoot, true, time.Second, o.After)
	testutil.Require(t, "hold server lock", err, nil)
	testutil.Require(t, "server lock acquired", won, true)
	<-done
	defer releaseLock(f)
	var wait time.Duration
	o.After = func(duration time.Duration) <-chan time.Time {
		wait = duration
		fired := make(chan time.Time)
		close(fired)
		return fired
	}
	err = Serve(context.Background(), o)
	var running *AlreadyRunningError
	testutil.Require(t, "timeout refusal type", errors.As(err, &running), true)
	testutil.Expect(t, "timeout address unknown", running.Address, "")
	testutil.Expect(t, "default lock wait", wait, 5*time.Second)
	_, err = os.Stat(recordPath(o.Roots.StateRoot))
	testutil.Expect(t, "timeout publishes no record", errors.Is(err, os.ErrNotExist), true)
}

func TestServeDigestFailure(t *testing.T) {
	t.Parallel()
	o := serveOptions(t)
	o.DigestFunc = func() (string, error) { return "", errors.New("read denied") }
	err := Serve(context.Background(), o)
	testutil.Require(t, "digest failure returned", err != nil, true)
	testutil.Expect(t, "digest failure message", err.Error(), "cannot read the serving executable: read denied")
	_, err = os.Stat(recordPath(o.Roots.StateRoot))
	testutil.Expect(t, "digest failure publishes no record", errors.Is(err, os.ErrNotExist), true)
	f, won, err := probeLock(o.Roots.StateRoot)
	testutil.Require(t, "failed server lock probe", err, nil)
	testutil.Require(t, "failed server releases lock", won, true)
	releaseLock(f)
}

func TestServePortInUseNamesConfiguration(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	testutil.Require(t, "occupy a kernel-assigned port", err, nil)
	defer listener.Close()
	o := serveOptions(t)
	o.Listen = listener.Addr().String()
	err = Serve(context.Background(), o)
	testutil.Require(t, "listen failure returned", err != nil, true)
	testutil.Expect(t, "listen failure names flag", strings.Contains(err.Error(), "--listen"), true)
	testutil.Expect(t, "listen failure names key", strings.Contains(err.Error(), "ui.listen"), true)
}

func TestValidateListen(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ input, normalized, refusal string }{
		{"127.0.0.1:0", "127.0.0.1:0", ""},
		{"127.2.3.4:65535", "127.2.3.4:65535", ""},
		{"127.0.0.1:00080", "127.0.0.1:80", ""},
		{"[0:0:0:0:0:0:0:1]:0", "[::1]:0", ""},
		{"localhost:80", "", "use 127.0.0.1"},
		{"unresolvable.invalid:80", "", "use 127.0.0.1"},
		{"0.0.0.0:80", "", "remote browser access is not supported yet"},
		{"192.0.2.1:80", "", "remote browser access is not supported yet"},
		{"[::]:80", "", "remote browser access is not supported yet"},
		{":80", "", "remote browser access is not supported yet"},
		{"127.0.0.1:65536", "", "remote browser access is not supported yet"},
		{"127.0.0.1:-1", "", "remote browser access is not supported yet"},
		{"127.0.0.1:+1", "", "remote browser access is not supported yet"},
		{"127.0.0.1:http", "", "remote browser access is not supported yet"},
		{"127.0.0.1:", "", "remote browser access is not supported yet"},
		{"127.0.0.1", "", "remote browser access is not supported yet"},
	} {
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()
			got, err := ValidateListen(test.input)
			testutil.Expect(t, "normalized address", got, test.normalized)
			if test.refusal == "" {
				testutil.Expect(t, "validation error", err, nil)
			} else {
				testutil.Require(t, "refusal returned", err != nil, true)
				testutil.Expect(t, "refusal guidance", strings.Contains(err.Error(), test.refusal), true)
			}
		})
	}
}

func TestExecutableChanged(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, recorded, current string
		changed                 bool
	}{
		{"same", "sha256:a", "sha256:a", false},
		{"changed", "sha256:a", "sha256:b", true},
		{"no recorded digest", "", "sha256:b", false},
		{"no current digest", "sha256:a", "", false},
		{"neither digest", "", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			testutil.Expect(t, "executable changed", ExecutableChanged(Record{ExecutableDigest: test.recorded}, test.current), test.changed)
		})
	}
}

// F1: work the Ready hook starts must be able to end while this process still
// owns the checkout. A caller that waited only after Serve returned would have
// released the lock and removed the record first, so a predecessor's tick could
// still move the accepted ref after its successor had started. Releasing is the
// hook that closes that window, and this asserts the ordering rather than the
// hook's existence: when it runs, the record is still published and the lock is
// still held, which is what "still owns the checkout" means on disk.
func TestServeEndsReadysWorkBeforeReleasingTheCheckout(t *testing.T) {
	t.Parallel()
	o := serveOptions(t)

	var recordPresentAtRelease, lockHeldAtRelease, ranAtAll bool
	o.Releasing = func() {
		ranAtAll = true
		_, err := readRecord(o.Roots.StateRoot)
		recordPresentAtRelease = err == nil
		// A second waiter cannot take the lock while Serve still holds it.
		_, won, _, lockErr := waitLock(o.Roots.StateRoot, true, 0, nil)
		lockHeldAtRelease = lockErr == nil && !won
	}

	_, stop := runTestServer(t, o)
	stop()

	testutil.Expect(t, "Releasing ran", ranAtAll, true)
	testutil.Expect(t, "the record was still published", recordPresentAtRelease, true)
	testutil.Expect(t, "the lock was still held", lockHeldAtRelease, true)
	_, err := readRecord(o.Roots.StateRoot)
	testutil.Expect(t, "and the record is gone once Serve has returned", err != nil, true)
}
