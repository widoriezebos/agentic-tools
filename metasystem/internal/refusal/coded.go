package refusal

import "errors"

// Coded is a refusal as a person reads it ("Messages a Person Reads" in
// docs/design/design-principles.md): Error is the plain reason and, when a
// command resolves it, that command on a second "run:" line. The register
// code and any background are its Detail, which only --verbose, --json and
// tests show.
type Coded struct {
	Code       string
	Reason     string
	Run        string
	Background string
}

// Error is the refusal's two lines: the reason, then the command.
func (c *Coded) Error() string {
	if c.Run == "" {
		return c.Reason
	}
	return c.Reason + "\nrun: " + c.Run
}

// Detail is the refusal's code with its reason and background.
func (c *Coded) Detail() string {
	detail := c.Code + ": " + c.Reason
	if c.Background != "" {
		detail += ": " + c.Background
	}
	return detail
}

// Detailed is an error that carries a detail beside its plain text.
type Detailed interface {
	error
	Detail() string
}

// Detail is the detail of the first detailed error in err's chain, or ""
// when it holds none.
func Detail(err error) string {
	var detailed Detailed
	if errors.As(err, &detailed) {
		return detailed.Detail()
	}
	return ""
}

// CodeOf is the register code of the Coded refusal in err's chain, or "".
func CodeOf(err error) string {
	var coded *Coded
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}
