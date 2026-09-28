package applaunch

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"time"
)

// ProbeOnce asks the contract's readiness probe once. The two probed forms
// answer here; the two observed forms have nothing to ask and say so.
func ProbeOnce(contract Contract, address string) error {
	facts := FactsFor(address)
	switch contract.ReadyKind() {
	case ReadyHTTP:
		return probeHTTP(facts.Substitute(contract.Ready.URL))
	case ReadyTCP:
		return probeTCP(facts.Substitute(contract.Ready.Address))
	}
	return errNotProbed
}

var errNotProbed = errors.New("this readiness form is a startup observation, not a probe")

// NotProbed reports the error ProbeOnce returns for the log and none forms.
func NotProbed(err error) bool { return errors.Is(err, errNotProbed) }

var probeClient = &http.Client{Timeout: 3 * time.Second,
	// A readiness probe asks this one URL. A redirect is an answer about
	// somewhere else, so it is not followed and not counted as ready.
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func probeHTTP(url string) error {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := probeClient.Do(request)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<16))
		_ = response.Body.Close()
	}()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("%s answered %d", url, response.StatusCode)
	}
	return nil
}

func probeTCP(address string) error {
	connection, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return err
	}
	return connection.Close()
}

// LogMatch seeks the log readiness pattern in the log from the offset this
// run's supervisor opened it at. Once seen it is ready for the run's life,
// which is why the caller stops asking; a pattern cannot unmatch, and
// nothing reads it as liveness afterwards.
func LogMatch(path, pattern string, offset int64) (bool, error) {
	expression, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()
	if offset > 0 {
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			return false, err
		}
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if expression.MatchString(scanner.Text()) {
			return true, nil
		}
	}
	return false, scanner.Err()
}

// AwaitReady waits for the contract's readiness form until the deadline. An
// alive check that answers false ends the wait at once: an application that
// exited before it was ready is reported as such, never as running.
func AwaitReady(ctx context.Context, contract Contract, address, logPath string, logOffset int64, alive func() bool, wait time.Duration) error {
	kind := contract.ReadyKind()
	if kind == ReadyNone {
		if alive != nil && !alive() {
			return errExitedBeforeReady
		}
		return nil
	}
	deadline := time.Now().Add(wait)
	for {
		var ready bool
		var err error
		switch kind {
		case ReadyLog:
			ready, err = LogMatch(logPath, contract.Ready.Pattern, logOffset)
			if err != nil {
				return err
			}
		default:
			ready = ProbeOnce(contract, address) == nil
		}
		// Alive is asked after the probe and before its answer is accepted:
		// an answer at the address while the owned child is gone is another
		// service's, never this application's readiness.
		if alive != nil && !alive() {
			return errExitedBeforeReady
		}
		if ready {
			return nil
		}
		if !time.Now().Before(deadline) {
			return errReadyTimeout
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

var (
	errReadyTimeout      = errors.New("the application did not become ready in time")
	errExitedBeforeReady = errors.New("the start command exited before the application became ready")
)

// ReadyTimeout and ExitedBeforeReady name the two ways a wait for readiness
// ends without readiness. They are different sentences because they call for
// different repairs: one is a slow start, the other a start command that
// backgrounds its work and leaves nothing to own.
func ReadyTimeout(err error) bool      { return errors.Is(err, errReadyTimeout) }
func ExitedBeforeReady(err error) bool { return errors.Is(err, errExitedBeforeReady) }
