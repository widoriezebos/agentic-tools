package goal

import "errors"

// A coded refusal keeps its refusal code beside its words ("Messages a Person
// Reads", docs/design/design-principles.md): the words are what a person
// reads by default, and the code is data a --verbose or --json reader, a
// record and a register row see. Error() is the words alone.
type codedRefusal struct {
	code string
	err  error
}

func (r *codedRefusal) Error() string { return r.err.Error() }
func (r *codedRefusal) Unwrap() error { return r.err }

// coded marks err with its refusal code; a nil err stays nil.
func coded(code string, err error) error {
	if err == nil {
		return nil
	}
	return &codedRefusal{code: code, err: err}
}

// Coded is coded for another package's refusal.
func Coded(code string, err error) error { return coded(code, err) }

// RefusalCode is the code of the outermost coded refusal in err's chain, or
// empty when err carries none.
func RefusalCode(err error) string {
	var refusal interface{ RefusalCode() string }
	if errors.As(err, &refusal) {
		return refusal.RefusalCode()
	}
	return ""
}

// RefusalCode is the refusal's code.
func (r *codedRefusal) RefusalCode() string { return r.code }

// RecordText is err as a record keeps it: its code, when it has one, before
// its words. Records are read by machinery and --verbose, never by default.
func RecordText(err error) string {
	if err == nil {
		return ""
	}
	if code := RefusalCode(err); code != "" {
		return code + ": " + err.Error()
	}
	return err.Error()
}
