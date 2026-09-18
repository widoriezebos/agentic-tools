package batch

import (
	"errors"
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
)

// boundaryRefusal names a policy decision made at a batch boundary. Callers
// use its type, rather than error text, when deciding whether a member may be
// ejected from a landing batch.
type boundaryRefusal struct {
	Code, Detail string
}

func (refusal *boundaryRefusal) Error() string {
	return fmt.Sprintf("%s: %s", refusal.Code, refusal.Detail)
}

func refuseBatch(code, detail string) error {
	return &boundaryRefusal{Code: code, Detail: detail}
}

func isBoundaryRefusal(err error) bool {
	var batchRefusal *boundaryRefusal
	if errors.As(err, &batchRefusal) {
		return true
	}
	var contractRefusal *contractgit.Refusal
	return errors.As(err, &contractRefusal)
}
