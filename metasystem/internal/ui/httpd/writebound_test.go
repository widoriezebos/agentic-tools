package httpd

// What the shared decoder takes, at its two edges.
//
// The bound is DECLARED: a write body is a title, a handful of ids and a
// sentence, and this server does not carry a megabyte of them into memory. It
// was not enforced — the reader was limited to one byte past the bound and
// nothing counted what came back, so a complete object exactly one byte over
// was decoded and written. And "one JSON value" was asked of json.Decoder.More,
// which reports whether another element of the array or object being parsed
// follows: a body ending in a stray `]` said no, and the write went through
// (Astra A-04). The document's own decoder counts its bytes; this one now does
// the same, and asks for the end of the document rather than for the absence of
// a sibling.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func TestAWriteBodyOneBytePastTheBoundIsRefusedForItsSize(t *testing.T) {
	t.Parallel()
	const prefix = `{"kind":"design","title":"`
	const suffix = `"}`
	rec := &recorder{}
	served := New(rec.writing(), loopback(), testBundle())

	response := post(t, served, recordsPath,
		prefix+strings.Repeat("x", maxWriteBody+1-len(prefix)-len(suffix))+suffix, nil)

	testutil.Expect(t, "status", response.Code, http.StatusRequestEntityTooLarge)
	testutil.Expect(t, "the writer was not reached", rec.reached(), 0)
}

// A body at the bound is still taken: the bound is what it says it is, and a
// refusal one byte early would be a different bound.
func TestAWriteBodyAtTheBoundIsTaken(t *testing.T) {
	t.Parallel()
	const prefix = `{"kind":"design","title":"`
	const suffix = `"}`
	rec := &recorder{}
	served := New(rec.writing(), loopback(), testBundle())

	response := post(t, served, recordsPath,
		prefix+strings.Repeat("x", maxWriteBody-len(prefix)-len(suffix))+suffix, nil)

	testutil.Expect(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "the writer was reached once", len(rec.records), 1)
}

func TestAWriteBodyWithJSONAfterTheObjectIsRefused(t *testing.T) {
	t.Parallel()
	for _, trailing := range []struct {
		what string
		body string
	}{
		{"a closing bracket nobody opened", `{"kind":"design","title":"A"}]`},
		{"a closing brace nobody opened", `{"kind":"design","title":"A"}}`},
		{"a second object", `{"kind":"design","title":"A"}{"kind":"design","title":"B"}`},
		{"a bare word", `{"kind":"design","title":"A"} nonsense`},
	} {
		t.Run(trailing.what, func(t *testing.T) {
			t.Parallel()
			rec := &recorder{}
			served := New(rec.writing(), loopback(), testBundle())

			response := post(t, served, recordsPath, trailing.body, nil)

			testutil.Expect(t, "status", response.Code, http.StatusBadRequest)
			testutil.Expect(t, "the writer was not reached", rec.reached(), 0)
			testutil.Expect(t, "the refusal says what is wrong",
				refusalBody(t, "the refusal", response).Error,
				"the request body carries more than one JSON value")
		})
	}
}

// And what is NOT a second value: the whitespace a client's serializer leaves
// after an object is the end of the document.
func TestAWriteBodyThatEndsInWhitespaceIsTaken(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	served := New(rec.writing(), loopback(), testBundle())

	response := post(t, served, recordsPath, `{"kind":"design","title":"A"}`+"\n", nil)

	testutil.Expect(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "the writer was reached once", len(rec.records), 1)
}

// The document decoder had the same hole as the write decoder: json.Decoder.More
// answered "no sibling" to a stray closing bracket, and the preview or the save
// went through with junk after its object. It asks for the end of the document
// now, as decode does (the Go builder's "left" after Astra A-04).
func TestADocumentBodyWithJSONAfterTheObjectIsRefused(t *testing.T) {
	t.Parallel()
	for _, trailing := range []struct {
		what string
		body string
	}{
		{"a closing bracket nobody opened", `{"source":"# A\n"}]`},
		{"a closing brace nobody opened", `{"source":"# A\n"}}`},
		{"a bare word", `{"source":"# A\n"} nonsense`},
	} {
		t.Run(trailing.what, func(t *testing.T) {
			t.Parallel()
			rec := &recorder{}
			served := New(rec.writing(), loopback(), testBundle())

			response := post(t, served, "/api/documents/preview", trailing.body, nil)

			testutil.Expect(t, "status", response.Code, http.StatusBadRequest)
			testutil.Expect(t, "the reader was not reached", len(rec.previews), 0)
			testutil.Expect(t, "the refusal says what is wrong",
				refusalBody(t, "the refusal", response).Error,
				"the request body carries more than one JSON value")
		})
	}
}

func TestADocumentBodyThatEndsInWhitespaceIsTaken(t *testing.T) {
	t.Parallel()
	rec := &recorder{}
	served := New(rec.writing(), loopback(), testBundle())

	response := post(t, served, "/api/documents/preview", `{"source":"# A\n"}`+"\n", nil)

	testutil.Expect(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "the reader was reached once", len(rec.previews), 1)
}
