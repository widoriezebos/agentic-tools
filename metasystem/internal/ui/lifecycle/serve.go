package lifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type Options struct {
	Checkout      string
	Listen        string
	EngineBuild   string
	DigestFunc    func() (string, error)
	NewHandler    func(bound net.Addr, rec Record) http.Handler
	Prober        identity.Prober
	Ready         func(address string)
	LockWait      time.Duration
	After         func(time.Duration) <-chan time.Time
	ShutdownGrace time.Duration
	Now           func() time.Time
}

type AlreadyRunningError struct{ Address string }

func (e *AlreadyRunningError) Error() string {
	if e.Address != "" {
		return fmt.Sprintf("an interface server already runs for this checkout at http://%s; stop it with: metasystem ui stop", e.Address)
	}
	return "an interface server already runs for this checkout"
}

func ValidateListen(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid listen address: remote browser access is not supported yet: %w", err)
	}
	ip := net.ParseIP(host)
	if host != "" && ip == nil {
		return "", errors.New("the listen address must use a loopback IP literal; use 127.0.0.1")
	}
	if ip == nil || !ip.IsLoopback() {
		return "", errors.New("remote browser access is not supported yet")
	}
	if port == "" {
		return "", errors.New("the listen port must be numeric, from 0 to 65535; remote browser access is not supported yet")
	}
	for _, digit := range port {
		if digit < '0' || digit > '9' {
			return "", errors.New("the listen port must be numeric, from 0 to 65535; remote browser access is not supported yet")
		}
	}
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return "", errors.New("the listen port must be numeric, from 0 to 65535; remote browser access is not supported yet")
	}
	return net.JoinHostPort(ip.String(), strconv.FormatUint(n, 10)), nil
}

func ExecutableDigest() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", hash.Sum(nil)), nil
}

func ExecutableChanged(rec Record, currentDigest string) bool {
	return rec.ExecutableDigest != "" && currentDigest != "" && rec.ExecutableDigest != currentDigest
}

func Serve(ctx context.Context, o Options) (result error) {
	listen, err := ValidateListen(o.Listen)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(Dir(o.Checkout), 0o755); err != nil {
		return err
	}
	if rec, err := readRecord(o.Checkout); err == nil {
		ref, _ := identity.ParseRef(rec.Process)
		if identity.AliveRef(o.Prober, ref) == identity.Alive {
			return &AlreadyRunningError{rec.Address}
		}
	}
	if o.LockWait == 0 {
		o.LockWait = 5 * time.Second
	}
	f, won, _, err := waitLock(o.Checkout, true, o.LockWait, o.After)
	if err != nil {
		return err
	}
	if !won {
		refusal := &AlreadyRunningError{}
		if rec, err := readRecord(o.Checkout); err == nil {
			refusal.Address = rec.Address
		}
		return refusal
	}
	defer releaseLock(f)
	if err := removeRecord(o.Checkout); err != nil {
		return err
	}
	if err := rotateLog(o.Checkout); err != nil {
		return err
	}
	self, state, err := o.Prober.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return errors.New("the interface server cannot read its own identity")
	}
	process, err := identity.EncodeRef(self.Ref())
	if err != nil {
		return err
	}
	if o.DigestFunc == nil {
		o.DigestFunc = ExecutableDigest
	}
	digest, err := o.DigestFunc()
	if err != nil {
		return fmt.Errorf("cannot read the serving executable: %w", err)
	}
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("cannot listen at %s; choose another address with --listen or ui.listen: %w", listen, err)
	}
	defer listener.Close()
	if o.Now == nil {
		o.Now = time.Now
	}
	rec := Record{1, process, listener.Addr().String(), o.Checkout, o.Now().UTC().Format(time.RFC3339), o.EngineBuild, digest}
	server := &http.Server{
		Handler:           o.NewHandler(listener.Addr(), rec),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if _, err := atomicfile.WriteText(recordPath(o.Checkout), string(data)+"\n", o.Checkout); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, removeRecord(o.Checkout)) }()
	if o.Ready != nil {
		o.Ready(rec.Address)
	}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	select {
	case err := <-served:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		if o.ShutdownGrace == 0 {
			o.ShutdownGrace = 10 * time.Second
		}
		shutdown, cancel := context.WithTimeout(context.Background(), o.ShutdownGrace)
		defer cancel()
		_ = server.Shutdown(shutdown)
		_ = server.Close()
		<-served
	}
	return nil
}

func rotateLog(checkout string) error {
	path := filepath.Join(Dir(checkout), "server.log")
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || info.Size() <= 1<<20 {
		return err
	}
	source, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.Create(path + ".1")
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return err
	}
	return source.Truncate(0)
}
