package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// The whiteboard (g1-s71): the review of g1-s21 names an Evidence path, copied
// from its design, that lies outside the checkout, as a build lane's does; the
// Behaves walk presents from the listing it is handed; and a question about
// the handoff is answered with a drawing.

// whiteboardDesign is the design g1-s21's review copies its Evidence line from.
const whiteboardDesign = "plans/designs/overview-reads.md"

// whiteboardDesignText is that design, with the evidence path written in.
func whiteboardDesignText(evidence string) string {
	return "# The Overview reads what needs a human\n\n- Kind: design\n- Id: design-overview-reads\n" +
		"- Status: accepted\n- Goals: g1-s21\n- Evidence: " + evidence + "\n\n## Outcome\n\n" +
		"The Overview names what waits on a human, and nothing else.\n"
}

// evidenceDir is where this fixture keeps the build's evidence: beside the
// checkout and never inside it.
func evidenceDir(checkout string) string { return checkout + "-evidence" }

// plantEvidence writes the evidence a build lane would have left: a
// screenshot, the report it wrote, and a headingless run log.
func plantEvidence(dir string) error {
	shot, err := screenshot()
	if err != nil {
		return err
	}
	for name, body := range map[string][]byte{
		"room-1280-light.png": shot,
		"report.md": []byte("# Build report: the Overview reads what needs a human\n\n## What ran\n\n" +
			"The Overview opened on the fixture with two goals waiting; each was named once.\n\n" +
			"## What was not tried\n\nA workspace with nothing waiting.\n"),
		"run.txt": []byte("started the walkthrough server on 127.0.0.1:7979\nopened /overview at 1280 wide\n" +
			"two goals named as waiting on a human\nstopped\n"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("cannot make the evidence directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			return fmt.Errorf("cannot plant the evidence %s: %v", name, err)
		}
	}
	return nil
}

// screenshot is a small picture of a page: a header band, a rail and two
// cards, which is enough to see an image on the desk as an image.
func screenshot() ([]byte, error) {
	picture := image.NewRGBA(image.Rect(0, 0, 640, 400))
	fill := func(x0, y0, x1, y1 int, shade color.RGBA) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				picture.Set(x, y, shade)
			}
		}
	}
	fill(0, 0, 640, 400, color.RGBA{0xfa, 0xf8, 0xf3, 0xff})
	fill(0, 0, 640, 48, color.RGBA{0x2b, 0x2a, 0x27, 0xff})
	fill(0, 48, 120, 400, color.RGBA{0xee, 0xeb, 0xe3, 0xff})
	fill(150, 80, 610, 180, color.RGBA{0xff, 0xff, 0xff, 0xff})
	fill(150, 80, 156, 180, color.RGBA{0xc8, 0x8a, 0x1e, 0xff})
	fill(150, 210, 610, 310, color.RGBA{0xff, 0xff, 0xff, 0xff})
	fill(150, 210, 156, 310, color.RGBA{0x3a, 0x7d, 0x44, 0xff})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		return nil, fmt.Errorf("cannot draw the fixture's screenshot: %v", err)
	}
	return encoded.Bytes(), nil
}

// The phrases the whiteboard's canned answers are narrowed to.
const (
	behavesWalk   = "Walk me through Behaves"
	handoffAsking = "handoff"
)

// whiteboardAnswers are the Behaves walk and the drawing.
var whiteboardAnswers = []fakeacp.Answer{
	{When: behavesWalk, Chunks: []string{
		"Behaves, from the evidence the build recorded.\n\n",
		"First the Overview at 1280 in the light theme, `room-1280-light.png`; then the run log the lane wrote, " +
			"`run.txt`, which has no headings.\n",
	}},
	{When: handoffAsking, Chunks: []string{
		"The handoff, as the owner runs it:\n\n",
		"```mermaid\nsequenceDiagram\n  participant P as Press\n  participant O as Owner\n  participant L as Ledger\n" +
			"  P->>O: begin(press)\n  O->>O: take the lock\n  O->>L: publish\n  O->>L: reconcile\n  O->>O: release\n" +
			"  O-->>P: done\n```\n\n",
		"The lock is held across both calls, `internal/owner/owner.go:14-27`.\n",
	}},
}

// whiteboardReads are what the Behaves walk puts on the desk: the evidence, by
// its evidence-relative paths, as the listing named them.
var whiteboardReads = []fakeacp.Read{
	{
		When:   behavesWalk,
		Name:   "mcp__" + uitools.ServerName + "__" + uitools.OpPresent,
		Title:  "present(evidence)",
		Result: uitools.PresentedLine + "\n" + uitools.PresentHeader + "evidence\n" + uitools.PresentPath + "room-1280-light.png\n",
	},
	{
		When:   behavesWalk,
		Name:   "mcp__" + uitools.ServerName + "__" + uitools.OpPresent,
		Title:  "present(evidence)",
		Result: uitools.PresentedLine + "\n" + uitools.PresentHeader + "evidence\n" + uitools.PresentPath + "run.txt\n",
	},
}
